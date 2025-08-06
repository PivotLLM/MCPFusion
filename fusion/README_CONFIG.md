# Fusion Configuration Guide

## 🎯 Overview

This comprehensive guide covers all configuration options for the Fusion package, including authentication strategies, parameter transformations, response processing, and production deployment patterns.

## 📋 Table of Contents

1. [Configuration Loading and Validation](#configuration-loading-and-validation)
2. [Authentication Strategies](#authentication-strategies)
3. [Service Configuration](#service-configuration)
4. [Endpoint Configuration](#endpoint-configuration)
5. [Parameter Handling](#parameter-handling)
6. [Response Processing](#response-processing)
7. [Production Features](#production-features)
8. [Environment Variables](#environment-variables)
9. [Microsoft 365 Setup](#microsoft-365-setup)
10. [Google APIs Setup](#google-apis-setup)
11. [Troubleshooting](#troubleshooting)

## 📥 Configuration Loading and Validation

## Features

### 1. JSON Configuration Loading
- **File loading**: Load configurations from JSON files using `LoadConfigFromFile(filePath)`
- **Data loading**: Load configurations from JSON byte data using `LoadConfigFromJSON(data, configPath)`
- **Error handling**: Detailed error messages with proper error types for different failure scenarios

### 2. Environment Variable Expansion
The configuration system supports environment variable substitution with the following syntax:

- `${VAR_NAME}` - Expands to the value of environment variable VAR_NAME
- `${VAR_NAME:default_value}` - Expands to VAR_NAME value, or uses default_value if not set
- Variables without defaults that are not set remain unexpanded as `${VAR_NAME}`

#### Examples:
```json
{
  "services": {
    "api_service": {
      "baseURL": "${API_BASE_URL:https://api.example.com}",
      "auth": {
        "type": "bearer",
        "config": {
          "token": "${API_TOKEN}"
        }
      }
    }
  }
}
```

### 3. Configuration Validation
Comprehensive validation is performed on all configuration elements:

#### Service Level Validation
- Service name is required
- Base URL is required and must be valid
- Authentication configuration must be valid
- At least one endpoint is required

#### Authentication Validation
- **OAuth2 Device Flow**: Requires `clientId` and `tokenURL`
- **Bearer Token**: Requires either `token` or `tokenEnvVar`
- **API Key**: Requires either `apiKey` or `apiKeyEnvVar`
- **Basic Auth**: Requires both `username` and `password`

#### Endpoint Validation
- Endpoint ID, name, and description are required
- HTTP method must be valid (GET, POST, PUT, DELETE, PATCH)
- Path is required
- Parameters are validated for type, location, and validation rules
- Response configuration is validated

#### Parameter Validation
- Name and type are required
- Location must be valid (path, query, body, header)
- Validation rules are checked (regex patterns, length constraints, enum values)

### 4. Error Handling
The system uses structured error types for different scenarios:

- **ConfigurationError**: For configuration validation issues
- **ValidationError**: For parameter validation failures
- **File reading errors**: For file access issues
- **JSON parsing errors**: For malformed JSON

### 5. Utility Functions

#### Configuration Management
- `GetServiceByName(name)`: Find service by human-readable name
- `GetAllEndpoints()`: Get all endpoints with service context
- `ValidateServiceConfig(serviceName)`: Validate specific service
- `GetRequiredEnvironmentVariables()`: List all required environment variables
- `Clone()`: Create deep copy of configuration
- `MergeConfig(other)`: Merge another configuration

#### Service Configuration
- `GetEndpointByID(id)`: Find endpoint by ID within service
- `GetRequiredParameters()`: Get all required parameters for endpoint
- `GetParameterByName(name)`: Find parameter by name

#### Parameter Configuration
- `GetTransformedParameterName()`: Get target name if transform is configured
- `IsValidEnumValue(value)`: Check enum validation
- `MatchesPattern(value)`: Check regex pattern validation
- `IsValidLength(value)`: Check length constraints

## Usage Examples

### Basic Configuration Loading
```go
// Load from file
config, err := LoadConfigFromFile("config.json")
if err != nil {
    log.Fatalf("Failed to load config: %v", err)
}

// Validate specific service  
err = config.ValidateServiceConfig("google")
if err != nil {
    log.Fatalf("Invalid service config: %v", err)
}
```

### Environment Variable Usage
```go
// Set environment variables
os.Setenv("GOOGLE_TOKEN", "your-token-here")
os.Setenv("API_BASE", "https://custom-api.com")

// Load config with variable expansion
config, err := LoadConfigFromFile("config.json")
// Variables will be automatically expanded
```

### Configuration Utilities
```go
// Get all required environment variables
requiredVars := config.GetRequiredEnvironmentVariables()
for _, varName := range requiredVars {
    if os.Getenv(varName) == "" {
        log.Printf("Warning: Required environment variable %s is not set", varName)
    }
}

// Find service by name
service := config.GetServiceByName("Google APIs")
if service != nil {
    fmt.Printf("Found service with %d endpoints\n", len(service.Endpoints))
}

// Get all endpoints across all services
allEndpoints := config.GetAllEndpoints()
for _, ep := range allEndpoints {
    fmt.Printf("Service: %s, Endpoint: %s\n", ep.ServiceName, ep.Endpoint.Name)
}
```

## Schema Validation

The configuration follows a JSON schema defined in `configs/schema.json`. The schema ensures:

- Required fields are present
- Data types are correct
- Enum values are valid
- URL formats are proper
- Array constraints are met

## Error Messages

The system provides detailed error messages for common issues:

- **Missing required fields**: "service name is required"
- **Invalid authentication**: "bearer auth requires either token or tokenEnvVar"
- **Invalid HTTP methods**: "invalid HTTP method: INVALID"
- **Validation failures**: "validation failed for parameter x: message"
- **Environment variable issues**: "failed to expand environment variables"

## Testing

Comprehensive tests cover:

- Valid and invalid JSON parsing
- Environment variable expansion with various scenarios
- Configuration validation for all supported auth types
- File loading success and failure cases
- Integration tests with real configuration files
- Utility function behavior
- Error handling and structured error types

The test suite includes over 40 test cases ensuring robust configuration handling.

---

# 🔐 Authentication Strategies

## OAuth2 Device Flow (Microsoft 365, Google)

```json
{
  "auth": {
    "type": "oauth2_device",
    "config": {
      "clientId": "${MS365_CLIENT_ID}",
      "tenantId": "${MS365_TENANT_ID}",
      "scope": [
        "https://graph.microsoft.com/User.Read",
        "https://graph.microsoft.com/Calendars.ReadWrite",
        "https://graph.microsoft.com/Mail.Read"
      ],
      "authorizationURL": "https://login.microsoftonline.com/{tenantId}/oauth2/v2.0/devicecode",
      "tokenURL": "https://login.microsoftonline.com/{tenantId}/oauth2/v2.0/token",
      "pollInterval": 5,
      "expiresIn": 900,
      "tokenRefreshBuffer": "5m"
    }
  }
}
```

## Bearer Token Authentication

```json
{
  "auth": {
    "type": "bearer",
    "config": {
      "tokenEnvVar": "API_TOKEN",
      "refreshToken": "${REFRESH_TOKEN}",
      "refreshTokenEnvVar": "REFRESH_TOKEN",
      "tokenURL": "https://api.example.com/refresh",
      "expiryBuffer": "5m"
    }
  }
}
```

## API Key Authentication

```json
{
  "auth": {
    "type": "api_key",
    "config": {
      "apiKeyEnvVar": "API_KEY",
      "headerName": "X-API-Key",
      "queryParam": "api_key",
      "prefix": "ApiKey "
    }
  }
}
```

# ⚙️ Service Configuration

## Complete Production Service

```json
{
  "production_service": {
    "name": "Production API Service",
    "baseURL": "https://api.production.com/v1",
    "auth": { /* auth config */ },
    "retryConfig": {
      "strategy": "exponential",
      "maxAttempts": 5,
      "baseDelay": "2s",
      "maxDelay": "60s",
      "jitter": true,
      "retryableErrors": ["NetworkError", "ServerError", "RateLimitError"]
    },
    "circuitBreaker": {
      "enabled": true,
      "failureThreshold": 10,
      "recoveryTimeout": "120s",
      "halfOpenMaxRequests": 5
    },
    "defaultCaching": {
      "enabled": true,
      "ttl": "10m",
      "maxSize": "5MB"
    },
    "endpoints": [/* endpoint configs */]
  }
}
```

# 🎯 Endpoint Configuration

## Advanced Endpoint with All Features

```json
{
  "id": "advanced_endpoint",
  "name": "Advanced API Endpoint", 
  "description": "Complete endpoint with validation, transformation, caching",
  "method": "POST",
  "path": "/api/v1/resources/{resourceId}",
  "parameters": [
    {
      "name": "resourceId",
      "type": "string",
      "required": true,
      "location": "path",
      "validation": {
        "pattern": "^[a-zA-Z0-9-]+$",
        "minLength": 1,
        "maxLength": 50
      }
    },
    {
      "name": "startDate",
      "type": "string",
      "required": true,
      "location": "query",
      "validation": {"pattern": "^\\d{8}$"},
      "transform": {
        "targetName": "start_datetime",
        "expression": "concat(slice(0,4), '-', slice(4,6), '-', slice(6,8), 'T00:00:00Z')"
      }
    }
  ],
  "response": {
    "type": "json",
    "transform": ".data | map({id: .id, name: .name, status: .status})",
    "paginated": true,
    "paginationConfig": {
      "nextPageTokenPath": "pagination.nextToken",
      "dataPath": "data.items",
      "pageSize": 100
    },
    "caching": {
      "enabled": true,
      "ttl": "5m",
      "keyTemplate": "endpoint_{resourceId}_{hash}"
    }
  }
}
```

# 🔧 Parameter Transformations

## Date Format Transformations

```json
{
  "name": "eventDate",
  "transform": {
    "targetName": "datetime",
    "expression": "concat(slice(0,4), '-', slice(4,6), '-', slice(6,8), 'T', slice(8,10), ':', slice(10,12), ':00Z')"
  }
}
// Input: "20241225143000" → Output: "2024-12-25T14:30:00Z"
```

## Conditional Transformations

```json
{
  "name": "status",
  "transform": {
    "expression": "if(. == 'on') then 'active' else if(. == 'off') then 'inactive' else . end"
  }
}
```

# 📊 Response Processing

## Pagination Configuration

```json
{
  "response": {
    "paginated": true,
    "paginationConfig": {
      "nextPageTokenPath": "@odata.nextLink",  // Microsoft Graph
      "dataPath": "value",
      "pageSize": 100,
      "maxPages": 20,
      "mergeStrategy": "concat"
    }
  }
}
```

## Response Caching

```json
{
  "caching": {
    "enabled": true,
    "ttl": "15m",
    "keyTemplate": "{service}_{endpoint}_{hash}",
    "invalidateOn": ["create_resource", "update_resource"],
    "compression": true,
    "maxSize": "1MB"
  }
}
```

# 🌍 Environment Variables

## Production Environment Setup

```bash
# Core Settings
FUSION_LOG_LEVEL=info
FUSION_TIMEOUT=30s
FUSION_MAX_RETRIES=3
FUSION_CACHE_ENABLED=true
FUSION_METRICS_ENABLED=true
FUSION_CIRCUIT_BREAKER_ENABLED=true

# Microsoft 365
MS365_CLIENT_ID=your-client-id
MS365_TENANT_ID=your-tenant-id

# Google APIs
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret

# Security
FUSION_TOKEN_ENCRYPTION=true
FUSION_TLS_VERIFY=true
```

# 📧 Microsoft 365 Setup

## Azure App Registration

1. Go to Azure Portal → App registrations → New registration
2. Name: "MCPFusion Integration"
3. Redirect URI: "Public client/native (mobile & desktop)"
4. URI: `https://login.microsoftonline.com/common/oauth2/nativeclient`
5. Copy Application (client) ID and Directory (tenant) ID

## Required API Permissions

- `User.Read` - Sign in and read user profile
- `Calendars.ReadWrite` - Manage user calendars
- `Mail.Read` - Read user mail
- `Contacts.Read` - Read user contacts

## Microsoft 365 Configuration

```json
{
  "services": {
    "microsoft365": {
      "name": "Microsoft 365 Graph API",
      "baseURL": "https://graph.microsoft.com/v1.0",
      "auth": {
        "type": "oauth2_device",
        "config": {
          "clientId": "${MS365_CLIENT_ID}",
          "tenantId": "${MS365_TENANT_ID}",
          "scope": [
            "https://graph.microsoft.com/User.Read",
            "https://graph.microsoft.com/Calendars.ReadWrite",
            "https://graph.microsoft.com/Mail.Read"
          ],
          "authorizationURL": "https://login.microsoftonline.com/{tenantId}/oauth2/v2.0/devicecode",
          "tokenURL": "https://login.microsoftonline.com/{tenantId}/oauth2/v2.0/token"
        }
      },
      "endpoints": [
        {
          "id": "calendar_events",
          "name": "Get Calendar Events",
          "method": "GET",
          "path": "/me/calendarView",
          "parameters": [
            {
              "name": "startDate",
              "type": "string",
              "required": true,
              "location": "query",
              "transform": {
                "targetName": "startDateTime", 
                "expression": "concat(slice(0,4), '-', slice(4,6), '-', slice(6,8), 'T00:00:00Z')"
              }
            }
          ]
        }
      ]
    }
  }
}
```

# 🔍 Google APIs Setup

## Google Cloud Console

1. Go to Google Cloud Console → APIs & Services → Credentials
2. Create OAuth 2.0 Client ID
3. Application type: "Desktop application"
4. Name: "MCPFusion Integration"
5. Copy Client ID and Client Secret

## Enable Required APIs

- Google Calendar API
- Gmail API
- Google Drive API
- Google People API

## Google APIs Configuration

```json
{
  "services": {
    "google": {
      "name": "Google APIs",
      "baseURL": "https://www.googleapis.com",
      "auth": {
        "type": "oauth2_device",
        "config": {
          "clientId": "${GOOGLE_CLIENT_ID}",
          "clientSecret": "${GOOGLE_CLIENT_SECRET}",
          "scope": [
            "https://www.googleapis.com/auth/calendar",
            "https://www.googleapis.com/auth/gmail.readonly",
            "https://www.googleapis.com/auth/drive.readonly"
          ],
          "authorizationURL": "https://oauth2.googleapis.com/device/code",
          "tokenURL": "https://oauth2.googleapis.com/token"
        }
      },
      "endpoints": [
        {
          "id": "calendar_events",
          "name": "List Calendar Events",
          "method": "GET", 
          "path": "/calendar/v3/calendars/primary/events",
          "response": {
            "type": "json",
            "paginated": true,
            "paginationConfig": {
              "nextPageTokenPath": "nextPageToken",
              "nextPageTokenParam": "pageToken",
              "dataPath": "items"
            }
          }
        }
      ]
    }
  }
}
```

# 🔧 Troubleshooting

## Common Issues

### OAuth2 Device Flow Problems

```bash
# Test device code endpoint
curl -X POST "https://login.microsoftonline.com/${TENANT_ID}/oauth2/v2.0/devicecode" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "client_id=${CLIENT_ID}&scope=https://graph.microsoft.com/User.Read"
```

### Parameter Validation Issues

```json
// Enable debug mode for parameter processing
{
  "parameters": [
    {
      "name": "date",
      "debug": true,
      "transform": {
        "debugOutput": true
      }
    }
  ]
}
```

### Cache Problems

```bash
# Check cache permissions
ls -la ~/.cache/mcpfusion/

# Clear cache
rm -rf ~/.cache/mcpfusion/*

# Test without cache
curl -H "Cache-Control: no-cache" http://localhost:8080/api/endpoint
```

## Debug Configuration

```bash
# Enable debug mode
export FUSION_DEBUG_MODE=true
export FUSION_LOG_LEVEL=debug

# Run with debug output
go run . -debug -config=config.json 2>&1 | tee debug.log
```

## Configuration Validation

```bash
# Validate configuration
curl -X POST http://localhost:8080/admin/validate-config \
  -H "Content-Type: application/json" \
  -d @config.json

# Check service health
curl http://localhost:8080/health
```

---

📧 **Need Help?** Open an issue with your configuration for assistance.

📖 **More Examples?** See the [examples directory](examples/) for complete working configurations.