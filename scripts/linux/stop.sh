#!/bin/bash

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
cd "$DIR"

echo "========================================================="
echo "  Stopping ProximaX Sirius Native Node Gracefully..."
echo "========================================================="

PORT="${PORT:-8080}"
STOPPED=false

# 1. Preferred Method: Graceful API Shutdown via HTTP POST /api/system/shutdown
# This allows the Go supervisor to initiate StopNode(), drain disruptor block queues,
# flush RocksDB state, and sync physical disks cleanly without external signal truncation.
TOKEN=""
if [ -f "$DIR/chainconfig/resources/.sirius-token" ]; then
    TOKEN=$(cat "$DIR/chainconfig/resources/.sirius-token" 2>/dev/null | tr -d ' \r\n' || true)
fi

if command -v curl >/dev/null 2>&1; then
    if [ -z "$TOKEN" ]; then
        TOKEN=$(curl -s "http://127.0.0.1:$PORT/api/auth/token" -H "Origin: http://127.0.0.1:$PORT" --max-time 2 2>/dev/null | grep -o '"token":"[^"]*"' | cut -d'"' -f4 || true)
    fi

    CURL_AUTH=()
    if [ -n "$TOKEN" ]; then
        CURL_AUTH=(-H "X-Sirius-Token: $TOKEN" -H "Authorization: Bearer $TOKEN")
    fi

    if curl -s -f -o /dev/null -X POST "http://127.0.0.1:$PORT/api/system/shutdown" \
        -H "Origin: http://127.0.0.1:$PORT" \
        -H "Content-Type: application/json" \
        "${CURL_AUTH[@]}" \
        -d '{}' --max-time 5 2>/dev/null; then
        echo "-> Sent graceful shutdown command to Node Manager API (port $PORT)..."
        echo -n "-> Waiting for blockchain engine & RocksDB state flushes to finish"

        # Wait up to 60 seconds for supervisor and sirius.bc to terminate cleanly
        for i in {1..60}; do
            MGR_ALIVE=false
            if [ -f "$DIR/.sirius-core.pid" ]; then
                MGR_PID=$(cat "$DIR/.sirius-core.pid" 2>/dev/null || true)
                if [ -n "$MGR_PID" ] && kill -0 "$MGR_PID" 2>/dev/null; then
                    MGR_ALIVE=true
                fi
            fi
            ENGINE_ALIVE=false
            if pgrep -f "sirius.bc" >/dev/null 2>&1; then
                ENGINE_ALIVE=true
            fi

            if [ "$MGR_ALIVE" = false ] && [ "$ENGINE_ALIVE" = false ]; then
                STOPPED=true
                echo ""
                echo "✓ Node manager and engine terminated cleanly via API."
                break
            fi
            echo -n "."
            sleep 1
        done
        echo ""
    fi
fi

# 2. Fallback: If API was unreachable or process is still running, handle via signals
if [ "$STOPPED" = false ]; then
    # 2a. Signal the Manager process
    if [ -f "$DIR/.sirius-core.pid" ]; then
        PID=$(cat "$DIR/.sirius-core.pid" 2>/dev/null || true)
        if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
            echo "-> Sending SIGINT to Manager process (PID $PID)..."
            kill -INT "$PID" 2>/dev/null || true
            for i in {1..20}; do
                if ! kill -0 "$PID" 2>/dev/null; then
                    break
                fi
                sleep 1
            done
        fi
    fi

    # 2b. Gracefully signal the C++ Catapult engine (sirius.bc) with forward-progress tracking
    if pgrep -f "sirius.bc" >/dev/null 2>&1; then
        echo "-> Gracefully terminating sirius.bc engine (SIGINT)..."
        pkill -INT -f "sirius.bc" 2>/dev/null || true

        LOGS_DIR="$DIR/chainconfig/logs"
        INDEX_FILE="$DIR/chainconfig/data/index.dat"

        LAST_ACTIVE_LOG=""
        LAST_LOG_SIZE=0
        LAST_INDEX_MTIME=""
        IDLE_SECS=0
        ELAPSED=0
        ENGINE_STOPPED=false

        echo -n "-> Gracefully committing in-flight blocks & flushing RocksDB"

        while true; do
            if ! pgrep -f "sirius.bc" >/dev/null 2>&1; then
                ENGINE_STOPPED=true
                echo ""
                echo "✓ sirius.bc engine finished clean shutdown (${ELAPSED}s elapsed)."
                break
            fi

            # Multi-Signal Forward Progress Check:
            # Signal 1: Active server_*.log file growth or log rotation
            PROGRESS=false
            CUR_LOG=$(ls -t "$LOGS_DIR"/server_*.log 2>/dev/null | head -n 1 || true)
            if [ -n "$CUR_LOG" ] && [ -f "$CUR_LOG" ]; then
                CUR_SIZE=$(wc -c < "$CUR_LOG" 2>/dev/null | tr -d ' ' || echo 0)
                if [ "$CUR_LOG" != "$LAST_ACTIVE_LOG" ]; then
                    PROGRESS=true
                    LAST_ACTIVE_LOG="$CUR_LOG"
                    LAST_LOG_SIZE=$CUR_SIZE
                elif [ "$CUR_SIZE" -gt "$LAST_LOG_SIZE" ]; then
                    PROGRESS=true
                    LAST_LOG_SIZE=$CUR_SIZE
                fi
            fi

            # Signal 2: Storage height / index.dat modification
            if [ -f "$INDEX_FILE" ]; then
                CUR_INDEX_MTIME=$(stat -f "%m" "$INDEX_FILE" 2>/dev/null || stat -c "%Y" "$INDEX_FILE" 2>/dev/null || true)
                if [ -n "$CUR_INDEX_MTIME" ] && [ "$CUR_INDEX_MTIME" != "$LAST_INDEX_MTIME" ]; then
                    if [ -n "$LAST_INDEX_MTIME" ]; then
                        PROGRESS=true
                    fi
                    LAST_INDEX_MTIME="$CUR_INDEX_MTIME"
                fi
            fi

            if [ "$PROGRESS" = true ]; then
                IDLE_SECS=0
            else
                IDLE_SECS=$((IDLE_SECS + 1))
            fi

            # Strictly progress-based timeout: NEVER kill as long as forward progress continues.
            # Only escalate if process made ZERO forward progress for 60 consecutive seconds (deadlock):
            if [ "$IDLE_SECS" -ge 60 ]; then
                echo ""
                echo "<warning> sirius.bc made ZERO forward progress for 60 seconds (deadlock detected). Escalating with SIGKILL..."
                pkill -KILL -f "sirius.bc" 2>/dev/null || true
                break
            fi

            echo -n "."
            sleep 1
            ELAPSED=$((ELAPSED + 1))
        done
    fi

    # 2c. Clean up any remaining manager daemon processes
    pkill -INT -f "sirius-core.*-chainconfig" 2>/dev/null || true
    pkill -INT -f "sirius-core" 2>/dev/null || true
    sleep 1

    # Only kill unresponsive manager processes holding the HTTP port
    PORT_PID=$(lsof -ti :$PORT 2>/dev/null || true)
    if [ -n "$PORT_PID" ]; then
        kill -9 $PORT_PID 2>/dev/null || true
    fi
fi

# Clean PID file
rm -f "$DIR/.sirius-core.pid"

# 3. Flush OS filesystem buffers to physical storage media (vital for external SSDs)
if command -v sync >/dev/null 2>&1; then
    echo "-> Flushing OS filesystem buffers to physical disk (sync)..."
    sync 2>/dev/null || true
fi

# 4. Detect data directory and clean stale lock files ONLY after verifying engine is dead
if ! pgrep -f "sirius.bc" >/dev/null 2>&1; then
    DATA_DIR="$DIR/chainconfig/data"
    USER_PROPS="$DIR/chainconfig/resources/config-user.properties"
    if [ -f "$USER_PROPS" ]; then
        CUSTOM_DATA=$(grep -E "^[[:space:]]*data.path[[:space:]]*=" "$USER_PROPS" | cut -d'=' -f2- | tr -d "[:space:]\"'")
        if [ -n "$CUSTOM_DATA" ]; then
            if [[ "$CUSTOM_DATA" = /* ]]; then
                DATA_DIR="$CUSTOM_DATA"
            else
                DATA_DIR="$DIR/$CUSTOM_DATA"
            fi
        fi
    fi

    if [ -d "$DATA_DIR" ]; then
        echo "-> Cleaning lock files in $DATA_DIR..."
        rm -f "$DATA_DIR"/*.lock 2>/dev/null || true
        rm -f "$DATA_DIR"/statedb/*/LOCK 2>/dev/null || true

        # 5. Verify post-shutdown state integrity (Storage Height vs Cache Height)
        INDEX_FILE="$DATA_DIR/index.dat"
        SUPP_FILE="$DATA_DIR/state/supplemental.dat"
        [ ! -f "$SUPP_FILE" ] && SUPP_FILE="$DATA_DIR/supplemental.dat"

        if [ -f "$INDEX_FILE" ] && [ -f "$SUPP_FILE" ] && command -v od >/dev/null 2>&1; then
            STORAGE_HEIGHT=$(od -An -j0 -N8 -t u8 "$INDEX_FILE" 2>/dev/null | tr -d ' ' || echo 0)
            CACHE_HEIGHT=$(od -An -j32 -N8 -t u8 "$SUPP_FILE" 2>/dev/null | tr -d ' ' || echo 0)
            if [ -n "$STORAGE_HEIGHT" ] && [ -n "$CACHE_HEIGHT" ]; then
                if [ "$STORAGE_HEIGHT" -eq "$CACHE_HEIGHT" ]; then
                    echo "✓ Shutdown integrity verified: Storage Height ($STORAGE_HEIGHT) == Cache Height ($CACHE_HEIGHT)."
                elif [ "$STORAGE_HEIGHT" -gt 1 ]; then
                    echo "<warning> Height divergence detected: Storage Height ($STORAGE_HEIGHT) != Cache Height ($CACHE_HEIGHT)!"
                    if [ -x "$DIR/bin/catapult.recovery" ]; then
                        echo "-> Running catapult.recovery to reconcile..."
                        ulimit -n 65536 2>/dev/null || true
                        DYLD_LIBRARY_PATH="$DIR/bin:${DYLD_LIBRARY_PATH:-}" LD_LIBRARY_PATH="$DIR/bin:${LD_LIBRARY_PATH:-}" "$DIR/bin/catapult.recovery" "$DIR/chainconfig" 2>&1 || true
                        sync 2>/dev/null || true
                    fi
                fi
            fi
        fi
    fi
else
    echo "<warning> sirius.bc process is still detected. Preserving lock files to prevent dual-instance conflict."
fi

echo "========================================================="
echo "  ProximaX Sirius Node is stopped."
echo "========================================================="
