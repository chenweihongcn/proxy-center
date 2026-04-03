#!/bin/sh
# Quick start script for iStoreOS NanoPi deployment
# Run on iStoreOS: sh -c "$(curl -fsSL https://raw.githubusercontent.com/chenweihongcn/proxy-center/main/deploy/istoreios-quick-start.sh)"

set -e

VERSION="1.0.0"
INSTALL_PATH="/opt/proxy-center"
DATA_PATH="$INSTALL_PATH/data"
ADMIN_PASS="${PROXY_ADMIN_PASS:-}"
COMPOSE_MODE=""

run_compose() {
  if [ "$COMPOSE_MODE" = "docker-compose" ]; then
    docker-compose "$@"
  else
    docker compose "$@"
  fi
}

check_health() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsS http://localhost:8090/healthz >/dev/null 2>&1
    return $?
  fi
  wget -q -O - http://localhost:8090/healthz >/dev/null 2>&1
}

get_device_ip() {
  for iface in br-lan eth1 eth0; do
    device_ip=$(ip -4 addr show "$iface" 2>/dev/null | awk '/inet / {print $2}' | cut -d/ -f1 | head -1)
    if [ -n "$device_ip" ]; then
      echo "$device_ip"
      return 0
    fi
  done

  ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i = 1; i <= NF; i++) if ($i == "src") {print $(i + 1); exit}}'
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

echo "🚀 proxy-center quick start for iStoreOS"
echo "   Version: $VERSION"
echo "   Install path: $INSTALL_PATH"
echo ""

echo "📋 Checking prerequisites..."

if ! command -v docker >/dev/null 2>&1; then
  echo "❌ Docker not found. Please install Docker from LuCI/iStore first."
  exit 1
fi

DOCKER_ROOT=$(docker info --format '{{.DockerRootDir}}' 2>/dev/null || true)
if [ -n "$DOCKER_ROOT" ]; then
  DOCKER_FREE_MB=$(df -Pm "$DOCKER_ROOT" 2>/dev/null | awk 'NR==2 {print $4}')
  if [ -n "$DOCKER_FREE_MB" ] && [ "$DOCKER_FREE_MB" -lt 2048 ]; then
    echo "⚠️  Docker 根目录剩余空间约 ${DOCKER_FREE_MB}MB，设备上直接构建镜像可能失败。"
    echo "   更稳妥的方式是先在 Windows 主机编译 arm64 镜像或二进制，再上传到 iStoreOS。"
  fi
fi

if docker compose version >/dev/null 2>&1; then
  COMPOSE_MODE="docker-compose-plugin"
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE_MODE="docker-compose"
else
  echo "❌ Compose not found. Please install docker compose plugin or docker-compose."
  exit 1
fi

if ! command -v git >/dev/null 2>&1; then
  echo "⚠️  Git not found, will use direct download"
fi

if [ -z "$ADMIN_PASS" ]; then
  ADMIN_PASS=$(generate_password)
fi

echo "📁 Creating directories..."
mkdir -p "$INSTALL_PATH" "$DATA_PATH"
cd "$INSTALL_PATH"

if [ -d ".git" ]; then
  echo "📦 Updating existing repository..."
  git pull origin main || true
else
  echo "📦 Cloning proxy-center repository..."
  if command -v git >/dev/null 2>&1; then
    if ! git clone https://github.com/chenweihongcn/proxy-center.git .; then
      echo "⚠️  Git clone failed, falling back to source archive..."
      wget -q https://github.com/chenweihongcn/proxy-center/archive/main.tar.gz -O - | tar xz --strip=1
    fi
  else
    echo "⬇️  Downloading source archive..."
    wget -q https://github.com/chenweihongcn/proxy-center/archive/main.tar.gz -O - | tar xz --strip=1
  fi
fi

if [ ! -f "docker-compose.yml" ]; then
  echo "🔧 Creating docker-compose.yml..."
  cat > docker-compose.yml << EOF
version: '3.8'
services:
  proxy-center:
    image: proxy-center:armv8-${VERSION}
    container_name: proxy-center
    restart: always
    ports:
      - "8080:8080"
      - "1080:1080"
      - "8090:8090"
    environment:
      PROXY_DB_PATH: /data/proxy-center.db
      PROXY_HTTP_LISTEN: :8080
      PROXY_SOCKS_LISTEN: :1080
      PROXY_WEB_LISTEN: :8090
      PROXY_ADMIN_USER: admin
      PROXY_ADMIN_PASS: ${ADMIN_PASS}
      PROXY_LOG_DOMAINS: "true"
    volumes:
      - ./data:/data
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "3"
    healthcheck:
      test: ["CMD", "wget", "-q", "-O", "-", "http://localhost:8090/healthz"]
      interval: 30s
      timeout: 5s
      retries: 3
EOF
fi

echo "🔨 Building Docker image for ARM64..."
if docker buildx version >/dev/null 2>&1; then
  docker buildx create --use --name istore-builder 2>/dev/null || docker buildx use istore-builder
  docker buildx build \
    --platform linux/arm64 \
    --tag "proxy-center:armv8-${VERSION}" \
    --tag "proxy-center:latest" \
    --load \
    .
else
  docker build \
    --tag "proxy-center:armv8-${VERSION}" \
    --tag "proxy-center:latest" \
    .
fi

echo "▶️  Starting services..."
run_compose down 2>/dev/null || true
run_compose up -d

echo ""
echo "⏳ Waiting for service to be ready..."
for i in $(seq 1 30); do
  if check_health; then
    break
  fi
  sleep 1
done

DEVICE_IP=$(get_device_ip)
if [ -z "$DEVICE_IP" ]; then
  DEVICE_IP="192.168.50.94"
fi

echo ""
echo "✅ proxy-center started successfully!"
echo ""
echo "📋 Quick reference:"
echo "   Web UI:       http://${DEVICE_IP}:8090"
echo "   Username:     admin"
echo "   Password:     $ADMIN_PASS"
echo ""
echo "   HTTP Proxy:   http://${DEVICE_IP}:8080"
echo "   SOCKS5:       socks5://${DEVICE_IP}:1080"
echo ""
echo "📝 Logs:"
if [ "$COMPOSE_MODE" = "docker-compose" ]; then
  echo "   docker-compose logs -f"
else
  echo "   docker compose logs -f"
fi
echo ""
echo "⚠️  IMPORTANT: Change the admin password immediately!"
echo "   Visit http://${DEVICE_IP}:8090 and update credentials"
echo ""
echo "📚 Documentation: $INSTALL_PATH/deploy/ISTOREIOS_DEPLOYMENT.md"
