#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
CONFIG_USER="$DIR/chainconfig/resources/config-user.properties"
CONFIGURED_DATA_DIR=$(grep -E '^[[:space:]]*dataDirectory[[:space:]]*=' "$CONFIG_USER" 2>/dev/null | cut -d'=' -f2- | tr -d ' \r\t' || true)
TARGET_DIR="${1:-${CONFIGURED_DATA_DIR:-$DIR/chainconfig/data}}"

echo "========================================================="
echo "  ProximaX Sirius - Reset Node Data to Genesis (Block 1) "
echo "  Target Data Directory: $TARGET_DIR"
echo "========================================================="

# 1. Stop running node if active
pkill -9 -f "sirius.bc" 2>/dev/null || true
sleep 1

# 2. Clear old block data and re-create target data directories
rm -rf "$TARGET_DIR/00000" "$TARGET_DIR/spool" "$TARGET_DIR/index.dat" "$TARGET_DIR/server.lock"
mkdir -p "$TARGET_DIR/00000" "$TARGET_DIR/spool"

# 3. Copy authentic genesis files
cp "$DIR/chainconfig/genesis_seed/00000/00001.dat" "$TARGET_DIR/00000/"
cp "$DIR/chainconfig/genesis_seed/00000/hashes.dat" "$TARGET_DIR/00000/"
cp "$DIR/chainconfig/genesis_seed/index.dat" "$TARGET_DIR/"

# 4. Clean lock files and stale spools
rm -f "$TARGET_DIR/server.lock" 2>/dev/null || true
rm -rf "$TARGET_DIR/spool/"* 2>/dev/null || true

echo "-> Reset to Genesis Block 1 completed successfully."
echo "-> Data folder layout:"
ls -lh "$TARGET_DIR" "$TARGET_DIR/00000"
echo "========================================================="
