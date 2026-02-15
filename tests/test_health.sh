#!/bin/bash

################################################################################
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                          #
# Please see LICENSE file for details.                                         #
################################################################################

# Health Tool Test Suite
# Tests the health MCP tool that returns server and service operational status.
#
# Test Numbering Convention:
#   1.x - Basic health response structure
#   2.x - Server fields validation
#   3.x - Service fields validation

#===============================================================================
# Configuration
#===============================================================================

# API key for authentication - set this before running
APIKEY="TEST API KEY HERE"

# Server URL (MCPFusion must be running)
SERVER_URL="http://127.0.0.1:9999/mcp"
TRANSPORT="http"

# PROBE can be overridden via environment variable
: "${PROBE:=probe}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

PASS_COUNT=0
FAIL_COUNT=0

#===============================================================================
# Pre-flight Checks
#===============================================================================

echo ""
echo "${BOLD}============================================${NC}"
echo "${BOLD}   Health Tool Test Suite${NC}"
echo "${BOLD}============================================${NC}"
echo ""

# Check if APIKEY has been set
if [ "$APIKEY" = "SET_YOUR_API_KEY_HERE" ]; then
    echo "${RED}ERROR: APIKEY has not been set${NC}"
    echo "Edit this script and set the APIKEY variable to a valid API token."
    echo "Generate a token with: ./mcpfusion -token-add \"test\""
    exit 1
fi

# Check if PROBE exists
if [ "${PROBE#/}" = "$PROBE" ]; then
    PROBE_FULL=$(command -v "$PROBE" 2>/dev/null)
    if [ -z "$PROBE_FULL" ]; then
        echo "${RED}ERROR: probe binary not found in PATH${NC}"
        exit 1
    fi
    PROBE="$PROBE_FULL"
elif [ ! -f "$PROBE" ]; then
    echo "${RED}ERROR: probe not found at: $PROBE${NC}"
    exit 1
fi

if [ ! -x "$PROBE" ]; then
    echo "${RED}ERROR: probe is not executable: $PROBE${NC}"
    exit 1
fi

echo "${GREEN}Pre-flight checks passed${NC}"
echo "  Probe:  $PROBE"
echo "  Server: $SERVER_URL"
echo ""

#===============================================================================
# Helper Functions
#===============================================================================

print_section() {
    echo ""
    echo "${BOLD}${BLUE}============================================${NC}"
    echo "${BOLD}${BLUE}   $1${NC}"
    echo "${BOLD}${BLUE}============================================${NC}"
    echo ""
}

print_subsection() {
    echo "${CYAN}--- $1 ---${NC}"
}

# Run a test expecting success, optionally check for expected string
run_test() {
    local test_name="$1"
    local tool="$2"
    local params="$3"
    local expected="$4"

    echo "  ${test_name}"
    result=$($PROBE -url "$SERVER_URL" -transport "$TRANSPORT" -headers "Authorization:Bearer $APIKEY" -call "$tool" -params "$params" 2>&1)

    if echo "$result" | grep -q "Tool call succeeded"; then
        if [ -n "$expected" ]; then
            if echo "$result" | grep -q "$expected"; then
                echo "    ${GREEN}PASS${NC}: Found expected: $expected"
                PASS_COUNT=$((PASS_COUNT + 1))
            else
                echo "    ${RED}FAIL${NC}: Expected '$expected' not found"
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

#===============================================================================
# SECTION 1: Basic Health Response
#===============================================================================

print_section "SECTION 1: Basic Health Response"

print_subsection "1.1 Tool invocation"

run_test "1.1.1 Health tool returns successfully with no parameters" \
    "health" \
    '{}' \
    ""

run_test "1.1.2 Response contains server object" \
    "health" \
    '{}' \
    '"server"'

run_test "1.1.3 Response contains services array" \
    "health" \
    '{}' \
    '"services"'

#===============================================================================
# SECTION 2: Server Fields
#===============================================================================

print_section "SECTION 2: Server Fields"

print_subsection "2.1 Server identity"

run_test "2.1.1 Server name is MCPFusion" \
    "health" \
    '{}' \
    '"name": "MCPFusion"'

run_test "2.1.2 Server version is present" \
    "health" \
    '{}' \
    '"version"'

print_subsection "2.2 Server status"

run_test "2.2.1 Server status field is present" \
    "health" \
    '{}' \
    '"status"'

run_test "2.2.2 Server uptime field is present" \
    "health" \
    '{}' \
    '"uptime"'

#===============================================================================
# SECTION 3: Service Fields
#===============================================================================

print_section "SECTION 3: Service Fields"

print_subsection "3.1 Service entries"

run_test "3.1.1 Services contain name field" \
    "health" \
    '{}' \
    '"name"'

run_test "3.1.2 Services contain type field" \
    "health" \
    '{}' \
    '"type"'

run_test "3.1.3 Services contain status field" \
    "health" \
    '{}' \
    '"status"'

#===============================================================================
# Summary
#===============================================================================

echo ""
echo "${BOLD}============================================${NC}"
echo "${BOLD}   Test Summary${NC}"
echo "${BOLD}============================================${NC}"
echo ""
echo "  ${GREEN}Passed: $PASS_COUNT${NC}"
echo "  ${RED}Failed: $FAIL_COUNT${NC}"
TOTAL=$((PASS_COUNT + FAIL_COUNT))
echo "  Total:  $TOTAL"
echo ""

if [ "$FAIL_COUNT" -gt 0 ]; then
    echo "${RED}SOME TESTS FAILED${NC}"
    exit 1
else
    echo "${GREEN}ALL TESTS PASSED${NC}"
    exit 0
fi
