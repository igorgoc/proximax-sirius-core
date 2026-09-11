#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR"

# 1. Clear macOS Gatekeeper quarantine if on Darwin
if [ "$(uname)" = "Darwin" ]; then
    xattr -cr "$DIR" 2>/dev/null || true
fi

# 2. Ensure binaries and scripts are executable
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
fi

exec "$DIR/sirius-core" -port 8080 -chainconfig "$DIR/chainconfig"
