#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR"

echo "========================================================="
echo "  ProximaX Sirius Standalone Native Node Launcher"
echo "  Architecture: Apple Silicon ARM64 (Optimized Async WAL)"
echo "========================================================="

# 1. Raise file descriptor limit for RocksDB multi-cache
ulimit -n 65536 2>/dev/null || true
echo "-> File descriptor limit set to: $(ulimit -n)"

# 2. Stop any conflicting background Docker container on port 7900
if command -v docker >/dev/null 2>&1; then
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "sirius-mainnet-peer"; then
        echo "-> Stopping conflicting background Docker container (sirius-mainnet-peer)..."
        docker stop sirius-mainnet-peer >/dev/null 2>&1 || true
    fi
fi

# 3. Check for rogue processes holding ports 7900, 7901, 7903
for port in 7900 7901 7903; do
    PORT_PID=$(lsof -ti :$port 2>/dev/null || true)
    if [ -n "$PORT_PID" ]; then
        echo "-> Freeing occupied port $port (killing process $PORT_PID)..."
        kill -9 $PORT_PID 2>/dev/null || true
    fi
done

# 4. Resolve data directory path from config-user.properties
DATA_DIR="$DIR/chainconfig/data"
USER_PROPS="$DIR/chainconfig/resources/config-user.properties"
if [ -f "$USER_PROPS" ]; then
    CUSTOM_DATA=$(grep -E "^[[:space:]]*data.path[[:space:]]*=" "$USER_PROPS" | cut -d'=' -f2- | tr -d "[:space:]\"'")
    if [ -n "$CUSTOM_DATA" ]; then
        if [[ "$CUSTOM_DATA" = /* ]]; then
            DATA_DIR="$CUSTOM_DATA"
        else
            DATA_DIR="$DIR/$CUSTOM_DATA"
        fi
    fi
fi
echo "-> Blockchain data directory: $DATA_DIR"

# 5. Unlock and clear stale lock files
if [ -d "$DATA_DIR" ]; then
    chmod 777 "$DATA_DIR"/*.lock 2>/dev/null || true
    rm -f "$DATA_DIR"/*.lock 2>/dev/null || true
    chmod 777 "$DATA_DIR"/statedb/*/LOCK 2>/dev/null || true
    rm -f "$DATA_DIR"/statedb/*/LOCK 2>/dev/null || true
fi

# 6. Configure DYLD dynamic library path
BOOST_LIB="$HOME/boost-build-1.81.0/lib"
DYLD_PATH="$DIR/bin"
if [ -d "$BOOST_LIB" ]; then
    DYLD_PATH="$DYLD_PATH:$BOOST_LIB"
fi
if [ -n "$BOOST_ROOT" ] && [ -d "$BOOST_ROOT/lib" ]; then
    DYLD_PATH="$DYLD_PATH:$BOOST_ROOT/lib"
fi
export DYLD_LIBRARY_PATH="$DYLD_PATH"
export LD_LIBRARY_PATH="$DYLD_PATH"

# 7. Start Sirius Core native binary
echo "========================================================="
echo "  Starting sirius.bc engine..."
echo "  Press Ctrl+C to gracefully stop the node at any time."
echo "========================================================="

exec "$DIR/bin/sirius.bc" "$DIR/chainconfig"
