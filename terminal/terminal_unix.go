//go:build !windows

package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

func newTerminalImpl() (*terminalImpl, error) {
	// 查找可用 shell
	defaultShells := []string{"zsh", "bash", "sh"}
	shell := ""
	for _, s := range defaultShells {
		if _, err := exec.LookPath(s); err == nil {
			shell = s
			break
		}
	}
	if shell == "" {
		return nil, fmt.Errorf("no supported shell found")
	}

	// 创建进程
	cmd := exec.Command(shell)
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	tty, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start pty: %v", err)
	}

	// 设置初始终端大小
	pty.Setsize(tty, &pty.Winsize{Rows: 24, Cols: 80})

	return &terminalImpl{
		shell: shell,
		term: &unixTerminal{
			tty: tty,
			cmd: cmd,
		},
	}, nil
}

type unixTerminal struct {
	tty *os.File
	cmd *exec.Cmd
}

func (t *unixTerminal) Close() error {
	pgid, err := syscall.Getpgid(t.cmd.Process.Pid)
	if err != nil {
		return t.cmd.Process.Kill()
	}
	return syscall.Kill(-pgid, syscall.SIGKILL)
}

func (t *unixTerminal) Read(p []byte) (int, error) {
	return t.tty.Read(p)
}

func (t *unixTerminal) Write(p []byte) (int, error) {
	return t.tty.Write(p)
}

func (t *unixTerminal) Resize(cols, rows int) error {
	return pty.Setsize(t.tty, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

func (t *unixTerminal) Wait() error {
	return t.cmd.Wait()
}
