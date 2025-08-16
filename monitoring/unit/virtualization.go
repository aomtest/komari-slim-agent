package monitoring

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	cpuid "github.com/klauspost/cpuid/v2"
)

func Virtualized() string {
	// Windows: use CPUID to detect hypervisor presence and vendor.
	if runtime.GOOS == "windows" {
		return detectByCPUID()
	}

	// Linux/others: prefer systemd-detect-virt if available; fallback to CPUID.
	if out, err := exec.Command("systemd-detect-virt").Output(); err == nil {
		virt := strings.TrimSpace(string(out))
		if virt != "" {
			return virt
		}
	}

	// Non-systemd environments (e.g., Alpine containers): try container heuristics.
	if ct := detectContainer(); ct != "" {
		return ct
	}

	// Fallback (any OS): CPUID hypervisor bit and vendor mapping.
	return detectByCPUID()
}

// detectByCPUID uses cpuid to check if running under a hypervisor and maps vendor to a common name.
func detectByCPUID() string {
	if !cpuid.CPU.VM() {
		// Align with systemd-detect-virt for bare metal.
		return "none"
	}
	vendor := strings.ToLower(cpuid.CPU.HypervisorVendorString)
	switch {
	case vendor == "kvm" || strings.Contains(vendor, "kvm"):
		return "kvm"
	case vendor == "microsoft" || strings.Contains(vendor, "hyper-v") || strings.Contains(vendor, "msvm") || strings.Contains(vendor, "mshyperv"):
		return "microsoft" // systemd uses "microsoft" for Hyper-V/WSL
	case strings.Contains(vendor, "vmware"):
		return "vmware"
	case strings.Contains(vendor, "xen"):
		return "xen"
	case strings.Contains(vendor, "bhyve"):
		return "bhyve"
	case strings.Contains(vendor, "qemu"):
		return "qemu"
	case strings.Contains(vendor, "parallels"):
		return "parallels"
	case strings.Contains(vendor, "oracle") || strings.Contains(vendor, "virtualbox") || strings.Contains(vendor, "vbox"):
		return "oracle" // systemd reports "oracle" for VirtualBox
	case strings.Contains(vendor, "acrn"):
		return "acrn"
	default:
		if vendor != "" {
			return fmt.Sprintf("hypervisor:%s", vendor)
		}
		return "virtualized"
	}
}

// detectContainer attempts to detect common Linux container environments when systemd isn't available.
// Returns a systemd-detect-virt-like string such as "docker", "podman", "lxc", "container" or empty if not detected.
func detectContainer() string {
	// Quick file markers used by Docker/Podman/CRI-O
	if fileExists("/.dockerenv") {
		return "docker"
	}
	if fileExists("/run/.containerenv") {
		// podman/cri-o often drop this file
		// Try to refine using cgroup content.
		if s := parseCgroupForContainer(); s != "" {
			return s
		}
		return "container"
	}

	// Check cgroup info for container keywords.
	if s := parseCgroupForContainer(); s != "" {
		return s
	}

	// Check mounts for overlay/containers hints.
	if data, err := os.ReadFile("/proc/self/mountinfo"); err == nil {
		lower := strings.ToLower(string(data))
		switch {
		case strings.Contains(lower, "/docker/"):
			return "docker"
		case strings.Contains(lower, "/containers/overlay-containers/") || strings.Contains(lower, "/lib/containers/"):
			return "podman"
		case strings.Contains(lower, "/kubelet/"):
			return "kubernetes"
		}
	}

	return ""
}

func fileExists(p string) bool {
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return true
	}
	return false
}

func parseCgroupForContainer() string {
	// cgroup v1 & v2 paths can contain docker, kubepods, containerd, crio, lxc, podman
	if data, err := os.ReadFile("/proc/self/cgroup"); err == nil {
		lower := strings.ToLower(string(data))
		switch {
		case strings.Contains(lower, "docker"):
			return "docker"
		case strings.Contains(lower, "containerd"):
			return "container"
		case strings.Contains(lower, "kubepods") || strings.Contains(lower, "kubelet"):
			return "kubernetes"
		case strings.Contains(lower, "crio"):
			return "container"
		case strings.Contains(lower, "lxc"):
			return "lxc"
		case strings.Contains(lower, "podman"):
			return "podman"
		}
	}
	return ""
}
