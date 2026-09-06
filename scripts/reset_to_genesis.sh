#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
TARGET_DIR="${1:-/Volumes/SSD/Sirius_data}"

echo "========================================================="
echo "  ProximaX Sirius - Reset Node Data to Genesis (Block 1) "
echo "  Target Data Directory: $TARGET_DIR"
echo "========================================================="

# 1. Stop running node if active
pkill -9 -f "sirius.bc" 2>/dev/null || true
sleep 1

# 2. Re-create target data directory
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
