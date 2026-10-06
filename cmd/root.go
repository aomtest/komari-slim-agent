package cmd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"syscall"

	"github.com/komari-monitor/komari-agent/dnsresolver"
	"github.com/komari-monitor/komari-agent/monitoring/netstatic"
	monitoring "github.com/komari-monitor/komari-agent/monitoring/unit"
	"github.com/komari-monitor/komari-agent/server"
	"github.com/komari-monitor/komari-agent/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	pkg_flags "github.com/komari-monitor/komari-agent/cmd/flags"
)

var flags = pkg_flags.GlobalConfig

var RootCmd = &cobra.Command{
	Use:   "komari-agent",
	Short: "komari agent",
	Long:  `komari agent`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 先记下命令行显式指定的 flag。
		//
		// 必须在 loadFromEnv 之前做：此刻 flags 里是「默认值 + 命令行值」，
		// 还没被环境变量或配置文件污染，能准确反映用户到底传了什么。
		var fromCLI []*pflag.Flag
		cmd.Flags().Visit(func(f *pflag.Flag) { fromCLI = append(fromCLI, f) })

		// 优先级（低 → 高）：配置文件 < 环境变量 < 命令行。
		//
		// 配置文件路径本身也遵循同样的规则，但命令行路径必须在加载环境变量
		// 前保存，否则 AGENT_CONFIG_FILE 会让程序读取错误的文件。
		cliConfigFile := ""
		for _, f := range fromCLI {
			if f.Name == "config" {
				cliConfigFile = f.Value.String()
				break
			}
		}
		if configFile := configFilePath(cliConfigFile); configFile != "" {
			if err := loadConfigFile(configFile); err != nil {
				return err
			}
		}
		loadFromEnv()
		// 把命令行显式指定的值盖回去，使命令行真正拥有最高优先级。
		//
		// 用 Flags().Set 而不是手工维护「字段 → flag 名」映射：flag 是通过
		// StringVar / BoolVar 绑定到字段指针的，Set 会自动写回对应字段；而 flag 名
		// 并非字段名的简单变形（AutoDiscoveryKey → auto-discovery），字符串推导不可靠。
		for _, f := range fromCLI {
			if err := cmd.Flags().Set(f.Name, f.Value.String()); err != nil {
				return fmt.Errorf("failed to reapply --%s: %w", f.Name, err)
			}
		}
		if flags.PreferIPVersion != "" && flags.PreferIPVersion != "4" && flags.PreferIPVersion != "6" {
			return fmt.Errorf("invalid --prefer-ip-version value %q: expected 4 or 6", flags.PreferIPVersion)
		}
		// 捕获中止信号，优雅退出
		stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		// 原实现由 shutdownCoordinator 串联「撤销 motd 安全警告 -> 停止 netstatic -> 退出」。
		// 裁剪后 motd 警告机制已整体移除，这里直接完成剩余两步。
		go func() {
			<-stopCtx.Done()
			log.Printf("shutting down gracefully...")
			if err := netstatic.Stop(); err != nil {
				log.Printf("Failed to stop netstatic monitoring: %v", err)
			}
			os.Exit(0)
		}()

		if flags.MonthRotate != 0 {
			err := netstatic.StartOrContinue()
			if err != nil {
				log.Println("Failed to start netstatic monitoring:", err)
			}
			nics, err := monitoring.InterfaceList()
			if err != nil {
				log.Println("Failed to get interface list for netstatic:", err)
			}
			err = netstatic.SetNewConfig(netstatic.NetStaticConfig{
				Nics: nics,
			})
			if err != nil {
				log.Println("Failed to set netstatic config:", err)
			}
		}

		log.Println("komari-slim Agent", version.CurrentVersion)

		// 设置 DNS 解析行为
		if flags.CustomDNS != "" {
			dnsresolver.SetCustomDNSServer(flags.CustomDNS)
			log.Printf("Using custom DNS server: %s", flags.CustomDNS)
		} else {
			// 未设置则使用系统默认 DNS（不使用内置列表）
			log.Printf("Using system default DNS resolver")
		}

		// Auto discovery
		if flags.AutoDiscoveryKey != "" {
			err := handleAutoDiscovery()
			if err != nil {
				return fmt.Errorf("auto-discovery failed: %w", err)
			}
		}
		diskList, err := monitoring.DiskList()
		if err != nil {
			log.Println("Failed to get disk list:", err)
		}
		log.Println("Monitoring Mountpoints:", diskList)
		interfaceList, err := monitoring.InterfaceList()
		if err != nil {
			log.Println("Failed to get interface list:", err)
		}
		log.Println("Monitoring Interfaces:", interfaceList)

		// 忽略不安全的证书
		if flags.IgnoreUnsafeCert {
			http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		}
		go server.DoUploadBasicInfoWorks()
		for {
			server.UpdateBasicInfo()
			server.EstablishWebSocketConnection()
		}
	},
}

func configFilePath(cliConfigFile string) string {
	if cliConfigFile != "" {
		return cliConfigFile
	}
	return os.Getenv("AGENT_CONFIG_FILE")
}

func loadConfigFile(path string) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	if err := json.Unmarshal(bytes, flags); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}
	return nil
}

func Execute() {
	for i, arg := range os.Args {
		if arg == "-memory-mode-available" || arg == "--memory-mode-available" {
			//flags.MemoryIncludeCache = true
			log.Println("WARNING: The --memory-mode-available flag is deprecated in version 1.0.70 and later. Use --memory-include-cache to report memory usage including cache/buffer.")
			os.Args = append(os.Args[:i], os.Args[i+1:]...)
		}
	}

	if err := RootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func init() {
	// 禁用 cobra 自带的 completion 子命令：它会为各类 shell 生成补全脚本，
	// 与「只做监控上报」的职责无关，属于可裁剪的无必要交互面。
	RootCmd.CompletionOptions.DisableDefaultCmd = true

	RootCmd.PersistentFlags().StringVarP(&flags.Token, "token", "t", "", "API token")
	//RootCmd.MarkPersistentFlagRequired("token")
	RootCmd.PersistentFlags().StringVarP(&flags.Endpoint, "endpoint", "e", "", "API endpoint")
	//RootCmd.MarkPersistentFlagRequired("endpoint")
	RootCmd.PersistentFlags().StringVar(&flags.AutoDiscoveryKey, "auto-discovery", "", "Auto discovery key for the agent")
	//RootCmd.PersistentFlags().BoolVar(&flags.MemoryModeAvailable, "memory-mode-available", false, "[deprecated]Report memory as available instead of used.")
	RootCmd.PersistentFlags().Float64VarP(&flags.Interval, "interval", "i", 3.0, "Interval in seconds")
	RootCmd.PersistentFlags().BoolVarP(&flags.IgnoreUnsafeCert, "ignore-unsafe-cert", "u", false, "Ignore unsafe certificate errors")
	RootCmd.PersistentFlags().IntVarP(&flags.MaxRetries, "max-retries", "r", 3, "Maximum number of retries")
	RootCmd.PersistentFlags().IntVarP(&flags.ReconnectInterval, "reconnect-interval", "c", 5, "Reconnect interval in seconds")
	RootCmd.PersistentFlags().IntVar(&flags.InfoReportInterval, "info-report-interval", 5, "Interval in minutes for reporting basic info")
	RootCmd.PersistentFlags().StringVar(&flags.IncludeNics, "include-nics", "", "Comma-separated list of network interfaces to include")
	RootCmd.PersistentFlags().StringVar(&flags.ExcludeNics, "exclude-nics", "", "Comma-separated list of network interfaces to exclude")
	RootCmd.PersistentFlags().StringVar(&flags.IncludeMountpoints, "include-mountpoint", "", "Semicolon-separated list of mount points to include for disk statistics")
	RootCmd.PersistentFlags().IntVar(&flags.MonthRotate, "month-rotate", 0, "Month reset for network statistics (0 to disable)")
	RootCmd.PersistentFlags().BoolVar(&flags.MemoryIncludeCache, "memory-include-cache", false, "Include cache/buffer in memory usage")
	RootCmd.PersistentFlags().BoolVar(&flags.MemoryReportRawUsed, "memory-exclude-bcf", false, "Use \"raminfo.Used = v.Total - v.Free - v.Buffers - v.Cached\" calculation for memory usage")
	RootCmd.PersistentFlags().StringVar(&flags.CustomDNS, "custom-dns", "", "Custom DNS server to use (e.g. 8.8.8.8, 114.114.114.114). By default, the program uses the system DNS resolver.")
	RootCmd.PersistentFlags().BoolVar(&flags.EnableGPU, "gpu", false, "Enable detailed GPU monitoring (usage, memory, multi-GPU support)")
	RootCmd.PersistentFlags().StringVar(&flags.CustomIpv4, "custom-ipv4", "", "Custom IPv4 address to use")
	RootCmd.PersistentFlags().StringVar(&flags.CustomIpv6, "custom-ipv6", "", "Custom IPv6 address to use")
	RootCmd.PersistentFlags().BoolVar(&flags.GetIpAddrFromNic, "get-ip-addr-from-nic", false, "Get IP address from network interface")
	RootCmd.PersistentFlags().StringVar(&flags.ConfigFile, "config", "", "Path to the configuration file")
	RootCmd.PersistentFlags().BoolVar(&flags.DisableCompression, "disable-compression", false, "Disable v2 gzip/permessage-deflate compression")
	RootCmd.PersistentFlags().StringVar(&flags.PreferIPVersion, "prefer-ip-version", "", "Prefer IP version for dashboard connections: 4 or 6")
	RootCmd.PersistentFlags().ParseErrorsWhitelist.UnknownFlags = true
}

func loadFromEnv() {
	val := reflect.ValueOf(flags).Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Get the env tag
		envTag := fieldType.Tag.Get("env")
		if envTag == "" {
			continue
		}

		// Get the environment variable value
		envValue := os.Getenv(envTag)
		if envValue == "" {
			continue
		}

		// Set the field based on its type
		switch field.Kind() {
		case reflect.String:
			field.SetString(envValue)
		case reflect.Bool:
			// 必须同时支持 false。原先只在 "true"/"1" 时 SetBool(true)，
			// 其余值什么都不做，于是环境变量只能把布尔项打开、永远无法关闭 ——
			// 想用 AGENT_ENABLE_GPU=false 覆盖配置文件里的 true 是做不到的。
			switch strings.ToLower(strings.TrimSpace(envValue)) {
			case "true", "1", "yes", "on":
				field.SetBool(true)
			case "false", "0", "no", "off":
				field.SetBool(false)
			}
		case reflect.Int:
			if intVal, err := strconv.Atoi(envValue); err == nil {
				field.SetInt(int64(intVal))
			}
		case reflect.Float64:
			if floatVal, err := strconv.ParseFloat(envValue, 64); err == nil {
				field.SetFloat(floatVal)
			}
		}
	}
}
