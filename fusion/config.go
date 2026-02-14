/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
)

// AuthType represents the type of authentication to use
type AuthType string

const (
	AuthTypeOAuth2Device    AuthType = "oauth2_device"
	AuthTypeOAuth2External  AuthType = "oauth2_external"
	AuthTypeBearer          AuthType = "bearer"
	AuthTypeAPIKey          AuthType = "api_key"
	AuthTypeBasic           AuthType = "basic"
	AuthTypeSessionJWT      AuthType = "session_jwt"
	AuthTypeUserCredentials AuthType = "user_credentials"
	AuthTypeNone            AuthType = "none"
)

// DefaultTokenInvalidationStatusCodes defines HTTP status codes that trigger token invalidation by default
var DefaultTokenInvalidationStatusCodes = []int{http.StatusUnauthorized} // 401

// ParameterType represents the type of a parameter
type ParameterType string

const (
	ParameterTypeString  ParameterType = "string"
	ParameterTypeNumber  ParameterType = "number"
	ParameterTypeInteger ParameterType = "integer" // JSON Schema integer type (alias for number)
	ParameterTypeBoolean ParameterType = "boolean"
	ParameterTypeArray   ParameterType = "array"
	ParameterTypeObject  ParameterType = "object"
)

// ParameterLocation represents where a parameter should be placed in the request
type ParameterLocation string

const (
	ParameterLocationPath        ParameterLocation = "path"
	ParameterLocationQuery       ParameterLocation = "query"
	ParameterLocationBody        ParameterLocation = "body"
	ParameterLocationHeader      ParameterLocation = "header"
	ParameterLocationArgument    ParameterLocation = "argument"    // Command-line argument
	ParameterLocationArglist     ParameterLocation = "arglist"     // Array of arguments
	ParameterLocationEnvironment ParameterLocation = "environment" // Environment variable
	ParameterLocationStdin       ParameterLocation = "stdin"       // Standard input
	ParameterLocationControl     ParameterLocation = "control"     // Execution control
)

// ResponseType represents the type of response expected
type ResponseType string

const (
	ResponseTypeJSON   ResponseType = "json"
	ResponseTypeText   ResponseType = "text"
	ResponseTypeBinary ResponseType = "binary"
)

// Config holds the main configuration for the fusion package
type Config struct {
	Logger     global.Logger                  `json:"-"`
	Services   map[string]*ServiceConfig      `json:"services"`
	Commands   map[string]*CommandGroupConfig `json:"commands"` // Command execution configs
	HTTPClient *http.Client                   `json:"-"`
	Cache      Cache                          `json:"-"`
	ConfigPath string                         `json:"-"`
}

// ServiceConfig represents the configuration for a single service
type ServiceConfig struct {
	ServiceKey     string                `json:"-"`
	Name           string                `json:"name"`
	BaseURL        string                `json:"baseURL"`
	Auth           AuthConfig            `json:"auth"`
	Endpoints      []EndpointConfig      `json:"endpoints"`
	Retry          *RetryConfig          `json:"retry,omitempty"`
	CircuitBreaker *CircuitBreakerConfig `json:"circuitBreaker,omitempty"`
}

// CommandGroupConfig represents a group of related commands
type CommandGroupConfig struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Commands    []CommandConfig `json:"commands"`
}

// CommandConfig represents configuration for a single command
type CommandConfig struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  []ParameterConfig `json:"parameters"`
}

// TokenInvalidationConfig represents configuration for automatic token invalidation
// When specific HTTP status codes are encountered, the cached/stored token can be automatically
// invalidated and optionally a retry attempted with fresh authentication.
type TokenInvalidationConfig struct {
	// StatusCodes lists HTTP status codes that should trigger token invalidation.
	// If empty, defaults to [401] (Unauthorized).
	// Common values: 401 (Unauthorized), 403 (Forbidden)
	StatusCodes []int `json:"statusCodes,omitempty"`

	// RetryOnInvalidation determines whether to automatically retry the request with
	// fresh authentication after invalidating the token.
	// Set to false for APIs that implement rate limiting after authentication failures,
	// or when you want to handle auth failures explicitly without automatic retries.
	// Default: true
	RetryOnInvalidation bool `json:"retryOnInvalidation"`

	// RetryDelay specifies the delay before retrying with fresh authentication.
	// Helps prevent overwhelming the authentication server.
	// If not specified or empty, defaults to 100ms.
	// Example values: "100ms", "500ms", "1s"
	RetryDelay    time.Duration `json:"-"`
	RetryDelayStr string        `json:"retryDelay,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling for TokenInvalidationConfig
func (t *TokenInvalidationConfig) UnmarshalJSON(data []byte) error {
	type Alias TokenInvalidationConfig
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(t),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Parse RetryDelay string to duration
	if t.RetryDelayStr != "" {
		duration, err := time.ParseDuration(t.RetryDelayStr)
		if err != nil {
			return fmt.Errorf("invalid retryDelay duration '%s': %w", t.RetryDelayStr, err)
		}
		t.RetryDelay = duration
	}

	return nil
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Type              AuthType                 `json:"type"`
	Config            map[string]interface{}   `json:"config"`
	TokenInvalidation *TokenInvalidationConfig `json:"tokenInvalidation,omitempty"`
}

// RequestBodyConfig represents configuration for encoding request body parameters
// before sending. When set, body parameters without a transform.targetName are
// collected, passed through the named encoder, and the encoded result is placed
// at wrapperPath in the JSON body. Parameters with targetName bypass encoding.
type RequestBodyConfig struct {
	Encoding    string `json:"encoding"`
	WrapperPath string `json:"wrapperPath"`
}

// EndpointConfig represents configuration for a single API endpoint
type EndpointConfig struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Method      string             `json:"method"`
	Path        string             `json:"path"`
	BaseURL     string             `json:"baseURL,omitempty"` // Overrides service BaseURL when set
	Parameters  []ParameterConfig  `json:"parameters"`
	RequestBody *RequestBodyConfig `json:"requestBody,omitempty"`
	Response    ResponseConfig     `json:"response"`
	Retry       *RetryConfig       `json:"retry,omitempty"`
	Connection  *ConnectionConfig  `json:"connection,omitempty"`
	Hints       *HintsConfig       `json:"hints,omitempty"`
}

// HintsConfig represents MCP tool hint configuration for an endpoint.
// All fields are optional pointers to allow explicit override of computed defaults.
// When a field is nil, the default value computed from the HTTP method is used.
type HintsConfig struct {
	// ReadOnly indicates the tool does not modify any state
	ReadOnly *bool `json:"readOnly,omitempty"`

	// Destructive indicates the tool may perform destructive operations
	Destructive *bool `json:"destructive,omitempty"`

	// Idempotent indicates calling the tool multiple times produces the same result
	Idempotent *bool `json:"idempotent,omitempty"`

	// OpenWorld indicates the tool interacts with external systems
	OpenWorld *bool `json:"openWorld,omitempty"`
}

// ParameterConfig represents configuration for a parameter
type ParameterConfig struct {
	Name        string            `json:"name"`
	Alias       string            `json:"alias,omitempty"`  // MCP-compliant name alias
	Prefix      string            `json:"prefix,omitempty"` // Prefix for argument location (e.g., "-p", "--port")
	Description string            `json:"description"`
	Type        ParameterType     `json:"type"`
	Required    bool              `json:"required"`
	Location    ParameterLocation `json:"location"`
	Default     interface{}       `json:"default,omitempty"`
	Examples    []interface{}     `json:"examples,omitempty"`
	Validation  *ValidationConfig `json:"validation,omitempty"`
	Transform   *TransformConfig  `json:"transform,omitempty"`
	Quoted      bool              `json:"quoted,omitempty"` // Whether to quote the parameter value
	Static      bool              `json:"static,omitempty"` // Whether this is a static parameter (not exposed to MCP, always uses default)
}

// ValidationConfig represents validation rules for a parameter
type ValidationConfig struct {
	Pattern   string        `json:"pattern,omitempty"`
	MinLength *int          `json:"minLength,omitempty"`
	MaxLength *int          `json:"maxLength,omitempty"`
	Minimum   *float64      `json:"minimum,omitempty"`
	Maximum   *float64      `json:"maximum,omitempty"`
	Enum      []interface{} `json:"enum,omitempty"`
	Format    string        `json:"format,omitempty"`
}

// IsValidEnumValue checks if a value is valid according to the enum constraints
func (v *ValidationConfig) IsValidEnumValue(value interface{}) bool {
	if len(v.Enum) == 0 {
		return true // No enum constraints
	}

	for _, allowedValue := range v.Enum {
		if fmt.Sprintf("%v", value) == fmt.Sprintf("%v", allowedValue) {
			return true
		}
	}
	return false
}

// TransformConfig represents transformation rules for a parameter
type TransformConfig struct {
	TargetName string `json:"targetName,omitempty"`
	Expression string `json:"expression,omitempty"`
}

// RetryStrategy represents different retry strategies
type RetryStrategy string

const (
	RetryStrategyExponential RetryStrategy = "exponential"
	RetryStrategyLinear      RetryStrategy = "linear"
	RetryStrategyFixed       RetryStrategy = "fixed"
)

// RetryConfig represents configuration for retry logic
type RetryConfig struct {
	Enabled         bool          `json:"enabled"`
	MaxAttempts     int           `json:"maxAttempts"`
	Strategy        RetryStrategy `json:"strategy"`
	BaseDelay       time.Duration `json:"-"`
	BaseDelayStr    string        `json:"baseDelay"`
	MaxDelay        time.Duration `json:"-"`
	MaxDelayStr     string        `json:"maxDelay"`
	Jitter          bool          `json:"jitter"`
	BackoffFactor   float64       `json:"backoffFactor"`
	RetryableErrors []string      `json:"retryableErrors,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling for RetryConfig
func (r *RetryConfig) UnmarshalJSON(data []byte) error {
	type Alias RetryConfig
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Parse BaseDelay string to duration
	if r.BaseDelayStr != "" {
		duration, err := time.ParseDuration(r.BaseDelayStr)
		if err != nil {
			return fmt.Errorf("invalid baseDelay duration '%s': %w", r.BaseDelayStr, err)
		}
		r.BaseDelay = duration
	}

	// Parse MaxDelay string to duration
	if r.MaxDelayStr != "" {
		duration, err := time.ParseDuration(r.MaxDelayStr)
		if err != nil {
			return fmt.Errorf("invalid maxDelay duration '%s': %w", r.MaxDelayStr, err)
		}
		r.MaxDelay = duration
	}

	// Set defaults
	if r.MaxAttempts == 0 {
		r.MaxAttempts = 3
	}
	if r.Strategy == "" {
		r.Strategy = RetryStrategyExponential
	}
	if r.BaseDelay == 0 {
		r.BaseDelay = time.Second
	}
	if r.BackoffFactor == 0 {
		r.BackoffFactor = 2.0
	}

	return nil
}

// CircuitBreakerConfig represents configuration for circuit breaker pattern
type CircuitBreakerConfig struct {
	Enabled          bool          `json:"enabled"`
	FailureThreshold int           `json:"failureThreshold"`
	SuccessThreshold int           `json:"successThreshold"`
	Timeout          time.Duration `json:"-"`
	TimeoutStr       string        `json:"timeout"`
	HalfOpenMaxCalls int           `json:"halfOpenMaxCalls"`
	ResetTimeout     time.Duration `json:"-"`
	ResetTimeoutStr  string        `json:"resetTimeout"`
}

// UnmarshalJSON implements custom JSON unmarshaling for CircuitBreakerConfig
func (c *CircuitBreakerConfig) UnmarshalJSON(data []byte) error {
	type Alias CircuitBreakerConfig
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(c),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Parse Timeout string to duration
	if c.TimeoutStr != "" {
		duration, err := time.ParseDuration(c.TimeoutStr)
		if err != nil {
			return fmt.Errorf("invalid circuit breaker timeout duration '%s': %w", c.TimeoutStr, err)
		}
		c.Timeout = duration
	}

	// Parse ResetTimeout string to duration
	if c.ResetTimeoutStr != "" {
		duration, err := time.ParseDuration(c.ResetTimeoutStr)
		if err != nil {
			return fmt.Errorf("invalid circuit breaker resetTimeout duration '%s': %w", c.ResetTimeoutStr, err)
		}
		c.ResetTimeout = duration
	}

	// Set defaults
	if c.FailureThreshold == 0 {
		c.FailureThreshold = 5
	}
	if c.SuccessThreshold == 0 {
		c.SuccessThreshold = 3
	}
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	if c.HalfOpenMaxCalls == 0 {
		c.HalfOpenMaxCalls = 3
	}
	if c.ResetTimeout == 0 {
		c.ResetTimeout = 60 * time.Second
	}

	return nil
}

// ConnectionConfig represents HTTP connection control settings for an endpoint
type ConnectionConfig struct {
	// DisableKeepAlive forces connection closure after each request
	DisableKeepAlive bool `json:"disableKeepAlive,omitempty"`
	// ForceNewConnection creates a new connection for each request (overrides pool reuse)
	ForceNewConnection bool `json:"forceNewConnection,omitempty"`
	// Timeout overrides the default request timeout for this endpoint
	Timeout string `json:"timeout,omitempty"`
}

// ResponseConfig represents configuration for response handling
type ResponseConfig struct {
	Type             ResponseType      `json:"type"`
	Transform        string            `json:"transform,omitempty"`
	Paginated        bool              `json:"paginated,omitempty"`
	PaginationConfig *PaginationConfig `json:"paginationConfig,omitempty"`
	Caching          *CachingConfig    `json:"caching,omitempty"`
	Retry            *RetryConfig      `json:"retry,omitempty"`
}

// CachingConfig represents configuration for response caching
type CachingConfig struct {
	Enabled bool          `json:"enabled"`
	TTL     time.Duration `json:"-"`
	TTLStr  string        `json:"ttl"`
	Key     string        `json:"key,omitempty"` // Custom cache key template
}

// UnmarshalJSON implements custom JSON unmarshaling for CachingConfig
func (c *CachingConfig) UnmarshalJSON(data []byte) error {
	type Alias CachingConfig
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(c),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Parse TTL string to duration
	if c.TTLStr != "" {
		duration, err := time.ParseDuration(c.TTLStr)
		if err != nil {
			return fmt.Errorf("invalid TTL duration '%s': %w", c.TTLStr, err)
		}
		c.TTL = duration
	}

	return nil
}

// PaginationConfig represents configuration for paginated responses
type PaginationConfig struct {
	NextPageTokenPath string `json:"nextPageTokenPath"`
	DataPath          string `json:"dataPath"`
	PageSize          int    `json:"pageSize"`
}

// LoadConfigFromFile loads configuration from a JSON file
func LoadConfigFromFile(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, NewConfigurationError("file", "",
			fmt.Sprintf("failed to read config file %s", filePath), err)
	}

	return LoadConfigFromJSON(data, filePath)
}

// LoadConfigFromJSONWithLogger loads configuration from JSON data with logging support
func LoadConfigFromJSONWithLogger(data []byte, configPath string, logger global.Logger) (*Config, error) {
	if logger != nil {
		logger.Infof("Loading configuration from %s", configPath)
		logger.Debugf("Configuration data size: %d bytes", len(data))
	}

	// First, expand environment variables
	if logger != nil {
		logger.Debug("Expanding environment variables in configuration")
	}
	expandedData, err := expandEnvironmentVariables(data)
	if err != nil {
		if logger != nil {
			logger.Errorf("Failed to expand environment variables: %v", err)
		}
		return nil, NewConfigurationError("environment_variables", "",
			"failed to expand environment variables", err)
	}

	if logger != nil {
		logger.Debug("Parsing JSON configuration")
	}
	var config Config
	if err := json.Unmarshal(expandedData, &config); err != nil {
		if logger != nil {
			logger.Errorf("Failed to parse JSON configuration: %v", err)
		}
		return nil, NewConfigurationError("json", "",
			"failed to parse JSON configuration", err)
	}

	config.ConfigPath = configPath

	// Validate the configuration
	if logger != nil {
		logger.Debug("Validating configuration")
	}
	if err := config.ValidateWithLogger(logger); err != nil {
		if logger != nil {
			logger.Errorf("Configuration validation failed: %v", err)
		}
		return nil, NewConfigurationError("validation", "",
			"configuration validation failed", err)
	}

	// Always set ServiceKey from the config map key
	for serviceName, service := range config.Services {
		service.ServiceKey = serviceName
		config.Services[serviceName] = service
	}

	if logger != nil {
		logger.Infof("Successfully loaded configuration with %d services", len(config.Services))
		for serviceName, service := range config.Services {
			logger.Debugf("Service '%s': %d endpoints, auth type: %s", serviceName, len(service.Endpoints), service.Auth.Type)
		}
	}

	return &config, nil
}

// LoadConfigFromJSON loads configuration from JSON data
func LoadConfigFromJSON(data []byte, configPath string) (*Config, error) {
	return LoadConfigFromJSONWithLogger(data, configPath, nil)
}

// ValidateWithLogger validates the configuration with logging support
func (c *Config) ValidateWithLogger(logger global.Logger) error {
	if logger != nil {
		logger.Debug("Starting configuration validation")
	}

	// Require at least one service OR one command group
	if len(c.Services) == 0 && len(c.Commands) == 0 {
		if logger != nil {
			logger.Error("Configuration validation failed: no services or commands configured")
		}
		return fmt.Errorf("no services or commands configured")
	}

	if logger != nil {
		logger.Debugf("Validating %d services", len(c.Services))
	}

	for serviceName, service := range c.Services {
		if logger != nil {
			logger.Debugf("Validating service: %s", serviceName)
		}
		if err := service.ValidateWithLogger(serviceName, logger); err != nil {
			if logger != nil {
				logger.Errorf("Service validation failed for %s: %v", serviceName, err)
			}
			return fmt.Errorf("service %s: %w", serviceName, err)
		}
	}

	if logger != nil {
		logger.Debug("Configuration validation completed successfully")
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	return c.ValidateWithLogger(nil)
}

// ValidateWithLogger validates a service configuration with logging support
func (s *ServiceConfig) ValidateWithLogger(serviceName string, logger global.Logger) error {
	if logger != nil {
		logger.Debugf("Validating service configuration for: %s", serviceName)
	}

	if s.Name == "" {
		if logger != nil {
			logger.Errorf("Service %s: name is required", serviceName)
		}
		return fmt.Errorf("service name is required")
	}

	if s.BaseURL == "" {
		if logger != nil {
			logger.Errorf("Service %s: baseURL is required", serviceName)
		}
		return fmt.Errorf("service baseURL is required")
	}

	if logger != nil {
		logger.Debugf("Service %s: validating auth configuration (type: %s)", serviceName, s.Auth.Type)
	}
	if err := s.Auth.ValidateWithLogger(serviceName, logger); err != nil {
		if logger != nil {
			logger.Errorf("Service %s: auth configuration validation failed: %v", serviceName, err)
		}
		return fmt.Errorf("auth configuration: %w", err)
	}

	if len(s.Endpoints) == 0 {
		if logger != nil {
			logger.Errorf("Service %s: at least one endpoint is required", serviceName)
		}
		return fmt.Errorf("at least one endpoint is required")
	}

	if logger != nil {
		logger.Debugf("Service %s: validating %d endpoints", serviceName, len(s.Endpoints))
	}
	for i, endpoint := range s.Endpoints {
		if logger != nil {
			logger.Debugf("Service %s: validating endpoint %d (%s)", serviceName, i, endpoint.ID)
		}
		if err := endpoint.ValidateWithLogger(serviceName, logger); err != nil {
			if logger != nil {
				logger.Errorf("Service %s: endpoint %d validation failed: %v", serviceName, i, err)
			}
			return fmt.Errorf("endpoint %d: %w", i, err)
		}
	}

	if logger != nil {
		logger.Debugf("Service %s: validation completed successfully", serviceName)
	}

	return nil
}

// Validate validates a service configuration
func (s *ServiceConfig) Validate() error {
	return s.ValidateWithLogger("", nil)
}

// ValidateWithLogger validates an auth configuration with logging support
func (a *AuthConfig) ValidateWithLogger(serviceName string, logger global.Logger) error {
	if logger != nil {
		logger.Debugf("Service %s: validating auth configuration (type: %s)", serviceName, a.Type)
	}

	switch a.Type {
	case AuthTypeOAuth2Device:
		if _, ok := a.Config["clientId"]; !ok {
			if logger != nil {
				logger.Errorf("Service %s: oauth2_device auth requires clientId", serviceName)
			}
			return fmt.Errorf("oauth2_device auth requires clientId")
		}
		if _, ok := a.Config["tokenURL"]; !ok {
			if logger != nil {
				logger.Errorf("Service %s: oauth2_device auth requires tokenURL", serviceName)
			}
			return fmt.Errorf("oauth2_device auth requires tokenURL")
		}
		if logger != nil {
			logger.Debugf("Service %s: OAuth2 device flow configuration validated", serviceName)
		}
	case AuthTypeOAuth2External:
		if _, ok := a.Config["clientId"]; !ok {
			if logger != nil {
				logger.Errorf("Service %s: oauth2_external auth requires clientId", serviceName)
			}
			return fmt.Errorf("oauth2_external auth requires clientId")
		}
		if _, ok := a.Config["tokenURL"]; !ok {
			if logger != nil {
				logger.Errorf("Service %s: oauth2_external auth requires tokenURL", serviceName)
			}
			return fmt.Errorf("oauth2_external auth requires tokenURL")
		}
		if logger != nil {
			logger.Debugf("Service %s: OAuth2 external flow configuration validated", serviceName)
		}
	case AuthTypeBearer:
		if _, hasToken := a.Config["token"]; !hasToken {
			if _, hasEnvVar := a.Config["tokenEnvVar"]; !hasEnvVar {
				if logger != nil {
					logger.Errorf("Service %s: bearer auth requires either token or tokenEnvVar", serviceName)
				}
				return fmt.Errorf("bearer auth requires either token or tokenEnvVar")
			}
			if logger != nil {
				logger.Debugf("Service %s: bearer token will be loaded from environment variable", serviceName)
			}
		} else {
			if logger != nil {
				logger.Debugf("Service %s: bearer token configured directly", serviceName)
			}
		}
	case AuthTypeAPIKey:
		if _, hasKey := a.Config["apiKey"]; !hasKey {
			if _, hasEnvVar := a.Config["apiKeyEnvVar"]; !hasEnvVar {
				if logger != nil {
					logger.Errorf("Service %s: api_key auth requires either apiKey or apiKeyEnvVar", serviceName)
				}
				return fmt.Errorf("api_key auth requires either apiKey or apiKeyEnvVar")
			}
			if logger != nil {
				logger.Debugf("Service %s: API key will be loaded from environment variable", serviceName)
			}
		} else {
			if logger != nil {
				logger.Debugf("Service %s: API key configured directly", serviceName)
			}
		}
	case AuthTypeBasic:
		if _, ok := a.Config["username"]; !ok {
			if logger != nil {
				logger.Errorf("Service %s: basic auth requires username", serviceName)
			}
			return fmt.Errorf("basic auth requires username")
		}
		if _, ok := a.Config["password"]; !ok {
			if logger != nil {
				logger.Errorf("Service %s: basic auth requires password", serviceName)
			}
			return fmt.Errorf("basic auth requires password")
		}
		if logger != nil {
			logger.Debugf("Service %s: basic auth configuration validated", serviceName)
		}
	case AuthTypeSessionJWT:
		if _, ok := a.Config["loginURL"]; !ok {
			if logger != nil {
				logger.Errorf("Service %s: session_jwt auth requires loginURL", serviceName)
			}
			return fmt.Errorf("session_jwt auth requires loginURL")
		}
		if _, ok := a.Config["tokenPath"]; !ok {
			if logger != nil {
				logger.Errorf("Service %s: session_jwt auth requires tokenPath", serviceName)
			}
			return fmt.Errorf("session_jwt auth requires tokenPath")
		}
		if _, ok := a.Config["tokenLocation"]; !ok {
			if logger != nil {
				logger.Errorf("Service %s: session_jwt auth requires tokenLocation", serviceName)
			}
			return fmt.Errorf("session_jwt auth requires tokenLocation")
		}
		// Validate tokenLocation value
		tokenLocation, _ := a.Config["tokenLocation"].(string)
		if tokenLocation != "header" && tokenLocation != "cookie" && tokenLocation != "query" {
			if logger != nil {
				logger.Errorf("Service %s: session_jwt tokenLocation must be 'header', 'cookie', or 'query'", serviceName)
			}
			return fmt.Errorf("session_jwt tokenLocation must be 'header', 'cookie', or 'query'")
		}
		// Validate conditional requirements based on tokenLocation
		switch tokenLocation {
		case "cookie":
			if _, ok := a.Config["cookieName"]; !ok {
				if logger != nil {
					logger.Errorf("Service %s: session_jwt with tokenLocation=cookie requires cookieName", serviceName)
				}
				return fmt.Errorf("session_jwt with tokenLocation=cookie requires cookieName")
			}
		case "query":
			if _, ok := a.Config["queryParam"]; !ok {
				if logger != nil {
					logger.Errorf("Service %s: session_jwt with tokenLocation=query requires queryParam", serviceName)
				}
				return fmt.Errorf("session_jwt with tokenLocation=query requires queryParam")
			}
			// "header" doesn't require additional fields - defaults to Authorization header
		}
		if logger != nil {
			logger.Debugf("Service %s: session_jwt auth configuration validated", serviceName)
		}
	case AuthTypeUserCredentials:
		fieldsRaw, ok := a.Config["fields"]
		if !ok {
			if logger != nil {
				logger.Errorf("Service %s: user_credentials auth requires 'fields' in config", serviceName)
			}
			return fmt.Errorf("user_credentials auth requires 'fields' in config")
		}
		fields, ok := fieldsRaw.([]interface{})
		if !ok || len(fields) == 0 {
			if logger != nil {
				logger.Errorf("Service %s: user_credentials 'fields' must be a non-empty array", serviceName)
			}
			return fmt.Errorf("user_credentials 'fields' must be a non-empty array")
		}
		validLocations := map[string]bool{"query": true, "header": true, "cookie": true}
		for i, fieldRaw := range fields {
			field, ok := fieldRaw.(map[string]interface{})
			if !ok {
				return fmt.Errorf("user_credentials field %d must be an object", i)
			}
			name, _ := field["name"].(string)
			if name == "" {
				return fmt.Errorf("user_credentials field %d requires 'name'", i)
			}
			location, _ := field["location"].(string)
			if !validLocations[location] {
				return fmt.Errorf("user_credentials field '%s' has invalid location '%s' (must be query, header, or cookie)", name, location)
			}
		}
		if logger != nil {
			logger.Debugf("Service %s: user_credentials auth configuration validated (%d fields)", serviceName, len(fields))
		}
	case AuthTypeNone:
		if logger != nil {
			logger.Debugf("Service %s: no authentication configured", serviceName)
		}
		// No validation needed for none auth type
	default:
		if logger != nil {
			logger.Errorf("Service %s: unsupported auth type: %s", serviceName, a.Type)
		}
		return fmt.Errorf("unsupported auth type: %s", a.Type)
	}

	if logger != nil {
		logger.Debugf("Service %s: auth configuration validated successfully", serviceName)
	}

	return nil
}

// Validate validates an auth configuration
func (a *AuthConfig) Validate() error {
	return a.ValidateWithLogger("", nil)
}

// GetEffectiveTokenInvalidationConfig returns the effective token invalidation configuration
// Returns configured values with defaults for missing fields, or defaults if not configured
func (a *AuthConfig) GetEffectiveTokenInvalidationConfig() *TokenInvalidationConfig {
	if a.TokenInvalidation != nil {
		// Use configured values, with defaults for missing fields
		config := *a.TokenInvalidation
		if len(config.StatusCodes) == 0 {
			config.StatusCodes = DefaultTokenInvalidationStatusCodes
		}
		if config.RetryDelay == 0 {
			config.RetryDelay = 100 * time.Millisecond // Default 100ms delay
		}
		return &config
	}
	// Return default config
	return &TokenInvalidationConfig{
		StatusCodes:         DefaultTokenInvalidationStatusCodes,
		RetryOnInvalidation: true,
		RetryDelay:          100 * time.Millisecond,
	}
}

// ValidateWithLogger validates an endpoint configuration with logging support
func (e *EndpointConfig) ValidateWithLogger(serviceName string, logger global.Logger) error {
	if logger != nil {
		logger.Debugf("Service %s: validating endpoint %s (%s %s)", serviceName, e.ID, e.Method, e.Path)
	}

	if e.ID == "" {
		if logger != nil {
			logger.Errorf("Service %s: endpoint ID is required", serviceName)
		}
		return fmt.Errorf("endpoint ID is required")
	}

	if e.Name == "" {
		if logger != nil {
			logger.Errorf("Service %s: endpoint %s name is required", serviceName, e.ID)
		}
		return fmt.Errorf("endpoint name is required")
	}

	if e.Method == "" {
		if logger != nil {
			logger.Errorf("Service %s: endpoint %s method is required", serviceName, e.ID)
		}
		return fmt.Errorf("endpoint method is required")
	}

	validMethods := map[string]bool{
		"GET":    true,
		"POST":   true,
		"PUT":    true,
		"DELETE": true,
		"PATCH":  true,
	}

	if !validMethods[e.Method] {
		if logger != nil {
			logger.Errorf("Service %s: endpoint %s has invalid HTTP method: %s", serviceName, e.ID, e.Method)
		}
		return fmt.Errorf("invalid HTTP method: %s", e.Method)
	}

	if e.Path == "" {
		if logger != nil {
			logger.Errorf("Service %s: endpoint %s path is required", serviceName, e.ID)
		}
		return fmt.Errorf("endpoint path is required")
	}

	if logger != nil {
		logger.Debugf("Service %s: endpoint %s has %d parameters", serviceName, e.ID, len(e.Parameters))
	}
	for i, param := range e.Parameters {
		if logger != nil {
			logger.Debugf("Service %s: endpoint %s validating parameter %d (%s)", serviceName, e.ID, i, param.Name)
		}
		if err := param.ValidateWithLogger(serviceName, e.ID, logger); err != nil {
			if logger != nil {
				logger.Errorf("Service %s: endpoint %s parameter %d validation failed: %v", serviceName, e.ID, i, err)
			}
			return fmt.Errorf("parameter %d: %w", i, err)
		}
	}

	// Validate requestBody encoding configuration if present
	if e.RequestBody != nil {
		if e.RequestBody.Encoding == "" {
			if logger != nil {
				logger.Errorf("Service %s: endpoint %s requestBody.encoding is required", serviceName, e.ID)
			}
			return fmt.Errorf("requestBody.encoding is required")
		}
		if e.RequestBody.WrapperPath == "" {
			if logger != nil {
				logger.Errorf("Service %s: endpoint %s requestBody.wrapperPath is required", serviceName, e.ID)
			}
			return fmt.Errorf("requestBody.wrapperPath is required")
		}
		if _, ok := GetBodyEncoder(e.RequestBody.Encoding); !ok {
			if logger != nil {
				logger.Errorf("Service %s: endpoint %s unknown requestBody encoding: %s", serviceName, e.ID, e.RequestBody.Encoding)
			}
			return fmt.Errorf("unknown requestBody encoding: %s", e.RequestBody.Encoding)
		}
		if logger != nil {
			logger.Debugf("Service %s: endpoint %s requestBody encoding validated: %s → %s",
				serviceName, e.ID, e.RequestBody.Encoding, e.RequestBody.WrapperPath)
		}
	}

	if logger != nil {
		logger.Debugf("Service %s: endpoint %s validating response configuration", serviceName, e.ID)
	}
	if err := e.Response.ValidateWithLogger(serviceName, e.ID, logger); err != nil {
		if logger != nil {
			logger.Errorf("Service %s: endpoint %s response configuration validation failed: %v", serviceName, e.ID, err)
		}
		return fmt.Errorf("response configuration: %w", err)
	}

	if logger != nil {
		logger.Debugf("Service %s: endpoint %s validation completed successfully", serviceName, e.ID)
	}

	return nil
}

// Validate validates an endpoint configuration
func (e *EndpointConfig) Validate() error {
	return e.ValidateWithLogger("", nil)
}

// ValidateWithLogger validates a parameter configuration with logging support
func (p *ParameterConfig) ValidateWithLogger(serviceName, endpointID string, logger global.Logger) error {
	if logger != nil {
		logger.Debugf("Service %s: endpoint %s validating parameter %s (type: %s, location: %s)",
			serviceName, endpointID, p.Name, p.Type, p.Location)
	}

	if p.Name == "" {
		if logger != nil {
			logger.Errorf("Service %s: endpoint %s parameter name is required", serviceName, endpointID)
		}
		return fmt.Errorf("parameter name is required")
	}

	validTypes := map[ParameterType]bool{
		ParameterTypeString:  true,
		ParameterTypeNumber:  true,
		ParameterTypeInteger: true,
		ParameterTypeBoolean: true,
		ParameterTypeArray:   true,
		ParameterTypeObject:  true,
	}

	if !validTypes[p.Type] {
		if logger != nil {
			logger.Errorf("Service %s: endpoint %s parameter %s has invalid type: %s", serviceName, endpointID, p.Name, p.Type)
		}
		return fmt.Errorf("invalid parameter type: %s", p.Type)
	}

	validLocations := map[ParameterLocation]bool{
		ParameterLocationPath:   true,
		ParameterLocationQuery:  true,
		ParameterLocationBody:   true,
		ParameterLocationHeader: true,
	}

	if !validLocations[p.Location] {
		if logger != nil {
			logger.Errorf("Service %s: endpoint %s parameter %s has invalid location: %s", serviceName, endpointID, p.Name, p.Location)
		}
		return fmt.Errorf("invalid parameter location: %s", p.Location)
	}

	if p.Validation != nil {
		if logger != nil {
			logger.Debugf("Service %s: endpoint %s parameter %s validating validation rules", serviceName, endpointID, p.Name)
		}
		if err := p.Validation.ValidateWithLogger(serviceName, endpointID, p.Name, logger); err != nil {
			if logger != nil {
				logger.Errorf("Service %s: endpoint %s parameter %s validation config failed: %v", serviceName, endpointID, p.Name, err)
			}
			return fmt.Errorf("validation config: %w", err)
		}
	}

	if logger != nil {
		logger.Debugf("Service %s: endpoint %s parameter %s validation completed successfully", serviceName, endpointID, p.Name)
	}

	return nil
}

// Validate validates a parameter configuration
func (p *ParameterConfig) Validate() error {
	return p.ValidateWithLogger("", "", nil)
}

// ValidateWithLogger validates a validation configuration with logging support
func (v *ValidationConfig) ValidateWithLogger(serviceName, endpointID, parameterName string, logger global.Logger) error {
	if logger != nil {
		logger.Debugf("Service %s: endpoint %s parameter %s validating validation rules", serviceName, endpointID, parameterName)
	}

	if v.Pattern != "" {
		if logger != nil {
			logger.Debugf("Service %s: endpoint %s parameter %s validating regex pattern: %s", serviceName, endpointID, parameterName, v.Pattern)
		}
		_, err := regexp.Compile(v.Pattern)
		if err != nil {
			if logger != nil {
				logger.Errorf("Service %s: endpoint %s parameter %s has invalid regex pattern '%s': %v", serviceName, endpointID, parameterName, v.Pattern, err)
			}
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
	}

	if v.MinLength != nil && *v.MinLength < 0 {
		if logger != nil {
			logger.Errorf("Service %s: endpoint %s parameter %s minLength cannot be negative: %d", serviceName, endpointID, parameterName, *v.MinLength)
		}
		return fmt.Errorf("minLength cannot be negative")
	}

	if v.MaxLength != nil && v.MinLength != nil && *v.MaxLength > 0 && *v.MinLength > *v.MaxLength {
		if logger != nil {
			logger.Errorf("Service %s: endpoint %s parameter %s minLength (%d) cannot be greater than maxLength (%d)", serviceName, endpointID, parameterName, *v.MinLength, *v.MaxLength)
		}
		return fmt.Errorf("minLength cannot be greater than maxLength")
	}

	if logger != nil && len(v.Enum) > 0 {
		logger.Debugf("Service %s: endpoint %s parameter %s has %d enum values", serviceName, endpointID, parameterName, len(v.Enum))
	}

	if logger != nil {
		logger.Debugf("Service %s: endpoint %s parameter %s validation rules validated successfully", serviceName, endpointID, parameterName)
	}

	return nil
}

// Validate validates a validation configuration
func (v *ValidationConfig) Validate() error {
	return v.ValidateWithLogger("", "", "", nil)
}

// ValidateWithLogger validates a response configuration with logging support
func (r *ResponseConfig) ValidateWithLogger(serviceName, endpointID string, logger global.Logger) error {
	if logger != nil {
		logger.Debugf("Service %s: endpoint %s validating response configuration (type: %s, paginated: %t)",
			serviceName, endpointID, r.Type, r.Paginated)
	}

	validTypes := map[ResponseType]bool{
		ResponseTypeJSON:   true,
		ResponseTypeText:   true,
		ResponseTypeBinary: true,
	}

	if !validTypes[r.Type] {
		if logger != nil {
			logger.Errorf("Service %s: endpoint %s has invalid response type: %s", serviceName, endpointID, r.Type)
		}
		return fmt.Errorf("invalid response type: %s", r.Type)
	}

	if r.Paginated && r.PaginationConfig == nil {
		if logger != nil {
			logger.Errorf("Service %s: endpoint %s paginated response requires paginationConfig", serviceName, endpointID)
		}
		return fmt.Errorf("paginated response requires paginationConfig")
	}

	if r.PaginationConfig != nil {
		if logger != nil {
			logger.Debugf("Service %s: endpoint %s validating pagination configuration", serviceName, endpointID)
		}
		if err := r.PaginationConfig.ValidateWithLogger(serviceName, endpointID, logger); err != nil {
			if logger != nil {
				logger.Errorf("Service %s: endpoint %s pagination config validation failed: %v", serviceName, endpointID, err)
			}
			return fmt.Errorf("pagination config: %w", err)
		}
	}

	if r.Transform != "" && logger != nil {
		logger.Debugf("Service %s: endpoint %s has response transformation: %s", serviceName, endpointID, r.Transform)
	}

	if logger != nil {
		logger.Debugf("Service %s: endpoint %s response configuration validated successfully", serviceName, endpointID)
	}

	return nil
}

// Validate validates a response configuration
func (r *ResponseConfig) Validate() error {
	return r.ValidateWithLogger("", "", nil)
}

// ValidateWithLogger validates a pagination configuration with logging support
func (p *PaginationConfig) ValidateWithLogger(serviceName, endpointID string, logger global.Logger) error {
	if logger != nil {
		logger.Debugf("Service %s: endpoint %s validating pagination configuration (pageSize: %d)",
			serviceName, endpointID, p.PageSize)
	}

	if p.NextPageTokenPath == "" {
		if logger != nil {
			logger.Errorf("Service %s: endpoint %s nextPageTokenPath is required for pagination", serviceName, endpointID)
		}
		return fmt.Errorf("nextPageTokenPath is required for pagination")
	}

	if p.DataPath == "" {
		if logger != nil {
			logger.Errorf("Service %s: endpoint %s dataPath is required for pagination", serviceName, endpointID)
		}
		return fmt.Errorf("dataPath is required for pagination")
	}

	if p.PageSize <= 0 {
		if logger != nil {
			logger.Errorf("Service %s: endpoint %s pageSize must be positive, got: %d", serviceName, endpointID, p.PageSize)
		}
		return fmt.Errorf("pageSize must be positive")
	}

	if logger != nil {
		logger.Debugf("Service %s: endpoint %s pagination configuration validated successfully (nextToken: %s, data: %s)",
			serviceName, endpointID, p.NextPageTokenPath, p.DataPath)
	}

	return nil
}

// Validate validates a pagination configuration
func (p *PaginationConfig) Validate() error {
	return p.ValidateWithLogger("", "", nil)
}

// expandEnvironmentVariables expands ${VAR_NAME} and ${VAR_NAME:default} patterns in JSON data
func expandEnvironmentVariables(data []byte) ([]byte, error) {
	content := string(data)

	// Find all ${VAR_NAME} and ${VAR_NAME:default} patterns
	re := regexp.MustCompile(`\$\{([^}:]+)(?::([^}]*))?\}`)

	result := re.ReplaceAllStringFunc(content, func(match string) string {
		// Extract variable name and default value
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		varName := matches[1]
		hasDefault := strings.Contains(match, ":") // Check if colon was present in original match
		defaultValue := ""
		if hasDefault && len(matches) > 2 {
			defaultValue = matches[2]
		}

		// Get environment variable value
		if value := os.Getenv(varName); value != "" {
			return value
		}

		// If not found and has default (even empty), use default
		if hasDefault {
			return defaultValue
		}

		// If not found and no default, return the original pattern
		return match
	})

	return []byte(result), nil
}

// GetEndpointByID finds an endpoint by ID within a service
func (s *ServiceConfig) GetEndpointByID(id string) *EndpointConfig {
	for i := range s.Endpoints {
		if s.Endpoints[i].ID == id {
			return &s.Endpoints[i]
		}
	}
	return nil
}

// GetRequiredParameters returns all required parameters for an endpoint
func (e *EndpointConfig) GetRequiredParameters() []ParameterConfig {
	var required []ParameterConfig
	for _, param := range e.Parameters {
		if param.Required {
			required = append(required, param)
		}
	}
	return required
}

// GetParameterByName finds a parameter by name
func (e *EndpointConfig) GetParameterByName(name string) *ParameterConfig {
	for i := range e.Parameters {
		if e.Parameters[i].Name == name {
			return &e.Parameters[i]
		}
	}
	return nil
}

// GetTransformedParameterName returns the target name if transform is configured, otherwise the original name
func (p *ParameterConfig) GetTransformedParameterName() string {
	if p.Transform != nil && p.Transform.TargetName != "" {
		return p.Transform.TargetName
	}
	return p.Name
}

// MatchesPattern checks if a string value matches the validation pattern
func (v *ValidationConfig) MatchesPattern(value string) bool {
	if v.Pattern == "" {
		return true
	}

	matched, err := regexp.MatchString(v.Pattern, value)
	if err != nil {
		return false
	}

	return matched
}

// IsValidLength checks if a string value meets length requirements
func (v *ValidationConfig) IsValidLength(value string) bool {
	length := len(value)

	if v.MinLength != nil && *v.MinLength > 0 && length < *v.MinLength {
		return false
	}

	if v.MaxLength != nil && *v.MaxLength > 0 && length > *v.MaxLength {
		return false
	}

	return true
}

// GetServiceByName returns a service configuration by name
func (c *Config) GetServiceByName(name string) *ServiceConfig {
	for _, service := range c.Services {
		if service.Name == name {
			return service
		}
	}
	return nil
}

// GetAllEndpoints returns all endpoints from all services with their service context
func (c *Config) GetAllEndpoints() []EndpointWithService {
	var endpoints []EndpointWithService
	for serviceName, service := range c.Services {
		for _, endpoint := range service.Endpoints {
			endpoints = append(endpoints, EndpointWithService{
				ServiceName: serviceName,
				Service:     service,
				Endpoint:    endpoint,
			})
		}
	}
	return endpoints
}

// EndpointWithService represents an endpoint with its associated service information
type EndpointWithService struct {
	ServiceName string
	Service     *ServiceConfig
	Endpoint    EndpointConfig
}

// ValidateServiceConfig validates a specific service configuration by name
func (c *Config) ValidateServiceConfig(serviceName string) error {
	service, exists := c.Services[serviceName]
	if !exists {
		return NewConfigurationError("service", serviceName,
			fmt.Sprintf("service '%s' not found", serviceName), nil)
	}

	return service.Validate()
}

// GetRequiredEnvironmentVariables scans the configuration and returns all environment variables that are referenced
func (c *Config) GetRequiredEnvironmentVariables() []string {
	content, _ := json.Marshal(c.Services)
	return extractEnvironmentVariables(content)
}

// extractEnvironmentVariables finds all ${VAR_NAME} patterns in data (excluding those with defaults)
func extractEnvironmentVariables(data []byte) []string {
	content := string(data)
	re := regexp.MustCompile(`\$\{([^}:]+)(?::([^}]*))?\}`)
	matches := re.FindAllStringSubmatch(content, -1)

	varMap := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			varName := match[1]
			hasDefault := len(match) > 2 && strings.Contains(match[0], ":")
			// Only include variables without defaults as "required"
			if !hasDefault {
				varMap[varName] = true
			}
		}
	}

	var vars []string
	for varName := range varMap {
		vars = append(vars, varName)
	}

	return vars
}

// Clone creates a deep copy of the configuration
func (c *Config) Clone() (*Config, error) {
	data, err := json.Marshal(c.Services)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config for cloning: %w", err)
	}

	clone := &Config{
		ConfigPath: c.ConfigPath,
		Services:   make(map[string]*ServiceConfig),
	}

	if err := json.Unmarshal(data, &clone.Services); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cloned config: %w", err)
	}

	return clone, nil
}

// MergeConfig merges another configuration into this one
func (c *Config) MergeConfig(other *Config) error {
	if other == nil {
		return nil
	}

	for serviceName, service := range other.Services {
		if _, exists := c.Services[serviceName]; exists {
			return NewConfigurationError("merge", serviceName,
				fmt.Sprintf("service '%s' already exists in target configuration", serviceName), nil)
		}
		c.Services[serviceName] = service
	}

	return c.Validate()
}

// GetEffectiveRetryConfig returns the effective retry configuration for an endpoint
// Endpoint-level config overrides service-level config, which overrides global defaults
func (e *EndpointConfig) GetEffectiveRetryConfig(service *ServiceConfig) *RetryConfig {
	// Endpoint-level override takes precedence
	if e.Retry != nil {
		return e.Retry
	}

	// Fall back to service-level config
	if service != nil && service.Retry != nil {
		return service.Retry
	}

	// Return default config if nothing is specified
	return &RetryConfig{
		Enabled:       false, // Disabled by default for backward compatibility
		MaxAttempts:   3,
		Strategy:      RetryStrategyExponential,
		BaseDelay:     time.Second,
		MaxDelay:      30 * time.Second,
		Jitter:        true,
		BackoffFactor: 2.0,
		RetryableErrors: []string{
			"network_error", "timeout", "rate_limited", "server_error",
		},
	}
}

// GetEffectiveCircuitBreakerConfig returns the effective circuit breaker configuration for a service
func (s *ServiceConfig) GetEffectiveCircuitBreakerConfig() *CircuitBreakerConfig {
	if s.CircuitBreaker != nil {
		return s.CircuitBreaker
	}

	// Return default config if nothing is specified
	return &CircuitBreakerConfig{
		Enabled:          false, // Disabled by default for backward compatibility
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
		HalfOpenMaxCalls: 3,
		ResetTimeout:     60 * time.Second,
	}
}

// IsRetryEnabled checks if retry is enabled for this endpoint
func (e *EndpointConfig) IsRetryEnabled(service *ServiceConfig) bool {
	config := e.GetEffectiveRetryConfig(service)
	return config.Enabled
}

// IsCircuitBreakerEnabled checks if circuit breaker is enabled for this service
func (s *ServiceConfig) IsCircuitBreakerEnabled() bool {
	config := s.GetEffectiveCircuitBreakerConfig()
	return config.Enabled
}
