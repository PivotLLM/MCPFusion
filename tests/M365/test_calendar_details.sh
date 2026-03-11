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

# Test Microsoft 365 Calendar Details API
FULL_SERVER_URL="${SERVER_URL}/mcp"

echo "=== Testing Microsoft 365 Calendar Details API ===" 
echo "Timestamp: $(date)"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

# Calculate date ranges
start_date=$(date -v-30d +%Y%m%d 2>/dev/null || date -d "-30 days" +%Y%m%d)  # 30 days ago
end_date=$(date +%Y%m%d)           # Today
future_start=$(date +%Y%m%d)       # Today
future_end=$(date -v+30d +%Y%m%d 2>/dev/null || date -d "+30 days" +%Y%m%d)  # 30 days from now

echo "Test 1: Calendar details (last 30 days)"
echo "Command: microsoft365_calendar_read_details"
echo "Parameters: {\"startDate\": \"$start_date\", \"endDate\": \"$end_date\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_read_details -params "{\"startDate\": \"$start_date\", \"endDate\": \"$end_date\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 2: Calendar details with custom fields"
echo "Command: microsoft365_calendar_read_details with field selection"
echo "Parameters: {\"startDate\": \"$start_date\", \"endDate\": \"$end_date\", \"\\$select\": \"subject,start,end,organizer,location\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_read_details -params "{\"startDate\": \"$start_date\", \"endDate\": \"$end_date\", \"\$select\": \"subject,start,end,organizer,location\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 3: Calendar details (next 30 days)"
echo "Command: microsoft365_calendar_read_details for future events"
echo "Parameters: {\"startDate\": \"$future_start\", \"endDate\": \"$future_end\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_read_details -params "{\"startDate\": \"$future_start\", \"endDate\": \"$future_end\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 4: Calendar details with minimal fields"
echo "Command: microsoft365_calendar_read_details with minimal field selection"
echo "Parameters: {\"startDate\": \"$start_date\", \"endDate\": \"$end_date\", \"\\$select\": \"subject,start,end\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_read_details -params "{\"startDate\": \"$start_date\", \"endDate\": \"$end_date\", \"\$select\": \"subject,start,end\"}"

echo ""
echo "=== Calendar Details API Tests Complete ==="