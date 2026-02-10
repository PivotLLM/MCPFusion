#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

# Load environment variables
if [ -f ".env" ]; then
    source .env
else
    echo "Error: .env file not found"
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

# Test Microsoft 365 Mail Draft Reply All API
echo "=== Testing Microsoft 365 Mail Draft Reply All API ==="
echo "Timestamp: $(date)"
FULL_SERVER_URL="${SERVER_URL}/sse"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

echo "Test 1: Create reply-all draft with comment"
echo "Command: microsoft365_mail_draft_reply_all with messageId and comment"
echo "Parameters: {\"messageId\": \"MESSAGE_ID_HERE\", \"comment\": \"Adding my input for the entire team. I agree with the proposed timeline and resource allocation.\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_draft_reply_all -params '{"messageId": "MESSAGE_ID_HERE", "comment": "Adding my input for the entire team. I agree with the proposed timeline and resource allocation."}'

echo ""
echo "=========================================="
echo ""

echo "Test 2: Create reply-all draft without comment"
echo "Command: microsoft365_mail_draft_reply_all with messageId only"
echo "Parameters: {\"messageId\": \"MESSAGE_ID_HERE\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_draft_reply_all -params '{"messageId": "MESSAGE_ID_HERE"}'

echo ""
echo "=== Mail Draft Reply All API Tests Complete ==="
