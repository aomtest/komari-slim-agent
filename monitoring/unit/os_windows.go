//go:build windows
// +build windows

package monitoring

import (
	"strconv"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func OSName() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return "Microsoft Windows"
	}
	defer key.Close()

	productName, _, err := key.GetStringValue("ProductName")
	if err != nil {
		return "Microsoft Windows"
	}

	// Windows 11
	majorVersion, _, err := key.GetIntegerValue("CurrentMajorVersionNumber")
	if err == nil && majorVersion >= 10 {
		buildNumberStr, _, err := key.GetStringValue("CurrentBuild")
		if err == nil {
			buildNumber, err := strconv.Atoi(buildNumberStr)
			if err == nil && buildNumber >= 22000 { // Windows 11 starts at build 22000
				// Windows 11 Windows 10 Pro for Workstations
				edition := strings.Replace(productName, "Windows 10 ", "", 1)
				return "Windows 11 " + edition
			}
		}
		// DisplayVersion
		displayVersion, _, err := key.GetStringValue("DisplayVersion")
		if err == nil && displayVersion >= "21H2" {
			edition := strings.Replace(productName, "Windows 10 ", "", 1)
			return "Windows 11 " + edition
		}
	}

	return productName
}
