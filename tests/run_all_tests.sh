#!/bin/bash

# MCPFusion Test Runner
# Executes all individual tests and saves output to separate .log files

# Configuration
TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVER_URL="http://127.0.0.1:8888/sse"
PROBE_TOOL="/Users/eric/source/MCPProbe/probe"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== MCPFusion Test Suite Runner ===${NC}"
echo "Timestamp: $(date)"
echo "Tests Directory: $TESTS_DIR"
echo "Server URL: $SERVER_URL"
echo ""

# Check prerequisites
echo -e "${BLUE}[INFO]${NC} Checking prerequisites..."

if [ ! -f "$PROBE_TOOL" ]; then
    echo -e "${RED}[ERROR]${NC} Probe tool not found at: $PROBE_TOOL"
    exit 1
fi

if [ ! -x "$PROBE_TOOL" ]; then
    echo -e "${RED}[ERROR]${NC} Probe tool is not executable: $PROBE_TOOL"
    exit 1
fi

echo -e "${GREEN}[PASS]${NC} Prerequisites check complete"
echo ""

# Test server connectivity
echo -e "${BLUE}[INFO]${NC} Testing server connectivity..."
if ! "$PROBE_TOOL" -url "$SERVER_URL" -transport sse -list-only >/dev/null 2>&1; then
    echo -e "${RED}[ERROR]${NC} Cannot connect to MCP server at $SERVER_URL"
    echo "Please ensure the MCPFusion server is running"
    exit 1
fi

echo -e "${GREEN}[PASS]${NC} Server is available"
echo ""

# Make test scripts executable
chmod +x "$TESTS_DIR"/*.sh

# Run individual tests
tests_run=0
tests_passed=0
tests_failed=0

run_test() {
    local test_name="$1"
    local test_script="$2"
    local log_file="$3"
    
    ((tests_run++))
    echo -e "${BLUE}[INFO]${NC} Running $test_name..."
    echo "Output file: $log_file"
    
    if "$test_script" > "$log_file" 2>&1; then
        echo -e "${GREEN}[PASS]${NC} $test_name completed successfully"
        ((tests_passed++))
    else
        echo -e "${RED}[FAIL]${NC} $test_name failed"
        echo "Check $log_file for details"
        ((tests_failed++))
    fi
    echo ""
}

# Execute all tests
echo -e "${BLUE}[INFO]${NC} Starting test execution..."
echo ""

run_test "Profile API Test" \
    "$TESTS_DIR/test_profile.sh" \
    "$TESTS_DIR/profile_test_${TIMESTAMP}.log"

run_test "Calendar Summary API Test" \
    "$TESTS_DIR/test_calendar_summary.sh" \
    "$TESTS_DIR/calendar_summary_test_${TIMESTAMP}.log"

run_test "Calendar Details API Test" \
    "$TESTS_DIR/test_calendar_details.sh" \
    "$TESTS_DIR/calendar_details_test_${TIMESTAMP}.log"

run_test "Mail API Test" \
    "$TESTS_DIR/test_mail.sh" \
    "$TESTS_DIR/mail_test_${TIMESTAMP}.log"

run_test "Contacts API Test" \
    "$TESTS_DIR/test_contacts.sh" \
    "$TESTS_DIR/contacts_test_${TIMESTAMP}.log"

run_test "Calendars List API Test" \
    "$TESTS_DIR/test_calendars_list.sh" \
    "$TESTS_DIR/calendars_list_test_${TIMESTAMP}.log"

run_test "Calendar Events API Test" \
    "$TESTS_DIR/test_calendar_events.sh" \
    "$TESTS_DIR/calendar_events_test_${TIMESTAMP}.log"

run_test "Mail Folders API Test" \
    "$TESTS_DIR/test_mail_folders.sh" \
    "$TESTS_DIR/mail_folders_test_${TIMESTAMP}.log"

run_test "Mail Folder Messages API Test" \
    "$TESTS_DIR/test_mail_folder_messages.sh" \
    "$TESTS_DIR/mail_folder_messages_test_${TIMESTAMP}.log"

run_test "Individual Items API Test" \
    "$TESTS_DIR/test_individual_items.sh" \
    "$TESTS_DIR/individual_items_test_${TIMESTAMP}.log"

run_test "Calendar Search API Test" \
    "$TESTS_DIR/test_calendar_search.sh" \
    "$TESTS_DIR/calendar_search_test_${TIMESTAMP}.log"

run_test "Mail Search API Test" \
    "$TESTS_DIR/test_mail_search.sh" \
    "$TESTS_DIR/mail_search_test_${TIMESTAMP}.log"

run_test "Files Search API Test" \
    "$TESTS_DIR/test_files_search.sh" \
    "$TESTS_DIR/files_search_test_${TIMESTAMP}.log"

# Summary
echo -e "${BLUE}=== Test Summary ===${NC}"
echo "Total Tests: $tests_run"
echo -e "${GREEN}Passed: $tests_passed${NC}"
echo -e "${RED}Failed: $tests_failed${NC}"

if [ $tests_failed -eq 0 ]; then
    echo -e "${GREEN}[SUCCESS]${NC} All tests passed! 🎉"
    echo ""
    echo "Test output files created in $TESTS_DIR:"
    ls -la "$TESTS_DIR"/*_${TIMESTAMP}.log
    exit 0
else
    echo -e "${RED}[FAILURE]${NC} Some tests failed"
    echo ""
    echo "Check the following log files for details:"
    ls -la "$TESTS_DIR"/*_${TIMESTAMP}.log
    exit 1
fi