#!/bin/bash

# Test Microsoft 365 Calendar Summary API
echo "=== Testing Microsoft 365 Calendar Summary API ===" 
echo "Timestamp: $(date)"
echo "Server: http://127.0.0.1:8888/sse"
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
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_calendar_read_summary -params "{\"startDate\": \"$start_date\", \"endDate\": \"$end_date\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 2: Calendar summary (next 30 days)"
echo "Command: microsoft365_calendar_read_summary for future events"
echo "Parameters: {\"startDate\": \"$future_start\", \"endDate\": \"$future_end\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_calendar_read_summary -params "{\"startDate\": \"$future_start\", \"endDate\": \"$future_end\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 3: Calendar summary (specific date range - July 2024)"
echo "Command: microsoft365_calendar_read_summary"
echo "Parameters: {\"startDate\": \"20240701\", \"endDate\": \"20240731\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_calendar_read_summary -params '{"startDate": "20240701", "endDate": "20240731"}'

echo ""
echo "=== Calendar Summary API Tests Complete ==="