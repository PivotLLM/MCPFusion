#!/bin/bash
################################################################################
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                          #
# Please see LICENSE file for details.                                         #
################################################################################

# MCPFusion Regression Test Suite
#
# Runs the full Go unit-test suite under the race detector, then optionally
# runs the live MCP integration tests (requires a running server + credentials).
#
# Usage:
#   ./test.sh          Full regression suite (race detector on, recommended)
#   ./test.sh -f       Fast mode — disables race detector (local iteration only)
#   ./test.sh -i       Also run MCP integration tests (requires running server)
#   ./test.sh -x       Preserve test artifacts on completion
#   ./test.sh -n       No-color output (also honoured via NO_COLOR env var)
#
# Exit codes:
#   0   All tests passed
#   1   One or more tests failed
#
# Test phases:
#   0.x  Pre-flight: build verification
#   1.x  Unit tests (go test -race -count=1 ./...)
#   2.x  Integration tests (optional, -i flag)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

################################################################################
# Flags
################################################################################

FAST_MODE=false
INTEGRATION=false
PRESERVE_ARTIFACTS=false
NO_COLOR_FLAG=false

while getopts "finx" opt; do
    case $opt in
        f) FAST_MODE=true ;;
        i) INTEGRATION=true ;;
        n) NO_COLOR_FLAG=true ;;
        x) PRESERVE_ARTIFACTS=true ;;
        *)
            echo "Usage: $0 [-f] [-i] [-n] [-x]"
            echo "  -f  Fast mode (no race detector)"
            echo "  -i  Run integration tests (requires running server + credentials)"
            echo "  -n  No-color output"
            echo "  -x  Preserve test artifacts after completion"
            exit 1
            ;;
    esac
done

################################################################################
# Colors — disabled when stdout is not a terminal, NO_COLOR is set, or -n used
################################################################################

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "$NO_COLOR_FLAG" = false ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    CYAN='\033[0;36m'
    BOLD='\033[1m'
    NC='\033[0m'
else
    RED='' GREEN='' YELLOW='' BLUE='' CYAN='' BOLD='' NC=''
fi

################################################################################
# Counters
################################################################################

PKG_PASS=0
PKG_FAIL=0
PKG_SKIP=0   # packages with no test files

################################################################################
# Helpers
################################################################################

print_section() {
    echo ""
    echo "${BOLD}${BLUE}============================================${NC}"
    echo "${BOLD}${BLUE}   $1${NC}"
    echo "${BOLD}${BLUE}============================================${NC}"
    echo ""
}

pass() { echo "  ${GREEN}PASS${NC}  $1"; PKG_PASS=$((PKG_PASS + 1)); }
fail() { echo "  ${RED}FAIL${NC}  $1"; PKG_FAIL=$((PKG_FAIL + 1)); }
skip() { echo "  ${CYAN}SKIP${NC}  $1"; PKG_SKIP=$((PKG_SKIP + 1)); }
warn() { echo "  ${YELLOW}WARN${NC}  $1"; }

################################################################################
# Header
################################################################################

echo ""
echo "${BOLD}============================================${NC}"
echo "${BOLD}   MCPFusion Regression Test Suite${NC}"
echo "${BOLD}============================================${NC}"
echo ""

if [ "$FAST_MODE" = true ]; then
    echo "${YELLOW}WARNING: Fast mode enabled — race detector is OFF.${NC}"
    echo "${YELLOW}         This run is NOT suitable for CI or pre-commit verification.${NC}"
    echo ""
fi

################################################################################
# Phase 0: Pre-flight
################################################################################

print_section "0.x  Pre-flight"

echo "  Verifying Go toolchain..."
if ! command -v go &>/dev/null; then
    echo "${RED}ERROR: 'go' not found in PATH${NC}"
    exit 1
fi
GO_VERSION=$(go version)
echo "  ${GO_VERSION}"
echo ""

echo "  0.1  Build check (go build ./...)"
if go build ./... 2>&1; then
    pass "Build succeeded"
else
    fail "Build failed — fix compilation errors before running tests"
    echo ""
    echo "${RED}Build failed. Aborting.${NC}"
    exit 1
fi

echo ""
echo "  0.2  Vet check (go vet ./...)"
if go vet ./... 2>&1; then
    pass "go vet clean"
else
    fail "go vet reported issues"
fi

################################################################################
# Phase 1: Unit tests
################################################################################

print_section "1.x  Unit Tests"

if [ "$FAST_MODE" = true ]; then
    GO_TEST_FLAGS="-count=1 -timeout 180s"
    echo "  Running: go test -count=1 -timeout 180s ./..."
else
    GO_TEST_FLAGS="-race -count=1 -timeout 180s"
    echo "  Running: go test -race -count=1 -timeout 180s ./..."
fi
echo ""

# Capture output and also stream it
TMPOUT=$(mktemp)
trap 'rm -f "$TMPOUT"' EXIT

set +e
go test $GO_TEST_FLAGS ./... 2>&1 | tee "$TMPOUT"
GO_TEST_EXIT=${PIPESTATUS[0]}
set -e

echo ""

# Parse per-package results from go test output
while IFS= read -r line; do
    if [[ "$line" =~ ^ok[[:space:]] ]]; then
        pkg=$(echo "$line" | awk '{print $2}')
        pass "$pkg"
    elif [[ "$line" =~ ^FAIL[[:space:]] ]]; then
        pkg=$(echo "$line" | awk '{print $2}')
        fail "$pkg"
    elif [[ "$line" =~ ^\?[[:space:]] ]]; then
        pkg=$(echo "$line" | awk '{print $2}')
        skip "$pkg  [no test files]"
    fi
done < "$TMPOUT"

################################################################################
# Phase 2: Integration tests (optional)
################################################################################

if [ "$INTEGRATION" = true ]; then
    print_section "2.x  Integration Tests"

    TESTS_DIR="$SCRIPT_DIR/tests"

    if [ ! -f "$TESTS_DIR/run_all_tests.sh" ]; then
        warn "tests/run_all_tests.sh not found — skipping integration tests"
    else
        echo "  Running: tests/run_all_tests.sh"
        echo "  (Requires a running MCPFusion server and APIKEY env var)"
        echo ""
        if bash "$TESTS_DIR/run_all_tests.sh"; then
            pass "Integration tests (tests/run_all_tests.sh)"
        else
            fail "Integration tests (tests/run_all_tests.sh)"
        fi
    fi
else
    echo ""
    echo "  ${CYAN}Integration tests skipped (pass -i to enable).${NC}"
    echo "  ${CYAN}Requires a running server and APIKEY env var.${NC}"
fi

################################################################################
# Summary
################################################################################

echo ""
echo "${BOLD}============================================${NC}"
echo "${BOLD}   TEST SUMMARY${NC}"
echo "${BOLD}============================================${NC}"
echo ""

TOTAL_TESTED=$((PKG_PASS + PKG_FAIL))

printf "  %-20s %d\n" "Total packages:" "$TOTAL_TESTED"
printf "  %-20s ${GREEN}%d${NC}\n" "Passed:" "$PKG_PASS"
printf "  %-20s ${RED}%d${NC}\n" "Failed:" "$PKG_FAIL"
printf "  %-20s ${CYAN}%d${NC}\n" "Skipped:" "$PKG_SKIP"

echo ""

if [ "$FAST_MODE" = true ]; then
    echo "${YELLOW}  WARNING: Race detector was disabled. Re-run without -f for a complete check.${NC}"
    echo ""
fi

if [ "$PKG_FAIL" -gt 0 ] || [ "$GO_TEST_EXIT" -ne 0 ]; then
    echo "${BOLD}${RED}  FAILURES DETECTED${NC}"
    echo ""
    exit 1
else
    echo "${BOLD}${GREEN}  All tests passed!${NC}"
    echo ""
    exit 0
fi
