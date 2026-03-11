#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           *
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

echo "====================================================="
echo "Testing Microsoft 365 File Content Download"
echo "====================================================="
echo

# Configuration
PROBE_PATH="${PROBE_PATH:-probe}"
FULL_SERVER_URL="${SERVER_URL}/mcp"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
OUTPUT_FILE="files_content_download_${TIMESTAMP}.log"

echo "Test run: $TIMESTAMP" | tee "$OUTPUT_FILE"
echo "Server: $FULL_SERVER_URL" | tee -a "$OUTPUT_FILE"
echo "Using API Token: ${APIKEY:0:8}..." | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Step 1: Find a file to download
echo "=== Step 1: Find a File for Download ===" | tee -a "$OUTPUT_FILE"
FILE_SEARCH_RESPONSE=$("$PROBE_PATH" -url "$FULL_SERVER_URL" -transport http \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_search \
  -params '{"searchQuery": "txt", "$top": 3}' \
  2>&1)
echo "$FILE_SEARCH_RESPONSE" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

FILE_ID=$(echo "$FILE_SEARCH_RESPONSE" | grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' | head -1)

if [ -n "$FILE_ID" ]; then
    echo "=== Test 1: Download File Content ===" | tee -a "$OUTPUT_FILE"
    echo "File ID: $FILE_ID" | tee -a "$OUTPUT_FILE"
    "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport http \
      -headers "Authorization:Bearer $APIKEY" \
      -call microsoft365_files_download_content \
      -params "{\"id\": \"${FILE_ID}\"}" \
      2>&1 | head -30 | tee -a "$OUTPUT_FILE"
    echo "[Content truncated for log readability]" | tee -a "$OUTPUT_FILE"
else
    echo "No files found for download test" | tee -a "$OUTPUT_FILE"
fi
echo | tee -a "$OUTPUT_FILE"

# Test 2: Error handling - invalid file ID
echo "=== Test 2: Invalid File ID (Error Test) ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport http \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_download_content \
  -params '{"id": "invalid-file-id-12345"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "===============================================" | tee -a "$OUTPUT_FILE"
echo "File Content Download Tests Complete" | tee -a "$OUTPUT_FILE"
echo "Results saved to: $OUTPUT_FILE" | tee -a "$OUTPUT_FILE"
echo "===============================================" | tee -a "$OUTPUT_FILE"
