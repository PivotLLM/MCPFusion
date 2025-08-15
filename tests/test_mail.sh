#!/bin/bash

# Test Microsoft 365 Mail API
echo "=== Testing Microsoft 365 Mail API ===" 
echo "Timestamp: $(date)"
echo "Server: http://127.0.0.1:8888/sse"
echo ""

echo "Test 1: Default inbox messages"
echo "Command: microsoft365_mail_read_inbox with default parameters"
echo "Parameters: {}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_mail_read_inbox -params '{}'

echo ""
echo "=========================================="
echo ""

echo "Test 2: Limited message count (5 messages)"
echo "Command: microsoft365_mail_read_inbox with top 5"
echo "Parameters: {\"\\$top\": \"5\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_mail_read_inbox -params '{"$top": "5"}'

echo ""
echo "=========================================="
echo ""

echo "Test 3: Unread messages only"
echo "Command: microsoft365_mail_read_inbox filtered for unread"
echo "Parameters: {\"\\$filter\": \"isRead eq false\", \"\\$top\": \"10\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_mail_read_inbox -params '{"$filter": "isRead eq false", "$top": "10"}'

echo ""
echo "=========================================="
echo ""

echo "Test 4: Custom fields selection"
echo "Command: microsoft365_mail_read_inbox with custom fields"
echo "Parameters: {\"\\$select\": \"subject,from,receivedDateTime,isRead\", \"\\$top\": \"5\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_mail_read_inbox -params '{"$select": "subject,from,receivedDateTime,isRead", "$top": "5"}'

echo ""
echo "=== Mail API Tests Complete ==="