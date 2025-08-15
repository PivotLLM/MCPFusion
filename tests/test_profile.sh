#!/bin/bash

# Test Microsoft 365 Profile API
echo "=== Testing Microsoft 365 Profile API ===" 
echo "Timestamp: $(date)"
echo "Server: http://127.0.0.1:8888/sse"
echo ""

echo "Test 1: Basic profile retrieval"
echo "Command: microsoft365_profile_get with default parameters"
echo "Parameters: {}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_profile_get -params '{}'

echo ""
echo "=========================================="
echo ""

echo "Test 2: Profile with custom fields"
echo "Command: microsoft365_profile_get with custom field selection"
echo "Parameters: {\"\\$select\": \"displayName,mail,userPrincipalName,jobTitle,department\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_profile_get -params '{"$select": "displayName,mail,userPrincipalName,jobTitle,department"}'

echo ""
echo "=== Profile API Tests Complete ==="