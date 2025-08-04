# Fusion Package

The Fusion package is a dynamic, configuration-driven MCP provider that enables access to multiple APIs through JSON configuration. It supports various authentication methods (OAuth2 device flow, bearer tokens, API keys, basic auth) and allows adding new API endpoints without code changes.

## Package Structure

```
fusion/
├── fusion.go           # Main package entry point with New() and functional options
├── config.go           # Configuration structures and JSON parsing
├── auth.go             # Authentication manager and strategies
├── cache.go            # Token and response caching
├── errors.go           # Custom error types
├── fusion_test.go      # Unit tests
├── example_test.go     # Example usage tests
└── configs/            # Example JSON configurations
    ├── microsoft365.json
    ├── google.json
    └── schema.json
```

## Core Features

### 1. Configuration-Driven API Access
- Define APIs through JSON configuration files
- Support for multiple services in a single configuration
- Environment variable expansion for sensitive values
- Runtime validation of configuration

### 2. Multiple Authentication Methods
- **OAuth2 Device Flow**: For services like Microsoft 365
- **Bearer Token**: For APIs with static tokens
- **API Key**: For key-based authentication
- **Basic Auth**: For username/password authentication

### 3. Dynamic Tool Generation
- Automatically generates MCP tools from endpoint configurations
- Parameter validation and transformation
- Response processing and transformation
- Pagination support

### 4. Extensible Architecture
- Interface-based design for easy extension
- Pluggable authentication strategies
- Configurable caching system
- Comprehensive error handling

## Quick Start

### Basic Usage

```go
package main

import (
    "github.com/PivotLLM/MCPFusion/fusion"
    "github.com/PivotLLM/MCPFusion/mlogger"
)

func main() {
    // Create logger
    logger, _ := mlogger.New()
    defer logger.Close()

    // Create fusion provider with configuration
    fusionProvider := fusion.New(
        fusion.WithJSONConfig("config.json"),
        fusion.WithLogger(logger),
        fusion.WithInMemoryCache(),
    )

    // Register with MCP server
    server.AddToolProvider(fusionProvider)
}
```

### Configuration Example

```json
{
  "services": {
    "myapi": {
      "name": "My API",
      "baseURL": "https://api.example.com",
      "auth": {
        "type": "bearer",
        "config": {
          "tokenEnvVar": "MY_API_TOKEN"
        }
      },
      "endpoints": [
        {
          "id": "get_user",
          "name": "Get User",
          "description": "Get user information",
          "method": "GET",
          "path": "/users/{userId}",
          "parameters": [
            {
              "name": "userId",
              "description": "User ID",
              "type": "string",
              "required": true,
              "location": "path"
            }
          ],
          "response": {
            "type": "json"
          }
        }
      ]
    }
  }
}
```

## Core Types

### Config
Main configuration structure that holds all service definitions.

### ServiceConfig
Configuration for a single API service including:
- Name and base URL
- Authentication configuration
- List of endpoints

### EndpointConfig
Configuration for a single API endpoint including:
- HTTP method and path
- Parameters with validation and transformation
- Response handling configuration

### AuthConfig
Authentication configuration supporting multiple auth types:
- OAuth2 device flow
- Bearer tokens
- API keys
- Basic authentication

### ParameterConfig
Parameter definition with:
- Type validation
- Location specification (path, query, body, header)
- Transformation rules
- Default values

## Authentication Strategies

### OAuth2 Device Flow
```json
{
  "type": "oauth2_device",
  "config": {
    "clientId": "${CLIENT_ID}",
    "tenantId": "${TENANT_ID}",
    "scope": ["https://graph.microsoft.com/Calendars.Read"],
    "authorizationURL": "https://login.microsoftonline.com/{tenantId}/oauth2/v2.0/devicecode",
    "tokenURL": "https://login.microsoftonline.com/{tenantId}/oauth2/v2.0/token"
  }
}
```

### Bearer Token
```json
{
  "type": "bearer",
  "config": {
    "tokenEnvVar": "API_TOKEN"
  }
}
```

### API Key
```json
{
  "type": "api_key",
  "config": {
    "apiKeyEnvVar": "API_KEY",
    "headerName": "X-API-Key"
  }
}
```

### Basic Auth
```json
{
  "type": "basic",
  "config": {
    "username": "user",
    "password": "${PASSWORD}"
  }
}
```

## Parameter Transformation

Transform parameter values before sending to API:

```json
{
  "name": "startDate",
  "description": "Start date in YYYYMMDD format",
  "type": "string",
  "required": true,
  "location": "query",
  "validation": {
    "pattern": "^\\d{8}$"
  },
  "transform": {
    "targetName": "startDateTime",
    "expression": "concat(slice(0,4), '-', slice(4,6), '-', slice(6,8), 'T00:00:00Z')"
  }
}
```

## Response Processing

### Simple JSON Response
```json
{
  "response": {
    "type": "json",
    "transform": ".data | map({id: .id, name: .name})"
  }
}
```

### Paginated Response
```json
{
  "response": {
    "type": "json",
    "paginated": true,
    "paginationConfig": {
      "nextPageTokenPath": "@odata.nextLink",
      "dataPath": "value",
      "pageSize": 50
    }
  }
}
```

## Error Handling

The package provides structured error types:

- `AuthenticationError`: Authentication failures
- `ConfigurationError`: Configuration issues
- `ValidationError`: Parameter validation failures
- `APIError`: API call failures
- `DeviceCodeError`: OAuth2 device flow errors
- `TokenError`: Token-related errors
- `NetworkError`: Network connectivity issues

## Functional Options

Configure the Fusion instance using functional options:

```go
fusion := New(
    WithJSONConfig("config.json"),           // Load from file
    WithJSONConfigData(jsonBytes, "config"), // Load from bytes
    WithLogger(logger),                       // Set logger
    WithHTTPClient(client),                   // Custom HTTP client
    WithInMemoryCache(),                      // Enable caching
    WithTimeout(30*time.Second),              // Set timeout
)
```

## Interface Compliance

The Fusion package implements the required MCPFusion interfaces:

- `global.ToolProvider`: Provides dynamic tools
- `global.ResourceProvider`: Optional resource provision (empty by default)
- `global.PromptProvider`: Optional prompt provision (empty by default)

## Testing

Run tests with:
```bash
go test ./fusion
go test ./fusion -run Example  # Run example tests
```

## Next Steps

This foundation provides:
1. ✅ Core configuration types and validation
2. ✅ Authentication strategy interfaces and basic implementations
3. ✅ Dynamic tool registration from configuration
4. ✅ Error handling and caching infrastructure
5. ✅ Example configurations for Microsoft 365 and Google APIs

**Future Implementation Phases:**
1. **OAuth2 Device Flow Implementation**: Complete the device flow authentication
2. **HTTP Request/Response Handling**: Implement actual API calls with retries
3. **Parameter Transformation**: Implement expression-based transformations
4. **Response Processing**: Add JQ-like response transformations
5. **Pagination Support**: Handle paginated API responses

The package structure is designed to be extensible and follows Go best practices with comprehensive error handling, interface-based design, and thorough testing.