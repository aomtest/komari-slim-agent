package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/komari-monitor/komari-agent/cmd/flags"
	monitoring "github.com/komari-monitor/komari-agent/monitoring/unit"
	"github.com/komari-monitor/komari-agent/update"
)

func DoUploadBasicInfoWorks() {
	ticker := time.NewTicker(time.Duration(flags.InfoReportInterval) * time.Minute)
	for range ticker.C {
		err := uploadBasicInfo()
		if err != nil {
			log.Println("Error uploading basic info:", err)
		}
	}
}
func UpdateBasicInfo() {
	err := uploadBasicInfo()
	if err != nil {
		log.Println("Error uploading basic info:", err)
	} else {
		log.Println("Basic info uploaded successfully")
	}
}
func uploadBasicInfo() error {
	cpu := monitoring.Cpu()

	osname := monitoring.OSName()
	ipv4, ipv6, _ := monitoring.GetIPAddress()

	data := map[string]interface{}{
		"cpu_name":       cpu.CPUName,
		"cpu_cores":      cpu.CPUCores,
		"arch":           cpu.CPUArchitecture,
		"os":             osname,
		"ipv4":           ipv4,
		"ipv6":           ipv6,
		"mem_total":      monitoring.Ram().Total,
		"swap_total":     monitoring.Swap().Total,
		"disk_total":     monitoring.Disk().Total,
		"gpu_name":       monitoring.GpuName(),
		"virtualization": monitoring.Virtualized(),
		"version":        update.CurrentVersion,
	}

	endpoint := strings.TrimSuffix(flags.Endpoint, "/") + "/api/clients/uploadBasicInfo?token=" + flags.Token
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	message := string(body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code: %d,%s", resp.StatusCode, message)
	}

	return nil
}
