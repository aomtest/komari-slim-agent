//go:build !windows

package selfupdate

import (
	"fmt"
	"log"
	"os"
	"syscall"
)

// restartSelf 用 syscall.Exec 原地替换进程映像。
//
// 为什么不用 systemctl restart:agent 以普通用户运行(unit 里是
// User=$service_user),没有权限操作 systemd。而 syscall.Exec 只是让当前
// 进程加载新映像,不需要任何额外权限。
//
// 为什么不用「退出 + 靠 Restart=always 拉起」:那要依赖 unit 里确实配了
// Restart=always(用户可能改过),而 syscall.Exec 不依赖任何外部配置。
// 代价是 PID 不变,但 agent 不依赖 PID 做任何判断。
func restartSelf(exe string) error {
	log.Printf("[selfupdate] restarting in place with %s", exe)
	// syscall.Exec 成功时不会返回 —— 进程映像已经被替换掉了。
	err := syscall.Exec(exe, os.Args, os.Environ())
	// 走到这里说明失败了。
	return fmt.Errorf("cannot restart in place: %w", err)
}
