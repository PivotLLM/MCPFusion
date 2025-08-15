#!/bin/bash

# Test Microsoft 365 Calendar Summary API
echo "=== Testing Microsoft 365 Calendar Summary API ===" 
echo "Timestamp: $(date)"
echo "Server: http://127.0.0.1:8888/sse"
echo ""

# Calculate date ranges
start_date=$(date -v-30d +%Y%m%d)  # 30 days ago
end_date=$(date -v+30d +%Y%m%d)  # 30 days from now

echo "Calendar summary (last 30 days)"
echo "Command: microsoft365_calendar_read_summary"
echo "Parameters: {\"startDate\": \"$start_date\", \"endDate\": \"$end_date\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_calendar_read_summary -params "{\"startDate\": \"$start_date\", \"endDate\": \"$end_date\"}"

