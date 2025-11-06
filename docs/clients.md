# MCPFusion Client Configuration Guide

This guide explains how to configure various MCP clients to connect to MCPFusion servers.

## Table of Contents

- [Overview](#overview)
- [Claude Code](#claude code)
- [Claude Desktop](#claude desktop)
- [Cline IDE Integration](#cline)


## Overview

MCPFusion serves as an MCP (Model Context Protocol) server that provides AI clients with access to external APIs through standardized tools. Clients connect to MCPFusion using either the legacy SSE transport or the modern Streamable HTTP transport.

**Supported Transports**

Both transports are always available simultaneously - clients can use whichever they support:

- **Streamable HTTP Transport (modern)**: Unified HTTP endpoint per MCP specification
    - Unified endpoint: `http://localhost:8888/mcp`

- **SSE Transport (legacy)**: Server-Sent Events for real-time bidirectional communication
  - Stream endpoint: `http://localhost:8888/sse`
  - Message endpoint: `http://localhost:8888/message`

**Note**: Replace `localhost:8888` with your actual server address and port.

**Unsupported Clients**

For clients unable or unwilling to support MCP over HTTP, it may be preferable to use MCPRelay to access a remote MCP server via a stdio transport. For further information, please see https://github.com/PivotLLM/MCPRelay

## Claude Code
MCP servers can be added to Claude code via the command line. To add an MCP server scoped to the user (all projects):

`claude mcp add --transport sse Fusion --scope user http://127.0.0.1:8888/sse --header "Authorization: Bearer <token>"`

For further information use:

`claude mcp -h`


## Claude Desktop

Claude Desktop does not support HTTP header bearer tokens, nor will it support HTTP (as opposed to HTTPS) even on localhost. The author's decision to only support OAUTH authentication may be future-facing, but it ignore the practical solutions required today.

One workaround is to use the MCPRelay utility:

```json
{
  "mcpServers": {
    "Fusion": {
      "command": "/opt/mcprelay/mcprelay",
      "args": [
        "-url",
        "http://127.0.0.1:8888/sse",
        "-headers",
        "{\"Authorization\":\"Bearer <token>\"}",
        "-log",
        "/opt/mcprelay/relay-fusion.log",
        "-debug"
      ]
    }
  }
}
```

Please see https://github.com/PivotLLM/MCPRelay for further information.

## Cline

[Cline](https://github.com/cline/cline) is a VS Code extension that provides AI-powered coding assistance with MCP support.

**NOTE: An open issue in Cline causes the Authorization header to not be sent:
https://github.com/cline/cline/issues/4391**

**A temporary workaround in secure environments may be to use MCPFusion's -no-auth command line switch or MCPRelay.**

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

### Example configuration

```json
{
  "mcpServers": {
    "MCPFusion": {
      "url": "http://localhost:8888/sse",
      "headers": {
				"Authorization": "Bearer <token>"
			},
      "disabled": false,
      "timeout": 3600,
      "transportType": "sse"
    }
  }
}
```

### Configuration Fields

| Field           | Type    | Required | Description                                     |
| --------------- | ------- | -------- | ----------------------------------------------- |
| `disabled`      | boolean | No       | Whether to disable this server (default: false) |
| `timeout`       | number  | No       | Connection timeout in seconds (default: 30)     |
| `url`           | string  | Yes      | MCPFusion server URL                            |
| `transportType` | string  | Yes      | Transport protocol ("sse" or "http")            |

For more information please refer to https://docs.cline.bot/mcp/configuring-mcp-servers

## Visual Studio Code with GitHub Copilot

The is more than one way to configure VS Code with Copilot. Opening the command palate and searching for @mcp should find "MCP: Open User Configuration."

VS Code appears to prefer the more modern streaming HTTP. The following configuration is recommended:

```json
{
  "servers": {
    "Fusion": {
    "type": "http",
	  "url": "http://localhost:8888/mcp",
      "headers": {
        "Authorization": "Bearer <your-key>"
      }
    }
  },
  "inputs": []
}
```

Copyright (c) 2025 Tenebris Technologies Inc. All rights reserved.

