#!/bin/bash
# Build proxy-center for ARM64 (armv8) architecture
# Usage: ./build-armv8.sh [version-tag]

set -e

VERSION=${1:-dev-$(date +%Y%m%d%H%M%S)}
IMAGE_NAME="proxy-center"
IMAGE_TAG="armv8-${VERSION}"

echo "📦 Building ${IMAGE_NAME}:${IMAGE_TAG} for linux/arm64"

# Option 1: Using docker buildx (recommended, requires Docker buildx plugin)
if command -v docker buildx &> /dev/null; then
    echo "✓ Using docker buildx for multi-platform build"
    docker buildx build \
        --platform linux/arm64 \
        --tag "${IMAGE_NAME}:${IMAGE_TAG}" \
        --tag "${IMAGE_NAME}:latest-armv8" \
        --push=false \
        --output type=docker \
        .
    exit 0
fi

# Option 2: Cross-compile on Linux host with Go installed
if command -v go &> /dev/null && [ "$(uname -m)" = "x86_64" ]; then
    echo "✓ Using native Go cross-compilation (linux/amd64 -> linux/arm64)"
    
    # Compile binary
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
        go build -trimpath \
            -ldflags="-s -w -X main.version=${VERSION}" \
            -o build/proxyd-arm64 ./cmd/proxyd
    
    echo "✓ Binary compiled to build/proxyd-arm64"
    exit 0
fi

# Option 3: Compile directly on iStoreOS (ssh from remote)
if [ -n "$ISTOREIP" ]; then
    echo "✓ Compiling on remote iStoreOS instance: $ISTOREIP"
    ssh root@"$ISTOREIP" << 'EOSSH'
        cd /tmp
        git clone https://github.com/chenweihongcn/proxy-center.git || true
        cd proxy-center
        go build -o /tmp/proxyd ./cmd/proxyd
EOSSH
    scp "root@$ISTOREIP:/tmp/proxyd" "build/proxyd-arm64"
    exit 0
fi

echo "❌ No suitable build method found:"
echo "   - docker buildx not installed"
echo "   - Go not installed or not on linux/amd64"
echo "   - ISTOREIP not set for remote compilation"
echo ""
echo "To compile on iStoreOS directly:"
echo "  ISTOREIP=192.168.50.94 ./build-armv8.sh"
exit 1
