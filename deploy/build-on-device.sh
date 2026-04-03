#!/bin/sh
# 在 iStoreOS 上直接运行此脚本来构建 proxy-center
# Usage: sh /tmp/build-on-device.sh

set -e

INSTALL_DIR="/opt/proxy-center"
DATA_DIR="$INSTALL_DIR/data"
SRC_DIR="/tmp/proxy-center-src"
ADMIN_PASS="${PROXY_ADMIN_PASS:-}"

echo "============================================"
echo " proxy-center 设备端构建脚本"
echo " 目标: iStoreOS NanoPi R2S Plus (ARMv8)"
echo "============================================"
echo ""

run_compose() {
    if docker compose version >/dev/null 2>&1; then
        docker compose "$@"
    else
        docker-compose "$@"
    fi
}

generate_password() {
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -base64 18 | tr -d '=+/\n' | cut -c1-20
        return 0
    fi

    if [ -r /dev/urandom ]; then
        tr -dc 'A-Za-z0-9' </dev/urandom | head -c 20
        return 0
    fi

    date +%s | sha256sum | cut -c1-20
}

# 检测可用的构建方式
HAS_DOCKER=false
HAS_GO=false

if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    HAS_DOCKER=true
    DOCKER_VER=$(docker version --format '{{.Server.Version}}' 2>/dev/null || echo "unknown")
    echo "✓ Docker: $DOCKER_VER"
fi

if command -v go >/dev/null 2>&1; then
    HAS_GO=true
    GO_VER=$(go version 2>/dev/null)
    echo "✓ Go: $GO_VER"
fi

if [ "$HAS_DOCKER" = false ] && [ "$HAS_GO" = false ]; then
    echo "✗ 没有 Docker 也没有 Go，无法构建"
    echo ""
    echo "安装 Go:"
    echo "  opkg update && opkg install golang"
    echo ""
    echo "或安装 Docker:"
    echo "  # 在 iStoreOS LuCI 界面 → iStore 应用市场 → 搜索 Docker"
    exit 1
fi

echo ""

if [ -z "$ADMIN_PASS" ]; then
    ADMIN_PASS=$(generate_password)
fi

# 创建目录
mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$SRC_DIR"

# 检查源码是否已上传
if [ ! -f "$SRC_DIR/go.mod" ]; then
    echo "⚠ 源代码不在 $SRC_DIR，尝试其他位置..."
    if [ -f "$INSTALL_DIR/go.mod" ]; then
        SRC_DIR="$INSTALL_DIR"
    else
        echo "✗ 找不到源代码！请先上传到 $SRC_DIR"
        exit 1
    fi
fi

echo "✓ 源代码位置: $SRC_DIR"
echo ""

if [ "$HAS_GO" = true ]; then
    # ===== 方案 A: 直接用 Go 编译（最快）=====
    echo "▶ 使用 Go 直接编译..."
    cd "$SRC_DIR"
    
    # 设置 GOPATH 和缓存
    export GOPATH=/tmp/go-build
    export GOCACHE=/tmp/go-cache
    mkdir -p "$GOPATH" "$GOCACHE"
    
    # 配置 GOPROXY 使用国内镜像
    export GOPROXY=https://goproxy.cn,direct
    export GONOSUMCHECK=*
    export GOFLAGS=-mod=mod
    
    echo "  下载依赖..."
    go mod download 2>&1 | grep -v "^#" || true
    
    echo "  编译..."
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
        go build -trimpath -ldflags="-s -w" \
        -o "$INSTALL_DIR/proxyd" \
        ./cmd/proxyd
    
    chmod +x "$INSTALL_DIR/proxyd"
    echo "✓ 编译完成: $INSTALL_DIR/proxyd"
    echo "  大小: $(du -sh $INSTALL_DIR/proxyd | cut -f1)"
    echo ""

elif [ "$HAS_DOCKER" = true ]; then
    # ===== 方案 B: 使用 Docker 构建 =====
    echo "▶ 使用 Docker 构建..."
    cd "$SRC_DIR"
    
    # 检查是否有 Dockerfile，如果没有则创建内联版本
    if [ ! -f "Dockerfile" ]; then
    cat > /tmp/Dockerfile.proxy << DOCKERFILE
FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && \
    go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
    -trimpath -ldflags="-s -w" \
    -o /out/proxyd ./cmd/proxyd

FROM alpine:3.21
RUN adduser -D -H -u 10001 proxy && mkdir -p /data && chown proxy:proxy /data
COPY --from=builder /out/proxyd /usr/local/bin/proxyd
USER proxy
ENV PROXY_DB_PATH=/data/proxy-center.db
ENV PROXY_HTTP_LISTEN=:8080
ENV PROXY_SOCKS_LISTEN=:1080
ENV PROXY_WEB_LISTEN=:8090
ENV PROXY_ADMIN_USER=admin
ENV PROXY_ADMIN_PASS=${ADMIN_PASS}
EXPOSE 8080 1080 8090
ENTRYPOINT ["proxyd"]
DOCKERFILE
        DOCKERFILE_PATH=/tmp/Dockerfile.proxy
    else
        DOCKERFILE_PATH="$SRC_DIR/Dockerfile"
    fi
    
    echo "  构建 Docker 镜像..."
    docker build -f "$DOCKERFILE_PATH" -t proxy-center:armv8-1.0 "$SRC_DIR"
    echo "✓ 镜像构建完成"
fi

echo ""

# 创建 docker-compose.yml 如果没有
if [ "$HAS_DOCKER" = true ]; then
        cat > "$INSTALL_DIR/docker-compose.yml" << COMPOSE
version: '3.8'
services:
  proxy-center:
    image: proxy-center:armv8-1.0
    container_name: proxy-center
    restart: always
    ports:
      - "8080:8080"
      - "1080:1080"
      - "8090:8090"
    volumes:
      - ./data:/data
    environment:
      PROXY_DB_PATH: /data/proxy-center.db
      PROXY_ADMIN_USER: admin
            PROXY_ADMIN_PASS: ${ADMIN_PASS}
      PROXY_LOG_DOMAINS: "true"
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "3"
COMPOSE
fi

# 创建 OpenWrt procd init 脚本
cat > /etc/init.d/proxy-center << INITD
#!/bin/sh /etc/rc.common
START=99
STOP=10
USE_PROCD=1

PROG=/opt/proxy-center/proxyd
DATA_DIR=/opt/proxy-center/data

start_service() {
    mkdir -p "$DATA_DIR"
    procd_open_instance
    procd_set_param command "$PROG"
    procd_set_param env \
        PROXY_DB_PATH="$DATA_DIR/proxy-center.db" \
        PROXY_HTTP_LISTEN=":8080" \
        PROXY_SOCKS_LISTEN=":1080" \
        PROXY_WEB_LISTEN=":8090" \
        PROXY_ADMIN_USER="admin" \
        PROXY_ADMIN_PASS="${ADMIN_PASS}" \
        PROXY_LOG_DOMAINS="true"
    procd_set_param respawn 3600 5 0
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}
INITD

chmod +x /etc/init.d/proxy-center

echo "▶ 启动 proxy-center..."

if [ "$HAS_DOCKER" = true ] && [ -f "$INSTALL_DIR/docker-compose.yml" ]; then
    # Docker 方式启动
    cd "$INSTALL_DIR"
    run_compose down 2>/dev/null || true
    run_compose up -d
    echo "✓ Docker 服务已启动"
else
    # 原生方式启动
    /etc/init.d/proxy-center enable
    /etc/init.d/proxy-center start
    echo "✓ 原生服务已启动"
fi

echo ""
echo "⏳ 等待服务就绪..."
for i in $(seq 1 20); do
    if wget -q -O - http://localhost:8090/healthz >/dev/null 2>&1; then
        echo "✓ 服务就绪!"
        break
    fi
    sleep 1
done

echo ""

# 获取设备 IP
DEVICE_IP=$(ip addr show br-lan 2>/dev/null | grep 'inet ' | awk '{print $2}' | cut -d/ -f1)
if [ -z "$DEVICE_IP" ]; then
    DEVICE_IP=$(ip addr show eth0 2>/dev/null | grep 'inet ' | awk '{print $2}' | cut -d/ -f1 | head -1)
fi
if [ -z "$DEVICE_IP" ]; then
    DEVICE_IP="192.168.50.94"
fi

echo "============================================"
echo "🎉 proxy-center 部署完成！"
echo "============================================"
echo ""
echo "📋 访问信息:"
echo "   Web UI:       http://${DEVICE_IP}:8090"
echo "   HTTP Proxy:   http://${DEVICE_IP}:8080"
echo "   SOCKS5:       socks5://${DEVICE_IP}:1080"
echo ""
echo "   管理账号:     admin"
echo "   管理密码:     ${ADMIN_PASS}"
echo ""
echo "⚠️  立即访问 Web UI 修改默认密码！"
echo ""
echo "📝 常用命令:"
if [ "$HAS_DOCKER" = true ]; then
    echo "   查看日志:     docker logs -f proxy-center"
    echo "   重启服务:     docker restart proxy-center"
    echo "   停止服务:     docker stop proxy-center"
else
    echo "   查看日志:     logread -f | grep proxy"
    echo "   重启服务:     /etc/init.d/proxy-center restart"
    echo "   停止服务:     /etc/init.d/proxy-center stop"
fi
echo ""
