package server

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/komari-monitor/komari-agent/cmd/flags"
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
