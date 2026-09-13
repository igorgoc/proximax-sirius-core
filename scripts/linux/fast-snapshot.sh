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

DATA_DIR="$DIR/chainconfig/data"
SNAPSHOT_URL="${1:-https://huggingface.co/datasets/igorgoc/sirius-snapshot/resolve/main/sirius-data-backup-2026-09-10-131735.tar.zst}"

echo "================================================================="
echo "  ProximaX Sirius Core - Native Host Fast Sync Stream"
echo "================================================================="
echo "Target directory: $DATA_DIR"
echo "Snapshot URL:     $SNAPSHOT_URL"
echo ""

mkdir -p "$DATA_DIR"

echo "[1/4] Stopping Sirius Core containers to release file locks..."
cd "$DIR" && docker compose stop sirius-core 2>/dev/null || true

echo "[2/4] Removing stale server.lock..."
rm -f "$DATA_DIR/server.lock"

echo "[3/4] Streaming and decompressing snapshot directly to disk..."
echo "      (Zero intermediate archive storage)"
if [[ "$SNAPSHOT_URL" == *".zst"* ]] && command -v zstd &> /dev/null; then
    curl -L --progress-bar "$SNAPSHOT_URL" | zstd -d -c | tar -xf - -C "$DIR/chainconfig" -m -b 2048
elif command -v curl &> /dev/null; then
    curl -L --progress-bar "$SNAPSHOT_URL" | tar -xf - -C "$DIR/chainconfig" -m -b 2048
elif command -v wget &> /dev/null; then
    wget -qO- "$SNAPSHOT_URL" | tar -xf - -C "$DIR/chainconfig" -m -b 2048
else
    echo "Error: neither curl nor wget found on host system."
    exit 1
fi

echo "[4/4] Restarting Sirius Core Web Dashboard..."
rm -f "$DATA_DIR/server.lock"
docker compose start sirius-core 2>/dev/null || docker compose up -d 2>/dev/null || true

echo ""
echo "================================================================="
echo "  Snapshot extraction complete! Blockchain data is ready."
echo "  Open dashboard: http://localhost:8080"
echo "================================================================="
