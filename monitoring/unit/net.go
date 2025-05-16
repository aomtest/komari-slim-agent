package monitoring

import (
	"fmt"
	"time"

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
	// 获取第一次网络IO计数器
	ioCounters1, err := net.IOCounters(false)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to get network IO counters: %w", err)
	}

	if len(ioCounters1) == 0 {
		return 0, 0, 0, 0, fmt.Errorf("no network interfaces found")
	}

	// 统计第一次所有非回环接口的流量
	var totalUp1, totalDown1 uint64
	for _, interfaceStats := range ioCounters1 {
		// 使用映射表进行O(1)查找
		if _, isLoopback := loopbackNames[interfaceStats.Name]; isLoopback {
			continue // 跳过回环接口
		}
		totalUp1 += interfaceStats.BytesSent
		totalDown1 += interfaceStats.BytesRecv
	}

	// 等待1秒
	time.Sleep(time.Second)

	// 获取第二次网络IO计数器
	ioCounters2, err := net.IOCounters(false)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to get network IO counters: %w", err)
	}

	if len(ioCounters2) == 0 {
		return 0, 0, 0, 0, fmt.Errorf("no network interfaces found")
	}

	// 统计第二次所有非回环接口的流量
	var totalUp2, totalDown2 uint64
	for _, interfaceStats := range ioCounters2 {
		if _, isLoopback := loopbackNames[interfaceStats.Name]; isLoopback {
			continue // 跳过回环接口
		}
		totalUp2 += interfaceStats.BytesSent
		totalDown2 += interfaceStats.BytesRecv
	}

	// 计算速度 (每秒的速率)
	upSpeed = totalUp2 - totalUp1
	downSpeed = totalDown2 - totalDown1

	return totalUp2, totalDown2, upSpeed, downSpeed, nil
}
