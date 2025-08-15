#!/bin/bash

# Test Microsoft 365 Calendars List API
echo "=== Testing Microsoft 365 Calendars List API ===" 
echo "Timestamp: $(date)"
echo "Server: http://127.0.0.1:8888/sse"
echo ""

echo "Test 1: List all calendars (default fields)"
echo "Command: microsoft365_calendars_list"
echo "Parameters: {}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_calendars_list -params "{}"

echo ""
echo "=========================================="
echo ""

echo "Test 2: List calendars with custom fields"
echo "Command: microsoft365_calendars_list"
echo "Parameters: {\"\$select\": \"name,id,color,canEdit,canShare,isDefaultCalendar\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_calendars_list -params '{"$select": "name,id,color,canEdit,canShare,isDefaultCalendar"}'

echo ""
echo "=========================================="
echo ""

echo "Test 3: List calendars with pagination"
echo "Command: microsoft365_calendars_list"
echo "Parameters: {\"\$top\": \"5\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_calendars_list -params '{"$top": "5"}'

echo ""
echo "=== Calendars List API Tests Complete ==="