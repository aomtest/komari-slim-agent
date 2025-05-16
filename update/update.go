package update

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
)

var (
	CurrentVersion string = "0.0.1"
	Repo           string = "komari-monitor/komari-agent"
)

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
	currentSemVer, err := semver.Parse(CurrentVersion)
	if err != nil {
		return fmt.Errorf("failed to parse current version: %v", err)
	}

	// Create selfupdate configuration
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
		fmt.Println("Current version is the latest:", CurrentVersion)
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
	fmt.Printf("Successfully updated to version %s\n", latest.Version)
	os.Exit(42)
	return nil
}
