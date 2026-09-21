# komari-agent

> **【非官方精简版 / Unofficial trimmed fork】**
>
> 本项目是 [komari-monitor](https://github.com/komari-monitor) 的非官方精简修改版，**仅供个人学习与自用，禁止商业用途**。
> 若原作者或任何权利人对本衍生版本有异议，请提 Issue，**我会立即删除本仓库及相关发布物**。
>
> This is an unofficial, trimmed derivative of komari-monitor, for **personal study and
> non-commercial use only**. If any rights holder objects, please open an issue and
> **I will remove this repository and its releases immediately**.
>
> 详见 [DISCLAIMER.md](./DISCLAIMER.md)

## 安装

### 一行命令（Linux / macOS）

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/aomtest/komari-slim-agent/main/install.sh) -e "https://你的面板地址" -t "你的-agent-token"
```

### 一行命令（Windows PowerShell，需管理员权限）

```powershell
iex "& { $(irm https://raw.githubusercontent.com/aomtest/komari-slim-agent/main/install.ps1) } -e 'https://你的面板地址' -t '你的-agent-token'"
```

### 手动安装

从 [Releases](https://github.com/aomtest/komari-slim-agent/releases/latest) 下载对应平台的二进制文件，
覆盖后重启服务即可。本版本已移除自升级功能，不会自动更新。

### 安装脚本可选参数

| 参数 | 说明 | 默认值 |
| --- | --- | --- |
| `--install-dir <路径>` | 安装目录 | Linux `/opt/komari`；Windows `C:\komari` |
| `--install-service-name <名称>` | 服务名 | `komari-agent` |
| `--install-ghproxy <代理前缀>` | GitHub 加速代理前缀 | 空（脚本内置镜像自动重试） |
| `--install-no-mirror` | 关闭自动镜像重试（仅 Linux） | 关闭 |
| `--install-version <版本>` | 安装指定版本 | 最新版 |

> 其余参数（如 `-e` / `-t`）会原样传给 agent 本体。
>
> 国内网络访问 `raw.githubusercontent.com` 可能失败，此时可先把脚本下载到本地再执行。

## 配置方式

agent 参数可以通过命令行参数、环境变量或 JSON 配置文件传入。

最小启动示例：

```bash
./komari-agent --endpoint "https://example.com" --token "your-token"
```

使用环境变量：

```bash
export AGENT_ENDPOINT="https://example.com"
export AGENT_TOKEN="your-token"
./komari-agent
```

使用 JSON 配置文件：

```bash
./komari-agent --config ./config.json
```

`config.json` 示例：

```json
{
  "endpoint": "https://example.com",
  "token": "your-token",
  "interval": 3,
  "ignore_unsafe_cert": false
}
```

配置优先级从低到高为：默认值、命令行参数、环境变量、JSON 配置文件。

常用配置项：

表中支持版本表示该参数本身首次在发布 tag 中出现；环境变量和 JSON 配置文件方式从 `1.1.33` 起支持，早于最早 tag 的参数记为 `0.0.9`。

| JSON 字段 | 环境变量 | 命令行参数 | 说明 | 支持版本 |
| --- | --- | --- | --- | --- |
| `endpoint` | `AGENT_ENDPOINT` | `--endpoint`, `-e` | 面板地址 | `0.0.9` |
| `token` | `AGENT_TOKEN` | `--token`, `-t` | agent token | `0.0.9` |
| `interval` | `AGENT_INTERVAL` | `--interval`, `-i` | 数据采集间隔，单位秒 | `0.0.9` |
| `ignore_unsafe_cert` | `AGENT_IGNORE_UNSAFE_CERT` | `--ignore-unsafe-cert`, `-u` | 忽略不安全证书 | `0.0.9` |
| `include_nics` | `AGENT_INCLUDE_NICS` | `--include-nics` | 仅统计指定网卡，逗号分隔 | `0.0.22` |
| `exclude_nics` | `AGENT_EXCLUDE_NICS` | `--exclude-nics` | 排除指定网卡，逗号分隔 | `0.0.22` |
| `include_mountpoints` | `AGENT_INCLUDE_MOUNTPOINTS` | `--include-mountpoint` | 仅统计指定挂载点，分号分隔 | `0.1.0` |
| `month_rotate` | `AGENT_MONTH_ROTATE` | `--month-rotate` | 流量统计每月重置日期，`0` 为禁用 | `0.1.0` |
| `auto_discovery_key` | `AGENT_AUTO_DISCOVERY_KEY` | `--auto-discovery` | 自动发现密钥 | `1.0.40` |
| `custom_dns` | `AGENT_CUSTOM_DNS` | `--custom-dns` | 自定义 DNS 服务器 | `1.0.80` |
| `enable_gpu` | `AGENT_ENABLE_GPU` | `--gpu` | 启用详细 GPU 监控 | `1.0.80` |
| `disable_compression` | `AGENT_DISABLE_COMPRESSION` | `--disable-compression` | 禁用 v2 传输压缩 | `1.2.10` |
| `prefer_ip_version` | `AGENT_PREFER_IP_VERSION` | `--prefer-ip-version` | 优先使用 IP 版本，可选 `4` 或 `6` | 未发布 |

完整参数可运行：

```bash
./komari-agent --help
```

详见 `cmd/flags/flag.go` 及 `cmd/root.go`
