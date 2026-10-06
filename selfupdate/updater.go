package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/komari-monitor/komari-agent/version"
)

const (
	updateRepoOwner = "aomtest"
	updateRepoName  = "komari-slim-agent"

	metadataTimeout = 30 * time.Second
	downloadTimeout = 10 * time.Minute
	maxAssetSize    = 200 << 20
	maxBackups      = 3
)

type asset struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
	URL    string `json:"browser_download_url"`
}

type release struct {
	TagName string `json:"tag_name"`
	Draft   bool   `json:"draft"`
	Pre     bool   `json:"prerelease"`
	Assets  []asset `json:"assets"`
}

// Update 执行一次自更新。返回值只用于日志,调用方不需要区分处理。
//
// 流程刻意做成「先查版本、再决定要不要动」:这一步让重放攻击失去意义 ——
// 第一次触发会真的更新,之后 agent 已是最新版,再收到触发就只查一次
// 版本号然后返回,不下载、不替换、不重启。
func Update() error {
	if runtime.GOOS == "windows" {
		// 提前拒绝,而不是等到 rename 那一步才失败 —— 那条错误信息
		// 对使用者没有任何指导意义。
		return fmt.Errorf("self-update is not supported on Windows; update manually")
	}

	current := version.CurrentVersion
	log.Printf("[selfupdate] update requested, current version %s", current)

	rel, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("cannot check the latest release: %w", err)
	}
	if !isNewer(current, rel.TagName) {
		log.Printf("[selfupdate] already up to date (%s), nothing to do", current)
		return nil
	}

	a, err := pickAsset(rel)
	if err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine the running binary: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("cannot resolve the binary path: %w", err)
	}

	log.Printf("[selfupdate] updating %s -> %s", current, rel.TagName)

	// 下载到二进制同目录,保证后续 rename 是同文件系统操作。
	tmp, digest, err := download(a, filepath.Dir(exe))
	if err != nil {
		return err
	}
	defer func() {
		// 替换成功后文件已被 rename 走,这里删不到东西;失败时清理残留。
		os.Remove(tmp)
	}()

	if !strings.EqualFold(digest, strings.TrimPrefix(a.Digest, "sha256:")) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s",
			strings.TrimPrefix(a.Digest, "sha256:"), digest)
	}

	// 先把当前二进制 rename 成备份,再把新文件放上去。
	// 顺序很重要:如果直接删掉旧文件、rename 新文件失败,就没有二进制可用了。
	backupPath, err := backupBinary(exe)
	if err != nil {
		return err
	}

	if err := os.Rename(tmp, exe); err != nil {
		// 替换失败 —— 把备份还原回去,agent 仍能继续跑旧版本。
		if rerr := os.Rename(backupPath, exe); rerr != nil {
			return fmt.Errorf("failed to place the new binary (%v) and could not restore the backup (%v); manual intervention required", err, rerr)
		}
		return fmt.Errorf("failed to place the new binary: %w", err)
	}
	if err := os.Chmod(exe, 0o755); err != nil {
		log.Printf("[selfupdate] warning: cannot set permissions on %s: %v", exe, err)
	}

	cleanOldBackups(filepath.Dir(exe))

	log.Printf("[selfupdate] %s installed, restarting", rel.TagName)
	return restartSelf(exe)
}

// fetchLatestRelease 拉取最新正式 release。
func fetchLatestRelease() (*release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest",
		updateRepoOwner, updateRepoName)

	client := &http.Client{Timeout: metadataTimeout}
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "komari-slim-agent")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release API returned HTTP %d", resp.StatusCode)
	}

	var rel release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		return nil, err
	}
	if rel.Draft || rel.Pre {
		return nil, fmt.Errorf("latest release is not a stable release")
	}
	return &rel, nil
}

// pickAsset 挑出当前平台对应的资产。
func pickAsset(rel *release) (*asset, error) {
	want, err := currentAssetName()
	if err != nil {
		return nil, err
	}
	for i := range rel.Assets {
		a := &rel.Assets[i]
		if a.Name != want {
			continue
		}
		// 没有 digest 就拒绝更新,不降级成「下载了就装」。
		if !strings.HasPrefix(a.Digest, "sha256:") {
			return nil, fmt.Errorf("asset %q has no usable sha256 digest", want)
		}
		return a, nil
	}
	return nil, fmt.Errorf("release %s has no asset named %q", rel.TagName, want)
}

// currentAssetName 返回当前平台对应的资产文件名。
func currentAssetName() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return "komari-agent-linux-" + runtime.GOARCH, nil
	case "windows":
		return "komari-agent-windows-" + runtime.GOARCH + ".exe", nil
	default:
		return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

// download 下载资产到 dir 下的临时文件,边下边算 SHA-256。
func download(a *asset, dir string) (string, string, error) {
	client := &http.Client{Timeout: downloadTimeout}
	req, _ := http.NewRequest(http.MethodGet, a.URL, nil)
	req.Header.Set("User-Agent", "komari-slim-agent")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp(dir, ".komari-agent-update-*.tmp")
	if err != nil {
		return "", "", fmt.Errorf("cannot create a temp file next to the binary: %w", err)
	}
	tmpPath := tmp.Name()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, hasher), io.LimitReader(resp.Body, maxAssetSize+1))
	if err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("download interrupted: %w", err)
	}
	if written > maxAssetSize {
		tmp.Close()
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("asset exceeds the %d byte limit", int64(maxAssetSize))
	}
	if a.Size > 0 && written != a.Size {
		tmp.Close()
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("size mismatch: expected %d bytes, got %d", a.Size, written)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", "", err
	}
	tmp.Close()

	return tmpPath, hex.EncodeToString(hasher.Sum(nil)), nil
}

// backupDir 返回备份目录,固定在二进制旁边。
func backupDir(exeDir string) string {
	return filepath.Join(exeDir, "backup")
}

// backupBinary 把当前二进制原子重命名为备份。
func backupBinary(exe string) (string, error) {
	dir := backupDir(filepath.Dir(exe))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("cannot create the backup directory: %w", err)
	}

	name := fmt.Sprintf("komari-agent.bak.%s.%s",
		version.CurrentVersion, time.Now().Format("20060102-150405"))
	target := filepath.Join(dir, name)

	if err := os.Rename(exe, target); err != nil {
		return "", fmt.Errorf("cannot move the current binary aside: %w", err)
	}
	return target, nil
}

// cleanOldBackups 只保留最近 maxBackups 个备份。
func cleanOldBackups(exeDir string) {
	dir := backupDir(exeDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var backups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "komari-agent.bak.") {
			backups = append(backups, filepath.Join(dir, e.Name()))
		}
	}
	if len(backups) <= maxBackups {
		return
	}
	sort.Strings(backups)
	for _, p := range backups[:len(backups)-maxBackups] {
		os.Remove(p)
	}
}

// isNewer 判断 latest 是否比 current 新。
func isNewer(current, latest string) bool {
	c := parseVersion(current)
	l := parseVersion(latest)
	if len(l) == 0 {
		return false
	}
	if len(c) == 0 {
		return true
	}
	for i := 0; i < len(c) || i < len(l); i++ {
		var cv, lv int
		if i < len(c) {
			cv = c[i]
		}
		if i < len(l) {
			lv = l[i]
		}
		if lv != cv {
			return lv > cv
		}
	}
	return false
}

// parseVersion 把版本号拆成整数段,容忍 v 前缀和预发布后缀。
func parseVersion(v string) []int {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if v == "" {
		return nil
	}
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	return out
}
