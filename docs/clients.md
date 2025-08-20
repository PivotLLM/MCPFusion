# MCPFusion Client Configuration Guide

This guide explains how to configure various MCP clients to connect to MCPFusion servers.

## Table of Contents

- [Overview](#overview)
- [Cline IDE Integration](#cline-ide-integration)
- [Claude Desktop](#claude-desktop)
- [Custom MCP Clients](#custom-mcp-clients)
- [Connection Types](#connection-types)
- [Troubleshooting](#troubleshooting)

## Overview

MCPFusion serves as an MCP (Model Context Protocol) server that provides AI clients with access to external APIs through standardized tools. Clients connect to MCPFusion using either SSE (Server-Sent Events) or HTTP transport.

**Supported Transports:**
- **SSE (Server-Sent Events)**: Real-time bidirectional communication (recommended)
- **HTTP**: Simple request/response for basic integrations

**Default Configuration:**
- **Server Address**: `http://localhost:8888`
- **SSE Endpoint**: `http://localhost:8888/sse`
- **HTTP Endpoint**: `http://localhost:8888/http`

## Cline IDE Integration

[Cline](https://github.com/cline/cline) is a VS Code extension that provides AI-powered coding assistance with MCP support.

### Configuration File Location

Create or edit the Cline configuration file:

**macOS/Linux:**
```
~/.cline/config.json
```

**Windows:**
```
%APPDATA%\Cline\config.json
```

### Basic Configuration

```json
{
  "mcpServers": {
    "MCPFusion": {
      "disabled": false,
      "timeout": 3600,
      "url": "http://localhost:8888/sse",
      "transportType": "sse"
    }
  }
}
```

### Configuration Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `disabled` | boolean | No | Whether to disable this server (default: false) |
| `timeout` | number | No | Connection timeout in seconds (default: 30) |
| `url` | string | Yes | MCPFusion server URL |
| `transportType` | string | Yes | Transport protocol ("sse" or "http") |

### Advanced Configuration

```json
{
  "mcpServers": {
    "MCPFusion-Microsoft365": {
      "disabled": false,
      "timeout": 3600,
      "url": "http://localhost:8888/sse",
      "transportType": "sse",
      "description": "Microsoft 365 API integration via MCPFusion",
      "retryAttempts": 3,
      "retryDelay": 1000
    },
    "MCPFusion-Google": {
      "disabled": false,
      "timeout": 1800,
      "url": "http://localhost:8889/sse",
      "transportType": "sse",
      "description": "Google APIs integration via MCPFusion"
    }
  }
}
```

### Multiple Server Configuration

You can configure multiple MCPFusion instances for different API services:

```json
{
  "mcpServers": {
    "MCPFusion-Production": {
      "disabled": false,
      "timeout": 3600,
      "url": "http://localhost:8888/sse",
      "transportType": "sse",
      "description": "Production Microsoft 365 integration"
    },
    "MCPFusion-Development": {
      "disabled": true,
      "timeout": 1800,
      "url": "http://localhost:8889/sse", 
      "transportType": "sse",
      "description": "Development/testing environment"
    },
    "MCPFusion-Custom-API": {
      "disabled": false,
      "timeout": 2400,
      "url": "http://localhost:8890/sse",
      "transportType": "sse",
      "description": "Custom API integration"
    }
  }
}
```

### Cline Usage Tips

**1. Server Status**
- Check Cline's MCP status in VS Code status bar
- Green indicator = connected and working
- Red indicator = connection issues

**2. Available Tools**
- Use Cline's command palette to see available MCP tools
- Tools are prefixed with server name (e.g., "MCPFusion: microsoft365_calendar_read_summary")

**3. Debugging**
- Enable Cline debug logging to see MCP communication
- Check VS Code Developer Console for error messages

## Claude Desktop

Claude Desktop supports MCP servers through configuration files.

### Configuration File Location

**macOS:**
```
~/Library/Application Support/Claude/claude_desktop_config.json
```

**Windows:**
```
%APPDATA%\Claude\claude_desktop_config.json
```

**Linux:**
```
~/.config/Claude/claude_desktop_config.json
```

### Basic Configuration

```json
{
  "mcpServers": {
    "mcpfusion": {
      "command": "curl",
      "args": [
        "-X", "POST",
        "-H", "Content-Type: application/json",
        "-H", "Accept: text/event-stream",
        "http://localhost:8888/sse"
      ],
      "env": {
        "NODE_ENV": "production"
      }
    }
  }
}
```

### Alternative Configuration (if supporting stdio)

If MCPFusion adds stdio support in the future:

```json
{
  "mcpServers": {
    "mcpfusion": {
      "command": "/path/to/mcpfusion",
      "args": [
        "-config", "/path/to/config.json",
        "-transport", "stdio"
      ],
      "env": {
        "MS365_CLIENT_ID": "your-client-id",
        "MS365_TENANT_ID": "your-tenant-id"
      }
    }
  }
}
```

## Custom MCP Clients

### Direct SSE Connection

For custom clients using Server-Sent Events:

```javascript
// JavaScript example
const eventSource = new EventSource('http://localhost:8888/sse');

eventSource.onopen = function(event) {
    console.log('Connected to MCPFusion SSE');
};

eventSource.onmessage = function(event) {
    const data = JSON.parse(event.data);
    console.log('Received:', data);
};

eventSource.onerror = function(event) {
    console.error('SSE error:', event);
};

// Send MCP request
function sendMCPRequest(method, params) {
    const request = {
        jsonrpc: "2.0",
        id: Date.now(),
        method: method,
        params: params
    };
    
    fetch('http://localhost:8888/sse', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(request)
    });
}

// List available tools
sendMCPRequest('tools/list', {});

// Call a tool
sendMCPRequest('tools/call', {
    name: 'microsoft365_calendar_read_summary',
    arguments: {
        startDate: '20240101',
        endDate: '20240131'
    }
});
```

### Direct HTTP Connection

For simpler HTTP-based integration:

```python
# Python example
import requests
import json

class MCPFusionClient:
    def __init__(self, base_url="http://localhost:8888"):
        self.base_url = base_url
        self.session = requests.Session()
        self.request_id = 0
    
    def _make_request(self, method, params=None):
        self.request_id += 1
        payload = {
            "jsonrpc": "2.0",
            "id": self.request_id,
            "method": method,
            "params": params or {}
        }
        
        response = self.session.post(
            f"{self.base_url}/http",
            headers={"Content-Type": "application/json"},
            json=payload,
            timeout=30
        )
        response.raise_for_status()
        return response.json()
    
    def list_tools(self):
        """Get list of available tools"""
        return self._make_request("tools/list")
    
    def call_tool(self, tool_name, arguments):
        """Call a specific tool"""
        return self._make_request("tools/call", {
            "name": tool_name,
            "arguments": arguments
        })
    
    def list_resources(self):
        """Get list of available resources"""
        return self._make_request("resources/list")

# Usage example
client = MCPFusionClient()

# List available tools
tools = client.list_tools()
print("Available tools:", [tool['name'] for tool in tools.get('result', [])])

# Call Microsoft 365 calendar tool
calendar_result = client.call_tool(
    "microsoft365_calendar_read_summary",
    {
        "startDate": "20240101", 
        "endDate": "20240131"
    }
)
print("Calendar events:", calendar_result)
```

### Go Client Example

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type MCPRequest struct {
    JSONRPC string      `json:"jsonrpc"`
    ID      int         `json:"id"`
    Method  string      `json:"method"`
    Params  interface{} `json:"params,omitempty"`
}

type MCPResponse struct {
    JSONRPC string      `json:"jsonrpc"`
    ID      int         `json:"id"`
    Result  interface{} `json:"result,omitempty"`
    Error   interface{} `json:"error,omitempty"`
}

type MCPFusionClient struct {
    BaseURL    string
    HTTPClient *http.Client
    requestID  int
}

func NewMCPFusionClient(baseURL string) *MCPFusionClient {
    return &MCPFusionClient{
        BaseURL: baseURL,
        HTTPClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

func (c *MCPFusionClient) makeRequest(method string, params interface{}) (*MCPResponse, error) {
    c.requestID++
    
    request := MCPRequest{
        JSONRPC: "2.0",
        ID:      c.requestID,
        Method:  method,
        Params:  params,
    }
    
    jsonData, err := json.Marshal(request)
    if err != nil {
        return nil, err
    }
    
    resp, err := c.HTTPClient.Post(
        c.BaseURL+"/http",
        "application/json",
        bytes.NewBuffer(jsonData),
    )
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var mcpResp MCPResponse
    err = json.NewDecoder(resp.Body).Decode(&mcpResp)
    if err != nil {
        return nil, err
    }
    
    return &mcpResp, nil
}

func (c *MCPFusionClient) ListTools() (*MCPResponse, error) {
    return c.makeRequest("tools/list", nil)
}

func (c *MCPFusionClient) CallTool(name string, arguments map[string]interface{}) (*MCPResponse, error) {
    params := map[string]interface{}{
        "name":      name,
        "arguments": arguments,
    }
    return c.makeRequest("tools/call", params)
}

func main() {
    client := NewMCPFusionClient("http://localhost:8888")
    
    // List tools
    tools, err := client.ListTools()
    if err != nil {
        panic(err)
    }
    fmt.Printf("Tools response: %+v\n", tools)
    
    // Call calendar tool
    result, err := client.CallTool("microsoft365_calendar_read_summary", map[string]interface{}{
        "startDate": "20240101",
        "endDate":   "20240131",
    })
    if err != nil {
        panic(err)
    }
    fmt.Printf("Calendar result: %+v\n", result)
}
```

## Connection Types

### SSE (Server-Sent Events) - Recommended

**Advantages:**
- Real-time bidirectional communication
- Automatic reconnection handling
- Better for interactive applications
- Lower latency for multiple requests

**Best for:**
- IDE integrations (Cline, VS Code extensions)
- Interactive AI chat applications
- Real-time data processing

**Connection URL:** `http://localhost:8888/sse`

### HTTP - Simple Integration

**Advantages:**
- Simple request/response model
- Easy to implement in any language
- No persistent connections
- Good for scripting and batch operations

**Best for:**
- Command-line tools
- Batch processing scripts
- Simple integrations
- Testing and debugging

**Connection URL:** `http://localhost:8888/http`

## Troubleshooting

### Common Connection Issues

**1. Connection Refused**
```
Error: Connection refused to http://localhost:8888
```
**Solutions:**
- Check MCPFusion server is running
- Verify port 8888 is not blocked by firewall
- Try different port with `-port` flag

**2. Timeout Errors**
```
Error: Request timeout after 30s
```
**Solutions:**
- Increase timeout in client configuration
- Check network connectivity
- Verify server is responding (try direct HTTP request)

**3. Authentication Errors**
```
Error: Authentication failed for Microsoft 365
```
**Solutions:**
- Check environment variables are set (`~/.mcp` file)
- Verify OAuth2 client ID and tenant ID
- Ensure proper scopes are configured
- Try re-authenticating (delete cached tokens)

**4. Tool Not Found**
```
Error: Tool 'microsoft365_calendar_read_summary' not found
```
**Solutions:**
- Check server configuration includes the endpoint
- Verify service is properly configured
- Restart MCPFusion server
- Check logs for configuration errors

### Debug Commands

**1. Test Server Connectivity**
```bash
# Test basic connectivity
curl http://localhost:8888/health

# List available tools
curl -X POST http://localhost:8888/http \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}'
```

**2. Check Server Logs**
```bash
# Run server with debug logging
./mcpfusion -config configs/microsoft365.json -debug

# Check for authentication issues
grep -i "auth" mcp.log

# Check for configuration errors
grep -i "error" mcp.log
```

**3. Validate Client Configuration**
```bash
# Check Cline config syntax
cat ~/.cline/config.json | jq '.'

# Verify environment variables
env | grep -E "(MS365|CLIENT_ID|API_KEY)"
```

### Performance Optimization

**1. Connection Pooling**
For high-volume clients, use connection pooling:

```json
{
  "mcpServers": {
    "MCPFusion": {
      "timeout": 3600,
      "maxConnections": 10,
      "keepAlive": true
    }
  }
}
```

**2. Caching**
Enable response caching in MCPFusion configuration:

```json
{
  "response": {
    "caching": {
      "enabled": true,
      "ttl": "5m"
    }
  }
}
```

**3. Timeout Tuning**
Adjust timeouts based on API response times:

```json
{
  "mcpServers": {
    "MCPFusion": {
      "timeout": 1800,  // 30 minutes for long operations
      "url": "http://localhost:8888/sse"
    }
  }
}
```

### Client Configuration Templates

**Development Environment:**
```json
{
  "mcpServers": {
    "MCPFusion-Dev": {
      "disabled": false,
      "timeout": 300,
      "url": "http://localhost:8888/sse",
      "transportType": "sse",
      "debug": true
    }
  }
}
```

**Production Environment:**
```json
{
  "mcpServers": {
    "MCPFusion-Prod": {
      "disabled": false,
      "timeout": 3600,
      "url": "http://mcpfusion.internal:8888/sse",
      "transportType": "sse",
      "retryAttempts": 5,
      "retryDelay": 2000
    }
  }
}
```

**Load Balanced Setup:**
```json
{
  "mcpServers": {
    "MCPFusion-Primary": {
      "disabled": false,
      "timeout": 1800,
      "url": "http://mcpfusion-1.internal:8888/sse",
      "transportType": "sse",
      "priority": 1
    },
    "MCPFusion-Secondary": {
      "disabled": true,
      "timeout": 1800, 
      "url": "http://mcpfusion-2.internal:8888/sse",
      "transportType": "sse",
      "priority": 2
    }
  }
}
```

For more information on MCPFusion server configuration, see [config.md](config.md).