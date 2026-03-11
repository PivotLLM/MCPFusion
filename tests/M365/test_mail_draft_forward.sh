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

# Test Microsoft 365 Mail Draft Forward API
echo "=== Testing Microsoft 365 Mail Draft Forward API ==="
echo "Timestamp: $(date)"
FULL_SERVER_URL="${SERVER_URL}/mcp"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

echo "Test 1: Create forward draft with comment and toRecipients"
echo "Command: microsoft365_mail_draft_forward with messageId, comment, and toRecipients"
echo "Parameters: {\"messageId\": \"MESSAGE_ID_HERE\", \"comment\": \"FYI - Please review the details below and let me know if you have any questions.\", \"toRecipients\": \"colleague@example.com,team-lead@example.com\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_draft_forward -params '{"messageId": "MESSAGE_ID_HERE", "comment": "FYI - Please review the details below and let me know if you have any questions.", "toRecipients": "colleague@example.com,team-lead@example.com"}'

echo ""
echo "=========================================="
echo ""

echo "Test 2: Create forward draft without comment"
echo "Command: microsoft365_mail_draft_forward with messageId and toRecipients only"
echo "Parameters: {\"messageId\": \"MESSAGE_ID_HERE\", \"toRecipients\": \"user@example.com\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_draft_forward -params '{"messageId": "MESSAGE_ID_HERE", "toRecipients": "user@example.com"}'

echo ""
echo "=== Mail Draft Forward API Tests Complete ==="
