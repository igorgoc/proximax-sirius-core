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

# 2. Check if already running or free port if occupied by stale process
PORT="${PORT:-8080}"
STOP_CMD="./scripts/linux/stop.sh"
[ -f "$DIR/stop.sh" ] && STOP_CMD="./stop.sh"

if curl -s -f "http://127.0.0.1:$PORT/api/status" >/dev/null 2>&1; then
    echo "========================================================="
    echo "  Sirius Core Web Manager is ALREADY RUNNING on port $PORT!"
    echo "  Web Dashboard: http://localhost:$PORT"
    echo "  To stop:       $STOP_CMD"
    echo "========================================================="
    if command -v xdg-open >/dev/null 2>&1 && [ -n "$DISPLAY" ]; then
        xdg-open "http://localhost:$PORT" 2>/dev/null || true
    fi
    exit 0
fi

OLD_PID=$(lsof -ti :$PORT 2>/dev/null || true)
if [ -n "$OLD_PID" ]; then
    echo "-> Freeing occupied port $PORT (PID $OLD_PID)..."
    kill -INT "$OLD_PID" 2>/dev/null || true
    sleep 1
    if kill -0 "$OLD_PID" 2>/dev/null; then
        kill -9 "$OLD_PID" 2>/dev/null || true
        sleep 1
    fi
fi

# 3. Ensure Sirius Catapult engine and shared libraries exist and match platform architecture
if [ "$(uname -s)" = "Darwin" ]; then
    if [ ! -f "$DIR/bin/sirius.bc" ] || ! file -b "$DIR/bin/sirius.bc" | grep -q "Mach-O" || [ ! -f "$DIR/bin/libcatapult.plugins.config.dylib" ]; then
        echo "-> Catapult macOS arm64 engine or libraries missing/incomplete. Downloading native binaries..."
        mkdir -p "$DIR/bin"
        if command -v curl >/dev/null 2>&1; then
            (curl -f -sSL "https://github.com/igorgoc/cpp-xpx-chain/releases/latest/download/sirius-darwin-arm64.tar.gz" || \
             curl -f -sSL "https://github.com/igorgoc/cpp-xpx-chain/releases/download/1.9.9/sirius-darwin-arm64.tar.gz" || \
             curl -f -sSL "https://github.com/igorgoc/cpp-xpx-chain/releases/download/1.9.8/sirius-darwin-arm64.tar.gz") | tar -xz -C "$DIR"
            echo "-> Sirius macOS engine unpacked successfully."
        fi
    fi
elif [ "$(uname -s)" = "Linux" ]; then
    ARCH="$(uname -m)"
    if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
        ENGINE_PKG="sirius-linux-arm64.tar.gz"
    else
        ENGINE_PKG="sirius-linux-amd64.tar.gz"
    fi
    if [ ! -f "$DIR/bin/sirius.bc" ] || ! file -b "$DIR/bin/sirius.bc" | grep -q "ELF" || [ ! -f "$DIR/bin/libcatapult.plugins.config.so" ]; then
        echo "-> Catapult Linux engine ($ARCH) or libraries missing/incomplete. Downloading precompiled Sirius Linux binaries..."
        mkdir -p "$DIR/bin"
        if command -v curl >/dev/null 2>&1; then
            (curl -f -sSL "https://github.com/igorgoc/cpp-xpx-chain/releases/latest/download/${ENGINE_PKG}" || \
             curl -f -sSL "https://github.com/igorgoc/cpp-xpx-chain/releases/download/1.9.9/${ENGINE_PKG}" || \
             curl -f -sSL "https://github.com/igorgoc/cpp-xpx-chain/releases/download/1.9.8/${ENGINE_PKG}") | tar -xz -C "$DIR"
            echo "-> Catapult engine unpacked successfully."
        fi
    fi
fi

# 3.1 Ensure genesis nemesis block exists and has valid size
mkdir -p "$DIR/chainconfig/data/00000"
if [ ! -s "$DIR/chainconfig/data/00000/00001.dat" ] || [ ! -s "$DIR/chainconfig/data/00000/hashes.dat" ]; then
    if [ -s "$DIR/chainconfig/genesis_seed/00000/00001.dat" ] && [ -s "$DIR/chainconfig/genesis_seed/00000/hashes.dat" ]; then
        echo "-> Seeding official genesis block from genesis_seed..."
        cp -p "$DIR/chainconfig/genesis_seed/00000/00001.dat" "$DIR/chainconfig/data/00000/00001.dat"
        cp -p "$DIR/chainconfig/genesis_seed/00000/hashes.dat" "$DIR/chainconfig/data/00000/hashes.dat"
        [ -s "$DIR/chainconfig/genesis_seed/index.dat" ] && cp -p "$DIR/chainconfig/genesis_seed/index.dat" "$DIR/chainconfig/data/index.dat"
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

FOREGROUND=false
for arg in "$@"; do
    if [ "$arg" = "--foreground" ] || [ "$arg" = "-f" ]; then
        FOREGROUND=true
        break
    fi
done

if [ "$FOREGROUND" = true ]; then
    echo "========================================================="
    echo "  ProximaX Sirius Core Standalone Node Manager"
    echo "  Architecture: $(uname -s) $(uname -m) (Foreground Mode)"
    echo "========================================================="
    echo "-> Starting Sirius Core Web Manager on port $PORT..."
    echo "-> Web Dashboard: http://localhost:$PORT"
    echo "-> To stop: Press Ctrl+C or run $STOP_CMD"
    echo "========================================================="
    exec "$SIRIUS_CORE_BIN" -port "$PORT" -chainconfig "$DIR/chainconfig" "$@"
else
    LOGS_DIR="$DIR/chainconfig/logs"
    mkdir -p "$LOGS_DIR"
    LOG_FILE="$LOGS_DIR/manager.log"

    echo "-> Starting Sirius Core Web Manager in background on port $PORT..."
    nohup "$SIRIUS_CORE_BIN" -port "$PORT" -chainconfig "$DIR/chainconfig" "$@" >> "$LOG_FILE" 2>&1 &
    NEW_PID=$!
    echo "$NEW_PID" > "$DIR/.sirius-core.pid"

    # Wait for Web Dashboard readiness
    READY=false
    for i in {1..30}; do
        if curl -s -f -o /dev/null "http://127.0.0.1:$PORT/api/status" 2>/dev/null || curl -s -f -o /dev/null "http://127.0.0.1:$PORT/" 2>/dev/null; then
            READY=true
            break
        fi
        if ! kill -0 "$NEW_PID" 2>/dev/null; then
            echo "ERROR: Sirius Core failed to start. Check logs: tail -n 25 $LOG_FILE" >&2
            tail -n 25 "$LOG_FILE" >&2
            rm -f "$DIR/.sirius-core.pid"
            exit 1
        fi
        sleep 0.2
    done

    echo "========================================================="
    echo "  ProximaX Sirius Core Node Manager is ONLINE (PID: $NEW_PID)!"
    echo "  Architecture: $(uname -s) $(uname -m) (Background Task)"
    echo ""
    echo "  Web Dashboard: http://localhost:$PORT"
    echo "  View Logs:     tail -f $LOG_FILE"
    echo "  Stop Node:     $STOP_CMD"
    echo "========================================================="
    exit 0
fi
