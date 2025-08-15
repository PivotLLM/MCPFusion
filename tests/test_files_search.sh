#!/bin/bash

# Test script for Files Search functionality
# Tests various search and filtering capabilities for OneDrive

echo "==============================================="
echo "Testing Microsoft 365 Files Search"
echo "==============================================="
echo

# Configuration
PROBE_PATH="/Users/eric/source/MCPProbe/probe"
SERVER_URL="http://localhost:8888"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
OUTPUT_FILE="files_search_${TIMESTAMP}.log"

echo "Test run: $TIMESTAMP" | tee "$OUTPUT_FILE"
echo "Server: $SERVER_URL" | tee -a "$OUTPUT_FILE"
echo "Probe tool: $PROBE_PATH" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 1: Search for files with 'invoice' in name
echo "=== Test 1: Search for files containing 'invoice' ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" call microsoft365_files_search \
  --server "$SERVER_URL" \
  --searchQuery "invoice" \
  --\$top 10 \
  | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 2: Search for PDF files
echo "=== Test 2: Search for PDF files ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" call microsoft365_files_search \
  --server "$SERVER_URL" \
  --searchQuery "*.pdf" \
  --\$top 10 \
  | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 3: Search for report files
echo "=== Test 3: Search for report files from 2025 ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" call microsoft365_files_search \
  --server "$SERVER_URL" \
  --searchQuery "report 2025" \
  --\$select "name,size,lastModifiedDateTime" \
  --\$top 10 \
  | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 4: Search with PDF filter
echo "=== Test 4: Search files with PDF MIME type filter ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" call microsoft365_files_search \
  --server "$SERVER_URL" \
  --searchQuery "*" \
  --\$filter "file/mimeType eq 'application/pdf'" \
  --\$top 5 \
  | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 5: Search for recent files
echo "=== Test 5: Search for files modified after Jan 1, 2025 ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" call microsoft365_files_search \
  --server "$SERVER_URL" \
  --searchQuery "*" \
  --\$filter "lastModifiedDateTime ge 2025-01-01T00:00:00Z" \
  --\$orderby "lastModifiedDateTime desc" \
  --\$top 10 \
  | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 6: Search for large files
echo "=== Test 6: Search for files larger than 1MB ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" call microsoft365_files_search \
  --server "$SERVER_URL" \
  --searchQuery "*" \
  --\$filter "size gt 1048576" \
  --\$select "name,size,lastModifiedDateTime" \
  --\$orderby "size desc" \
  --\$top 10 \
  | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 7: Search for presentation files
echo "=== Test 7: Search for presentation files ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" call microsoft365_files_search \
  --server "$SERVER_URL" \
  --searchQuery "presentation" \
  --\$top 10 \
  | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 8: Search for Word documents
echo "=== Test 8: Search for Word documents ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" call microsoft365_files_search \
  --server "$SERVER_URL" \
  --searchQuery "*.docx" \
  --\$orderby "name asc" \
  --\$top 10 \
  | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 9: Search for budget-related files
echo "=== Test 9: Search for budget-related files ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" call microsoft365_files_search \
  --server "$SERVER_URL" \
  --searchQuery "budget" \
  --\$select "name,size,webUrl" \
  --\$top 5 \
  | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 10: Search for files only (no folders)
echo "=== Test 10: Search files only (excluding folders) ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" call microsoft365_files_search \
  --server "$SERVER_URL" \
  --searchQuery "*" \
  --\$filter "file ne null" \
  --\$select "name,file,size" \
  --\$top 15 \
  | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 11: List all files in root directory
echo "=== Test 11: List all files in root directory ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" call microsoft365_files_list \
  --server "$SERVER_URL" \
  --\$top 20 \
  | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 12: List only folders in root directory
echo "=== Test 12: List only folders in root directory ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" call microsoft365_files_list \
  --server "$SERVER_URL" \
  --\$filter "folder ne null" \
  --\$select "name,folder,childCount" \
  --\$orderby "name asc" \
  --\$top 10 \
  | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 13: List only files (no folders) in root directory
echo "=== Test 13: List only files in root directory ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" call microsoft365_files_list \
  --server "$SERVER_URL" \
  --\$filter "file ne null" \
  --\$select "name,file,size,lastModifiedDateTime" \
  --\$orderby "lastModifiedDateTime desc" \
  --\$top 10 \
  | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 14: List recent files only
echo "=== Test 14: List recent files in root directory ===" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" call microsoft365_files_list \
  --server "$SERVER_URL" \
  --\$filter "lastModifiedDateTime ge 2025-01-01T00:00:00Z" \
  --\$orderby "lastModifiedDateTime desc" \
  --\$top 10 \
  | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "==============================================="
echo "Files Search tests completed."
echo "Results saved to: $OUTPUT_FILE"
echo "==============================================="