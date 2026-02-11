# MCPFusion Configuration Guide

This guide explains how to create JSON configuration files for MCPFusion to integrate with any API that uses OAuth2 or Bearer token authentication.

## Table of Contents

- [Overview](#overview)
- [Configuration Structure](#configuration-structure)
- [Authentication Types](#authentication-types)
- [Endpoint Configuration](#endpoint-configuration)
- [Parameter Configuration](#parameter-configuration)
- [Response Configuration](#response-configuration)
- [Advanced Features](#advanced-features)
- [HTTP Session Management](#http-session-management)
- [Best Practices](#best-practices)
- [Complete Examples](#complete-examples)
- [Troubleshooting](#troubleshooting)

## Overview

MCPFusion uses JSON configuration files to dynamically create MCP (Model Context Protocol) tools from API specifications. Each configuration file defines:

- **Services**: API endpoints grouped by service (e.g., Microsoft 365, Google APIs)
- **Authentication**: How to authenticate with the API
- **Endpoints**: Individual API endpoints with parameters and responses
- **Validation**: Parameter validation rules and constraints
- **Caching**: Response caching strategies
- **Retry Logic**: Error handling and retry behavior

## Configuration Structure

### Root Structure

```json
{
  "services": {
    "service_name": {
      "name": "Human-readable service name",
      "baseURL": "https://api.example.com/v1",
      "auth": { /* Authentication configuration */ },
      "retry": { /* Optional retry configuration */ },
      "circuitBreaker": { /* Optional circuit breaker configuration */ },
      "endpoints": [ /* Array of endpoint configurations */ ]
    }
  }
}
```

### Service Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Human-readable service name displayed in MCP tools |
| `baseURL` | string | Yes | Base URL for all API endpoints in this service |
| `auth` | object | Yes | Authentication configuration |
| `endpoints` | array | Yes | Array of endpoint configurations |
| `retry` | object | No | Service-level retry configuration |
| `circuitBreaker` | object | No | Circuit breaker configuration |

## Authentication Types

### OAuth2 Device Flow

Best for desktop applications and command-line tools:

```json
{
  "type": "oauth2_device",
  "config": {
    "clientId": "${CLIENT_ID}",
    "tenantId": "${TENANT_ID}",
    "scope": "openid profile User.Read Calendars.Read",
    "authorizationURL": "https://login.microsoftonline.com/{tenantId}/oauth2/v2.0/devicecode",
    "tokenURL": "https://login.microsoftonline.com/{tenantId}/oauth2/v2.0/token"
  }
}
```

**Environment Variables Required:**
- `CLIENT_ID`: Your application's client ID
- `TENANT_ID`: The tenant ID (for Microsoft APIs)

### OAuth2 External

For services where OAuth2 tokens are provided by an external helper (e.g., `fusion-auth`). Use this when the provider doesn't support device flow or restricts its scopes. The strategy uses stored tokens and supports automatic token refresh, but does not initiate any interactive authentication flow — if no token is found, it returns an error directing the user to run `fusion-auth`.

```json
{
  "type": "oauth2_external",
  "config": {
    "clientId": "${CLIENT_ID}",
    "clientSecret": "${CLIENT_SECRET}",
    "scope": "scope1 scope2",
    "tokenURL": "https://provider.example.com/token"
  }
}
```

**Environment Variables Required:**
- `CLIENT_ID`: Your application's client ID
- `CLIENT_SECRET`: Your application's client secret

**Config Fields:**
- `clientId` (required): OAuth2 client ID
- `clientSecret` (optional but recommended): OAuth2 client secret, included in token refresh requests
- `tokenURL` (required): Token endpoint URL for refreshing tokens
- `scope` (optional): Space-separated OAuth2 scopes, included in refresh requests if set

### Bearer Token

For APIs that use static bearer tokens:

```json
{
  "type": "bearer",
  "config": {
    "token": "${API_TOKEN}"
  }
}
```

**Environment Variables Required:**
- `API_TOKEN`: Your API bearer token

### API Key

For APIs that use API keys:

```json
{
  "type": "api_key",
  "config": {
    "key": "${API_KEY}",
    "header": "X-API-Key"
  }
}
```

**Environment Variables Required:**
- `API_KEY`: Your API key

### Basic Authentication

For APIs that use username/password:

```json
{
  "type": "basic",
  "config": {
    "username": "${API_USERNAME}",
    "password": "${API_PASSWORD}"
  }
}
```

**Environment Variables Required:**
- `API_USERNAME`: Username for basic auth
- `API_PASSWORD`: Password for basic auth

## Endpoint Configuration

### Basic Endpoint Structure

```json
{
  "id": "unique_endpoint_id",
  "name": "Human Readable Name",
  "description": "Description shown to LLM",
  "method": "GET",
  "path": "/api/resource",
  "baseURL": "https://alternative-api.example.com",
  "parameters": [ /* Parameter definitions */ ],
  "response": { /* Response configuration */ },
  "retry": { /* Optional endpoint-specific retry */ }
}
```

### Endpoint Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Unique identifier within service |
| `name` | string | Yes | Human-readable name |
| `description` | string | Yes | Description shown to LLM |
| `method` | string | Yes | HTTP method (GET, POST, PUT, DELETE, PATCH) |
| `path` | string | Yes | API path (may include {placeholders}) |
| `baseURL` | string | No | Overrides the service-level `baseURL` for this endpoint. Useful when a service spans multiple API hosts (e.g., Google APIs use `www.googleapis.com` for most services but `people.googleapis.com` for contacts). |
| `parameters` | array | No | Array of parameter definitions |
| `response` | object | No | Response handling configuration |
| `retry` | object | No | Endpoint-specific retry override |

### Path Parameters

Use curly braces for path placeholders:

```json
{
  "path": "/users/{userId}/posts/{postId}",
  "parameters": [
    {
      "name": "userId",
      "location": "path",
      "required": true,
      "type": "string"
    },
    {
      "name": "postId", 
      "location": "path",
      "required": true,
      "type": "string"
    }
  ]
}
```

## Parameter Configuration

### Parameter Structure

```json
{
  "name": "parameter_name",
  "description": "Parameter description for LLM",
  "type": "string",
  "required": true,
  "location": "query",
  "default": "default_value",
  "examples": ["example1", "example2"],
  "validation": { /* Validation rules */ },
  "transform": { /* Parameter transformation */ }
}
```

### Parameter Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Actual API parameter name (can include special characters) |
| `alias` | string | No | **MCP-compliant alias name** (overrides auto-sanitization) |
| `description` | string | Yes | Description shown to LLM |
| `type` | string | Yes | Parameter type (see types below) |
| `required` | boolean | Yes | Whether parameter is required |
| `location` | string | Yes | Where parameter goes (see locations below) |
| `default` | any | No | Default value if not provided |
| `examples` | array | No | Example values for LLM |
| `validation` | object | No | Validation rules |
| `transform` | object | No | Parameter transformation rules |
| `quoted` | boolean | No | Whether to quote the parameter value |
| `static` | boolean | No | **Static parameter** - not exposed to MCP, always uses default value |

### ⚠️ **MCP Parameter Naming Requirements**

**MCP Specification**: Parameter names must match `^[a-zA-Z0-9_.-]{1,64}$`
- ✅ **Valid**: `select`, `user_id`, `api-key`, `filter.name`
- ❌ **Invalid**: `$select`, `user id`, `@param`, `100%complete`

**Solution Options**:
1. **Use `alias` field** (recommended): Provides explicit control
2. **Auto-sanitization**: System removes invalid characters (logs warning)

**Configuration Examples**:
```json
// Option 1: Explicit alias (recommended)
{
  "name": "$select",
  "alias": "select", 
  "description": "OData select fields",
  "type": "string"
}

// Option 2: Auto-sanitization (will sanitize $filter → filter)
{
  "name": "$filter",
  "description": "OData filter expression", 
  "type": "string"
}
```

**System Behavior**:
- **With alias**: Uses alias name for MCP, logs INFO message
- **Without alias**: Auto-sanitizes name, logs WARNING to add explicit alias
- **API calls**: Always uses original `name` value for actual API requests
- **Conflicts**: System validates no two parameters map to same MCP name |

### Parameter Types

| Type | Description | MCP Schema |
|------|-------------|------------|
| `string` | Text values | `type: string` |
| `number` | Numeric values (int/float) | `type: number` |
| `boolean` | True/false values | `type: boolean` |
| `array` | Array of values | `type: array` |
| `object` | JSON objects | `type: object` |

### Parameter Locations

| Location | Description | Usage |
|----------|-------------|-------|
| `path` | URL path segment | `/users/{id}` |
| `query` | Query string parameter | `?name=value` |
| `header` | HTTP header | `X-Custom-Header: value` |
| `body` | Request body field | JSON body for POST/PUT |

### Validation Rules

```json
{
  "validation": {
    "pattern": "^\\d{8}$",
    "minLength": 1,
    "maxLength": 100,
    "minimum": 0.0,
    "maximum": 1000.0,
    "enum": ["option1", "option2", "option3"],
    "format": "email"
  }
}
```

**Validation Fields:**
- `pattern`: Regular expression for string validation
- `minLength`/`maxLength`: String length constraints
- `minimum`/`maximum`: Numeric range constraints  
- `enum`: List of valid values
- `format`: Format hint (email, date, uri, etc.)

### Static Parameters

Static parameters are fixed values that are not exposed to MCP clients. They are always sent with their default value and cannot be overridden by users. This is useful for:
- API keys or client IDs that should be fixed per deployment
- API version parameters that should remain constant
- Service-specific configuration values

**Configuration Example:**
```json
{
  "parameters": [
    {
      "name": "client_id",
      "description": "Application client ID",
      "type": "string",
      "required": false,
      "location": "query",
      "default": "${CLIENT_ID}",
      "static": true  // This parameter is not exposed to MCP
    },
    {
      "name": "api_version",
      "description": "API version",
      "type": "string", 
      "required": false,
      "location": "header",
      "default": "v2.0",
      "static": true  // Always uses "v2.0"
    }
  ]
}
```

**Important Notes:**
- Static parameters **MUST** have a `default` value
- They are not visible in the MCP tool schema
- Users cannot provide or override these values
- Combine with environment variables for deployment-specific fixed values
- Set `required: false` since the value comes from the default

### Parameter Transformation

Transform parameter values before sending to API:

```json
{
  "transform": {
    "targetName": "actual_api_parameter_name",
    "expression": "transformation_rule"
  }
}
```

**Built-in Transformations:**
- `toString`: Convert to string
- `toInt`: Convert to integer
- `toFloat`: Convert to float
- `toLowerCase`: Convert to lowercase
- `toUpperCase`: Convert to uppercase
- `trim`: Remove whitespace

**Date Transformation Example:**
```json
{
  "name": "startDate",
  "type": "string",
  "validation": {
    "pattern": "^\\d{8}$"
  },
  "transform": {
    "targetName": "startDateTime",
    "expression": "concat(slice(0,4), '-', slice(4,6), '-', slice(6,8), 'T00:00:00Z')"
  }
}
```

## Response Configuration

### Basic Response Configuration

```json
{
  "response": {
    "type": "json",
    "transform": "$.data",
    "paginated": true,
    "paginationConfig": {
      "nextPageTokenPath": "@odata.nextLink",
      "dataPath": "value",
      "pageSize": 50
    },
    "caching": {
      "enabled": true,
      "ttl": "10m",
      "key": "custom-cache-key-{param1}"
    }
  }
}
```

### Response Types

| Type | Description |
|------|-------------|
| `json` | JSON response (default) |
| `text` | Plain text response |
| `binary` | Binary data response |

### Pagination Configuration

For APIs that return paginated results:

```json
{
  "paginated": true,
  "paginationConfig": {
    "nextPageTokenPath": "@odata.nextLink",
    "dataPath": "value", 
    "pageSize": 50
  }
}
```

**Pagination Fields:**
- `nextPageTokenPath`: JSON path to next page URL/token
- `dataPath`: JSON path to array of items
- `pageSize`: Items per page (for metrics)

### Caching Configuration

```json
{
  "caching": {
    "enabled": true,
    "ttl": "10m",
    "key": "service:endpoint:{param1}:{param2}"
  }
}
```

**Caching Fields:**
- `enabled`: Whether to cache responses
- `ttl`: Time-to-live (e.g., "5m", "1h", "30s")
- `key`: Custom cache key template (optional)

## Advanced Features

### Retry Configuration

Service-level or endpoint-level retry configuration:

```json
{
  "retry": {
    "enabled": true,
    "maxAttempts": 3,
    "strategy": "exponential",
    "baseDelay": "1s",
    "maxDelay": "30s",
    "jitter": true,
    "backoffFactor": 2.0,
    "retryableErrors": ["network_error", "timeout", "server_error", "rate_limited"]
  }
}
```

### Circuit Breaker Configuration

Protect against cascading failures:

```json
{
  "circuitBreaker": {
    "enabled": true,
    "failureThreshold": 5,
    "successThreshold": 3,
    "timeout": "30s",
    "halfOpenMaxCalls": 3,
    "resetTimeout": "60s"
  }
}
```

## HTTP Session Management

MCPFusion includes advanced HTTP session management to handle connection timeouts and improve reliability with external APIs. This is particularly useful for APIs that may have intermittent connectivity issues or strict connection limits.

### Connection Configuration

You can configure connection behavior at the endpoint level to handle problematic APIs:

```json
{
  "id": "microsoft365_mail_search",
  "name": "Search emails in Microsoft 365",
  "method": "GET",
  "path": "/me/messages",
  "connection": {
    "disableKeepAlive": true,
    "forceNewConnection": false,
    "timeout": "45s"
  },
  "parameters": [...]
}
```

### Connection Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `disableKeepAlive` | boolean | false | Forces connection closure after each request (adds `Connection: close` header) |
| `forceNewConnection` | boolean | false | Creates a new connection for each request, bypassing connection pool |
| `timeout` | string | "60s" | Custom timeout for this specific endpoint (format: "30s", "2m", etc.) |

### Default Transport Settings

MCPFusion uses optimized HTTP transport settings:

- **Connection Limits**: 100 total idle connections, 10 per host, 50 max per host
- **Timeouts**: 30s idle timeout, 10s connection establishment, 60s overall request timeout
- **Keep-Alive**: 30s probe interval with automatic health validation
- **Automatic Cleanup**: Periodic cleanup of idle connections every 5 minutes

### When to Use Connection Control

**Use `disableKeepAlive: true` when:**
- API servers close connections unpredictably
- You experience frequent timeout errors
- The API has strict connection limits

**Use `forceNewConnection: true` when:**
- Connection reuse causes authentication issues
- API requires fresh connections for each request
- Debugging connection-related problems

**Use custom `timeout` when:**
- Endpoint typically takes longer than 60s to respond
- API has known slow response times
- Need tighter timeout control for specific operations

### Example: Microsoft 365 Configuration

```json
{
  "id": "microsoft365_mail_search",
  "name": "Search Microsoft 365 emails",
  "method": "GET",
  "path": "/me/messages",
  "connection": {
    "disableKeepAlive": true,
    "timeout": "45s"
  },
  "parameters": [
    {
      "name": "$filter",
      "alias": "filter",
      "description": "OData filter expression",
      "type": "string",
      "required": false,
      "location": "query"
    }
  ]
}
```

### Monitoring Connection Health

MCPFusion automatically logs connection management activities:

```
[DEBUG] Timeout detected, triggering connection cleanup [correlation-id]
[DEBUG] Connection error detected, triggering connection cleanup [correlation-id]
[DEBUG] Cleaned up idle HTTP connections
```

Monitor these logs to identify endpoints that may benefit from connection configuration.

## Best Practices

### 1. Environment Variables

**Always use environment variables for secrets:**

```json
{
  "auth": {
    "type": "oauth2_device",
    "config": {
      "clientId": "${CLIENT_ID}",
      "clientSecret": "${CLIENT_SECRET}"
    }
  }
}
```

**Set environment variables in `~/.mcp` file:**
```bash
CLIENT_ID=your-client-id
CLIENT_SECRET=your-client-secret
API_BASE_URL=https://api.example.com
```

### 2. Parameter Design

**Provide sensible defaults:**
```json
{
  "name": "$top",
  "type": "number",
  "default": 25,
  "validation": {
    "enum": [5, 10, 25, 50, 100, 250, 500, 1000]
  }
}
```

**Use descriptive names and descriptions:**
```json
{
  "name": "startDate",
  "description": "Start date in YYYYMMDD format",
  "validation": {
    "pattern": "^\\d{8}$"
  }
}
```

### 3. Caching Strategy

**Cache stable data longer:**
```json
{
  "caching": {
    "enabled": true,
    "ttl": "1h"  // User profiles change infrequently
  }
}
```

**Cache dynamic data briefly:**
```json
{
  "caching": {
    "enabled": true,
    "ttl": "2m"  // Messages change frequently
  }
}
```

### 4. Error Handling

**Configure appropriate retry for different endpoints:**
```json
{
  "retry": {
    "enabled": true,
    "maxAttempts": 5,     // More retries for critical operations
    "strategy": "exponential"
  }
}
```

### 5. Tool Naming

**Use consistent naming patterns:**
- `service_resource_action`: `microsoft365_calendar_read_summary`
- `service_resource_action_modifier`: `microsoft365_mail_folder_messages`

### **Parameter Naming Best Practices**

#### **Rule 1: Always Use Aliases for Special Characters**

When your API uses parameters that violate MCP naming rules, always add explicit aliases:

```json
// ❌ Bad: Will auto-sanitize with warnings
{
  "name": "$select",
  "description": "OData select fields",
  "type": "string"
}

// ✅ Good: Explicit alias with clear intent
{
  "name": "$select", 
  "alias": "select",
  "description": "OData select fields (comma-separated)",
  "type": "string",
  "examples": ["displayName,mail", "id,displayName,mail,userPrincipalName"]
}
```

#### **Rule 2: Choose Meaningful Alias Names**

Use descriptive, LLM-friendly alias names:

```json
// ❌ Poor: Ambiguous alias
{
  "name": "$top",
  "alias": "t",
  "type": "number"
}

// ✅ Good: Clear, descriptive alias  
{
  "name": "$top",
  "alias": "limit",
  "description": "Maximum number of items to return",
  "type": "number",
  "default": 10,
  "examples": [10, 25, 50, 100]
}
```

#### **Rule 3: Handle Common API Patterns**

**Microsoft Graph / OData APIs:**
```json
{
  "name": "$select", "alias": "select",
  "name": "$filter", "alias": "filter", 
  "name": "$orderby", "alias": "orderby",
  "name": "$top", "alias": "limit",
  "name": "$skip", "alias": "offset",
  "name": "$expand", "alias": "expand",
  "name": "$search", "alias": "search",
  "name": "$count", "alias": "include_count"
}
```

**APIs with Spaces:**
```json
{
  "name": "user name", 
  "alias": "user_name",
  "name": "file size",
  "alias": "file_size"
}
```

**APIs with Special Characters:**
```json
{
  "name": "query[filters]",
  "alias": "filters",
  "name": "@timestamp", 
  "alias": "timestamp",
  "name": "100%complete",
  "alias": "completion_percent"
}
```

#### **Rule 4: Add Rich Descriptions and Examples**

Make your parameters LLM-friendly with enhanced descriptions:

```json
{
  "name": "$filter",
  "alias": "filter", 
  "description": "OData filter expression to narrow results. Supports eq, ne, gt, lt, ge, le, and, or, not operators. Common patterns: startswith(field,'value'), contains(field,'value'), field eq 'value'",
  "type": "string",
  "required": false,
  "location": "query",
  "examples": [
    "startswith(displayName,'John')",
    "mail eq 'user@example.com'", 
    "createdDateTime ge 2024-01-01T00:00:00Z",
    "startswith(displayName,'A') and department eq 'Engineering'"
  ]
}
```

#### **Rule 5: Validate No Conflicts**

Ensure no two parameters map to the same MCP name:

```json
// ❌ Bad: Both map to "filter"
[
  {"name": "$filter", "alias": "filter"},
  {"name": "search_filter", "alias": "filter"}  // CONFLICT!
]

// ✅ Good: Unique aliases
[
  {"name": "$filter", "alias": "odata_filter"},
  {"name": "search_filter", "alias": "search_filter"}
]
```

#### **System Behavior Summary**

| Scenario | MCP Name | Log Level | Recommendation |
|----------|----------|-----------|----------------|
| `{"name": "validParam"}` | `validParam` | None | ✅ No action needed |
| `{"name": "$select", "alias": "select"}` | `select` | INFO | ✅ Best practice |
| `{"name": "$filter"}` | `filter` | WARNING | ⚠️ Add explicit alias |
| `{"name": "$select"}, {"name": "select"}` | Conflict! | ERROR | ❌ Fix naming conflict |

#### **LLM Guidance Template**

When creating configurations, include this guidance for LLMs:

```json
{
  "// IMPORTANT": "Parameter names must match ^[a-zA-Z0-9_.-]{1,64}$",
  "// RULE 1": "If API parameter contains $, @, %, spaces, or special chars, add 'alias' field",
  "// RULE 2": "Choose descriptive alias names (e.g., $top -> limit, $filter -> filter)",  
  "// RULE 3": "Always include examples and detailed descriptions",
  "// RULE 4": "Verify no two parameters map to same alias name"
}
```

## Complete Examples

### API with Static Parameters

This example shows how to use static parameters for fixed API configuration:

```json
{
  "services": {
    "enterprise_api": {
      "name": "Enterprise API Service",
      "baseURL": "https://api.enterprise.com",
      "auth": {
        "type": "oauth2_device",
        "config": {
          "clientId": "${CLIENT_ID}",
          "scope": "read write",
          "authorizationURL": "https://auth.enterprise.com/device",
          "tokenURL": "https://auth.enterprise.com/token"
        }
      },
      "endpoints": [
        {
          "id": "data_query",
          "name": "Query Data",
          "description": "Query enterprise data with fixed client context",
          "method": "POST",
          "path": "/api/query",
          "parameters": [
            {
              "name": "client_id",
              "description": "Client application ID",
              "type": "string",
              "required": false,
              "location": "header",
              "default": "${APP_CLIENT_ID}",
              "static": true  // Fixed per deployment
            },
            {
              "name": "api_version",
              "description": "API version",
              "type": "string",
              "required": false,
              "location": "header",
              "default": "2024-01-01",
              "static": true  // Fixed version
            },
            {
              "name": "tenant_id",
              "description": "Tenant identifier",
              "type": "string",
              "required": false,
              "location": "query",
              "default": "${TENANT_ID}",
              "static": true  // Fixed per deployment
            },
            {
              "name": "query",
              "description": "The query to execute",
              "type": "string",
              "required": true,
              "location": "body"  // User-provided parameter
            },
            {
              "name": "limit",
              "description": "Maximum results to return",
              "type": "number",
              "required": false,
              "location": "query",
              "default": 100  // User can override this
            }
          ]
        }
      ]
    }
  }
}
```

In this example:
- `client_id`, `api_version`, and `tenant_id` are static parameters that are always sent with fixed values
- These static parameters are not exposed in the MCP tool interface
- `query` and `limit` are regular parameters that users can provide
- Static parameters use environment variables for deployment-specific configuration

### Simple REST API with Bearer Token

```json
{
  "services": {
    "jsonplaceholder": {
      "name": "JSONPlaceholder API",
      "baseURL": "https://jsonplaceholder.typicode.com",
      "auth": {
        "type": "bearer",
        "config": {
          "token": "${API_TOKEN}"
        }
      },
      "endpoints": [
        {
          "id": "posts_list",
          "name": "List Posts",
          "description": "Get all posts",
          "method": "GET",
          "path": "/posts",
          "parameters": [
            {
              "name": "$top",
              "description": "Number of posts to retrieve",
              "type": "number",
              "required": false,
              "location": "query",
              "default": 10,
              "validation": {
                "minimum": 1,
                "maximum": 100
              }
            }
          ],
          "response": {
            "type": "json",
            "caching": {
              "enabled": true,
              "ttl": "5m"
            }
          }
        },
        {
          "id": "posts_read",
          "name": "Read Post",
          "description": "Get a specific post by ID",
          "method": "GET", 
          "path": "/posts/{id}",
          "parameters": [
            {
              "name": "id",
              "description": "Post ID to retrieve",
              "type": "number",
              "required": true,
              "location": "path"
            }
          ],
          "response": {
            "type": "json",
            "caching": {
              "enabled": true,
              "ttl": "10m"
            }
          }
        }
      ]
    }
  }
}
```

### OAuth2 API with Pagination

```json
{
  "services": {
    "github": {
      "name": "GitHub API",
      "baseURL": "https://api.github.com",
      "auth": {
        "type": "oauth2_device",
        "config": {
          "clientId": "${GITHUB_CLIENT_ID}",
          "scope": "repo user",
          "authorizationURL": "https://github.com/login/device/code",
          "tokenURL": "https://github.com/login/oauth/access_token"
        }
      },
      "retry": {
        "enabled": true,
        "maxAttempts": 3,
        "strategy": "exponential",
        "baseDelay": "1s",
        "maxDelay": "10s"
      },
      "endpoints": [
        {
          "id": "repos_list",
          "name": "List Repositories",
          "description": "List user repositories",
          "method": "GET",
          "path": "/user/repos",
          "parameters": [
            {
              "name": "type",
              "description": "Repository type filter",
              "type": "string",
              "required": false,
              "location": "query",
              "default": "all",
              "validation": {
                "enum": ["all", "owner", "public", "private", "member"]
              }
            },
            {
              "name": "sort",
              "description": "Sort repositories by",
              "type": "string", 
              "required": false,
              "location": "query",
              "default": "updated",
              "validation": {
                "enum": ["created", "updated", "pushed", "full_name"]
              }
            },
            {
              "name": "per_page",
              "description": "Results per page (max 100)",
              "type": "number",
              "required": false,
              "location": "query",
              "default": 30,
              "validation": {
                "minimum": 1,
                "maximum": 100
              }
            },
            {
              "name": "page",
              "description": "Page number",
              "type": "number",
              "required": false,
              "location": "query",
              "default": 1,
              "validation": {
                "minimum": 1
              }
            }
          ],
          "response": {
            "type": "json",
            "paginated": true,
            "paginationConfig": {
              "nextPageTokenPath": "$.links.next",
              "dataPath": "$",
              "pageSize": 30
            },
            "caching": {
              "enabled": true,
              "ttl": "5m"
            }
          }
        }
      ]
    }
  }
}
```

## Troubleshooting

### Common Issues

**1. Parameter Naming Errors**
```
Error: tools.0.custom.input_schema.properties: Property keys should match pattern '^[a-zA-Z0-9_.-]{1,64}$'
```
**Cause**: Parameter names contain characters not allowed by MCP specification (`$`, `@`, spaces, etc.).

**Solutions:**
- Add explicit `alias` field to problematic parameters:
  ```json
  {"name": "$select", "alias": "select", "type": "string"}
  ```
- Review server logs for auto-sanitization warnings:
  ```
  WARNING: Auto-sanitized parameter '$filter' to 'filter' - consider adding explicit alias
  ```
- Check for alias conflicts (two parameters mapping to same MCP name)
- Ensure aliases match MCP pattern: `^[a-zA-Z0-9_.-]{1,64}$`

**2. Authentication Failures**
```
Error: Failed to apply authentication
```
**Solutions:**
- Check environment variables are set correctly
- Verify client ID and tenant ID for OAuth2
- Ensure API token is valid for bearer auth
- Check scopes include required permissions

**2. Parameter Validation Errors**
```
Error: Parameter validation failed: required parameter missing
```
**Solutions:**
- Check `required: true` parameters are provided
- Verify parameter names match exactly
- Check data types match parameter definitions
- Validate enum values are in allowed list

**3. URL Building Errors**
```
Error: Failed to build URL
```
**Solutions:**
- Check path parameters have corresponding parameter definitions
- Verify placeholder names match parameter names
- Ensure base URL is valid and accessible

**4. Response Parsing Errors**
```
Error: Failed to parse JSON response
```
**Solutions:**
- Check API returns valid JSON
- Verify response type matches actual response
- Check transform expressions are valid
- Validate pagination paths exist in response

### Debug Tips

**1. Enable Debug Logging**
```bash
./mcpfusion -config config.json -debug
```

**2. Test Individual Endpoints**
Use the test scripts in `tests/` directory to verify each endpoint works correctly.

**3. Check Environment Variables**
```bash
# List all environment variables
env | grep -E "(CLIENT_ID|API_KEY|TOKEN)"

# Check specific variable
echo $CLIENT_ID
```

**4. Validate JSON Configuration**
```bash
# Use a JSON validator
cat config.json | jq '.'
```

**5. Test Authentication Separately**
Create a minimal config with just one endpoint to test authentication in isolation.

### Error Categories

| Error Type | Typical Causes | Solutions |
|------------|---------------|-----------|
| **Parameter Naming** | Special characters in parameter names | Add explicit `alias` fields, review MCP naming rules |
| **Configuration** | Invalid JSON, missing fields | Validate JSON syntax, check required fields |
| **Authentication** | Invalid credentials, expired tokens | Verify environment variables, refresh tokens |
| **Network** | Connection issues, timeouts | Check network connectivity, increase timeouts |
| **API** | Rate limiting, server errors | Implement retry logic, check API status |
| **Validation** | Invalid parameters | Check parameter types and constraints |

## Environment Variable Reference

Create a `~/.mcp` file with your API credentials:

```bash
# Microsoft 365
MS365_CLIENT_ID=your-client-id
MS365_TENANT_ID=your-tenant-id

# Google APIs  
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret

# Generic APIs
API_KEY=your-api-key
API_TOKEN=your-bearer-token
API_BASE_URL=https://api.example.com

# Basic Auth
API_USERNAME=your-username
API_PASSWORD=your-password
```

## Configuration Validation

MCPFusion validates configurations on startup. Common validation errors:

| Error | Cause | Fix |
|-------|-------|-----|
| `parameter name conflict` | Two parameters map to same MCP name | Use different aliases or sanitized names |
| `parameter name not MCP-compliant` | Alias contains invalid characters | Ensure alias matches `^[a-zA-Z0-9_.-]{1,64}$` |
| `endpoint ID is required` | Missing `id` field | Add unique `id` to endpoint |
| `parameter name is required` | Missing `name` field | Add `name` to parameter |
| `invalid HTTP method` | Invalid `method` value | Use GET, POST, PUT, DELETE, PATCH |
| `invalid parameter location` | Invalid `location` value | Use path, query, body, header |
| `invalid parameter type` | Invalid `type` value | Use string, number, boolean, array, object |

Always test your configuration with the provided test scripts before deploying to production.