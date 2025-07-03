package monitoring

import (
	"fmt"
	"strings"
	"time"

	"github.com/komari-monitor/komari-agent/cmd/flags"
	"github.com/shirou/gopsutil/net"
)

func ConnectionsCount() (tcpCount, udpCount int, err error) {
	tcps, err := net.Connections("tcp")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get TCP connections: %w", err)
	}
	udps, err := net.Connections("udp")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get UDP connections: %w", err)
	}

	return len(tcps), len(udps), nil
}

var (
	// 预定义常见的回环和虚拟接口名称
	loopbackNames = map[string]struct{}{
		"lo": {}, "lo0": {}, "localhost": {},
		"brd0": {}, "docker0": {}, "docker1": {},
		"veth0": {}, "veth1": {}, "veth2": {}, "veth3": {},
		"veth4": {}, "veth5": {}, "veth6": {}, "veth7": {},
	}
)

func NetworkSpeed() (totalUp, totalDown, upSpeed, downSpeed uint64, err error) {
	includeNics := parseNics(flags.IncludeNics)
	excludeNics := parseNics(flags.ExcludeNics)

	// 获取第一次网络IO计数器
	ioCounters1, err := net.IOCounters(true)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to get network IO counters: %w", err)
	}

	if len(ioCounters1) == 0 {
		return 0, 0, 0, 0, fmt.Errorf("no network interfaces found")
	}

	// 统计第一次所有非回环接口的流量
	var totalUp1, totalDown1 uint64
	for _, interfaceStats := range ioCounters1 {
		if shouldInclude(interfaceStats.Name, includeNics, excludeNics) {
			totalUp1 += interfaceStats.BytesSent
			totalDown1 += interfaceStats.BytesRecv
		}
	}

	// 等待1秒
	time.Sleep(time.Second)

	// 获取第二次网络IO计数器
	ioCounters2, err := net.IOCounters(true)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to get network IO counters: %w", err)
	}

	if len(ioCounters2) == 0 {
		return 0, 0, 0, 0, fmt.Errorf("no network interfaces found")
	}

	// 统计第二次所有非回环接口的流量
	var totalUp2, totalDown2 uint64
	for _, interfaceStats := range ioCounters2 {
		if shouldInclude(interfaceStats.Name, includeNics, excludeNics) {
			totalUp2 += interfaceStats.BytesSent
			totalDown2 += interfaceStats.BytesRecv
		}
	}

	// 计算速度 (每秒的速率)
	upSpeed = totalUp2 - totalUp1
	downSpeed = totalDown2 - totalDown1

	return totalUp2, totalDown2, upSpeed, downSpeed, nil
}

func parseNics(nics string) map[string]struct{} {
	if nics == "" {
		return nil
	}
	nicSet := make(map[string]struct{})
	for _, nic := range strings.Split(nics, ",") {
		nicSet[strings.TrimSpace(nic)] = struct{}{}
	}
	return nicSet
}

func shouldInclude(nicName string, includeNics, excludeNics map[string]struct{}) bool {
	// 默认排除回环接口
	if _, isLoopback := loopbackNames[nicName]; isLoopback {
		return false
	}

	// 如果定义了白名单，则只包括白名单中的接口
	if len(includeNics) > 0 {
		_, ok := includeNics[nicName]
		return ok
	}

	// 如果定义了黑名单，则排除黑名单中的接口
	if len(excludeNics) > 0 {
		if _, ok := excludeNics[nicName]; ok {
			return false
		}
	}

	return true
}
