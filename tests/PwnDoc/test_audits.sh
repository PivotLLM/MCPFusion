#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

# PwnDoc Audit API Tests

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

echo "=== Testing PwnDoc Audit API ==="
echo "Timestamp: $(date)"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

# Test 1: List all audits
echo "Test 1: List all audits"
echo "Command: pwndoc_list_audits"
echo "Parameters: {}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call pwndoc_list_audits -params '{}'

echo ""
echo "=========================================="
echo ""

# Test 2: List audits with finding filter
echo "Test 2: List audits with finding title filter"
echo "Command: pwndoc_list_audits with finding_title filter"
echo "Parameters: {\"finding_title\": \"SQL\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call pwndoc_list_audits -params '{"finding_title": "SQL"}'

echo ""
echo "=========================================="
echo ""

echo "=== PwnDoc Audit API Tests Complete ==="
