#!/bin/bash

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

# Test script for Files Search functionality
echo "==============================================="
echo "Testing Microsoft 365 Files Search (Fixed)"
echo "==============================================="
echo

# Configuration
PROBE_PATH="/Users/eric/source/MCPProbe/probe"
SERVER_URL="http://127.0.0.1:8888"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
OUTPUT_FILE="files_search_fixed_${TIMESTAMP}.log"

echo "Test run: $TIMESTAMP" | tee "$OUTPUT_FILE"
echo "Server: $SERVER_URL" | tee -a "$OUTPUT_FILE"
echo "Using API Token: ${APIKEY:0:8}..." | tee -a "$OUTPUT_FILE"
echo "Probe tool: $PROBE_PATH" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 1: Search for files with 'document' in name
echo "=== Test 1: Search for files containing 'document' ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$SERVER_URL/sse" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_search \
  -params '{"searchQuery":"document","$top":"10"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 2: Search for files with different query
echo "=== Test 2: Search for files containing 'report' ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$SERVER_URL/sse" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_search \
  -params '{"searchQuery":"report","$top":"5"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 3: Search for files with field selection
echo "=== Test 3: Search with custom field selection ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$SERVER_URL/sse" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_search \
  -params '{"searchQuery":"presentation","$select":"name,size,lastModifiedDateTime","$top":"5"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 4: Search for common file types
echo "=== Test 4: Search for files (top 5) ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$SERVER_URL/sse" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_search \
  -params '{"searchQuery":"doc","$top":"5"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "===============================================" | tee -a "$OUTPUT_FILE"
echo "Files Search Tests Complete" | tee -a "$OUTPUT_FILE"
echo "Results saved to: $OUTPUT_FILE" | tee -a "$OUTPUT_FILE"
echo "===============================================" | tee -a "$OUTPUT_FILE"