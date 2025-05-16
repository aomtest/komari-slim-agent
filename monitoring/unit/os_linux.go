//go:build !windows
// +build !windows

package monitoring

import (
	"bufio"
	"os"
	"strings"
)

func OSName() string {
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
