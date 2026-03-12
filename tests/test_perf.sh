#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

# Perf Provider Functional Tests
#
# Tests each perf tool for correct behaviour. Requires the perf provider to be
# enabled (MCP_FUSION_PERF=true).
#
# Test Numbering Convention:
#   1.x - perf_echo
#   2.x - perf_delay
#   3.x - perf_counter
#   4.x - perf_error
#   5.x - perf_random_data

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ -f "$SCRIPT_DIR/.env" ]; then
    source "$SCRIPT_DIR/.env"
else
    echo "Error: .env file not found in $SCRIPT_DIR"
    echo "Please create a .env file with APIKEY=your-api-token and SERVER_URL=your-server-url"
    exit 1
fi

[ -z "$APIKEY" ]     && { echo "Error: APIKEY not set in .env";     exit 1; }
[ -z "$SERVER_URL" ] && { echo "Error: SERVER_URL not set in .env"; exit 1; }

PROBE_PATH="${PROBE_PATH:-probe}"
FULL_SERVER_URL="${SERVER_URL}/mcp"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

PASS_COUNT=0
FAIL_COUNT=0

echo ""
echo "${BOLD}============================================${NC}"
echo "${BOLD}   Perf Provider Functional Tests${NC}"
echo "${BOLD}============================================${NC}"
echo "Server: $FULL_SERVER_URL"
echo ""

# Check probe binary
if ! command -v "$PROBE_PATH" > /dev/null 2>&1; then
    echo "${RED}ERROR: probe not found at '$PROBE_PATH'${NC}"
    exit 1
fi

#-------------------------------------------------------------------------------
# Helpers
#-------------------------------------------------------------------------------

print_section() {
    echo ""
    echo "${BOLD}${BLUE}============================================${NC}"
    echo "${BOLD}${BLUE}   $1${NC}"
    echo "${BOLD}${BLUE}============================================${NC}"
    echo ""
}

# run_test_ok <name> <tool> <params> [expected_string]
# Expects tool call to succeed; optionally checks for a string in the output.
run_test_ok() {
    local test_name="$1"
    local tool="$2"
    local params="$3"
    local expected="$4"

    echo "  ${test_name}"
    result=$("$PROBE_PATH" -url "$FULL_SERVER_URL" -transport http \
        -headers "Authorization:Bearer $APIKEY" \
        -call "$tool" -params "$params" 2>&1)

    if echo "$result" | grep -q "Tool call succeeded"; then
        if [ -n "$expected" ]; then
            if echo "$result" | grep -q "$expected"; then
                echo "    ${GREEN}PASS${NC}: Found expected: $expected"
                PASS_COUNT=$((PASS_COUNT + 1))
            else
                echo "    ${RED}FAIL${NC}: Expected '$expected' not found in output"
                echo "    Output: $result"
                FAIL_COUNT=$((FAIL_COUNT + 1))
            fi
        else
            echo "    ${GREEN}PASS${NC}: Tool call succeeded"
            PASS_COUNT=$((PASS_COUNT + 1))
        fi
    else
        echo "    ${RED}FAIL${NC}: Tool call failed unexpectedly"
        echo "    Output: $result"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
}

# run_test_err <name> <tool> <params>
# Expects tool call to return an error (non-zero exit or error in output).
run_test_err() {
    local test_name="$1"
    local tool="$2"
    local params="$3"

    echo "  ${test_name}"
    result=$("$PROBE_PATH" -url "$FULL_SERVER_URL" -transport http \
        -headers "Authorization:Bearer $APIKEY" \
        -call "$tool" -params "$params" 2>&1)

    # Probe reports errors as "Tool call failed" or "error" in output
    if echo "$result" | grep -qiE "Tool call failed|\"error\"|isError|error:"; then
        echo "    ${GREEN}PASS${NC}: Tool returned error as expected"
        PASS_COUNT=$((PASS_COUNT + 1))
    else
        echo "    ${RED}FAIL${NC}: Expected an error but tool succeeded"
        echo "    Output: $result"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
}

#-------------------------------------------------------------------------------
# Check that perf provider is available
#-------------------------------------------------------------------------------

echo "${CYAN}Checking perf provider availability...${NC}"
if ! "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport http \
    -headers "Authorization:Bearer $APIKEY" \
    -call "perf_echo" -params '{"message":"ping"}' > /dev/null 2>&1; then
    echo "${YELLOW}WARNING: perf_echo not available — is MCP_FUSION_PERF=true?${NC}"
    echo "Skipping perf tests."
    exit 0
fi
echo "${GREEN}Perf provider is available${NC}"

#-------------------------------------------------------------------------------
# Section 1: perf_echo
#-------------------------------------------------------------------------------

print_section "SECTION 1: perf_echo"

run_test_ok "1.1 Basic echo returns successfully" \
    "perf_echo" '{"message":"hello world"}' ""

run_test_ok "1.2 Response contains message field" \
    "perf_echo" '{"message":"hello world"}' '"message"'

run_test_ok "1.3 Response echoes the input value" \
    "perf_echo" '{"message":"test payload 123"}' "test payload 123"

run_test_ok "1.4 Empty string is accepted" \
    "perf_echo" '{"message":""}' '"message"'

#-------------------------------------------------------------------------------
# Section 2: perf_delay
#-------------------------------------------------------------------------------

print_section "SECTION 2: perf_delay"

run_test_ok "2.1 Delay of 0 seconds returns immediately" \
    "perf_delay" '{"seconds":0}' '"slept_seconds"'

run_test_ok "2.2 Delay of 1 second returns successfully" \
    "perf_delay" '{"seconds":1}' '"slept_seconds"'

run_test_ok "2.3 Response contains slept_seconds field" \
    "perf_delay" '{"seconds":1}' '"slept_seconds":1'

#-------------------------------------------------------------------------------
# Section 3: perf_counter
#-------------------------------------------------------------------------------

print_section "SECTION 3: perf_counter"

run_test_ok "3.1 Counter returns successfully" \
    "perf_counter" '{}' '"count"'

run_test_ok "3.2 Second call returns a count" \
    "perf_counter" '{}' '"count"'

#-------------------------------------------------------------------------------
# Section 4: perf_error
#-------------------------------------------------------------------------------

print_section "SECTION 4: perf_error"

run_test_err "4.1 Default error message returns an error" \
    "perf_error" '{}'

run_test_err "4.2 Custom error message returns an error" \
    "perf_error" '{"message":"custom error"}'

#-------------------------------------------------------------------------------
# Section 5: perf_random_data
#-------------------------------------------------------------------------------

print_section "SECTION 5: perf_random_data"

run_test_ok "5.1 Request 64 bytes returns successfully" \
    "perf_random_data" '{"bytes":64}' '"bytes":64'

run_test_ok "5.2 Response contains data field" \
    "perf_random_data" '{"bytes":64}' '"data"'

run_test_ok "5.3 Request 1024 bytes" \
    "perf_random_data" '{"bytes":1024}' '"bytes":1024'

run_test_ok "5.4 Request 0 bytes returns empty data" \
    "perf_random_data" '{"bytes":0}' '"bytes":0'

run_test_ok "5.5 Request exceeding cap is clamped to 1MiB" \
    "perf_random_data" '{"bytes":2097152}' '"bytes":1048576'

#-------------------------------------------------------------------------------
# Summary
#-------------------------------------------------------------------------------

echo ""
echo "${BOLD}============================================${NC}"
echo "${BOLD}   Test Summary${NC}"
echo "${BOLD}============================================${NC}"
echo ""
echo "  ${GREEN}Passed: $PASS_COUNT${NC}"
echo "  ${RED}Failed: $FAIL_COUNT${NC}"
echo "  Total:  $((PASS_COUNT + FAIL_COUNT))"
echo ""

if [ "$FAIL_COUNT" -gt 0 ]; then
    echo "${RED}SOME TESTS FAILED${NC}"
    exit 1
else
    echo "${GREEN}ALL TESTS PASSED${NC}"
    exit 0
fi
