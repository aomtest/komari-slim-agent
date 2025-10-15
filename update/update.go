package update

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/komari-monitor/komari-agent/dnsresolver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
)

var (
	CurrentVersion string = "0.0.1"
	Repo           string = "komari-monitor/komari-agent"
)

// parseVersion 解析可能带有 v/V 前缀，以及预发布或构建元数据的版本字符串
func parseVersion(ver string) (semver.Version, error) {
	ver = strings.TrimPrefix(ver, "v")
	ver = strings.TrimPrefix(ver, "V")
	return semver.ParseTolerant(ver)
}

// needUpdate 判断是否需要更新
func needUpdate(current, latest semver.Version) bool {
	// 返回最新版本大于当前版本时需要更新
	return latest.Compare(current) > 0
}

func DoUpdateWorks() {
	ticker_ := time.NewTicker(time.Duration(6) * time.Hour)
	for range ticker_.C {
		CheckAndUpdate()
	}
}

// 检查更新并执行自动更新
func CheckAndUpdate() error {
	log.Println("Checking update...")
	// Parse current version
	currentSemVer, err := parseVersion(CurrentVersion)
	if err != nil {
		return fmt.Errorf("failed to parse current version: %v", err)
	}

	http.DefaultClient = dnsresolver.GetHTTPClient(60 * time.Second)
	config := selfupdate.Config{}
	updater, err := selfupdate.NewUpdater(config)
	if err != nil {
		return fmt.Errorf("failed to create updater: %v", err)
	}

	// Check for latest version
	latest, err := updater.UpdateSelf(currentSemVer, Repo)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %v", err)
	}

	// Determine if update is needed
	if latest.Version.Equals(currentSemVer) {
		log.Println("Current version is the latest:", CurrentVersion)
		return nil
	}
	// Default is installed as a service, so don't automatically restart
	//execPath, err := os.Executable()
	//if err != nil {
	//	return fmt.Errorf("failed to get current executable path: %v", err)
	//}

	// _, err = os.StartProcess(execPath, os.Args, &os.ProcAttr{
	// 	Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	// })
	// if err != nil {
	// 	return fmt.Errorf("failed to restart program: %v", err)
	// }
	log.Printf("Successfully updated to version %s\n", latest.Version)
	os.Exit(42)
	return nil
}
