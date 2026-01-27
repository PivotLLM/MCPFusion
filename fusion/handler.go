/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
)

// HTTPHandler handles HTTP requests for a specific endpoint
type HTTPHandler struct {
	service         *ServiceConfig
	endpoint        *EndpointConfig
	fusion          *Fusion
	parameterMapper *ParameterNameMapper
}

// NewHTTPHandler creates a new HTTP handler for an endpoint
func NewHTTPHandler(fusion *Fusion, service *ServiceConfig, endpoint *EndpointConfig) *HTTPHandler {
	// Build parameter name mappings
	parameterMapper, err := BuildParameterMappings(endpoint.Parameters, fusion.logger)
	if err != nil && fusion.logger != nil {
		fusion.logger.Errorf("Failed to build parameter mappings for %s.%s: %v",
			service.Name, endpoint.ID, err)
	}

	return &HTTPHandler{
		service:         service,
		endpoint:        endpoint,
		fusion:          fusion,
		parameterMapper: parameterMapper,
	}
}

// prepareAuthConfig creates a copy of the auth config with baseURL injected
func (h *HTTPHandler) prepareAuthConfig() AuthConfig {
	authConfig := h.service.Auth
	if authConfig.Config == nil {
		authConfig.Config = make(map[string]interface{})
	} else {
		// Make a copy of the config map to avoid modifying the original
		configCopy := make(map[string]interface{})
		for k, v := range authConfig.Config {
			configCopy[k] = v
		}
		authConfig.Config = configCopy
	}
	authConfig.Config["baseURL"] = h.service.BaseURL
	return authConfig
}

// Handle processes an HTTP request based on the endpoint configuration
func (h *HTTPHandler) Handle(ctx context.Context, args map[string]interface{}) (string, error) {
	// Generate correlation ID for request tracking
	correlationID := ""
	if h.fusion.correlationIDGenerator != nil {
		correlationID = h.fusion.correlationIDGenerator.Generate()
	}

	if h.fusion.logger != nil {
		h.fusion.logger.Infof("Handling request for %s.%s [%s]", h.service.Name, h.endpoint.ID, correlationID)
	}

	startTime := time.Now()
	var requestMetrics *RequestMetrics

	// Map MCP parameter names to original API parameter names
	if h.parameterMapper != nil {
		args = h.parameterMapper.MapArgsToOriginal(args)
		if h.fusion.logger != nil {
			h.fusion.logger.Debugf("Mapped MCP parameters to API parameters [%s]", correlationID)
		}
	}

	// Process time tokens in parameter values
	timeTokenProcessor := NewTimeTokenProcessor(h.fusion.logger)
	args = timeTokenProcessor.ProcessParameterArgs(args)

	// Validate parameters
	validator := NewValidator(h.fusion.logger)
	if err := validator.ValidateParameters(h.endpoint.Parameters, args); err != nil {
		if h.fusion.logger != nil {
			h.fusion.logger.Errorf("Parameter validation failed [%s]: %v", correlationID, err)
		}
		return "", err
	}

	// Build request
	req, err := h.buildRequest(ctx, args)
	if err != nil {
		if h.fusion.logger != nil {
			h.fusion.logger.Errorf("Failed to build request [%s]: %v", correlationID, err)
		}
		return "", err
	}

	// Apply multi-tenant authentication if available
	if h.fusion.multiTenantAuth != nil {
		// Extract tenant context from the request context (set by auth middleware)
		// Use the shared context key from global package
		if h.fusion.logger != nil {
			h.fusion.logger.Debugf("Looking for tenant context in request context [%s]", correlationID)
		}

		tenantContextValue := ctx.Value(global.TenantContextKey)

		if tenantContextValue != nil {
			if tenantContext, ok := tenantContextValue.(*TenantContext); ok {
				// Ensure service name and request ID are set
				tenantContext.ServiceName = h.service.Name
				tenantContext.RequestID = correlationID

				if h.fusion.logger != nil {
					h.fusion.logger.Debugf("Found tenant context for tenant %s service %s [%s]",
						tenantContext.ShortHash(), tenantContext.ServiceName, correlationID)
				}

				// Apply authentication using multi-tenant auth manager
				// Inject baseURL into auth config for strategies that need it (e.g., session_jwt)
				authConfig := h.prepareAuthConfig()

				if err := h.fusion.multiTenantAuth.ApplyAuthentication(ctx, req, tenantContext, authConfig); err != nil {
					if h.fusion.logger != nil {
						h.fusion.logger.Errorf("Authentication failed for tenant %s service %s [%s]: %v",
							tenantContext.ShortHash(), tenantContext.ServiceName, correlationID, err)
					}

					// Check if it's a DeviceCodeError - pass it up for client handling
					if deviceCodeErr, ok := AsDeviceCodeError(err); ok {
						return "", deviceCodeErr
					}

					return "", fmt.Errorf("authentication failed: %w", err)
				}

				if h.fusion.logger != nil {
					h.fusion.logger.Debugf("Successfully applied authentication for tenant %s service %s [%s]",
						tenantContext.ShortHash(), tenantContext.ServiceName, correlationID)
				}
			} else {
				if h.fusion.logger != nil {
					h.fusion.logger.Warningf("Invalid tenant context type in request context [%s]: %T", correlationID, tenantContextValue)
				}
				return "", fmt.Errorf("invalid tenant context in request")
			}
		} else {
			if h.fusion.logger != nil {
				h.fusion.logger.Warningf("No tenant context found in request context [%s]", correlationID)
			}
			return "", fmt.Errorf("no tenant context found - authentication required")
		}
	} else {
		if h.fusion.logger != nil {
			h.fusion.logger.Errorf("Multi-tenant authentication manager not available [%s]", correlationID)
		}
		return "", fmt.Errorf("authentication not configured")
	}

	// Log final request after authentication is applied
	if h.fusion.logger != nil {
		h.fusion.logger.Debugf("\n=== Final Request (with auth) ===")
		h.fusion.logger.Debugf("Method: %s", req.Method)
		h.fusion.logger.Debugf("URL: %s", req.URL.String())
		h.fusion.logger.Debugf("Headers after auth:")
		for name, values := range req.Header {
			for _, value := range values {
				sanitizedValue := SanitizeHeaderForLogging(name, value)
				h.fusion.logger.Debugf("  %s: %s", name, sanitizedValue)
			}
		}
		if req.Body != nil {
			// Try to peek at body if possible
			if req.GetBody != nil {
				bodyReader, _ := req.GetBody()
				if bodyReader != nil {
					bodyBytes, _ := io.ReadAll(bodyReader)
					bodyPreview := string(bodyBytes)
					if len(bodyPreview) > 500 {
						bodyPreview = bodyPreview[:500] + "... [truncated]"
					}
					h.fusion.logger.Debugf("Body: %s", bodyPreview)
				}
			} else {
				h.fusion.logger.Debugf("Body: <present but not readable>")
			}
		} else {
			h.fusion.logger.Debugf("Body: <none>")
		}
		h.fusion.logger.Debugf("=================================")
	}

	// Execute request with enhanced retry logic and metrics
	resp, requestMetrics, err := h.executeRequest(ctx, req, correlationID)
	if err != nil {
		if h.fusion.logger != nil {
			h.fusion.logger.Errorf("Request execution failed [%s]: %v", correlationID, err)
			if requestMetrics != nil {
				h.fusion.logger.Debugf("Request metrics: StatusCode=%d, Latency=%v, RetryCount=%d",
					requestMetrics.StatusCode, requestMetrics.Latency, requestMetrics.RetryCount)
			}
		}

		// Create API error with correlation ID if we have response details
		if requestMetrics != nil && requestMetrics.StatusCode > 0 {
			apiErr := NewAPIErrorWithCorrelation(h.service.Name, h.endpoint.ID,
				requestMetrics.StatusCode, err.Error(), "", false, correlationID)
			return "", apiErr
		}

		return "", err
	}
	// Use a closure variable to track the response body to close
	// This ensures proper cleanup whether we retry or not
	responseToClose := resp
	defer func() {
		if responseToClose != nil && responseToClose.Body != nil {
			_ = responseToClose.Body.Close()
		}
	}()

	// Check for token invalidation status codes (only retry once to prevent infinite loops)
	tokenInvalidationConfig := h.service.Auth.GetEffectiveTokenInvalidationConfig()
	shouldInvalidate := false
	for _, code := range tokenInvalidationConfig.StatusCodes {
		if resp.StatusCode == code {
			shouldInvalidate = true
			break
		}
	}

	if shouldInvalidate {
		// Get tenant context for invalidation
		if tenantContextValue := ctx.Value(global.TenantContextKey); tenantContextValue != nil {
			if tenantContext, ok := tenantContextValue.(*TenantContext); ok {
				// Invalidate the cached token (with nil check)
				if h.fusion.multiTenantAuth != nil {
					h.fusion.multiTenantAuth.InvalidateToken(tenantContext)
				}

				if h.fusion.logger != nil {
					h.fusion.logger.Debugf("Token invalidated due to %d response for tenant %s service %s [%s]",
						resp.StatusCode, tenantContext.ShortHash(), h.service.Name, correlationID)
				}

				// Retry with fresh authentication if configured
				if tokenInvalidationConfig.RetryOnInvalidation {
					// Check context cancellation before retry
					if ctx.Err() != nil {
						return "", fmt.Errorf("request cancelled before retry: %w", ctx.Err())
					}

					// Apply retry delay to avoid overwhelming the auth server
					if tokenInvalidationConfig.RetryDelay > 0 {
						time.Sleep(tokenInvalidationConfig.RetryDelay)
					}

					if h.fusion.logger != nil {
						h.fusion.logger.Infof("Retrying request with fresh authentication [%s]", correlationID)
					}

					// Close the original response body before retry
					_ = resp.Body.Close()
					responseToClose = nil // Prevent double-close in defer

					// Rebuild the request
					retryReq, err := h.buildRequest(ctx, args)
					if err != nil {
						if h.fusion.logger != nil {
							h.fusion.logger.Errorf("Failed to rebuild request for retry [%s]: %v", correlationID, err)
						}
						return "", fmt.Errorf("retry failed: failed to rebuild request: %w", err)
					}

					// Re-apply authentication
					tenantContext.ServiceName = h.service.Name
					tenantContext.RequestID = correlationID

					authConfig := h.prepareAuthConfig()

					if err := h.fusion.multiTenantAuth.ApplyAuthentication(ctx, retryReq, tenantContext, authConfig); err != nil {
						if h.fusion.logger != nil {
							h.fusion.logger.Errorf("Re-authentication failed for retry [%s]: %v", correlationID, err)
						}
						if deviceCodeErr, ok := AsDeviceCodeError(err); ok {
							return "", deviceCodeErr
						}
						return "", fmt.Errorf("re-authentication failed: %w", err)
					}

					// Execute the retry request (only one retry to prevent infinite loops)
					var retryMetrics *RequestMetrics
					resp, retryMetrics, err = h.executeRequest(ctx, retryReq, correlationID)
					if err != nil {
						if h.fusion.logger != nil {
							h.fusion.logger.Errorf("Retry request failed [%s]: %v", correlationID, err)
						}
						return "", err
					}
					// Update the response to close to the new response
					responseToClose = resp

					if h.fusion.logger != nil {
						h.fusion.logger.Infof("Retry request completed with status %d [%s]", resp.StatusCode, correlationID)
						if retryMetrics != nil {
							h.fusion.logger.Debugf("Retry metrics: StatusCode=%d, Latency=%v",
								retryMetrics.StatusCode, retryMetrics.Latency)
						}
					}
				}
			}
		}
	}

	// Handle response
	result, err := h.handleResponse(resp, correlationID)
	if err != nil {
		if h.fusion.logger != nil {
			h.fusion.logger.Errorf("Response handling failed [%s]: %v", correlationID, err)
		}
		return "", err
	}

	totalLatency := time.Since(startTime)
	if h.fusion.logger != nil {
		h.fusion.logger.Infof("Successfully handled request for %s.%s in %v [%s]", h.service.Name, h.endpoint.ID, totalLatency, correlationID)
	}

	return result, nil
}

// buildRequest constructs an HTTP request based on the endpoint configuration
func (h *HTTPHandler) buildRequest(ctx context.Context, args map[string]interface{}) (*http.Request, error) {
	mapper := NewMapper(h.fusion.logger)

	// Build URL with path parameters
	requestUrl, err := mapper.BuildURL(h.service.BaseURL, h.endpoint.Path, h.endpoint.Parameters, args)
	if err != nil {
		return nil, fmt.Errorf("failed to build URL: %w", err)
	}

	// Build request body
	var body io.Reader
	if h.endpoint.Method == "POST" || h.endpoint.Method == "PUT" || h.endpoint.Method == "PATCH" {
		bodyData, err := mapper.BuildRequestBody(h.endpoint.Parameters, args)
		if err != nil {
			return nil, fmt.Errorf("failed to build request body: %w", err)
		}
		if bodyData != nil {
			bodyBytes, err := json.Marshal(bodyData)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %w", err)
			}
			body = bytes.NewReader(bodyBytes)
		}
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, h.endpoint.Method, requestUrl, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	// Add custom headers from parameters
	if err := mapper.ApplyHeaders(req, h.endpoint.Parameters, args); err != nil {
		return nil, fmt.Errorf("failed to apply headers: %w", err)
	}

	// Add query parameters
	if err := mapper.ApplyQueryParams(req, h.endpoint.Parameters, args); err != nil {
		return nil, fmt.Errorf("failed to apply query parameters: %w", err)
	}

	return req, nil
}

// executeRequest executes an HTTP request with enhanced retry logic and circuit breaker
func (h *HTTPHandler) executeRequest(ctx context.Context, req *http.Request, correlationID string) (*http.Response, *RequestMetrics, error) {
	startTime := time.Now()

	// Create request metrics
	metrics := &RequestMetrics{
		ServiceName:   h.service.Name,
		EndpointID:    h.endpoint.ID,
		Method:        req.Method,
		URL:           req.URL.String(),
		CorrelationID: correlationID,
		Timestamp:     startTime,
	}

	// Get effective retry configuration
	retryConfig := h.endpoint.GetEffectiveRetryConfig(h.service)

	// Check if circuit breaker is enabled for this service
	circuitBreakerConfig := h.service.GetEffectiveCircuitBreakerConfig()
	var circuitBreaker *CircuitBreaker
	if circuitBreakerConfig.Enabled {
		// Get or create circuit breaker for this service
		circuitBreaker = h.fusion.getOrCreateCircuitBreaker(h.service.Name, circuitBreakerConfig)
	}

	// Apply connection control settings if configured
	httpClient := h.fusion.httpClient
	if h.endpoint.Connection != nil {
		if h.endpoint.Connection.DisableKeepAlive {
			// Add Connection: close header to force connection closure
			req.Header.Set("Connection", "close")
			if h.fusion.logger != nil {
				h.fusion.logger.Debugf("Disabling keep-alive for request [%s]", correlationID)
			}
		}

		if h.endpoint.Connection.ForceNewConnection {
			// Create a new HTTP client with disabled connection pooling
			transport := h.fusion.httpClient.Transport.(*http.Transport).Clone()
			transport.DisableKeepAlives = true
			transport.MaxIdleConns = -1
			httpClient = &http.Client{
				Transport: transport,
				Timeout:   h.fusion.httpClient.Timeout,
			}
			if h.fusion.logger != nil {
				h.fusion.logger.Debugf("Forcing new connection for request [%s]", correlationID)
			}
		}

		if h.endpoint.Connection.Timeout != "" {
			// Parse custom timeout for this endpoint
			if timeout, err := time.ParseDuration(h.endpoint.Connection.Timeout); err == nil {
				// Create a custom client with the specified timeout
				transport := httpClient.Transport
				if h.endpoint.Connection.ForceNewConnection {
					// Already cloned above
				} else {
					transport = h.fusion.httpClient.Transport.(*http.Transport).Clone()
				}
				httpClient = &http.Client{
					Transport: transport,
					Timeout:   timeout,
				}
				if h.fusion.logger != nil {
					h.fusion.logger.Debugf("Using custom timeout %v for request [%s]", timeout, correlationID)
				}
			} else if h.fusion.logger != nil {
				h.fusion.logger.Warningf("Invalid timeout format '%s' for endpoint %s [%s]",
					h.endpoint.Connection.Timeout, h.endpoint.ID, correlationID)
			}
		}
	}

	// Execute with circuit breaker if enabled
	var resp *http.Response
	var err error

	executeFunc := func() error {
		if retryConfig.Enabled {
			// Use retry executor
			retryExecutor := NewRetryExecutor(retryConfig, h.fusion.logger)
			resp, err = retryExecutor.Execute(ctx, httpClient, req)
			if err != nil && resp == nil {
				// Count retry attempts from the error context
				metrics.RetryCount = retryConfig.MaxAttempts - 1
			}
		} else {
			// Single attempt without retry
			if h.fusion.logger != nil {
				h.fusion.logger.Debugf("Executing HTTP request: %s %s", req.Method, req.URL.String())
			}
			resp, err = httpClient.Do(req)
			if err != nil {
				if h.fusion.logger != nil {
					h.fusion.logger.Debugf("HTTP request failed: %v", err)
				}
				// Wrap network errors even when not using retry
				err = h.wrapNetworkError(err, req)
			} else if h.fusion.logger != nil {
				h.fusion.logger.Debugf("HTTP response received: Status=%d", resp.StatusCode)
			}
		}
		return err
	}

	if circuitBreaker != nil {
		err = circuitBreaker.Execute(ctx, executeFunc)
	} else {
		err = executeFunc()
	}

	// Calculate latency
	metrics.Latency = time.Since(startTime)

	// Update metrics based on result
	if err != nil {
		metrics.Success = false
		// Categorize the error
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
			metrics.ErrorCategory = ErrorCategoryTimeout
			// Automatically cleanup connections on timeout errors
			if h.fusion.logger != nil {
				h.fusion.logger.Debugf("Timeout detected, triggering connection cleanup [%s]", correlationID)
			}
			h.fusion.ForceConnectionCleanup()
		} else if strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "network") {
			metrics.ErrorCategory = ErrorCategoryNetwork
			// Automatically cleanup connections on connection errors
			if h.fusion.logger != nil {
				h.fusion.logger.Debugf("Connection error detected, triggering connection cleanup [%s]", correlationID)
			}
			h.fusion.ForceConnectionCleanup()
		} else if strings.Contains(err.Error(), "circuit breaker") {
			metrics.ErrorCategory = ErrorCategoryServer
		} else {
			metrics.ErrorCategory = ErrorCategoryPermanent
		}

		// Enhance error with correlation ID if it's a network error
		if netErr, ok := AsNetworkError(err); ok && netErr.CorrelationID == "" {
			netErr.CorrelationID = correlationID
		}
	} else if resp != nil {
		metrics.StatusCode = resp.StatusCode
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			metrics.Success = true
		} else {
			metrics.Success = false
			metrics.ErrorCategory = categorizeHTTPError(resp.StatusCode)
		}
	}

	// Record metrics if collector is available
	if h.fusion.metricsCollector != nil {
		h.fusion.metricsCollector.RecordRequest(*metrics)
	}

	return resp, metrics, err
}

// handleResponse processes the HTTP response
func (h *HTTPHandler) handleResponse(resp *http.Response, correlationID string) (string, error) {
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Debug log the response
	if h.fusion.logger != nil {
		h.fusion.logger.Debugf("\n=== HTTP Response Debug ===")
		h.fusion.logger.Debugf("Status Code: %d", resp.StatusCode)
		h.fusion.logger.Debugf("Status: %s", resp.Status)
		h.fusion.logger.Debugf("Headers:")
		for name, values := range resp.Header {
			for _, value := range values {
				h.fusion.logger.Debugf("  %s: %s", name, value)
			}
		}
		bodyPreview := string(body)
		if len(bodyPreview) > 1000 {
			bodyPreview = bodyPreview[:1000] + "... [truncated]"
		}
		h.fusion.logger.Debugf("Body (%d bytes): %s", len(body), bodyPreview)
		h.fusion.logger.Debugf("============================")
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		if h.fusion.logger != nil {
			h.fusion.logger.Errorf("API error [%s]: status=%d, body=%s", correlationID, resp.StatusCode, string(body))
		}
		// Return the actual API error response instead of a generic message
		// This allows LLMs to understand what went wrong and correct their requests
		return string(body), nil
	}

	// Handle different response types
	switch h.endpoint.Response.Type {
	case "json":
		// Parse JSON response
		var data interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return "", fmt.Errorf("failed to parse JSON response: %w", err)
		}

		// Apply transformation if specified
		if h.endpoint.Response.Transform != "" {
			mapper := NewMapper(h.fusion.logger)
			transformed, err := mapper.TransformResponse(data, h.endpoint.Response.Transform)
			if err != nil {
				return "", fmt.Errorf("failed to transform response: %w", err)
			}
			data = transformed
		}

		// Convert back to JSON string
		result, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal response: %w", err)
		}
		return string(result), nil

	case "text":
		return string(body), nil

	case "binary":
		// For binary responses, return base64 encoded string
		// This is a simplified implementation
		return fmt.Sprintf("Binary data (%d bytes)", len(body)), nil

	default:
		// Default to JSON
		return string(body), nil
	}
}

// wrapNetworkError wraps network errors in NetworkError type
func (h *HTTPHandler) wrapNetworkError(err error, req *http.Request) error {
	if err == nil {
		return nil
	}

	// Check if it's already a NetworkError (avoid double wrapping)
	if _, ok := AsNetworkError(err); ok {
		return err
	}

	// Determine if it's a timeout error
	timeout := false
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		//if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		timeout = true
	}

	// Determine if it's retryable
	retryable := true

	// Check for specific error types
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Timeout() {
		//if urlErr, ok := err.(*url.Error); ok {
		// DNS errors, connection errors, etc. are retryable
		if strings.Contains(urlErr.Error(), "no such host") ||
			strings.Contains(urlErr.Error(), "connection refused") ||
			strings.Contains(urlErr.Error(), "connection reset") ||
			strings.Contains(urlErr.Error(), "network unreachable") ||
			strings.Contains(urlErr.Error(), "timeout") ||
			strings.Contains(urlErr.Error(), "deadline exceeded") {
			retryable = true
		}
	}

	message := err.Error()
	return NewNetworkError(req.URL.String(), req.Method, message, err, timeout, retryable)
}
