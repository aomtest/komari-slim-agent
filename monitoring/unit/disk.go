package monitoring

import (
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
)

type DiskInfo struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

func Disk() DiskInfo {
	diskinfo := DiskInfo{}
	usage, err := disk.Partitions(false) // 使用 false 只获取物理分区
	if err != nil {
		diskinfo.Total = 0
		diskinfo.Used = 0
	} else {
		for _, part := range usage {
			// 排除临时文件系统和网络驱动器
			if isPhysicalDisk(part) {
				u, err := disk.Usage(part.Mountpoint)
				if err != nil {
					continue
				} else {
					diskinfo.Total += u.Total
					diskinfo.Used += u.Used
				}
			}
		}
	}
	return diskinfo
}

// isPhysicalDisk 判断分区是否为物理磁盘
func isPhysicalDisk(part disk.PartitionStat) bool {
	// 对于LXC等基于loop的根文件系统，始终包含根挂载点
	if part.Mountpoint == "/" {
		return true
	}
	mountpoint := strings.ToLower(part.Mountpoint)
	// 临时文件系统
	if mountpoint == "/tmp" || mountpoint == "/var/tmp" || mountpoint == "/dev/shm" ||
		mountpoint == "/run" || mountpoint == "/run/lock" {
		return false
	}

	fstype := strings.ToLower(part.Fstype)
	// 网络驱动器
	if strings.HasPrefix(fstype, "nfs") || strings.HasPrefix(fstype, "cifs") ||
		strings.HasPrefix(fstype, "smb") || fstype == "vboxsf" || fstype == "9p" ||
		strings.Contains(fstype, "fuse") {
		return false
	} // Windows 网络驱动器通常是映射盘符，但不容易通过fstype判断
	// 可以通过opts判断，Windows网络驱动通常有相关选项
	optsStr := strings.ToLower(strings.Join(part.Opts, ","))
	if strings.Contains(optsStr, "remote") || strings.Contains(optsStr, "network") {
		return false
	}

	// Docker overlay
	if fstype == "overlay" {
		return false
	}

	// 虚拟内存
	if strings.HasPrefix(part.Device, "/dev/loop") || fstype == "devtmpfs" || fstype == "tmpfs" {
		return false
	}

	return true
}
