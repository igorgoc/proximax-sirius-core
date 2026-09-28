#!/usr/bin/env bash
set -euo pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/../.." && pwd )"
cd "$DIR"

VERSION="${VERSION:-${GITHUB_REF_NAME:-1.9.10}}"
VERSION="${VERSION#v}"
if [[ ! "$VERSION" =~ ^[0-9] ]]; then
    VERSION="1.9.10"
fi
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
mkdir -p "$BUILD_DIR/bin/macos"
if [ "$(uname)" = "Darwin" ]; then
    (cd backend && go build -ldflags="-s -w" -o "$BUILD_DIR/sirius-core" .)
else
    (cd backend && GOOS=darwin GOARCH="$ARCH" go build -ldflags="-s -w" -o "$BUILD_DIR/sirius-core" .)
fi
cp "$BUILD_DIR/sirius-core" "$BUILD_DIR/bin/macos/sirius-core"

# 3. Stage directories & clean configurations
echo "-> Staging configuration templates and runtime structure..."
mkdir -p "$BUILD_DIR/bin"
if [ -d "bin" ]; then
    cp -R bin/* "$BUILD_DIR/bin/" 2>/dev/null || true
fi

# Ensure engine binaries and all required shared libraries exist in the package
if [ ! -f "$BUILD_DIR/bin/sirius.bc" ] || [ ! -f "$BUILD_DIR/bin/libcatapult.plugins.config.dylib" ]; then
    echo "-> Engine binaries or dynamic libraries missing in staging. Downloading official macOS engine..."
    if command -v curl >/dev/null 2>&1; then
        (curl -f -sSL "https://github.com/igorgoc/cpp-xpx-chain/releases/latest/download/sirius-darwin-${ARCH}.tar.gz" || \
         curl -f -sSL "https://github.com/igorgoc/cpp-xpx-chain/releases/download/1.9.10/sirius-darwin-${ARCH}.tar.gz" || \
         curl -f -sSL "https://github.com/igorgoc/cpp-xpx-chain/releases/download/1.9.8/sirius-darwin-${ARCH}.tar.gz") | tar -xz -C "$BUILD_DIR"
        echo "-> Official macOS engine unpacked into package successfully."
    fi
fi

if [ -f "bin/version.txt" ]; then
    cp bin/version.txt "$BUILD_DIR/bin/version.txt"
else
    echo "${VERSION}" > "$BUILD_DIR/bin/version.txt"
fi
mkdir -p "$BUILD_DIR/chainconfig/resources"
mkdir -p "$BUILD_DIR/chainconfig/data/00000"
mkdir -p "$BUILD_DIR/chainconfig/logs"

# Copy engine, manager & snapshot compatibility manifests
cp chainconfig/engine.compat.json "$BUILD_DIR/chainconfig/"
[ -f "chainconfig/manager.compat.json" ] && cp chainconfig/manager.compat.json "$BUILD_DIR/chainconfig/"
[ -f "chainconfig/snapshot.compat.json" ] && cp chainconfig/snapshot.compat.json "$BUILD_DIR/chainconfig/"

# Copy resources while strictly scrubbing private keys and state
cp -R chainconfig/resources/* "$BUILD_DIR/chainconfig/resources/"
rm -f "$BUILD_DIR/chainconfig/resources/config-harvesting.properties"
rm -f "$BUILD_DIR/chainconfig/resources/config-storage.properties"
rm -f "$BUILD_DIR/chainconfig/resources/config-user.properties"
rm -f "$BUILD_DIR/chainconfig/resources/config-manager.properties"
rm -f "$BUILD_DIR/chainconfig/resources/.sirius-token"
rm -f "$BUILD_DIR/chainconfig/resources/harvest-stats.json"

# Populate clean non-template properties
[ -f "chainconfig/resources/config-harvesting.properties.template" ] && \
    cp "chainconfig/resources/config-harvesting.properties.template" "$BUILD_DIR/chainconfig/resources/config-harvesting.properties"
[ -f "chainconfig/resources/config-storage.properties.template" ] && \
    cp "chainconfig/resources/config-storage.properties.template" "$BUILD_DIR/chainconfig/resources/config-storage.properties"
[ -f "chainconfig/resources/config-user.properties.template" ] && \
    cp "chainconfig/resources/config-user.properties.template" "$BUILD_DIR/chainconfig/resources/config-user.properties"
[ -f "chainconfig/resources/config-manager.properties.template" ] && \
    cp "chainconfig/resources/config-manager.properties.template" "$BUILD_DIR/chainconfig/resources/config-manager.properties"

# Remove all .template files from bundle package so only non-template configuration files remain
rm -f "$BUILD_DIR/chainconfig/resources"/*.template

# Genesis bootstrap data & seed package
if [ -d "chainconfig/genesis_seed" ]; then
    cp -R chainconfig/genesis_seed "$BUILD_DIR/chainconfig/"
    cp "chainconfig/genesis_seed/00000/00001.dat" "$BUILD_DIR/chainconfig/data/00000/" 2>/dev/null || true
    cp "chainconfig/genesis_seed/00000/hashes.dat" "$BUILD_DIR/chainconfig/data/00000/" 2>/dev/null || true
    [ -f "chainconfig/genesis_seed/index.dat" ] && cp "chainconfig/genesis_seed/index.dat" "$BUILD_DIR/chainconfig/data/"
fi
if [ -f "chainconfig/data/00000/00001.dat" ]; then
    cp "chainconfig/data/00000/00001.dat" "$BUILD_DIR/chainconfig/data/00000/"
    cp "chainconfig/data/00000/hashes.dat" "$BUILD_DIR/chainconfig/data/00000/"
    [ -f "chainconfig/data/index.dat" ] && cp "chainconfig/data/index.dat" "$BUILD_DIR/chainconfig/data/"
fi

# Include launcher helper scripts in scripts/macos
mkdir -p "$BUILD_DIR/scripts/macos"
cp -R scripts/macos/* "$BUILD_DIR/scripts/macos/"
rm -f "$BUILD_DIR/scripts/macos/package-macos.sh" "$BUILD_DIR/scripts/macos/bundle_dylib_deps.py"

# Stage ONLY top-level single entry launchers and README in release root
cp scripts/macos/start.sh scripts/macos/start.command scripts/macos/stop.sh "$BUILD_DIR/"
[ -f "scripts/macos/README.md" ] && cp scripts/macos/README.md "$BUILD_DIR/"
chmod +x "$BUILD_DIR"/*.sh "$BUILD_DIR"/*.command "$BUILD_DIR/sirius-core" "$BUILD_DIR/scripts/macos"/* 2>/dev/null || true

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
