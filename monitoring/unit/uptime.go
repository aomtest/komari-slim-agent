package monitoring

import (
	"github.com/shirou/gopsutil/host"
)

func Uptime() (uint64, error) {

	return host.Uptime()

}
