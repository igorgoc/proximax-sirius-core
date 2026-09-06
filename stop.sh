#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR"

echo "Stopping ProximaX Sirius Native Node..."

# 1. Stop via PID if file exists
if [ -f "$DIR/.sirius-core.pid" ]; then
    PID=$(cat "$DIR/.sirius-core.pid")
    if kill -0 "$PID" 2>/dev/null; then
        echo "Stopping Manager process (PID $PID)..."
        kill -INT "$PID" 2>/dev/null || true
        for i in {1..10}; do
            if ! kill -0 "$PID" 2>/dev/null; then
                break
            fi
            sleep 1
        done
        if kill -0 "$PID" 2>/dev/null; then
            kill -KILL "$PID" 2>/dev/null || true
        fi
    fi
    rm -f "$DIR/.sirius-core.pid"
fi

# 2. Also ensure any running sirius.bc or recovery processes are gracefully stopped
if pgrep -f "sirius.bc" >/dev/null 2>&1; then
    echo "Stopping sirius.bc core engine..."
    pkill -INT -f "sirius.bc" 2>/dev/null || true
    for i in {1..15}; do
        if ! pgrep -f "sirius.bc" >/dev/null 2>&1; then
            break
        fi
        sleep 1
    done
    pkill -KILL -f "sirius.bc" 2>/dev/null || true
fi
pkill -INT -f "sirius-core.*-chainconfig" 2>/dev/null || true

# 3. Detect data directory and clean all stale lock files
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
    echo "Cleaning lock files in $DATA_DIR..."
    chmod 777 "$DATA_DIR"/*.lock 2>/dev/null || true
    rm -f "$DATA_DIR"/*.lock 2>/dev/null || true
    chmod 777 "$DATA_DIR"/statedb/*/LOCK 2>/dev/null || true
    rm -f "$DATA_DIR"/statedb/*/LOCK 2>/dev/null || true
fi

echo "ProximaX Sirius Node is stopped."
