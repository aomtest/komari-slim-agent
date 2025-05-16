package monitoring

import (
	"github.com/shirou/gopsutil/disk"
)

type DiskInfo struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

func Disk() DiskInfo {
	diskinfo := DiskInfo{}
	usage, err := disk.Partitions(true)
	if err != nil {
		diskinfo.Total = 0
		diskinfo.Used = 0
	} else {
		for _, part := range usage {
			if part.Mountpoint != "/tmp" && part.Mountpoint != "/var/tmp" && part.Mountpoint != "/dev/shm" {
				// Skip /tmp, /var/tmp, and /dev/shm
				// 获取磁盘使用情况
				u, err := disk.Usage(part.Mountpoint)
				if err != nil {
					diskinfo.Total = 0
					diskinfo.Used = 0
				} else {
					diskinfo.Total += u.Total
					diskinfo.Used += u.Used
				}
			}
		}
	}
	return diskinfo
}
