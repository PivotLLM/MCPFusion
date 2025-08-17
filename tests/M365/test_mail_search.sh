#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025 Tenebris Technologies Inc.                                *
# All rights reserved.                                                         *
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

# Test script for Mail Search functionality
# Tests various search and filtering capabilities

echo "==============================================="
echo "Testing Microsoft 365 Mail Search"
echo "==============================================="
echo

# Configuration
PROBE_PATH="/Users/eric/source/MCPProbe/probe"
FULL_SERVER_URL="${SERVER_URL}/sse"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
OUTPUT_FILE="mail_search_${TIMESTAMP}.log"

echo "Test run: $TIMESTAMP" | tee "$OUTPUT_FILE"
echo "Server: $FULL_SERVER_URL" | tee -a "$OUTPUT_FILE"
echo "Using API Token: ${APIKEY:0:8}..." | tee -a "$OUTPUT_FILE"
echo "Probe tool: $PROBE_PATH" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 1: Search by subject content
echo "=== Test 1: Search for emails containing 'Invoice' in subject ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_mail_search \
  -params '{"$filter":"contains(subject,'\''Invoice'\'')","$top":"10"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 2: Search by sender email
echo "=== Test 2: Search for emails from specific domain ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_mail_search \
  -params '{"$filter":"contains(from/emailAddress/address,'"'"'@microsoft.com'"'"')", "$select":"subject,from,receivedDateTime", "$top":5}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 3: Search with date range and subject
echo "=== Test 3: Search for 'Project' emails after Jan 1, 2025 ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_mail_search \
  -params '{"$filter":"contains(subject,'"'"'Project'"'"') and receivedDateTime ge 2025-01-01T00:00:00Z", "$orderby":"receivedDateTime desc", "$top":10}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 4: Search for unread emails
echo "=== Test 4: Search for unread emails ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_mail_search \
  -params '{"$filter":"isRead eq false", "$select":"subject,from,receivedDateTime,isRead", "$top":15}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 5: Search for emails with attachments
echo "=== Test 5: Search for emails with attachments ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_mail_search \
  -params '{"$filter":"hasAttachments eq true", "$select":"subject,from,receivedDateTime,hasAttachments", "$top":10}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 6: Search with OR condition in body or subject
echo "=== Test 6: Search for 'urgent' in body or subject ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_mail_search \
  -params '{"$filter":"contains(bodyPreview,'"'"'urgent'"'"') or contains(subject,'"'"'urgent'"'"')", "$select":"subject,bodyPreview,from", "$top":10}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 7: Search for emails to specific recipients
echo "=== Test 7: Search for emails sent to team addresses ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_mail_search \
  -params '{"$filter":"toRecipients/any(r:contains(r/emailAddress/address,'"'"'team'"'"'))", "$select":"subject,toRecipients,from", "$top":5}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 8: Full-text search using $search parameter
echo "=== Test 8: Full-text search for 'invoice payment' ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_mail_search \
  -params '{"$search":"invoice payment", "$top":10}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 9: Full-text search with from constraint
echo "=== Test 9: Full-text search with sender constraint ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_mail_search \
  -params '{"$search":"from:microsoft.com", "$select":"subject,from,receivedDateTime", "$top":10}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 10: Search for meeting-related emails
echo "=== Test 10: Full-text search for meeting-related content ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_mail_search \
  -params '{"$search":"subject:meeting", "$orderby":"receivedDateTime desc", "$top":10}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 11: Search for emails with PDF attachments
echo "=== Test 11: Full-text search for PDF attachments ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_mail_search \
  -params '{"$search":"attachment:*.pdf", "$select":"subject,hasAttachments,from", "$top":5}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 12: Search with custom ordering
echo "=== Test 12: Search ordered by subject ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_mail_search \
  -params '{"$filter":"contains(subject,'"'"'e'"'"')", "$orderby":"subject asc", "$select":"subject,from", "$top":10}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "==============================================="
echo "Mail Search tests completed."
echo "Results saved to: $OUTPUT_FILE"
echo "==============================================="
