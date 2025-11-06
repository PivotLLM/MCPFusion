#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025 Tenebris Technologies Inc.                                *
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

# Test Microsoft 365 Calendar Summary API
echo "=== Testing Microsoft 365 Calendar Summary API ===" 
echo "Timestamp: $(date)"
FULL_SERVER_URL="${SERVER_URL}/sse"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

# Calculate date ranges
start_date=$(date -v-30d +%Y%m%d)  # 30 days ago
end_date=$(date +%Y%m%d)           # Today
future_start=$(date +%Y%m%d)       # Today
future_end=$(date -v+30d +%Y%m%d)  # 30 days from now

echo "Test 1: Calendar summary (last 30 days)"
echo "Command: microsoft365_calendar_read_summary"
echo "Parameters: {\"startDate\": \"$start_date\", \"endDate\": \"$end_date\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_read_summary -params "{\"startDate\": \"$start_date\", \"endDate\": \"$end_date\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 2: Calendar summary (next 30 days)"
echo "Command: microsoft365_calendar_read_summary for future events"
echo "Parameters: {\"startDate\": \"$future_start\", \"endDate\": \"$future_end\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_read_summary -params "{\"startDate\": \"$future_start\", \"endDate\": \"$future_end\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 3: Calendar summary (specific date range - July 2024)"
echo "Command: microsoft365_calendar_read_summary"
echo "Parameters: {\"startDate\": \"20240701\", \"endDate\": \"20240731\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_read_summary -params '{"startDate": "20240701", "endDate": "20240731"}'

echo ""
echo "=== Calendar Summary API Tests Complete ==="