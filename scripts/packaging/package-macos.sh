#!/usr/bin/env bash
set -euo pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/../.." && pwd )"
cd "$DIR"

VERSION="1.9.7"
ARCH="$(uname -m)"
if [ "$ARCH" = "x86_64" ]; then ARCH="amd64"; fi
if [ "$ARCH" = "arm64" ] || [ "$ARCH" = "aarch64" ]; then ARCH="arm64"; fi

DIST_DIR="$DIR/dist/macos-${ARCH}"
BUILD_DIR="$DIR/dist/build-macos-${ARCH}/proximax-sirius-core"

echo "========================================================="
echo "  Packaging ProximaX Sirius Core Standalone for macOS"
echo "  Architecture: ${ARCH}"
echo "  Version:      ${VERSION}"
echo "========================================================="

rm -rf "$BUILD_DIR" "$DIST_DIR"
mkdir -p "$BUILD_DIR" "$DIST_DIR"

# 1. Build Frontend UI if not built
if [ ! -d "backend/dist" ]; then
    echo "-> Building React Frontend..."
    (cd frontend && npm run build)
fi

# 2. Compile Go Backend
echo "-> Compiling Go backend for macOS (${ARCH})..."
(cd backend && go build -ldflags="-s -w" -o "$BUILD_DIR/sirius-core" .)

# 3. Stage directories & clean configurations
echo "-> Staging configuration templates and runtime structure..."
mkdir -p "$BUILD_DIR/bin"
mkdir -p "$BUILD_DIR/chainconfig/resources"
mkdir -p "$BUILD_DIR/chainconfig/data/00000"
mkdir -p "$BUILD_DIR/chainconfig/logs"

# Copy engine & manager compatibility manifests
cp chainconfig/engine.compat.json "$BUILD_DIR/chainconfig/"
[ -f "chainconfig/manager.compat.json" ] && cp chainconfig/manager.compat.json "$BUILD_DIR/chainconfig/"

# Copy resources while strictly scrubbing private keys and state
cp -R chainconfig/resources/* "$BUILD_DIR/chainconfig/resources/"
rm -f "$BUILD_DIR/chainconfig/resources/config-harvesting.properties"
rm -f "$BUILD_DIR/chainconfig/resources/config-storage.properties"
rm -f "$BUILD_DIR/chainconfig/resources/config-user.properties"
rm -f "$BUILD_DIR/chainconfig/resources/config-manager.properties"
rm -f "$BUILD_DIR/chainconfig/resources/.sirius-token"
rm -f "$BUILD_DIR/chainconfig/resources/harvest-stats.json"

# Populate from templates
[ -f "chainconfig/resources/config-harvesting.properties.template" ] && \
    cp "chainconfig/resources/config-harvesting.properties.template" "$BUILD_DIR/chainconfig/resources/config-harvesting.properties"
[ -f "chainconfig/resources/config-storage.properties.template" ] && \
    cp "chainconfig/resources/config-storage.properties.template" "$BUILD_DIR/chainconfig/resources/config-storage.properties"
[ -f "chainconfig/resources/config-user.properties.template" ] && \
    cp "chainconfig/resources/config-user.properties.template" "$BUILD_DIR/chainconfig/resources/config-user.properties"

# Genesis bootstrap data
if [ -f "chainconfig/data/00000/00001.dat" ]; then
    cp "chainconfig/data/00000/00001.dat" "$BUILD_DIR/chainconfig/data/00000/"
    cp "chainconfig/data/00000/hashes.dat" "$BUILD_DIR/chainconfig/data/00000/"
fi

# Include launcher helper scripts
cp start-node.sh stop.sh restart.sh "$BUILD_DIR/"
cp stop.sh "$BUILD_DIR/stop-node.sh"
chmod +x "$BUILD_DIR"/*.sh "$BUILD_DIR/sirius-core"

# Set strict permissions on properties
chmod 0600 "$BUILD_DIR"/chainconfig/resources/*.properties || true

# Code signing if on macOS
if [ "$(uname)" = "Darwin" ]; then
    echo "-> Applying ad-hoc code signature..."
    xattr -cr "$BUILD_DIR"
    codesign --force --sign - "$BUILD_DIR/sirius-core" || true
fi

# 4. Create standalone tarball
TARBALL_NAME="proximax-sirius-darwin-${ARCH}-${VERSION}.tar.gz"
echo "-> Creating release archive: ${TARBALL_NAME}..."
tar -czf "$DIST_DIR/${TARBALL_NAME}" -C "$DIR/dist/build-macos-${ARCH}" proximax-sirius-core

echo "========================================================="
echo "  macOS Standalone Packaging Complete!"
echo "  Tarball: $DIST_DIR/${TARBALL_NAME}"
echo "========================================================="
