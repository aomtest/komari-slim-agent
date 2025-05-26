package monitoring

import (
	"os/exec"
	"runtime"
	"strings"
)

func Virtualized() string {
	if runtime.GOOS == "windows" {
		return "Unknown"
	}
	out, err := exec.Command("systemd-detect-virt").Output()
	if err != nil {
		return "Unknown"
	}
	virt := strings.TrimSpace(string(out))
	return virt
}
