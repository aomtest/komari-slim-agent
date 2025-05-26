//go:build linux
// +build linux

package monitoring

import (
	"os/exec"
	"strings"
)

func GpuName() string {
	accept := []string{"vga", "nvidia", "amd", "radeon", "render"}
	out, err := exec.Command("lspci").Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			for _, a := range accept {
				if strings.Contains(strings.ToLower(line), a) {
					parts := strings.SplitN(line, ":", 4)
					if len(parts) >= 4 {
						return strings.TrimSpace(parts[3])
					} else if len(parts) == 3 {
						return strings.TrimSpace(parts[2])
					} else if len(parts) == 2 {
						return strings.TrimSpace(parts[1])
					}
				}
			}
		}
	}
	return "None"
}
