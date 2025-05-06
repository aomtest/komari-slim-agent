package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari-agent/config"
	"github.com/komari-monitor/komari-agent/monitoring"
	"github.com/komari-monitor/komari-agent/update"
)

var (
	CurrentVersion string = "0.0.1"
	repo                  = "komari-monitor/komari-agent"
)

func main() {
	log.Printf("Komari Agent %s\n", CurrentVersion)
	localConfig, err := config.LoadConfig()
	if err != nil {
		log.Fatalln("Failed to load local config:", err)
	}
	if localConfig.IgnoreUnsafeCert {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	go func() {
		err = uploadBasicInfo(localConfig.Endpoint, localConfig.Token)
		ticker := time.NewTicker(time.Duration(time.Minute * 15))
		for range ticker.C {
			err = uploadBasicInfo(localConfig.Endpoint, localConfig.Token)
			if err != nil {
				log.Fatalln("Failed to upload basic info:", err)
			}
		}
	}()

	websocketEndpoint := strings.TrimSuffix(localConfig.Endpoint, "/") + "/api/clients/report?token=" + localConfig.Token
	websocketEndpoint = "ws" + strings.TrimPrefix(websocketEndpoint, "http")

	var conn *websocket.Conn
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()

	ticker := time.NewTicker(time.Duration(localConfig.Interval * float64(time.Second)))
	defer ticker.Stop()

	if localConfig.AutoUpdate {
		go func() {
			ticker_ := time.NewTicker(time.Duration(6) * time.Hour)
			update_komari()
			for range ticker_.C {
				update_komari()
			}
		}()
	}

	for range ticker.C {
		// If no connection, attempt to connect
		if conn == nil {
			log.Println("Attempting to connect to WebSocket...")
			retry := 0
			for retry < localConfig.MaxRetries {
				conn, err = connectWebSocket(websocketEndpoint)
				if err == nil {
					log.Println("WebSocket connected")
					go handleWebSocketMessages(localConfig, conn, make(chan struct{}))
					break
				}
				retry++
				time.Sleep(time.Duration(localConfig.ReconnectInterval) * time.Second)
			}

			if retry >= localConfig.MaxRetries {
				log.Println("Max retries reached, falling back to POST")
				// Send report via POST and continue
				data := report(localConfig)
				if err := reportWithPOST(localConfig.Endpoint, data); err != nil {
					log.Println("Failed to send POST report:", err)
				}
				continue
			}
		}

		// Send report via WebSocket
		data := report(localConfig)
		err = conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			log.Println("Failed to send WebSocket message:", err)
			conn.Close()
			conn = nil // Mark connection as dead
			continue
		}

	}
}

func update_komari() {
	// 初始化 Updater
	updater := update.NewUpdater(CurrentVersion, repo)

	// 检查并更新
	err := updater.CheckAndUpdate()
	if err != nil {
		log.Println("Update Failed: %v", err)
	}
}

// connectWebSocket attempts to establish a WebSocket connection and upload basic info
func connectWebSocket(websocketEndpoint string) (*websocket.Conn, error) {
	dialer := &websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}
	conn, _, err := dialer.Dial(websocketEndpoint, nil)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func handleWebSocketMessages(localConfig config.LocalConfig, conn *websocket.Conn, done chan<- struct{}) {
	defer close(done)
	for {
		_, message_raw, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			return
		}
		// TODO: Remote config update
		// TODO: Handle incoming messages
		log.Println("Received message:", string(message_raw))
		message := make(map[string]interface{})
		err = json.Unmarshal(message_raw, &message)
		if err != nil {
			log.Println("Bad ws message:", err)
			continue
		}

	}
}

func reportWithPOST(endpoint string, data []byte) error {
	url := strings.TrimSuffix(endpoint, "/") + "/api/clients/report"
	req, err := http.NewRequest("POST", url, strings.NewReader(string(data)))
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

	if resp.StatusCode != http.StatusOK {
		return err
	}
	return nil
}

func uploadBasicInfo(endpoint string, token string) error {
	log.Println("Uploading basic info...")
	defer log.Println("Upload complete")
	cpu := monitoring.Cpu()

	osname := monitoring.OSName()
	ipv4, ipv6, _ := monitoring.GetIPAddress()

	data := map[string]interface{}{
		"cpu_name":   cpu.CPUName,
		"cpu_cores":  cpu.CPUCores,
		"arch":       cpu.CPUArchitecture,
		"os":         osname,
		"ipv4":       ipv4,
		"ipv6":       ipv6,
		"mem_total":  monitoring.Ram().Total,
		"swap_total": monitoring.Swap().Total,
		"disk_total": monitoring.Disk().Total,
		"gpu_name":   "Unknown",
		"version":    CurrentVersion,
	}

	endpoint = strings.TrimSuffix(endpoint, "/") + "/api/clients/uploadBasicInfo?token=" + token
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

func report(localConfig config.LocalConfig) []byte {
	message := ""
	data := map[string]interface{}{}

	cpu := monitoring.Cpu()
	data["cpu"] = map[string]interface{}{
		"usage": cpu.CPUUsage,
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

	totalUp, totalDown, networkUp, networkDown, err := monitoring.NetworkSpeed(int(localConfig.Interval))
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
