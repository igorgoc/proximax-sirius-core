#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR"

# 1. Clear macOS Gatekeeper quarantine recursively on all files in this directory
if [ "$(uname)" = "Darwin" ]; then
    xattr -cr "$DIR" 2>/dev/null || true
fi

# 2. Ensure binaries and scripts are executable
chmod +x "$DIR/sirius-core" "$DIR"/*.sh "$DIR"/*.command 2>/dev/null || true
[ -f "$DIR/bin/sirius.bc" ] && chmod +x "$DIR/bin/sirius.bc" 2>/dev/null || true

echo "========================================================="
echo "  ProximaX Sirius Core Standalone Node Manager"
echo "  Architecture: $(uname -s) $(uname -m)"
echo "========================================================="
echo "-> Starting Sirius Core Web Manager..."
echo "-> Access URL: http://localhost:8080"
echo "-> Press Ctrl+C in this terminal window to stop."
echo "========================================================="

# 3. Automatically open default web browser after 1 second
( sleep 1 && open "http://localhost:8080" 2>/dev/null || true ) &

# 4. Launch sirius-core manager
exec "$DIR/sirius-core" -port 8080 -chainconfig "$DIR/chainconfig"
