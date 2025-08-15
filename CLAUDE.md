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
# Run all Go unit tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run MCP function tests (requires running server)
cd tests && ./run_all_tests.sh

# Run individual MCP function tests
cd tests && ./test_profile.sh > profile_output.log
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
5. **Multi-Tenant Authentication**: Use database-backed token system for production deployments
6. **Bearer Token Authentication**: Support standard Authorization: Bearer <token> headers

## Multi-Tenant Authentication

MCPFusion now supports multi-tenant authentication with the following components:

### Database Package (`db/`)
- BoltDB-based persistent storage (`go.etcd.io/bbolt`)
- API token management with auto-generation
- OAuth token storage per tenant per service
- Service credentials management
- Comprehensive CLI tools for token administration

### Authentication Flow
1. **API Token Generation**: Use `mcpfusion-token add` to create tenant tokens
2. **Bearer Authentication**: Include `Authorization: Bearer <token>` in HTTP requests
3. **Tenant Isolation**: Each API token represents a separate tenant namespace
4. **Service Independence**: Each tenant has independent OAuth tokens for each service

### Environment Variables for Multi-Tenant Mode
```bash
# Enable database for persistent storage
MCP_ENABLE_DATABASE=true

# Enable bearer token authentication
MCP_ENABLE_BEARER_TOKENS=true

# Optional: Set custom database directory
MCP_DB_DATA_DIR=/path/to/data
```

### Token Management CLI
```bash
# Generate new API token
mcpfusion-token add "Production environment"

# List all tokens
mcpfusion-token list

# Delete token
mcpfusion-token delete abc12345
```

## Testing Requirements

**IMPORTANT**: Each MCP tool provided by the server MUST have at least one separate test file in the `tests/` directory.

### Test Structure
- Each MCP tool should have its own dedicated test script (e.g., `test_toolname.sh`)
- Test scripts should generate timestamped `.log` files with complete request/response data
- The `run_all_tests.sh` script should include all individual test scripts

### Current Test Coverage
- `test_profile.sh` - Tests `microsoft365_profile_get`
- `test_calendar_summary.sh` - Tests `microsoft365_calendar_read_summary`
- `test_calendar_details.sh` - Tests `microsoft365_calendar_read_details`
- `test_mail.sh` - Tests `microsoft365_mail_read_inbox`
- `test_contacts.sh` - Tests `microsoft365_contacts_list`

### Adding New Tool Tests
When adding new MCP tools:
1. Create a dedicated test script in `tests/test_newtool.sh`
2. Add the test to `tests/run_all_tests.sh`
3. Update the test documentation in `tests/README.md`
4. Ensure the test covers multiple scenarios (default params, custom fields, edge cases)

## MCP Protocol Details

The server implements MCP v1.0.0 with:
- Tool execution via `tools/call`
- Resource retrieval via `resources/read`
- Prompt rendering via `prompts/get`
- Capability negotiation during initialization