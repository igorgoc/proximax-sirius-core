#!/usr/bin/env bash
set -e

# Load Home Assistant bashio library if available
if [ -f /usr/lib/bashio/bashio.sh ]; then
    source /usr/lib/bashio/bashio.sh
    LOG_INFO() { bashio::log.info "$@"; }
    LOG_WARN() { bashio::log.warning "$@"; }
    LOG_ERR() { bashio::log.error "$@"; }
    GET_CONFIG() { bashio::config "$1"; }
else
    LOG_INFO() { echo "[INFO] $@"; }
    LOG_WARN() { echo "[WARN] $@"; }
    LOG_ERR() { echo "[ERROR] $@"; }
    GET_CONFIG() {
        if [ -f /data/options.json ]; then
            jq -r ".$1 // empty" /data/options.json
        else
            echo ""
        fi
    }
fi

LOG_INFO "========================================================="
LOG_INFO "  Starting ProximaX Sirius Validator (Home Assistant)    "
LOG_INFO "========================================================="

# 1. Read configuration options from Home Assistant
BOOT_KEY=$(GET_CONFIG 'boot_key')
HARVEST_KEY=$(GET_CONFIG 'harvest_key')
FRIENDLY_NAME=$(GET_CONFIG 'friendly_name')
FAST_SYNC=$(GET_CONFIG 'fast_sync')
CUSTOM_SNAPSHOT_URL=$(GET_CONFIG 'custom_snapshot_url')

[ -z "$FRIENDLY_NAME" ] && FRIENDLY_NAME="HomeAssistant-Validator"
[ -z "$FAST_SYNC" ] && FAST_SYNC="true"

# 2. Setup persistent directory structure on SSD (/data)
DATA_DIR="/data/chainconfig"
mkdir -p "$DATA_DIR/resources" "$DATA_DIR/data/00000" "$DATA_DIR/logs" "$DATA_DIR/bin"

# Copy baseline configuration templates if not present
if [ ! -f "$DATA_DIR/resources/config-network.properties" ]; then
    LOG_INFO "Initializing baseline chain configurations in $DATA_DIR/resources..."
    cp -R /etc/sirius/chainconfig/resources/* "$DATA_DIR/resources/" 2>/dev/null || true
fi

# 3. Apply key configurations securely
if [ -z "$BOOT_KEY" ] || [ -z "$HARVEST_KEY" ]; then
    LOG_WARN "WARNING: boot_key or harvest_key is not configured yet!"
    LOG_WARN "Please enter your keys in the Add-on Configuration tab."
fi

# Update config-user.properties
USER_CONF="$DATA_DIR/resources/config-user.properties"
if [ ! -f "$USER_CONF" ]; then
    cp /etc/sirius/chainconfig/resources/config-user.properties.template "$USER_CONF" 2>/dev/null || touch "$USER_CONF"
fi
[ -n "$BOOT_KEY" ] && sed -i "s|^bootKey *=.*|bootKey = ${BOOT_KEY}|" "$USER_CONF" 2>/dev/null || true
sed -i "s|^friendlyName *=.*|friendlyName = ${FRIENDLY_NAME}|" "$USER_CONF" 2>/dev/null || echo "friendlyName = ${FRIENDLY_NAME}" >> "$USER_CONF"
sed -i "s|^dataDirectory *=.*|dataDirectory = ${DATA_DIR}/data|" "$USER_CONF" 2>/dev/null || echo "dataDirectory = ${DATA_DIR}/data" >> "$USER_CONF"

# Update config-harvesting.properties with 0600 permissions
HARVEST_CONF="$DATA_DIR/resources/config-harvesting.properties"
if [ ! -f "$HARVEST_CONF" ]; then
    cp /etc/sirius/chainconfig/resources/config-harvesting.properties.template "$HARVEST_CONF" 2>/dev/null || touch "$HARVEST_CONF"
fi
[ -n "$HARVEST_KEY" ] && sed -i "s|^harvestKey *=.*|harvestKey = ${HARVEST_KEY}|" "$HARVEST_CONF" 2>/dev/null || true
sed -i "s|^isAutoHarvestingEnabled *=.*|isAutoHarvestingEnabled = true|" "$HARVEST_CONF" 2>/dev/null || echo "isAutoHarvestingEnabled = true" >> "$HARVEST_CONF"
sed -i "s|^maxUnlockedAccounts *=.*|maxUnlockedAccounts = 1|" "$HARVEST_CONF" 2>/dev/null || echo "maxUnlockedAccounts = 1" >> "$HARVEST_CONF"

chmod 0600 "$DATA_DIR/resources/"*.properties 2>/dev/null || true

# 4. Check & Perform Fast-Sync Snapshot if data directory is empty
DATA_STORAGE="$DATA_DIR/data"
INDEX_FILE="$DATA_STORAGE/index.dat"
BLOCK_GENESIS="$DATA_STORAGE/00000/00001.dat"

if [ ! -s "$INDEX_FILE" ] || [ ! -s "$BLOCK_GENESIS" ]; then
    if [ "$FAST_SYNC" = "true" ]; then
        SNAPSHOT_URL="https://huggingface.co/datasets/igorgoc/sirius-snapshot/resolve/main/sirius-data-backup-2026-09-10-131735.tar.zst"
        [ -n "$CUSTOM_SNAPSHOT_URL" ] && SNAPSHOT_URL="$CUSTOM_SNAPSHOT_URL"
        
        LOG_INFO "Database is empty. Initiating direct streaming Fast-Sync restore from snapshot..."
        LOG_INFO "Streaming from: $SNAPSHOT_URL"
        
        mkdir -p "$DATA_STORAGE"
        if curl -f -sSL "$SNAPSHOT_URL" | tar --zstd -x -C "$DATA_STORAGE"; then
            LOG_INFO "Fast-Sync snapshot restored successfully onto SSD storage."
        else
            LOG_WARN "Streaming snapshot download failed; initializing genesis block seed fallback..."
            if [ -d /etc/sirius/chainconfig/genesis_seed ]; then
                cp -p /etc/sirius/chainconfig/genesis_seed/00000/00001.dat "$DATA_STORAGE/00000/" 2>/dev/null || true
                cp -p /etc/sirius/chainconfig/genesis_seed/00000/hashes.dat "$DATA_STORAGE/00000/" 2>/dev/null || true
                cp -p /etc/sirius/chainconfig/genesis_seed/index.dat "$DATA_STORAGE/" 2>/dev/null || true
            fi
        fi
    fi
fi

# 5. Clear stale lock files
rm -f "$DATA_STORAGE"/*.lock "$DATA_STORAGE"/statedb/*/LOCK 2>/dev/null || true

# 6. Locate or auto-fetch Sirius Catapult engine binary
SIRIUS_BIN="/usr/local/bin/sirius.bc"
if [ -f "/etc/sirius/bin/sirius.bc" ]; then
    SIRIUS_BIN="/etc/sirius/bin/sirius.bc"
elif [ -f "$DATA_DIR/bin/sirius.bc" ]; then
    SIRIUS_BIN="$DATA_DIR/bin/sirius.bc"
elif [ -f "/usr/bin/sirius.bc" ]; then
    SIRIUS_BIN="/usr/bin/sirius.bc"
fi

if [ ! -f "$SIRIUS_BIN" ]; then
    LOG_INFO "Sirius engine not found locally. Checking for precompiled ARM64 Linux engine..."
    ENGINE_URL="https://github.com/igorgoc/cpp-xpx-chain/releases/download/v1.9.8/sirius-linux-arm64.tar.gz"
    mkdir -p "$DATA_DIR/bin"
    if curl -f -sSL "$ENGINE_URL" | tar -xz -C "$DATA_DIR"; then
        LOG_INFO "ARM64 Catapult engine unpacked successfully to $DATA_DIR/bin"
        SIRIUS_BIN="$DATA_DIR/bin/sirius.bc"
    fi
fi

export LD_LIBRARY_PATH="/usr/local/lib:/etc/sirius/bin:/etc/sirius/lib:$DATA_DIR/bin:$LD_LIBRARY_PATH"

if [ ! -f "$SIRIUS_BIN" ]; then
    LOG_ERR "Error: sirius.bc engine binary not found at $SIRIUS_BIN"
    LOG_INFO "Building or waiting for ARM64 engine binary release..."
    sleep 60
    exit 1
fi

LOG_INFO "Launching Sirius Catapult Engine ($SIRIUS_BIN) on P2P port 7900..."
cd "$DATA_DIR"
exec "$SIRIUS_BIN" -r "$DATA_DIR/resources"
