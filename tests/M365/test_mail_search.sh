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
echo "Testing Microsoft 365 Mail Search"
echo "==============================================="
echo

# Configuration
PROBE_PATH="${PROBE_PATH:-probe}"
FULL_SERVER_URL="${SERVER_URL}/mcp"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
OUTPUT_FILE="mail_search_${TIMESTAMP}.log"

echo "Test run: $TIMESTAMP" | tee "$OUTPUT_FILE"
echo "Server: $FULL_SERVER_URL" | tee -a "$OUTPUT_FILE"
echo "Using API Token: ${APIKEY:0:8}..." | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 1: Filter search by subject
echo "=== Test 1: Search for unread emails ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport http \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_mail_search \
  -params '{"$filter":"isRead eq false","$select":"subject,from,receivedDateTime","$top":"5"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 2: Full-text search
echo "=== Test 2: Full-text search ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport http \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_mail_search \
  -params '{"$search":"meeting","$top":"5"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "==============================================="
echo "Mail Search tests completed."
echo "Results saved to: $OUTPUT_FILE"
echo "==============================================="
