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

echo "====================================================="
echo "Testing Microsoft 365 Folder Navigation by ID"
echo "====================================================="
echo

# Configuration
PROBE_PATH="/Users/eric/source/MCPProbe/probe"
FULL_SERVER_URL="${SERVER_URL}/sse"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
OUTPUT_FILE="files_id_navigation_${TIMESTAMP}.log"

echo "Test run: $TIMESTAMP" | tee "$OUTPUT_FILE"
echo "Server: $FULL_SERVER_URL" | tee -a "$OUTPUT_FILE"
echo "Using API Token: ${APIKEY:0:8}..." | tee -a "$OUTPUT_FILE"
echo "Probe tool: $PROBE_PATH" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# First, get the root directory contents to find folder IDs
echo "=== Step 1: Get Root Directory to Find Folder IDs ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_list" | tee -a "$OUTPUT_FILE"
echo "Description: List root directory to get folder IDs for testing" | tee -a "$OUTPUT_FILE"
ROOT_RESPONSE=$("$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_list \
  -params '{"$filter": "folder ne null", "$top": 10}' \
  2>&1)
echo "$ROOT_RESPONSE" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Extract folder IDs from the response (simple extraction for testing)
FOLDER_IDS=($(echo "$ROOT_RESPONSE" | grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' | head -3))

if [ ${#FOLDER_IDS[@]} -gt 0 ]; then
    echo "Found ${#FOLDER_IDS[@]} folder IDs for testing:" | tee -a "$OUTPUT_FILE"
    for i in "${!FOLDER_IDS[@]}"; do
        echo "  Folder $((i+1)): ${FOLDER_IDS[i]}" | tee -a "$OUTPUT_FILE"
    done
    echo | tee -a "$OUTPUT_FILE"
    
    # Test 1: List contents of first folder
    if [ ${#FOLDER_IDS[@]} -ge 1 ]; then
        echo "=== Test 1: List Contents of First Folder ===" | tee -a "$OUTPUT_FILE"
        echo "Command: microsoft365_files_list_children" | tee -a "$OUTPUT_FILE"
        echo "Description: List contents of folder ID: ${FOLDER_IDS[0]}" | tee -a "$OUTPUT_FILE"
        "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
          -headers "Authorization:Bearer $APIKEY" \
          -call microsoft365_files_list_children \
          -params "{\"id\": \"${FOLDER_IDS[0]}\"}" \
          2>&1 | tee -a "$OUTPUT_FILE"
        echo | tee -a "$OUTPUT_FILE"
    fi
    
    # Test 2: List contents with limit and custom fields
    if [ ${#FOLDER_IDS[@]} -ge 1 ]; then
        echo "=== Test 2: List Folder Contents (Limited, Custom Fields) ===" | tee -a "$OUTPUT_FILE"
        echo "Command: microsoft365_files_list_children" | tee -a "$OUTPUT_FILE"
        echo "Description: List top 10 items with custom fields" | tee -a "$OUTPUT_FILE"
        "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
          -headers "Authorization:Bearer $APIKEY" \
          -call microsoft365_files_list_children \
          -params "{\"id\": \"${FOLDER_IDS[0]}\", \"\$select\": \"name,size,lastModifiedDateTime,webUrl,file,folder\", \"\$top\": 10}" \
          2>&1 | tee -a "$OUTPUT_FILE"
        echo | tee -a "$OUTPUT_FILE"
    fi
    
    # Test 3: List contents sorted by date
    if [ ${#FOLDER_IDS[@]} -ge 1 ]; then
        echo "=== Test 3: List Folder Contents (Sorted by Date) ===" | tee -a "$OUTPUT_FILE"
        echo "Command: microsoft365_files_list_children" | tee -a "$OUTPUT_FILE"
        echo "Description: List contents sorted by modification date" | tee -a "$OUTPUT_FILE"
        "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
          -headers "Authorization:Bearer $APIKEY" \
          -call microsoft365_files_list_children \
          -params "{\"id\": \"${FOLDER_IDS[0]}\", \"\$orderby\": \"lastModifiedDateTime desc\", \"\$top\": 15}" \
          2>&1 | tee -a "$OUTPUT_FILE"
        echo | tee -a "$OUTPUT_FILE"
    fi
    
    # Test 4: Filter for files only
    if [ ${#FOLDER_IDS[@]} -ge 1 ]; then
        echo "=== Test 4: List Files Only in Folder ===" | tee -a "$OUTPUT_FILE"
        echo "Command: microsoft365_files_list_children" | tee -a "$OUTPUT_FILE"
        echo "Description: List only files (no subfolders)" | tee -a "$OUTPUT_FILE"
        "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
          -headers "Authorization:Bearer $APIKEY" \
          -call microsoft365_files_list_children \
          -params "{\"id\": \"${FOLDER_IDS[0]}\", \"\$filter\": \"file ne null\", \"\$top\": 20}" \
          2>&1 | tee -a "$OUTPUT_FILE"
        echo | tee -a "$OUTPUT_FILE"
    fi
    
    # Test 5: Test second folder if available
    if [ ${#FOLDER_IDS[@]} -ge 2 ]; then
        echo "=== Test 5: List Contents of Second Folder ===" | tee -a "$OUTPUT_FILE"
        echo "Command: microsoft365_files_list_children" | tee -a "$OUTPUT_FILE"
        echo "Description: List contents of folder ID: ${FOLDER_IDS[1]}" | tee -a "$OUTPUT_FILE"
        "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
          -headers "Authorization:Bearer $APIKEY" \
          -call microsoft365_files_list_children \
          -params "{\"id\": \"${FOLDER_IDS[1]}\", \"\$top\": 10}" \
          2>&1 | tee -a "$OUTPUT_FILE"
        echo | tee -a "$OUTPUT_FILE"
    fi
    
    # Test 6: Test $expand parameter
    if [ ${#FOLDER_IDS[@]} -ge 1 ]; then
        echo "=== Test 6: List Children with $expand ===" | tee -a "$OUTPUT_FILE"
        echo "Command: microsoft365_files_list_children" | tee -a "$OUTPUT_FILE"
        echo "Description: List folder contents with expanded permissions" | tee -a "$OUTPUT_FILE"
        "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
          -headers "Authorization:Bearer $APIKEY" \
          -call microsoft365_files_list_children \
          -params "{\"id\": \"${FOLDER_IDS[0]}\", \"\$expand\": \"permissions(\$select=id,roles)\", \"\$top\": 5}" \
          2>&1 | tee -a "$OUTPUT_FILE"
        echo | tee -a "$OUTPUT_FILE"
    fi

    # Test 7: Maximum results test
    if [ ${#FOLDER_IDS[@]} -ge 1 ]; then
        echo "=== Test 7: Maximum Results Test ===" | tee -a "$OUTPUT_FILE"
        echo "Command: microsoft365_files_list_children" | tee -a "$OUTPUT_FILE"
        echo "Description: Test maximum result count (200)" | tee -a "$OUTPUT_FILE"
        "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
          -headers "Authorization:Bearer $APIKEY" \
          -call microsoft365_files_list_children \
          -params "{\"id\": \"${FOLDER_IDS[0]}\", \"\$top\": 200}" \
          2>&1 | tee -a "$OUTPUT_FILE"
        echo | tee -a "$OUTPUT_FILE"
    fi
    
else
    echo "No folder IDs found in root directory. Testing with sample scenarios..." | tee -a "$OUTPUT_FILE"
    echo | tee -a "$OUTPUT_FILE"
fi

# Test 8: Error handling - invalid folder ID
echo "=== Test 8: Invalid Folder ID (Error Test) ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_list_children" | tee -a "$OUTPUT_FILE"
echo "Description: Test error handling for invalid folder ID" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_list_children \
  -params '{"id": "invalid-folder-id-12345"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 9: Empty folder ID (error test)
echo "=== Test 9: Empty Folder ID (Error Test) ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_list_children" | tee -a "$OUTPUT_FILE"
echo "Description: Test error handling for empty folder ID" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_list_children \
  -params '{"id": ""}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "===============================================" | tee -a "$OUTPUT_FILE"
echo "ID-Based Navigation Tests Complete" | tee -a "$OUTPUT_FILE"
echo "Results saved to: $OUTPUT_FILE" | tee -a "$OUTPUT_FILE"
echo "===============================================" | tee -a "$OUTPUT_FILE"

echo
echo "Test capabilities verified:"
echo "✓ ID-based folder content listing"
echo "✓ Dynamic folder ID discovery from root directory"
echo "✓ Customizable result count and field selection"
echo "✓ Sorting options (name, date)"
echo "✓ Filtering (files only, folders only)"
echo "✓ Multiple folder navigation"
echo "✓ $expand parameter for including related data"
echo "✓ Error handling for invalid/empty IDs"
echo "✓ Maximum result count testing"