#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

# Load environment variables
if [ -f ".env" ]; then
    source .env
else
    echo "Error: .env file not found"
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

# Test Microsoft 365 Mail Draft Update API
echo "=== Testing Microsoft 365 Mail Draft Update API ==="
echo "Timestamp: $(date)"
FULL_SERVER_URL="${SERVER_URL}/sse"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

echo "Test 1: Update draft subject"
echo "Command: microsoft365_mail_draft_update with draftId and subject"
echo "Parameters: {\"draftId\": \"DRAFT_ID_HERE\", \"subject\": \"Updated: Project Update - Q1 Review (Revised)\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_draft_update -params '{"draftId": "DRAFT_ID_HERE", "subject": "Updated: Project Update - Q1 Review (Revised)"}'

echo ""
echo "=========================================="
echo ""

echo "Test 2: Update draft body"
echo "Command: microsoft365_mail_draft_update with draftId and body"
echo "Parameters: {\"draftId\": \"DRAFT_ID_HERE\", \"body\": \"<html><body><p>This is the revised content for the draft message.</p><p>Please disregard the previous version.</p></body></html>\", \"bodyContentType\": \"HTML\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_draft_update -params '{"draftId": "DRAFT_ID_HERE", "body": "<html><body><p>This is the revised content for the draft message.</p><p>Please disregard the previous version.</p></body></html>", "bodyContentType": "HTML"}'

echo ""
echo "=========================================="
echo ""

echo "Test 3: Update draft recipients"
echo "Command: microsoft365_mail_draft_update with draftId and updated recipients"
echo "Parameters: {\"draftId\": \"DRAFT_ID_HERE\", \"toRecipients\": \"new-recipient@example.com,another-recipient@example.com\", \"ccRecipients\": \"cc-user@example.com\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_draft_update -params '{"draftId": "DRAFT_ID_HERE", "toRecipients": "new-recipient@example.com,another-recipient@example.com", "ccRecipients": "cc-user@example.com"}'

echo ""
echo "=== Mail Draft Update API Tests Complete ==="
