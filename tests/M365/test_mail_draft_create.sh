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

# Check if PROBE_PATH is set, otherwise use default
PROBE_PATH="${PROBE_PATH:-probe}"

# Test Microsoft 365 Mail Draft Create API
echo "=== Testing Microsoft 365 Mail Draft Create API ==="
echo "Timestamp: $(date)"
FULL_SERVER_URL="${SERVER_URL}/mcp"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

echo "Test 1: Create draft with subject, HTML body, and toRecipients"
echo "Command: microsoft365_mail_draft_create with subject, body, and toRecipients"
echo "Parameters: {\"subject\": \"Project Update - Q1 Review\", \"body\": \"<html><body><h1>Q1 Review</h1><p>Please find the quarterly update attached.</p></body></html>\", \"bodyContentType\": \"HTML\", \"toRecipients\": \"user@example.com\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_draft_create -params '{"subject": "Project Update - Q1 Review", "body": "<html><body><h1>Q1 Review</h1><p>Please find the quarterly update attached.</p></body></html>", "bodyContentType": "HTML", "toRecipients": "user@example.com"}'

echo ""
echo "=========================================="
echo ""

echo "Test 2: Create draft with subject and text body only (no recipients)"
echo "Command: microsoft365_mail_draft_create with subject and plain text body"
echo "Parameters: {\"subject\": \"Meeting Notes - Draft\", \"body\": \"These are the meeting notes from today's standup.\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_draft_create -params '{"subject": "Meeting Notes - Draft", "body": "These are the meeting notes from today'\''s standup."}'

echo ""
echo "=========================================="
echo ""

echo "Test 3: Create draft with all recipient types (to, cc, bcc) and high importance"
echo "Command: microsoft365_mail_draft_create with to, cc, bcc recipients and importance"
echo "Parameters: {\"subject\": \"Urgent: Budget Approval Required\", \"body\": \"<html><body><p>Please review and approve the attached budget proposal by EOD Friday.</p></body></html>\", \"bodyContentType\": \"HTML\", \"toRecipients\": \"manager@example.com,director@example.com\", \"ccRecipients\": \"finance@example.com\", \"bccRecipients\": \"archive@example.com\", \"importance\": \"high\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_draft_create -params '{"subject": "Urgent: Budget Approval Required", "body": "<html><body><p>Please review and approve the attached budget proposal by EOD Friday.</p></body></html>", "bodyContentType": "HTML", "toRecipients": "manager@example.com,director@example.com", "ccRecipients": "finance@example.com", "bccRecipients": "archive@example.com", "importance": "high"}'

echo ""
echo "=== Mail Draft Create API Tests Complete ==="
