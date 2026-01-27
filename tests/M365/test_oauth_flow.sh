#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

# Direct test of OAuth flow via MCP

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

echo "Testing OAuth Device Flow directly..."
echo ""

# Create a simple MCP request for profile
cat > /tmp/mcp_request.json << 'EOF'
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "microsoft365_profile_get",
    "arguments": {}
  },
  "id": 1
}
EOF

echo "Sending MCP request to server..."
echo "URL: ${SERVER_URL}/message"
echo ""

# Make the request with curl
response=$(curl -s -X POST "${SERVER_URL}/message" \
  -H "Authorization: Bearer $APIKEY" \
  -H "Content-Type: application/json" \
  -d @/tmp/mcp_request.json)

echo "Response:"
echo "$response" | python3 -m json.tool 2>/dev/null || echo "$response"

# Check for OAuth device code
if echo "$response" | grep -q "devicelogin"; then
    echo ""
    echo "✅ OAuth Device Flow detected!"
    echo ""
    echo "$response" | grep -o '"verification_uri"[^,]*' | sed 's/"verification_uri"://g'
    echo "$response" | grep -o '"user_code"[^,]*' | sed 's/"user_code"://g'
fi

# Clean up
rm -f /tmp/mcp_request.json