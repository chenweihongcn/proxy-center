#!/bin/bash
# Build release artifacts for proxy-center

set -e

VERSION="${1:-dev}"
RELEASE_DIR="./release"
ARTIFACT_DIR="$RELEASE_DIR/proxy-center-$VERSION"

echo "📦 Building release artifacts for version: $VERSION"
echo ""

# Clean up old release
rm -rf "$RELEASE_DIR"
mkdir -p "$ARTIFACT_DIR"

echo "[1/6] Copying source files..."
mkdir -p "$ARTIFACT_DIR/src"
cp -r cmd internal go.mod go.sum "$ARTIFACT_DIR/src/" 2>/dev/null || true
cp Dockerfile "$ARTIFACT_DIR/src/" 2>/dev/null || true

echo "[2/6] Copying deployment files..."
mkdir -p "$ARTIFACT_DIR/deploy"
cp -r deploy/* "$ARTIFACT_DIR/deploy/"
chmod +x "$ARTIFACT_DIR/deploy"/*.sh 2>/dev/null || true

echo "[3/6] Copying documentation..."
cp README.md QUICK_REFERENCE.md DELIVERY_CHECKLIST.md CHANGELOG.md "$ARTIFACT_DIR/" 2>/dev/null || true
cp deploy/ISTOREIOS_DEPLOYMENT.md "$ARTIFACT_DIR/" 2>/dev/null || true

echo "[4/6] Creating cross-compilation scripts..."
mkdir -p "$ARTIFACT_DIR/scripts"
cp build-armv8.sh build-armv8.ps1 "$ARTIFACT_DIR/scripts/" 2>/dev/null || true
chmod +x "$ARTIFACT_DIR/scripts"/*.sh 2>/dev/null || true

echo "[5/6] Creating release packages..."
cd "$RELEASE_DIR"

# Tar.gz for Linux/Unix
tar czf "proxy-center-$VERSION-source.tar.gz" "proxy-center-$VERSION/"
echo "   ✓ Created: proxy-center-$VERSION-source.tar.gz"

# ZIP for Windows
if command -v zip &> /dev/null; then
    zip -r "proxy-center-$VERSION-source.zip" "proxy-center-$VERSION/" -q
    echo "   ✓ Created: proxy-center-$VERSION-source.zip"
fi

cd ..

echo "[6/6] Creating checksums..."
cd "$RELEASE_DIR"
shasum -a 256 *.tar.gz *.zip 2>/dev/null > SHA256SUMS || true
cat SHA256SUMS 2>/dev/null || echo "   ℹ️  (checksums generation skipped)"

echo ""
echo "✅ Release build completed!"
echo ""
echo "📁 Artifacts location: $RELEASE_DIR/"
ls -lh "$RELEASE_DIR/" | grep -E "tar.gz|zip|SHA256SUMS"
echo ""
echo "📝 Next steps:"
echo "   1. Review the contents in: $ARTIFACT_DIR/"
echo "   2. Upload to GitHub Releases"
echo "   3. Tag the release: git tag -a v$VERSION -m 'Release v$VERSION'"
echo ""
