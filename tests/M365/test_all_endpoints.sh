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

if [ -z "$APIKEY" ]; then
    echo "Error: APIKEY not set in .env file"
    exit 1
fi

# Check if SERVER_URL is set
if [ -z "$SERVER_URL" ]; then
    echo "Error: SERVER_URL not set in .env file"
    exit 1
fi

FULL_SERVER_URL="${SERVER_URL}/sse"
echo "=== Testing All Microsoft 365 Endpoints ==="
echo "Timestamp: $(date)"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

# Function to run a test with timeout
run_test() {
    local tool_name=$1
    local params=$2
    local description=$3
    
    echo "----------------------------------------"
    echo "Testing: $tool_name"
    echo "Description: $description"
    echo "Parameters: $params"
    
    # Run with timeout and capture result
    result=$(gtimeout 8 /Users/eric/source/MCPProbe/probe \
        -url "$FULL_SERVER_URL" \
        -transport sse \
        -headers "Authorization:Bearer $APIKEY" \
        -call "$tool_name" \
        -params "$params" 2>&1)
    
    # Check for specific patterns
    if echo "$result" | grep -q "Please visit.*devicelogin"; then
        echo "❌ NEEDS AUTH: OAuth device flow required"
        echo "$result" | grep "Please visit" | head -1
    elif echo "$result" | grep -q "Failed to call tool"; then
        echo "❌ ERROR: Tool call failed"
        echo "$result" | grep -A1 "Failed to call tool" | head -2
    elif echo "$result" | grep -q '"error"'; then
        echo "❌ ERROR:"
        echo "$result" | grep -A2 '"error"' | head -3
    elif echo "$result" | grep -q "=== Finished ==="; then
        echo "✅ SUCCESS: Response received"
        echo "$result" | grep -B2 "=== Finished ===" | head -1 | cut -c1-100
    elif echo "$result" | grep -q "@odata"; then
        echo "✅ SUCCESS: Microsoft 365 data received"
        echo "$result" | grep -A1 "@odata" | head -2 | cut -c1-100
    elif echo "$result" | grep -q '"content"'; then
        echo "✅ SUCCESS: Response received"
        echo "$result" | grep '"content"' | head -1 | cut -c1-100
    elif echo "$result" | grep -q "Tool response:"; then
        echo "✅ SUCCESS: Tool responded"
        echo "$result" | grep -A1 "Tool response:" | head -2
    else
        echo "⚠️  TIMEOUT or UNKNOWN: No clear response within 8 seconds"
    fi
    echo ""
}

# Calculate date ranges
start_date=$(date -v-7d +%Y%m%d 2>/dev/null || date -d "-7 days" +%Y%m%d)
end_date=$(date -v+7d +%Y%m%d 2>/dev/null || date -d "+7 days" +%Y%m%d)

echo "Date range for tests: $start_date to $end_date"
echo ""

# Test each endpoint
run_test "microsoft365_profile_get" '{}' "Get user profile"

run_test "microsoft365_calendar_read_summary" "{\"startDate\":\"$start_date\",\"endDate\":\"$end_date\"}" "Calendar summary"

run_test "microsoft365_calendar_read_details" "{\"startDate\":\"$start_date\",\"endDate\":\"$end_date\"}" "Calendar details"

run_test "microsoft365_calendars_list" '{"$top":"2"}' "List calendars"

run_test "microsoft365_mail_read_inbox" '{"$top":"5"}' "Read inbox"

run_test "microsoft365_mail_folders_list" '{"$top":"5"}' "List mail folders"

run_test "microsoft365_contacts_list" '{"$top":"5"}' "List contacts"

run_test "microsoft365_mail_search" '{"query":"test","$top":"5"}' "Search mail"

run_test "microsoft365_calendar_search" "{\"query\":\"meeting\",\"startDate\":\"$start_date\",\"endDate\":\"$end_date\"}" "Search calendar"

run_test "microsoft365_files_search" '{"query":"document","$top":"5"}' "Search files"

echo "=========================================="
echo "Test Summary Complete"
echo ""
echo "Legend:"
echo "✅ SUCCESS - Endpoint returned data"
echo "❌ NEEDS AUTH - OAuth authentication required"
echo "❌ ERROR - Endpoint returned an error"
echo "⚠️  TIMEOUT - No response within 3 seconds"