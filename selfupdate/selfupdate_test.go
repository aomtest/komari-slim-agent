package selfupdate

import (
	"net"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in   string
		want []int
	}{
		{"v1.0.3", []int{1, 0, 3}},
		{"1.0.3", []int{1, 0, 3}},
		{"V2.3.18", []int{2, 3, 18}},
		{"v1.0.2-rc1", []int{1, 0, 2}},
		{"", nil},
		{"unknown", nil},
		{"v1.x", nil},
	}
	for _, c := range cases {
		got := parseVersion(c.in)
		if len(got) != len(c.want) {
			t.Errorf("parseVersion(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("parseVersion(%q) = %v, want %v", c.in, got, c.want)
				break
			}
		}
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v1.0.2", "v1.0.3", true},
		{"v1.0.3", "v1.0.3", false},
		{"v1.0.4", "v1.0.3", false},
		{"1.0.2", "v1.0.3", true}, // 容忍 v 前缀不一致
		{"v1.0.2", "v1.0.10", true}, // 不能按字符串比较
		{"v1.0.10", "v1.0.2", false},
		{"unknown", "v1.0.3", true}, // 构建时未注入版本号,按"可能有更新"处理
		{"v1.0.2", "unknown", false}, // latest 解析不了就不提示
	}
	for _, c := range cases {
		if got := isNewer(c.current, c.latest); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestCurrentAssetName(t *testing.T) {
	name, err := currentAssetName()
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		if err == nil {
			t.Fatalf("expected an error on %s", runtime.GOOS)
		}
		return
	}
	if err != nil {
		t.Fatalf("currentAssetName() error = %v", err)
	}
	// 命名必须与 release workflow 产出的资产名一致。
	switch runtime.GOOS {
	case "linux":
		if want := "komari-agent-linux-" + runtime.GOARCH; name != want {
			t.Errorf("got %q, want %q", name, want)
		}
	case "windows":
		if want := "komari-agent-windows-" + runtime.GOARCH + ".exe"; name != want {
			t.Errorf("got %q, want %q", name, want)
		}
	}
}

func TestConstantTimeEqual(t *testing.T) {
	if !constantTimeEqual("abc", "abc") {
		t.Error("identical strings must compare equal")
	}
	if constantTimeEqual("abc", "abd") {
		t.Error("different strings must not compare equal")
	}
	if constantTimeEqual("abc", "ab") {
		t.Error("different lengths must not compare equal")
	}
	if !constantTimeEqual("", "") {
		t.Error("two empty strings must compare equal")
	}
}

func TestPickAssetRejectsMissingDigest(t *testing.T) {
	rel := &release{TagName: "v9.9.9"}
	rel.Assets = append(rel.Assets, asset{
		ID: 1, Name: mustAssetName(t), Size: 100, Digest: "",
	})
	if _, err := pickAsset(rel); err == nil {
		t.Fatal("an asset without a digest must be rejected")
	}
}

func TestPickAssetAcceptsValid(t *testing.T) {
	rel := &release{TagName: "v9.9.9"}
	rel.Assets = append(rel.Assets, asset{
		ID: 7, Name: mustAssetName(t), Size: 123, Digest: "sha256:deadbeef",
	})
	a, err := pickAsset(rel)
	if err != nil {
		t.Fatalf("pickAsset() error = %v", err)
	}
	if a.ID != 7 || a.Size != 123 {
		t.Errorf("unexpected asset: %+v", a)
	}
}

// NewListener 在地址为空时必须返回 nil —— 这是"默认关闭"的实现基础。
func TestNewListenerDisabledWhenAddressEmpty(t *testing.T) {
	if l := NewListener("", "token", nil); l != nil {
		t.Fatal("an empty listen address must disable the feature")
	}
	if l := NewListener("   ", "token", nil); l != nil {
		t.Fatal("a blank listen address must disable the feature")
	}
}

// 令牌为空时不应启动监听:那等于给所有人开放"反复重启 agent"的开关。
func TestRunRefusesWithoutToken(t *testing.T) {
	l := NewListener("127.0.0.1:0", "", nil)
	if l == nil {
		t.Fatal("listener should be created")
	}
	done := make(chan struct{})
	go func() {
		l.Run() // 应当立即返回,不进入 Accept 循环
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run() must return immediately when no token is configured")
	}
}

// 端到端:只有内容完全匹配令牌才会触发,其他一切输入都静默丢弃。
func TestListenerTriggersOnlyOnExactToken(t *testing.T) {
	const token = "s3cret-token"

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot create a test listener: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	var mu sync.Mutex
	var fired int

	l := NewListener(addr, token, func(Trigger) {
		mu.Lock()
		fired++
		mu.Unlock()
	})

	// 直接驱动 handle,避免 Run() 里 Accept 循环带来的不确定性。
	real, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("cannot listen on %s: %v", addr, err)
	}
	defer real.Close()

	go func() {
		for {
			conn, err := real.Accept()
			if err != nil {
				return
			}
			go l.handle(conn)
		}
	}()

	send := func(payload string) {
		conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
		if err != nil {
			t.Fatalf("dial failed: %v", err)
		}
		conn.Write([]byte(payload))
		conn.Close()
		// 给 handle 一点时间完成判定
		time.Sleep(150 * time.Millisecond)
	}

	send("wrong-token\n")
	send(token + "-suffix\n") // 前缀相同但不等
	send("\n")
	send(token + "\n") // 唯一应当触发的一次

	mu.Lock()
	got := fired
	mu.Unlock()
	if got != 1 {
		t.Fatalf("expected exactly 1 trigger, got %d", got)
	}

	// 触发之后立刻再发一次,应被频率限制挡住。
	send(token + "\n")
	mu.Lock()
	got = fired
	mu.Unlock()
	if got != 1 {
		t.Fatalf("a second trigger within the cooldown must be ignored, got %d", got)
	}
}

func mustAssetName(t *testing.T) string {
	t.Helper()
	name, err := currentAssetName()
	if err != nil {
		t.Skipf("unsupported platform %s: %v", runtime.GOOS, err)
	}
	return name
}
