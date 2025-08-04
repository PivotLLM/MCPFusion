# Configuration Loading and Validation

This document describes the JSON configuration loading and validation functionality implemented for the MCPFusion package.

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