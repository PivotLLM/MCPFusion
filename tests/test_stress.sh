#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

# MCPFusion Stress Test
#
# Tests MCPFusion under increasing concurrent load using local tools only
# (no external API dependencies). Tests are structured in phases from light
# to heavy load.
#
# Requires probe with -repeat and -concurrent support.
#
# Phases:
#   1. Warm-up       - 10 repeat,   5 concurrent  (health)
#   2. Light         - 50 repeat,  10 concurrent  (health + knowledge)
#   3. Medium        - 200 repeat, 25 concurrent  (health + knowledge)
#   4. Heavy         - 500 repeat, 50 concurrent  (health)
#   5. Maximum       - 1000 repeat, 50 concurrent (health)
#   6. Echo tools    - Runs if echo provider is available (all echo tools)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ -f "$SCRIPT_DIR/.env" ]; then
    source "$SCRIPT_DIR/.env"
else
    echo "Error: .env file not found in $SCRIPT_DIR"
    exit 1
fi

[ -z "$APIKEY" ]      && { echo "Error: APIKEY not set";      exit 1; }
[ -z "$SERVER_URL" ]  && { echo "Error: SERVER_URL not set";  exit 1; }

PROBE_PATH="${PROBE_PATH:-probe}"
FULL_SERVER_URL="${SERVER_URL}/mcp"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
OUTPUT_FILE="${SCRIPT_DIR}/stress_test_${TIMESTAMP}.log"
PASS=0
FAIL=0

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo "$@" | tee -a "$OUTPUT_FILE"; }

run_phase() {
    local phase_name="$1"
    local repeat="$2"
    local concurrent="$3"
    local tool="$4"
    local params="$5"

    log ""
    log "=== Phase: $phase_name (repeat=$repeat, concurrent=$concurrent) ==="
    log "Tool: $tool"
    log "Params: $params"
    log "Started: $(date)"

    local start_time=$SECONDS

    "$PROBE_PATH" \
        -url "$FULL_SERVER_URL" \
        -transport http \
        -headers "Authorization:Bearer $APIKEY" \
        -call "$tool" \
        -params "$params" \
        -repeat "$repeat" \
        -concurrent "$concurrent" \
        2>&1 | tee -a "$OUTPUT_FILE"

    local exit_code=${PIPESTATUS[0]}
    local elapsed=$(( SECONDS - start_time ))

    if [ $exit_code -eq 0 ]; then
        log -e "${GREEN}[PASS]${NC} $phase_name completed in ${elapsed}s"
        ((PASS++))
    else
        log -e "${RED}[FAIL]${NC} $phase_name failed (exit code $exit_code) after ${elapsed}s"
        ((FAIL++))
    fi
}

# Check prerequisites
if ! command -v "$PROBE_PATH" > /dev/null 2>&1; then
    echo "Error: probe not found at '$PROBE_PATH'"
    exit 1
fi

# Check server connectivity
log "=== MCPFusion Stress Test ==="
log "Timestamp: $(date)"
log "Server: $FULL_SERVER_URL"
log "Using API Token: ${APIKEY:0:8}..."
log ""

log "Checking server connectivity..."
if ! "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport http \
    -headers "Authorization:Bearer $APIKEY" \
    -call mcp__fusion__health_status -params '{}' > /dev/null 2>&1; then
    log "Error: Server not available at $FULL_SERVER_URL"
    exit 1
fi
log "Server is available"

# Record baseline memory (Linux)
if [ -f /proc/self/status ]; then
    PID=$(pgrep -f mcpfusion | head -1)
    if [ -n "$PID" ]; then
        BASELINE_MEM=$(awk '/VmRSS/{print $2}' /proc/$PID/status 2>/dev/null)
        log "MCPFusion PID: $PID, baseline RSS: ${BASELINE_MEM} kB"
    fi
fi
log ""

#-------------------------------------------------------------------------------
# Phase 1: Warm-up — light load, health check only
#-------------------------------------------------------------------------------
run_phase "Warm-up" 10 5 \
    "mcp__fusion__health_status" '{}'

#-------------------------------------------------------------------------------
# Phase 2: Light — health + knowledge write/read
#-------------------------------------------------------------------------------
run_phase "Light - health" 50 10 \
    "mcp__fusion__health_status" '{}'

run_phase "Light - knowledge write" 50 10 \
    "mcp__fusion__knowledge_set" '{"key":"stress_test","value":"stress test value"}'

run_phase "Light - knowledge read" 50 10 \
    "mcp__fusion__knowledge_get" '{"key":"stress_test"}'

#-------------------------------------------------------------------------------
# Phase 3: Medium — sustained concurrent load
#-------------------------------------------------------------------------------
run_phase "Medium - health" 200 25 \
    "mcp__fusion__health_status" '{}'

run_phase "Medium - knowledge write" 200 25 \
    "mcp__fusion__knowledge_set" '{"key":"stress_test","value":"stress test value medium phase"}'

run_phase "Medium - knowledge read" 200 25 \
    "mcp__fusion__knowledge_get" '{"key":"stress_test"}'

#-------------------------------------------------------------------------------
# Phase 4: Heavy — high concurrency
#-------------------------------------------------------------------------------
run_phase "Heavy - health" 500 50 \
    "mcp__fusion__health_status" '{}'

run_phase "Heavy - knowledge write" 500 50 \
    "mcp__fusion__knowledge_set" '{"key":"stress_test","value":"heavy phase"}'

run_phase "Heavy - knowledge read" 500 50 \
    "mcp__fusion__knowledge_get" '{"key":"stress_test"}'

#-------------------------------------------------------------------------------
# Phase 5: Maximum — 1000 repeat, 50 concurrent
#-------------------------------------------------------------------------------
run_phase "Maximum - health" 1000 50 \
    "mcp__fusion__health_status" '{}'

run_phase "Maximum - knowledge" 1000 50 \
    "mcp__fusion__knowledge_get" '{"key":"stress_test"}'

#-------------------------------------------------------------------------------
# Phase 6: Echo provider (skip if not available)
#-------------------------------------------------------------------------------
log ""
log "=== Phase 6: Echo Provider (skipped if not available) ==="

if "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport http \
    -headers "Authorization:Bearer $APIKEY" \
    -call echo -params '{"message":"ping"}' > /dev/null 2>&1; then

    run_phase "Echo - warm-up" 50 10 \
        "echo" '{"message":"hello"}'

    run_phase "Echo - medium" 200 25 \
        "echo" '{"message":"stress test payload"}'

    run_phase "Echo - maximum" 1000 50 \
        "echo" '{"message":"maximum load"}'

    run_phase "Echo - delay (2s, light)" 10 5 \
        "delay" '{"seconds":2}'

    run_phase "Echo - delay (5s, concurrent)" 20 10 \
        "delay" '{"seconds":5}'

    run_phase "Echo - large response (10KB)" 100 20 \
        "random_data" '{"bytes":10240}'

    run_phase "Echo - large response (100KB)" 50 10 \
        "random_data" '{"bytes":102400}'

    run_phase "Echo - error handling" 100 20 \
        "error" '{"message":"test error"}'

    run_phase "Echo - counter (race detection)" 1000 50 \
        "counter" '{}'

else
    log "Echo provider not available — skipping Phase 6"
    log "(Deploy MCPFusion with echo provider enabled and re-run)"
fi

#-------------------------------------------------------------------------------
# Final report
#-------------------------------------------------------------------------------
log ""
log "=== Stress Test Summary ==="
log "Total phases passed: $PASS"
log "Total phases failed: $FAIL"

# Post-test memory
if [ -n "$PID" ] && [ -f /proc/$PID/status ]; then
    FINAL_MEM=$(awk '/VmRSS/{print $2}' /proc/$PID/status 2>/dev/null)
    DELTA=$(( FINAL_MEM - BASELINE_MEM ))
    log "MCPFusion RSS at end: ${FINAL_MEM} kB (delta: +${DELTA} kB)"
    if [ $DELTA -gt 51200 ]; then
        log -e "${YELLOW}WARNING: RSS grew by more than 50MB — possible memory leak${NC}"
    fi
fi

log "Completed: $(date)"
log "Results saved to: $OUTPUT_FILE"

if [ $FAIL -gt 0 ]; then
    exit 1
fi
