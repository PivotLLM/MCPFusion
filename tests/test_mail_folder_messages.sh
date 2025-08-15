#!/bin/bash

# Test Microsoft 365 Mail Folder Messages API
echo "=== Testing Microsoft 365 Mail Folder Messages API ===" 
echo "Timestamp: $(date)"
echo "Server: http://127.0.0.1:8888/sse"
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
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_mail_folder_messages -params "{\"folderId\": \"$INBOX_ID\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 2: Read unread messages from Inbox"
echo "Command: microsoft365_mail_folder_messages"
echo "Parameters: {\"folderId\": \"$INBOX_ID\", \"\$filter\": \"isRead eq false\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_mail_folder_messages -params "{\"folderId\": \"$INBOX_ID\", \"\$filter\": \"isRead eq false\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 3: Read messages from Sent Items"
echo "Command: microsoft365_mail_folder_messages"
echo "Parameters: {\"folderId\": \"$SENT_ID\", \"\$top\": \"5\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_mail_folder_messages -params "{\"folderId\": \"$SENT_ID\", \"\$top\": \"5\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 4: Read messages with custom fields"
echo "Command: microsoft365_mail_folder_messages"
echo "Parameters: {\"folderId\": \"$INBOX_ID\", \"\$select\": \"subject,from,receivedDateTime,importance\", \"\$top\": \"3\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_mail_folder_messages -params "{\"folderId\": \"$INBOX_ID\", \"\$select\": \"subject,from,receivedDateTime,importance\", \"\$top\": \"3\"}"

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