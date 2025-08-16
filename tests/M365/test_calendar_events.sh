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
    echo "Please create a .env file with APIKEY=your-api-token"
    exit 1
fi

# Check if APIKEY is set
if [ -z "$APIKEY" ]; then
    echo "Error: APIKEY not set in .env file"
    exit 1
fi

# Test Microsoft 365 Calendar Events API (calendar-specific)
echo "=== Testing Microsoft 365 Calendar Events API ===" 
echo "Timestamp: $(date)"
echo "Server: http://127.0.0.1:8888/sse"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

# First get calendar list to find a calendar ID
echo "Step 1: Getting calendar list to find calendar ID..."
calendar_response=$(/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendars_list -params '{"$top": "3"}' 2>&1)

# Check if the call succeeded
if echo "$calendar_response" | grep -q "error"; then
    echo "Error getting calendar list:"
    echo "$calendar_response" | grep -i error
    echo ""
    echo "Please ensure you are authenticated and have calendar permissions"
    exit 1
fi

# Extract the first calendar ID using grep and sed
# Looking for pattern: "id": "value"
CALENDAR_ID=$(echo "$calendar_response" | grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')

if [ -z "$CALENDAR_ID" ]; then
    echo "Warning: Could not extract calendar ID from response"
    echo "Response was:"
    echo "$calendar_response" | head -20
    echo ""
    echo "Using default calendar ID (may fail if not available)"
    CALENDAR_ID="calendar"
else
    echo "Found calendar ID: $CALENDAR_ID"
fi
echo ""

# Calculate date ranges
start_date=$(date -v-7d +%Y%m%d)   # 7 days ago
end_date=$(date -v+7d +%Y%m%d)     # 7 days from now

echo "Test 1: Calendar events summary for specific calendar"
echo "Command: microsoft365_calendar_events_read_summary"
echo "Parameters: {\"calendarId\": \"$CALENDAR_ID\", \"startDate\": \"$start_date\", \"endDate\": \"$end_date\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_events_read_summary -params "{\"calendarId\": \"$CALENDAR_ID\", \"startDate\": \"$start_date\", \"endDate\": \"$end_date\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 2: Calendar events details for specific calendar"
echo "Command: microsoft365_calendar_events_read_details"
echo "Parameters: {\"calendarId\": \"$CALENDAR_ID\", \"startDate\": \"$start_date\", \"endDate\": \"$end_date\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_events_read_details -params "{\"calendarId\": \"$CALENDAR_ID\", \"startDate\": \"$start_date\", \"endDate\": \"$end_date\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 3: Calendar events without date filtering (default time range)"
echo "Command: microsoft365_calendar_events_read_summary"
echo "Parameters: {\"calendarId\": \"$CALENDAR_ID\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_events_read_summary -params "{\"calendarId\": \"$CALENDAR_ID\"}"

echo ""
echo "=== Calendar Events API Tests Complete ==="
echo ""
echo "Calendar ID used: $CALENDAR_ID"
echo "Date range: $start_date to $end_date"