#!/bin/sh
# iStoreOS 部署验证脚本 - 检查环境和执行部署

set -e

echo "============================================"
echo "proxy-center iStoreOS 部署验证"
echo "============================================"
echo ""

# 1. 环境检查
echo "[1/5] 检查系统环境..."
echo "System Info:"
uname -a
echo ""
echo "Docker Status:"
docker version 2>/dev/null | head -3 || echo "⚠️  Docker 未安装或未启动"
DOCKER_ROOT=$(docker info --format '{{.DockerRootDir}}' 2>/dev/null || true)
if [ -n "$DOCKER_ROOT" ]; then
    DOCKER_FREE_MB=$(df -Pm "$DOCKER_ROOT" 2>/dev/null | awk 'NR==2 {print $4}')
    if [ -n "$DOCKER_FREE_MB" ]; then
        echo "Docker Root: $DOCKER_ROOT (${DOCKER_FREE_MB}MB 可用)"
        if [ "$DOCKER_FREE_MB" -lt 2048 ]; then
            echo "⚠️  Docker 空间低于 2GB，建议优先使用外部编译后上传。"
        fi
    fi
fi
echo ""

# 2. 网络检查
echo "[2/5] 检查网络连通性..."
ping -c 1 8.8.8.8 2>/dev/null && echo "✓ 互联网连接正常" || echo "⚠️  无法访问公网"
ping -c 1 hub.docker.com 2>/dev/null && echo "✓ Docker Hub 连接正常" || echo "⚠️  Docker Hub 无法访问（可用国内镜像）"
echo ""

# 3. 存储空间检查
echo "[3/5] 检查存储空间..."
df -h | grep -E "^/|Filesystem" | head -5
AVAILABLE_MB=$(df -Pm / | awk 'NR==2 {print $4}')
if [ -n "$AVAILABLE_MB" ] && [ "$AVAILABLE_MB" -gt 1024 ]; then
    echo "✓ 存储空间充足 (${AVAILABLE_MB}MB 可用)"
else
    echo "⚠️  存储空间有限 (${AVAILABLE_MB:-未知}MB 可用)"
fi
echo ""

# 4. Go 版本检查 (如果直接编译)
echo "[4/5] 检查 Go 版本..."
if command -v go >/dev/null 2>&1; then
    go version
    echo "✓ Go 已安装"
else
    echo "⚠️  Go 未安装（使用 Docker 构建）"
fi
echo ""

# 5. 创建工作目录
echo "[5/5] 创建工作目录..."
INSTALL_PATH="/opt/proxy-center"
mkdir -p "$INSTALL_PATH"
cd "$INSTALL_PATH"
echo "✓ 工作目录：$INSTALL_PATH"
echo ""

echo "============================================"
echo "✅ 环境检查完成，准备开始部署"
echo "============================================"
echo ""

DEVICE_IP=$(ip -4 addr show br-lan 2>/dev/null | awk '/inet / {print $2}' | cut -d/ -f1 | head -1)
if [ -z "$DEVICE_IP" ]; then
    DEVICE_IP=$(ip -4 addr show eth1 2>/dev/null | awk '/inet / {print $2}' | cut -d/ -f1 | head -1)
fi
if [ -z "$DEVICE_IP" ]; then
    DEVICE_IP=$(ip -4 addr show eth0 2>/dev/null | awk '/inet / {print $2}' | cut -d/ -f1 | head -1)
fi
if [ -z "$DEVICE_IP" ]; then
    DEVICE_IP="192.168.50.94"
fi

# 提供下一步指令
echo "下一步操作："
echo "1. 选择部署方式："
echo "   a) 方式 A (推荐): 使用 Docker Compose"
if docker compose version >/dev/null 2>&1; then
    echo "      docker compose pull && docker compose up -d"
else
    echo "      docker-compose pull && docker-compose up -d"
fi
echo ""
echo "   b) 方式 B: 本地编译运行"
echo "      go build -o proxyd ./cmd/proxyd && ./proxyd"
echo ""
echo "2. 验证服务状态："
echo "   docker ps | grep proxy-center"
echo ""
echo "3. 访问 Web 界面："
echo "   http://${DEVICE_IP}:8090"
echo ""
