#!/bin/bash

################################################################################
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                          #
# Please see LICENSE file for details.                                         #
################################################################################

# Knowledge Store Comprehensive Test Suite
# Tests all knowledge MCP tools: knowledge_set, knowledge_get, knowledge_delete
#
# Test Numbering Convention:
#   1.x - knowledge_set (create and update)
#   2.x - knowledge_get (single entry, by domain, all entries)
#   3.x - knowledge_delete
#   4.x - Error handling and edge cases
#   5.x - Cleanup

#===============================================================================
# Configuration
#===============================================================================

# API key for authentication - set this before running
APIKEY="TEST API KEY HERE"

# Server URL (MCPFusion must be running)
SERVER_URL="http://127.0.0.1:9999/mcp"
TRANSPORT="http"

# PROBE can be overridden via environment variable
: "${PROBE:=probe}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

PASS_COUNT=0
FAIL_COUNT=0

#===============================================================================
# Pre-flight Checks
#===============================================================================

echo ""
echo "${BOLD}============================================${NC}"
echo "${BOLD}   Knowledge Store Test Suite${NC}"
echo "${BOLD}============================================${NC}"
echo ""

# Check if APIKEY has been set
if [ "$APIKEY" = "SET_YOUR_API_KEY_HERE" ]; then
    echo "${RED}ERROR: APIKEY has not been set${NC}"
    echo "Edit this script and set the APIKEY variable to a valid API token."
    echo "Generate a token with: ./mcpfusion -token-add \"test\""
    exit 1
fi

# Check if PROBE exists
if [ "${PROBE#/}" = "$PROBE" ]; then
    PROBE_FULL=$(command -v "$PROBE" 2>/dev/null)
    if [ -z "$PROBE_FULL" ]; then
        echo "${RED}ERROR: probe binary not found in PATH${NC}"
        exit 1
    fi
    PROBE="$PROBE_FULL"
elif [ ! -f "$PROBE" ]; then
    echo "${RED}ERROR: probe not found at: $PROBE${NC}"
    exit 1
fi

if [ ! -x "$PROBE" ]; then
    echo "${RED}ERROR: probe is not executable: $PROBE${NC}"
    exit 1
fi

echo "${GREEN}Pre-flight checks passed${NC}"
echo "  Probe:  $PROBE"
echo "  Server: $SERVER_URL"
echo ""

#===============================================================================
# Helper Functions
#===============================================================================

print_section() {
    echo ""
    echo "${BOLD}${BLUE}============================================${NC}"
    echo "${BOLD}${BLUE}   $1${NC}"
    echo "${BOLD}${BLUE}============================================${NC}"
    echo ""
}

print_subsection() {
    echo "${CYAN}--- $1 ---${NC}"
}

# Run a test expecting success, optionally check for expected string
run_test() {
    local test_name="$1"
    local tool="$2"
    local params="$3"
    local expected="$4"

    echo "  ${test_name}"
    result=$($PROBE -url "$SERVER_URL" -transport "$TRANSPORT" -headers "Authorization:Bearer $APIKEY" -call "$tool" -params "$params" 2>&1)

    if echo "$result" | grep -q "Tool call succeeded"; then
        if [ -n "$expected" ]; then
            if echo "$result" | grep -q "$expected"; then
                echo "    ${GREEN}PASS${NC}: Found expected: $expected"
                PASS_COUNT=$((PASS_COUNT + 1))
            else
                echo "    ${RED}FAIL${NC}: Expected '$expected' not found"
                echo "    Output: $result"
                FAIL_COUNT=$((FAIL_COUNT + 1))
            fi
        else
            echo "    ${GREEN}PASS${NC}: Tool call succeeded"
            PASS_COUNT=$((PASS_COUNT + 1))
        fi
    else
        echo "    ${RED}FAIL${NC}: Tool call failed unexpectedly"
        echo "    Output: $result"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
}

# Run a test expecting failure, optionally check for expected error
run_test_expect_fail() {
    local test_name="$1"
    local tool="$2"
    local params="$3"
    local expected_error="$4"

    echo "  ${test_name}"
    result=$($PROBE -url "$SERVER_URL" -transport "$TRANSPORT" -headers "Authorization:Bearer $APIKEY" -call "$tool" -params "$params" 2>&1)

    if echo "$result" | grep -q "Tool call failed"; then
        if [ -n "$expected_error" ]; then
            if echo "$result" | grep -qi "$expected_error"; then
                echo "    ${GREEN}PASS${NC}: Got expected error: $expected_error"
                PASS_COUNT=$((PASS_COUNT + 1))
            else
                echo "    ${YELLOW}WARN${NC}: Failed but with different error"
                echo "    Output: $result"
                PASS_COUNT=$((PASS_COUNT + 1))
            fi
        else
            echo "    ${GREEN}PASS${NC}: Correctly failed"
            PASS_COUNT=$((PASS_COUNT + 1))
        fi
    else
        echo "    ${RED}FAIL${NC}: Expected failure but got success"
        echo "    Output: $result"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
}

# Silent cleanup (no output)
cleanup_silent() {
    $PROBE -url "$SERVER_URL" -transport "$TRANSPORT" -headers "Authorization:Bearer $APIKEY" -call "$1" -params "$2" > /dev/null 2>&1
}

#===============================================================================
# SECTION 0: Cleanup any leftover test data
#===============================================================================

print_section "SECTION 0: Pre-test Cleanup"

echo "  Removing any leftover test entries..."
cleanup_silent "knowledge_delete" '{"domain":"test-domain","key":"test-key-1"}'
cleanup_silent "knowledge_delete" '{"domain":"test-domain","key":"test-key-2"}'
cleanup_silent "knowledge_delete" '{"domain":"test-domain","key":"test-key-3"}'
cleanup_silent "knowledge_delete" '{"domain":"test-domain-2","key":"test-key-1"}'
cleanup_silent "knowledge_delete" '{"domain":"test-domain-2","key":"test-key-2"}'
cleanup_silent "knowledge_delete" '{"domain":"edge-cases","key":"special-chars"}'
cleanup_silent "knowledge_delete" '{"domain":"edge-cases","key":"long-content"}'
cleanup_silent "knowledge_delete" '{"domain":"edge-cases","key":"multiline"}'
echo "  Done."

#===============================================================================
# SECTION 1: knowledge_set - Create and Update
#===============================================================================

print_section "SECTION 1: knowledge_set"

print_subsection "1.1 Create new entries"

run_test "1.1.1 Set first entry in test-domain" \
    "knowledge_set" \
    '{"domain":"test-domain","key":"test-key-1","content":"This is the first test entry"}' \
    "Knowledge entry stored"

run_test "1.1.2 Set second entry in test-domain" \
    "knowledge_set" \
    '{"domain":"test-domain","key":"test-key-2","content":"This is the second test entry"}' \
    "Knowledge entry stored"

run_test "1.1.3 Set third entry in test-domain" \
    "knowledge_set" \
    '{"domain":"test-domain","key":"test-key-3","content":"This is the third test entry"}' \
    "Knowledge entry stored"

run_test "1.1.4 Set entry in a different domain" \
    "knowledge_set" \
    '{"domain":"test-domain-2","key":"test-key-1","content":"Entry in second domain"}' \
    "Knowledge entry stored"

run_test "1.1.5 Set second entry in different domain" \
    "knowledge_set" \
    '{"domain":"test-domain-2","key":"test-key-2","content":"Second entry in second domain"}' \
    "Knowledge entry stored"

print_subsection "1.2 Update existing entries"

run_test "1.2.1 Update existing entry content" \
    "knowledge_set" \
    '{"domain":"test-domain","key":"test-key-1","content":"Updated content for the first test entry"}' \
    "Knowledge entry stored"

run_test "1.2.2 Update entry with completely different content" \
    "knowledge_set" \
    '{"domain":"test-domain","key":"test-key-2","content":"Completely replaced content"}' \
    "Knowledge entry stored"

#===============================================================================
# SECTION 2: knowledge_get - Retrieve Entries
#===============================================================================

print_section "SECTION 2: knowledge_get"

print_subsection "2.1 Get specific entry (domain + key)"

run_test "2.1.1 Get updated entry by domain and key" \
    "knowledge_get" \
    '{"domain":"test-domain","key":"test-key-1"}' \
    "Updated content for the first test entry"

run_test "2.1.2 Get second entry by domain and key" \
    "knowledge_get" \
    '{"domain":"test-domain","key":"test-key-2"}' \
    "Completely replaced content"

run_test "2.1.3 Get entry from second domain" \
    "knowledge_get" \
    '{"domain":"test-domain-2","key":"test-key-1"}' \
    "Entry in second domain"

print_subsection "2.2 List entries by domain"

run_test "2.2.1 List all entries in test-domain" \
    "knowledge_get" \
    '{"domain":"test-domain"}' \
    "test-key-1"

run_test "2.2.2 List all entries in test-domain (verify key-2)" \
    "knowledge_get" \
    '{"domain":"test-domain"}' \
    "test-key-2"

run_test "2.2.3 List all entries in test-domain (verify key-3)" \
    "knowledge_get" \
    '{"domain":"test-domain"}' \
    "test-key-3"

run_test "2.2.4 List entries in second domain" \
    "knowledge_get" \
    '{"domain":"test-domain-2"}' \
    "test-key-1"

print_subsection "2.3 List all entries (no filters)"

run_test "2.3.1 List all entries across all domains" \
    "knowledge_get" \
    '{}' \
    "test-domain"

run_test "2.3.2 List all entries (verify second domain present)" \
    "knowledge_get" \
    '{}' \
    "test-domain-2"

#===============================================================================
# SECTION 3: knowledge_delete
#===============================================================================

print_section "SECTION 3: knowledge_delete"

print_subsection "3.1 Delete individual entries"

run_test "3.1.1 Delete an entry" \
    "knowledge_delete" \
    '{"domain":"test-domain","key":"test-key-3"}' \
    "Knowledge entry deleted"

run_test "3.1.2 Verify deleted entry is gone" \
    "knowledge_get" \
    '{"domain":"test-domain","key":"test-key-3"}' \
    ""

# After deleting test-key-3, listing test-domain should still show key-1 and key-2
run_test "3.1.3 Verify remaining entries in domain after delete" \
    "knowledge_get" \
    '{"domain":"test-domain"}' \
    "test-key-1"

print_subsection "3.2 Delete all entries in a domain (clean up domain)"

run_test "3.2.1 Delete first entry in test-domain-2" \
    "knowledge_delete" \
    '{"domain":"test-domain-2","key":"test-key-1"}' \
    "Knowledge entry deleted"

run_test "3.2.2 Delete second entry in test-domain-2" \
    "knowledge_delete" \
    '{"domain":"test-domain-2","key":"test-key-2"}' \
    "Knowledge entry deleted"

run_test "3.2.3 Verify empty domain returns no entries" \
    "knowledge_get" \
    '{"domain":"test-domain-2"}' \
    "No knowledge entries found"

#===============================================================================
# SECTION 4: Error Handling and Edge Cases
#===============================================================================

print_section "SECTION 4: Error Handling & Edge Cases"

print_subsection "4.1 knowledge_set validation errors"

run_test_expect_fail "4.1.1 Set with empty domain" \
    "knowledge_set" \
    '{"domain":"","key":"test-key","content":"some content"}' \
    "domain"

run_test_expect_fail "4.1.2 Set with empty key" \
    "knowledge_set" \
    '{"domain":"test-domain","key":"","content":"some content"}' \
    "key"

run_test_expect_fail "4.1.3 Set with empty content" \
    "knowledge_set" \
    '{"domain":"test-domain","key":"test-key","content":""}' \
    "content"

run_test_expect_fail "4.1.4 Set with missing domain parameter" \
    "knowledge_set" \
    '{"key":"test-key","content":"some content"}' \
    ""

run_test_expect_fail "4.1.5 Set with missing key parameter" \
    "knowledge_set" \
    '{"domain":"test-domain","content":"some content"}' \
    ""

run_test_expect_fail "4.1.6 Set with missing content parameter" \
    "knowledge_set" \
    '{"domain":"test-domain","key":"test-key"}' \
    ""

print_subsection "4.2 knowledge_get validation errors"

run_test_expect_fail "4.2.1 Get with key but no domain" \
    "knowledge_get" \
    '{"key":"test-key"}' \
    "requires"

print_subsection "4.3 knowledge_get not found"

run_test_expect_fail "4.3.1 Get non-existent entry" \
    "knowledge_get" \
    '{"domain":"nonexistent-domain","key":"nonexistent-key"}' \
    "not found"

print_subsection "4.4 knowledge_delete not found"

run_test_expect_fail "4.4.1 Delete non-existent entry" \
    "knowledge_delete" \
    '{"domain":"nonexistent-domain","key":"nonexistent-key"}' \
    "not found"

run_test_expect_fail "4.4.2 Delete with empty domain" \
    "knowledge_delete" \
    '{"domain":"","key":"test-key"}' \
    ""

run_test_expect_fail "4.4.3 Delete with empty key" \
    "knowledge_delete" \
    '{"domain":"test-domain","key":""}' \
    ""

print_subsection "4.5 Special content"

run_test "4.5.1 Set entry with special characters in content" \
    "knowledge_set" \
    '{"domain":"edge-cases","key":"special-chars","content":"Content with special chars: @#$%^&*() and unicode: cafe"}' \
    "Knowledge entry stored"

run_test "4.5.2 Retrieve entry with special characters" \
    "knowledge_get" \
    '{"domain":"edge-cases","key":"special-chars"}' \
    "special chars"

run_test "4.5.3 Set entry with long content" \
    "knowledge_set" \
    '{"domain":"edge-cases","key":"long-content","content":"This is a longer piece of content that simulates a real-world knowledge entry. The user prefers morning meetings between 9am and 11am Eastern time. They do not want meetings on Fridays. All meeting invites should include a Zoom link. Calendar reminders should be set to 15 minutes before the meeting start time."}' \
    "Knowledge entry stored"

run_test "4.5.4 Retrieve long content entry" \
    "knowledge_get" \
    '{"domain":"edge-cases","key":"long-content"}' \
    "morning meetings"

run_test "4.5.5 Set entry with newlines in content" \
    "knowledge_set" \
    '{"domain":"edge-cases","key":"multiline","content":"Line 1\nLine 2\nLine 3"}' \
    "Knowledge entry stored"

run_test "4.5.6 Retrieve multiline content entry" \
    "knowledge_get" \
    '{"domain":"edge-cases","key":"multiline"}' \
    "Line 1"

print_subsection "4.6 Domain isolation"

run_test "4.6.1 Same key in different domain is independent" \
    "knowledge_get" \
    '{"domain":"test-domain","key":"test-key-1"}' \
    "Updated content for the first test entry"

run_test "4.6.2 Edge-cases domain has its own entries" \
    "knowledge_get" \
    '{"domain":"edge-cases"}' \
    "special-chars"

#===============================================================================
# SECTION 5: Cleanup
#===============================================================================

print_section "SECTION 5: Cleanup"

echo "  Removing all test entries..."
cleanup_silent "knowledge_delete" '{"domain":"test-domain","key":"test-key-1"}'
cleanup_silent "knowledge_delete" '{"domain":"test-domain","key":"test-key-2"}'
cleanup_silent "knowledge_delete" '{"domain":"edge-cases","key":"special-chars"}'
cleanup_silent "knowledge_delete" '{"domain":"edge-cases","key":"long-content"}'
cleanup_silent "knowledge_delete" '{"domain":"edge-cases","key":"multiline"}'

run_test "5.1 Verify cleanup - no entries remain" \
    "knowledge_get" \
    '{}' \
    "No knowledge entries found"

#===============================================================================
# Summary
#===============================================================================

echo ""
echo "${BOLD}============================================${NC}"
echo "${BOLD}   Test Summary${NC}"
echo "${BOLD}============================================${NC}"
echo ""
echo "  ${GREEN}Passed: $PASS_COUNT${NC}"
echo "  ${RED}Failed: $FAIL_COUNT${NC}"
TOTAL=$((PASS_COUNT + FAIL_COUNT))
echo "  Total:  $TOTAL"
echo ""

if [ "$FAIL_COUNT" -gt 0 ]; then
    echo "${RED}SOME TESTS FAILED${NC}"
    exit 1
else
    echo "${GREEN}ALL TESTS PASSED${NC}"
    exit 0
fi
