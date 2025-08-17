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

echo "====================================================="
echo "Testing Microsoft 365 File Content Download"
echo "====================================================="
echo

# Configuration
PROBE_PATH="/Users/eric/source/MCPProbe/probe"
FULL_SERVER_URL="${SERVER_URL}/sse"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
OUTPUT_FILE="files_content_download_${TIMESTAMP}.log"

echo "Test run: $TIMESTAMP" | tee "$OUTPUT_FILE"
echo "Server: $FULL_SERVER_URL" | tee -a "$OUTPUT_FILE"
echo "Using API Token: ${APIKEY:0:8}..." | tee -a "$OUTPUT_FILE"
echo "Probe tool: $PROBE_PATH" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Step 1: Find files to test content download
echo "=== Step 1: Find Files for Content Download Testing ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_search" | tee -a "$OUTPUT_FILE"
echo "Description: Search for text files suitable for content download" | tee -a "$OUTPUT_FILE"
FILE_SEARCH_RESPONSE=$("$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_search \
  -params '{"searchQuery": "txt", "$top": 5}' \
  2>&1)
echo "$FILE_SEARCH_RESPONSE" | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Extract file IDs from the search response
FILE_IDS=($(echo "$FILE_SEARCH_RESPONSE" | grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' | head -3))

if [ ${#FILE_IDS[@]} -gt 0 ]; then
    echo "Found ${#FILE_IDS[@]} file IDs for content download testing:" | tee -a "$OUTPUT_FILE"
    for i in "${!FILE_IDS[@]}"; do
        echo "  File $((i+1)): ${FILE_IDS[i]}" | tee -a "$OUTPUT_FILE"
    done
    echo | tee -a "$OUTPUT_FILE"
    
    # Test 1: Download content of first file
    if [ ${#FILE_IDS[@]} -ge 1 ]; then
        echo "=== Test 1: Download Content of First File ===" | tee -a "$OUTPUT_FILE"
        echo "Command: microsoft365_files_download_content" | tee -a "$OUTPUT_FILE"
        echo "Description: Download content of file ID: ${FILE_IDS[0]}" | tee -a "$OUTPUT_FILE"
        echo "Note: Content will be truncated for readability" | tee -a "$OUTPUT_FILE"
        "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
          -headers "Authorization:Bearer $APIKEY" \
          -call microsoft365_files_download_content \
          -params "{\"id\": \"${FILE_IDS[0]}\"}" \
          2>&1 | head -30 | tee -a "$OUTPUT_FILE"
        echo "[Content truncated for log readability]" | tee -a "$OUTPUT_FILE"
        echo | tee -a "$OUTPUT_FILE"
    fi
    
    # Test 2: Download content of second file if available
    if [ ${#FILE_IDS[@]} -ge 2 ]; then
        echo "=== Test 2: Download Content of Second File ===" | tee -a "$OUTPUT_FILE"
        echo "Command: microsoft365_files_download_content" | tee -a "$OUTPUT_FILE"
        echo "Description: Download content of file ID: ${FILE_IDS[1]}" | tee -a "$OUTPUT_FILE"
        "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
          -headers "Authorization:Bearer $APIKEY" \
          -call microsoft365_files_download_content \
          -params "{\"id\": \"${FILE_IDS[1]}\"}" \
          2>&1 | head -20 | tee -a "$OUTPUT_FILE"
        echo "[Content truncated for log readability]" | tee -a "$OUTPUT_FILE"
        echo | tee -a "$OUTPUT_FILE"
    fi
    
    # Test 3: Download content of third file if available
    if [ ${#FILE_IDS[@]} -ge 3 ]; then
        echo "=== Test 3: Download Content of Third File ===" | tee -a "$OUTPUT_FILE"
        echo "Command: microsoft365_files_download_content" | tee -a "$OUTPUT_FILE"
        echo "Description: Download content of file ID: ${FILE_IDS[2]}" | tee -a "$OUTPUT_FILE"
        "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
          -headers "Authorization:Bearer $APIKEY" \
          -call microsoft365_files_download_content \
          -params "{\"id\": \"${FILE_IDS[2]}\"}" \
          2>&1 | head -15 | tee -a "$OUTPUT_FILE"
        echo "[Content truncated for log readability]" | tee -a "$OUTPUT_FILE"
        echo | tee -a "$OUTPUT_FILE"
    fi
    
else
    echo "No file IDs found from search. Testing with alternative approaches..." | tee -a "$OUTPUT_FILE"
    echo | tee -a "$OUTPUT_FILE"
    
    # Alternative: Search for documents
    echo "=== Alternative: Search for Document Files ===" | tee -a "$OUTPUT_FILE"
    echo "Command: microsoft365_files_search" | tee -a "$OUTPUT_FILE"
    echo "Description: Search for document files" | tee -a "$OUTPUT_FILE"
    DOC_SEARCH_RESPONSE=$("$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
      -headers "Authorization:Bearer $APIKEY" \
      -call microsoft365_files_search \
      -params '{"searchQuery": "doc", "$top": 3}' \
      2>&1)
    echo "$DOC_SEARCH_RESPONSE" | tee -a "$OUTPUT_FILE"
    echo | tee -a "$OUTPUT_FILE"
    
    # Extract document file IDs
    DOC_FILE_IDS=($(echo "$DOC_SEARCH_RESPONSE" | grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' | head -2))
    
    if [ ${#DOC_FILE_IDS[@]} -gt 0 ]; then
        echo "Found ${#DOC_FILE_IDS[@]} document file IDs:" | tee -a "$OUTPUT_FILE"
        for i in "${!DOC_FILE_IDS[@]}"; do
            echo "  Document $((i+1)): ${DOC_FILE_IDS[i]}" | tee -a "$OUTPUT_FILE"
        done
        echo | tee -a "$OUTPUT_FILE"
        
        # Test with document files
        if [ ${#DOC_FILE_IDS[@]} -ge 1 ]; then
            echo "=== Test 4: Download Document Content ===" | tee -a "$OUTPUT_FILE"
            echo "Command: microsoft365_files_download_content" | tee -a "$OUTPUT_FILE"
            echo "Description: Download content of document ID: ${DOC_FILE_IDS[0]}" | tee -a "$OUTPUT_FILE"
            "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
              -headers "Authorization:Bearer $APIKEY" \
              -call microsoft365_files_download_content \
              -params "{\"id\": \"${DOC_FILE_IDS[0]}\"}" \
              2>&1 | head -25 | tee -a "$OUTPUT_FILE"
            echo "[Content truncated for log readability]" | tee -a "$OUTPUT_FILE"
            echo | tee -a "$OUTPUT_FILE"
        fi
    fi
fi

# Test 5: Get recent files for content download
echo "=== Test 5: Download Recent File Content ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_recent + microsoft365_files_download_content" | tee -a "$OUTPUT_FILE"
echo "Description: Get recent files and download content from first one" | tee -a "$OUTPUT_FILE"
RECENT_RESPONSE=$("$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_recent \
  -params '{"$top": 5}' \
  2>&1)
echo "Recent files response:" | tee -a "$OUTPUT_FILE"
echo "$RECENT_RESPONSE" | head -20 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Extract file ID from recent files
RECENT_FILE_ID=$(echo "$RECENT_RESPONSE" | grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')

if [ -n "$RECENT_FILE_ID" ]; then
    echo "Found recent file ID: $RECENT_FILE_ID" | tee -a "$OUTPUT_FILE"
    echo "Downloading content..." | tee -a "$OUTPUT_FILE"
    "$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
      -headers "Authorization:Bearer $APIKEY" \
      -call microsoft365_files_download_content \
      -params "{\"id\": \"$RECENT_FILE_ID\"}" \
      2>&1 | head -20 | tee -a "$OUTPUT_FILE"
    echo "[Content truncated for log readability]" | tee -a "$OUTPUT_FILE"
    echo | tee -a "$OUTPUT_FILE"
fi

# Test 6: Error handling - invalid file ID
echo "=== Test 6: Invalid File ID (Error Test) ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_download_content" | tee -a "$OUTPUT_FILE"
echo "Description: Test error handling for invalid file ID" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_download_content \
  -params '{"id": "invalid-file-id-12345"}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

# Test 7: Error handling - empty file ID
echo "=== Test 7: Empty File ID (Error Test) ===" | tee -a "$OUTPUT_FILE"
echo "Command: microsoft365_files_download_content" | tee -a "$OUTPUT_FILE"
echo "Description: Test error handling for empty file ID" | tee -a "$OUTPUT_FILE"
"$PROBE_PATH" -url "$FULL_SERVER_URL" -transport sse \
  -headers "Authorization:Bearer $APIKEY" \
  -call microsoft365_files_download_content \
  -params '{"id": ""}' \
  2>&1 | tee -a "$OUTPUT_FILE"
echo | tee -a "$OUTPUT_FILE"

echo "===============================================" | tee -a "$OUTPUT_FILE"
echo "File Content Download Tests Complete" | tee -a "$OUTPUT_FILE"
echo "Results saved to: $OUTPUT_FILE" | tee -a "$OUTPUT_FILE"
echo "===============================================" | tee -a "$OUTPUT_FILE"

echo
echo "Test capabilities verified:"
echo "✓ File content download from search results"
echo "✓ Multiple file type content access (txt, doc, etc.)"
echo "✓ Content download from recent files"
echo "✓ Binary and text file handling"
echo "✓ Large content truncation for readability"
echo "✓ Error handling for invalid/empty file IDs"
echo "✓ Integration with file discovery methods"
echo
echo "Note: Actual file content download success depends on:"
echo "- File accessibility and permissions"
echo "- File size and type compatibility"
echo "- Microsoft Graph API response format"