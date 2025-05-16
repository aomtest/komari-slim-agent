//go:build !windows

package terminal

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

// StartTerminal 在Unix/Linux系统上启动终端
func StartTerminal(conn *websocket.Conn) {
	// 获取shell
	defalut_shell := []string{"zsh", "bash", "sh"}
	shell := ""
	for _, s := range defalut_shell {
		if _, err := exec.LookPath(s); err == nil {
			shell = s
			break
		}
	}
	if shell == "" {
		conn.WriteMessage(websocket.TextMessage, []byte("No supported shell found."))
		return
	}
	// 创建进程
	cmd := exec.Command(shell)
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	tty, err := pty.Start(cmd)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: %v\r\n", err)))
		return
	}
	defer tty.Close()
	// 设置终端大小
	pty.Setsize(tty, &pty.Winsize{
		Rows: 24,
		Cols: 80,
		X:    0,
		Y:    0,
	})
	terminateConn := func() {
		pgid, err := syscall.Getpgid(cmd.Process.Pid)
		if err != nil {
			cmd.Process.Kill()
		}
		syscall.Kill(-pgid, syscall.SIGKILL)
		if conn != nil {
			conn.Close()
		}
	}
	err_chan := make(chan error, 1)
	// 从WebSocket读取数据并写入pty
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
							pty.Setsize(tty, &pty.Winsize{
								Rows: uint16(cmd.Rows),
								Cols: uint16(cmd.Cols),
							})
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

	err = <-err_chan
	if err != nil && conn != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: %v\r\n", err)))
	}
	terminateConn()
}
