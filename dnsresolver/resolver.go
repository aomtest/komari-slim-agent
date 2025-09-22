package dnsresolver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

var (
	// DNS服务器列表，按优先级排序
	DNSServers = []string{
		"114.114.114.114:53", // 114DNS，中国大陆
		"8.8.8.8:53",         // Google DNS，全球
		"8.8.4.4:53",         // Google DNS备用，全球
		"1.1.1.1:53",         // Cloudflare DNS，全球
		"223.5.5.5:53",       // 阿里DNS，中国大陆
		"119.29.29.29:53",    // DNSPod，中国大陆

	}

	// CustomDNSServer 自定义DNS服务器，可以通过命令行参数设置
	CustomDNSServer string
)

// SetCustomDNSServer 设置自定义DNS服务器
func SetCustomDNSServer(dnsServer string) {
	if dnsServer != "" {
		// 检查是否已包含端口，如果没有则添加默认端口53
		if !strings.Contains(dnsServer, ":") {
			dnsServer = dnsServer + ":53"
		}
		CustomDNSServer = dnsServer
	}
}

// getCurrentDNSServer 获取当前要使用的DNS服务器
func getCurrentDNSServer() string {
	if CustomDNSServer != "" {
		return CustomDNSServer
	}
	// 如果没有设置自定义DNS，返回空字符串，表示应使用系统默认解析器
	return ""
}

// GetCustomResolver 返回一个解析器：
// - 若设置了自定义 DNS：使用该服务器（并在失败时尝试内置列表作为兜底）。
// - 若未设置自定义 DNS：返回系统默认解析器（不使用内置列表）。
func GetCustomResolver() *net.Resolver {
	// 未设置自定义 DNS，直接使用系统默认解析器
	if getCurrentDNSServer() == "" {
		return net.DefaultResolver
	}

	// 设置了自定义 DNS，则构造使用自定义 DNS 的解析器
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 10 * time.Second}

			// 优先使用自定义 DNS 服务器
			dnsServer := getCurrentDNSServer()
			if dnsServer != "" {
				if conn, err := d.DialContext(ctx, "udp", dnsServer); err == nil {
					return conn, nil
				}
			}

			// 如果自定义DNS不可用，则尝试内置列表作为兜底
			for _, server := range DNSServers {
				if server == dnsServer {
					continue
				}
				if conn, err := d.DialContext(ctx, "udp", server); err == nil {
					return conn, nil
				}
			}

			return nil, fmt.Errorf("no available DNS server")
		},
	}
}

// GetHTTPClient 返回一个使用自定义DNS解析器的HTTP客户端
func GetHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	customResolver := GetCustomResolver()

	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				ips, err := customResolver.LookupHost(ctx, host)
				if err != nil {
					return nil, err
				}
				for _, ip := range ips {
					dialer := &net.Dialer{
						Timeout:   30 * time.Second,
						KeepAlive: 30 * time.Second,
						DualStack: true,
					}
					conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
					if err == nil {
						return conn, nil
					}
				}
				return nil, fmt.Errorf("failed to dial to any of the resolved IPs")
			},
			MaxIdleConns:          10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: timeout,
	}
}

// GetNetDialer 返回一个使用自定义DNS解析器的网络拨号器
func GetNetDialer(timeout time.Duration) *net.Dialer {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		Resolver:  GetCustomResolver(),
	}
}
