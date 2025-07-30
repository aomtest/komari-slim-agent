//go:build !windows
// +build !windows

package monitoring

import (
	"bufio"
	"os"
	"strings"
)

func OSName() string {
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
