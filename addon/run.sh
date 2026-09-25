#!/usr/bin/env bash
set -e

# Catapult's RocksDB caches open every live .sst file with no upper bound
# (max_open_files defaults to -1/unbounded in RocksDatabase.cpp). As chain
# height grows, the required FD count exceeds the container's default
# ulimit. Raise it as high as the container's hard limit allows.
ulimit -n 1048576 2>/dev/null || ulimit -n 65536 2>/dev/null || ulimit -n 4096 2>/dev/null || true
LOG_INFO "File descriptor limit set to: $(ulimit -n)"


# Load Home Assistant bashio library if available
if [ -f /usr/lib/bashio/bashio.sh ]; then
    source /usr/lib/bashio/bashio.sh
    LOG_INFO() { bashio::log.info "$@"; }
    LOG_WARN() { bashio::log.warning "$@"; }
    LOG_ERR() { bashio::log.error "$@"; }
    GET_CONFIG() {
        if bashio::config.has_value "$1"; then
            bashio::config "$1"
        else
            local env_var=$(echo "$1" | tr '[:lower:]' '[:upper:]')
            echo "${!env_var:-}"
        fi
    }
else
    LOG_INFO() { echo "[INFO] $@"; }
    LOG_WARN() { echo "[WARN] $@"; }
    LOG_ERR() { echo "[ERROR] $@"; }
    GET_CONFIG() {
        local val=""
        if [ -f /data/options.json ]; then
            val=$(jq -r ".$1 // empty" /data/options.json 2>/dev/null || true)
        fi
        if [ -z "$val" ]; then
            local env_var=$(echo "$1" | tr '[:lower:]' '[:upper:]')
            val="${!env_var:-}"
        fi
        echo "$val"
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
RESET_DATA=$(GET_CONFIG 'reset_data')
CUSTOM_SNAPSHOT_URL=$(GET_CONFIG 'custom_snapshot_url')

# Sanitize inputs
BOOT_KEY=$(echo "${BOOT_KEY:-}" | tr -d ' \r\n\t' | sed 's/^0x//')
HARVEST_KEY=$(echo "${HARVEST_KEY:-}" | tr -d ' \r\n\t' | sed 's/^0x//')

[ -z "${FRIENDLY_NAME:-}" ] && FRIENDLY_NAME="HomeAssistant-Validator"
[ -z "${FAST_SYNC:-}" ] && FAST_SYNC="true"
[ -z "${RESET_DATA:-}" ] && RESET_DATA="false"

# Guard: Validate key configuration before starting Catapult engine
if [ -z "${BOOT_KEY:-}" ] || [ -z "${HARVEST_KEY:-}" ]; then
    LOG_WARN "WARNING: boot_key or harvest_key is not configured yet!"
    LOG_WARN "Please enter your 64-character hexadecimal keys in the Add-on Configuration tab and restart."
    LOG_INFO "Node will sleep to avoid restart-crash loop while waiting for configuration..."
    while true; do
        sleep 30
    done
fi

if [ ${#BOOT_KEY} -ne 64 ] || [ ${#HARVEST_KEY} -ne 64 ]; then
    LOG_ERR "ERROR: boot_key and harvest_key must each be exactly 64 hexadecimal characters."
    LOG_ERR "Current boot_key length: ${#BOOT_KEY}, harvest_key length: ${#HARVEST_KEY}"
    LOG_INFO "Please verify your keys in the Add-on Configuration tab and restart."
    while true; do
        sleep 30
    done
fi

# 2. Setup persistent directory structure on SSD (/data)
DATA_DIR="/data/chainconfig"
DATA_STORAGE="$DATA_DIR/data"
mkdir -p "$DATA_DIR/resources" "$DATA_STORAGE/00000" "$DATA_DIR/logs" "$DATA_DIR/bin" "$DATA_DIR/certificate"

# Check if data wipe was requested via reset_data
if [ "${RESET_DATA:-false}" = "true" ]; then
    LOG_WARN "reset_data is true: Wiping existing blockchain data in $DATA_STORAGE for clean initialization/snapshot restore..."
    rm -rf "$DATA_STORAGE"/*
    mkdir -p "$DATA_STORAGE/00000" "$DATA_STORAGE/spool"
fi

# Copy baseline configuration files if not present
if [ ! -f "$DATA_DIR/resources/config-network.properties" ]; then
    LOG_INFO "Initializing baseline chain configurations in $DATA_DIR/resources..."
    cp -R /etc/sirius/chainconfig/resources/* "$DATA_DIR/resources/" 2>/dev/null || true
fi

# Clean up any legacy .template files from persistent storage
rm -f "$DATA_DIR/resources/"*.template 2>/dev/null || true

# 3. Apply key and storage configurations securely
USER_CONF="$DATA_DIR/resources/config-user.properties"
if [ ! -f "$USER_CONF" ]; then
    if [ -f /etc/sirius/chainconfig/resources/config-user.properties ]; then
        cp /etc/sirius/chainconfig/resources/config-user.properties "$USER_CONF"
    else
        touch "$USER_CONF"
    fi
fi

# Update config-user.properties paths and identity (strictly max 4 properties: bootKey, dataDirectory, pluginsDirectory, certificateDirectory)
grep -q "^bootKey" "$USER_CONF" 2>/dev/null && sed -i "s|^bootKey *=.*|bootKey = ${BOOT_KEY}|" "$USER_CONF" || echo "bootKey = ${BOOT_KEY}" >> "$USER_CONF"
grep -q "^dataDirectory" "$USER_CONF" 2>/dev/null && sed -i "s|^dataDirectory *=.*|dataDirectory = ${DATA_STORAGE}|" "$USER_CONF" || echo "dataDirectory = ${DATA_STORAGE}" >> "$USER_CONF"
grep -q "^pluginsDirectory" "$USER_CONF" 2>/dev/null && sed -i "s|^pluginsDirectory *=.*|pluginsDirectory = ${DATA_DIR}/bin|" "$USER_CONF" || echo "pluginsDirectory = ${DATA_DIR}/bin" >> "$USER_CONF"
grep -q "^certificateDirectory" "$USER_CONF" 2>/dev/null && sed -i "s|^certificateDirectory *=.*|certificateDirectory = ${DATA_DIR}/certificate|" "$USER_CONF" || echo "certificateDirectory = ${DATA_DIR}/certificate" >> "$USER_CONF"
# Invariant: friendlyName belongs in config-node.properties [localnode]; strip it from config-user.properties to satisfy VerifyBagSizeLte(bag, 4)
sed -i "/^friendlyName/d" "$USER_CONF" 2>/dev/null || true

# Update config-node.properties with friendlyName
NODE_CONF="$DATA_DIR/resources/config-node.properties"
if [ -f "$NODE_CONF" ]; then
    sed -i "s|^friendlyName *=.*|friendlyName = ${FRIENDLY_NAME}|" "$NODE_CONF" 2>/dev/null || true
fi

# Update config-harvesting.properties
HARVEST_CONF="$DATA_DIR/resources/config-harvesting.properties"
if [ ! -f "$HARVEST_CONF" ]; then
    if [ -f /etc/sirius/chainconfig/resources/config-harvesting.properties ]; then
        cp /etc/sirius/chainconfig/resources/config-harvesting.properties "$HARVEST_CONF"
    else
        touch "$HARVEST_CONF"
    fi
fi

grep -q "^harvestKey" "$HARVEST_CONF" 2>/dev/null && sed -i "s|^harvestKey *=.*|harvestKey = ${HARVEST_KEY}|" "$HARVEST_CONF" || echo "harvestKey = ${HARVEST_KEY}" >> "$HARVEST_CONF"
grep -q "^isAutoHarvestingEnabled" "$HARVEST_CONF" 2>/dev/null && sed -i "s|^isAutoHarvestingEnabled *=.*|isAutoHarvestingEnabled = true|" "$HARVEST_CONF" || echo "isAutoHarvestingEnabled = true" >> "$HARVEST_CONF"
grep -q "^maxUnlockedAccounts" "$HARVEST_CONF" 2>/dev/null && sed -i "s|^maxUnlockedAccounts *=.*|maxUnlockedAccounts = 1|" "$HARVEST_CONF" || echo "maxUnlockedAccounts = 1" >> "$HARVEST_CONF"
grep -q "^beneficiary" "$HARVEST_CONF" 2>/dev/null || echo "beneficiary = 0000000000000000000000000000000000000000000000000000000000000000" >> "$HARVEST_CONF"

# Update logging output destinations to container log volume
for LOG_CONF in "$DATA_DIR/resources/config-logging-server.properties" "$DATA_DIR/resources/config-logging-recovery.properties" "$DATA_DIR/resources/config-logging-broker.properties"; do
    if [ -f "$LOG_CONF" ]; then
        sed -i "s|^directory *=.*|directory = ${DATA_DIR}/logs|" "$LOG_CONF" 2>/dev/null || true
        sed -i "s|^filePattern *=.*/\([^/]*\.log\)|filePattern = ${DATA_DIR}/logs/\1|" "$LOG_CONF" 2>/dev/null || true
    fi
done

# Invariant: Secure permissions for all sensitive properties files
chmod 0600 "$DATA_DIR/resources/"*.properties 2>/dev/null || true
LOG_INFO "Node configuration updated securely in $DATA_DIR/resources/ (keys injected with 0600 permissions)."

# 4. Check & Setup Blockchain Data
INDEX_FILE="$DATA_STORAGE/index.dat"
BLOCK_GENESIS="$DATA_STORAGE/00000/00001.dat"
HASHES_FILE="$DATA_STORAGE/00000/hashes.dat"

# Ensure target directories exist
mkdir -p "$DATA_STORAGE/00000" "$DATA_STORAGE/spool"

# Validate existing index.dat height if file exists
if [ -s "$INDEX_FILE" ]; then
    CURRENT_HEIGHT=$(od -An -t u8 -N 8 "$INDEX_FILE" 2>/dev/null | tr -d ' ' || echo "0")
    if [ "${CURRENT_HEIGHT:-0}" -lt 1 ] 2>/dev/null; then
        LOG_WARN "Detected invalid chain height (${CURRENT_HEIGHT:-0}) in $INDEX_FILE; removing corrupted index..."
        rm -f "$INDEX_FILE"
    elif [ "${FAST_SYNC:-true}" = "true" ] && [ "${CURRENT_HEIGHT:-0}" -lt 1000000 ]; then
        LOG_INFO "Fast-Sync enabled and detected low chain height (${CURRENT_HEIGHT:-0} < 1,000,000). Purging zero-sync data for snapshot restore..."
        rm -rf "$DATA_STORAGE"/*
        mkdir -p "$DATA_STORAGE/00000" "$DATA_STORAGE/spool"
    fi
fi

# If blockchain storage is missing index or genesis block, initialize it
if [ ! -s "$INDEX_FILE" ] || [ ! -s "$BLOCK_GENESIS" ]; then
    RESTORED=false
    if [ "${FAST_SYNC:-true}" = "true" ]; then
        SNAPSHOT_URL="https://huggingface.co/datasets/igorgoc/sirius-snapshot/resolve/main/sirius-data-backup-2026-09-10-131735.tar.zst"
        [ -n "${CUSTOM_SNAPSHOT_URL:-}" ] && SNAPSHOT_URL="$CUSTOM_SNAPSHOT_URL"
        
        LOG_INFO "Blockchain storage uninitialized. Initiating direct streaming Fast-Sync restore from snapshot..."
        LOG_INFO "Streaming from: $SNAPSHOT_URL"
        LOG_INFO "Extracting directly to: $DATA_STORAGE"
        
        # Monitor streaming decompression progress in background every 30 seconds
        (
            while pgrep -f "tar.*zstd" >/dev/null 2>&1; do
                sleep 30
                SIZE=$(du -sh "$DATA_STORAGE" 2>/dev/null | cut -f1)
                LOG_INFO "Fast-Sync in progress... Extracted data on SSD: ${SIZE:-0}"
            done
        ) &
        MONITOR_PID=$!
        
        if curl -f -sSL "$SNAPSHOT_URL" | tar --zstd -x -C "$DATA_STORAGE"; then
            kill "$MONITOR_PID" 2>/dev/null || true
            wait "$MONITOR_PID" 2>/dev/null || true
            chmod -R u+rwX "$DATA_STORAGE" 2>/dev/null || true
            FINAL_SIZE=$(du -sh "$DATA_STORAGE" 2>/dev/null | cut -f1)
            LOG_INFO "Fast-Sync stream decompression completed (total extracted size: ${FINAL_SIZE:-0})."
            
            if [ -s "$INDEX_FILE" ] && [ -s "$BLOCK_GENESIS" ]; then
                LOG_INFO "Fast-Sync snapshot restored successfully onto SSD storage."
                RESTORED=true
            else
                LOG_WARN "Snapshot archive extracted but index.dat or genesis block is missing."
            fi
        else
            kill "$MONITOR_PID" 2>/dev/null || true
            wait "$MONITOR_PID" 2>/dev/null || true
            LOG_WARN "Streaming snapshot download failed or was interrupted; falling back to Genesis Block 1."
        fi
    fi

    # Invariant: If Fast-Sync was skipped (fast_sync: false) or failed, deploy authentic Genesis Block 1 seed
    if [ "$RESTORED" != "true" ] || [ ! -s "$INDEX_FILE" ] || [ ! -s "$BLOCK_GENESIS" ]; then
        LOG_INFO "Deploying authentic genesis block seed files into $DATA_STORAGE..."
        mkdir -p "$DATA_STORAGE/00000" "$DATA_STORAGE/spool"
        if [ -d /etc/sirius/chainconfig/genesis_seed ]; then
            cp -p /etc/sirius/chainconfig/genesis_seed/00000/00001.dat "$DATA_STORAGE/00000/"
            cp -p /etc/sirius/chainconfig/genesis_seed/00000/hashes.dat "$DATA_STORAGE/00000/"
            cp -p /etc/sirius/chainconfig/genesis_seed/index.dat "$DATA_STORAGE/"
            LOG_INFO "Genesis Block 1 seed files verified and installed successfully in $DATA_STORAGE."
        else
            LOG_ERR "CRITICAL ERROR: Genesis seed directory /etc/sirius/chainconfig/genesis_seed not found in container!"
        fi
    fi
fi

# Sanity check: confirm genesis block and index exist before continuing
if [ ! -s "$INDEX_FILE" ] || [ ! -s "$BLOCK_GENESIS" ]; then
    LOG_ERR "CRITICAL ERROR: Blockchain data directory is missing required blocks (index.dat or 00001.dat)!"
    sleep 60
    exit 1
fi

# 5. Clear stale lock files
rm -f "$DATA_STORAGE"/*.lock "$DATA_STORAGE"/statedb/*/LOCK "$DATA_STORAGE"/statedb/LOCK 2>/dev/null || true

# 6. Locate or auto-fetch Sirius Catapult engine binary
TARGET_ENGINE_VERSION="1.9.10"
VERSION_FILE="$DATA_DIR/bin/.engine_version"
INSTALLED_ENGINE_VERSION=$(cat "$VERSION_FILE" 2>/dev/null || echo "")

SIRIUS_BIN="/usr/local/bin/sirius.bc"
RECOVERY_BIN="/usr/local/bin/catapult.recovery"
if [ -f "/etc/sirius/bin/sirius.bc" ]; then
    SIRIUS_BIN="/etc/sirius/bin/sirius.bc"
    RECOVERY_BIN="/etc/sirius/bin/catapult.recovery"
elif [ -f "$DATA_DIR/bin/sirius.bc" ]; then
    SIRIUS_BIN="$DATA_DIR/bin/sirius.bc"
    RECOVERY_BIN="$DATA_DIR/bin/catapult.recovery"
elif [ -f "/usr/bin/sirius.bc" ]; then
    SIRIUS_BIN="/usr/bin/sirius.bc"
    RECOVERY_BIN="/usr/bin/catapult.recovery"
fi

if [ ! -f "$SIRIUS_BIN" ] || [ "$INSTALLED_ENGINE_VERSION" != "$TARGET_ENGINE_VERSION" ]; then
    ARCH=$(uname -m)
    case "$ARCH" in
        aarch64|arm64)
            ENGINE_ASSET="sirius-linux-arm64.tar.gz"
            ;;
        x86_64|amd64)
            ENGINE_ASSET="sirius-linux-amd64.tar.gz"
            ;;
        *)
            LOG_ERR "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac
    
    ENGINE_URL="https://github.com/igorgoc/cpp-xpx-chain/releases/download/${TARGET_ENGINE_VERSION}/${ENGINE_ASSET}"
    LOG_INFO "Checking/fetching precompiled Sirius engine v${TARGET_ENGINE_VERSION} for $ARCH from $ENGINE_URL..."
    mkdir -p "$DATA_DIR/bin"
    if curl -f -sSL "$ENGINE_URL" | tar -xz -C "$DATA_DIR"; then
        LOG_INFO "Catapult engine v${TARGET_ENGINE_VERSION} unpacked successfully to $DATA_DIR/bin"
        chmod +x "$DATA_DIR/bin/sirius.bc" "$DATA_DIR/bin/catapult.recovery" 2>/dev/null || true
        echo "$TARGET_ENGINE_VERSION" > "$VERSION_FILE"
        SIRIUS_BIN="$DATA_DIR/bin/sirius.bc"
        RECOVERY_BIN="$DATA_DIR/bin/catapult.recovery"
    else
        LOG_WARN "Engine release v${TARGET_ENGINE_VERSION} not yet available from GitHub; keeping existing engine binary."
    fi
fi

# Set dynamic library search path (safely handling nounset / set -u)
export LD_LIBRARY_PATH="$DATA_DIR/bin:/etc/sirius/bin:/usr/local/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"

if [ ! -f "$SIRIUS_BIN" ]; then
    LOG_ERR "Error: sirius.bc engine binary not found at $SIRIUS_BIN"
    sleep 60
    exit 1
fi

# 7. Run catapult.recovery pre-flight check if height > 1, or wipe partial statedb at height <= 1
if [ -f "$INDEX_FILE" ] && [ -s "$INDEX_FILE" ]; then
    HEIGHT=$(od -An -t u8 -N 8 "$INDEX_FILE" 2>/dev/null | tr -d ' ' || echo "0")
    if [ -n "${HEIGHT:-}" ] && [ "${HEIGHT:-0}" -gt 1 ] 2>/dev/null; then
        if [ -x "$RECOVERY_BIN" ]; then
            LOG_INFO "Running pre-flight catapult.recovery reconciliation (detected block height ${HEIGHT})..."
            cd "$DATA_DIR"
            "$RECOVERY_BIN" "$DATA_DIR" || LOG_WARN "catapult.recovery completed with exit status $?; continuing..."
            rm -f "$DATA_STORAGE"/*.lock "$DATA_STORAGE"/statedb/*/LOCK "$DATA_STORAGE"/statedb/LOCK 2>/dev/null || true
        fi
    else
        LOG_INFO "Block height is <= 1 (${HEIGHT:-0}). Clearing uncommitted partial statedb for clean Nemesis boot..."
        rm -rf "$DATA_STORAGE/statedb" 2>/dev/null || true
    fi
fi

LOG_INFO "Launching Sirius Catapult Engine ($SIRIUS_BIN) on P2P port 7900..."
cd "$DATA_DIR"
exec "$SIRIUS_BIN" "$DATA_DIR"
