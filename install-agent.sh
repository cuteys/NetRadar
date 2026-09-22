#!/bin/sh
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() {
    printf "${GREEN}[NetRadar]${NC} %s\n" "$1"
}

log_warn() {
    printf "${YELLOW}[WARN]${NC} %s\n" "$1"
}

log_error() {
    printf "${RED}[ERROR]${NC} %s\n" "$1"
    exit 1
}

printf "${CYAN}"
cat << 'EOF'
 _   _      _   ____           _             
| \ | | ___| |_|  _ \ __ _  __| | __ _ _ __  
|  \| |/ _ \ __| |_) / _` |/ _` |/ _` | '__| 
| |\  |  __/ |_|  _ < (_| | (_| | (_| | |    
|_| \_|\___|\__|_| \_\__,_|\__,_|\__,_|_|    
           NetRadar 边缘探针管理程序
EOF
printf "${NC}\n"

GITHUB_REPO="cuteys/NetRadar"
VERSION="latest"
SERVER_ADDR=""
AGENT_TOKEN=""
NODE_NAME=""
USE_TLS=false
WS_URL=""
ACTION="install"

do_uninstall() {
    log_info "正在卸载 NetRadar Agent 探针..."
    
    if command -v systemctl >/dev/null 2>&1; then
        systemctl stop netradar-agent 2>/dev/null || true
        systemctl disable netradar-agent 2>/dev/null || true
        rm -f /etc/systemd/system/netradar-agent.service
        systemctl daemon-reload 2>/dev/null || true
    fi

    if [ -f "/etc/init.d/netradar-agent" ]; then
        /etc/init.d/netradar-agent stop 2>/dev/null || true
        /etc/init.d/netradar-agent disable 2>/dev/null || true
        rm -f /etc/init.d/netradar-agent
    fi

    for f in /etc/rc.local /data/etc/rc.local /etc/firewall.user /data/auto_ssh/auto_ssh.sh; do
        if [ -f "$f" ]; then
            sed -i '/netradar/d' "$f" 2>/dev/null || true
        fi
    done

    if command -v uci >/dev/null 2>&1 && [ -f "/etc/config/firewall" ]; then
        uci -q delete firewall.netradar_agent 2>/dev/null || true
        uci commit firewall 2>/dev/null || true
    fi

    if [ -f "/etc/crontabs/root" ]; then
        sed -i '/netradar/d' /etc/crontabs/root 2>/dev/null || true
        /etc/init.d/cron restart 2>/dev/null || true
    fi

    killall agent 2>/dev/null || true
    pkill -9 -f "netradar/agent" 2>/dev/null || true
    pkill -9 -f "agent.*-server" 2>/dev/null || true

    rm -rf /opt/netradar/agent /data/netradar/agent /etc/netradar/agent
    rm -f /usr/local/bin/agent /usr/bin/agent_netradar
    rmdir /opt/netradar 2>/dev/null || true
    rmdir /data/netradar 2>/dev/null || true
    rmdir /etc/netradar 2>/dev/null || true

    printf "\n${GREEN}===================================================================${NC}\n"
    printf "${GREEN}   NetRadar Agent 探针已成功卸载！${NC}\n"
    printf "${GREEN}===================================================================${NC}\n\n"
    exit 0
}

while [ $# -gt 0 ]; do
    case "$1" in
        --uninstall|-uninstall|uninstall)
            ACTION="uninstall"
            shift 1
            ;;
        -s|--server|-server)
            SERVER_ADDR="$2"
            shift 2
            ;;
        -t|--token|-token)
            AGENT_TOKEN="$2"
            shift 2
            ;;
        -n|--name|-name|--node-name|-node-name)
            NODE_NAME="$2"
            shift 2
            ;;
        --tls|-tls)
            USE_TLS=true
            shift 1
            ;;
        -v|--version|-version)
            VERSION="$2"
            shift 2
            ;;
        -h|--help|-help)
            echo "用法:"
            echo "  安装: $0 -s <Server地址:端口> -t <通信密钥Token> [--tls]"
            echo "  卸载: $0 --uninstall"
            exit 0
            ;;
        *)
            if [ -z "$SERVER_ADDR" ]; then
                SERVER_ADDR="$1"
            elif [ -z "$AGENT_TOKEN" ]; then
                AGENT_TOKEN="$1"
            fi
            shift 1
            ;;
    esac
done

if [ "$ACTION" = "uninstall" ]; then
    do_uninstall
fi

if [ -z "$SERVER_ADDR" ]; then
    log_error "缺少对接地址，请指定 -s 参数（如 -s 192.168.1.100:8899）"
fi

if [ -z "$AGENT_TOKEN" ]; then
    log_error "缺少通信密钥，请指定 -t 参数"
fi

SERVER_ADDR=$(echo "$SERVER_ADDR" | sed -e 's|^ws://||' -e 's|^wss://||' -e 's|^http://||' -e 's|^https://||')
if [ "$USE_TLS" = true ]; then
    WS_URL="wss://${SERVER_ADDR}/ws/agent"
else
    WS_URL="ws://${SERVER_ADDR}/ws/agent"
fi

if [ -z "$NODE_NAME" ]; then
    NODE_NAME="$(hostname 2>/dev/null || echo 'Router-Node')"
fi

log_info "对接服务端: ${WS_URL}"
log_info "探针节点名: ${NODE_NAME}"

RAW_OS="$(uname -s 2>/dev/null || echo 'Linux')"
case "$RAW_OS" in
    *Linux*|*linux*)
        OS="linux"
        ;;
    *Darwin*|*darwin*)
        OS="darwin"
        ;;
    *FreeBSD*|*freebsd*)
        OS="freebsd"
        ;;
    *)
        OS="linux"
        ;;
esac
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64|amd64)
        TARGET_ARCH="amd64"
        ;;
    aarch64|arm64)
        TARGET_ARCH="arm64"
        ;;
    armv7*|armhf)
        TARGET_ARCH="armv7"
        ;;
    mipsle|mipsel)
        TARGET_ARCH="mipsle"
        ;;
    mips)
        IS_LITTLE_ENDIAN=false
        if grep -qiE "mipsel|ramips" /etc/openwrt_release /etc/os-release 2>/dev/null; then
            IS_LITTLE_ENDIAN=true
        elif grep -qiE "mt7621|mt7620|mt7628" /proc/cpuinfo 2>/dev/null; then
            IS_LITTLE_ENDIAN=true
        elif command -v hexdump >/dev/null 2>&1 && [ -f /bin/sh ]; then
            ELF_ENDIAN=$(hexdump -s 5 -n 1 -e '"%02x"' /bin/sh 2>/dev/null || echo "")
            if [ "$ELF_ENDIAN" = "01" ]; then
                IS_LITTLE_ENDIAN=true
            fi
        elif command -v opkg >/dev/null 2>&1 && opkg print-architecture 2>/dev/null | grep -qi "mipsel"; then
            IS_LITTLE_ENDIAN=true
        elif [ -f /bin/busybox ] && /bin/busybox 2>&1 | grep -qi "mipsel"; then
            IS_LITTLE_ENDIAN=true
        fi

        if [ "$IS_LITTLE_ENDIAN" = true ]; then
            TARGET_ARCH="mipsle"
        else
            TARGET_ARCH="mips"
        fi
        ;;
    i386|i686)
        TARGET_ARCH="386"
        ;;
    *)
        log_error "暂不支持的硬件架构: $ARCH"
        ;;
esac

log_info "系统环境: ${OS}/${TARGET_ARCH}"

# 检测系统下载工具及选项
USE_TOOL=""
WGET_EXTRA=""
if command -v curl >/dev/null 2>&1; then
    USE_TOOL="curl"
elif command -v wget >/dev/null 2>&1; then
    USE_TOOL="wget"
    if wget --help 2>&1 | grep -qi "no-check-certificate"; then
        WGET_EXTRA="--no-check-certificate"
    fi
else
    log_error "系统未找到 curl 或 wget 工具，请先安装后再运行本脚本"
fi

IS_OPENWRT=false
if [ -f "/etc/openwrt_release" ] || [ -f "/etc/openwrt_version" ] || [ -f "/etc/miwifi_version" ] || grep -qi "openwrt" /etc/os-release 2>/dev/null; then
    IS_OPENWRT=true
fi

# 优先选择持久化可写目录，防止路由器重启丢失
TARGET_DIR=""
if [ -d "/data" ] && [ -w "/data" ]; then
    mkdir -p "/data/netradar/agent" 2>/dev/null || true
    if [ -w "/data/netradar/agent" ]; then
        TARGET_DIR="/data/netradar/agent"
    fi
fi

if [ -z "$TARGET_DIR" ]; then
    mkdir -p "/opt/netradar/agent" 2>/dev/null || true
    if [ -w "/opt/netradar/agent" ]; then
        TARGET_DIR="/opt/netradar/agent"
    fi
fi

if [ -z "$TARGET_DIR" ]; then
    mkdir -p "/etc/netradar/agent" 2>/dev/null || true
    if [ -w "/etc/netradar/agent" ]; then
        TARGET_DIR="/etc/netradar/agent"
    fi
fi

if [ -z "$TARGET_DIR" ] && [ -w "/usr/bin" ]; then
    TARGET_DIR="/usr/bin"
fi

if [ -z "$TARGET_DIR" ]; then
    TARGET_DIR="/tmp/netradar/agent"
    mkdir -p "$TARGET_DIR" 2>/dev/null || true
    log_warn "未检测到可写持久分区，已临时安装到 /tmp"
fi

INSTALL_DIR="$TARGET_DIR"
AGENT_BIN="${INSTALL_DIR}/agent"
log_info "安装路径: ${INSTALL_DIR}"

TMP_DIR=$(mktemp -d 2>/dev/null || echo "/tmp/netradar_install")
mkdir -p "$TMP_DIR"

# 自动解析最新版本 Tag (当未手动指定 -v 时)
if [ "$VERSION" = "latest" ]; then
    log_info "正在自动解析 NetRadar Agent 最新版本号..."
    FETCHED_TAG=""

    # 1. 尝试通过 GitHub Releases API 解析
    API_ENDPOINTS="
https://gh-proxy.com/https://api.github.com/repos/${GITHUB_REPO}/releases/latest
https://api.github.com/repos/${GITHUB_REPO}/releases/latest
"
    for api_url in $API_ENDPOINTS; do
        api_body=""
        if [ "$USE_TOOL" = "curl" ]; then
            api_body=$(curl -fsSL -k --connect-timeout 6 -m 10 "$api_url" 2>/dev/null || true)
        else
            api_body=$(wget $WGET_EXTRA -qO- -T 6 "$api_url" 2>/dev/null || true)
        fi
        tag_val=$(echo "$api_body" | grep -m1 '"tag_name":' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/' || true)
        if [ -n "$tag_val" ]; then
            FETCHED_TAG="$tag_val"
            break
        fi
    done

    # 2. 若 API 被频控或不可用，尝试通过网页 302 重定向解析 Location
    if [ -z "$FETCHED_TAG" ]; then
        REDIRECT_ENDPOINTS="
https://gh-proxy.com/https://github.com/${GITHUB_REPO}/releases/latest
https://github.com/${GITHUB_REPO}/releases/latest
"
        for r_url in $REDIRECT_ENDPOINTS; do
            loc_header=""
            if [ "$USE_TOOL" = "curl" ]; then
                loc_header=$(curl -sI -k --connect-timeout 6 -m 10 "$r_url" 2>/dev/null | grep -i "^location:" | tr -d '\r\n' || true)
            else
                loc_header=$(wget $WGET_EXTRA --spider --server-response -T 6 "$r_url" 2>&1 | grep -i "Location:" | tr -d '\r\n' || true)
            fi
            tag_val=$(echo "$loc_header" | sed -E 's|.*/tag/([^/?# ]+).*|\1|' || true)
            if [ -n "$tag_val" ] && [ "$tag_val" != "$loc_header" ]; then
                FETCHED_TAG="$tag_val"
                break
            fi
        done
    fi

    if [ -n "$FETCHED_TAG" ]; then
        VERSION="$FETCHED_TAG"
        log_info "成功获取最新发布版本: ${VERSION}"
    else
        log_warn "未能在线解析到具体版本号，将直接采用最新分发资源 (latest) 下载"
    fi
fi

# 下载执行器封装函数（带错误日志捕获与输出）
try_fetch() {
    target_url="$1"
    dest_path="$2"
    source_name="$3"
    err_log="${TMP_DIR}/fetch_err.log"
    rm -f "$dest_path" "$err_log"

    printf "  -> 尝试从 [%s] 下载...\n" "$source_name"
    cmd_exit=0

    if [ "$USE_TOOL" = "curl" ]; then
        curl -fL -k --connect-timeout 12 -m 120 "$target_url" -o "$dest_path" 2>"$err_log" || cmd_exit=$?
    else
        wget $WGET_EXTRA -T 12 -O "$dest_path" "$target_url" 2>"$err_log" || cmd_exit=$?
    fi

    if [ $cmd_exit -eq 0 ] && [ -s "$dest_path" ]; then
        file_sz=$(ls -lh "$dest_path" 2>/dev/null | awk '{print $5}' || echo "OK")
        log_info "下载成功 (大小: ${file_sz})"
        rm -f "$err_log"
        return 0
    else
        detail_err=""
        if [ -f "$err_log" ]; then
            detail_err=$(grep -vE '^\s*$' "$err_log" | tail -n 2 | tr '\n' ' ' | sed 's/^[ \t]*//;s/[ \t]*$//' || true)
        fi
        [ -z "$detail_err" ] && detail_err="网络中断或返回空文件 (错误码: ${cmd_exit})"
        log_warn "下载失败: ${detail_err}"
        rm -f "$dest_path"
        return 1
    fi
}

log_info "准备下载探针程序 (架构: ${OS}/${TARGET_ARCH}, 版本: ${VERSION})..."
DOWNLOAD_SUCCESS=false

# 构造直连与镜像源地址前缀（纯单二进制文件分发，零解包依赖）
RAW_NAME="agent-${OS}-${TARGET_ARCH}"
if [ "$VERSION" != "latest" ]; then
    DIRECT_RAW="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${RAW_NAME}"
else
    DIRECT_RAW="https://github.com/${GITHUB_REPO}/releases/latest/download/${RAW_NAME}"
fi

MIRROR_SOURCES="
gh-proxy.com|https://gh-proxy.com/${DIRECT_RAW}
GitHub-Direct|${DIRECT_RAW}
"

for item in $MIRROR_SOURCES; do
    s_name=$(echo "$item" | cut -d'|' -f1)
    s_url=$(echo "$item" | cut -d'|' -f2)
    if [ -n "$s_name" ] && [ -n "$s_url" ]; then
        if try_fetch "$s_url" "${TMP_DIR}/agent" "${s_name}"; then
            DOWNLOAD_SUCCESS=true
            break
        fi
    fi
done

if [ "$DOWNLOAD_SUCCESS" = false ] || [ ! -s "${TMP_DIR}/agent" ]; then
    log_error "所有加速源与直连均下载失败！\n=======================================================\n可能原因：路由器当前 DNS 无法解析镜像站或外部网络被阻断。\n备选方案：您可在路由器终端手动执行单行下载命令：\n  mkdir -p ${INSTALL_DIR} && curl -fsSL -k \"https://gh-proxy.com/${DIRECT_RAW}\" -o ${AGENT_BIN} && chmod +x ${AGENT_BIN}\n======================================================="
fi

if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet netradar-agent 2>/dev/null; then
    systemctl stop netradar-agent 2>/dev/null || true
fi
killall agent 2>/dev/null || true

cp -f "${TMP_DIR}/agent" "$AGENT_BIN"
chmod +x "$AGENT_BIN"
rm -rf "$TMP_DIR"

if [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
    ln -sf "$AGENT_BIN" /usr/local/bin/agent 2>/dev/null || true
fi

# 启用内核连接跟踪流量统计
if [ -d "/proc/sys/net/netfilter" ]; then
    sysctl -w net.netfilter.nf_conntrack_acct=1 >/dev/null 2>&1 || true
    if [ -d "/etc/sysctl.d" ]; then
        echo "net.netfilter.nf_conntrack_acct = 1" > /etc/sysctl.d/99-netradar.conf 2>/dev/null || true
    elif [ -f "/etc/sysctl.conf" ]; then
        if ! grep -q "net.netfilter.nf_conntrack_acct" /etc/sysctl.conf; then
            echo "net.netfilter.nf_conntrack_acct = 1" >> /etc/sysctl.conf 2>/dev/null || true
        fi
    fi
fi

RUNNER_SCRIPT="${INSTALL_DIR}/start_agent.sh"
AGENT_CONFIG="${INSTALL_DIR}/config.yaml"

cat << EOF > "$RUNNER_SCRIPT"
#!/bin/sh
sysctl -w net.netfilter.nf_conntrack_acct=1 >/dev/null 2>&1 || true
if ! ps -w 2>/dev/null | grep -v grep | grep -q "${AGENT_BIN}"; then
    chmod +x "${AGENT_BIN}" 2>/dev/null || true
    ${AGENT_BIN} -c "${AGENT_CONFIG}" -server "${WS_URL}" -token "${AGENT_TOKEN}" >/tmp/netradar-agent.log 2>&1 &
fi
EOF
chmod +x "$RUNNER_SCRIPT"

if command -v systemctl >/dev/null 2>&1 && [ -d "/etc/systemd/system" ]; then
    cat << EOF > /etc/systemd/system/netradar-agent.service
[Unit]
Description=NetRadar Agent
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${AGENT_BIN} -c "${AGENT_CONFIG}" -server "${WS_URL}" -token "${AGENT_TOKEN}"
Restart=always
RestartSec=3s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable netradar-agent
    systemctl restart netradar-agent
    log_info "Systemd 守护已配置"

elif [ "$IS_OPENWRT" = true ] && [ -d "/etc/init.d" ] && [ -w "/etc/init.d" ]; then
    cat << EOF > /etc/init.d/netradar-agent
#!/bin/sh /etc/rc.common
USE_PROCD=1
START=99
STOP=10

start_service() {
    procd_open_instance
    procd_set_param command ${AGENT_BIN} -c "${AGENT_CONFIG}" -server "${WS_URL}" -token "${AGENT_TOKEN}"
    procd_set_param respawn 3600 3 0
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}
EOF
    chmod +x /etc/init.d/netradar-agent
    /etc/init.d/netradar-agent enable 2>/dev/null || true
    /etc/init.d/netradar-agent restart 2>/dev/null || true
    log_info "OpenWrt procd 守护已配置"
fi

# OpenWrt / 小米路由器 UCI Firewall 核心开机持久化（重启 100% 自动拉起）
if command -v uci >/dev/null 2>&1 && [ -f "/etc/config/firewall" ]; then
    uci -q delete firewall.netradar_agent 2>/dev/null || true
    uci set firewall.netradar_agent=include
    uci set firewall.netradar_agent.type='script'
    uci set firewall.netradar_agent.path="${RUNNER_SCRIPT}"
    uci set firewall.netradar_agent.enabled='1'
    uci commit firewall 2>/dev/null || true
    log_info "UCI Firewall 持久化开机自启已配置"
fi

# 写入自启动与巡检保活
if [ -f "/data/auto_ssh/auto_ssh.sh" ]; then
    if ! grep -q "start_agent.sh" /data/auto_ssh/auto_ssh.sh; then
        echo "${RUNNER_SCRIPT} >/dev/null 2>&1 & # netradar" >> /data/auto_ssh/auto_ssh.sh
    fi
fi

if [ -f "/etc/firewall.user" ] && [ -w "/etc/firewall.user" ]; then
    if ! grep -q "start_agent.sh" /etc/firewall.user; then
        echo "${RUNNER_SCRIPT} >/dev/null 2>&1 & # netradar" >> /etc/firewall.user
    fi
fi

for rc in /etc/rc.local /data/etc/rc.local; do
    if [ -f "$rc" ] && [ -w "$rc" ]; then
        if ! grep -q "start_agent.sh" "$rc"; then
            sed -i -e '$i\'"${RUNNER_SCRIPT} >/dev/null 2>&1 & # netradar" "$rc" 2>/dev/null || true
        fi
    fi
done

CRON_DIR="/etc/crontabs"
if [ -d "$CRON_DIR" ] && [ -w "$CRON_DIR" ]; then
    CRON_FILE="${CRON_DIR}/root"
    touch "$CRON_FILE" 2>/dev/null || true
    if ! grep -q "start_agent.sh" "$CRON_FILE"; then
        echo "*/2 * * * * ${RUNNER_SCRIPT} >/dev/null 2>&1 # netradar" >> "$CRON_FILE"
        /etc/init.d/cron restart 2>/dev/null || crond restart 2>/dev/null || true
    fi
fi

"$RUNNER_SCRIPT"

printf "\n${GREEN}===================================================================${NC}\n"
printf "${GREEN}   NetRadar Agent 探针安装完成！${NC}\n"
printf "   服务端: ${WS_URL}\n"
printf "   路径  : ${AGENT_BIN}\n"
printf "   卸载  : curl -fsSL -k https://raw.githubusercontent.com/${GITHUB_REPO}/master/install-agent.sh | sh -s -- --uninstall\n"
printf "${GREEN}===================================================================${NC}\n\n"
