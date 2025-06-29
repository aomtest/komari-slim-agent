//go:build darwin
// +build darwin

package monitoring

import (
	"os/exec"
	"strings"
)

// OSName returns the name of the operating system on Darwin (macOS)
func OSName() string {
	cmd := exec.Command("sw_vers", "-productName")
	output, err := cmd.Output()
	if err != nil {
		return "macOS"
	}

	name := strings.TrimSpace(string(output))
	return name
}
