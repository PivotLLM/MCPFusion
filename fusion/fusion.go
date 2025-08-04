// Copyright (c) 2025 Tenebris Technologies Inc.
// Please see LICENSE for details.

// Package fusion provides a dynamic, configuration-driven MCP provider that enables 
// access to multiple APIs through JSON configuration. It supports various authentication 
// methods and allows adding new API endpoints without code changes.
package fusion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
)

// Ensure Fusion implements the required interfaces
var _ global.ToolProvider = (*Fusion)(nil)
var _ global.ResourceProvider = (*Fusion)(nil)
var _ global.PromptProvider = (*Fusion)(nil)

// Fusion serves as the main package object and holds configuration information
type Fusion struct {
	config      *Config
	authManager *AuthManager
	httpClient  *http.Client
	cache       Cache
	logger      global.Logger
}

// Option defines a function type for configuration options
type Option func(*Fusion)

// WithJSONConfig loads configuration from a JSON file
func WithJSONConfig(configPath string) Option {
	return func(f *Fusion) {
		if f.logger != nil {
			f.logger.Infof("Loading configuration from file: %s", configPath)
		}
		
		data, err := os.ReadFile(configPath)
		if err != nil {
			if f.logger != nil {
				f.logger.Fatalf("Failed to read config file %s: %v", configPath, err)
			}
			return
		}
		
		config, err := LoadConfigFromJSONWithLogger(data, configPath, f.logger)
		if err != nil {
			if f.logger != nil {
				f.logger.Fatalf("Failed to load config from %s: %v", configPath, err)
			}
			return
		}
		f.config = config
	}
}

// WithJSONConfigData loads configuration from JSON data
func WithJSONConfigData(jsonData []byte, configPath string) Option {
	return func(f *Fusion) {
		if f.logger != nil {
			f.logger.Infof("Loading configuration from JSON data (path: %s)", configPath)
		}
		
		config, err := LoadConfigFromJSONWithLogger(jsonData, configPath, f.logger)
		if err != nil {
			if f.logger != nil {
				f.logger.Fatalf("Failed to load config from JSON data: %v", err)
			}
			return
		}
		f.config = config
	}
}

// WithConfig sets the configuration directly
func WithConfig(config *Config) Option {
	return func(f *Fusion) {
		f.config = config
	}
}

// WithLogger sets the logger
func WithLogger(logger global.Logger) Option {
	return func(f *Fusion) {
		f.logger = logger
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) Option {
	return func(f *Fusion) {
		f.httpClient = client
	}
}

// WithCache sets a custom cache implementation
func WithCache(cache Cache) Option {
	return func(f *Fusion) {
		f.cache = cache
	}
}

// WithInMemoryCache enables in-memory caching
func WithInMemoryCache() Option {
	return func(f *Fusion) {
		f.cache = NewInMemoryCacheWithLogger(f.logger)
	}
}

// WithNoCache disables caching
func WithNoCache() Option {
	return func(f *Fusion) {
		f.cache = NewNoOpCache()
	}
}

// WithTimeout sets the HTTP client timeout
func WithTimeout(timeout time.Duration) Option {
	return func(f *Fusion) {
		if f.httpClient == nil {
			f.httpClient = &http.Client{}
		}
		f.httpClient.Timeout = timeout
	}
}

// New creates a new Fusion instance with the provided options
func New(options ...Option) *Fusion {
	fusion := &Fusion{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache: NewInMemoryCache(), // Default to in-memory cache (will be updated with logger later)
	}
	
	// Apply all options
	for _, opt := range options {
		opt(fusion)
	}
	
	if fusion.logger != nil {
		fusion.logger.Debug("Initializing Fusion instance")
		fusion.logger.Debugf("HTTP client timeout: %v", fusion.httpClient.Timeout)
	}
	
	// Update cache with logger if we're using the default in-memory cache
	if _, isInMemory := fusion.cache.(*InMemoryCache); isInMemory {
		fusion.cache = NewInMemoryCacheWithLogger(fusion.logger)
	}
	
	// Initialize auth manager if we have a config
	if fusion.config != nil {
		if fusion.logger != nil {
			fusion.logger.Debug("Initializing authentication manager")
		}
		
		fusion.authManager = NewAuthManager(fusion.cache, fusion.logger)
		
		// Register default authentication strategies
		fusion.registerDefaultAuthStrategies()
		
		// Set references in config
		fusion.config.Logger = fusion.logger
		fusion.config.AuthManager = fusion.authManager
		fusion.config.HTTPClient = fusion.httpClient
		fusion.config.Cache = fusion.cache
		
		if fusion.logger != nil {
			fusion.logger.Infof("Fusion initialized with %d services", len(fusion.config.Services))
			
			// Log service summary
			for serviceName, service := range fusion.config.Services {
				fusion.logger.Debugf("Service '%s': baseURL=%s, auth=%s, endpoints=%d", 
					serviceName, service.BaseURL, service.Auth.Type, len(service.Endpoints))
			}
		}
	} else {
		if fusion.logger != nil {
			fusion.logger.Warning("No configuration provided - services will need to be configured separately")
		}
	}
	
	if fusion.logger != nil {
		fusion.logger.Info("Fusion instance initialization completed")
	}
	
	return fusion
}

// registerDefaultAuthStrategies registers the built-in authentication strategies
func (f *Fusion) registerDefaultAuthStrategies() {
	if f.authManager == nil {
		return
	}
	
	// Register OAuth2 device flow strategy
	oauth2Strategy := NewOAuth2DeviceFlowStrategy(f.httpClient, f.logger)
	f.authManager.RegisterStrategy(oauth2Strategy)
	
	// Register bearer token strategy
	bearerStrategy := NewBearerTokenStrategy(f.logger)
	f.authManager.RegisterStrategy(bearerStrategy)
	
	// Register API key strategy
	apiKeyStrategy := NewAPIKeyStrategy(f.logger)
	f.authManager.RegisterStrategy(apiKeyStrategy)
	
	// Register basic auth strategy
	basicStrategy := NewBasicAuthStrategy(f.logger)
	f.authManager.RegisterStrategy(basicStrategy)
}

// GetConfig returns the current configuration
func (f *Fusion) GetConfig() *Config {
	return f.config
}

// GetAuthManager returns the authentication manager
func (f *Fusion) GetAuthManager() *AuthManager {
	return f.authManager
}

// GetHTTPClient returns the HTTP client
func (f *Fusion) GetHTTPClient() *http.Client {
	return f.httpClient
}

// GetCache returns the cache
func (f *Fusion) GetCache() Cache {
	return f.cache
}

// GetLogger returns the logger
func (f *Fusion) GetLogger() global.Logger {
	return f.logger
}

// RegisterTools implements the global.ToolProvider interface
// This method will dynamically generate tools based on the configuration
func (f *Fusion) RegisterTools() []global.ToolDefinition {
	if f.config == nil {
		if f.logger != nil {
			f.logger.Warning("No configuration loaded, cannot register tools")
		}
		return []global.ToolDefinition{}
	}
	
	var tools []global.ToolDefinition
	
	for serviceName, service := range f.config.Services {
		for _, endpoint := range service.Endpoints {
			tool := f.createToolDefinition(serviceName, service, &endpoint)
			tools = append(tools, tool)
		}
	}
	
	if f.logger != nil {
		f.logger.Infof("Registered %d dynamic tools from configuration", len(tools))
	}
	
	return tools
}

// createToolDefinition creates a tool definition from an endpoint configuration
func (f *Fusion) createToolDefinition(serviceName string, service *ServiceConfig, endpoint *EndpointConfig) global.ToolDefinition {
	// Create tool parameters from endpoint parameters
	var parameters []global.Parameter
	for _, param := range endpoint.Parameters {
		parameters = append(parameters, global.Parameter{
			Name:        param.Name,
			Description: param.Description,
			Required:    param.Required,
		})
	}
	
	// Create the tool handler
	handler := f.createToolHandler(serviceName, service, endpoint)
	
	// Generate tool name by combining service and endpoint names
	toolName := fmt.Sprintf("%s_%s", serviceName, endpoint.ID)
	
	return global.ToolDefinition{
		Name:        toolName,
		Description: fmt.Sprintf("%s: %s", service.Name, endpoint.Description),
		Parameters:  parameters,
		Handler:     handler,
	}
}

// createToolHandler creates a handler function for a specific endpoint
func (f *Fusion) createToolHandler(serviceName string, service *ServiceConfig, endpoint *EndpointConfig) global.ToolHandler {
	return func(options map[string]any) (string, error) {
		startTime := time.Now()
		toolName := fmt.Sprintf("%s_%s", serviceName, endpoint.ID)
		
		if f.logger != nil {
			f.logger.Infof("Starting tool execution: %s", toolName)
			f.logger.Debugf("Tool %s parameters: %v", toolName, options)
		}
		
		ctx := context.Background()
		
		// Build the request
		if f.logger != nil {
			f.logger.Debugf("Building HTTP request for tool: %s", toolName)
		}
		req, err := f.buildRequest(ctx, serviceName, service, endpoint, options)
		if err != nil {
			if f.logger != nil {
				f.logger.Errorf("Failed to build request for %s: %v", toolName, err)
			}
			return "", err
		}
		
		if f.logger != nil {
			f.logger.Debugf("Built request for %s: %s %s", toolName, req.Method, req.URL.String())
			
			// Log sanitized headers for debugging
			if req.Header != nil {
				sanitizedHeaders := f.sanitizeHeaders(req.Header)
				f.logger.Debugf("Request headers for %s: %v", toolName, sanitizedHeaders)
			}
		}
		
		// Apply authentication
		if f.authManager != nil {
			if f.logger != nil {
				f.logger.Debugf("Applying authentication for tool: %s", toolName)
			}
			if err := f.authManager.ApplyAuthentication(ctx, req, serviceName, service.Auth); err != nil {
				if f.logger != nil {
					f.logger.Errorf("Failed to apply authentication for %s: %v", toolName, err)
				}
				return "", err
			}
		} else {
			if f.logger != nil {
				f.logger.Warning("No auth manager available for authentication")
			}
		}
		
		// Execute the request
		if f.logger != nil {
			f.logger.Debugf("Executing HTTP request for tool: %s", toolName)
		}
		
		reqStart := time.Now()
		resp, err := f.httpClient.Do(req)
		reqDuration := time.Since(reqStart)
		
		if err != nil {
			if f.logger != nil {
				f.logger.Errorf("HTTP request failed for %s after %v: %v", toolName, reqDuration, err)
			}
			return "", NewNetworkError(req.URL.String(), req.Method, "HTTP request failed", err, false, true)
		}
		defer resp.Body.Close()
		
		if f.logger != nil {
			f.logger.Debugf("HTTP request completed for %s: %s (%v)", toolName, resp.Status, reqDuration)
			
			// Log sanitized response headers for debugging
			if resp.Header != nil {
				sanitizedHeaders := f.sanitizeHeaders(resp.Header)
				f.logger.Debugf("Response headers for %s: %v", toolName, sanitizedHeaders)
			}
		}
		
		// Process the response
		if f.logger != nil {
			f.logger.Debugf("Processing response for tool: %s", toolName)
		}
		result, err := f.processResponse(resp, endpoint, serviceName)
		if err != nil {
			if f.logger != nil {
				f.logger.Errorf("Failed to process response for %s: %v", toolName, err)
			}
			return "", err
		}
		
		totalDuration := time.Since(startTime)
		if f.logger != nil {
			f.logger.Infof("Successfully executed tool %s (total: %v, request: %v)", toolName, totalDuration, reqDuration)
			f.logger.Debugf("Tool %s response length: %d characters", toolName, len(result))
		}
		
		return result, nil
	}
}

// RegisterResources implements the global.ResourceProvider interface
func (f *Fusion) RegisterResources() []global.ResourceDefinition {
	// Fusion doesn't provide static resources by default
	return []global.ResourceDefinition{}
}

// RegisterResourceTemplates implements the global.ResourceProvider interface
func (f *Fusion) RegisterResourceTemplates() []global.ResourceTemplateDefinition {
	// Fusion doesn't provide resource templates by default
	return []global.ResourceTemplateDefinition{}
}

// RegisterPrompts implements the global.PromptProvider interface
func (f *Fusion) RegisterPrompts() []global.PromptDefinition {
	// Fusion doesn't provide prompts by default
	return []global.PromptDefinition{}
}

// Validate validates the current configuration
func (f *Fusion) Validate() error {
	if f.config == nil {
		return NewConfigurationError("config", "", "no configuration loaded", nil)
	}
	
	return f.config.Validate()
}

// ReloadConfig reloads the configuration from the original file
func (f *Fusion) ReloadConfig() error {
	if f.config == nil || f.config.ConfigPath == "" {
		return NewConfigurationError("configPath", "", "no config path available for reload", nil)
	}
	
	newConfig, err := LoadConfigFromFile(f.config.ConfigPath)
	if err != nil {
		return NewConfigurationError("config", "", "failed to reload configuration", err)
	}
	
	// Update configuration
	f.config = newConfig
	f.config.Logger = f.logger
	f.config.AuthManager = f.authManager
	f.config.HTTPClient = f.httpClient
	f.config.Cache = f.cache
	
	if f.logger != nil {
		f.logger.Info("Configuration reloaded successfully")
	}
	
	return nil
}

// GetServiceNames returns a list of configured service names
func (f *Fusion) GetServiceNames() []string {
	if f.config == nil {
		return []string{}
	}
	
	names := make([]string, 0, len(f.config.Services))
	for name := range f.config.Services {
		names = append(names, name)
	}
	return names
}

// GetService returns a service configuration by name
func (f *Fusion) GetService(name string) *ServiceConfig {
	if f.config == nil {
		return nil
	}
	
	return f.config.Services[name]
}

// HasService checks if a service is configured
func (f *Fusion) HasService(name string) bool {
	if f.config == nil {
		return false
	}
	
	_, exists := f.config.Services[name]
	return exists
}

// GetEndpoint returns an endpoint configuration by service and endpoint ID
func (f *Fusion) GetEndpoint(serviceName, endpointID string) *EndpointConfig {
	service := f.GetService(serviceName)
	if service == nil {
		return nil
	}
	
	return service.GetEndpointByID(endpointID)
}

// GetSupportedAuthTypes returns a list of supported authentication types
func (f *Fusion) GetSupportedAuthTypes() []AuthType {
	if f.authManager == nil {
		return []AuthType{}
	}
	
	return f.authManager.GetRegisteredStrategies()
}

// InvalidateTokens invalidates all cached tokens
func (f *Fusion) InvalidateTokens() {
	if f.authManager == nil {
		return
	}
	
	for serviceName := range f.config.Services {
		f.authManager.InvalidateToken(serviceName)
	}
	
	if f.logger != nil {
		f.logger.Info("All tokens invalidated")
	}
}

// InvalidateServiceToken invalidates the cached token for a specific service
func (f *Fusion) InvalidateServiceToken(serviceName string) {
	if f.authManager == nil {
		return
	}
	
	f.authManager.InvalidateToken(serviceName)
	
	if f.logger != nil {
		f.logger.Infof("Token invalidated for service: %s", serviceName)
	}
}

// buildRequest constructs an HTTP request from endpoint configuration and user options
func (f *Fusion) buildRequest(ctx context.Context, serviceName string, service *ServiceConfig, endpoint *EndpointConfig, options map[string]any) (*http.Request, error) {
	if f.logger != nil {
		f.logger.Debugf("Building request for service %s, endpoint %s", serviceName, endpoint.ID)
	}

	// Start with the base URL and path
	requestURL := strings.TrimSuffix(service.BaseURL, "/") + "/" + strings.TrimPrefix(endpoint.Path, "/")
	if f.logger != nil {
		f.logger.Debugf("Initial request URL: %s", requestURL)
	}
	
	// Parse URL for modifications
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		if f.logger != nil {
			f.logger.Errorf("Failed to parse URL %s: %v", requestURL, err)
		}
		return nil, NewConfigurationError("path", serviceName, fmt.Sprintf("invalid URL: %s", requestURL), err)
	}
	
	// Prepare request body and query parameters
	queryParams := parsedURL.Query()
	var requestBody interface{}
	bodyParameters := make(map[string]interface{})
	pathParams := make(map[string]interface{})
	headerParams := make(map[string]interface{})
	
	if f.logger != nil {
		f.logger.Debugf("Processing %d parameters for endpoint %s", len(endpoint.Parameters), endpoint.ID)
	}
	
	// Process each parameter
	for _, param := range endpoint.Parameters {
		if f.logger != nil {
			f.logger.Debugf("Processing parameter: %s (type: %s, location: %s, required: %t)", 
				param.Name, param.Type, param.Location, param.Required)
		}

		value, provided := options[param.Name]
		
		// Check if required parameter is missing
		if param.Required && !provided {
			if f.logger != nil {
				f.logger.Errorf("Required parameter missing: %s", param.Name)
			}
			return nil, NewValidationError(param.Name, nil, "required", "parameter is required")
		}
		
		// Use default value if not provided
		if !provided && param.Default != nil {
			value = param.Default
			provided = true
			if f.logger != nil {
				f.logger.Debugf("Using default value for parameter %s: %v", param.Name, value)
			}
		}
		
		// Skip optional parameters that weren't provided
		if !provided {
			if f.logger != nil {
				f.logger.Debugf("Skipping optional parameter: %s", param.Name)
			}
			continue
		}
		
		// Validate the parameter
		if f.logger != nil {
			f.logger.Debugf("Validating parameter %s with value: %v", param.Name, value)
		}
		if err := f.validateParameter(&param, value); err != nil {
			if f.logger != nil {
				f.logger.Errorf("Parameter validation failed for %s: %v", param.Name, err)
			}
			return nil, err
		}
		
		// Transform the parameter if needed
		transformedValue, err := f.transformParameter(&param, value)
		if err != nil {
			if f.logger != nil {
				f.logger.Errorf("Parameter transformation failed for %s: %v", param.Name, err)
			}
			return nil, err
		}
		
		if transformedValue != value && f.logger != nil {
			f.logger.Debugf("Parameter %s transformed: %v -> %v", param.Name, value, transformedValue)
		}
		
		// Apply parameter to appropriate location
		switch param.Location {
		case ParameterLocationPath:
			pathParams[param.GetTransformedParameterName()] = transformedValue
			// Replace path parameter
			placeholder := "{" + param.GetTransformedParameterName() + "}"
			parsedURL.Path = strings.ReplaceAll(parsedURL.Path, placeholder, fmt.Sprintf("%v", transformedValue))
			if f.logger != nil {
				f.logger.Debugf("Applied path parameter %s: %v", param.Name, transformedValue)
			}
		case ParameterLocationQuery:
			queryParams.Set(param.GetTransformedParameterName(), fmt.Sprintf("%v", transformedValue))
			if f.logger != nil {
				f.logger.Debugf("Applied query parameter %s: %v", param.Name, transformedValue)
			}
		case ParameterLocationHeader:
			headerParams[param.GetTransformedParameterName()] = transformedValue
			if f.logger != nil {
				f.logger.Debugf("Prepared header parameter %s: %v", param.Name, transformedValue)
			}
		case ParameterLocationBody:
			bodyParameters[param.GetTransformedParameterName()] = transformedValue
			if f.logger != nil {
				f.logger.Debugf("Applied body parameter %s: %v", param.Name, transformedValue)
			}
		}
	}
	
	// Set query parameters
	parsedURL.RawQuery = queryParams.Encode()
	if f.logger != nil && len(queryParams) > 0 {
		f.logger.Debugf("Final query string: %s", parsedURL.RawQuery)
	}
	
	// Prepare request body
	var bodyReader io.Reader
	if len(bodyParameters) > 0 {
		switch strings.ToUpper(endpoint.Method) {
		case "POST", "PUT", "PATCH":
			if f.logger != nil {
				f.logger.Debugf("Marshaling request body with %d parameters", len(bodyParameters))
			}
			bodyData, err := json.Marshal(bodyParameters)
			if err != nil {
				if f.logger != nil {
					f.logger.Errorf("Failed to marshal request body: %v", err)
				}
				return nil, NewTransformationError("request", "body", "json.Marshal", bodyParameters, "failed to marshal request body", err)
			}
			bodyReader = bytes.NewReader(bodyData)
			requestBody = bodyParameters
			if f.logger != nil {
				sanitizedBody := f.sanitizeRequestBody(bodyData)
				f.logger.Debugf("Request body: %s", sanitizedBody)
			}
		default:
			if f.logger != nil {
				f.logger.Warningf("Body parameters provided for %s request but will be ignored", endpoint.Method)
			}
		}
	}
	
	// Create the HTTP request
	if f.logger != nil {
		f.logger.Debugf("Creating HTTP request: %s %s", strings.ToUpper(endpoint.Method), parsedURL.String())
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(endpoint.Method), parsedURL.String(), bodyReader)
	if err != nil {
		if f.logger != nil {
			f.logger.Errorf("Failed to create HTTP request: %v", err)
		}
		return nil, NewNetworkError(parsedURL.String(), endpoint.Method, "failed to create HTTP request", err, false, false)
	}
	
	// Set content type for body requests
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
		if f.logger != nil {
			f.logger.Debug("Set Content-Type: application/json")
		}
	}
	
	// Set header parameters
	headerCount := 0
	for _, param := range endpoint.Parameters {
		if param.Location == ParameterLocationHeader {
			if value, exists := options[param.Name]; exists {
				headerName := param.GetTransformedParameterName()
				headerValue := fmt.Sprintf("%v", value)
				req.Header.Set(headerName, headerValue)
				headerCount++
				if f.logger != nil {
					f.logger.Debugf("Set header %s: %s", headerName, headerValue)
				}
			}
		}
	}
	
	if f.logger != nil {
		f.logger.Infof("Successfully built request: %s %s (path params: %d, query params: %d, headers: %d, body params: %d)", 
			req.Method, req.URL.String(), len(pathParams), len(queryParams), headerCount, len(bodyParameters))
	}
	
	return req, nil
}

// processResponse processes the HTTP response according to endpoint configuration
func (f *Fusion) processResponse(resp *http.Response, endpoint *EndpointConfig, serviceName string) (string, error) {
	if f.logger != nil {
		f.logger.Debugf("Processing response for service %s, endpoint %s (status: %s)", serviceName, endpoint.ID, resp.Status)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)
		
		if f.logger != nil {
			sanitizedResponse := f.sanitizeResponseBody(bodyBytes, 500)
			f.logger.Errorf("HTTP error response for service %s, endpoint %s: %s - Body: %s", 
				serviceName, endpoint.ID, resp.Status, sanitizedResponse)
		}
		
		retryable := resp.StatusCode >= 500 || resp.StatusCode == 429 // Server errors and rate limiting
		return "", NewAPIError(serviceName, endpoint.ID, resp.StatusCode, resp.Status, bodyStr, retryable)
	}
	
	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		if f.logger != nil {
			f.logger.Errorf("Failed to read response body for service %s, endpoint %s: %v", serviceName, endpoint.ID, err)
		}
		return "", NewNetworkError("", "", "failed to read response body", err, false, false)
	}
	
	if f.logger != nil {
		sanitizedResponse := f.sanitizeResponseBody(bodyBytes, 1000)
		f.logger.Debugf("Response body for service %s, endpoint %s: %s", serviceName, endpoint.ID, sanitizedResponse)
	}
	
	// Process based on response type
	switch endpoint.Response.Type {
	case ResponseTypeJSON:
		if f.logger != nil {
			f.logger.Debugf("Processing JSON response for service %s, endpoint %s", serviceName, endpoint.ID)
		}
		return f.processJSONResponse(bodyBytes, endpoint, serviceName)
	case ResponseTypeText:
		if f.logger != nil {
			f.logger.Debugf("Processing text response for service %s, endpoint %s", serviceName, endpoint.ID)
		}
		return string(bodyBytes), nil
	case ResponseTypeBinary:
		if f.logger != nil {
			f.logger.Debugf("Processing binary response for service %s, endpoint %s (%d bytes)", 
				serviceName, endpoint.ID, len(bodyBytes))
		}
		// For binary responses, return base64 encoded data or metadata
		return fmt.Sprintf("Binary response received (%d bytes, Content-Type: %s)", 
			len(bodyBytes), resp.Header.Get("Content-Type")), nil
	default:
		if f.logger != nil {
			f.logger.Debugf("Processing response as text (default) for service %s, endpoint %s", serviceName, endpoint.ID)
		}
		// Default to treating as text
		return string(bodyBytes), nil
	}
}

// processJSONResponse processes JSON responses with optional transformation
func (f *Fusion) processJSONResponse(bodyBytes []byte, endpoint *EndpointConfig, serviceName string) (string, error) {
	// Parse JSON
	var responseData interface{}
	if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
		return "", NewTransformationError("response", "json", "json.Unmarshal", string(bodyBytes), "failed to parse JSON response", err)
	}
	
	// Apply transformation if specified
	if endpoint.Response.Transform != "" {
		transformed, err := f.applyResponseTransform(responseData, endpoint.Response.Transform)
		if err != nil {
			return "", err
		}
		responseData = transformed
	}
	
	// Handle pagination if enabled
	if endpoint.Response.Paginated && endpoint.Response.PaginationConfig != nil {
		return f.handlePaginatedResponse(responseData, endpoint, serviceName)
	}
	
	// Convert back to JSON string for consistent output
	result, err := json.MarshalIndent(responseData, "", "  ")
	if err != nil {
		return "", NewTransformationError("response", "json", "json.MarshalIndent", responseData, "failed to marshal response", err)
	}
	
	return string(result), nil
}

// validateParameter validates a parameter value according to its configuration
func (f *Fusion) validateParameter(param *ParameterConfig, value interface{}) error {
	if f.logger != nil {
		f.logger.Debugf("Validating parameter %s (type: %s, value: %v)", param.Name, param.Type, value)
	}

	// Type checking should always happen
	switch param.Type {
	case ParameterTypeString:
		strValue, ok := value.(string)
		if !ok {
			// Try to convert other types to string as a recovery mechanism
			if convertedStr := fmt.Sprintf("%v", value); f.logger != nil {
				f.logger.Warningf("Parameter %s: converting %T to string: %v -> %s", param.Name, value, value, convertedStr)
			}
			strValue = fmt.Sprintf("%v", value)
		}
		
		// Additional validation if config exists
		if param.Validation != nil {
			validation := param.Validation
			
			// Length validation
			if !validation.IsValidLength(strValue) {
				message := fmt.Sprintf("string length must be between %d and %d (actual: %d)", 
					validation.MinLength, validation.MaxLength, len(strValue))
				if f.logger != nil {
					f.logger.Errorf("Parameter %s validation failed: %s", param.Name, message)
				}
				return NewValidationError(param.Name, value, "length", message)
			}
			
			// Pattern validation
			if !validation.MatchesPattern(strValue) {
				message := fmt.Sprintf("string must match pattern: %s", validation.Pattern)
				if f.logger != nil {
					f.logger.Errorf("Parameter %s validation failed: %s", param.Name, message)
				}
				return NewValidationError(param.Name, value, "pattern", message)
			}
		}
		
	case ParameterTypeNumber:
		// Accept both int and float with better error handling
		var numValue float64
		var converted bool
		
		switch v := value.(type) {
		case float64:
			numValue = v
			converted = true
		case float32:
			numValue = float64(v)
			converted = true
		case int:
			numValue = float64(v)
			converted = true
		case int64:
			numValue = float64(v)
			converted = true
		case int32:
			numValue = float64(v)
			converted = true
		case string:
			// Try to parse string as number for recovery
			if parsedValue, err := strconv.ParseFloat(v, 64); err == nil {
				numValue = parsedValue
				converted = true
				if f.logger != nil {
					f.logger.Warningf("Parameter %s: converted string to number: %s -> %f", param.Name, v, numValue)
				}
			}
		}
		
		if !converted {
			message := fmt.Sprintf("parameter must be a number (received %T: %v)", value, value)
			if f.logger != nil {
				f.logger.Errorf("Parameter %s validation failed: %s", param.Name, message)
			}
			return NewValidationError(param.Name, value, "type", message)
		}
		
	case ParameterTypeBoolean:
		if _, ok := value.(bool); !ok {
			// Try to convert string to boolean for recovery
			if strValue, isString := value.(string); isString {
				if boolValue, err := strconv.ParseBool(strValue); err == nil {
					if f.logger != nil {
						f.logger.Warningf("Parameter %s: converted string to boolean: %s -> %t", param.Name, strValue, boolValue)
					}
				} else {
					message := fmt.Sprintf("parameter must be a boolean (received %T: %v)", value, value)
					if f.logger != nil {
						f.logger.Errorf("Parameter %s validation failed: %s", param.Name, message)
					}
					return NewValidationError(param.Name, value, "type", message)
				}
			} else {
				message := fmt.Sprintf("parameter must be a boolean (received %T: %v)", value, value)
				if f.logger != nil {
					f.logger.Errorf("Parameter %s validation failed: %s", param.Name, message)
				}
				return NewValidationError(param.Name, value, "type", message)
			}
		}
		
	case ParameterTypeArray:
		if reflect.TypeOf(value).Kind() != reflect.Slice {
			message := fmt.Sprintf("parameter must be an array (received %T: %v)", value, value)
			if f.logger != nil {
				f.logger.Errorf("Parameter %s validation failed: %s", param.Name, message)
			}
			return NewValidationError(param.Name, value, "type", message)
		}
		
	case ParameterTypeObject:
		if reflect.TypeOf(value).Kind() != reflect.Map {
			message := fmt.Sprintf("parameter must be an object (received %T: %v)", value, value)
			if f.logger != nil {
				f.logger.Errorf("Parameter %s validation failed: %s", param.Name, message)
			}
			return NewValidationError(param.Name, value, "type", message)
		}
	}
	
	// Enum validation if config exists
	if param.Validation != nil && !param.Validation.IsValidEnumValue(value) {
		message := fmt.Sprintf("value must be one of: %v (received: %v)", param.Validation.Enum, value)
		if f.logger != nil {
			f.logger.Errorf("Parameter %s validation failed: %s", param.Name, message)
		}
		return NewValidationError(param.Name, value, "enum", message)
	}
	
	if f.logger != nil {
		f.logger.Debugf("Parameter %s validation successful", param.Name)
	}
	
	return nil
}

// transformParameter applies parameter transformation if configured
func (f *Fusion) transformParameter(param *ParameterConfig, value interface{}) (interface{}, error) {
	if param.Transform == nil || param.Transform.Expression == "" {
		return value, nil
	}
	
	if f.logger != nil {
		f.logger.Debugf("Applying transformation to parameter %s: %s", param.Name, param.Transform.Expression)
	}
	
	// For now, implement basic transformations
	// This can be extended with a full expression evaluator
	expression := param.Transform.Expression
	
	switch expression {
	case "toString":
		result := fmt.Sprintf("%v", value)
		if f.logger != nil {
			f.logger.Debugf("Parameter %s toString transformation: %v -> %s", param.Name, value, result)
		}
		return result, nil
		
	case "toInt":
		switch v := value.(type) {
		case string:
			intValue, err := strconv.Atoi(v)
			if err != nil {
				if f.logger != nil {
					f.logger.Errorf("Parameter %s toInt transformation failed: %v", param.Name, err)
				}
				return nil, NewTransformationError("parameter", param.Name, expression, value, "failed to convert string to integer", err)
			}
			if f.logger != nil {
				f.logger.Debugf("Parameter %s toInt transformation: %s -> %d", param.Name, v, intValue)
			}
			return intValue, nil
		case float64:
			intValue := int(v)
			if f.logger != nil {
				f.logger.Debugf("Parameter %s toInt transformation: %f -> %d", param.Name, v, intValue)
			}
			return intValue, nil
		case float32:
			intValue := int(v)
			if f.logger != nil {
				f.logger.Debugf("Parameter %s toInt transformation: %f -> %d", param.Name, v, intValue)
			}
			return intValue, nil
		default:
			if f.logger != nil {
				f.logger.Warningf("Parameter %s toInt transformation: unsupported type %T, returning unchanged", param.Name, value)
			}
			return value, nil
		}
		
	case "toFloat":
		switch v := value.(type) {
		case string:
			floatValue, err := strconv.ParseFloat(v, 64)
			if err != nil {
				if f.logger != nil {
					f.logger.Errorf("Parameter %s toFloat transformation failed: %v", param.Name, err)
				}
				return nil, NewTransformationError("parameter", param.Name, expression, value, "failed to convert string to float", err)
			}
			if f.logger != nil {
				f.logger.Debugf("Parameter %s toFloat transformation: %s -> %f", param.Name, v, floatValue)
			}
			return floatValue, nil
		case int:
			floatValue := float64(v)
			if f.logger != nil {
				f.logger.Debugf("Parameter %s toFloat transformation: %d -> %f", param.Name, v, floatValue)
			}
			return floatValue, nil
		case int64:
			floatValue := float64(v)
			if f.logger != nil {
				f.logger.Debugf("Parameter %s toFloat transformation: %d -> %f", param.Name, v, floatValue)
			}
			return floatValue, nil
		default:
			if f.logger != nil {
				f.logger.Warningf("Parameter %s toFloat transformation: unsupported type %T, returning unchanged", param.Name, value)
			}
			return value, nil
		}
		
	case "toLowerCase":
		if strValue, ok := value.(string); ok {
			result := strings.ToLower(strValue)
			if f.logger != nil {
				f.logger.Debugf("Parameter %s toLowerCase transformation: %s -> %s", param.Name, strValue, result)
			}
			return result, nil
		}
		if f.logger != nil {
			f.logger.Warningf("Parameter %s toLowerCase transformation: value is not a string (%T), returning unchanged", param.Name, value)
		}
		return value, nil
		
	case "toUpperCase":
		if strValue, ok := value.(string); ok {
			result := strings.ToUpper(strValue)
			if f.logger != nil {
				f.logger.Debugf("Parameter %s toUpperCase transformation: %s -> %s", param.Name, strValue, result)
			}
			return result, nil
		}
		if f.logger != nil {
			f.logger.Warningf("Parameter %s toUpperCase transformation: value is not a string (%T), returning unchanged", param.Name, value)
		}
		return value, nil
		
	case "trim":
		if strValue, ok := value.(string); ok {
			result := strings.TrimSpace(strValue)
			if f.logger != nil {
				f.logger.Debugf("Parameter %s trim transformation: '%s' -> '%s'", param.Name, strValue, result)
			}
			return result, nil
		}
		if f.logger != nil {
			f.logger.Warningf("Parameter %s trim transformation: value is not a string (%T), returning unchanged", param.Name, value)
		}
		return value, nil
		
	default:
		// If we don't recognize the expression, just return the value unchanged
		if f.logger != nil {
			f.logger.Warningf("Parameter %s: unknown transformation expression '%s', returning value unchanged", param.Name, expression)
		}
		return value, nil
	}
}

// applyResponseTransform applies transformation to response data
func (f *Fusion) applyResponseTransform(data interface{}, transform string) (interface{}, error) {
	// For now, implement basic JSON path extraction
	// This can be extended with a full transformation engine
	
	if strings.HasPrefix(transform, "$.") {
		// Simple JSON path extraction
		return f.extractJSONPath(data, transform)
	}
	
	// If we don't recognize the transform, return data unchanged
	return data, nil
}

// extractJSONPath performs simple JSON path extraction
func (f *Fusion) extractJSONPath(data interface{}, path string) (interface{}, error) {
	// Remove the leading "$."
	path = strings.TrimPrefix(path, "$.")
	
	// Split path into parts
	parts := strings.Split(path, ".")
	
	current := data
	for _, part := range parts {
		if part == "" {
			continue
		}
		
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		default:
			return nil, NewTransformationError("response", "json_path", path, data, fmt.Sprintf("cannot navigate to '%s' in non-object", part), nil)
		}
		
		if current == nil {
			return nil, NewTransformationError("response", "json_path", path, data, fmt.Sprintf("path '%s' not found", part), nil)
		}
	}
	
	return current, nil
}

// handlePaginatedResponse handles paginated API responses
func (f *Fusion) handlePaginatedResponse(data interface{}, endpoint *EndpointConfig, serviceName string) (string, error) {
	// For now, just return the current page
	// Full pagination support would require additional context and state management
	
	config := endpoint.Response.PaginationConfig
	
	// Extract the data array
	dataArray, err := f.extractJSONPath(data, config.DataPath)
	if err != nil {
		return "", NewTransformationError("response", "pagination", config.DataPath, data, "failed to extract data array", err)
	}
	
	// Check if there's a next page token
	nextToken, _ := f.extractJSONPath(data, config.NextPageTokenPath)
	
	result := map[string]interface{}{
		"data": dataArray,
	}
	
	if nextToken != nil {
		result["nextPageToken"] = nextToken
		result["hasNextPage"] = true
	} else {
		result["hasNextPage"] = false
	}
	
	jsonResult, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", NewTransformationError("response", "json", "json.MarshalIndent", result, "failed to marshal paginated response", err)
	}
	
	return string(jsonResult), nil
}

// sanitizeHeaders removes or masks sensitive information from HTTP headers for logging
func (f *Fusion) sanitizeHeaders(headers http.Header) map[string]string {
	sensitiveHeaders := map[string]bool{
		"authorization":  true,
		"x-api-key":      true,
		"api-key":        true,
		"apikey":         true,
		"token":          true,
		"bearer":         true,
		"x-auth-token":   true,
		"x-access-token": true,
		"cookie":         true,
		"set-cookie":     true,
	}
	
	sanitized := make(map[string]string)
	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		if sensitiveHeaders[lowerKey] {
			sanitized[key] = "[REDACTED]"
		} else {
			sanitized[key] = strings.Join(values, ", ")
		}
	}
	return sanitized
}

// sanitizeQueryParams removes or masks sensitive information from query parameters for logging
func (f *Fusion) sanitizeQueryParams(params url.Values) map[string]string {
	sensitiveParams := map[string]bool{
		"token":       true,
		"access_token": true,
		"api_key":     true,
		"apikey":      true,
		"key":         true,
		"secret":      true,
		"password":    true,
		"pwd":         true,
		"auth":        true,
		"authorization": true,
	}
	
	sanitized := make(map[string]string)
	for key, values := range params {
		lowerKey := strings.ToLower(key)
		if sensitiveParams[lowerKey] {
			sanitized[key] = "[REDACTED]"
		} else {
			sanitized[key] = strings.Join(values, ", ")
		}
	}
	return sanitized
}

// sanitizeRequestBody removes or masks sensitive information from request body for logging
func (f *Fusion) sanitizeRequestBody(body []byte) string {
	// Try to parse as JSON first
	var jsonData map[string]interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		// If not JSON, check if it contains sensitive keywords and truncate/mask if needed
		bodyStr := string(body)
		if f.containsSensitiveData(bodyStr) {
			return "[REDACTED - contains sensitive data]"
		}
		// Truncate long non-JSON bodies
		if len(bodyStr) > 500 {
			return bodyStr[:500] + "...[truncated]"
		}
		return bodyStr
	}
	
	// Sanitize JSON data
	sanitized := f.sanitizeJSONData(jsonData)
	sanitizedBytes, _ := json.Marshal(sanitized)
	return string(sanitizedBytes)
}

// sanitizeJSONData recursively sanitizes JSON data by masking sensitive fields
func (f *Fusion) sanitizeJSONData(data interface{}) interface{} {
	sensitiveFields := map[string]bool{
		"password":      true,
		"token":         true,
		"access_token":  true,
		"refresh_token": true,
		"api_key":       true,
		"apikey":        true,
		"secret":        true,
		"key":           true,
		"auth":          true,
		"authorization": true,
		"credential":    true,
		"credentials":   true,
	}
	
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			lowerKey := strings.ToLower(key)
			if sensitiveFields[lowerKey] {
				result[key] = "[REDACTED]"
			} else {
				result[key] = f.sanitizeJSONData(value)
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = f.sanitizeJSONData(item)
		}
		return result
	default:
		return v
	}
}

// containsSensitiveData checks if a string contains sensitive keywords
func (f *Fusion) containsSensitiveData(data string) bool {
	sensitiveKeywords := []string{
		"password", "token", "secret", "key", "auth", "credential",
		"bearer", "oauth", "jwt", "api_key", "apikey",
	}
	
	lowerData := strings.ToLower(data)
	for _, keyword := range sensitiveKeywords {
		if strings.Contains(lowerData, keyword) {
			return true
		}
	}
	return false
}

// sanitizeResponseBody removes or masks sensitive information from response body for logging
func (f *Fusion) sanitizeResponseBody(body []byte, maxLength int) string {
	if maxLength <= 0 {
		maxLength = 1000 // Default max length
	}
	
	// Try to parse as JSON first
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		// If not JSON, check for sensitive data and truncate
		bodyStr := string(body)
		if f.containsSensitiveData(bodyStr) {
			return "[REDACTED - response contains sensitive data]"
		}
		// Truncate long responses
		if len(bodyStr) > maxLength {
			return bodyStr[:maxLength] + fmt.Sprintf("...[truncated, %d more bytes]", len(bodyStr)-maxLength)
		}
		return bodyStr
	}
	
	// Sanitize JSON data
	sanitized := f.sanitizeJSONData(jsonData)
	sanitizedBytes, _ := json.Marshal(sanitized)
	sanitizedStr := string(sanitizedBytes)
	
	// Truncate if too long
	if len(sanitizedStr) > maxLength {
		return sanitizedStr[:maxLength] + fmt.Sprintf("...[truncated, %d more bytes]", len(sanitizedStr)-maxLength)
	}
	
	return sanitizedStr
}