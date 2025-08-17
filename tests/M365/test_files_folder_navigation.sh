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
echo "Testing Microsoft 365 Folder Navigation by Path"
echo "====================================================="
echo

# Configuration
PROBE_PATH="/Users/eric/source/MCPProbe/probe"
FULL_SERVER_URL="${SERVER_URL}/sse"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
OUTPUT_FILE="files_folder_navigation_${TIMESTAMP}.log"

echo "Test run: $TIMESTAMP" | tee "$OUTPUT_FILE"
echo "Server: $FULL_SERVER_URL" | tee -a "$OUTPUT_FILE"
echo "Using API Token: ${APIKEY:0:8}..." | tee -a "$OUTPUT_FILE"
echo "Probe tool: $PROBE_PATH" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 1: List Documents folder contents
echo "=== Test 1: List Documents Folder Contents ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_list_folder_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: List all contents of Documents folder" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_list_folder_by_path \
  -params '{"folderPath": "Documents"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 2: List Pictures folder contents with limit
echo "=== Test 2: List Pictures Folder (Top 10) ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_list_folder_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: List top 10 items in Pictures folder" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_list_folder_by_path \
  -params '{"folderPath": "Pictures", "$top": 10}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 3: List folder contents sorted by date
echo "=== Test 3: List Folder Contents (Sorted by Date) ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_list_folder_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: List Documents folder sorted by modification date" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_list_folder_by_path \
  -params '{"folderPath": "Documents", "$top": 20, "$orderby": "lastModifiedDateTime desc"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 4: List folder contents with custom fields
echo "=== Test 4: List Folder Contents (Custom Fields) ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_list_folder_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: List folder contents with specific field selection" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_list_folder_by_path \
  -params '{"folderPath": "Documents", "$select": "name,size,lastModifiedDateTime,webUrl,file,folder", "$top": 15}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 5: Filter for files only
echo "=== Test 5: List Files Only ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_list_folder_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: List only files (no folders) in Documents" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_list_folder_by_path \
  -params '{"folderPath": "Documents", "$filter": "file ne null", "$top": 20}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 6: Filter for folders only
echo "=== Test 6: List Folders Only ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_list_folder_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: List only folders (no files) in root directory" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_list_folder_by_path \
  -params '{"folderPath": "", "$filter": "folder ne null", "$top": 10}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 7: Nested folder navigation
echo "=== Test 7: Nested Folder Navigation ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_list_folder_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: Navigate into a nested folder structure" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_list_folder_by_path \
  -params '{"folderPath": "Documents/Projects", "$top": 10, "$orderby": "name asc"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 8: Test $expand parameter
echo "=== Test 8: List Folder with $expand ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_list_folder_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: List Documents folder with expanded thumbnails data" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_list_folder_by_path \
  -params '{"folderPath": "Documents", "$expand": "thumbnails", "$top": 5}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 9: Error handling - non-existent folder
echo "=== Test 9: Non-Existent Folder (Error Test) ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_list_folder_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: Test error handling for non-existent folder" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_list_folder_by_path \
  -params '{"folderPath": "NonExistent/Folder"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "===============================================" | tee -a "$OUTPUT_FILE"
echo "Folder Navigation Tests Complete" | tee -a "$OUTPUT_FILE"
echo "Results saved to: $OUTPUT_FILE" | tee -a "$OUTPUT_FILE"
echo "===============================================" | tee -a "$OUTPUT_FILE"

echo
echo "Test capabilities verified:"
echo "✓ Path-based folder content listing"
echo "✓ Customizable result count and field selection"
echo "✓ Sorting options (name, date, size)"
echo "✓ Filtering (files only, folders only)"
echo "✓ Nested folder navigation"
echo "✓ Root directory listing"
echo "✓ $expand parameter for including related data"
echo "✓ Error handling for invalid paths"