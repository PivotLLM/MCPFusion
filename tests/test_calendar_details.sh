#!/bin/bash

# Test Microsoft 365 Calendar Details API
echo "=== Testing Microsoft 365 Calendar Details API ===" 
echo "Timestamp: $(date)"
echo "Server: http://127.0.0.1:8888/sse"
echo ""

# Calculate date ranges
start_date=$(date -v-30d +%Y%m%d)  # 30 days ago
end_date=$(date +%Y%m%d)           # Today
future_start=$(date +%Y%m%d)       # Today
future_end=$(date -v+30d +%Y%m%d)  # 30 days from now

echo "Test 1: Calendar details (last 30 days)"
echo "Command: microsoft365_calendar_read_details"
echo "Parameters: {\"startDate\": \"$start_date\", \"endDate\": \"$end_date\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_calendar_read_details -params "{\"startDate\": \"$start_date\", \"endDate\": \"$end_date\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 2: Calendar details with custom fields"
echo "Command: microsoft365_calendar_read_details with field selection"
echo "Parameters: {\"startDate\": \"$start_date\", \"endDate\": \"$end_date\", \"\\$select\": \"subject,start,end,organizer,location\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_calendar_read_details -params "{\"startDate\": \"$start_date\", \"endDate\": \"$end_date\", \"\$select\": \"subject,start,end,organizer,location\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 3: Calendar details (next 30 days)"
echo "Command: microsoft365_calendar_read_details for future events"
echo "Parameters: {\"startDate\": \"$future_start\", \"endDate\": \"$future_end\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_calendar_read_details -params "{\"startDate\": \"$future_start\", \"endDate\": \"$future_end\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 4: Calendar details with minimal fields"
echo "Command: microsoft365_calendar_read_details with minimal field selection"
echo "Parameters: {\"startDate\": \"$start_date\", \"endDate\": \"$end_date\", \"\\$select\": \"subject,start,end\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_calendar_read_details -params "{\"startDate\": \"$start_date\", \"endDate\": \"$end_date\", \"\$select\": \"subject,start,end\"}"

echo ""
echo "=== Calendar Details API Tests Complete ==="