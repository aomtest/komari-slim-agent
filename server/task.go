package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	ping "github.com/go-ping/ping"
	"github.com/komari-monitor/komari-agent/cmd/flags"
	"github.com/komari-monitor/komari-agent/ws"
)

func NewTask(task_id, command string) {
	if task_id == "" {
		return
	}
	if command == "" {
		uploadTaskResult(task_id, "No command provided", 0, time.Now())
		return
	}
	if flags.DisableWebSsh {
		uploadTaskResult(task_id, "Web SSH (REC) is disabled.", -1, time.Now())
		return
	}
	log.Printf("Executing task %s with command: %s", task_id, command)
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; "+command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	finishedAt := time.Now()

	result := stdout.String()
	if stderr.Len() > 0 {
		result += "\n" + stderr.String()
	}
	result = strings.ReplaceAll(result, "\r\n", "\n")
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
	}

	uploadTaskResult(task_id, result, exitCode, finishedAt)
}

func uploadTaskResult(taskID, result string, exitCode int, finishedAt time.Time) {
	payload := map[string]interface{}{
		"task_id":     taskID,
		"result":      result,
		"exit_code":   exitCode,
		"finished_at": finishedAt,
	}

	jsonData, _ := json.Marshal(payload)
	endpoint := flags.Endpoint + "/api/clients/task/result?token=" + flags.Token

	resp, _ := http.Post(endpoint, "application/json", bytes.NewBuffer(jsonData))
	maxRetry := flags.MaxRetries
	for i := 0; i < maxRetry && resp.StatusCode != http.StatusOK; i++ {
		log.Printf("Failed to upload task result, retrying %d/%d", i+1, maxRetry)
		time.Sleep(2 * time.Second) // Wait before retrying
		resp, _ = http.Post(endpoint, "application/json", bytes.NewBuffer(jsonData))
	}
	if resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.Printf("Failed to upload task result: %s", resp.Status)
		}
	}
}

// resolveIP 解析域名到 IP 地址，排除 DNS 查询时间
func resolveIP(target string) (string, error) {
	// 如果已经是 IP 地址，直接返回
	if net.ParseIP(target) != nil {
		return target, nil
	}
	// 解析域名到 IP
	addrs, err := net.LookupHost(target)
	if err != nil || len(addrs) == 0 {
		return "", errors.New("failed to resolve target")
	}
	return addrs[0], nil // 返回第一个解析的 IP
}

func icmpPing(target string, timeout time.Duration) (int64, error) {
	// 先解析 IP 地址
	ip, err := resolveIP(target)
	if err != nil {
		return 0, err
	}

	pinger, err := ping.NewPinger(ip)
	if err != nil {
		return 0, err
	}
	pinger.Count = 1
	pinger.Timeout = timeout
	pinger.SetPrivileged(true)
	err = pinger.Run()
	if err != nil {
		return 0, err
	}
	stats := pinger.Statistics()
	if stats.PacketsRecv == 0 {
		return 0, errors.New("no packets received")
	}
	return stats.AvgRtt.Milliseconds(), nil
}

func tcpPing(target string, timeout time.Duration) (int64, error) {
	addr := strings.Split(target, ":")
	ip, err := resolveIP(addr[0])
	if err != nil {
		return 0, err
	}

	port := "80"
	if len(addr) > 1 {
		port = addr[1]
	}
	targetAddr := ip + ":" + port
	start := time.Now()
	conn, err := net.DialTimeout("tcp", targetAddr, timeout)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	return time.Since(start).Milliseconds(), nil
}

func httpPing(target string, timeout time.Duration) (int64, error) {
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "http://" + target
	}

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// 在 Dial 之前解析 IP，排除 DNS 时间
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				ip, err := resolveIP(host)
				if err != nil {
					return nil, err
				}
				return net.DialTimeout(network, ip+":"+port, timeout)
			},
		},
	}
	start := time.Now()
	resp, err := client.Get(target)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return latency, nil
	}
	return latency, errors.New("http status not ok")
}

func NewPingTask(conn *ws.SafeConn, taskID uint, pingType, pingTarget string) {
	if taskID == 0 {
		log.Printf("Invalid task ID: %d", taskID)
		return
	}

	pingResult := 0
	timeout := 3 * time.Second // 默认超时时间
	switch pingType {
	case "icmp":
		if latency, err := icmpPing(pingTarget, timeout); err == nil {
			pingResult = int(latency)
		}
	case "tcp":
		if latency, err := tcpPing(pingTarget, timeout); err == nil {
			pingResult = int(latency)
		}
	case "http":
		if latency, err := httpPing(pingTarget, timeout); err == nil {
			pingResult = int(latency)
		}
	default:
		log.Printf("Unsupported ping type: %s", pingType)
		return
	}

	payload := map[string]interface{}{
		"type":        "ping_result",
		"task_id":     taskID,
		"ping_type":   pingType,
		"value":       pingResult,
		"finished_at": time.Now(),
	}
	if err := conn.WriteJSON(payload); err != nil {
		log.Printf("Failed to write JSON to WebSocket: %v", err)
	}

}
