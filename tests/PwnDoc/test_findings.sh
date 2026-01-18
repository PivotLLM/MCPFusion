#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025 Tenebris Technologies Inc.                                *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

# PwnDoc Finding API Tests

# Get the directory of the script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load environment variables
if [ -f "$SCRIPT_DIR/.env" ]; then
    source "$SCRIPT_DIR/.env"
else
    echo "Error: .env file not found in $SCRIPT_DIR"
    echo "Please create a .env file with APIKEY=your-api-token and SERVER_URL=your-server-url"
    exit 1
fi

# Check if APIKEY is set
if [ -z "$APIKEY" ]; then
    echo "Error: APIKEY not set in .env file"
    exit 1
fi

# Check if SERVER_URL is set
if [ -z "$SERVER_URL" ]; then
    echo "Error: SERVER_URL not set in .env file"
    exit 1
fi

# Check if PROBE_PATH is set, otherwise use default
PROBE_PATH="${PROBE_PATH:-/Users/eric/source/MCPProbe/probe}"

# Append /sse to the base URL
FULL_SERVER_URL="${SERVER_URL}/sse"

echo "=== Testing PwnDoc Finding API ==="
echo "Timestamp: $(date)"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

# Test 1: Search findings
echo "Test 1: Search all findings"
echo "Command: pwndoc_search_findings"
echo "Parameters: {}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call pwndoc_search_findings -params '{}'

echo ""
echo "=========================================="
echo ""

# Test 2: Search findings by severity
echo "Test 2: Search findings by severity"
echo "Command: pwndoc_search_findings with severity filter"
echo "Parameters: {\"severity\": \"Critical\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call pwndoc_search_findings -params '{"severity": "Critical"}'

echo ""
echo "=========================================="
echo ""

# Test 3: Get all findings with context
echo "Test 3: Get all findings with full context"
echo "Command: pwndoc_get_all_findings_with_context"
echo "Parameters: {\"include_failed\": false}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call pwndoc_get_all_findings_with_context -params '{"include_failed": false}'

echo ""
echo "=========================================="
echo ""

echo "=== PwnDoc Finding API Tests Complete ==="
