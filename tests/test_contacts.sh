#!/bin/bash

# Test Microsoft 365 Contacts API
echo "=== Testing Microsoft 365 Contacts API ===" 
echo "Timestamp: $(date)"
echo "Server: http://127.0.0.1:8888/sse"
echo ""

echo "Test 1: Default contacts list"
echo "Command: microsoft365_contacts_list with default parameters"
echo "Parameters: {}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_contacts_list -params '{}'

echo ""
echo "=========================================="
echo ""

echo "Test 2: Limited contact count (10 contacts)"
echo "Command: microsoft365_contacts_list with top 10"
echo "Parameters: {\"\\$top\": \"10\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_contacts_list -params '{"$top": "10"}'

echo ""
echo "=========================================="
echo ""

echo "Test 3: Custom fields selection"
echo "Command: microsoft365_contacts_list with custom fields"
echo "Parameters: {\"\\$select\": \"displayName,emailAddresses\", \"\\$top\": \"5\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_contacts_list -params '{"$select": "displayName,emailAddresses", "$top": "5"}'

echo ""
echo "=== Contacts API Tests Complete ==="