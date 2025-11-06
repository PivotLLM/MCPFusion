#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025 Tenebris Technologies Inc.                                *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

# Get the directory of the script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load environment variables
if [ -f "$SCRIPT_DIR/.env" ]; then
    source "$SCRIPT_DIR/.env"
else
    echo "Error: .env file not found in $SCRIPT_DIR"
    echo "Please create a .env file with APIKEY=your-api-token and SERVER_URL=your-server-url"
    exit 1
fi

# Check if APIKEY is set
if [ -z "$APIKEY" ]; then
    echo "Error: APIKEY not set in .env file"
    exit 1
fi

# Check if SERVER_URL is set
if [ -z "$SERVER_URL" ]; then
    echo "Error: SERVER_URL not set in .env file"
    exit 1
fi

# Append /sse to the base URL
FULL_SERVER_URL="${SERVER_URL}/sse"

# Test Microsoft 365 Profile API
echo "=== Testing Microsoft 365 Profile API ===" 
echo "Timestamp: $(date)"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

echo "Test 1: Basic profile retrieval"
echo "Command: microsoft365_profile_get with default parameters"
echo "Parameters: {}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_profile_get -params '{}'

echo ""
echo "=========================================="
echo ""

echo "Test 2: Profile with custom fields"
echo "Command: microsoft365_profile_get with custom field selection"
echo "Parameters: {\"\\$select\": \"displayName,mail,userPrincipalName,jobTitle,department\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_profile_get -params '{"$select": "displayName,mail,userPrincipalName,jobTitle,department"}'

echo ""
echo "=== Profile API Tests Complete ==="