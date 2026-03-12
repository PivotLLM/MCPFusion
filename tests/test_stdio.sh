#!/bin/bash

################################################################################
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                          #
# Please see LICENSE file for details.                                         #
################################################################################

# Stdio Hub Service Test Suite
# Tests that a configured stdio hub service appears correctly in the health
# status returned by the health_status tool.
#
# REQUIREMENTS:
#   - A running MCPFusion server
#   - At least one stdio hub service configured in the server's config files
#     (e.g., configs/maestro.json or configs/playwright.json)
#   - The STDIO_SERVICE_NAME variable must match a service name visible in the
#     health_status response (default: "maestro")
#
# Test Numbering Convention:
#   1.x - Transport presence in health response
#   2.x - Service name presence in health response
#   3.x - Service operational status

#===============================================================================
# Configuration
#===============================================================================

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

SERVER_URL="${SERVER_URL}/mcp"
TRANSPORT="http"

# Name of the stdio hub service to verify (overridable via environment variable)
: "${STDIO_SERVICE_NAME:=maestro}"

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
echo "${BOLD}   Stdio Hub Service Test Suite${NC}"
echo "${BOLD}============================================${NC}"
echo ""

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
echo "  Probe:        $PROBE"
echo "  Server:       $SERVER_URL"
echo "  Service name: $STDIO_SERVICE_NAME"
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
# SECTION 1: Transport Presence
#===============================================================================

print_section "SECTION 1: Transport Presence"

print_subsection "1.1 Stdio transport in health response"

run_test "1.1.1 Response contains mcp_stdio transport" \
    "health_status" \
    '{}' \
    '"mcp_stdio"'

#===============================================================================
# SECTION 2: Service Name Presence
#===============================================================================

print_section "SECTION 2: Service Name Presence"

print_subsection "2.1 Stdio service name in health response"

run_test "2.1.1 Response contains configured stdio service name" \
    "health_status" \
    '{}' \
    "\"$STDIO_SERVICE_NAME\""

#===============================================================================
# SECTION 3: Service Operational Status
#===============================================================================

print_section "SECTION 3: Service Operational Status"

print_subsection "3.1 Stdio service status"

run_test "3.1.1 Stdio service shows operational status" \
    "health_status" \
    '{}' \
    '"operational"'

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
