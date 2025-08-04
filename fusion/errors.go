// Copyright (c) 2025 Tenebris Technologies Inc.
// Please see LICENSE for details.

package fusion

import (
	"fmt"
	"time"
)

// DeviceCodeError represents an error during OAuth2 device flow that requires user action
type DeviceCodeError struct {
	VerificationURL     string        `json:"verification_uri"`
	VerificationURLComplete string    `json:"verification_uri_complete,omitempty"`
	UserCode            string        `json:"user_code"`
	DeviceCode          string        `json:"device_code"`
	ExpiresIn           int           `json:"expires_in"`
	Interval            int           `json:"interval"`
	Message             string        `json:"message,omitempty"`
}

// Error implements the error interface
func (e DeviceCodeError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("OAuth2 Device Flow: %s\nPlease visit %s and enter code: %s", 
			e.Message, e.VerificationURL, e.UserCode)
	}
	return fmt.Sprintf("Please visit %s and enter code: %s", 
		e.VerificationURL, e.UserCode)
}

// IsExpired checks if the device code has expired
func (e DeviceCodeError) IsExpired() bool {
	// This would require storing the creation time, which we'll add if needed
	return false
}

// AuthenticationError represents authentication-related errors
type AuthenticationError struct {
	Type    AuthType `json:"type"`
	Service string   `json:"service"`
	Message string   `json:"message"`
	Cause   error    `json:"-"`
}

// Error implements the error interface
func (e AuthenticationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("authentication failed for service %s (%s): %s: %v", 
			e.Service, e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("authentication failed for service %s (%s): %s", 
		e.Service, e.Type, e.Message)
}

// GetUserFriendlyMessage returns a user-friendly error message with suggestions
func (e AuthenticationError) GetUserFriendlyMessage() string {
	switch e.Type {
	case AuthTypeBearer:
		return fmt.Sprintf("Bearer token authentication failed for service '%s'. Please check that your token is valid and has not expired. You may need to generate a new token from the service provider.", e.Service)
	case AuthTypeAPIKey:
		return fmt.Sprintf("API key authentication failed for service '%s'. Please verify that your API key is correct and has the necessary permissions. Check your service account settings if needed.", e.Service)
	case AuthTypeBasic:
		return fmt.Sprintf("Basic authentication failed for service '%s'. Please verify your username and password are correct.", e.Service)
	case AuthTypeOAuth2Device:
		return fmt.Sprintf("OAuth2 device flow authentication failed for service '%s'. You may need to re-authorize the application or check if the device code has expired.", e.Service)
	default:
		return fmt.Sprintf("Authentication failed for service '%s' using %s authentication. %s", e.Service, e.Type, e.Message)
	}
}

// Unwrap returns the underlying error
func (e AuthenticationError) Unwrap() error {
	return e.Cause
}

// ConfigurationError represents configuration-related errors
type ConfigurationError struct {
	Field   string `json:"field"`
	Service string `json:"service,omitempty"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
}

// Error implements the error interface
func (e ConfigurationError) Error() string {
	if e.Service != "" {
		if e.Cause != nil {
			return fmt.Sprintf("configuration error in service %s, field %s: %s: %v", 
				e.Service, e.Field, e.Message, e.Cause)
		}
		return fmt.Sprintf("configuration error in service %s, field %s: %s", 
			e.Service, e.Field, e.Message)
	}
	
	if e.Cause != nil {
		return fmt.Sprintf("configuration error in field %s: %s: %v", 
			e.Field, e.Message, e.Cause)
	}
	return fmt.Sprintf("configuration error in field %s: %s", 
		e.Field, e.Message)
}

// GetUserFriendlyMessage returns a user-friendly error message with suggestions
func (e ConfigurationError) GetUserFriendlyMessage() string {
	if e.Service != "" {
		switch e.Field {
		case "baseURL":
			return fmt.Sprintf("The base URL for service '%s' is invalid or missing. Please check your configuration file and ensure the baseURL field contains a valid HTTP/HTTPS URL.", e.Service)
		case "auth":
			return fmt.Sprintf("Authentication configuration for service '%s' is invalid. Please verify your authentication settings including credentials and auth type.", e.Service)
		case "clientId":
			return fmt.Sprintf("OAuth2 client ID is missing for service '%s'. Please add the clientId field to your authentication configuration.", e.Service)
		case "tokenURL":
			return fmt.Sprintf("OAuth2 token URL is missing for service '%s'. Please add the tokenURL field to your authentication configuration.", e.Service)
		default:
			return fmt.Sprintf("Configuration error in service '%s' for field '%s': %s Please check your configuration file and fix the indicated field.", e.Service, e.Field, e.Message)
		}
	}
	
	switch e.Field {
	case "file":
		return fmt.Sprintf("Configuration file could not be loaded: %s Please check the file path and ensure the file exists and is readable.", e.Message)
	case "json":
		return fmt.Sprintf("Configuration file contains invalid JSON: %s Please check the syntax of your configuration file.", e.Message)
	case "validation":
		return fmt.Sprintf("Configuration validation failed: %s Please review your configuration against the schema documentation.", e.Message)
	case "environment_variables":
		return fmt.Sprintf("Environment variable expansion failed: %s Please check that all referenced environment variables are set correctly.", e.Message)
	default:
		return fmt.Sprintf("Configuration error in field '%s': %s Please check your configuration file.", e.Field, e.Message)
	}
}

// Unwrap returns the underlying error
func (e ConfigurationError) Unwrap() error {
	return e.Cause
}

// ValidationError represents parameter validation errors
type ValidationError struct {
	Parameter string      `json:"parameter"`
	Value     interface{} `json:"value"`
	Rule      string      `json:"rule"`
	Message   string      `json:"message"`
}

// Error implements the error interface
func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed for parameter %s: %s (value: %v)", 
		e.Parameter, e.Message, e.Value)
}

// GetUserFriendlyMessage returns a user-friendly error message with suggestions
func (e ValidationError) GetUserFriendlyMessage() string {
	switch e.Rule {
	case "required":
		return fmt.Sprintf("The parameter '%s' is required but was not provided. Please include this parameter in your request.", e.Parameter)
	case "type":
		return fmt.Sprintf("The parameter '%s' has an incorrect type. %s Please check the expected data type and try again.", e.Parameter, e.Message)
	case "length":
		return fmt.Sprintf("The parameter '%s' does not meet length requirements. %s Please adjust the parameter value accordingly.", e.Parameter, e.Message)
	case "pattern":
		return fmt.Sprintf("The parameter '%s' does not match the required format. %s Please ensure the value follows the expected pattern.", e.Parameter, e.Message)
	case "enum":
		return fmt.Sprintf("The parameter '%s' has an invalid value. %s Please use one of the allowed values.", e.Parameter, e.Message)
	default:
		return fmt.Sprintf("Parameter '%s' validation failed: %s (current value: %v)", e.Parameter, e.Message, e.Value)
	}
}

// APIError represents errors from API calls
type APIError struct {
	Service    string `json:"service"`
	Endpoint   string `json:"endpoint"`
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Response   string `json:"response,omitempty"`
	Retryable  bool   `json:"retryable"`
}

// Error implements the error interface
func (e APIError) Error() string {
	if e.Response != "" {
		return fmt.Sprintf("API error from %s:%s (HTTP %d): %s - Response: %s", 
			e.Service, e.Endpoint, e.StatusCode, e.Message, e.Response)
	}
	return fmt.Sprintf("API error from %s:%s (HTTP %d): %s", 
		e.Service, e.Endpoint, e.StatusCode, e.Message)
}

// IsRetryable returns whether the error is retryable
func (e APIError) IsRetryable() bool {
	return e.Retryable
}

// TransformationError represents errors during parameter or response transformation
type TransformationError struct {
	Type       string      `json:"type"` // "parameter" or "response"
	Name       string      `json:"name"`
	Expression string      `json:"expression"`
	Value      interface{} `json:"value"`
	Message    string      `json:"message"`
	Cause      error       `json:"-"`
}

// Error implements the error interface
func (e TransformationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s transformation failed for %s using expression '%s': %s: %v", 
			e.Type, e.Name, e.Expression, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s transformation failed for %s using expression '%s': %s", 
		e.Type, e.Name, e.Expression, e.Message)
}

// Unwrap returns the underlying error
func (e TransformationError) Unwrap() error {
	return e.Cause
}

// CacheError represents errors related to caching operations
type CacheError struct {
	Operation string `json:"operation"` // "get", "set", "delete"
	Key       string `json:"key"`
	Message   string `json:"message"`
	Cause     error  `json:"-"`
}

// Error implements the error interface
func (e CacheError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("cache %s operation failed for key %s: %s: %v", 
			e.Operation, e.Key, e.Message, e.Cause)
	}
	return fmt.Sprintf("cache %s operation failed for key %s: %s", 
		e.Operation, e.Key, e.Message)
}

// Unwrap returns the underlying error
func (e CacheError) Unwrap() error {
	return e.Cause
}

// TokenError represents token-related errors
type TokenError struct {
	Type    AuthType  `json:"type"`
	Service string    `json:"service"`
	Reason  string    `json:"reason"` // "expired", "invalid", "missing", "refresh_failed"
	Message string    `json:"message"`
	Cause   error     `json:"-"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Error implements the error interface
func (e TokenError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("token error for service %s (%s): %s - %s: %v", 
			e.Service, e.Type, e.Reason, e.Message, e.Cause)
	}
	return fmt.Sprintf("token error for service %s (%s): %s - %s", 
		e.Service, e.Type, e.Reason, e.Message)
}

// Unwrap returns the underlying error
func (e TokenError) Unwrap() error {
	return e.Cause
}

// IsExpired returns true if the token error is due to expiration
func (e TokenError) IsExpired() bool {
	return e.Reason == "expired"
}

// IsRefreshable returns true if the token can potentially be refreshed
func (e TokenError) IsRefreshable() bool {
	return e.Reason == "expired" || e.Reason == "refresh_failed"
}

// NetworkError represents network-related errors
type NetworkError struct {
	URL       string        `json:"url"`
	Method    string        `json:"method"`
	Message   string        `json:"message"`
	Cause     error         `json:"-"`
	Timeout   bool          `json:"timeout"`
	Retryable bool          `json:"retryable"`
	RetryAfter *time.Duration `json:"retry_after,omitempty"`
}

// Error implements the error interface
func (e NetworkError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("network error for %s %s: %s: %v", 
			e.Method, e.URL, e.Message, e.Cause)
	}
	return fmt.Sprintf("network error for %s %s: %s", 
		e.Method, e.URL, e.Message)
}

// Unwrap returns the underlying error
func (e NetworkError) Unwrap() error {
	return e.Cause
}

// IsTimeout returns true if the error was due to a timeout
func (e NetworkError) IsTimeout() bool {
	return e.Timeout
}

// IsRetryable returns true if the error is retryable
func (e NetworkError) IsRetryable() bool {
	return e.Retryable
}

// GetRetryAfter returns the suggested retry delay
func (e NetworkError) GetRetryAfter() time.Duration {
	if e.RetryAfter != nil {
		return *e.RetryAfter
	}
	return 0
}

// NewDeviceCodeError creates a new DeviceCodeError
func NewDeviceCodeError(verificationURL, userCode, deviceCode string, expiresIn, interval int) *DeviceCodeError {
	return &DeviceCodeError{
		VerificationURL: verificationURL,
		UserCode:        userCode,
		DeviceCode:      deviceCode,
		ExpiresIn:       expiresIn,
		Interval:        interval,
	}
}

// NewAuthenticationError creates a new AuthenticationError
func NewAuthenticationError(authType AuthType, service, message string, cause error) *AuthenticationError {
	return &AuthenticationError{
		Type:    authType,
		Service: service,
		Message: message,
		Cause:   cause,
	}
}

// NewConfigurationError creates a new ConfigurationError
func NewConfigurationError(field, service, message string, cause error) *ConfigurationError {
	return &ConfigurationError{
		Field:   field,
		Service: service,
		Message: message,
		Cause:   cause,
	}
}

// NewValidationError creates a new ValidationError
func NewValidationError(parameter string, value interface{}, rule, message string) *ValidationError {
	return &ValidationError{
		Parameter: parameter,
		Value:     value,
		Rule:      rule,
		Message:   message,
	}
}

// NewAPIError creates a new APIError
func NewAPIError(service, endpoint string, statusCode int, message, response string, retryable bool) *APIError {
	return &APIError{
		Service:    service,
		Endpoint:   endpoint,
		StatusCode: statusCode,
		Message:    message,
		Response:   response,
		Retryable:  retryable,
	}
}

// NewTransformationError creates a new TransformationError
func NewTransformationError(transformType, name, expression string, value interface{}, message string, cause error) *TransformationError {
	return &TransformationError{
		Type:       transformType,
		Name:       name,
		Expression: expression,
		Value:      value,
		Message:    message,
		Cause:      cause,
	}
}

// NewTokenError creates a new TokenError
func NewTokenError(authType AuthType, service, reason, message string, cause error) *TokenError {
	return &TokenError{
		Type:    authType,
		Service: service,
		Reason:  reason,
		Message: message,
		Cause:   cause,
	}
}

// NewNetworkError creates a new NetworkError
func NewNetworkError(url, method, message string, cause error, timeout, retryable bool) *NetworkError {
	return &NetworkError{
		URL:       url,
		Method:    method,
		Message:   message,
		Cause:     cause,
		Timeout:   timeout,
		Retryable: retryable,
	}
}