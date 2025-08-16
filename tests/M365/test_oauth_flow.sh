#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025 Tenebris Technologies Inc.                                *
# All rights reserved.                                                         *
#*******************************************************************************

# Direct test of OAuth flow via MCP

source .env

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
echo "URL: http://127.0.0.1:8888/message"
echo ""

# Make the request with curl
response=$(curl -s -X POST http://127.0.0.1:8888/message \
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