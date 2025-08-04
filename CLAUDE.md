# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Working with Claude Code Sub-Agents

When working on this codebase, Claude Code will proactively use specialized sub-agents for different tasks:

- **golang-architect**: Used for designing new application architectures, planning package structures, making architectural decisions, or adding new packages
- **golang-developer**: Used for implementing Go code including writing functions, methods, structs, interfaces, business logic, data processing, utilities, tests, and features
- **code-quality-inspector**: Used for code review and quality assurance of recently written or modified code

These sub-agents will be automatically engaged based on the nature of your request to ensure high-quality code and architectural decisions.

## Project Overview

MCPFusion is a Model Context Protocol (MCP) server implementation in Go that enables AI clients to interact with APIs and services through a standardized interface. It provides tools, resources, and prompts to MCP clients.

## Key Commands

### Running the Server
```bash
# Basic run (defaults to port 8080)
go run .

# Run with custom port
go run . -port 8081

# Run with debug logging
go run . -debug

# Run without SSE streaming
go run . -no-streaming

# Build the binary
go build -o mcpfusion .
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...
```

## Architecture

### Core Components

1. **MCP Server** (`mcpserver/`)
   - Handles MCP protocol implementation
   - Manages SSE and HTTP transports
   - Routes tool/resource/prompt calls to providers

2. **Provider Interfaces** (`global/`)
   - `ToolProvider`: Implements tools (functions AI can call)
   - `ResourceProvider`: Implements resources (data AI can read)
   - `PromptProvider`: Implements prompts (templates for AI)
   - All providers implement `RegisterTools()`, `RegisterResources()`, or `RegisterPrompts()`

3. **Transport Layer**
   - SSE transport for real-time communication (default)
   - HTTP transport for simpler request/response

### Handler Patterns

Tool handlers follow this signature:
```go
func(args map[string]interface{}) (string, error)
```

Resource handlers follow:
```go
func(uri string) (string, error)
```

Prompt handlers follow:
```go
func(args map[string]interface{}) (*global.PromptResponse, error)
```

## Configuration

MCPFusion uses environment variables loaded from `~/.mcp` file:
```bash
# Example ~/.mcp file
API_KEY=your-api-key
API_BASE_URL=https://api.example.com
```

## Creating New Providers

1. Create a new package in the project root
2. Implement the appropriate interface(s) from `global/`
3. Use functional options pattern for configuration:
```go
type Option func(*Config)

func WithAPIKey(key string) Option {
    return func(c *Config) {
        c.APIKey = key
    }
}
```

4. Register your provider in `main.go`:
```go
provider := yourprovider.New(
    yourprovider.WithAPIKey(apiKey),
)
server.AddToolProvider(provider)
```

## Example Providers

- **example1/**: Demonstrates a full REST API wrapper with:
  - GET/POST/DELETE tools
  - Resources for data retrieval
  - Prompts for common operations
  - Environment-based configuration

- **example2/**: Simple time service showing:
  - Basic tool implementation
  - Multiple handler registration
  - No external dependencies

## Important Patterns

1. **Error Handling**: Always return structured errors from handlers
2. **Logging**: Use `mlogger` package for consistent logging
3. **Configuration**: Use functional options for flexible configuration
4. **Testing**: Each provider should have its own test file

## MCP Protocol Details

The server implements MCP v1.0.0 with:
- Tool execution via `tools/call`
- Resource retrieval via `resources/read`
- Prompt rendering via `prompts/get`
- Capability negotiation during initialization