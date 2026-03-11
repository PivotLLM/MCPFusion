#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

# Object Array Parameters Regression Test - M365
#
# This test specifically covers the bug fix where array parameters containing
# objects (not strings) were being silently dropped. It verifies that object
# arrays such as toRecipients, ccRecipients, bccRecipients, and attendees are
# correctly passed through to the API.
#
# NOTE: This test creates real draft messages and a calendar event that must
# be cleaned up manually after running. The calendar event uses a date far
# in the future (2099) to make it easy to identify and delete.

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

# Append /mcp to the base URL
FULL_SERVER_URL="${SERVER_URL}/mcp"

echo "=== M365 Object Array Parameters Regression Test ==="
echo "Timestamp: $(date)"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""
echo "NOTE: This test creates real draft messages and a calendar event."
echo "      Created items must be cleaned up manually after running."
echo "      The calendar event uses date 2099-12-31 for easy identification."
echo ""

# Test 1: Create mail draft with toRecipients as an array of objects
echo "Test 1: Create mail draft with toRecipients as an object array"
echo "Command: microsoft365_mail_draft_create"
echo "Parameters: {\"subject\": \"Object Array Regression Test\", \"body\": \"Testing toRecipients as object array\", \"toRecipients\": [{\"emailAddress\": {\"address\": \"test@example.com\", \"name\": \"Test User\"}}]}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_draft_create -params '{"subject": "Object Array Regression Test", "body": "Testing toRecipients as object array", "toRecipients": [{"emailAddress": {"address": "test@example.com", "name": "Test User"}}]}'

echo ""
echo "=========================================="
echo ""

# Test 2: Create mail draft with toRecipients, ccRecipients, and bccRecipients all as object arrays
echo "Test 2: Create mail draft with toRecipients, ccRecipients, and bccRecipients as object arrays"
echo "Command: microsoft365_mail_draft_create"
echo "Parameters: {\"subject\": \"Object Array Multi-Recipient Test\", \"body\": \"Testing multiple recipient object arrays\", \"toRecipients\": [{\"emailAddress\": {\"address\": \"to@example.com\", \"name\": \"To User\"}}], \"ccRecipients\": [{\"emailAddress\": {\"address\": \"cc@example.com\", \"name\": \"CC User\"}}], \"bccRecipients\": [{\"emailAddress\": {\"address\": \"bcc@example.com\", \"name\": \"BCC User\"}}]}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_draft_create -params '{"subject": "Object Array Multi-Recipient Test", "body": "Testing multiple recipient object arrays", "toRecipients": [{"emailAddress": {"address": "to@example.com", "name": "To User"}}], "ccRecipients": [{"emailAddress": {"address": "cc@example.com", "name": "CC User"}}], "bccRecipients": [{"emailAddress": {"address": "bcc@example.com", "name": "BCC User"}}]}'

echo ""
echo "=========================================="
echo ""

# Test 3: List drafts to verify they were created (not silently dropped)
echo "Test 3: List drafts to verify the drafts from Tests 1 and 2 were created"
echo "Command: microsoft365_mail_draft_list"
echo "Parameters: {}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call microsoft365_mail_draft_list -params '{}'

echo ""
echo "=========================================="
echo ""

# Test 4: Create calendar event with attendees as an object array
echo "Test 4: Create calendar event with attendees as an object array"
echo "Command: microsoft365_calendar_event_create"
echo "Parameters: {\"subject\": \"Object Array Regression Test Event\", \"body\": \"Testing attendees as object array - please delete\", \"start\": \"2099-12-31T10:00:00\", \"end\": \"2099-12-31T11:00:00\", \"attendees\": [{\"emailAddress\": {\"address\": \"test@example.com\", \"name\": \"Test Attendee\"}, \"type\": \"required\"}]}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport http -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_event_create -params '{"subject": "Object Array Regression Test Event", "body": "Testing attendees as object array - please delete", "start": "2099-12-31T10:00:00", "end": "2099-12-31T11:00:00", "attendees": [{"emailAddress": {"address": "test@example.com", "name": "Test Attendee"}, "type": "required"}]}'

echo ""
echo "=========================================="
echo ""
echo "NOTE: Manual cleanup required:"
echo "  - Delete the two draft messages created in Tests 1 and 2"
echo "    (subjects: 'Object Array Regression Test' and 'Object Array Multi-Recipient Test')"
echo "  - Delete the calendar event created in Test 4"
echo "    (subject: 'Object Array Regression Test Event', date: 2099-12-31)"
echo ""
echo "=== M365 Object Array Parameters Regression Test Complete ==="
