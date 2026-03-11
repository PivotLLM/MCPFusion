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

echo "==============================================="
echo "Testing Microsoft 365 Enhanced File Operations"
echo "==============================================="
echo

# Configuration
PROBE_PATH="${PROBE_PATH:-probe}"
FULL_SERVER_URL="${SERVER_URL}/mcp"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
OUTPUT_FILE="files_enhanced_${TIMESTAMP}.log"

echo "Test run: $TIMESTAMP" | tee "$OUTPUT_FILE"
echo "Server: $FULL_SERVER_URL" | tee -a "$OUTPUT_FILE"
echo "Using API Token: ${APIKEY:0:8}..." | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 1: List recent files
echo "=== Test 1: List Recent Files ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport http \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_recent \
  -params '{"$top": 5}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 2: List root directory and navigate into first folder
echo "=== Test 2: List Root Directory ===" | tee -a "$OUTPUT_FILE"
ROOT_RESPONSE=$("$PROBE_PATH" -url "$FULL_SERVER_URL" -transport http \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_list \
  -params '{"$filter":"folder ne null","$top":"5"}' \
  2>&1)
echo "$ROOT_RESPONSE" | tee -a "$OUTPUT_FILE"

FOLDER_ID=$(echo "$ROOT_RESPONSE" | grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')

if [ -n "$FOLDER_ID" ]; then
    echo | tee -a "$OUTPUT_FILE"
    echo "=== Test 3: List Folder Contents by ID ===" | tee -a "$OUTPUT_FILE"
    echo "Folder ID: $FOLDER_ID" | tee -a "$OUTPUT_FILE"
    "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport http \
      -headers "Authorization:Bearer $APIKEY" \
      -call microsoft365_files_list_children \
      -params "{\"id\":\"$FOLDER_ID\",\"\$top\":\"5\"}" \
      2>&1 | tee -a "$OUTPUT_FILE"
fi
echo | tee -a "$OUTPUT_FILE"

echo "===============================================" | tee -a "$OUTPUT_FILE"
echo "Enhanced File Operations Tests Complete" | tee -a "$OUTPUT_FILE"
echo "Results saved to: $OUTPUT_FILE" | tee -a "$OUTPUT_FILE"
echo "===============================================" | tee -a "$OUTPUT_FILE"
