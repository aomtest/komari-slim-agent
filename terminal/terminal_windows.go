//go:build windows

package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/UserExistsError/conpty"
	"github.com/gorilla/websocket"
)

// StartTerminal 在Windows系统上启动终端
func StartTerminal(conn *websocket.Conn) {
	// 创建进程
	shell, err := exec.LookPath("powershell.exe")
	if err != nil || shell == "" {
		shell = "cmd.exe"
	}
	current_dir := "."
	executable, err := os.Executable()
	if err == nil {
		current_dir = filepath.Dir(executable)
	}
	if shell == "" || current_dir == "" {
		conn.WriteMessage(websocket.TextMessage, []byte("No supported shell found."))
		return
	}

	tty, err := conpty.Start(shell, conpty.ConPtyWorkDir(current_dir))
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: %v\r\n", err)))
		return
	}
	defer tty.Close()
	err_chan := make(chan error, 1)
	// 设置终端大小
	tty.Resize(80, 24)

	go func() {
		for {
			t, p, err := conn.ReadMessage()
			if err != nil {
				err_chan <- err
				return
			}
			if t == websocket.TextMessage {
				var cmd struct {
					Type  string `json:"type"`
					Cols  int    `json:"cols,omitempty"`
					Rows  int    `json:"rows,omitempty"`
					Input string `json:"input,omitempty"`
				}

				if err := json.Unmarshal(p, &cmd); err == nil {
					switch cmd.Type {
					case "resize":
						if cmd.Cols > 0 && cmd.Rows > 0 {
							tty.Resize(cmd.Cols, cmd.Rows)
						}
					case "input":
						if cmd.Input != "" {
							tty.Write([]byte(cmd.Input))
						}
					}
				} else {
					tty.Write(p)
				}
			}
			if t == websocket.BinaryMessage {
				tty.Write(p)
			}
		}
	}()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := tty.Read(buf)
			if err != nil {
				err_chan <- err
				return
			}

			err = conn.WriteMessage(websocket.BinaryMessage, buf[:n])
			if err != nil {
				err_chan <- err
				return
			}
		}
	}()

	go func() {
		err := <-err_chan
		if err != nil && tty != nil {
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: %v\r\n", err)))
		}
		conn.Close()
		tty.Close()
	}()
	tty.Wait(context.Background())
	tty.Close()

}
