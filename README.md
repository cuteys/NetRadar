# NetRadar

轻量、安全、美观的边缘网络流量态势感知系统，专为家用路由器（OpenWrt / 小米等）及 Linux / macOS 主机设计。

---

## 🌟 特性

- 🎨 **苹果风格极简大屏**：磨砂毛玻璃质感、微光飞线流动，完美支持深浅色无缝切换与移动端自适应。
- 🛡️ **安全鉴权体系**：控制台登录鉴权，探针预共享 Token 双向认证，敏感信息加密存储与安全脱敏。
- 🚀 **极低资源消耗**：流式读取内核连接跟踪表计算增量，探针内存占用低于 8MB，无抓包性能损耗。
- 🗺️ **真实离线地图解析**：支持 MMDB 离线地理信息库与高精度中国 IP 库（GeoCN），保护外联网络隐私。
- 🔄 **全自动升级**：探针内置静默检查更新机制，支持国内代理加速（gh-proxy.com）。
- 📦 **单二进制与容器化**：Go 静态嵌入前端资源，控制台单文件即可运行，提供全架构 Docker 镜像。

---

## 🚀 快速开始

### 1. 部署服务端 (Server)

#### 方式 A：Docker Compose（推荐）

```yaml
services:
  netradar:
    image: cuteys/netradar:latest
    container_name: netradar
    restart: unless-stopped
    ports:
      - 8899:8899
    volumes:
      - /etc/localtime:/etc/localtime:ro
      - ./data:/data
    environment:
      - TZ=Asia/Shanghai
```

启动并查看初始密码：

```bash
docker compose up -d
docker compose logs -f netradar
```

浏览器打开 `http://YOUR_SERVER_IP:8899` 即可访问。

#### 方式 B：单二进制运行

从 [Releases](https://github.com/cuteys/NetRadar/releases) 下载对应系统的 `netradar` 直接运行：

```bash
./netradar
```

---

### 2. 部署探针 (Agent)

探针优先持久化安装至 `/data/netradar/agent`（OpenWrt/小米路由）或 `/opt/netradar/agent`。

#### Linux / OpenWrt / 小米路由器 / macOS

一键安装（自动识别 CPU 架构、开启内核流量记账并配置开机自启守护）：

```bash
curl -fsSL -k https://raw.githubusercontent.com/cuteys/NetRadar/master/install-agent.sh | sh -s -- -s "YOUR_SERVER_IP:8899" -t "YOUR_AGENT_TOKEN" -n "路由器名称"
```

> **国内服务器加速**：
> ```bash
> curl -fsSL -k https://gh-proxy.com/https://raw.githubusercontent.com/cuteys/NetRadar/master/install-agent.sh | sh -s -- -s "YOUR_SERVER_IP:8899" -t "YOUR_AGENT_TOKEN"
> ```
> **一键卸载**：
> ```bash
> curl -fsSL -k https://raw.githubusercontent.com/cuteys/NetRadar/master/install-agent.sh | sh -s -- --uninstall
> ```

#### 免安装单二进制运行

从 [Releases](https://github.com/cuteys/NetRadar/releases) 下载 `agent` 单文件：
- `./agent -server "ws://YOUR_SERVER_IP:8899/ws/agent" -token "TOKEN"`

---

## 🛠️ 配置参数

### Server 控制端

| 环境变量 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `NETRADAR_LISTEN` | `:8899` | 监听地址 |
| `NETRADAR_ADMIN_PASSWORD` | 随机生成 | 管理员初始密码（首次启动日志打印，可随时在 Web 面板或命令行修改） |
| `NETRADAR_AGENT_TOKEN` | 随机生成 | 探针鉴权密钥（可在 Web 面板查看与修改） |
| `NETRADAR_DB_PATH` | `/data/sqlite/sqlite.db` | SQLite 数据库路径 |
| `NETRADAR_GEOIP_DIR` | `/data/geoip` | GeoIP 离线库目录 |

### Agent 探针

| 参数 | 说明 |
| :--- | :--- |
| `-server` | 控制端 WebSocket 地址（如 `ws://10.0.0.1:8899/ws/agent`） |
| `-token` | 探针通信密钥 |
| `-node-name` | 节点显示名称（别名 `-name`） |
| `-interval` | 采样上报间隔（默认 3 秒） |

---

## 📄 开源协议

MIT License
