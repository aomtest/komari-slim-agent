// Package version 保存构建期注入的版本号。
//
// 该常量原先位于 update 包（自动更新模块）。裁剪改造移除了整个 update 包
// （它会替换自身二进制），但版本号是上报给主控的必需字段
// （见 server/basicInfo.go 的 "version" 项），因此独立成包保留。
package version

// CurrentVersion 由构建脚本通过 -ldflags -X 注入，例如：
//
//	go build -ldflags="-X github.com/komari-monitor/komari-agent/version.CurrentVersion=v1.2.3"
var CurrentVersion = "0.0.1"
