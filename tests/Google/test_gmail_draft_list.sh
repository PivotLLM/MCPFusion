#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

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

# Append /sse to the base URL
FULL_SERVER_URL="${SERVER_URL}/sse"

# Test Google Gmail Draft List API
echo "=== Testing Google Gmail Draft List API ==="
echo "Timestamp: $(date)"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

echo "Test 1: List drafts with defaults"
echo "Command: google_gmail_draft_list with default parameters"
echo "Parameters: {}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call google_gmail_draft_list -params '{}'

echo ""
echo "=========================================="
echo ""

echo "Test 2: List drafts with maxResults 5"
echo "Command: google_gmail_draft_list with maxResults 5"
echo "Parameters: {\"maxResults\": \"5\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call google_gmail_draft_list -params '{"maxResults": "5"}'

echo ""
echo "=========================================="
echo ""

echo "Test 3: List drafts with search query"
echo "Command: google_gmail_draft_list with query filter"
echo "Parameters: {\"q\": \"subject:meeting\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call google_gmail_draft_list -params '{"q": "subject:meeting"}'

echo ""
echo "=== Gmail Draft List API Tests Complete ==="
