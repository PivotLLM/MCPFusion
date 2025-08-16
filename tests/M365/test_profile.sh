#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025 Tenebris Technologies Inc.                                *
# All rights reserved.                                                         *
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

# Test Microsoft 365 Profile API
echo "=== Testing Microsoft 365 Profile API ===" 
echo "Timestamp: $(date)"
echo "Server: http://127.0.0.1:8888/sse"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

echo "Test 1: Basic profile retrieval"
echo "Command: microsoft365_profile_get with default parameters"
echo "Parameters: {}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_profile_get -params '{}'

echo ""
echo "=========================================="
echo ""

echo "Test 2: Profile with custom fields"
echo "Command: microsoft365_profile_get with custom field selection"
echo "Parameters: {\"\\$select\": \"displayName,mail,userPrincipalName,jobTitle,department\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_profile_get -params '{"$select": "displayName,mail,userPrincipalName,jobTitle,department"}'

echo ""
echo "=== Profile API Tests Complete ==="