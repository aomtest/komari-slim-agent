package update

import (
	"fmt"
	"log"
	"os"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
)

type Updater struct {
	CurrentVersion string // 当前版本
	Repo           string // GitHub 仓库，例如 "komari-monitor/komari-agent"
}

func NewUpdater(currentVersion, repo string) *Updater {
	return &Updater{
		CurrentVersion: currentVersion,
		Repo:           repo,
	}
}

// 检查更新并执行自动更新
func (u *Updater) CheckAndUpdate() error {
	log.Println("Checking update...")
	// 解析当前版本
	currentSemVer, err := semver.Parse(u.CurrentVersion)
	if err != nil {
		return fmt.Errorf("解析当前版本失败: %v", err)
	}

	// 创建 selfupdate 配置
	config := selfupdate.Config{}
	updater, err := selfupdate.NewUpdater(config)
	if err != nil {
		return fmt.Errorf("创建 updater 失败: %v", err)
	}

	// 检查最新版本
	latest, err := updater.UpdateSelf(currentSemVer, u.Repo)
	if err != nil {
		return fmt.Errorf("检查更新失败: %v", err)
	}

	// 判断是否需要更新
	if latest.Version.Equals(currentSemVer) {
		fmt.Println("当前版本已是最新版本:", u.CurrentVersion)
		return nil
	}
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取当前执行路径失败: %v", err)
	}

	_, err = os.StartProcess(execPath, os.Args, &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		return fmt.Errorf("重新启动程序失败: %v", err)
	}
	fmt.Printf("成功更新到版本 %s\n", latest.Version)
	fmt.Printf("发布说明:\n%s\n", latest.ReleaseNotes)
	os.Exit(0)
	return nil
}
