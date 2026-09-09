#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR"

# Detect Platform Architecture
OS_NAME="$(uname -s)"
ARCH_NAME="$(uname -m)"
PLATFORM_DESC="$OS_NAME $ARCH_NAME"
if [ "$OS_NAME" = "Darwin" ] && [ "$ARCH_NAME" = "arm64" ]; then
    PLATFORM_DESC="Apple Silicon ARM64"
elif [ "$OS_NAME" = "Linux" ] && [ "$ARCH_NAME" = "x86_64" ]; then
    PLATFORM_DESC="Linux x86_64"
fi

echo "========================================================="
echo "  ProximaX Sirius Mainnet Peer Node (Standalone Native)"
echo "  Architecture: $PLATFORM_DESC (Zero Docker)"
echo "========================================================="

# 1. Raise file limits for RocksDB state cache
ulimit -n 65536 2>/dev/null || true

# 2. Ensure logs, data, and bin directories exist
mkdir -p "$DIR/chainconfig/logs" "$DIR/chainconfig/data" "$DIR/bin"

# 2. Verify binaries exist in bin/
if [ ! -f "$DIR/bin/sirius.bc" ]; then
    echo "Error: bin/sirius.bc native binary not found!"
    exit 1
fi

# 3. Toolchain environment setup
if [ -d "$HOME/node/bin" ]; then
    export PATH="$HOME/node/bin:$PATH"
fi
if [ -d "$HOME/go1.24/bin" ]; then
    export PATH="$HOME/go1.24/bin:$PATH"
fi
if [ -f "$HOME/.nvm/nvm.sh" ]; then
    source "$HOME/.nvm/nvm.sh"
    nvm use 20 2>/dev/null || true
fi

echo "Building React UI and native Go backend..."
(cd "$DIR/frontend" && npm run build)
(cd "$DIR/backend" && go build -o sirius-core .)

# 4. Stop any existing background instance
if [ -f "$DIR/.sirius-core.pid" ]; then
    PID=$(cat "$DIR/.sirius-core.pid")
    if kill -0 "$PID" 2>/dev/null; then
        echo "Stopping existing instance (PID $PID)..."
        kill -INT "$PID" 2>/dev/null || true
        sleep 2
    fi
    rm -f "$DIR/.sirius-core.pid"
fi

# Also ensure port 3080 is free
OLD_PID=$(lsof -ti :3080 2>/dev/null || true)
if [ -n "$OLD_PID" ]; then
    echo "Freeing port 3080 (killing PID $OLD_PID)..."
    kill -9 $OLD_PID 2>/dev/null || true
    sleep 1
fi

# 5. Start native manager in background
echo "Starting Sirius Core Native Manager on port 3080..."
DYLD_LIBRARY_PATH="$DIR/bin" LD_LIBRARY_PATH="$DIR/bin" "$DIR/backend/sirius-core" -port 3080 -chainconfig "$DIR/chainconfig" > "$DIR/chainconfig/logs/manager.log" 2>&1 &
NEW_PID=$!
echo "$NEW_PID" > "$DIR/.sirius-core.pid"

# 6. Wait for HTTP readiness
echo "Waiting for Web Dashboard to become ready..."
READY=false
for i in {1..20}; do
    if curl -s -f -o /dev/null "http://127.0.0.1:3080/"; then
        READY=true
        break
    fi
    sleep 0.5
done

echo ""
echo "========================================================="
if [ "$READY" = true ]; then
    echo "  ProximaX Sirius Native Node is ONLINE (PID: $NEW_PID)!"
    echo ""
    echo "  Access the GUI Dashboard in your browser:"
    echo "    >>> http://localhost:3080 <<<"
    echo ""
    echo "  Logs:   tail -f chainconfig/logs/manager.log"
    echo "  Stop:   ./stop.sh"
else
    echo "  Notice: Manager started with PID $NEW_PID, check logs:"
    echo "  tail -n 20 chainconfig/logs/manager.log"
fi
echo "========================================================="
