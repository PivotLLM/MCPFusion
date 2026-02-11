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

# Append /sse to the base URL
FULL_SERVER_URL="${SERVER_URL}/sse"

# Test Google Gmail Draft Create API
echo "=== Testing Google Gmail Draft Create API ==="
echo "Timestamp: $(date)"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

echo "Test 1: Create draft with to, subject, and body"
echo "Command: google_gmail_draft_create with to, subject, and body"
echo "Parameters: {\"to\": \"user@example.com\", \"subject\": \"Project Update - Q1 Review\", \"body\": \"Please find the quarterly update below.\", \"bodyContentType\": \"Text\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call google_gmail_draft_create -params '{"to": "user@example.com", "subject": "Project Update - Q1 Review", "body": "Please find the quarterly update below.", "bodyContentType": "Text"}'

echo ""
echo "=========================================="
echo ""

echo "Test 2: Create draft with subject and body only (no recipient)"
echo "Command: google_gmail_draft_create with subject and body only"
echo "Parameters: {\"subject\": \"Meeting Notes - Draft\", \"body\": \"These are the meeting notes from today.\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call google_gmail_draft_create -params '{"subject": "Meeting Notes - Draft", "body": "These are the meeting notes from today."}'

echo ""
echo "=========================================="
echo ""

echo "Test 3: Create draft with to, cc, bcc, subject, and body"
echo "Command: google_gmail_draft_create with all recipient types"
echo "Parameters: {\"to\": \"manager@example.com\", \"cc\": \"finance@example.com\", \"bcc\": \"archive@example.com\", \"subject\": \"Budget Approval Required\", \"body\": \"<html><body><p>Please review and approve the attached budget proposal.</p></body></html>\", \"bodyContentType\": \"HTML\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call google_gmail_draft_create -params '{"to": "manager@example.com", "cc": "finance@example.com", "bcc": "archive@example.com", "subject": "Budget Approval Required", "body": "<html><body><p>Please review and approve the attached budget proposal.</p></body></html>", "bodyContentType": "HTML"}'

echo ""
echo "=== Gmail Draft Create API Tests Complete ==="
