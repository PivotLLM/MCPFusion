#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           *
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
PROBE_PATH="${PROBE_PATH:-probe}"

# Append /mcp to the base URL
FULL_SERVER_URL="${SERVER_URL}/mcp"

# MCPFusion Integration Test audit ID
AUDIT_ID="696c3f8051d8f95c85499d33"
FINDING_ID="696c3f9151d8f95c85499d3d"

echo "=== Testing PwnDoc Finding API ==="
echo "Timestamp: $(date)"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

# Test 1: List findings for a known audit
echo "Test 1: List findings for audit"
echo "Command: pwndoc_list_audit_findings"
echo "Parameters: {\"audit_id\": \"$AUDIT_ID\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call pwndoc_list_audit_findings -params "{\"audit_id\": \"$AUDIT_ID\"}"

echo ""
echo "=========================================="
echo ""

# Test 2: Get a specific finding
echo "Test 2: Get specific finding"
echo "Command: pwndoc_get_finding"
echo "Parameters: {\"audit_id\": \"$AUDIT_ID\", \"finding_id\": \"$FINDING_ID\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call pwndoc_get_finding -params "{\"audit_id\": \"$AUDIT_ID\", \"finding_id\": \"$FINDING_ID\"}"

echo ""
echo "=========================================="
echo ""

echo "=== PwnDoc Finding API Tests Complete ==="
