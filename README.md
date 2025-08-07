# MCPFusion

A dynamic, configuration-driven MCP (Model Context Protocol) server that enables AI clients to interact with multiple APIs and services through a standardized interface.

## Features

- **Multi-Provider Support**: Connect to Microsoft 365, Google APIs, and custom REST services
- **OAuth2 Device Flow**: Secure authentication with automatic token refresh
- **Configuration-Driven**: Add new APIs without code changes through JSON configuration
- **Production Ready**: Circuit breakers, retry logic, metrics, and comprehensive error handling

## Quick Start

1. **Build and Run**:
   ```bash
   go build -o mcpfusion .
   ./mcpfusion -port 8888
   ```

2. **Add API Integration** (optional):
   ```bash
   ./mcpfusion -fusion-config fusion/configs/microsoft365.json -port 8888
   ```

## API Setup Guides

- 📧 **[Microsoft 365 API Setup](SETUP_MICROSOFT365.md)** - Complete Azure app registration and authentication guide
- 🔍 **[Google APIs Setup](fusion/README_CONFIG.md#google-apis-setup)** - Google Cloud Console configuration

## Documentation

- **[Fusion Package](fusion/README.md)** - Dynamic API provider with OAuth2 support  
- **[Configuration Guide](fusion/README_CONFIG.md)** - Detailed configuration options
- **[Quick Start](fusion/QUICKSTART.md)** - 5-minute setup guide
- **[Examples](fusion/examples/)** - Integration examples for main.go and Docker

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

## Available Providers

1. **Fusion Provider** - Dynamic API access through JSON configuration
   - Microsoft 365 Graph API (5 tools: profile, calendar, mail, contacts)
   - Google APIs (16 tools: Calendar, Gmail, Drive, Profile)
   - Custom REST APIs with multiple authentication methods

2. **Example Providers** - Reference implementations
   - example1: Generic REST API wrapper
   - example2: Simple time service

## Production Features

- ✅ **OAuth2 Device Flow** with automatic token refresh
- ✅ **Circuit Breakers** with configurable failure thresholds  
- ✅ **Retry Logic** with exponential backoff and jitter
- ✅ **Response Caching** with configurable TTL
- ✅ **Metrics Collection** with health monitoring
- ✅ **Correlation IDs** for request tracing
- ✅ **Comprehensive Error Handling** with user-friendly messages

## License

MIT License - see [LICENSE](LICENSE) for details.

