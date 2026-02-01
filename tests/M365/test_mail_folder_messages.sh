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

# Test Microsoft 365 Mail Folder Messages API
FULL_SERVER_URL="${SERVER_URL}/sse"

echo "=== Testing Microsoft 365 Mail Folder Messages API ===" 
echo "Timestamp: $(date)"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

# Common folder IDs that usually exist
INBOX_ID="inbox"
SENT_ID="sentitems"
DRAFTS_ID="drafts"

echo "Note: Using common folder names. For custom folders, run mail_folders_list first."
echo ""

echo "Test 1: Read messages from Inbox"
echo "Command: microsoft365_mail_folder_messages"
echo "Parameters: {\"folderId\": \"$INBOX_ID\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_folder_messages -params "{\"folderId\": \"$INBOX_ID\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 2: Read unread messages from Inbox"
echo "Command: microsoft365_mail_folder_messages"
echo "Parameters: {\"folderId\": \"$INBOX_ID\", \"\$filter\": \"isRead eq false\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_folder_messages -params "{\"folderId\": \"$INBOX_ID\", \"\$filter\": \"isRead eq false\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 3: Read messages from Sent Items"
echo "Command: microsoft365_mail_folder_messages"
echo "Parameters: {\"folderId\": \"$SENT_ID\", \"\$top\": \"5\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_folder_messages -params "{\"folderId\": \"$SENT_ID\", \"\$top\": \"5\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 4: Read messages with custom fields"
echo "Command: microsoft365_mail_folder_messages"
echo "Parameters: {\"folderId\": \"$INBOX_ID\", \"\$select\": \"subject,from,receivedDateTime,importance\", \"\$top\": \"3\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_folder_messages -params "{\"folderId\": \"$INBOX_ID\", \"\$select\": \"subject,from,receivedDateTime,importance\", \"\$top\": \"3\"}"

echo ""
echo "=== Mail Folder Messages API Tests Complete ==="
echo ""
echo "Common folder IDs you can use:"
echo "- inbox (Inbox)"
echo "- sentitems (Sent Items)"
echo "- drafts (Drafts)"
echo "- deleteditems (Deleted Items)"
echo "- junkemail (Junk Email)"
echo ""
echo "For custom folders, run: ./test_mail_folders.sh to get folder IDs"