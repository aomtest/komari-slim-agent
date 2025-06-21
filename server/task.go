package server

import (
	"bytes"
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
func icmpPing(target string, timeout time.Duration) error {
	pinger, err := getPinger(target)
	if err != nil {
		return err
	}
	pinger.Count = 1
	pinger.Timeout = timeout
	pinger.SetPrivileged(true)
	return pinger.Run()
}

func getPinger(target string) (*ping.Pinger, error) {
	return ping.NewPinger(target)
}

func tcpPing(target string, timeout time.Duration) error {
	if !strings.Contains(target, ":") {
		target += ":80"
	}
	conn, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}

func httpPing(target string, timeout time.Duration) (int64, error) {
	client := http.Client{
		Timeout: timeout,
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
		return
	}
	pingResult := 0
	timeout := 3 * time.Second
	switch pingType {
	case "icmp":
		start := time.Now()
		if err := icmpPing(pingTarget, timeout); err == nil {
			pingResult = int(time.Since(start).Milliseconds())
		}
	case "tcp":
		start := time.Now()
		if err := tcpPing(pingTarget, timeout); err == nil {
			pingResult = int(time.Since(start).Milliseconds())
		}
	case "http":
		if latency, err := httpPing(pingTarget, timeout); err == nil {
			pingResult = int(latency)
		}
	default:
		return
	}
	payload := map[string]interface{}{
		"type":        "ping_result",
		"task_id":     taskID,
		"value":       pingResult,
		"finished_at": time.Now(),
	}
	_ = conn.WriteJSON(payload)
}
