#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
cd "$DIR"

echo "=========================================================="
echo " Targeted Transition Persistence Test (Default -> SSD A -> SSD B -> Default -> User SSD)"
echo "=========================================================="

stop_daemon() {
  if [ -f "$DIR/.sirius-core.pid" ]; then
    PID=$(cat "$DIR/.sirius-core.pid")
    if kill -0 "$PID" 2>/dev/null; then
      echo "  [Kill] Sending kill -9 to daemon PID $PID..."
      kill -9 "$PID" 2>/dev/null || true
    fi
    rm -f "$DIR/.sirius-core.pid"
  fi
  PORT_PID=$(lsof -ti :8080 2>/dev/null || true)
  if [ -n "$PORT_PID" ]; then
    kill -9 $PORT_PID 2>/dev/null || true
  fi
  sleep 1
}

start_daemon() {
  echo "  [Boot] Cold-starting daemon fresh..."
  DYLD_LIBRARY_PATH="$DIR/bin" LD_LIBRARY_PATH="$DIR/bin" "$DIR/backend/sirius-core" -port 8080 -chainconfig "$DIR/chainconfig" > "$DIR/chainconfig/logs/manager.log" 2>&1 &
  NEW_PID=$!
  echo "$NEW_PID" > "$DIR/.sirius-core.pid"
  
  for attempt in {1..30}; do
    if curl -s -f "http://localhost:8080/api/config" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.2
  done
  echo "ERROR: Daemon failed to start within timeout"
  exit 1
}

test_step() {
  STEP_NUM="$1"
  STEP_DESC="$2"
  TARGET_PATH="$3"

  echo ""
  echo "----------------------------------------------------------"
  echo " STEP $STEP_NUM: $STEP_DESC"
  echo " Target Path: $TARGET_PATH"
  echo "----------------------------------------------------------"

  echo "  1. POST /api/config/save with dataPath: $TARGET_PATH"
  HTTP_RES=$(curl -s -w "\n%{http_code}" -X POST -H 'Content-Type: application/json' -d "{\"dataPath\":\"$TARGET_PATH\"}" http://localhost:8080/api/config/save)
  HTTP_BODY=$(echo "$HTTP_RES" | sed '$d')
  HTTP_CODE=$(echo "$HTTP_RES" | tail -n 1)

  if [ "$HTTP_CODE" != "200" ]; then
    echo "  FAILED: Save returned HTTP $HTTP_CODE: $HTTP_BODY"
    exit 1
  fi
  echo "  -> Save HTTP $HTTP_CODE OK"

  echo "  2. Verifying physical disk before kill..."
  MGR_VAL=$(grep "data.path" "$DIR/chainconfig/resources/config-manager.properties" | awk -F '=' '{print $2}' | tr -d ' \r\n')
  USER_VAL=$(grep "dataDirectory" "$DIR/chainconfig/resources/config-user.properties" | awk -F '=' '{print $2}' | tr -d ' \r\n')
  if [ "$MGR_VAL" != "$TARGET_PATH" ]; then
    echo "  FAILED: config-manager.properties has '$MGR_VAL', expected '$TARGET_PATH'"
    exit 1
  fi
  if [ "$USER_VAL" != "$TARGET_PATH" ]; then
    echo "  FAILED: config-user.properties has '$USER_VAL', expected '$TARGET_PATH'"
    exit 1
  fi
  echo "  -> Pre-kill disk values match target perfectly"

  echo "  3. Executing HARD KILL (kill -9)..."
  stop_daemon

  echo "  4. Cold boot daemon..."
  start_daemon

  echo "  5. Querying GET /api/config to verify persisted value after cold boot..."
  LOADED_CONFIG=$(curl -s "http://localhost:8080/api/config?_t=$(date +%s)")
  LOADED_PATH=$(echo "$LOADED_CONFIG" | grep -o '"dataPath":"[^"]*' | cut -d'"' -f4)

  if [ "$LOADED_PATH" != "$TARGET_PATH" ]; then
    echo "  FAILED: Cold boot returned '$LOADED_PATH', expected '$TARGET_PATH'!"
    exit 1
  fi
  echo "  -> API verified: dataPath='$LOADED_PATH'"

  MGR_POST=$(grep "data.path" "$DIR/chainconfig/resources/config-manager.properties" | awk -F '=' '{print $2}' | tr -d ' \r\n')
  if [ "$MGR_POST" != "$TARGET_PATH" ]; then
    echo "  FAILED: Post-boot disk value changed to '$MGR_POST'!"
    exit 1
  fi
  echo "  -> Post-boot disk verified: '$MGR_POST'"
  echo "  STEP $STEP_NUM PASSED!"
}

# 1. Reset to Default
test_step "1" "Initialize at DEFAULT project directory" "./chainconfig/data"

# 2. Transition from Default to SSD Path Alpha
test_step "2" "Transition from DEFAULT to SSD Path Alpha" "/Volumes/SSD/Sirius_data_alpha"

# 3. Transition from existing SSD Path Alpha to DIFFERENT SSD Path Beta
test_step "3" "Transition from existing SSD Path Alpha to DIFFERENT SSD Path Beta" "/Volumes/SSD/Sirius_data_beta"

# 4. Transition from SSD Path Beta back to Default
test_step "4" "Transition from SSD Path Beta back to DEFAULT" "./chainconfig/data"

# 5. Transition from Default back to Production SSD
test_step "5" "Transition from DEFAULT to User Production SSD" "/Volumes/SSD/Sirius_data"

echo ""
echo "=========================================================="
echo " ALL TRANSITIONS VERIFIED AND PASSED WITH 100% PERSISTENCE!"
echo "=========================================================="
