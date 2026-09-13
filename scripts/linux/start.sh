#!/bin/bash
set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
if [ -d "$SCRIPT_DIR/chainconfig" ]; then
    DIR="$SCRIPT_DIR"
elif [ -d "$SCRIPT_DIR/../chainconfig" ]; then
    DIR="$( cd "$SCRIPT_DIR/.." && pwd )"
elif [ -d "$SCRIPT_DIR/../../chainconfig" ]; then
    DIR="$( cd "$SCRIPT_DIR/../.." && pwd )"
else
    DIR="$SCRIPT_DIR"
fi
cd "$DIR"

# 1. Clear macOS Gatekeeper quarantine if on Darwin
if [ "$(uname)" = "Darwin" ]; then
    xattr -cr "$DIR" 2>/dev/null || true
fi

# 2. Free port 8080 if already held by an existing instance
OLD_PID=$(lsof -ti :8080 2>/dev/null || true)
if [ -n "$OLD_PID" ]; then
    echo "-> Stopping existing instance on port 8080 (PID $OLD_PID)..."
    kill -INT "$OLD_PID" 2>/dev/null || true
    sleep 1
    if kill -0 "$OLD_PID" 2>/dev/null; then
        kill -9 "$OLD_PID" 2>/dev/null || true
        sleep 1
    fi
fi

# 3. Ensure Sirius Catapult engine and shared libraries exist and match platform architecture
if [ "$(uname -s)" = "Darwin" ]; then
    if [ ! -f "$DIR/bin/sirius.bc" ] || ! file -b "$DIR/bin/sirius.bc" | grep -q "Mach-O"; then
        echo "-> Catapult macOS arm64 engine missing or invalid format. Downloading native binaries..."
        mkdir -p "$DIR/bin"
        if command -v curl >/dev/null 2>&1; then
            curl -f -sSL "https://github.com/igorgoc/cpp-xpx-chain/releases/download/v1.9.8/sirius-darwin-arm64.tar.gz" | tar -xz -C "$DIR"
            echo "-> Sirius macOS engine unpacked successfully."
        fi
    fi
elif [ "$(uname -s)" = "Linux" ]; then
    if [ ! -f "$DIR/bin/sirius.bc" ] || ! file -b "$DIR/bin/sirius.bc" | grep -q "ELF"; then
        echo "-> Catapult Linux engine missing or invalid format. Downloading precompiled Sirius Linux binaries..."
        mkdir -p "$DIR/bin"
        if command -v curl >/dev/null 2>&1; then
            curl -f -sSL "https://github.com/igorgoc/cpp-xpx-chain/releases/download/v1.9.8/sirius-linux-amd64.tar.gz" | tar -xz -C "$DIR"
            echo "-> Catapult engine unpacked successfully."
        fi
    fi
fi

# 4. Resolve or build the sirius-core binary
SIRIUS_CORE_BIN=""
if [ -f "$DIR/bin/linux/sirius-core" ]; then
    SIRIUS_CORE_BIN="$DIR/bin/linux/sirius-core"
elif [ -f "$DIR/bin/sirius-core" ]; then
    SIRIUS_CORE_BIN="$DIR/bin/sirius-core"
elif [ -f "$DIR/sirius-core" ]; then
    SIRIUS_CORE_BIN="$DIR/sirius-core"
elif [ -f "$DIR/backend/sirius-core" ]; then
    SIRIUS_CORE_BIN="$DIR/backend/sirius-core"
elif [ -d "$DIR/backend" ] && command -v go >/dev/null 2>&1; then
    echo "-> Compiling native Go manager binary..."
    mkdir -p "$DIR/bin/linux"
    (cd "$DIR/backend" && go build -ldflags="-s -w" -o "$DIR/bin/linux/sirius-core" .)
    SIRIUS_CORE_BIN="$DIR/bin/linux/sirius-core"
fi

if [ -z "$SIRIUS_CORE_BIN" ] || [ ! -f "$SIRIUS_CORE_BIN" ]; then
    echo "ERROR: sirius-core binary not found!" >&2
    echo "Please build with './scripts/linux/run.sh' or download a release package." >&2
    exit 1
fi

# 5. Ensure binaries and scripts are executable
chmod +x "$SIRIUS_CORE_BIN" 2>/dev/null || true
[ -d "$DIR/scripts/linux" ] && chmod +x "$DIR/scripts/linux"/*.sh 2>/dev/null || true
[ -f "$DIR/sirius-core" ] && chmod +x "$DIR/sirius-core" 2>/dev/null || true
[ -f "$DIR/bin/sirius.bc" ] && chmod +x "$DIR/bin/sirius.bc" 2>/dev/null || true

# 6. Configure dynamic library search paths
export DYLD_LIBRARY_PATH="$DIR/bin:${DYLD_LIBRARY_PATH:-}"
export LD_LIBRARY_PATH="$DIR/bin:${LD_LIBRARY_PATH:-}"

echo "========================================================="
echo "  ProximaX Sirius Core Standalone Node Manager"
echo "  Architecture: $(uname -s) $(uname -m)"
echo "========================================================="
echo "-> Starting Sirius Core Web Manager on port 8080..."
echo "-> Web Dashboard: http://localhost:8080"
echo "-> To stop: Press Ctrl+C or run ./scripts/linux/stop.sh"
echo "========================================================="

exec "$SIRIUS_CORE_BIN" -port 8080 -chainconfig "$DIR/chainconfig" "$@"
