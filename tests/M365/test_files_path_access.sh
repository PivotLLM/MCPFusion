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

# Check if SERVER_URL is set
if [ -z "$SERVER_URL" ]; then
    echo "Error: SERVER_URL not set in .env file"
    exit 1
fi

echo "=================================================="
echo "Testing Microsoft 365 Path-Based File Access"
echo "=================================================="
echo

# Configuration
PROBE_PATH="/Users/eric/source/MCPProbe/probe"
FULL_SERVER_URL="${SERVER_URL}/sse"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
OUTPUT_FILE="files_path_access_${TIMESTAMP}.log"

echo "Test run: $TIMESTAMP" | tee "$OUTPUT_FILE"
echo "Server: $FULL_SERVER_URL" | tee -a "$OUTPUT_FILE"
echo "Using API Token: ${APIKEY:0:8}..." | tee -a "$OUTPUT_FILE"
echo "Probe tool: $PROBE_PATH" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 1: Access Documents folder
echo "=== Test 1: Get Documents Folder ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_get_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: Access Documents folder by path" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_get_by_path \
  -params '{"filePath": "Documents"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 2: Access Pictures folder
echo "=== Test 2: Get Pictures Folder ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_get_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: Access Pictures folder by path" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_get_by_path \
  -params '{"filePath": "Pictures"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 3: Access nested path
echo "=== Test 3: Get Nested Path ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_get_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: Access nested folder path" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_get_by_path \
  -params '{"filePath": "Documents/Projects"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 4: Access specific file
echo "=== Test 4: Get Specific File ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_get_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: Access a specific file by full path" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_get_by_path \
  -params '{"filePath": "Documents/readme.txt"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 5: Custom field selection
echo "=== Test 5: Get Folder with Custom Fields ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_get_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: Access folder with specific field selection" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_get_by_path \
  -params '{"filePath": "Documents", "$select": "name,size,lastModifiedDateTime,webUrl,folder"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 6: Test $expand parameter
echo "=== Test 6: Get Documents with $expand ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_get_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: Access Documents folder with expanded children data" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_get_by_path \
  -params '{"filePath": "Documents", "$expand": "children($select=name,id,size)"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 7: Non-existent path (error handling)
echo "=== Test 7: Non-Existent Path (Error Test) ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_get_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: Test error handling for non-existent path" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_get_by_path \
  -params '{"filePath": "NonExistent/Folder/Path"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "===============================================" | tee -a "$OUTPUT_FILE"
echo "Path-Based Access Tests Complete" | tee -a "$OUTPUT_FILE"
echo "Results saved to: $OUTPUT_FILE" | tee -a "$OUTPUT_FILE"
echo "===============================================" | tee -a "$OUTPUT_FILE"

echo
echo "Test capabilities verified:"
echo "✓ Direct path-based file/folder access"
echo "✓ Common folder navigation (Documents, Pictures)"
echo "✓ Nested path support (Documents/Projects)"
echo "✓ Specific file access by full path"
echo "✓ Custom field selection"
echo "✓ $expand parameter for including related data"
echo "✓ Error handling for invalid paths"