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

# Test Microsoft 365 Mail Draft List API
echo "=== Testing Microsoft 365 Mail Draft List API ==="
echo "Timestamp: $(date)"
FULL_SERVER_URL="${SERVER_URL}/sse"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

echo "Test 1: List drafts with defaults"
echo "Command: microsoft365_mail_draft_list with default parameters"
echo "Parameters: {}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_draft_list -params '{}'

echo ""
echo "=========================================="
echo ""

echo "Test 2: List drafts with top 5"
echo "Command: microsoft365_mail_draft_list with top 5"
echo "Parameters: {\"\$top\": \"5\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_draft_list -params '{"$top": "5"}'

echo ""
echo "=========================================="
echo ""

echo "Test 3: List drafts filtered by last 7 days"
echo "Command: microsoft365_mail_draft_list filtered by date"
echo "Parameters: {\"\$filter\": \"lastModifiedDateTime ge 2025-01-01T00:00:00Z\", \"\$top\": \"10\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_draft_list -params '{"$filter": "lastModifiedDateTime ge 2025-01-01T00:00:00Z", "$top": "10"}'

echo ""
echo "=========================================="
echo ""

echo "Test 4: List drafts with custom select fields"
echo "Command: microsoft365_mail_draft_list with custom fields"
echo "Parameters: {\"\$select\": \"subject,toRecipients,lastModifiedDateTime,importance\", \"\$top\": \"5\"}"
echo ""
/Users/eric/source/MCPProbe/probe -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_draft_list -params '{"$select": "subject,toRecipients,lastModifiedDateTime,importance", "$top": "5"}'

echo ""
echo "=== Mail Draft List API Tests Complete ==="
