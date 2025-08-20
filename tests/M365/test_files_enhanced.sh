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

echo "==============================================="
echo "Testing Microsoft 365 Enhanced File Operations"
echo "==============================================="
echo

# Configuration
PROBE_PATH="/Users/eric/source/MCPProbe/probe"
FULL_SERVER_URL="${SERVER_URL}/sse"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
OUTPUT_FILE="files_enhanced_${TIMESTAMP}.log"

echo "Test run: $TIMESTAMP" | tee "$OUTPUT_FILE"
echo "Server: $FULL_SERVER_URL" | tee -a "$OUTPUT_FILE"
echo "Using API Token: ${APIKEY:0:8}..." | tee -a "$OUTPUT_FILE"
echo "Probe tool: $PROBE_PATH" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 1: List recent files
echo "=== Test 1: List Recent Files ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_recent" | tee -a "$OUTPUT_FILE"
echo "Description: Get recently accessed files across all drives" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_recent \
  -params '{"$top":"10"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 2: Get root folder contents to find a folder
echo "=== Test 2: List Root Directory Contents ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_list (root directory)" | tee -a "$OUTPUT_FILE"
echo "Description: List files and folders in root to find folder IDs" | tee -a "$OUTPUT_FILE"
ROOT_RESPONSE=$("$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_list \
  -params '{"$filter":"folder ne null","$top":"5"}' \
  2>&1)
echo "$ROOT_RESPONSE" | tee -a "$OUTPUT_FILE"

# Extract a folder ID for testing (simple extraction)
FOLDER_ID=$(echo "$ROOT_RESPONSE" | grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')

if [ -n "$FOLDER_ID" ]; then
    echo "Found folder ID: $FOLDER_ID" | tee -a "$OUTPUT_FILE"
    
    # Test 3: List folder contents by ID
    echo | tee -a "$OUTPUT_FILE"
    echo "=== Test 3: List Folder Contents by ID ===" | tee -a "$OUTPUT_FILE"
    echo "Command: microsoft365_files_list_children" | tee -a "$OUTPUT_FILE"
    echo "Description: List contents of folder with ID: $FOLDER_ID" | tee -a "$OUTPUT_FILE"
    "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
      -headers "Authorization:Bearer $APIKEY" \
      -call microsoft365_files_list_children \
      -params "{\"id\":\"$FOLDER_ID\",\"\$top\":\"10\"}" \
      2>&1 | tee -a "$OUTPUT_FILE"
else
    echo "No folder found in root directory for testing" | tee -a "$OUTPUT_FILE"
fi

echo | tee -a "$OUTPUT_FILE"

# Test 4: Access by path - try common folder paths
echo "=== Test 4: Get File/Folder by Path ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_get_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: Try to access Documents folder by path" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_get_by_path \
  -params '{"filePath":"Documents"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 5: List folder contents by path
echo "=== Test 5: List Folder Contents by Path ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_list_folder_by_path" | tee -a "$OUTPUT_FILE"
echo "Description: List contents of Documents folder by path" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_list_folder_by_path \
  -params '{"folderPath":"Documents","$top":"10","$orderby":"lastModifiedDateTime desc"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 6: Find a file to test content download
echo "=== Test 6: Find a File for Content Download ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_search (find text files)" | tee -a "$OUTPUT_FILE"
echo "Description: Search for text files to test content download" | tee -a "$OUTPUT_FILE"
FILE_SEARCH_RESPONSE=$("$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_search \
  -params '{"searchQuery":"txt","$top":"3"}' \
  2>&1)
echo "$FILE_SEARCH_RESPONSE" | tee -a "$OUTPUT_FILE"

# Extract a file ID for content download testing
FILE_ID=$(echo "$FILE_SEARCH_RESPONSE" | grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')

if [ -n "$FILE_ID" ]; then
    echo | tee -a "$OUTPUT_FILE"
    echo "Found file ID for content test: $FILE_ID" | tee -a "$OUTPUT_FILE"
    
    # Test 7: Download file content
    echo | tee -a "$OUTPUT_FILE"
    echo "=== Test 7: Download File Content ===" | tee -a "$OUTPUT_FILE"
    echo "Command: microsoft365_files_download_content" | tee -a "$OUTPUT_FILE"
    echo "Description: Download actual content of file with ID: $FILE_ID" | tee -a "$OUTPUT_FILE"
    echo "Note: This will show binary/text content or redirect URL" | tee -a "$OUTPUT_FILE"
    "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
      -headers "Authorization:Bearer $APIKEY" \
      -call microsoft365_files_download_content \
      -params "{\"id\":\"$FILE_ID\"}" \
      2>&1 | head -20 | tee -a "$OUTPUT_FILE"
    echo "[Content truncated for readability]" | tee -a "$OUTPUT_FILE"
else
    echo "No files found for content download testing" | tee -a "$OUTPUT_FILE"
fi

echo | tee -a "$OUTPUT_FILE"
echo "===============================================" | tee -a "$OUTPUT_FILE"
echo "Enhanced File Operations Tests Complete" | tee -a "$OUTPUT_FILE"
echo "Results saved to: $OUTPUT_FILE" | tee -a "$OUTPUT_FILE"
echo "===============================================" | tee -a "$OUTPUT_FILE"

echo "Summary of new file capabilities tested:" | tee -a "$OUTPUT_FILE"
echo "✓ Recent files access across all drives" | tee -a "$OUTPUT_FILE"
echo "✓ Folder navigation by ID" | tee -a "$OUTPUT_FILE"
echo "✓ File/folder access by path" | tee -a "$OUTPUT_FILE"
echo "✓ Folder listing by path" | tee -a "$OUTPUT_FILE"
echo "✓ File content download capability" | tee -a "$OUTPUT_FILE"