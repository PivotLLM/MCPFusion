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

# Test Microsoft 365 Calendars List API
FULL_SERVER_URL="${SERVER_URL}/sse"

echo "=== Testing Microsoft 365 Calendars List API ===" 
echo "Timestamp: $(date)"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

echo "Test 1: List all calendars (default fields)"
echo "Command: microsoft365_calendars_list"
echo "Parameters: {}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendars_list -params "{}"

echo ""
echo "=========================================="
echo ""

echo "Test 2: List calendars with custom fields"
echo "Command: microsoft365_calendars_list"
echo "Parameters: {\"\$select\": \"name,id,color,canEdit,canShare,isDefaultCalendar\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendars_list -params '{"$select": "name,id,color,canEdit,canShare,isDefaultCalendar"}'

echo ""
echo "=========================================="
echo ""

echo "Test 3: List calendars with pagination"
echo "Command: microsoft365_calendars_list"
echo "Parameters: {\"\$top\": \"5\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendars_list -params '{"$top": "5"}'

echo ""
echo "=== Calendars List API Tests Complete ==="