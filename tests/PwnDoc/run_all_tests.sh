#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

# PwnDoc Test Runner
# Executes all individual tests and saves output to separate .log files

# Configuration
TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROBE_PATH="${PROBE_PATH:-probe}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Load environment variables
if [ -f "$TESTS_DIR/.env" ]; then
    source "$TESTS_DIR/.env"
else
    echo -e "${RED}[ERROR]${NC} .env file not found in $TESTS_DIR"
    echo "Please create a .env file with APIKEY=your-api-token and SERVER_URL=your-server-url"
    exit 1
fi

# Check if APIKEY is set
if [ -z "$APIKEY" ]; then
    echo -e "${RED}[ERROR]${NC} APIKEY not set in .env file"
    exit 1
fi

# Check if SERVER_URL is set
if [ -z "$SERVER_URL" ]; then
    echo -e "${RED}[ERROR]${NC} SERVER_URL not set in .env file"
    echo "Please add SERVER_URL=your-server-url to .env file"
    exit 1
fi

# Append /mcp to the base URL
SERVER_URL="${SERVER_URL}/mcp"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== PwnDoc Test Suite Runner ===${NC}"
echo "Timestamp: $(date)"
echo "Tests Directory: $TESTS_DIR"
echo "Server URL: $SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

# Check prerequisites
echo -e "${BLUE}[INFO]${NC} Checking prerequisites..."

if ! command -v "$PROBE_PATH" > /dev/null 2>&1; then
    echo -e "${RED}[ERROR]${NC} Probe tool not found: $PROBE_PATH"
    echo "Install probe or set PROBE_PATH to the full path of the probe binary"
    exit 1
fi

echo -e "${GREEN}[PASS]${NC} Prerequisites check complete"
echo ""

# Test server connectivity
echo -e "${BLUE}[INFO]${NC} Testing server connectivity..."
if ! "$PROBE_PATH" -url "$SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -list-only >/dev/null 2>&1; then
    echo -e "${RED}[ERROR]${NC} Cannot connect to MCP server at $SERVER_URL"
    echo "Please ensure the MCPFusion server is running"
    exit 1
fi

echo -e "${GREEN}[PASS]${NC} Server is available"
echo ""

# Make test scripts executable
chmod +x "$TESTS_DIR"/*.sh

# Run individual tests
tests_run=0
tests_passed=0
tests_failed=0

run_test() {
    local test_name="$1"
    local test_script="$2"
    local log_file="$3"

    ((tests_run++))
    echo -e "${BLUE}[INFO]${NC} Running $test_name..."
    echo "Output file: $log_file"

    if "$test_script" > "$log_file" 2>&1; then
        echo -e "${GREEN}[PASS]${NC} $test_name completed successfully"
        ((tests_passed++))
    else
        echo -e "${RED}[FAIL]${NC} $test_name failed"
        echo "Check $log_file for details"
        ((tests_failed++))
    fi
    echo ""
}

# Execute all tests
echo -e "${BLUE}[INFO]${NC} Starting test execution..."
echo ""

run_test "Audits API Test" \
    "$TESTS_DIR/test_audits.sh" \
    "$TESTS_DIR/audits_test_${TIMESTAMP}.log"

run_test "Findings API Test" \
    "$TESTS_DIR/test_findings.sh" \
    "$TESTS_DIR/findings_test_${TIMESTAMP}.log"

run_test "Clients API Test" \
    "$TESTS_DIR/test_clients.sh" \
    "$TESTS_DIR/clients_test_${TIMESTAMP}.log"

run_test "Object Array Parameters Regression Test" \
    "$TESTS_DIR/test_object_array_params.sh" \
    "$TESTS_DIR/object_array_params_test_${TIMESTAMP}.log"

# Summary
echo -e "${BLUE}=== Test Summary ===${NC}"
echo "Total Tests: $tests_run"
echo -e "${GREEN}Passed: $tests_passed${NC}"
echo -e "${RED}Failed: $tests_failed${NC}"

if [ $tests_failed -eq 0 ]; then
    echo -e "${GREEN}[SUCCESS]${NC} All tests passed!"
    echo ""
    echo "Test output files created in $TESTS_DIR:"
    ls -la "$TESTS_DIR"/*_${TIMESTAMP}.log
    exit 0
else
    echo -e "${RED}[FAILURE]${NC} Some tests failed"
    echo ""
    echo "Check the following log files for details:"
    ls -la "$TESTS_DIR"/*_${TIMESTAMP}.log
    exit 1
fi
