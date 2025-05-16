package monitoring

import (
	"encoding/json"
	"fmt"
	"log"

	monitoring "github.com/komari-monitor/komari-agent/monitoring/unit"
)

func GenerateReport() []byte {
	message := ""
	data := map[string]interface{}{}

	cpu := monitoring.Cpu()
	cpuUsage := cpu.CPUUsage
	if cpuUsage <= 0.001 {
		cpuUsage = 0.001
	}
	data["cpu"] = map[string]interface{}{
		"usage": cpuUsage,
	}

	ram := monitoring.Ram()
	data["ram"] = map[string]interface{}{
		"total": ram.Total,
		"used":  ram.Used,
	}

	swap := monitoring.Swap()
	data["swap"] = map[string]interface{}{
		"total": swap.Total,
		"used":  swap.Used,
	}
	load := monitoring.Load()
	data["load"] = map[string]interface{}{
		"load1":  load.Load1,
		"load5":  load.Load5,
		"load15": load.Load15,
	}

	disk := monitoring.Disk()
	data["disk"] = map[string]interface{}{
		"total": disk.Total,
		"used":  disk.Used,
	}

	totalUp, totalDown, networkUp, networkDown, err := monitoring.NetworkSpeed()
	if err != nil {
		message += fmt.Sprintf("failed to get network speed: %v\n", err)
	}
	data["network"] = map[string]interface{}{
		"up":        networkUp,
		"down":      networkDown,
		"totalUp":   totalUp,
		"totalDown": totalDown,
	}

	tcpCount, udpCount, err := monitoring.ConnectionsCount()
	if err != nil {
		message += fmt.Sprintf("failed to get connections: %v\n", err)
	}
	data["connections"] = map[string]interface{}{
		"tcp": tcpCount,
		"udp": udpCount,
	}

	uptime, err := monitoring.Uptime()
	if err != nil {
		message += fmt.Sprintf("failed to get uptime: %v\n", err)
	}
	data["uptime"] = uptime

	processcount := monitoring.ProcessCount()
	data["process"] = processcount

	data["message"] = message

	s, err := json.Marshal(data)
	if err != nil {
		log.Println("Failed to marshal data:", err)
	}
	return s
}
