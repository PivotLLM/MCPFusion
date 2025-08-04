# Fusion Package Architecture and Implementation Plan

## Overview

The Fusion package is a dynamic, configuration-driven MCP provider that enables access to multiple APIs through JSON configuration. It supports various authentication methods (OAuth2 device flow, bearer tokens, API keys) and allows adding new API endpoints without code changes.

## Implementation Status

### ✅ Phase 1: Core Foundation - **COMPLETED**
**Status**: Fully implemented and tested ✅

**Deliverables Completed**:
1. ✅ **Package Structure and Core Types**
   - Complete directory structure created in `/fusion/`
   - All core Go structures implemented (Config, ServiceConfig, AuthConfig, etc.)
   - Full interface definitions (AuthManager, AuthStrategy, Cache)
   - Comprehensive error type hierarchy (DeviceCodeError, ConfigurationError, etc.)

2. ✅ **JSON Configuration Loading with Validation**
   - File-based configuration loading (`LoadConfigFromFile`, `LoadConfigFromJSON`)
   - Environment variable expansion (`${VAR_NAME}` and `${VAR_NAME:default}` syntax)
   - Multi-level validation (service, endpoint, parameter validation)
   - Clear error messages for configuration issues

3. ✅ **Functional Options Pattern Setup**
   - Complete `New()` constructor with sensible defaults
   - All WithXxx option functions implemented (`WithJSONConfig`, `WithLogger`, `WithInMemoryCache`, etc.)
   - Full provider interface implementations (ToolProvider, ResourceProvider, PromptProvider)
   - Dynamic tool generation from JSON configuration

4. ✅ **Basic Logging and Error Handling**
   - Comprehensive logging using mlogger patterns throughout
   - Enhanced error types with user-friendly messages (`GetUserFriendlyMessage()`)
   - Data sanitization for security in logs (tokens, sensitive data)
   - Performance monitoring and debugging capabilities

**Files Created**:
- `/fusion/fusion.go` - Main entry point with New() and provider interfaces
- `/fusion/config.go` - Configuration loading and validation
- `/fusion/auth.go` - Authentication manager and strategies  
- `/fusion/cache.go` - Caching system (in-memory and no-op)
- `/fusion/errors.go` - Custom error types
- `/fusion/configs/microsoft365.json` - Microsoft 365 example configuration
- `/fusion/configs/google.json` - Google APIs example configuration
- `/fusion/configs/schema.json` - JSON schema for validation
- `/fusion/*_test.go` - Comprehensive test suite

**Current Functionality**:
- ✅ Loads JSON configurations with environment variable expansion
- ✅ Validates configurations against schema
- ✅ Creates MCP tools dynamically from configuration
- ✅ Handles Bearer token, API key, and Basic authentication
- ✅ Executes HTTP requests with parameter validation and transformation
- ✅ Processes JSON and text responses
- ✅ Comprehensive logging and error handling
- ✅ Integration ready with MCPFusion server

**Integration Example**:
```go
// In main.go
fusionProvider := fusion.New(
    fusion.WithJSONConfig("configs/microsoft365.json"),
    fusion.WithLogger(logger),
    fusion.WithInMemoryCache(),
)
server.AddToolProvider(fusionProvider)
```

### 🔄 Next Phases (Pending)
- **Phase 2**: OAuth2 device flow implementation
- **Phase 3**: Advanced request/response handling with pagination
- **Phase 4**: Microsoft 365 Graph API integration
- **Phase 5**: Google APIs integration
- **Phase 6**: Enhanced error handling and retry logic
- **Phase 7**: Documentation and testing finalization

## Architecture Design

### Core Components

```
fusion/
├── fusion.go           # Main package entry point with New() and options
├── config.go           # Configuration structures and JSON parsing
├── auth.go             # Authentication manager and strategies
├── handler.go          # Generic HTTP handler creation
├── validator.go        # Parameter validation
├── mapper.go           # Request/response mapping
├── cache.go            # Token and response caching
├── errors.go           # Custom error types
└── configs/            # Example JSON configurations
    ├── microsoft365.json
    ├── google.json
    └── schema.json
```

### Key Design Decisions

1. **Configuration-Driven**: All API definitions live in JSON files
2. **Authentication Abstraction**: Pluggable auth strategies with automatic token management
3. **Dynamic Tool Generation**: Tools are created at runtime from configuration
4. **Error Handling**: User-friendly errors for auth flows (e.g., device code URL)
5. **Caching**: Token caching to minimize authentication requests
6. **Validation**: Runtime parameter validation based on JSON schema

## JSON Configuration Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "services": {
      "type": "object",
      "additionalProperties": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "baseURL": {"type": "string"},
          "auth": {
            "type": "object",
            "properties": {
              "type": {"enum": ["oauth2_device", "bearer", "api_key", "basic"]},
              "config": {
                "type": "object",
                "properties": {
                  // OAuth2 Device Flow
                  "clientId": {"type": "string"},
                  "tenantId": {"type": "string"},
                  "scope": {"type": "array", "items": {"type": "string"}},
                  "authorizationURL": {"type": "string"},
                  "tokenURL": {"type": "string"},
                  
                  // Bearer Token
                  "token": {"type": "string"},
                  "tokenEnvVar": {"type": "string"},
                  
                  // API Key
                  "apiKey": {"type": "string"},
                  "apiKeyEnvVar": {"type": "string"},
                  "headerName": {"type": "string"}
                }
              }
            }
          },
          "endpoints": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "id": {"type": "string"},
                "name": {"type": "string"},
                "description": {"type": "string"},
                "method": {"enum": ["GET", "POST", "PUT", "DELETE", "PATCH"]},
                "path": {"type": "string"},
                "parameters": {
                  "type": "array",
                  "items": {
                    "type": "object",
                    "properties": {
                      "name": {"type": "string"},
                      "description": {"type": "string"},
                      "type": {"enum": ["string", "number", "boolean", "array", "object"]},
                      "required": {"type": "boolean"},
                      "location": {"enum": ["path", "query", "body", "header"]},
                      "default": {},
                      "validation": {
                        "type": "object",
                        "properties": {
                          "pattern": {"type": "string"},
                          "minLength": {"type": "integer"},
                          "maxLength": {"type": "integer"},
                          "enum": {"type": "array"}
                        }
                      },
                      "transform": {
                        "type": "object",
                        "properties": {
                          "targetName": {"type": "string"},
                          "expression": {"type": "string"}
                        }
                      }
                    }
                  }
                },
                "response": {
                  "type": "object",
                  "properties": {
                    "type": {"enum": ["json", "text", "binary"]},
                    "transform": {"type": "string"},  // JQ expression for response transformation
                    "paginated": {"type": "boolean"},
                    "paginationConfig": {
                      "type": "object",
                      "properties": {
                        "nextPageTokenPath": {"type": "string"},
                        "dataPath": {"type": "string"},
                        "pageSize": {"type": "integer"}
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}
```

### Microsoft 365 Calendar Example Configuration

Note: The MCP tool accepts start and end dates in YYYYMMDD format, but the Microsoft Graph API requires ISO 8601 format. The configuration specifies how to transform the MCP tool parameters to API parameters.

```json
{
  "services": {
    "microsoft365": {
      "name": "Microsoft 365",
      "baseURL": "https://graph.microsoft.com/v1.0",
      "auth": {
        "type": "oauth2_device",
        "config": {
          "clientId": "${MS365_CLIENT_ID}",
          "tenantId": "${MS365_TENANT_ID}",
          "scope": ["https://graph.microsoft.com/Calendars.Read", "https://graph.microsoft.com/Mail.Read"],
          "authorizationURL": "https://login.microsoftonline.com/{tenantId}/oauth2/v2.0/devicecode",
          "tokenURL": "https://login.microsoftonline.com/{tenantId}/oauth2/v2.0/token"
        }
      },
      "endpoints": [
        {
          "id": "calendar_read_summary",
          "name": "Read Calendar Summary",
          "description": "Get calendar events with basic information (start, end, subject)",
          "method": "GET",
          "path": "/me/calendarView",
          "parameters": [
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
            },
            {
              "name": "endDate",
              "description": "End date in YYYYMMDD format",
              "type": "string",
              "required": true,
              "location": "query",
              "validation": {
                "pattern": "^\\d{8}$"
              },
              "transform": {
                "targetName": "endDateTime",
                "expression": "concat(slice(0,4), '-', slice(4,6), '-', slice(6,8), 'T23:59:59Z')"
              }
            }
          ],
          "response": {
            "type": "json",
            "transform": ".value | map({subject: .subject, start: .start.dateTime, end: .end.dateTime})"
          }
        },
        {
          "id": "calendar_read_details",
          "name": "Read Calendar Details",
          "description": "Get calendar events with full details",
          "method": "GET",
          "path": "/me/calendarView",
          "parameters": [
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
            },
            {
              "name": "endDate",
              "description": "End date in YYYYMMDD format",
              "type": "string",
              "required": true,
              "location": "query",
              "validation": {
                "pattern": "^\\d{8}$"
              },
              "transform": {
                "targetName": "endDateTime",
                "expression": "concat(slice(0,4), '-', slice(4,6), '-', slice(6,8), 'T23:59:59Z')"
              }
            },
            {
              "name": "$select",
              "description": "Fields to include in response",
              "type": "string",
              "required": false,
              "location": "query",
              "default": "subject,body,bodyPreview,organizer,attendees,start,end,location"
            }
          ],
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
      ]
    }
  }
}
```

## Core Structures

### Config Structure
```go
type Config struct {
    Logger      global.Logger
    Services    map[string]*ServiceConfig
    AuthManager *AuthManager
    HTTPClient  *http.Client
    Cache       *Cache
    ConfigPath  string
}

type ServiceConfig struct {
    Name      string                 `json:"name"`
    BaseURL   string                 `json:"baseURL"`
    Auth      AuthConfig             `json:"auth"`
    Endpoints []EndpointConfig       `json:"endpoints"`
}

type AuthConfig struct {
    Type   AuthType               `json:"type"`
    Config map[string]interface{} `json:"config"`
}

type EndpointConfig struct {
    ID          string              `json:"id"`
    Name        string              `json:"name"`
    Description string              `json:"description"`
    Method      string              `json:"method"`
    Path        string              `json:"path"`
    Parameters  []ParameterConfig   `json:"parameters"`
    Response    ResponseConfig      `json:"response"`
}
```

### Authentication System

```go
type AuthManager struct {
    strategies map[string]AuthStrategy
    tokens     map[string]*TokenInfo
    mu         sync.RWMutex
}

type AuthStrategy interface {
    Authenticate(ctx context.Context, config map[string]interface{}) (*TokenInfo, error)
    RefreshToken(ctx context.Context, tokenInfo *TokenInfo) (*TokenInfo, error)
}

type OAuth2DeviceFlowStrategy struct {
    httpClient *http.Client
    logger     global.Logger
}

// OAuth2 Device Flow Error for user action
type DeviceCodeError struct {
    VerificationURL string
    UserCode        string
    Message         string
}

func (e DeviceCodeError) Error() string {
    return fmt.Sprintf("Please visit %s and enter code: %s\n%s", 
        e.VerificationURL, e.UserCode, e.Message)
}
```

## Implementation Phases

### Phase 1: Core Foundation
1. Create package structure and core types
2. Implement JSON configuration loading with validation
3. Create functional options pattern setup
4. Implement basic logging and error handling

### Phase 2: Authentication System
1. Design AuthManager and AuthStrategy interfaces
2. Implement OAuth2 device flow strategy
3. Implement bearer token and API key strategies
4. Add token caching and refresh logic
5. Create user-friendly error messages for auth flows

### Phase 3: Dynamic Tool Generation
1. Implement RegisterTools() with dynamic generation
2. Create generic HTTP handler factory
3. Implement parameter validation and mapping
4. Add request building logic (path params, query params, body)
5. Implement parameter transformation (e.g., YYYYMMDD to ISO 8601)

### Phase 4: Request/Response Handling
1. Implement HTTP client with retries and timeouts
2. Add response transformation using JQ-like expressions
3. Implement pagination support
4. Add error handling and user-friendly messages

### Phase 5: Microsoft 365 Integration
1. Create Microsoft 365 configuration file
2. Test calendar summary and details endpoints
3. Add email inbox read endpoints
4. Handle Graph API specific requirements

### Phase 6: Google Integration
1. Create Google configuration file
2. Add Google Calendar endpoints
3. Add Gmail read endpoints
4. Test cross-service functionality

### Phase 7: Polish and Documentation
1. Add comprehensive error handling
2. Create example configurations
3. Write integration tests
4. Document configuration schema

## Usage Example

```go
// In main.go
configPath := flag.String("config", "", "Path to fusion configuration file")
flag.Parse()

if *configPath != "" {
    fusionProvider := fusion.New(
        fusion.WithJSONConfig(*configPath),
        fusion.WithLogger(logger),
    )
    server.AddToolProvider(fusionProvider)
}
```

## Error Handling Strategy

1. **Authentication Errors**: Return structured errors with user action required
2. **Configuration Errors**: Validate at startup, fail fast with clear messages
3. **API Errors**: Transform to user-friendly messages with retry guidance
4. **Network Errors**: Implement exponential backoff with status updates

## Security Considerations

1. **Token Storage**: Use in-memory cache with optional encrypted disk cache
2. **Environment Variables**: Support for sensitive values in env vars
3. **Scope Limitations**: Request minimal required scopes
4. **Token Refresh**: Automatic refresh before expiration
5. **Audit Logging**: Log all API access for security monitoring

## Testing Strategy

1. **Unit Tests**: Test each component in isolation
2. **Integration Tests**: Test with mock API servers
3. **Configuration Tests**: Validate various configuration scenarios
4. **Auth Flow Tests**: Test each authentication strategy
5. **Error Scenario Tests**: Ensure graceful handling of failures

## Future Enhancements

1. **Response Caching**: Cache responses based on configuration
2. **Rate Limiting**: Implement per-service rate limiting
3. **Webhook Support**: Add webhook endpoint configuration
4. **GraphQL Support**: Extend to support GraphQL APIs
5. **Custom Transformers**: Allow custom Go functions for response transformation
6. **Multi-tenancy**: Support multiple auth contexts per service

## Configuration Best Practices

1. Use environment variables for sensitive data
2. Validate configurations at startup
3. Provide clear error messages for misconfigurations
4. Include example configurations in the package
5. Document all available options in schema
6. Support configuration reloading without restart

This architecture provides a flexible, extensible system that can handle multiple APIs with different authentication methods while maintaining the clean patterns established in the MCPFusion codebase.