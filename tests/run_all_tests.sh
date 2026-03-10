#!/bin/bash

################################################################################
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                          #
# Please see LICENSE file for details.                                         #
################################################################################

# Run all MCP function tests.
#
# REQUIREMENTS:
#   - A running MCPFusion server (default port 8888)
#   - APIKEY environment variable set to a valid API token, or edit each
#     test script to set APIKEY directly.
#   - The 'probe' binary available in PATH.
#
# Each test script produces a timestamped .log file with the full
# request/response data.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

PASS=0
FAIL=0

run_test() {
    local script="$1"
    echo ""
    echo "Running: $script"
    if bash "$SCRIPT_DIR/$script"; then
        PASS=$((PASS + 1))
    else
        FAIL=$((FAIL + 1))
    fi
}

run_test test_health.sh
run_test test_knowledge.sh
run_test test_knowledge_key.sh
run_test test_stdio.sh

echo ""
echo "========================================"
echo "   All Tests Summary"
echo "========================================"
echo "  Passed: $PASS"
echo "  Failed: $FAIL"
echo "  Total:  $((PASS + FAIL))"
echo ""

if [ "$FAIL" -gt 0 ]; then
    echo "SOME TEST SUITES FAILED"
    exit 1
else
    echo "ALL TEST SUITES PASSED"
    exit 0
fi
