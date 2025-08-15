# MCPFusion

A production-ready, configuration-driven MCP (Model Context Protocol) server that enables AI clients to interact with multiple APIs and services through a standardized interface. Create powerful AI tools from any REST API using simple JSON configuration.

## Features

- **🔌 Universal API Integration**: Connect to any REST API using JSON configuration
- **🔐 Multi-Tenant Authentication**: Complete tenant isolation with database-backed token management
- **🎫 Bearer Token Support**: Industry-standard `Authorization: Bearer <token>` authentication
- **⚡ Enhanced Parameter System**: Rich parameter metadata with defaults, validation, and constraints
- **📁 Multi-Calendar & Mail Folder Support**: Target specific calendars and mail folders
- **🔄 Production-Grade Reliability**: Circuit breakers, retry logic, caching, and comprehensive error handling
- **📊 Real-time Metrics**: Health monitoring, correlation IDs, and detailed logging
- **🧪 Comprehensive Testing**: Full test suite for all endpoints and configurations
- **🛠️ CLI Token Management**: Complete command-line tools for token administration

## Quick Start

### **Single-Tenant Mode (Legacy)**

1. **Build and Run**:
   ```bash
   go build -o mcpfusion .
   ./mcpfusion -config configs/microsoft365.json -port 8888
   ```

2. **Configure Environment Variables**:
   ```bash
   # Create ~/.mcp file with your API credentials
   echo "MS365_CLIENT_ID=your-client-id" >> ~/.mcp
   echo "MS365_TENANT_ID=your-tenant-id" >> ~/.mcp
   ```

### **Multi-Tenant Mode (Recommended)**

1. **Enable Multi-Tenant Features**:
   ```bash
   # Enable database and bearer token authentication
   export MCP_ENABLE_DATABASE=true
   export MCP_ENABLE_BEARER_TOKENS=true
   
   # Optional: Custom database directory
   export MCP_DB_DATA_DIR=/opt/mcpfusion
   ```

2. **Build and Generate API Token**:
   ```bash
   # Build the server
   go build -o mcpfusion .
   
   # Generate API token for your application
   ./mcpfusion -token-add "Production environment"
   ```

3. **Start Server and Connect**:
   ```bash
   # Start server with multi-tenant support
   ./mcpfusion -config configs/microsoft365.json -port 8888
   
   # Connect using Bearer token authentication
   curl -H "Authorization: Bearer YOUR_TOKEN" http://localhost:8888/api/endpoint
   ```

### **Client Configuration**

- **Cline**: Add to `~/.cline/config.json`
- **Claude Desktop**: Add to MCP servers configuration
- **Custom Client**: Connect to `http://localhost:8888/sse`

See [Client Configuration Guide](docs/clients.md) and [Token Management Guide](docs/TOKEN_MANAGEMENT.md) for detailed setup instructions.

## Documentation

### **Configuration & Setup**
- 📚 **[Configuration Guide](docs/config.md)** - Complete guide to creating JSON configurations for any API
- 🔌 **[Client Integration](docs/clients.md)** - Connect Cline, Claude Desktop, and custom MCP clients
- 🎫 **[Token Management Guide](docs/TOKEN_MANAGEMENT.md)** - Multi-tenant authentication and CLI usage
- 📧 **[Microsoft 365 Setup](docs/Microsoft365.md)** - Azure app registration and authentication
- 🔍 **[Google APIs Setup](fusion/README_CONFIG.md#google-apis-setup)** - Google Cloud Console configuration

### **Development & Testing**
- ⚡ **[Quick Start Guide](fusion/QUICKSTART.md)** - 5-minute setup guide
- 🧪 **[Testing Guide](tests/README.md)** - Comprehensive test suite documentation
- 💻 **[Integration Examples](fusion/examples/)** - Code examples for custom integrations
- 🏗️ **[Architecture Overview](fusion/README.md)** - System design and components

## Architecture

```
MCPFusion/
├── mcpserver/          # Core MCP protocol implementation
├── fusion/             # Dynamic API provider package
│   ├── configs/        # Pre-configured API definitions
│   └── examples/       # Integration examples  
├── db/                 # Multi-tenant database package (BoltDB)
├── cmd/token/          # Token management CLI
├── example1/           # REST API provider example
├── example2/           # Simple time service example
└── global/             # Shared interfaces and utilities
```

### **Multi-Tenant Architecture**

MCPFusion supports complete tenant isolation where each API token represents a separate namespace:

- **🏢 Tenant Isolation**: Each API token has independent OAuth tokens and service credentials
- **🗄️ Database Storage**: BoltDB-based persistent storage with automatic cleanup
- **🔐 Secure Tokens**: SHA-256 hashed API tokens with auto-generation
- **⚖️ Load Balancing**: Multiple tenants can use the same MCPFusion instance
- **📊 Per-Tenant Analytics**: Independent metrics and monitoring per tenant

## Token Management

MCPFusion includes a comprehensive CLI for managing API tokens:

### **Token Commands**

```bash
# Generate new API token
./mcpfusion -token-add "Production environment"
> ✓ API Token created successfully
> ⚠ SECURITY WARNING: This token will only be displayed once!
> Token: 1a2b3c4d5e6f7890abcdef1234567890abcdef1234567890abcdef1234567890
> Hash: a1b2c3d4e5f6789...

# List all tokens
./mcpfusion -token-list
> PREFIX     HASH                 CREATED              LAST USED           DESCRIPTION
> 1a2b3c4d   a1b2c3d4e5f6...      2025-01-15 10:30:00  2025-01-15 11:45:00  Production environment
> 9f8e7d6c   f9e8d7c6b5a4...      2025-01-14 09:15:00  Never used           Development token

# Delete token by prefix or hash
./mcpfusion -token-delete 1a2b3c4d
> Token Details:
>   Hash: a1b2c3d4e5f6...
>   Description: Production environment
>   Created: 2025-01-15 10:30:00
> Are you sure you want to delete this token? (y/N): y
> Token deleted successfully
```

### **Token Features**

- **🔒 Auto-Generated**: Cryptographically secure 64-character hex tokens
- **🎯 Prefix Identification**: 8-character prefixes for easy management
- **🛡️ One-Time Display**: Tokens shown only during creation for security
- **⏰ Usage Tracking**: Creation time and last-used timestamps
- **🗑️ Safe Deletion**: Confirmation prompts with token details
- **📁 Custom Descriptions**: Label tokens for different environments/applications

### **Environment Configuration**

```bash
# Enable multi-tenant mode
export MCP_ENABLE_DATABASE=true
export MCP_ENABLE_BEARER_TOKENS=true

# Optional settings
export MCP_DB_DATA_DIR=/opt/mcpfusion  # Custom database directory
```

### **Authentication Usage**

Include tokens in HTTP requests using standard Bearer authentication:

```bash
# API requests
curl -H "Authorization: Bearer YOUR_TOKEN" \
     https://mcpfusion-server/api/microsoft365/profile

# MCP client configuration
{
  "command": "mcpfusion",
  "args": ["-config", "config.json"],
  "env": {
    "MCPFUSION_TOKEN": "YOUR_TOKEN"
  }
}
```

**📖 Complete Guide**: See [Token Management Documentation](docs/TOKEN_MANAGEMENT.md) for detailed usage instructions.

## Available Tools & APIs

### **Microsoft 365 Integration** (19 tools)
**Authentication:** OAuth2 Device Flow with automatic token refresh

| Category | Tools | Description |
|----------|-------|-------------|
| **Profile** | `microsoft365_profile_get` | User profile information |
| **Calendar Management** | `microsoft365_calendars_list` | List all user calendars |
| | `microsoft365_calendar_read_summary` | Calendar events (summary view) |
| | `microsoft365_calendar_read_details` | Calendar events (detailed view) |
| | `microsoft365_calendar_events_read_summary` | Events from specific calendar (summary) |
| | `microsoft365_calendar_events_read_details` | Events from specific calendar (detailed) |
| | `microsoft365_calendar_read_event` | Individual event by ID |
| | `microsoft365_calendar_search` | **Search calendar events with flexible filtering** |
| **Mail Management** | `microsoft365_mail_folders_list` | List all mail folders |
| | `microsoft365_mail_read_inbox` | Inbox messages |
| | `microsoft365_mail_folder_messages` | Messages from specific folder |
| | `microsoft365_mail_read_message` | Individual message by ID |
| | `microsoft365_mail_search` | **Search mail with filter and full-text search** |
| **File Management** | `microsoft365_files_list` | List OneDrive files and folders |
| | `microsoft365_files_search` | **Search OneDrive files with flexible filtering** |
| | `microsoft365_files_read_file` | Individual file details by ID |
| **Contacts** | `microsoft365_contacts_list` | List contacts |
| | `microsoft365_contacts_read_contact` | Individual contact by ID |

### **Google APIs Integration** (16 tools)
See [Google Configuration](fusion/README_CONFIG.md#google-apis-setup) for setup details.

### **Custom API Integration**
Create tools for any REST API using [JSON configuration](docs/config.md). Supports:
- **Authentication**: OAuth2, Bearer tokens, API keys, Basic Auth
- **Parameter Types**: String, number, boolean, array, object with validation
- **Response Handling**: JSON, text, binary with pagination support
- **Advanced Features**: Caching, retry logic, circuit breakers

## Key Features

### **Enhanced Parameter System**
- **Rich Metadata**: Parameters include defaults, validation rules, and constraint information
- **Type Safety**: Full support for string, number, boolean, array, and object types
- **Smart Validation**: Pattern matching, length constraints, enum values, and numeric ranges
- **LLM-Friendly**: Enhanced descriptions show constraints and examples to guide AI usage

### **Production-Grade Reliability**
- ✅ **Multiple Authentication Methods**: OAuth2 Device Flow, Bearer tokens, API keys, Basic Auth
- ✅ **Circuit Breakers**: Configurable failure thresholds with automatic recovery
- ✅ **Intelligent Retry Logic**: Exponential backoff with jitter and error categorization
- ✅ **Response Caching**: Configurable TTL with automatic cache invalidation
- ✅ **Real-time Metrics**: Request latency, success rates, and error categorization
- ✅ **Correlation Tracking**: Request IDs for distributed tracing and debugging
- ✅ **Comprehensive Error Handling**: User-friendly messages with actionable guidance

### **Advanced API Integration**
- **Pagination Support**: Automatic multi-page fetching for large datasets
- **Parameter Transformation**: Convert parameter formats before API calls
- **Response Processing**: JSON path extraction and data transformation
- **Flexible Routing**: Support for path parameters and multiple HTTP methods

## Testing

MCPFusion includes a comprehensive test suite for all endpoints:

```bash
# Run all tests
cd tests && ./run_all_tests.sh

# Run specific endpoint tests
./test_calendars_list.sh
./test_mail_folders.sh
./test_individual_items.sh
```

**Test Coverage:**
- ✅ All 19 Microsoft 365 endpoints (including search capabilities)
- ✅ Parameter validation and constraints
- ✅ Authentication flows
- ✅ Error handling scenarios
- ✅ Multiple data formats and edge cases
- ✅ Advanced search and filtering scenarios

See [Testing Guide](tests/README.md) for detailed testing documentation.

## Example Usage

Once connected through your MCP client, you can use natural language to interact with APIs:

**"Show me my calendar events for next week"**
→ Automatically calls `microsoft365_calendar_read_summary` with appropriate date range

**"List all my mail folders"**
→ Calls `microsoft365_mail_folders_list` to show folder structure

**"Get unread emails from my inbox"**
→ Calls `microsoft365_mail_folder_messages` with inbox ID and unread filter

**"Show details for calendar event {event-id}"**
→ Calls `microsoft365_calendar_read_event` with the specific event ID

**"Find all my meetings with John from last month"**
→ Calls `microsoft365_calendar_search` with date range and attendee filter

**"Search emails from boss@company.com about project status"**
→ Calls `microsoft365_mail_search` with sender and subject filters

**"Find all PDF files in my OneDrive modified this year"**
→ Calls `microsoft365_files_search` with file type and date filters

The enhanced parameter system ensures the AI has all the context needed to make proper API calls with appropriate defaults and validation.

## License

MIT License - see [LICENSE](LICENSE) for details.

