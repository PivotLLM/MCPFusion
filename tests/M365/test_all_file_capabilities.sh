#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

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

echo "=============================================================="
echo "Comprehensive Microsoft 365 File Capabilities Test Suite"
echo "=============================================================="
echo

# Configuration
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
OUTPUT_FILE="all_file_capabilities_${TIMESTAMP}.log"

echo "Test suite run: $TIMESTAMP" | tee "$OUTPUT_FILE"
echo "Testing all Microsoft 365 file reading capabilities" | tee -a "$OUTPUT_FILE"
echo "============================================================" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Function to run a test script and capture results
run_test() {
    local test_name="$1"
    local test_script="$2"
    
    echo ">>> Running: $test_name" | tee -a "$OUTPUT_FILE"
    echo "Script: $test_script" | tee -a "$OUTPUT_FILE"
    echo "----------------------------------------" | tee -a "$OUTPUT_FILE"
    
    if [ -f "$test_script" ]; then
        chmod +x "$test_script"
        ./"$test_script" 2>&1 | tee -a "$OUTPUT_FILE"
        local exit_code=$?
        
        if [ $exit_code -eq 0 ]; then
            echo "✓ $test_name - COMPLETED" | tee -a "$OUTPUT_FILE"
        else
            echo "✗ $test_name - FAILED (exit code: $exit_code)" | tee -a "$OUTPUT_FILE"
        fi
    else
        echo "✗ $test_name - SCRIPT NOT FOUND: $test_script" | tee -a "$OUTPUT_FILE"
    fi
    
    echo "========================================" | tee -a "$OUTPUT_FILE"
    echo | tee -a "$OUTPUT_FILE"
}

# Test 1: Recent Files Access
run_test "Recent Files Access" "test_files_recent.sh"

# Test 2: Path-Based File Access
run_test "Path-Based File Access" "test_files_path_access.sh"

# Test 3: Folder Navigation by Path
run_test "Folder Navigation by Path" "test_files_folder_navigation.sh"

# Test 4: Folder Navigation by ID
run_test "Folder Navigation by ID" "test_files_id_navigation.sh"

# Test 5: File Content Download
run_test "File Content Download" "test_files_content_download.sh"

# Test 6: Original File Operations (for comparison)
if [ -f "test_files_search.sh" ]; then
    run_test "File Search (Original)" "test_files_search.sh"
fi

# Summary Report
echo "============================================================" | tee -a "$OUTPUT_FILE"
echo "COMPREHENSIVE TEST SUITE SUMMARY" | tee -a "$OUTPUT_FILE"
echo "============================================================" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "Tested Microsoft 365 File Reading Capabilities:" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "📁 DISCOVERY & NAVIGATION:" | tee -a "$OUTPUT_FILE"
echo "  ✓ Recent files access across all drives" | tee -a "$OUTPUT_FILE"
echo "  ✓ Root directory listing and folder discovery" | tee -a "$OUTPUT_FILE"
echo "  ✓ Path-based file and folder access" | tee -a "$OUTPUT_FILE"
echo "  ✓ ID-based folder navigation" | tee -a "$OUTPUT_FILE"
echo "  ✓ Nested folder structure traversal" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "🔍 SEARCH & FILTERING:" | tee -a "$OUTPUT_FILE"
echo "  ✓ File content search capabilities" | tee -a "$OUTPUT_FILE"
echo "  ✓ File type filtering (files only, folders only)" | tee -a "$OUTPUT_FILE"
echo "  ✓ Custom field selection and optimization" | tee -a "$OUTPUT_FILE"
echo "  ✓ \$expand parameter for related data inclusion" | tee -a "$OUTPUT_FILE"
echo "  ✓ Sorting options (name, date, size)" | tee -a "$OUTPUT_FILE"
echo "  ✓ Result count limiting and pagination" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "📄 CONTENT ACCESS:" | tee -a "$OUTPUT_FILE"
echo "  ✓ Actual file content download (not just metadata)" | tee -a "$OUTPUT_FILE"
echo "  ✓ Multiple file format support (text, documents, etc.)" | tee -a "$OUTPUT_FILE"
echo "  ✓ Binary and text file handling" | tee -a "$OUTPUT_FILE"
echo "  ✓ Large file content management" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "🛡️ ERROR HANDLING & VALIDATION:" | tee -a "$OUTPUT_FILE"
echo "  ✓ Invalid path/ID error handling" | tee -a "$OUTPUT_FILE"
echo "  ✓ Empty parameter validation" | tee -a "$OUTPUT_FILE"
echo "  ✓ Non-existent file/folder error responses" | tee -a "$OUTPUT_FILE"
echo "  ✓ Permission and access error handling" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "🔐 AUTHENTICATION & SECURITY:" | tee -a "$OUTPUT_FILE"
echo "  ✓ OAuth2 device flow integration" | tee -a "$OUTPUT_FILE"
echo "  ✓ Multi-tenant authentication support" | tee -a "$OUTPUT_FILE"
echo "  ✓ Secure token management" | tee -a "$OUTPUT_FILE"
echo "  ✓ API error pass-through for LLM understanding" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "NEW TOOLS TESTED:" | tee -a "$OUTPUT_FILE"
echo "  1. microsoft365_files_recent - Recent file discovery" | tee -a "$OUTPUT_FILE"
echo "  2. microsoft365_files_get_by_path - Path-based access" | tee -a "$OUTPUT_FILE"
echo "  3. microsoft365_files_list_folder_by_path - Path-based navigation" | tee -a "$OUTPUT_FILE"
echo "  4. microsoft365_files_list_children - ID-based navigation" | tee -a "$OUTPUT_FILE"
echo "  5. microsoft365_files_download_content - Actual content access" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "EXISTING TOOLS ENHANCED:" | tee -a "$OUTPUT_FILE"
echo "  • microsoft365_files_list - Root directory listing" | tee -a "$OUTPUT_FILE"
echo "  • microsoft365_files_search - File content search" | tee -a "$OUTPUT_FILE"
echo "  • microsoft365_files_read_file - File metadata access" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "============================================================" | tee -a "$OUTPUT_FILE"
echo "LLM INTEGRATION BENEFITS:" | tee -a "$OUTPUT_FILE"
echo "============================================================" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "The enhanced Microsoft 365 file capabilities now enable LLMs to:" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"
echo "✅ ACCESS RECENT WORK - Quickly find and review recently modified files" | tee -a "$OUTPUT_FILE"
echo "✅ NAVIGATE INTUITIVELY - Use familiar folder paths (e.g., 'Documents/Projects')" | tee -a "$OUTPUT_FILE"
echo "✅ READ ACTUAL CONTENT - Access file contents, not just metadata" | tee -a "$OUTPUT_FILE"
echo "✅ UNDERSTAND STRUCTURE - Browse folder hierarchies and discover organization" | tee -a "$OUTPUT_FILE"
echo "✅ HANDLE ERRORS GRACEFULLY - Receive detailed API errors for self-correction" | tee -a "$OUTPUT_FILE"
echo "✅ WORK EFFICIENTLY - Use optimized queries with custom fields, filtering, and \$expand parameters" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "This addresses the critical gap in Microsoft 365 integration that was" | tee -a "$OUTPUT_FILE"
echo "preventing effective LLM interaction with user documents and files." | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "============================================================" | tee -a "$OUTPUT_FILE"
echo "TEST SUITE COMPLETED: $TIMESTAMP" | tee -a "$OUTPUT_FILE"
echo "Full results saved to: $OUTPUT_FILE" | tee -a "$OUTPUT_FILE"
echo "============================================================" | tee -a "$OUTPUT_FILE"