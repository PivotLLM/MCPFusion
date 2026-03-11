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

# Check if PROBE_PATH is set, otherwise use default
PROBE_PATH="${PROBE_PATH:-probe}"

# Test Microsoft 365 Mail API
echo "=== Testing Microsoft 365 Mail API ==="
echo "Timestamp: $(date)"
FULL_SERVER_URL="${SERVER_URL}/mcp"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

echo "Test 1: Default inbox messages"
echo "Command: microsoft365_mail_read_inbox with default parameters"
echo "Parameters: {}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_read_inbox -params '{}'

echo ""
echo "=========================================="
echo ""

echo "Test 2: Limited message count (5 messages)"
echo "Command: microsoft365_mail_read_inbox with top 5"
echo "Parameters: {\"\\$top\": \"5\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_read_inbox -params '{"$top": "5"}'

echo ""
echo "=========================================="
echo ""

echo "Test 3: Unread messages only"
echo "Command: microsoft365_mail_read_inbox filtered for unread"
echo "Parameters: {\"\\$filter\": \"isRead eq false\", \"\\$top\": \"10\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_read_inbox -params '{"$filter": "isRead eq false", "$top": "10"}'

echo ""
echo "=========================================="
echo ""

echo "Test 4: Custom fields selection"
echo "Command: microsoft365_mail_read_inbox with custom fields"
echo "Parameters: {\"\\$select\": \"subject,from,receivedDateTime,isRead\", \"\\$top\": \"5\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_read_inbox -params '{"$select": "subject,from,receivedDateTime,isRead", "$top": "5"}'

echo ""
echo "=== Mail API Tests Complete ==="