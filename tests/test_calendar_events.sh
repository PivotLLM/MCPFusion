#!/bin/bash

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
calendar_response=$(/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendars_list -params '{"$top": "1"}' 2>/dev/null)

# Extract calendar ID (this is a simple extraction - in real usage you'd parse JSON properly)
# For demo purposes, we'll use a placeholder
CALENDAR_ID="YOUR_CALENDAR_ID"

echo "Note: Replace YOUR_CALENDAR_ID with actual calendar ID from calendars_list call"
echo ""

# Calculate date ranges
start_date=$(date -v-7d +%Y%m%d)   # 7 days ago
end_date=$(date -v+7d +%Y%m%d)     # 7 days from now

echo "Test 1: Calendar events summary for specific calendar"
echo "Command: microsoft365_calendar_events_read_summary"
echo "Parameters: {\"calendarId\": \"$CALENDAR_ID\", \"startDate\": \"$start_date\", \"endDate\": \"$end_date\"}"
echo ""
echo "NOTE: This test requires a valid calendar ID. Run calendars_list first to get one."
# Commented out to avoid errors - uncomment when you have a real calendar ID
# /Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_calendar_events_read_summary -params "{\"calendarId\": \"$CALENDAR_ID\", \"startDate\": \"$start_date\", \"endDate\": \"$end_date\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 2: Calendar events details for specific calendar"
echo "Command: microsoft365_calendar_events_read_details"
echo "Parameters: {\"calendarId\": \"$CALENDAR_ID\", \"startDate\": \"$start_date\", \"endDate\": \"$end_date\"}"
echo ""
echo "NOTE: This test requires a valid calendar ID. Run calendars_list first to get one."
# Commented out to avoid errors - uncomment when you have a real calendar ID
# /Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_calendar_events_read_details -params "{\"calendarId\": \"$CALENDAR_ID\", \"startDate\": \"$start_date\", \"endDate\": \"$end_date\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 3: Calendar events without date filtering"
echo "Command: microsoft365_calendar_events_read_summary"
echo "Parameters: {\"calendarId\": \"$CALENDAR_ID\"}"
echo ""
echo "NOTE: This test requires a valid calendar ID. Run calendars_list first to get one."
# Commented out to avoid errors - uncomment when you have a real calendar ID
# /Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_calendar_events_read_summary -params "{\"calendarId\": \"$CALENDAR_ID\"}"

echo ""
echo "=== Calendar Events API Tests Complete ==="
echo ""
echo "To run these tests:"
echo "1. First run: ./test_calendars_list.sh"
echo "2. Copy a calendar ID from the output"
echo "3. Edit this script and replace YOUR_CALENDAR_ID with the real ID"
echo "4. Uncomment the probe commands above"
echo "5. Run this script again"