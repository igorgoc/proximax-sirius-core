#!/usr/bin/env bash
set -euo pipefail

# setup-engine-sandbox.sh
# Prepares a lightweight sandboxed environment for headless sirius.bc execution on CI runners.

SANDBOX_DIR="${1:-/tmp/sirius-engine-sandbox}"
ENGINE_VERSION="${2:-1.9.12}"

echo "=================================================================="
echo "  Setting up Sirius Catapult Engine Sandbox in: $SANDBOX_DIR"
echo "  Target Release Version: $ENGINE_VERSION"
echo "=================================================================="

mkdir -p "$SANDBOX_DIR/bin" "$SANDBOX_DIR/resources/delegated_keys" "$SANDBOX_DIR/data/00000" "$SANDBOX_DIR/logs"

# 1. Download or link Sirius Linux amd64 binary if on Linux
if [ "$(uname -s)" = "Linux" ] && [ "$(uname -m)" = "x86_64" ]; then
    if [ ! -f "$SANDBOX_DIR/bin/sirius.bc" ]; then
        echo "📥 Downloading precompiled Sirius Catapult binaries (v$ENGINE_VERSION)..."
        DOWNLOAD_URL="https://github.com/igorgoc/cpp-xpx-chain/releases/download/v${ENGINE_VERSION}/sirius-linux-amd64.tar.gz"
        FALLBACK_URL="https://github.com/igorgoc/cpp-xpx-chain/releases/latest/download/sirius-linux-amd64.tar.gz"

        (curl -f -sSL "$DOWNLOAD_URL" || curl -f -sSL "$FALLBACK_URL") | tar -xz -C "$SANDBOX_DIR"
        chmod +x "$SANDBOX_DIR/bin/sirius.bc" 2>/dev/null || true
        chmod +x "$SANDBOX_DIR/bin/catapult.recovery" 2>/dev/null || true
    fi

    # Ensure libatomic.so.1 and RocksDB links
    if [ -f "$SANDBOX_DIR/bin/librocksdb.so.8.5.3" ]; then
        ln -sf librocksdb.so.8.5.3 "$SANDBOX_DIR/bin/librocksdb.so.8" 2>/dev/null || true
        ln -sf librocksdb.so.8.5.3 "$SANDBOX_DIR/bin/librocksdb.so" 2>/dev/null || true
    fi
fi

# 2. Copy nemesis blocks from chainconfig
NEMESIS_SRC="./chainconfig/data"
if [ -d "$NEMESIS_SRC/00000" ]; then
    cp -f "$NEMESIS_SRC/00000"/00001.dat "$SANDBOX_DIR/data/00000/" 2>/dev/null || true
    cp -f "$NEMESIS_SRC/00000"/hashes.dat "$SANDBOX_DIR/data/00000/" 2>/dev/null || true
    cp -f "$NEMESIS_SRC"/index.dat "$SANDBOX_DIR/data/" 2>/dev/null || true
fi

# 3. Copy base config properties
if [ -d "./chainconfig/resources" ]; then
    cp -rf ./chainconfig/resources/* "$SANDBOX_DIR/resources/" 2>/dev/null || true
fi

# Ensure delegated_keys folder exists with 0700 permissions
mkdir -p "$SANDBOX_DIR/resources/delegated_keys"
chmod 700 "$SANDBOX_DIR/resources/delegated_keys"

# Seed primary harvest key
cat << 'EOF' > "$SANDBOX_DIR/resources/delegated_keys/primary.key"
B441F9CEF141F909FFEBCEEEB88F0654CA571871C67E7D9747CE075DE5F3DB5D
EOF
chmod 600 "$SANDBOX_DIR/resources/delegated_keys/primary.key"

echo "✅ Sirius Sandbox initialized at $SANDBOX_DIR"
