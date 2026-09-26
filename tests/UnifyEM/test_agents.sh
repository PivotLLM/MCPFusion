#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

# UnifyEM agent tests: read-only listing and lookup tools.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ -f "$SCRIPT_DIR/.env" ]; then
    source "$SCRIPT_DIR/.env"
else
    echo "Error: .env file not found in $SCRIPT_DIR"
    echo "Please create a .env file with APIKEY=your-api-token and SERVER_URL=your-server-url"
    exit 1
fi

if [ -z "$APIKEY" ] || [ -z "$SERVER_URL" ]; then
    echo "Error: APIKEY and SERVER_URL must be set in .env file"
    exit 1
fi

PROBE_PATH="${PROBE_PATH:-probe}"
FULL_SERVER_URL="${SERVER_URL}/mcp"

echo "=== Testing UnifyEM Agents ==="
echo "Timestamp: $(date)"
echo "Server: $FULL_SERVER_URL"
echo ""

echo "Test 1: unifyem_agent_list"
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call unifyem_agent_list -params '{}'

echo ""
echo "=========================================="
echo ""
echo "Test 2: unifyem_agent_list_by_tag (tag from UEM_TEST_TAG, default 'test')"
TAG="${UEM_TEST_TAG:-test}"
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call unifyem_agent_list_by_tag -params "{\"tag\": \"$TAG\"}"

echo ""
echo "=========================================="
echo ""
echo "Test 3: unifyem_request_list"
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call unifyem_request_list -params '{}'

echo ""
echo "=== UnifyEM Agent Tests Complete ==="
