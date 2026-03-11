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

PROBE_PATH="${PROBE_PATH:-probe}"
FULL_SERVER_URL="${SERVER_URL}/mcp"

echo "=== Testing Microsoft 365 Individual Item Retrieval ==="
echo "Timestamp: $(date)"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

# Test 1: Get a message ID from inbox and read it
echo "=== Test 1: Read specific mail message ==="
INBOX_RESPONSE=$("$PROBE_PATH" -url "$FULL_SERVER_URL" -transport http \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_mail_read_inbox \
  -params '{"$top": 1, "$select": "id,subject"}' 2>&1)
MESSAGE_ID=$(echo "$INBOX_RESPONSE" | grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')

if [ -n "$MESSAGE_ID" ]; then
    echo "Found message ID: $MESSAGE_ID"
    "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport http \
      -headers "Authorization:Bearer $APIKEY" \
      -call microsoft365_mail_read_message \
      -params "{\"id\": \"$MESSAGE_ID\", \"\$select\": \"subject,from,receivedDateTime,bodyPreview\"}"
else
    echo "No message ID found from inbox"
fi

echo ""
echo "=========================================="
echo ""

# Test 2: Get a contact ID and read it
echo "=== Test 2: Read specific contact ==="
CONTACTS_RESPONSE=$("$PROBE_PATH" -url "$FULL_SERVER_URL" -transport http \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_contacts_list \
  -params '{"$top": 1, "$select": "id,displayName"}' 2>&1)
CONTACT_ID=$(echo "$CONTACTS_RESPONSE" | grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')

if [ -n "$CONTACT_ID" ]; then
    echo "Found contact ID: $CONTACT_ID"
    "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport http \
      -headers "Authorization:Bearer $APIKEY" \
      -call microsoft365_contacts_read_contact \
      -params "{\"id\": \"$CONTACT_ID\", \"\$select\": \"displayName,emailAddresses,businessPhones\"}"
else
    echo "No contact ID found from contacts list"
fi

echo ""
echo "=== Individual Item Retrieval Tests Complete ==="
