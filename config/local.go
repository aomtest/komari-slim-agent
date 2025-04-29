package config

import (
	"encoding/json"
	"flag"
	"os"
)

type LocalConfig struct {
	Endpoint          string  `json:"endpoint"`
	Token             string  `json:"token"`
	Terminal          bool    `json:"terminal"`
	MaxRetries        int     `json:"maxRetries"`
	ReconnectInterval int     `json:"reconnectInterval"`
	IgnoreUnsafeCert  bool    `json:"ignoreUnsafeCert"`
	Interval          float64 `json:"interval"`
}

func LoadConfig() (LocalConfig, error) {

	var (
		endpoint          string
		token             string
		terminal          bool
		path              string
		maxRetries        int
		reconnectInterval int
		ignoreUnsafeCert  bool
		interval          float64
	)

	flag.StringVar(&endpoint, "e", "", "The endpoint URL")
	flag.StringVar(&token, "token", "", "The authentication token")
	flag.BoolVar(&terminal, "terminal", false, "Enable or disable terminal (default: false)")
	flag.StringVar(&path, "c", "agent.json", "Path to the configuration file")
	flag.IntVar(&maxRetries, "maxRetries", 10, "Maximum number of retries for WebSocket connection")
	flag.IntVar(&reconnectInterval, "reconnectInterval", 5, "Reconnect interval in seconds")
	flag.Float64Var(&interval, "interval", 1.1, "Interval in seconds for sending data to the server")
	flag.BoolVar(&ignoreUnsafeCert, "ignoreUnsafeCert", false, "Ignore unsafe certificate errors")
	flag.Parse()

	// Ensure -c cannot coexist with other flags
	if path != "agent.json" && (endpoint != "" || token != "" || !terminal) {
		return LocalConfig{}, flag.ErrHelp
	}

	// 必填项 Endpoint、Token 没有读取配置文件
	if endpoint == "" || token == "" {
		file, err := os.Open(path)
		if err != nil {
			return LocalConfig{}, err
		}
		defer file.Close()

		var localConfig LocalConfig
		if err := json.NewDecoder(file).Decode(&localConfig); err != nil {
			return LocalConfig{}, err
		}

		return localConfig, nil
	}

	return LocalConfig{
		Endpoint:          endpoint,
		Token:             token,
		Terminal:          terminal,
		MaxRetries:        maxRetries,
		ReconnectInterval: reconnectInterval,
		IgnoreUnsafeCert:  ignoreUnsafeCert,
		Interval:          interval,
	}, nil
}
