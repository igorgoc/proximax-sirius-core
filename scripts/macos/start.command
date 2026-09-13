#!/bin/bash
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
if [ -f "$SCRIPT_DIR/start.sh" ]; then
    exec "$SCRIPT_DIR/start.sh" "$@"
elif [ -f "$SCRIPT_DIR/../macos/start.sh" ]; then
    exec "$SCRIPT_DIR/../macos/start.sh" "$@"
else
    exec "$SCRIPT_DIR/start.sh" "$@"
fi
