# MCPFusion Client Configuration Guide

This guide explains how to configure various MCP clients to connect to MCPFusion servers.

## Supported Clients

- [Claude Code](#claude-code)
- [Claude Desktop](#claude-desktop)
- [Cline](#cline)
- [Gemini CLI](#gemini-cli)
- [PicoClaw](#picoclaw)
- [Visual Studio Code with GitHub Copilot](#visual-studio-code-with-github-copilot)

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

`claude mcp add --transport http fusion --scope user http://127.0.0.1:8888/mcp --header "Authorization: Bearer <token>"`

To list configured MCP servers:

`claude mcp list`

For further information use:

`claude mcp -h`


## Claude Desktop

Claude Desktop does not support HTTP header bearer tokens, nor will it support HTTP (as opposed to HTTPS) even on localhost. The author's decision to only support OAUTH authentication may be future-facing, but it ignores the practical solutions required today.

Since Claude Desktop does fully support "Local MCP servers" that use the stdio transport, you can use a utility such as MCPRelay to bridge between a stdio transport and a network-accessible MCP server.

Example:

```json
{
  "mcpServers": {
    "fusion": {
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

To access the configuration in Claude Desktop, click on your name at the lower left, then "Settings". Scroll to the bottom of the settings and click on "Developer". Local MCP servers will be displayed on the right pane. Click "Edit Config" to access the configuration file.

Please see https://github.com/PivotLLM/MCPRelay for further information.

## Cline

[Cline](https://github.com/cline/cline) is a VS Code extension that provides AI-powered coding assistance with MCP support.

**NOTE: An open issue in Cline causes the Authorization header to not be sent:
https://github.com/cline/cline/issues/4391**

**A temporary workaround in secure environments may be to use MCPFusion's -no-auth command line switch or use MCPRelay to bridge between a stdio MCP transport and the network.**

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
    "fusion": {
      "url": "http://127.0.0.1:8888/sse",
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

## Gemini CLI

[Gemini CLI](https://github.com/google-gemini/gemini-cli) supports MCP servers via the `gemini mcp add` command or by editing `~/.gemini/settings.json` directly.

### Command Line

To add MCPFusion scoped to the user (all projects):

```bash
gemini mcp add fusion http://127.0.0.1:8888/mcp --scope user --transport http --header "Authorization: Bearer <token>"
```

Omit `--scope user` to configure MCPFusion at the project level instead.

> **Warning:** The `--trust` flag grants Gemini CLI unrestricted access to all MCP tools without prompting for permission. Only use `--trust` in controlled environments where you fully trust the MCP server and its tools. Do not use it with servers you do not control or that have access to sensitive data or destructive operations.

To grant access to all tools without prompting (use with caution — see warning above):

```bash
gemini mcp add fusion http://127.0.0.1:8888/mcp --scope user --transport http --header "Authorization: Bearer <token>" --trust
```

### Manual Configuration

Alternatively, add the following to `~/.gemini/settings.json`:

```json
{
  "mcpServers": {
    "fusion": {
      "url": "http://127.0.0.1:8888/mcp",
      "type": "http",
      "headers": {
        "Authorization": "Bearer <token>"
      }
    }
  }
}
```

To allow Gemini CLI access to all tools without prompting for permission, add `"trust": true` to the server definition (use with caution — see warning above):

```json
{
  "mcpServers": {
    "fusion": {
      "url": "http://127.0.0.1:8888/mcp",
      "type": "http",
      "trust": true,
      "headers": {
        "Authorization": "Bearer <token>"
      }
    }
  }
}
```

## PicoClaw

Edit `~/.picoclaw/config.json` and add an `mcp` section (or merge it into an existing one):

```json
{
  "mcp": {
    "enabled": true,
    "servers": {
      "fusion": {
        "enabled": true,
        "type": "http",
        "url": "http://127.0.0.1:8888/mcp",
        "headers": {
          "Authorization": "Bearer <token>"
        }
      }
    }
  }
}
```

Replace `<token>` with your MCPFusion API token and `127.0.0.1:8888` with your server address if different.

## Visual Studio Code with GitHub Copilot

There is more than one way to configure VS Code with Copilot. Opening the command palette and searching for @mcp should find "MCP: Open User Configuration."

VS Code appears to prefer the more modern streaming HTTP. The following configuration is recommended:

```json
{
  "servers": {
    "fusion": {
      "type": "http",
      "url": "http://127.0.0.1:8888/mcp",
      "headers": {
        "Authorization": "Bearer <token>"
      }
    }
  },
  "inputs": []
}
```

Copyright (c) 2025 Tenebris Technologies Inc. All rights reserved.

