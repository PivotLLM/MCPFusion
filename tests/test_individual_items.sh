#!/bin/bash

# Test Microsoft 365 Individual Item Retrieval APIs
echo "=== Testing Microsoft 365 Individual Item Retrieval APIs ===" 
echo "Timestamp: $(date)"
echo "Server: http://127.0.0.1:8888/sse"
echo ""

# Placeholder IDs - replace with real IDs from list operations
CALENDAR_EVENT_ID="YOUR_EVENT_ID"
MESSAGE_ID="YOUR_MESSAGE_ID"
CONTACT_ID="YOUR_CONTACT_ID"

echo "NOTE: This script requires actual item IDs from list operations."
echo "Replace placeholder IDs with real ones obtained from other tests."
echo ""

echo "Test 1: Read specific calendar event"
echo "Command: microsoft365_calendar_read_event"
echo "Parameters: {\"id\": \"$CALENDAR_EVENT_ID\"}"
echo ""
echo "NOTE: Get event ID from calendar list operations first"
# Commented out to avoid errors - uncomment when you have real IDs
# /Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_calendar_read_event -params "{\"id\": \"$CALENDAR_EVENT_ID\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 2: Read specific calendar event with custom fields"
echo "Command: microsoft365_calendar_read_event"
echo "Parameters: {\"id\": \"$CALENDAR_EVENT_ID\", \"\$select\": \"subject,start,end,organizer,location\"}"
echo ""
echo "NOTE: Get event ID from calendar list operations first"
# Commented out to avoid errors - uncomment when you have real IDs
# /Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_calendar_read_event -params "{\"id\": \"$CALENDAR_EVENT_ID\", \"\$select\": \"subject,start,end,organizer,location\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 3: Read specific email message"
echo "Command: microsoft365_mail_read_message"
echo "Parameters: {\"id\": \"$MESSAGE_ID\"}"
echo ""
echo "NOTE: Get message ID from mail list operations first"
# Commented out to avoid errors - uncomment when you have real IDs
# /Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_mail_read_message -params "{\"id\": \"$MESSAGE_ID\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 4: Read specific email message with custom fields"
echo "Command: microsoft365_mail_read_message"
echo "Parameters: {\"id\": \"$MESSAGE_ID\", \"\$select\": \"subject,from,body,receivedDateTime\"}"
echo ""
echo "NOTE: Get message ID from mail list operations first"
# Commented out to avoid errors - uncomment when you have real IDs
# /Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_mail_read_message -params "{\"id\": \"$MESSAGE_ID\", \"\$select\": \"subject,from,body,receivedDateTime\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 5: Read specific contact"
echo "Command: microsoft365_contacts_read_contact"
echo "Parameters: {\"id\": \"$CONTACT_ID\"}"
echo ""
echo "NOTE: Get contact ID from contacts list operations first"
# Commented out to avoid errors - uncomment when you have real IDs
# /Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_contacts_read_contact -params "{\"id\": \"$CONTACT_ID\"}"

echo ""
echo "=========================================="
echo ""

echo "Test 6: Read specific contact with custom fields"
echo "Command: microsoft365_contacts_read_contact"
echo "Parameters: {\"id\": \"$CONTACT_ID\", \"\$select\": \"displayName,emailAddresses,businessPhones\"}"
echo ""
echo "NOTE: Get contact ID from contacts list operations first"
# Commented out to avoid errors - uncomment when you have real IDs
# /Users/eric/source/MCPProbe/probe -url http://127.0.0.1:8888/sse -transport sse -call microsoft365_contacts_read_contact -params "{\"id\": \"$CONTACT_ID\", \"\$select\": \"displayName,emailAddresses,businessPhones\"}"

echo ""
echo "=== Individual Item Retrieval API Tests Complete ==="
echo ""
echo "To run these tests:"
echo "1. First run list operations to get item IDs:"
echo "   - ./test_calendar_summary.sh (for event IDs)"
echo "   - ./test_mail.sh (for message IDs)"
echo "   - ./test_contacts.sh (for contact IDs)"
echo "2. Copy item IDs from the output"
echo "3. Edit this script and replace placeholder IDs with real ones"
echo "4. Uncomment the probe commands above"
echo "5. Run this script again"