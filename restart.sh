#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR"

"$DIR/stop.sh"
sleep 1

if [ -f "$DIR/start.sh" ]; then
    exec "$DIR/start.sh"
elif [ -f "$DIR/run.sh" ]; then
    exec "$DIR/run.sh"
else
    exec "$DIR/sirius-core"
fi
