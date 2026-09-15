#!/usr/bin/env bash
set -euo pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/../.." && pwd )"
cd "$DIR"

VERSION="${VERSION:-${GITHUB_REF_NAME:-1.9.8}}"
VERSION="${VERSION#v}"
if [[ ! "$VERSION" =~ ^[0-9] ]]; then
    VERSION="1.9.8"
fi
ARCH="${1:-amd64}"
DIST_DIR="$DIR/dist/linux-${ARCH}"
BUILD_DIR="$DIR/dist/build-linux-${ARCH}"

echo "========================================================="
echo "  Packaging ProximaX Sirius Core for Linux (${ARCH})"
echo "  Version: ${VERSION}"
echo "========================================================="

rm -rf "$BUILD_DIR" "$DIST_DIR"
mkdir -p "$BUILD_DIR/opt/proximax-sirius-core"
mkdir -p "$DIST_DIR"

# 1. Build Frontend UI if dist doesn't exist
if [ ! -d "backend/dist" ]; then
    echo "-> Building React Frontend..."
    (cd frontend && npm run build)
fi

# 2. Compile Go Backend for Linux
echo "-> Compiling/staging Go backend for Linux (${ARCH})..."
mkdir -p "$BUILD_DIR/opt/proximax-sirius-core/bin/linux"
if [ "$(uname)" = "Linux" ]; then
    (cd backend && go build -ldflags="-s -w" -o "$BUILD_DIR/opt/proximax-sirius-core/sirius-core" .)
elif [ -f "bin/linux/sirius-core" ]; then
    cp "bin/linux/sirius-core" "$BUILD_DIR/opt/proximax-sirius-core/sirius-core"
elif [ -f "backend/sirius-core-linux-${ARCH}" ]; then
    cp "backend/sirius-core-linux-${ARCH}" "$BUILD_DIR/opt/proximax-sirius-core/sirius-core"
elif [ -f "backend/sirius-core" ] && file "backend/sirius-core" | grep -qi "ELF"; then
    cp "backend/sirius-core" "$BUILD_DIR/opt/proximax-sirius-core/sirius-core"
else
    echo "ℹ Non-Linux host detected; cross-compiling pure Go backend for Linux..."
    (cd backend && GOOS=linux GOARCH="$ARCH" go build -ldflags="-s -w" -o "$BUILD_DIR/opt/proximax-sirius-core/sirius-core" .)
fi
cp "$BUILD_DIR/opt/proximax-sirius-core/sirius-core" "$BUILD_DIR/opt/proximax-sirius-core/bin/linux/sirius-core"

# 3. Stage directories & clean configurations
echo "-> Staging configuration templates and runtime structure..."
TARGET_OPT="$BUILD_DIR/opt/proximax-sirius-core"
mkdir -p "$TARGET_OPT/bin"
mkdir -p "$TARGET_OPT/chainconfig/resources"
mkdir -p "$TARGET_OPT/chainconfig/data/00000"
mkdir -p "$TARGET_OPT/chainconfig/logs"

# Copy engine, manager & snapshot compatibility manifests
cp chainconfig/engine.compat.json "$TARGET_OPT/chainconfig/"
[ -f "chainconfig/manager.compat.json" ] && cp chainconfig/manager.compat.json "$TARGET_OPT/chainconfig/"
[ -f "chainconfig/snapshot.compat.json" ] && cp chainconfig/snapshot.compat.json "$TARGET_OPT/chainconfig/"

# Copy resources while strictly scrubbing private keys and state
cp -R chainconfig/resources/* "$TARGET_OPT/chainconfig/resources/"
rm -f "$TARGET_OPT/chainconfig/resources/config-harvesting.properties"
rm -f "$TARGET_OPT/chainconfig/resources/config-storage.properties"
rm -f "$TARGET_OPT/chainconfig/resources/config-user.properties"
rm -f "$TARGET_OPT/chainconfig/resources/config-manager.properties"
rm -f "$TARGET_OPT/chainconfig/resources/.sirius-token"
rm -f "$TARGET_OPT/chainconfig/resources/harvest-stats.json"

# Populate clean non-template properties
[ -f "chainconfig/resources/config-harvesting.properties.template" ] && \
    cp "chainconfig/resources/config-harvesting.properties.template" "$TARGET_OPT/chainconfig/resources/config-harvesting.properties"
[ -f "chainconfig/resources/config-storage.properties.template" ] && \
    cp "chainconfig/resources/config-storage.properties.template" "$TARGET_OPT/chainconfig/resources/config-storage.properties"
[ -f "chainconfig/resources/config-user.properties.template" ] && \
    cp "chainconfig/resources/config-user.properties.template" "$TARGET_OPT/chainconfig/resources/config-user.properties"
[ -f "chainconfig/resources/config-manager.properties.template" ] && \
    cp "chainconfig/resources/config-manager.properties.template" "$TARGET_OPT/chainconfig/resources/config-manager.properties"

# Remove all .template files from bundle package so only non-template configuration files remain
rm -f "$TARGET_OPT/chainconfig/resources"/*.template

# Genesis bootstrap data & seed package
if [ -d "chainconfig/genesis_seed" ]; then
    cp -R chainconfig/genesis_seed "$TARGET_OPT/chainconfig/"
    cp "chainconfig/genesis_seed/00000/00001.dat" "$TARGET_OPT/chainconfig/data/00000/" 2>/dev/null || true
    cp "chainconfig/genesis_seed/00000/hashes.dat" "$TARGET_OPT/chainconfig/data/00000/" 2>/dev/null || true
    [ -f "chainconfig/genesis_seed/index.dat" ] && cp "chainconfig/genesis_seed/index.dat" "$TARGET_OPT/chainconfig/data/"
elif [ -f "chainconfig/data/00000/00001.dat" ]; then
    cp "chainconfig/data/00000/00001.dat" "$TARGET_OPT/chainconfig/data/00000/"
    cp "chainconfig/data/00000/hashes.dat" "$TARGET_OPT/chainconfig/data/00000/"
    [ -f "chainconfig/data/index.dat" ] && cp "chainconfig/data/index.dat" "$TARGET_OPT/chainconfig/data/"
fi

# Include launcher helper scripts (both at root of package and in scripts/linux)
cp scripts/linux/start.sh scripts/linux/stop.sh scripts/linux/restart.sh scripts/linux/start-node.sh "$TARGET_OPT/"
[ -f "scripts/linux/run.sh" ] && cp scripts/linux/run.sh "$TARGET_OPT/"
[ -f "scripts/linux/reset_to_genesis.sh" ] && cp scripts/linux/reset_to_genesis.sh "$TARGET_OPT/"

mkdir -p "$TARGET_OPT/scripts/linux"
cp -R scripts/linux/* "$TARGET_OPT/scripts/linux/"
rm -f "$TARGET_OPT/scripts/linux/package-linux.sh" "$TARGET_OPT/scripts/linux/test_"*.sh

chmod +x "$TARGET_OPT"/*.sh "$TARGET_OPT/sirius-core" "$TARGET_OPT/scripts/linux"/*.sh 2>/dev/null || true

# 4. Create standalone tarball
TARBALL_NAME="proximax-sirius-linux-${ARCH}-${VERSION}.tar.gz"
echo "-> Creating release archive: ${TARBALL_NAME}..."
tar -czf "$DIST_DIR/${TARBALL_NAME}" -C "$BUILD_DIR/opt" proximax-sirius-core

# 5. Build Debian package (.deb)
echo "-> Assembling Debian package..."
DEB_STAGE="$BUILD_DIR/deb-stage"
mkdir -p "$DEB_STAGE/DEBIAN"
mkdir -p "$DEB_STAGE/opt"
mkdir -p "$DEB_STAGE/etc/systemd/system"

cp -R "$BUILD_DIR/opt/proximax-sirius-core" "$DEB_STAGE/opt/"
cp scripts/linux/systemd/proximax-sirius.service "$DEB_STAGE/etc/systemd/system/"

# Control file
cat << EOF > "$DEB_STAGE/DEBIAN/control"
Package: proximax-sirius-core
Version: ${VERSION}
Section: net
Priority: optional
Architecture: ${ARCH}
Maintainer: ProximaX Foundation <support@proximax.io>
Description: ProximaX Sirius Mainnet Standalone Peer Node & Cockpit
 High-performance peer validator node with native web cockpit for POS+ block harvesting.
EOF

# Post-install script (creates unprivileged user and sets permissions)
cat << 'EOF' > "$DEB_STAGE/DEBIAN/postinst"
#!/bin/sh
set -e
if ! getent group sirius >/dev/null; then
    groupadd --system sirius
fi
if ! getent passwd sirius >/dev/null; then
    useradd --system --gid sirius --home-dir /opt/proximax-sirius-core --no-create-home --shell /usr/sbin/nologin sirius
fi
chown -R sirius:sirius /opt/proximax-sirius-core
chmod 0750 /opt/proximax-sirius-core
chmod 0600 /opt/proximax-sirius-core/chainconfig/resources/*.properties || true
systemctl daemon-reload
echo "ProximaX Sirius Core installed. Enable and start service via:"
echo "  sudo systemctl enable --now proximax-sirius"
exit 0
EOF
chmod 0755 "$DEB_STAGE/DEBIAN/postinst"

# Pre-removal script
cat << 'EOF' > "$DEB_STAGE/DEBIAN/prerm"
#!/bin/sh
set -e
if systemctl is-active --quiet proximax-sirius; then
    systemctl stop proximax-sirius
fi
exit 0
EOF
chmod 0755 "$DEB_STAGE/DEBIAN/prerm"

if command -v dpkg-deb >/dev/null 2>&1; then
    DEB_NAME="proximax-sirius-core_${VERSION}_${ARCH}.deb"
    dpkg-deb --build "$DEB_STAGE" "$DIST_DIR/${DEB_NAME}"
    echo "✓ Built Debian package: $DIST_DIR/${DEB_NAME}"
else
    echo "ℹ Note: dpkg-deb not found on host; Debian staging tree preserved at $DEB_STAGE"
fi

echo "========================================================="
echo "  Linux Packaging Complete!"
echo "  Tarball: $DIST_DIR/${TARBALL_NAME}"
echo "========================================================="
