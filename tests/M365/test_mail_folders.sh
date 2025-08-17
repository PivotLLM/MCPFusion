#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025 Tenebris Technologies Inc.                                *
# All rights reserved.                                                         *
#*******************************************************************************

# Load environment variables
if [ -f ".env" ]; then
    source .env
else
    echo "Error: .env file not found"
    echo "Please create a .env file with APIKEY=your-api-token"
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

# Test Microsoft 365 Mail Folders API
FULL_SERVER_URL="${SERVER_URL}/sse"

echo "=== Testing Microsoft 365 Mail Folders API ===" 
echo "Timestamp: $(date)"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

echo "Test 1: List all mail folders (default fields)"
echo "Command: microsoft365_mail_folders_list"
echo "Parameters: {}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_folders_list -params "{}"

echo ""
echo "=========================================="
echo ""

echo "Test 2: List mail folders with custom fields"
echo "Command: microsoft365_mail_folders_list"
echo "Parameters: {\"\$select\": \"displayName,id,unreadItemCount,totalItemCount\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_folders_list -params '{"$select": "displayName,id,unreadItemCount,totalItemCount"}'

echo ""
echo "=========================================="
echo ""

echo "Test 3: List mail folders with pagination"
echo "Command: microsoft365_mail_folders_list"
echo "Parameters: {\"\$top\": \"10\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_folders_list -params '{"$top": "10"}'

echo ""
echo "=== Mail Folders API Tests Complete ==="