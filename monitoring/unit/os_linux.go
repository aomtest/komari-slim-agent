//go:build !windows
// +build !windows

package monitoring

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
)

func OSName() string {
	if pveVersion := detectProxmoxVE(); pveVersion != "" {
		return pveVersion
	}
	
	if synologyName := detectSynology(); synologyName != "" {
		return synologyName
	}
	
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return "Linux"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(line[len("PRETTY_NAME="):], `"`)
		}
	}

	if err := scanner.Err(); err != nil {
		return "Linux"
	}

	return "Linux"
}

func detectSynology() string {
	synologyFiles := []string{
		"/etc/synoinfo.conf",
		"/etc.defaults/synoinfo.conf",
	}
	
	for _, file := range synologyFiles {
		if info, err := os.Stat(file); err == nil && !info.IsDir() {
			if synologyInfo := readSynologyInfo(file); synologyInfo != "" {
				return synologyInfo
			}
		}
	}
	
	if info, err := os.Stat("/usr/syno"); err == nil && info.IsDir() {
		return "Synology DSM"
	}
	
	return ""
}

func readSynologyInfo(filename string) string {
	file, err := os.Open(filename)
	if err != nil {
		return ""
	}
	defer file.Close()
	
	var unique, udcCheckState string
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if strings.HasPrefix(line, "unique=") {
			unique = strings.Trim(strings.TrimPrefix(line, "unique="), `"`)
		} else if strings.HasPrefix(line, "udc_check_state=") {
			udcCheckState = strings.Trim(strings.TrimPrefix(line, "udc_check_state="), `"`)
		}
	}
	
	if unique != "" && strings.Contains(unique, "synology_") {
		parts := strings.Split(unique, "_")
		if len(parts) >= 3 {
			model := strings.ToUpper(parts[len(parts)-1])
			
			result := "Synology " + model
			
			if udcCheckState != "" {
				result += " DSM " + udcCheckState
			} else {
				result += " DSM"
			}
			
			return result
		}
	}
	
	return ""
}

func detectProxmoxVE() string {
	if _, err := exec.LookPath("pveversion"); err != nil {
		return ""
	}
	
	out, err := exec.Command("pveversion").Output()
	if err != nil {
		return ""
	}
	
	output := strings.TrimSpace(string(out))
	lines := strings.Split(output, "\n")
	
	var version string
	var codename string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "pve-manager/") {
			parts := strings.Split(line, "/")
			if len(parts) >= 2 {
				versionPart := parts[1]
				if idx := strings.Index(versionPart, "~"); idx != -1 {
					versionPart = versionPart[:idx]
				}
				version = versionPart
			}
		}
		
	}
	
	if version != "" {
		if file, err := os.Open("/etc/os-release"); err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "VERSION_CODENAME=") {
					codename = strings.Trim(line[len("VERSION_CODENAME="):], `"`)
					break
				}
			}
		}
	}
	
	if version != "" {
		if codename != "" {
			return "Proxmox VE " + version + " (" + codename + ")"
		}
		return "Proxmox VE " + version
	}
	
	return "Proxmox VE"
}
