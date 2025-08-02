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
	// 如果是 Server 版本，直接返回原始名称
	if strings.Contains(productName, "Server") {
		return productName
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

// KernelVersion returns the kernel version on Windows systems (build number)
func KernelVersion() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return "Unknown"
	}
	defer key.Close()

	// Get current build number
	buildNumber, _, err := key.GetStringValue("CurrentBuild")
	if err != nil {
		return "Unknown"
	}

	// Get UBR (Update Build Revision) if available
	ubr, _, err := key.GetIntegerValue("UBR")
	if err != nil {
		// UBR not available, just return build number
		return buildNumber
	}

	return buildNumber + "." + strconv.FormatUint(ubr, 10)
}
