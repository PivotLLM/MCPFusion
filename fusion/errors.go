/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"errors"
	"fmt"
	"time"
)

// DeviceCodeError represents an error during OAuth2 device flow that requires user action
type DeviceCodeError struct {
	VerificationURL         string   `json:"verification_uri"`
	VerificationURLComplete string   `json:"verification_uri_complete,omitempty"`
	UserCode                string   `json:"user_code"`
	DeviceCode              string   `json:"device_code"`
	ExpiresIn               int      `json:"expires_in"`
	Interval                int      `json:"interval"`
	Message                 string   `json:"message,omitempty"`
	ClientID                string   `json:"client_id"`
	TokenURL                string   `json:"token_url"`
	Scopes                  []string `json:"scopes,omitempty"`
}

// Error implements the error interface
func (e DeviceCodeError) Error() string {
	message := "AUTHENTICATION REQUIRED - Please relay this message to the user:\n\n"
	message += "You need to authenticate to continue. Please:\n"
	message += fmt.Sprintf("1. Visit: %s\n", e.VerificationURL)
	message += fmt.Sprintf("2. Enter code: %s\n", e.UserCode)

	if e.ExpiresIn > 0 {
		message += fmt.Sprintf("\nThis code expires in %d minutes.", e.ExpiresIn/60)
	}

	message += "\n\nOnce authenticated, you can retry your request."

	// Include any additional message if provided
	if e.Message != "" {
		message += fmt.Sprintf("\n\nAdditional info: %s", e.Message)
	}

	return message
}

// IsExpired checks if the device code has expired
func (e DeviceCodeError) IsExpired() bool {
	// This would require storing the creation time, which we'll add if needed
	return false
}

// AuthenticationError represents authentication-related errors
type AuthenticationError struct {
	Type          AuthType `json:"type"`
	Service       string   `json:"service"`
	Message       string   `json:"message"`
	Cause         error    `json:"-"`
	OriginalError error    `json:"-"`
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

// ErrorCategory represents the category of an error
type ErrorCategory string

//goland:noinspection GoUnusedConst
const (
	ErrorCategoryTransient  ErrorCategory = "transient"  // Temporary errors that can be retried
	ErrorCategoryPermanent  ErrorCategory = "permanent"  // Permanent errors that should not be retried
	ErrorCategoryAuth       ErrorCategory = "auth"       // Authentication/authorization errors
	ErrorCategoryRateLimit  ErrorCategory = "ratelimit"  // Rate limiting errors
	ErrorCategoryValidation ErrorCategory = "validation" // Input validation errors
	ErrorCategoryNetwork    ErrorCategory = "network"    // Network connectivity errors
	ErrorCategoryTimeout    ErrorCategory = "timeout"    // Timeout errors
	ErrorCategoryServer     ErrorCategory = "server"     // Server-side errors
	ErrorCategoryClient     ErrorCategory = "client"     // Client-side errors
)

// APIError represents errors from API calls
type APIError struct {
	Service       string        `json:"service"`
	Endpoint      string        `json:"endpoint"`
	StatusCode    int           `json:"status_code"`
	Message       string        `json:"message"`
	Response      string        `json:"response,omitempty"`
	Retryable     bool          `json:"retryable"`
	Category      ErrorCategory `json:"category"`
	CorrelationID string        `json:"correlation_id,omitempty"`
	Timestamp     time.Time     `json:"timestamp"`
}

// Error implements the error interface
func (e APIError) Error() string {
	// For security reasons, don't include the full response body in error messages
	// Return a generic error message for client-facing errors
	if e.StatusCode == 401 || e.StatusCode == 403 {
		return "Invalid token"
	}

	// For other errors, provide minimal information without exposing sensitive data
	return fmt.Sprintf("API request failed (HTTP %d)", e.StatusCode)
}

// IsRetryable returns whether the error is retryable
func (e APIError) IsRetryable() bool {
	return e.Retryable
}

// IsTransient returns whether the error is transient
func (e APIError) IsTransient() bool {
	return e.Category == ErrorCategoryTransient || e.Category == ErrorCategoryTimeout ||
		e.Category == ErrorCategoryNetwork || e.Category == ErrorCategoryRateLimit ||
		e.Category == ErrorCategoryServer
}

// GetCategory returns the error category
func (e APIError) GetCategory() ErrorCategory {
	return e.Category
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
	Type      AuthType   `json:"type"`
	Service   string     `json:"service"`
	Reason    string     `json:"reason"` // "expired", "invalid", "missing", "refresh_failed"
	Message   string     `json:"message"`
	Cause     error      `json:"-"`
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
	URL           string         `json:"url"`
	Method        string         `json:"method"`
	Message       string         `json:"message"`
	Cause         error          `json:"-"`
	Timeout       bool           `json:"timeout"`
	Retryable     bool           `json:"retryable"`
	RetryAfter    *time.Duration `json:"retry_after,omitempty"`
	Category      ErrorCategory  `json:"category"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	Timestamp     time.Time      `json:"timestamp"`
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
//
//goland:noinspection GoUnusedExportedFunction
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
		Type:          authType,
		Service:       service,
		Message:       message,
		Cause:         cause,
		OriginalError: cause,
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

// NewAPIError creates a new APIError with automatic categorization
func NewAPIError(service, endpoint string, statusCode int, message, response string, retryable bool) *APIError {
	category := categorizeHTTPError(statusCode)

	return &APIError{
		Service:    service,
		Endpoint:   endpoint,
		StatusCode: statusCode,
		Message:    message,
		Response:   response,
		Retryable:  retryable,
		Category:   category,
		Timestamp:  time.Now(),
	}
}

// NewAPIErrorWithCorrelation creates a new APIError with correlation ID
func NewAPIErrorWithCorrelation(service, endpoint string, statusCode int, message, response string, retryable bool, correlationID string) *APIError {
	apiErr := NewAPIError(service, endpoint, statusCode, message, response, retryable)
	apiErr.CorrelationID = correlationID
	return apiErr
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
//
//goland:noinspection GoUnusedExportedFunction
func NewTokenError(authType AuthType, service, reason, message string, cause error) *TokenError {
	return &TokenError{
		Type:    authType,
		Service: service,
		Reason:  reason,
		Message: message,
		Cause:   cause,
	}
}

// NewNetworkError creates a new NetworkError with automatic categorization
func NewNetworkError(url, method, message string, cause error, timeout, retryable bool) *NetworkError {
	category := ErrorCategoryNetwork
	if timeout {
		category = ErrorCategoryTimeout
	}

	return &NetworkError{
		URL:       url,
		Method:    method,
		Message:   message,
		Cause:     cause,
		Timeout:   timeout,
		Retryable: retryable,
		Category:  category,
		Timestamp: time.Now(),
	}
}

// NewNetworkErrorWithCorrelation creates a new NetworkError with correlation ID
func NewNetworkErrorWithCorrelation(url, method, message string, cause error, timeout, retryable bool, correlationID string) *NetworkError {
	netErr := NewNetworkError(url, method, message, cause, timeout, retryable)
	netErr.CorrelationID = correlationID
	return netErr
}

// categorizeHTTPError categorizes an HTTP status code into an error category
func categorizeHTTPError(statusCode int) ErrorCategory {
	switch {
	case statusCode == 429:
		return ErrorCategoryRateLimit
	case statusCode == 408:
		return ErrorCategoryTimeout
	case statusCode == 401 || statusCode == 403:
		return ErrorCategoryAuth
	case statusCode >= 400 && statusCode < 500:
		return ErrorCategoryClient
	case statusCode >= 500:
		return ErrorCategoryServer
	default:
		return ErrorCategoryPermanent
	}
}

// Helper functions for safe error type checking with wrapped errors

// AsDeviceCodeError safely extracts a DeviceCodeError from an error chain
func AsDeviceCodeError(err error) (*DeviceCodeError, bool) {
	var dcErr *DeviceCodeError
	if errors.As(err, &dcErr) {
		return dcErr, true
	}
	return nil, false
}

// AsNetworkError safely extracts a NetworkError from an error chain
func AsNetworkError(err error) (*NetworkError, bool) {
	var netErr *NetworkError
	if errors.As(err, &netErr) {
		return netErr, true
	}
	return nil, false
}

// AsAPIError safely extracts an APIError from an error chain
func AsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}

// AsValidationError safely extracts a ValidationError from an error chain
func AsValidationError(err error) (*ValidationError, bool) {
	var valErr *ValidationError
	if errors.As(err, &valErr) {
		return valErr, true
	}
	return nil, false
}

// AsAuthenticationError safely extracts an AuthenticationError from an error chain
func AsAuthenticationError(err error) (*AuthenticationError, bool) {
	var authErr *AuthenticationError
	if errors.As(err, &authErr) {
		return authErr, true
	}
	return nil, false
}

// AsTokenError safely extracts a TokenError from an error chain
func AsTokenError(err error) (*TokenError, bool) {
	var tokErr *TokenError
	if errors.As(err, &tokErr) {
		return tokErr, true
	}
	return nil, false
}
