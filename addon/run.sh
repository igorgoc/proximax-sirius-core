#!/usr/bin/env bash
set -e

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
mkdir -p "$DATA_DIR/resources/delegated_keys" "$DATA_STORAGE/00000" "$DATA_DIR/logs" "$DATA_DIR/bin" "$DATA_DIR/certificate"

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

# Update config-node.properties with friendlyName and hardware stability settings
NODE_CONF="$DATA_DIR/resources/config-node.properties"
if [ -f "$NODE_CONF" ]; then
    sed -i "s|^friendlyName *=.*|friendlyName = ${FRIENDLY_NAME}|" "$NODE_CONF" 2>/dev/null || true
    # RPi4 Resource & Stability Hardening:
    # 1. Disable process abort when consumer queues fill under I/O bursts
    sed -i "s|^shouldAbortWhenDispatcherIsFull *=.*|shouldAbortWhenDispatcherIsFull = false|" "$NODE_CONF" 2>/dev/null || true
    # 2. Right-size caches to prevent Linux OOM-killer on 4GB/8GB Home Assistant hosts
    sed -i "s|^shortLivedCacheMaxSize *=.*|shortLivedCacheMaxSize = 500'000|" "$NODE_CONF" 2>/dev/null || true
    sed -i "s|^unconfirmedTransactionsCacheMaxSize *=.*|unconfirmedTransactionsCacheMaxSize = 50'000|" "$NODE_CONF" 2>/dev/null || true
    # 3. Limit incoming connections to prevent socket buffer exhaustion (512 -> 64)
    sed -i '/^\[incoming_connections\]/,/^\[/ s|^maxConnections *=.*|maxConnections = 64|' "$NODE_CONF" 2>/dev/null || true
    sed -i '/^\[incoming_connections\]/,/^\[/ s|^backlogSize *=.*|backlogSize = 64|' "$NODE_CONF" 2>/dev/null || true
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
MAX_DELEGATED_KEYS=$(GET_CONFIG 'max_delegated_keys')
[ -z "${MAX_DELEGATED_KEYS:-}" ] && MAX_DELEGATED_KEYS=10
grep -q "^maxUnlockedAccounts" "$HARVEST_CONF" 2>/dev/null && sed -i "s|^maxUnlockedAccounts *=.*|maxUnlockedAccounts = ${MAX_DELEGATED_KEYS}|" "$HARVEST_CONF" || echo "maxUnlockedAccounts = ${MAX_DELEGATED_KEYS}" >> "$HARVEST_CONF"
grep -q "^beneficiary" "$HARVEST_CONF" 2>/dev/null || echo "beneficiary = 0000000000000000000000000000000000000000000000000000000000000000" >> "$HARVEST_CONF"

# Invariant: Mirror primary harvestKey to delegated_keys to protect against FastFinalityUtils pruning
if [ -n "${HARVEST_KEY:-}" ]; then
    echo "${HARVEST_KEY}" > "$DATA_DIR/resources/delegated_keys/primary_harvest.key"
    chmod 0600 "$DATA_DIR/resources/delegated_keys/primary_harvest.key"
fi

# Update logging output destinations to container log volume
for LOG_CONF in "$DATA_DIR/resources/config-logging-server.properties" "$DATA_DIR/resources/config-logging-recovery.properties" "$DATA_DIR/resources/config-logging-broker.properties"; do
    if [ -f "$LOG_CONF" ]; then
        sed -i "s|^directory *=.*|directory = ${DATA_DIR}/logs|" "$LOG_CONF" 2>/dev/null || true
        sed -i "s|^filePattern *=.*/\([^/]*\.log\)|filePattern = ${DATA_DIR}/logs/\1|" "$LOG_CONF" 2>/dev/null || true
        sed -i '/^\[file\]/,/^\[/ s|^sinkType *=.*|sinkType = Async|' "$LOG_CONF" 2>/dev/null || true
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
TARGET_ENGINE_VERSION="1.9.12"
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

if [ ! -f "$SIRIUS_BIN" ] || [ "${INSTALLED_ENGINE_VERSION#v}" != "${TARGET_ENGINE_VERSION#v}" ]; then
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
    
    RELEASE_TAG="v${TARGET_ENGINE_VERSION#v}"
    ENGINE_URL="https://github.com/igorgoc/cpp-xpx-chain/releases/download/${RELEASE_TAG}/${ENGINE_ASSET}"
    LOG_INFO "Checking/fetching precompiled Sirius engine ${RELEASE_TAG} for $ARCH from $ENGINE_URL..."
    mkdir -p "$DATA_DIR/bin"
    TMP_ENGINE_TAR="/tmp/${ENGINE_ASSET}"
    if curl -f -sSL "$ENGINE_URL" -o "$TMP_ENGINE_TAR"; then
        tar -xzf "$TMP_ENGINE_TAR" -C "$DATA_DIR"
        rm -f "$TMP_ENGINE_TAR"
        LOG_INFO "Catapult engine ${RELEASE_TAG} unpacked successfully to $DATA_DIR/bin"
        chmod +x "$DATA_DIR/bin/sirius.bc" "$DATA_DIR/bin/catapult.recovery" 2>/dev/null || true
        echo "$RELEASE_TAG" > "$VERSION_FILE"
        SIRIUS_BIN="$DATA_DIR/bin/sirius.bc"
        RECOVERY_BIN="$DATA_DIR/bin/catapult.recovery"
    else
        rm -f "$TMP_ENGINE_TAR"
        LOG_WARN "Engine release ${RELEASE_TAG} not yet available from GitHub; keeping existing engine binary."
    fi
fi

# 6.1 Locate or auto-fetch Sirius Delegation Listener daemon
LISTENER_BIN="/usr/local/bin/sirius-delegation-listener"
if [ ! -x "$LISTENER_BIN" ] && [ -x "$DATA_DIR/bin/sirius-delegation-listener" ]; then
    LISTENER_BIN="$DATA_DIR/bin/sirius-delegation-listener"
fi

if [ ! -x "$LISTENER_BIN" ]; then
    ARCH=$(uname -m)
    case "$ARCH" in
        aarch64|arm64)
            LISTENER_ASSET="sirius-delegation-listener-linux-arm64"
            ;;
        x86_64|amd64)
            LISTENER_ASSET="sirius-delegation-listener-linux-amd64"
            ;;
        *)
            LISTENER_ASSET=""
            ;;
    esac

    if [ -n "$LISTENER_ASSET" ]; then
        LISTENER_URL="https://github.com/igorgoc/proximax-sirius-core/releases/download/v1.9.11/${LISTENER_ASSET}"
        LOG_INFO "Checking/fetching autonomous delegation listener daemon for $ARCH from $LISTENER_URL..."
        if curl -f -sSL "$LISTENER_URL" -o "$DATA_DIR/bin/sirius-delegation-listener"; then
            chmod +x "$DATA_DIR/bin/sirius-delegation-listener"
            LISTENER_BIN="$DATA_DIR/bin/sirius-delegation-listener"
            LOG_INFO "Delegation listener installed successfully at $LISTENER_BIN"
        else
            LOG_WARN "Could not fetch delegation listener daemon; remote validator auto-detection will be manual."
        fi
    fi
fi

# Set dynamic library search path (safely handling nounset / set -u)
export LD_LIBRARY_PATH="$DATA_DIR/bin:/etc/sirius/bin:/usr/local/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"

if [ ! -f "$SIRIUS_BIN" ]; then
    LOG_ERR "Error: sirius.bc engine binary not found at $SIRIUS_BIN"
    sleep 60
    exit 1
fi

# 7. Check index & Nemesis block initialization
if [ -f "$INDEX_FILE" ] && [ -s "$INDEX_FILE" ]; then
    HEIGHT=$(od -An -t u8 -N 8 "$INDEX_FILE" 2>/dev/null | tr -d ' ' || echo "0")
    if [ "${HEIGHT:-0}" -le 1 ] 2>/dev/null; then
        LOG_INFO "Block height is <= 1 (${HEIGHT:-0}). Clearing uncommitted partial statedb for clean Nemesis boot..."
        rm -rf "$DATA_STORAGE/statedb" 2>/dev/null || true
    fi
fi

# 8. Engine Supervisor & Automated Sync Watchdog
LOG_INFO "Starting Sirius Catapult Engine Supervisor with Automated Sync Watchdog..."
cd "$DATA_DIR"

get_network_height() {
    local h=""
    for ep in "https://aldebaran.xpxsirius.io/chain/height" "https://betelgeuse.xpxsirius.io/chain/height"; do
        h=$(curl -s --connect-timeout 3 -m 5 "$ep" 2>/dev/null | jq -r 'if .height then (.height[0] + .height[1] * 4294967296) else empty end' 2>/dev/null || true)
        if [ -n "$h" ] && [ "$h" -gt 0 ] 2>/dev/null; then
            echo "$h"
            return 0
        fi
    done
    echo "0"
}

cleanup_and_exit() {
    LOG_INFO "Shutdown signal received. Stopping sirius.bc gracefully..."
    if [ -n "${LISTENER_PID:-}" ] && kill -0 "$LISTENER_PID" 2>/dev/null; then
        LOG_INFO "Stopping delegation listener daemon..."
        kill -TERM "$LISTENER_PID" 2>/dev/null || true
    fi
    if [ -n "${ENGINE_PID:-}" ] && kill -0 "$ENGINE_PID" 2>/dev/null; then
        kill -INT "$ENGINE_PID" 2>/dev/null || true
        for i in $(seq 1 45); do
            if ! kill -0 "$ENGINE_PID" 2>/dev/null; then
                break
            fi
            sleep 1
        done
        if kill -0 "$ENGINE_PID" 2>/dev/null; then
            LOG_WARN "Engine did not exit in 45s; sending SIGKILL..."
            kill -KILL "$ENGINE_PID" 2>/dev/null || true
        fi
    fi
    exit 0
}

trap cleanup_and_exit SIGTERM SIGINT

# Watchdog timing parameters
WATCHDOG_TIMEOUT_SEC=480       # 8 minutes stall before checking network
WATCHDOG_GRACE_PERIOD_SEC=180  # 3 minutes startup grace period
WATCHDOG_COOLDOWN_SEC=300      # 5 minutes cooldown between stall recoveries
LAST_STALL_RECOVERY_TIME=0

while true; do
    # Clear stale lock files before boot
    rm -f "$DATA_STORAGE"/*.lock "$DATA_STORAGE"/statedb/*/LOCK "$DATA_STORAGE"/statedb/LOCK 2>/dev/null || true

    # Run recovery if height > 1
    if [ -f "$INDEX_FILE" ] && [ -s "$INDEX_FILE" ]; then
        HEIGHT=$(od -An -t u8 -N 8 "$INDEX_FILE" 2>/dev/null | tr -d ' ' || echo "0")
        if [ -n "${HEIGHT:-}" ] && [ "${HEIGHT:-0}" -gt 1 ] 2>/dev/null; then
            if [ -x "$RECOVERY_BIN" ]; then
                LOG_INFO "Running pre-flight catapult.recovery (detected block height ${HEIGHT})..."
                "$RECOVERY_BIN" "$DATA_DIR" || LOG_WARN "catapult.recovery exit code $?; continuing..."
                rm -f "$DATA_STORAGE"/*.lock "$DATA_STORAGE"/statedb/*/LOCK "$DATA_STORAGE"/statedb/LOCK 2>/dev/null || true
            fi
        fi
    fi

    # Start engine process in background
    "$SIRIUS_BIN" "$DATA_DIR" &
    ENGINE_PID=$!
    ENGINE_START_TIME=$(date +%s)
    LOG_INFO "sirius.bc started with PID $ENGINE_PID on P2P port 7900"

    # Start autonomous delegation listener if binary exists and not already running
    if [ -x "${LISTENER_BIN:-}" ]; then
        if [ -z "${LISTENER_PID:-}" ] || ! kill -0 "$LISTENER_PID" 2>/dev/null; then
            "$LISTENER_BIN" -resources "$DATA_DIR/resources" -data "$DATA_STORAGE" 2>&1 &
            LISTENER_PID=$!
            LOG_INFO "Autonomous delegation listener running (PID $LISTENER_PID, monitoring on-chain staking delegations)"
        fi
    fi

    # Watchdog state initialization
    LAST_OBSERVED_HEIGHT=0
    if [ -f "$INDEX_FILE" ] && [ -s "$INDEX_FILE" ]; then
        LAST_OBSERVED_HEIGHT=$(od -An -t u8 -N 8 "$INDEX_FILE" 2>/dev/null | tr -d ' ' || echo "0")
    fi
    LAST_ADVANCE_TIME=$(date +%s)
    LAST_STATUS_LOG_TIME=0

    # Watchdog monitoring loop (checks every 30 seconds)
    while kill -0 "$ENGINE_PID" 2>/dev/null; do
        sleep 30

        CURRENT_TIME=$(date +%s)
        UPTIME_SEC=$(( CURRENT_TIME - ENGINE_START_TIME ))

        # Read current local height from index.dat
        LOCAL_HEIGHT=0
        if [ -f "$INDEX_FILE" ] && [ -s "$INDEX_FILE" ]; then
            LOCAL_HEIGHT=$(od -An -t u8 -N 8 "$INDEX_FILE" 2>/dev/null | tr -d ' ' || echo "0")
        fi

        # If local height advanced, update progress tracking
        if [ "${LOCAL_HEIGHT:-0}" -gt "${LAST_OBSERVED_HEIGHT:-0}" ] 2>/dev/null; then
            LAST_OBSERVED_HEIGHT="$LOCAL_HEIGHT"
            LAST_ADVANCE_TIME="$CURRENT_TIME"
        fi

        # Periodic health log every 10 minutes
        if [ $(( CURRENT_TIME - LAST_STATUS_LOG_TIME )) -ge 600 ]; then
            LAST_STATUS_LOG_TIME="$CURRENT_TIME"
            LOG_INFO "[SENTRY] Node running (PID: $ENGINE_PID, local height: ${LOCAL_HEIGHT}, uptime: $(( UPTIME_SEC / 60 ))m)"
        fi

        # Skip stall evaluation during startup grace period (handshake & timesync establishment)
        if [ "$UPTIME_SEC" -lt "$WATCHDOG_GRACE_PERIOD_SEC" ]; then
            continue
        fi

        # Skip stall evaluation during cooldown window
        if [ "$LAST_STALL_RECOVERY_TIME" -gt 0 ]; then
            SINCE_LAST_RECOVERY=$(( CURRENT_TIME - LAST_STALL_RECOVERY_TIME ))
            if [ "$SINCE_LAST_RECOVERY" -lt "$WATCHDOG_COOLDOWN_SEC" ]; then
                continue
            fi
        fi

        # Calculate stalled duration
        IDLE_SEC=$(( CURRENT_TIME - LAST_ADVANCE_TIME ))

        # Check for sync stall: no local height advance for >= WATCHDOG_TIMEOUT_SEC (8 minutes)
        if [ "$IDLE_SEC" -ge "$WATCHDOG_TIMEOUT_SEC" ]; then
            NETWORK_HEIGHT=$(get_network_height)

            if [ -n "$NETWORK_HEIGHT" ] && [ "$NETWORK_HEIGHT" -gt 0 ] 2>/dev/null; then
                BEHIND_BLOCKS=$(( NETWORK_HEIGHT - LOCAL_HEIGHT ))

                # Invariant: If node is at tip or within 5 blocks of network height, DO NOT restart!
                if [ "$BEHIND_BLOCKS" -le 5 ]; then
                    LAST_ADVANCE_TIME="$CURRENT_TIME"
                    LOG_INFO "[WATCHDOG] Node is fully in sync at chain tip (local: ${LOCAL_HEIGHT}, network: ${NETWORK_HEIGHT}). Normal operation."
                    continue
                fi

                # Genuine sync stall: Node is behind by > 5 blocks and frozen for >= 8 min
                LOG_WARN "[WATCHDOG] ⚠️ Genuine sync stall detected! Local height stuck at ${LOCAL_HEIGHT} for ${IDLE_SEC}s while network height is ${NETWORK_HEIGHT} (${BEHIND_BLOCKS} blocks behind)."
            else
                # Network query failed (temporary DNS/network fluctuation): defer restart
                LOG_WARN "[WATCHDOG] Local height idle for ${IDLE_SEC}s, but unable to query public network height. Deferring restart."
                LAST_ADVANCE_TIME=$(( CURRENT_TIME - (WATCHDOG_TIMEOUT_SEC / 2) ))
                continue
            fi

            LOG_WARN "[WATCHDOG] Initiating automated graceful restart (45s grace period) to cycle stale P2P sockets..."
            LAST_STALL_RECOVERY_TIME="$CURRENT_TIME"

            kill -INT "$ENGINE_PID" 2>/dev/null || true
            for w in $(seq 1 45); do
                if ! kill -0 "$ENGINE_PID" 2>/dev/null; then
                    break
                fi
                sleep 1
            done
            if kill -0 "$ENGINE_PID" 2>/dev/null; then
                LOG_WARN "[WATCHDOG] Engine did not exit in 45s; sending SIGKILL..."
                kill -KILL "$ENGINE_PID" 2>/dev/null || true
            fi
            wait "$ENGINE_PID" 2>/dev/null || true
            break
        fi
    done

    wait "$ENGINE_PID" 2>/dev/null || true
    EXIT_CODE=$?
    if [ "$EXIT_CODE" -eq 137 ]; then
        LOG_ERR "Engine was terminated by SIGKILL (Exit code 137). Probable cause: Linux Kernel OOM killer on Raspberry Pi 4."
    elif [ "$EXIT_CODE" -eq 134 ]; then
        LOG_ERR "Engine crashed with SIGABRT (Exit code 134). Probable cause: Disruptor overflow or assertion abort."
    elif [ "$EXIT_CODE" -eq 130 ]; then
        LOG_INFO "Engine stopped via SIGINT (Exit code 130)."
    else
        LOG_WARN "sirius.bc exited with status $EXIT_CODE."
    fi
    LOG_INFO "Restarting engine in 5 seconds..."
    sleep 5
done

