#!/bin/bash

# Test Microsoft 365 Mail Folders API
echo "=== Testing Microsoft 365 Mail Folders API ===" 
echo "Timestamp: $(date)"
echo "Server: http://127.0.0.1:8888/sse"
echo ""

echo "Test 1: List all mail folders (default fields)"
echo "Command: microsoft365_mail_folders_list"
echo "Parameters: {}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_mail_folders_list -params "{}"

echo ""
echo "=========================================="
echo ""

echo "Test 2: List mail folders with custom fields"
echo "Command: microsoft365_mail_folders_list"
echo "Parameters: {\"\$select\": \"displayName,id,unreadItemCount,totalItemCount\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_mail_folders_list -params '{"$select": "displayName,id,unreadItemCount,totalItemCount"}'

echo ""
echo "=========================================="
echo ""

echo "Test 3: List mail folders with pagination"
echo "Command: microsoft365_mail_folders_list"
echo "Parameters: {\"\$top\": \"10\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_mail_folders_list -params '{"$top": "10"}'

echo ""
echo "=== Mail Folders API Tests Complete ==="