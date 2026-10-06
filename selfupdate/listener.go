// Package selfupdate 实现 agent 的自更新。
//
// 设计原则是「能力最小化」:触发通道只做一件事 —— 比对令牌,相等就更新。
// 它不接受参数、不执行任何来自网络的内容,更新内容也固定来自 GitHub 官方
// release。所以即使这个端口被暴露或被扫到,攻击者能做的上限也只是
// 「让 agent 更新到最新版」,而不是「让 agent 执行任意代码」。
//
// 与上游的区别:上游 komari-agent 有一个会替换自身二进制的 update 包,
// 本分支此前把它整体移除了(见 version/version.go 的注释)。这里按
// 「被动触发 + 只更新自己」的方式重新实现,不引入任何远程执行能力。
package selfupdate

import (
	"bufio"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	// 单次连接读取的上限。令牌很短,给足余量即可 —— 没有上限的话,
	// 一个持续发送数据的连接就能把内存吃掉。
	maxTriggerBytes = 512

	// 读取超时。防止连接建立后不发数据,把 goroutine 一直占着。
	readTimeout = 10 * time.Second

	// 两次触发之间的最小间隔。
	//
	// 这主要不是防重放 —— 重放已经被「先查版本、已是最新就跳过」挡住了。
	// 它防的是另一种情况:更新失败时(比如下载中断),版本仍然是旧的,
	// 此时重放会导致「反复尝试更新、反复失败、反复下载」。
	minTriggerInterval = 5 * time.Minute
)

// Trigger 是一个已通过校验的更新请求。
type Trigger struct {
	// 触发来源,仅用于日志。
	From string
	// At 是触发时间。
	At time.Time
}

// Listener 监听触发端口。
type Listener struct {
	addr  string
	token string

	mu          sync.Mutex
	running     bool          // 是否已有更新在执行
	lastAttempt time.Time     // 上次尝试触发的时间

	// handler 在收到合法触发时被调用。放在这里而不是写死,
	// 是为了让监听逻辑可以单独测试。
	handler func(Trigger)
}

// NewListener 创建一个监听器。addr 为空时返回 nil,表示该功能未启用。
func NewListener(addr, token string, handler func(Trigger)) *Listener {
	if strings.TrimSpace(addr) == "" {
		return nil
	}
	return &Listener{addr: addr, token: token, handler: handler}
}

// Run 启动监听,阻塞直到 listener 关闭。应当在 goroutine 中调用。
//
// 令牌为空时直接拒绝启动:一个不校验任何内容的触发端口等于给所有人
// 开放了「反复重启 agent」的开关。
func (l *Listener) Run() {
	if l == nil {
		return
	}
	if strings.TrimSpace(l.token) == "" {
		log.Printf("[selfupdate] refusing to start: no trigger token configured")
		return
	}

	ln, err := net.Listen("tcp", l.addr)
	if err != nil {
		log.Printf("[selfupdate] cannot listen on %s: %v", l.addr, err)
		return
	}
	log.Printf("[selfupdate] trigger listener started on %s", l.addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("[selfupdate] accept failed: %v", err)
			return
		}
		go l.handle(conn)
	}
}

// handle 处理单个连接。
//
// 无论成功与否都立即关闭连接,并且**不回任何响应** —— 不回显、不报错、
// 不区分「令牌错误」和「格式错误」。这样端口对外看起来就是个收到数据就
// 断开的黑洞,不给扫描器任何可用于识别服务的特征。
func (l *Listener) handle(conn net.Conn) {
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(readTimeout))

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return
	}
	// 超长输入直接丢弃,不做任何比较。
	if len(line) > maxTriggerBytes {
		return
	}

	// 常量时间比较,避免通过响应时间差推测令牌。
	if !constantTimeEqual(strings.TrimSpace(line), l.token) {
		return
	}

	from := conn.RemoteAddr().String()

	// 已有更新在执行 —— 忽略,防止并发替换同一个二进制。
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return
	}
	// 距上次尝试太近 —— 忽略。主要防「更新失败后反复重试」。
	if !l.lastAttempt.IsZero() && time.Since(l.lastAttempt) < minTriggerInterval {
		l.mu.Unlock()
		return
	}
	l.lastAttempt = time.Now()
	l.running = true
	l.mu.Unlock()

	go func() {
		defer func() {
			l.mu.Lock()
			l.running = false
			l.mu.Unlock()
		}()
		if l.handler != nil {
			l.handler(Trigger{From: from, At: time.Now()})
		}
	}()
}

// constantTimeEqual 逐字节比较,且总是走完全程。
//
// 不用 bytes.Equal:它会在第一个不同的字节处提前返回,理论上可以通过
// 响应时间差逐字节推测令牌。这里比较的是固定长度的小字符串,自己写更省事。
func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
