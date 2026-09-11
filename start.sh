#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
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

# 3. Ensure Sirius Catapult engine and shared libraries exist on Linux/WSL
if [ "$(uname -s)" = "Linux" ]; then
    if [ ! -f "$DIR/bin/librocksdb.so.8" ] || [ ! -f "$DIR/bin/sirius.bc" ]; then
        echo "-> Catapult engine shared libraries missing. Downloading precompiled Sirius Linux binaries..."
        mkdir -p "$DIR/bin"
        if command -v curl >/dev/null 2>&1; then
            curl -f -sSL "https://github.com/igorgoc/cpp-xpx-chain/releases/download/v1.9.8/sirius-linux-amd64.tar.gz" | tar -xz -C "$DIR"
            echo "-> Catapult engine unpacked successfully."
        fi
    fi
fi

# 4. Ensure binaries and scripts are executable
chmod +x "$DIR/sirius-core" "$DIR"/*.sh 2>/dev/null || true
[ -f "$DIR/start.command" ] && chmod +x "$DIR/start.command" 2>/dev/null || true
[ -f "$DIR/bin/sirius.bc" ] && chmod +x "$DIR/bin/sirius.bc" 2>/dev/null || true

echo "========================================================="
echo "  ProximaX Sirius Core Standalone Node Manager"
echo "  Architecture: $(uname -s) $(uname -m)"
echo "========================================================="
echo "-> Starting Sirius Core Web Manager on port 8080..."
echo "-> Web Dashboard: http://localhost:8080"
echo "-> To stop: Press Ctrl+C or run ./stop.sh"
echo "========================================================="

# 3. Automatically open web browser if display/desktop available
if [ "$(uname)" = "Darwin" ]; then
    ( sleep 1 && open "http://localhost:8080" 2>/dev/null || true ) &
elif command -v xdg-open >/dev/null 2>&1 && [ -n "$DISPLAY" ]; then
    ( sleep 1 && xdg-open "http://localhost:8080" 2>/dev/null || true ) &
elif grep -qi "microsoft" /proc/version 2>/dev/null; then
    ( sleep 1 && cmd.exe /c start http://localhost:8080 2>/dev/null || true ) &
fi

exec "$DIR/sirius-core" -port 8080 -chainconfig "$DIR/chainconfig"
