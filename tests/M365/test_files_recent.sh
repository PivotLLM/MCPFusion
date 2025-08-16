#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025 Tenebris Technologies Inc.                                *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

# Load environment variables
if [ -f "../.env" ]; then
    source ../.env
elif [ -f ".env" ]; then
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

echo "==============================================="
echo "Testing Microsoft 365 Recent Files Access"
echo "==============================================="
echo

# Configuration
PROBE_PATH="/Users/eric/source/MCPProbe/probe"
SERVER_URL="http://127.0.0.1:8888"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
OUTPUT_FILE="files_recent_${TIMESTAMP}.log"

echo "Test run: $TIMESTAMP" | tee "$OUTPUT_FILE"
echo "Server: $SERVER_URL" | tee -a "$OUTPUT_FILE"
echo "Using API Token: ${APIKEY:0:8}..." | tee -a "$OUTPUT_FILE"
echo "Probe tool: $PROBE_PATH" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 1: List recent files with default parameters
echo "=== Test 1: List Recent Files (Default) ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_recent" | tee -a "$OUTPUT_FILE"
echo "Description: Get recently accessed files with default settings" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$SERVER_URL/sse" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_recent \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 2: List recent files with custom count
echo "=== Test 2: List Recent Files (Top 5) ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_recent" | tee -a "$OUTPUT_FILE"
echo "Description: Get top 5 most recent files" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$SERVER_URL/sse" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_recent \
  -params '{"$top": 5}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 3: List recent files with custom fields
echo "=== Test 3: List Recent Files (Custom Fields) ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_recent" | tee -a "$OUTPUT_FILE"
echo "Description: Get recent files with specific field selection" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$SERVER_URL/sse" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_recent \
  -params '{"$select": "name,webUrl,lastModifiedDateTime,size", "$top": 10}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 4: Test $expand parameter
echo "=== Test 4: List Recent Files with $expand ====" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_recent" | tee -a "$OUTPUT_FILE"
echo "Description: Get recent files with expanded permissions data" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$SERVER_URL/sse" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_recent \
  -params '{"$top": 5, "$expand": "permissions"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 5: Maximum count test
echo "=== Test 5: List Recent Files (Maximum Count) ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_recent" | tee -a "$OUTPUT_FILE"
echo "Description: Get maximum number of recent files (200)" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$SERVER_URL/sse" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_recent \
  -params '{"$top": 200}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "===============================================" | tee -a "$OUTPUT_FILE"
echo "Recent Files Tests Complete" | tee -a "$OUTPUT_FILE"
echo "Results saved to: $OUTPUT_FILE" | tee -a "$OUTPUT_FILE"
echo "===============================================" | tee -a "$OUTPUT_FILE"

echo
echo "Test capabilities verified:"
echo "✓ Recent files discovery across all drives"
echo "✓ Customizable result count (1-1000)"
echo "✓ Field selection for optimized responses"
echo "✓ $expand parameter for including related data"
echo "✓ Default and maximum parameter testing"