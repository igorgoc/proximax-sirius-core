#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR"

echo "========================================================="
echo "  Stopping ProximaX Sirius Native Node Gracefully..."
echo "========================================================="

PORT="${PORT:-8080}"
STOPPED=false

# 1. Preferred Method: Graceful API Shutdown via HTTP POST /api/system/shutdown
# This allows the Go supervisor to initiate StopNode(), drain disruptor block queues,
# flush RocksDB state, and sync physical disks cleanly without external signal truncation.
if command -v curl >/dev/null 2>&1; then
    if curl -s -f -o /dev/null -X POST "http://127.0.0.1:$PORT/api/system/shutdown" --max-time 3 2>/dev/null; then
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

    # 2b. Gracefully signal the C++ Catapult engine (sirius.bc)
    if pgrep -f "sirius.bc" >/dev/null 2>&1; then
        echo "-> Gracefully terminating sirius.bc engine (SIGINT)..."
        pkill -INT -f "sirius.bc" 2>/dev/null || true

        # Give RocksDB multi-gigabyte state cache and block disruptor up to 60 seconds to flush
        echo -n "-> Waiting for RocksDB state cache to flush to disk"
        ENGINE_STOPPED=false
        for i in {1..60}; do
            if ! pgrep -f "sirius.bc" >/dev/null 2>&1; then
                ENGINE_STOPPED=true
                echo ""
                echo "✓ sirius.bc engine finished clean shutdown (${i}s elapsed)."
                break
            fi
            echo -n "."
            sleep 1
        done

        if [ "$ENGINE_STOPPED" = false ]; then
            echo ""
            echo "<warning> sirius.bc did not terminate within 60s (possible freeze). Escalating with SIGKILL..."
            pkill -KILL -f "sirius.bc" 2>/dev/null || true
        fi
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
    fi
else
    echo "<warning> sirius.bc process is still detected. Preserving lock files to prevent dual-instance conflict."
fi

echo "========================================================="
echo "  ProximaX Sirius Node is stopped."
echo "========================================================="
