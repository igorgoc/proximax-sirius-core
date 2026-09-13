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

if [ -f "$SCRIPT_DIR/stop.sh" ]; then
    "$SCRIPT_DIR/stop.sh"
elif [ -f "$DIR/scripts/linux/stop.sh" ]; then
    "$DIR/scripts/linux/stop.sh"
elif [ -f "$DIR/stop.sh" ]; then
    "$DIR/stop.sh"
fi
sleep 1

if [ -f "$SCRIPT_DIR/start.sh" ]; then
    exec "$SCRIPT_DIR/start.sh"
elif [ -f "$DIR/scripts/linux/start.sh" ]; then
    exec "$DIR/scripts/linux/start.sh"
elif [ -f "$DIR/start.sh" ]; then
    exec "$DIR/start.sh"
elif [ -f "$SCRIPT_DIR/run.sh" ]; then
    exec "$SCRIPT_DIR/run.sh"
elif [ -f "$DIR/scripts/linux/run.sh" ]; then
    exec "$DIR/scripts/linux/run.sh"
elif [ -f "$DIR/run.sh" ]; then
    exec "$DIR/run.sh"
else
    exec "$DIR/sirius-core"
fi
