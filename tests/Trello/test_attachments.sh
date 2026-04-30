#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

# Trello Card Attachment Tests

# Configurable card ID — override by setting TRELLO_CARD_ID in the environment or .env
TRELLO_CARD_ID="${TRELLO_CARD_ID:-PLACEHOLDER_CARD_ID}"

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

# Allow TRELLO_CARD_ID to be overridden from .env as well
TRELLO_CARD_ID="${TRELLO_CARD_ID:-PLACEHOLDER_CARD_ID}"

# Check if PROBE_PATH is set, otherwise use default
PROBE_PATH="${PROBE_PATH:-probe}"

# Append /mcp to the base URL
FULL_SERVER_URL="${SERVER_URL}/mcp"

echo "=== Testing Trello Card Attachment API ==="
echo "Timestamp: $(date)"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo "Card ID: $TRELLO_CARD_ID"
echo ""

# Test 1: List attachments on a card
echo "Test 1: List attachments on a card"
echo "Command: trello_list_card_attachments"
echo "Parameters: {\"cardId\": \"$TRELLO_CARD_ID\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" \
    -call trello_list_card_attachments \
    -params "{\"cardId\": \"$TRELLO_CARD_ID\"}"

echo ""
echo "=========================================="
echo ""

# Test 2: Attach a URL to a card
echo "Test 2: Add URL attachment to a card"
echo "Command: trello_add_card_attachment_url"
echo "Parameters: {\"cardId\": \"$TRELLO_CARD_ID\", \"url\": \"https://example.com\", \"name\": \"Example Link\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" \
    -call trello_add_card_attachment_url \
    -params "{\"cardId\": \"$TRELLO_CARD_ID\", \"url\": \"https://example.com\", \"name\": \"Example Link\"}"

echo ""
echo "=========================================="
echo ""

# Test 3: Upload a small text file as an attachment
echo "Test 3: Upload a file attachment to a card"
echo "Command: trello_add_card_attachment_file"
echo "Parameters: file_content, file_name, cardId"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" \
    -call trello_add_card_attachment_file \
    -params "{\"cardId\": \"$TRELLO_CARD_ID\", \"file_content\": \"# Test Notes\n\nThis is a test file uploaded via MCPFusion.\n\nTimestamp: $(date)\", \"file_name\": \"test_notes.md\", \"mimeType\": \"text/markdown\"}"

echo ""
echo "=========================================="
echo ""

echo "=== Trello Card Attachment Tests Complete ==="
