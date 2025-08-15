# MCPFusion

A production-ready, configuration-driven MCP (Model Context Protocol) server that enables AI clients to interact with multiple APIs and services through a standardized interface. Create powerful AI tools from any REST API using simple JSON configuration.

## Features

- **🔌 Universal API Integration**: Connect to any REST API using JSON configuration
- **🔐 Multiple Authentication**: OAuth2 Device Flow, Bearer tokens, API keys, and Basic Auth
- **⚡ Enhanced Parameter System**: Rich parameter metadata with defaults, validation, and constraints
- **📁 Multi-Calendar & Mail Folder Support**: Target specific calendars and mail folders
- **🔄 Production-Grade Reliability**: Circuit breakers, retry logic, caching, and comprehensive error handling
- **📊 Real-time Metrics**: Health monitoring, correlation IDs, and detailed logging
- **🧪 Comprehensive Testing**: Full test suite for all endpoints and configurations

## Quick Start

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

3. **Connect Your MCP Client**:
   - **Cline**: Add to `~/.cline/config.json`
   - **Claude Desktop**: Add to MCP servers configuration
   - **Custom Client**: Connect to `http://localhost:8888/sse`

See [Client Configuration Guide](docs/clients.md) for detailed setup instructions.

## Documentation

### **Configuration & Setup**
- 📚 **[Configuration Guide](docs/config.md)** - Complete guide to creating JSON configurations for any API
- 🔌 **[Client Integration](docs/clients.md)** - Connect Cline, Claude Desktop, and custom MCP clients
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
├── example1/           # REST API provider example
├── example2/           # Simple time service example
└── global/             # Shared interfaces and utilities
```

## Available Tools & APIs

### **Microsoft 365 Integration** (13 tools)
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
| **Mail Management** | `microsoft365_mail_folders_list` | List all mail folders |
| | `microsoft365_mail_read_inbox` | Inbox messages |
| | `microsoft365_mail_folder_messages` | Messages from specific folder |
| | `microsoft365_mail_read_message` | Individual message by ID |
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
- ✅ All 13 Microsoft 365 endpoints
- ✅ Parameter validation and constraints
- ✅ Authentication flows
- ✅ Error handling scenarios
- ✅ Multiple data formats and edge cases

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

The enhanced parameter system ensures the AI has all the context needed to make proper API calls with appropriate defaults and validation.

## License

MIT License - see [LICENSE](LICENSE) for details.

