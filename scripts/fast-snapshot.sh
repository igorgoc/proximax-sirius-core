#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
DATA_DIR="$DIR/chainconfig/data"
SNAPSHOT_URL="${1:-http://207.180.195.181/snapshot.tar.xz}"

echo "================================================================="
echo "  ProximaX Sirius Core - Native Host Fast Sync Stream"
echo "================================================================="
echo "Target directory: $DATA_DIR"
echo "Snapshot URL:     $SNAPSHOT_URL"
echo ""

mkdir -p "$DATA_DIR"

echo "[1/4] Stopping Sirius Core containers to release file locks..."
cd "$DIR" && docker compose stop sirius-core || true

echo "[2/4] Removing stale server.lock..."
rm -f "$DATA_DIR/server.lock"

echo "[3/4] Streaming and decompressing snapshot directly to macOS disk..."
echo "      (Zero intermediate archive storage)"
if command -v curl &> /dev/null; then
    curl -L --progress-bar "$SNAPSHOT_URL" | tar -xf - -C "$DIR/chainconfig" -m -b 2048
elif command -v wget &> /dev/null; then
    wget -qO- "$SNAPSHOT_URL" | tar -xf - -C "$DIR/chainconfig" -m -b 2048
else
    echo "Error: neither curl nor wget found on host system."
    exit 1
fi

echo "[4/4] Restarting Sirius Core Web Dashboard..."
rm -f "$DATA_DIR/server.lock"
docker compose start sirius-core || docker compose up -d

echo ""
echo "================================================================="
echo "  Snapshot extraction complete! Blockchain data is ready."
echo "  Open dashboard: http://localhost:3080"
echo "================================================================="
