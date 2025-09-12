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
	// 如果没有设置自定义DNS，返回默认的第一个
	return DNSServers[0]
}

// GetCustomResolver 返回一个使用指定DNS服务器的解析器
func GetCustomResolver() *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 10 * time.Second,
			}

			// 尝试自定义DNS或默认DNS
			dnsServer := getCurrentDNSServer()
			conn, err := d.DialContext(ctx, "udp", dnsServer)
			if err == nil {
				return conn, nil
			}

			// 如果连接失败，尝试其他DNS服务器
			for _, server := range DNSServers {
				if server != dnsServer { // 避免重复尝试
					conn, err := d.DialContext(ctx, "udp", server)
					if err == nil {
						return conn, nil
					}
				}
			}

			// 所有DNS服务器都失败，返回最后一次的错误
			return nil, err
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
