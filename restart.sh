#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR"

"$DIR/stop.sh"
sleep 1
"$DIR/run.sh"
