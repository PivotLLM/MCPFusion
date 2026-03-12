/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	crand "crypto/rand"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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

	// Coerce JSON-encoded string values to native array/object types.
	// Some MCP clients serialize structured parameters as strings.
	coerceArgumentTypes(h.endpoint.Parameters, args, h.fusion.logger)

	// Apply declared named transforms to string parameter values.
	if err := applyParameterTransforms(h.endpoint.Parameters, args, h.fusion.logger); err != nil {
		if h.fusion.logger != nil {
			h.fusion.logger.Errorf("Parameter transform validation failed [%s]: %v", correlationID, err)
		}
		return "", err
	}

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

	// Wrap ctx with an independent deadline for the outbound request and body read.
	// The timeout defaults to 60 seconds but respects endpoint-level connection.timeout
	// overrides so slow endpoints (e.g. report generation) can use longer values.
	// Deferred here so the cancel fires AFTER handleResponse reads the body.
	outboundTimeout := 60 * time.Second
	if h.endpoint.Connection != nil && h.endpoint.Connection.Timeout != "" {
		if t, err := time.ParseDuration(h.endpoint.Connection.Timeout); err == nil {
			outboundTimeout = t
		} else if h.fusion.logger != nil {
			h.fusion.logger.Warningf("Invalid connection.timeout %q for endpoint %s, using default %v: %v",
				h.endpoint.Connection.Timeout, h.endpoint.ID, outboundTimeout, err)
		}
	}
	outboundCtx, outboundCancel := context.WithTimeout(ctx, outboundTimeout)
	defer outboundCancel()
	req = req.WithContext(outboundCtx)

	if h.fusion.logger != nil {
		h.fusion.logger.Debugf("Outbound timeout for %s [%s]: %v", h.endpoint.ID, correlationID, outboundTimeout)
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
				tenantContext.ServiceName = h.service.ServiceKey
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
	resp, requestMetrics, err := h.executeRequest(outboundCtx, req, correlationID)
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
		// Get tenant context for token refresh/invalidation
		if tenantContextValue := ctx.Value(global.TenantContextKey); tenantContextValue != nil {
			if tenantContext, ok := tenantContextValue.(*TenantContext); ok {
				if h.fusion.multiTenantAuth != nil {
					// Attempt to refresh the token before falling back to invalidation
					authConfig := h.prepareAuthConfig()

					if h.fusion.logger != nil {
						h.fusion.logger.Infof("Attempting token refresh due to %d response for tenant %s service %s [%s]",
							resp.StatusCode, tenantContext.ShortHash(), h.service.Name, correlationID)
					}

					_, refreshErr := h.fusion.multiTenantAuth.RefreshIfPossible(ctx, tenantContext, authConfig)
					if refreshErr != nil {
						// Refresh failed - fall back to invalidation
						if h.fusion.logger != nil {
							h.fusion.logger.Warningf("Token refresh failed for tenant %s service %s [%s]: %v, falling back to invalidation",
								tenantContext.ShortHash(), h.service.Name, correlationID, refreshErr)
						}
						h.fusion.multiTenantAuth.InvalidateToken(tenantContext)

						if h.fusion.logger != nil {
							h.fusion.logger.Debugf("Token invalidated due to %d response for tenant %s service %s [%s]",
								resp.StatusCode, tenantContext.ShortHash(), h.service.Name, correlationID)
						}
					} else {
						if h.fusion.logger != nil {
							h.fusion.logger.Infof("Token refresh succeeded for tenant %s service %s [%s]",
								tenantContext.ShortHash(), h.service.Name, correlationID)
						}
					}
				}

				// Retry with fresh authentication if configured
				if tokenInvalidationConfig.RetryOnInvalidation {
					// Check context cancellation before retry
					if ctx.Err() != nil {
						return "", fmt.Errorf("request cancelled before retry: %w", ctx.Err())
					}

					// Apply retry delay to avoid overwhelming the auth server
					if tokenInvalidationConfig.RetryDelay > 0 {
						select {
						case <-time.After(tokenInvalidationConfig.RetryDelay):
							// Continue with retry after delay
						case <-ctx.Done():
							return "", fmt.Errorf("request cancelled during retry delay: %w", ctx.Err())
						}
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

					// Re-apply authentication (will use refreshed token from cache, or re-authenticate if invalidated)
					tenantContext.ServiceName = h.service.ServiceKey
					tenantContext.RequestID = correlationID

					retryAuthConfig := h.prepareAuthConfig()

					if err := h.fusion.multiTenantAuth.ApplyAuthentication(ctx, retryReq, tenantContext, retryAuthConfig); err != nil {
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
					resp, retryMetrics, err = h.executeRequest(outboundCtx, retryReq, correlationID)
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
	result, err := h.handleResponse(ctx, resp, correlationID, args)
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

	// Build URL with path parameters, using endpoint-level baseURL override if set
	baseURL := h.service.BaseURL
	if h.endpoint.BaseURL != "" {
		baseURL = h.endpoint.BaseURL
	}
	requestUrl, err := mapper.BuildURL(baseURL, h.endpoint.Path, h.endpoint.Parameters, args)
	if err != nil {
		return nil, fmt.Errorf("failed to build URL: %w", err)
	}

	// Build request body
	var body io.Reader
	if h.endpoint.Method == "POST" || h.endpoint.Method == "PUT" || h.endpoint.Method == "PATCH" {
		bodyData, err := mapper.BuildRequestBody(h.endpoint.Parameters, args, h.endpoint.RequestBody)
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
					Timeout:   timeout + time.Minute,
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
			if h.fusion.logger != nil {
				h.fusion.logger.Debugf("Executing HTTP request with retry (max %d attempts): %s %s [%s]",
					retryConfig.MaxAttempts, req.Method, req.URL.String(), correlationID)
			}
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
		} else if strings.Contains(err.Error(), "context canceled") || strings.Contains(err.Error(), "context deadline exceeded") {
			metrics.ErrorCategory = ErrorCategoryTimeout
			if h.fusion.logger != nil {
				h.fusion.logger.Debugf("Request cancelled (context done) for [%s]: %v", correlationID, err)
			}
			h.fusion.ForceConnectionCleanup()
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

	// Record to shared collector for cross-package health reporting.
	// Use ServiceKey (config map key, e.g. "microsoft365") rather than
	// ServiceName (display name, e.g. "Microsoft 365") to match registration.
	if h.fusion.sharedCollector != nil && metrics != nil {
		h.fusion.sharedCollector.RecordRequest(h.service.ServiceKey, !metrics.Success)
	}

	return resp, metrics, err
}

// handleResponse processes the HTTP response
func (h *HTTPHandler) handleResponse(ctx context.Context, resp *http.Response, correlationID string, args map[string]interface{}) (string, error) {
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
		retryable := resp.StatusCode >= 500 || resp.StatusCode == 429
		return "", NewAPIErrorWithCorrelation(h.service.Name, h.endpoint.ID,
			resp.StatusCode, "API request failed", string(body), retryable, correlationID)
	}

	// Handle different response types
	switch h.endpoint.Response.Type {
	case "json":
		// Parse JSON response
		var data interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return "", fmt.Errorf("failed to parse JSON response: %w", err)
		}

		// For paginated responses without an explicit transform, extract the data array
		// using the configured dataPath. When a transform is present it is expected to
		// select the data itself, so we leave the full object intact for the transform.
		if h.endpoint.Response.Paginated &&
			h.endpoint.Response.PaginationConfig != nil &&
			h.endpoint.Response.Transform == "" {
			dataPath := h.endpoint.Response.PaginationConfig.DataPath
			if dataPath != "" {
				if obj, ok := data.(map[string]interface{}); ok {
					if pageData, exists := obj[dataPath]; exists {
						data = pageData
					}
				}
			}
		}

		// Apply transformation if specified before enforcing the size limit,
		// so that transforms can reduce an oversized response to a valid one.
		if h.endpoint.Response.Transform != "" {
			mapper := NewMapper(h.fusion.logger)
			transformed, err := mapper.TransformResponse(data, h.endpoint.Response.Transform, args)
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

		// Enforce response size limit against the final (post-transform) output.
		if h.fusion.MaxResponseBytes() > 0 && len(result) > h.fusion.MaxResponseBytes() {
			if h.fusion.logger != nil {
				h.fusion.logger.Warningf("Response size %d bytes exceeds limit of %d bytes [%s]",
					len(result), h.fusion.MaxResponseBytes(), correlationID)
			}
			return fmt.Sprintf(
				"Response too large (%d bytes, limit %d bytes). Request fewer records or fields and try again.",
				len(result), h.fusion.MaxResponseBytes(),
			), nil
		}

		return string(result), nil

	case "text":
		return string(body), nil

	case "binary":
		dlDir := h.fusion.DownloadDir()
		if dlDir == "" {
			return fmt.Sprintf("Tool call succeeded. Binary data received (%d bytes), but MCP_FUSION_DL_DIR is not configured so the data was discarded. Set MCP_FUSION_DL_DIR to enable saving binary downloads to disk.", len(body)), nil
		}

		// Step A — Determine filename and extension.
		// Prefer Content-Disposition filename; fall back to service+endpoint name.
		var baseName, ext string
		if cdFilename := filenameFromContentDisposition(resp.Header.Get("Content-Disposition")); cdFilename != "" {
			baseName = sanitizeFilename(strings.TrimSuffix(cdFilename, filepath.Ext(cdFilename)))
			ext = sanitizeFilename(filepath.Ext(cdFilename))
		} else {
			baseName = sanitizeFilename(h.service.Name) + "_" + sanitizeFilename(h.endpoint.ID)
			ext = extensionFromContentType(resp.Header.Get("Content-Type"))
		}

		// Step B — Build final filename with timestamp + 4-char random hex suffix.
		randBytes := make([]byte, 2)
		_, _ = crand.Read(randBytes)
		filename := fmt.Sprintf("%s_%s_%s%s",
			baseName,
			time.Now().Format("20060102_150405"),
			hex.EncodeToString(randBytes),
			ext,
		)

		// Step C — Tenant subdirectory.
		var subDir string
		if tc, ok := ctx.Value(global.TenantContextKey).(*TenantContext); ok && tc != nil {
			subDir = tc.ShortHash()
		}

		var saveDir string
		if subDir != "" {
			saveDir = filepath.Join(dlDir, subDir)
		} else {
			saveDir = dlDir
		}

		if err := os.MkdirAll(saveDir, 0750); err != nil {
			return "", fmt.Errorf("failed to create download directory %s: %w", saveDir, err)
		}

		filePath := filepath.Join(saveDir, filename)

		if err := os.WriteFile(filePath, body, 0640); err != nil {
			return "", fmt.Errorf("failed to write download to %s: %w", filePath, err)
		}

		if h.fusion.logger != nil {
			h.fusion.logger.Infof("Binary response saved: %s (%d bytes)", filePath, len(body))
		}

		// Step D — Return message with filename prominently included.
		return fmt.Sprintf("File saved: %s (%d bytes) → %s", filename, len(body), filePath), nil

	default:
		// Default to JSON
		return string(body), nil
	}
}

// extensionFromContentType returns a file extension for common MIME types.
func extensionFromContentType(contentType string) string {
	// Strip parameters (e.g. "; charset=utf-8")
	if i := strings.Index(contentType, ";"); i >= 0 {
		contentType = strings.TrimSpace(contentType[:i])
	}
	switch contentType {
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return ".pptx"
	case "application/pdf":
		return ".pdf"
	case "application/zip":
		return ".zip"
	case "application/octet-stream":
		return ".bin"
	case "application/msword":
		return ".doc"
	case "application/vnd.ms-excel":
		return ".xls"
	case "application/vnd.ms-powerpoint":
		return ".ppt"
	case "application/json":
		return ".json"
	case "text/plain":
		return ".txt"
	case "text/csv":
		return ".csv"
	case "text/html":
		return ".html"
	default:
		return ".bin"
	}
}

// filenameFromContentDisposition extracts the filename parameter from a
// Content-Disposition header value, e.g.:
//
//	attachment; filename="report.docx"
//	attachment; filename=report.docx
//
// Returns empty string if no filename is found.
func filenameFromContentDisposition(header string) string {
	if header == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(header)
	if err != nil {
		return ""
	}
	name := params["filename"]
	if name == "" {
		return ""
	}
	// Strip any path components for safety.
	name = filepath.Base(name)
	// Reject names that are just a separator.
	if name == "." || name == ".." || name == "/" || name == "\\" {
		return ""
	}
	return name
}

// sanitizeFilename replaces characters that are unsafe in filenames with underscores.
func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' ||
			r == '"' || r == '<' || r == '>' || r == '|' || r == ' ' {
			b.WriteRune('_')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// coerceArgumentTypes attempts to convert string values to their declared types for
// array and object parameters. Some MCP clients serialize arrays/objects as
// JSON-encoded strings rather than native JSON types; this coercion ensures
// MCPFusion handles both representations uniformly.
func coerceArgumentTypes(params []ParameterConfig, args map[string]interface{}, logger global.Logger) {
	for _, param := range params {
		if param.Type != ParameterTypeArray && param.Type != ParameterTypeObject {
			continue
		}
		val, ok := args[param.Name]
		if !ok {
			continue
		}
		str, ok := val.(string)
		if !ok {
			continue // already the right type
		}
		var parsed interface{}
		if err := json.Unmarshal([]byte(str), &parsed); err == nil {
			args[param.Name] = parsed
			if logger != nil {
				logger.Warningf("coerced parameter %q from JSON string to native %s type — MCP client should send native JSON type", param.Name, param.Type)
			}
		} else {
			if logger != nil {
				logger.Warningf("parameter %q is declared as %s but value is a non-parseable string; passing raw string to API which will likely fail: %v", param.Name, param.Type, err)
			}
		}
	}
}

// interElementWhitespace matches whitespace-only content (including at least one newline)
// between an HTML closing tag boundary (>) and the next opening tag boundary (<).
// This strips inter-element text nodes that cause null-pointer crashes in HTML-to-OOXML
// converters such as PwnDoc's html2ooxml.js.
//
// The regex is safe for content inside <pre><code> blocks because those newlines appear
// between text characters and tag boundaries, not between two tag boundaries.
// Note: whitespace-only text nodes between adjacent tags such as <pre>\n<code> are also
// stripped — this is acceptable because no supported renderer relies on that distinction.
var interElementWhitespace = regexp.MustCompile(`>[ \t]*[\r\n]+[ \t\r\n]*<`)

// applyParameterTransforms applies declared named transforms to string parameter values.
// Transforms are applied in order after argument coercion and before validation.
// Currently supported transforms:
//   - "html_compact": strips whitespace-only text nodes between HTML tags, preventing
//     crashes in HTML-to-OOXML converters (e.g. PwnDoc) that cannot handle inter-element whitespace.
//   - "html_compact_fields:<field1>,<field2>,...": for array-of-object parameters, applies
//     interElementWhitespace stripping to the specified field keys in each element.
//   - "validate_object_fields:<field1>,<field2>,...": for array-of-object parameters, validates
//     that each specified dot-path field exists and is non-empty in every element. Returns an
//     error describing the first missing or empty field found.
func applyParameterTransforms(params []ParameterConfig, args map[string]interface{}, logger global.Logger) error {
	for _, param := range params {
		if len(param.Transforms) == 0 {
			continue
		}
		val, ok := args[param.Name]
		if !ok {
			continue
		}

		for _, transform := range param.Transforms {
			// --- html_compact (string parameter) ---
			if transform == "html_compact" {
				str, ok := val.(string)
				if !ok {
					continue
				}
				compacted := interElementWhitespace.ReplaceAllString(str, "><")
				if compacted != str {
					if logger != nil {
						logger.Debugf("transform html_compact modified parameter %q: stripped inter-element whitespace", param.Name)
					}
					str = compacted
				}
				args[param.Name] = str
				val = args[param.Name]
				continue
			}

			// --- html_compact_fields:<field1>,<field2>,... (array-of-object parameter) ---
			if strings.HasPrefix(transform, "html_compact_fields:") {
				if param.Type != ParameterTypeArray || param.Items != "object" {
					continue
				}
				fieldNames := strings.Split(strings.TrimPrefix(transform, "html_compact_fields:"), ",")
				arr, ok := val.([]interface{})
				if !ok {
					if logger != nil {
						logger.Warningf("transform html_compact_fields: parameter %q is not []interface{} — skipping", param.Name)
					}
					continue
				}
				for i, elem := range arr {
					obj, ok := elem.(map[string]interface{})
					if !ok {
						if logger != nil {
							logger.Warningf("transform html_compact_fields: element %d in parameter %q is not a map — skipping", i, param.Name)
						}
						continue
					}
					for _, field := range fieldNames {
						fieldVal, exists := obj[field]
						if !exists {
							continue
						}
						fieldStr, ok := fieldVal.(string)
						if !ok {
							continue
						}
						compacted := interElementWhitespace.ReplaceAllString(fieldStr, "><")
						if compacted != fieldStr {
							obj[field] = compacted
							if logger != nil {
								logger.Debugf("transform html_compact_fields modified field %q in element %d of parameter %q", field, i, param.Name)
							}
						}
					}
				}
				continue
			}

			// --- validate_object_fields:<field1>,<field2>,... (array-of-object parameter) ---
			if strings.HasPrefix(transform, "validate_object_fields:") {
				if param.Type != ParameterTypeArray || param.Items != "object" {
					continue
				}
				fieldPaths := strings.Split(strings.TrimPrefix(transform, "validate_object_fields:"), ",")
				arr, ok := val.([]interface{})
				if !ok {
					if logger != nil {
						logger.Warningf("transform validate_object_fields: parameter %q is not []interface{} — skipping", param.Name)
					}
					continue
				}
				for i, elem := range arr {
					obj, ok := elem.(map[string]interface{})
					if !ok {
						if logger != nil {
							logger.Warningf("transform validate_object_fields: element %d in parameter %q is not a map — skipping", i, param.Name)
						}
						continue
					}
					for _, path := range fieldPaths {
						fieldVal, found := getNestedField(obj, path)
						if !found {
							return fmt.Errorf("parameter %q element %d: required field %q is missing", param.Name, i, path)
						}
						// Check for empty string
						if str, ok := fieldVal.(string); ok && str == "" {
							return fmt.Errorf("parameter %q element %d: required field %q is empty", param.Name, i, path)
						}
						// Check for nil
						if fieldVal == nil {
							return fmt.Errorf("parameter %q element %d: required field %q is nil", param.Name, i, path)
						}
					}
				}
				continue
			}

			// --- unknown transform ---
			if logger != nil {
				logger.Warningf("unknown transform %q declared for parameter %q — skipping", transform, param.Name)
			}
		}
	}
	return nil
}

// getNestedField retrieves a value from obj using a dot-separated path.
// Splits on the first dot only, supporting two-level paths like "customField._id".
// Returns (value, true) if found at every level, (nil, false) if any key is missing.
func getNestedField(obj map[string]interface{}, dotPath string) (interface{}, bool) {
	dotIdx := strings.Index(dotPath, ".")
	if dotIdx < 0 {
		val, ok := obj[dotPath]
		return val, ok
	}
	parent := dotPath[:dotIdx]
	child := dotPath[dotIdx+1:]
	parentVal, ok := obj[parent]
	if !ok {
		return nil, false
	}
	parentMap, ok := parentVal.(map[string]interface{})
	if !ok {
		return nil, false
	}
	val, ok := parentMap[child]
	return val, ok
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

