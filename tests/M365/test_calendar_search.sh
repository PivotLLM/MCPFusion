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

# Test script for Calendar Search functionality
# Tests various search and filtering capabilities

echo "==============================================="
echo "Testing Microsoft 365 Calendar Search"
echo "==============================================="
echo

# Configuration
PROBE_PATH="/Users/eric/source/MCPProbe/probe"
FULL_SERVER_URL="${SERVER_URL}/sse"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
OUTPUT_FILE="calendar_search_${TIMESTAMP}.log"

echo "Test run: $TIMESTAMP" | tee "$OUTPUT_FILE"
echo "Server: $FULL_SERVER_URL" | tee -a "$OUTPUT_FILE"
echo "Using API Token: ${APIKEY:0:8}..." | tee -a "$OUTPUT_FILE"
echo "Probe tool: $PROBE_PATH" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Calculate date ranges (macOS compatible)
START_DATE=$(date -v-30d '+%Y%m%d')
END_DATE=$(date -v+30d '+%Y%m%d')

echo "Date range: $START_DATE to $END_DATE" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 1: Basic search for meetings
echo "=== Test 1: Search for events containing 'Meeting' ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_calendar_search with Meeting filter" | tee -a "$OUTPUT_FILE"
echo "Parameters: {\"startDate\": \"$START_DATE\", \"endDate\": \"$END_DATE\", \"\$filter\": \"contains(subject,'Meeting')\", \"\$top\": \"10\"}" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_search -params "{\"startDate\": \"$START_DATE\", \"endDate\": \"$END_DATE\", \"\$filter\": \"contains(subject,'Meeting')\", \"\$top\": \"10\"}" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 2: Search with multiple conditions
echo "=== Test 2: Search for 'Project' events after Jan 1, 2025 ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_calendar_search with Project filter and date condition" | tee -a "$OUTPUT_FILE"
echo "Parameters: {\"startDate\": \"$START_DATE\", \"endDate\": \"$END_DATE\", \"\$filter\": \"contains(subject,'Project')\", \"\$select\": \"subject,start,end,organizer\", \"\$top\": \"10\"}" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_search -params "{\"startDate\": \"$START_DATE\", \"endDate\": \"$END_DATE\", \"\$filter\": \"contains(subject,'Project')\", \"\$select\": \"subject,start,end,organizer\", \"\$top\": \"10\"}" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 3: Search by attendee email
echo "=== Test 3: Search for events with specific attendee ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_calendar_search with attendee filter" | tee -a "$OUTPUT_FILE"
echo "Parameters: {\"startDate\": \"$START_DATE\", \"endDate\": \"$END_DATE\", \"\$filter\": \"attendees/any(a:contains(a/emailAddress/address,'@'))\", \"\$select\": \"subject,start,end,attendees\", \"\$top\": \"5\"}" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_search -params "{\"startDate\": \"$START_DATE\", \"endDate\": \"$END_DATE\", \"\$filter\": \"attendees/any(a:contains(a/emailAddress/address,'@'))\", \"\$select\": \"subject,start,end,attendees\", \"\$top\": \"5\"}" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 4: Search by location
echo "=== Test 4: Search for events in specific location ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_calendar_search with location filter" | tee -a "$OUTPUT_FILE"
echo "Parameters: {\"startDate\": \"$START_DATE\", \"endDate\": \"$END_DATE\", \"\$filter\": \"contains(location/displayName,'Room')\", \"\$select\": \"subject,start,end,location\", \"\$top\": \"10\"}" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_search -params "{\"startDate\": \"$START_DATE\", \"endDate\": \"$END_DATE\", \"\$filter\": \"contains(location/displayName,'Room')\", \"\$select\": \"subject,start,end,location\", \"\$top\": \"10\"}" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 5: Search with OR condition
echo "=== Test 5: Search for 'Standup' OR 'Review' events ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_calendar_search with OR filter" | tee -a "$OUTPUT_FILE"
echo "Parameters: {\"startDate\": \"$START_DATE\", \"endDate\": \"$END_DATE\", \"\$filter\": \"contains(subject,'Standup') or contains(subject,'Review')\", \"\$select\": \"subject,start,end\", \"\$top\": \"15\"}" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_search -params "{\"startDate\": \"$START_DATE\", \"endDate\": \"$END_DATE\", \"\$filter\": \"contains(subject,'Standup') or contains(subject,'Review')\", \"\$select\": \"subject,start,end\", \"\$top\": \"15\"}" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 6: Search with custom field selection
echo "=== Test 6: Search with minimal field selection ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_calendar_search with minimal fields" | tee -a "$OUTPUT_FILE"
echo "Parameters: {\"startDate\": \"$START_DATE\", \"endDate\": \"$END_DATE\", \"\$filter\": \"contains(subject,'e')\", \"\$select\": \"subject,start\", \"\$top\": \"20\"}" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_search -params "{\"startDate\": \"$START_DATE\", \"endDate\": \"$END_DATE\", \"\$filter\": \"contains(subject,'e')\", \"\$select\": \"subject,start\", \"\$top\": \"20\"}" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 7: All events without filter (just date range)
echo "=== Test 7: All events in date range (no filter) ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_calendar_search with no filter" | tee -a "$OUTPUT_FILE"
echo "Parameters: {\"startDate\": \"$START_DATE\", \"endDate\": \"$END_DATE\", \"\$top\": \"5\"}" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call microsoft365_calendar_search -params "{\"startDate\": \"$START_DATE\", \"endDate\": \"$END_DATE\", \"\$top\": \"5\"}" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "==============================================="
echo "Calendar Search tests completed."
echo "Results saved to: $OUTPUT_FILE"
echo "==============================================="