#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

# Object Array Parameters Regression Test - PwnDoc
#
# This test specifically covers the bug fix where array parameters containing
# objects (not strings) were being silently dropped. It verifies that object
# arrays such as customFields are correctly passed through to the API.
#
# Fixed test IDs for the MCPFusion Integration Test audit:
#   Audit ID:             696c3f8051d8f95c85499d33  (MCPFusion Integration Test)
#   SQL Injection finding: 696c3f9151d8f95c85499d3d
#   XSS finding:          696c3f9151d8f95c85499d4d
#   OWASP custom field ID: 64b23406fc882b3cbf9eb035

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
PROBE_PATH="${PROBE_PATH:-/home/eric/bin/probe}"

# Append /sse to the base URL
FULL_SERVER_URL="${SERVER_URL}/sse"

echo "=== PwnDoc Object Array Parameters Regression Test ==="
echo "Timestamp: $(date)"
echo "Server: $FULL_SERVER_URL"
echo "Using API Token: ${APIKEY:0:8}..."
echo ""

# Test 1: Update SQL Injection finding with customFields as object array
echo "Test 1: Update SQL Injection finding with customFields as object array"
echo "Command: pwndoc_update_finding"
echo "Parameters: {\"audit_id\": \"696c3f8051d8f95c85499d33\", \"finding_id\": \"696c3f9151d8f95c85499d3d\", \"customFields\": [{\"customField\": \"64b23406fc882b3cbf9eb035\", \"text\": \"A03:2021-Injection\"}]}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call pwndoc_update_finding -params '{"audit_id": "696c3f8051d8f95c85499d33", "finding_id": "696c3f9151d8f95c85499d3d", "customFields": [{"customField": "64b23406fc882b3cbf9eb035", "text": "A03:2021-Injection"}]}'

echo ""
echo "=========================================="
echo ""

# Test 2: Get the SQL Injection finding and verify customFields were saved
echo "Test 2: Get SQL Injection finding to verify customFields were saved (not silently dropped)"
echo "Command: pwndoc_get_finding"
echo "Parameters: {\"audit_id\": \"696c3f8051d8f95c85499d33\", \"finding_id\": \"696c3f9151d8f95c85499d3d\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call pwndoc_get_finding -params '{"audit_id": "696c3f8051d8f95c85499d33", "finding_id": "696c3f9151d8f95c85499d3d"}'
echo ""
echo "Verify customFields array is present in the response above (not empty)"

echo ""
echo "=========================================="
echo ""

# Test 3: Clear customFields on SQL Injection finding (cleanup)
echo "Test 3: Clear customFields on SQL Injection finding (cleanup)"
echo "Command: pwndoc_update_finding"
echo "Parameters: {\"audit_id\": \"696c3f8051d8f95c85499d33\", \"finding_id\": \"696c3f9151d8f95c85499d3d\", \"customFields\": []}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call pwndoc_update_finding -params '{"audit_id": "696c3f8051d8f95c85499d33", "finding_id": "696c3f9151d8f95c85499d3d", "customFields": []}'

echo ""
echo "=========================================="
echo ""

# Test 4: Update XSS finding with customFields as object array
echo "Test 4: Update XSS finding with customFields as object array"
echo "Command: pwndoc_update_finding"
echo "Parameters: {\"audit_id\": \"696c3f8051d8f95c85499d33\", \"finding_id\": \"696c3f9151d8f95c85499d4d\", \"customFields\": [{\"customField\": \"64b23406fc882b3cbf9eb035\", \"text\": \"A03:2021-Injection\"}]}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call pwndoc_update_finding -params '{"audit_id": "696c3f8051d8f95c85499d33", "finding_id": "696c3f9151d8f95c85499d4d", "customFields": [{"customField": "64b23406fc882b3cbf9eb035", "text": "A03:2021-Injection"}]}'

echo ""
echo "=========================================="
echo ""

# Test 5: Get the XSS finding and verify customFields were saved
echo "Test 5: Get XSS finding to verify customFields were saved (not silently dropped)"
echo "Command: pwndoc_get_finding"
echo "Parameters: {\"audit_id\": \"696c3f8051d8f95c85499d33\", \"finding_id\": \"696c3f9151d8f95c85499d4d\"}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call pwndoc_get_finding -params '{"audit_id": "696c3f8051d8f95c85499d33", "finding_id": "696c3f9151d8f95c85499d4d"}'
echo ""
echo "Verify customFields array is present in the response above"

echo ""
echo "=========================================="
echo ""

# Test 6: Clear customFields on XSS finding (cleanup)
echo "Test 6: Clear customFields on XSS finding (cleanup)"
echo "Command: pwndoc_update_finding"
echo "Parameters: {\"audit_id\": \"696c3f8051d8f95c85499d33\", \"finding_id\": \"696c3f9151d8f95c85499d4d\", \"customFields\": []}"
echo ""
$PROBE_PATH -url "$FULL_SERVER_URL" -transport sse -headers "Authorization:Bearer $APIKEY" -call pwndoc_update_finding -params '{"audit_id": "696c3f8051d8f95c85499d33", "finding_id": "696c3f9151d8f95c85499d4d", "customFields": []}'

echo ""
echo "=========================================="
echo ""

echo "=== PwnDoc Object Array Parameters Regression Test Complete ==="
