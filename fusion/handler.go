/*=============================================================================
= Copyright (c) 2025 Tenebris Technologies Inc.                              =
= All rights reserved.                                                       =
=============================================================================*/

package fusion

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPHandler handles HTTP requests for a specific endpoint
type HTTPHandler struct {
	service  *ServiceConfig
	endpoint *EndpointConfig
	fusion   *Fusion
}

// NewHTTPHandler creates a new HTTP handler for an endpoint
func NewHTTPHandler(fusion *Fusion, service *ServiceConfig, endpoint *EndpointConfig) *HTTPHandler {
	return &HTTPHandler{
		service:  service,
		endpoint: endpoint,
		fusion:   fusion,
	}
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

	// Validate parameters
	validator := NewValidator(h.fusion.logger)
	if err := validator.ValidateParameters(h.endpoint.Parameters, args); err != nil {
		if h.fusion.logger != nil {
			h.fusion.logger.Errorf("Parameter validation failed [%s]: %v", correlationID, err)
		}
		return "", err
	}

	// Check cache if enabled
	var cacheKey string
	var cacheHit bool
	if h.endpoint.Response.Caching != nil && h.endpoint.Response.Caching.Enabled {
		cacheKey = h.generateCacheKey(args)
		if cachedResult, err := h.fusion.cache.Get(cacheKey); err == nil {
			if h.fusion.logger != nil {
				h.fusion.logger.Debugf("Cache hit for %s.%s with key %s [%s]", h.service.Name, h.endpoint.ID, cacheKey, correlationID)
			}
			if resultStr, ok := cachedResult.(string); ok {
				cacheHit = true
				// Record cache hit in metrics
				if h.fusion.metricsCollector != nil {
					cacheMetrics := RequestMetrics{
						ServiceName:   h.service.Name,
						EndpointID:    h.endpoint.ID,
						Method:        "GET", // Cache hits are typically for GET requests
						Success:       true,
						CacheHit:      true,
						Latency:       time.Since(startTime),
						CorrelationID: correlationID,
						Timestamp:     startTime,
					}
					h.fusion.metricsCollector.RecordRequest(cacheMetrics)
				}
				return resultStr, nil
			}
		}
	}

	// Build request
	req, err := h.buildRequest(ctx, args)
	if err != nil {
		if h.fusion.logger != nil {
			h.fusion.logger.Errorf("Failed to build request [%s]: %v", correlationID, err)
		}
		return "", err
	}

	// For now, skip authentication since we don't have a proper legacy auth manager
	// This needs to be properly implemented with multi-tenant support
	if h.fusion.logger != nil {
		h.fusion.logger.Warningf("Authentication temporarily disabled - requires multi-tenant implementation [%s]", correlationID)
	}

	// Log final request after authentication is applied
	if h.fusion.logger != nil {
		h.fusion.logger.Debugf("\n=== Final Request (with auth) ===")
		h.fusion.logger.Debugf("Method: %s", req.Method)
		h.fusion.logger.Debugf("URL: %s", req.URL.String())
		h.fusion.logger.Debugf("Headers after auth:")
		for name, values := range req.Header {
			// Redact sensitive headers but show they exist
			if strings.Contains(strings.ToLower(name), "authorization") || 
			   strings.Contains(strings.ToLower(name), "api-key") ||
			   strings.Contains(strings.ToLower(name), "token") ||
			   strings.Contains(strings.ToLower(name), "x-api") {
				if len(values) > 0 {
					h.fusion.logger.Debugf("  %s: [REDACTED - length %d]", name, len(values[0]))
				}
			} else {
				for _, value := range values {
					h.fusion.logger.Debugf("  %s: %s", name, value)
				}
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
	defer resp.Body.Close()

	// Handle response
	result, err := h.handleResponse(resp, correlationID)
	if err != nil {
		if h.fusion.logger != nil {
			h.fusion.logger.Errorf("Response handling failed [%s]: %v", correlationID, err)
		}
		return "", err
	}

	// Cache the result if caching is enabled
	if h.endpoint.Response.Caching != nil && h.endpoint.Response.Caching.Enabled && !cacheHit {
		ttl := h.endpoint.Response.Caching.TTL
		if ttl == 0 {
			ttl = 5 * time.Minute // Default TTL
		}

		if err := h.fusion.cache.Set(cacheKey, result, ttl); err != nil {
			if h.fusion.logger != nil {
				h.fusion.logger.Warningf("Failed to cache result for %s.%s [%s]: %v", h.service.Name, h.endpoint.ID, correlationID, err)
			}
		} else if h.fusion.logger != nil {
			h.fusion.logger.Debugf("Cached result for %s.%s with key %s (TTL: %v) [%s]", h.service.Name, h.endpoint.ID, cacheKey, ttl, correlationID)
		}
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
	url, err := mapper.BuildURL(h.service.BaseURL, h.endpoint.Path, h.endpoint.Parameters, args)
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
	req, err := http.NewRequestWithContext(ctx, h.endpoint.Method, url, body)
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

	// Execute with circuit breaker if enabled
	var resp *http.Response
	var err error

	executeFunc := func() error {
		if retryConfig.Enabled {
			// Use retry executor
			retryExecutor := NewRetryExecutor(retryConfig, h.fusion.logger)
			resp, err = retryExecutor.Execute(ctx, h.fusion.httpClient, req)
			if err != nil && resp == nil {
				// Count retry attempts from the error context
				metrics.RetryCount = retryConfig.MaxAttempts - 1
			}
		} else {
			// Single attempt without retry
			if h.fusion.logger != nil {
				h.fusion.logger.Debugf("Executing HTTP request: %s %s", req.Method, req.URL.String())
			}
			resp, err = h.fusion.httpClient.Do(req)
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
		} else if strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "network") {
			metrics.ErrorCategory = ErrorCategoryNetwork
		} else if strings.Contains(err.Error(), "circuit breaker") {
			metrics.ErrorCategory = ErrorCategoryServer
		} else {
			metrics.ErrorCategory = ErrorCategoryPermanent
		}

		// Enhance error with correlation ID if it's a network error
		if netErr, ok := err.(*NetworkError); ok && netErr.CorrelationID == "" {
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
		return "", NewAPIErrorWithCorrelation(h.service.Name, h.endpoint.ID, resp.StatusCode, "API request failed", string(body), false, correlationID)
	}

	// Handle different response types
	switch h.endpoint.Response.Type {
	case "json":
		// Parse JSON response
		var data interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return "", fmt.Errorf("failed to parse JSON response: %w", err)
		}

		// Handle pagination if configured
		if h.endpoint.Response.Paginated && h.endpoint.Response.PaginationConfig != nil {
			return h.handlePaginatedResponse(data)
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

// handlePaginatedResponse handles paginated API responses
func (h *HTTPHandler) handlePaginatedResponse(data interface{}) (string, error) {
	mapper := NewMapper(h.fusion.logger)
	config := h.endpoint.Response.PaginationConfig

	// Extract pagination info from the first response
	nextPageToken, currentData, err := mapper.ExtractPaginationInfo(data, *config)
	if err != nil {
		return "", fmt.Errorf("failed to extract pagination info: %w", err)
	}

	allData := currentData
	pageCount := 1
	maxPages := 10 // Limit to prevent infinite loops

	if h.fusion.logger != nil {
		h.fusion.logger.Debugf("Starting pagination: first page has %d items, next token: %s",
			len(currentData), nextPageToken)
	}

	// Fetch additional pages if they exist
	for nextPageToken != "" && pageCount < maxPages {
		if h.fusion.logger != nil {
			h.fusion.logger.Debugf("Fetching page %d with token: %s", pageCount+1, nextPageToken)
		}

		nextData, nextToken, err := h.fetchNextPage(nextPageToken)
		if err != nil {
			if h.fusion.logger != nil {
				h.fusion.logger.Warningf("Failed to fetch page %d, stopping pagination: %v", pageCount+1, err)
			}
			break
		}

		allData = append(allData, nextData...)
		nextPageToken = nextToken
		pageCount++
	}

	if h.fusion.logger != nil {
		h.fusion.logger.Infof("Pagination completed: %d pages, %d total items", pageCount, len(allData))
	}

	// Apply transformation if specified
	var finalData interface{} = allData
	if h.endpoint.Response.Transform != "" {
		// For paginated responses, we need to structure the data properly for transformation
		structuredData := map[string]interface{}{
			config.DataPath: allData,
		}
		transformed, err := mapper.TransformResponse(structuredData, h.endpoint.Response.Transform)
		if err != nil {
			return "", fmt.Errorf("failed to transform paginated response: %w", err)
		}
		finalData = transformed
	}

	// Convert to JSON string
	result, err := json.MarshalIndent(finalData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal paginated response: %w", err)
	}

	return string(result), nil
}

// fetchNextPage fetches the next page of a paginated response
func (h *HTTPHandler) fetchNextPage(nextPageURL string) ([]interface{}, string, error) {
	// Create a new request for the next page
	req, err := http.NewRequest("GET", nextPageURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create next page request: %w", err)
	}

	// Authentication for pagination is not yet implemented in multi-tenant mode
	// TODO: Implement multi-tenant authentication for pagination requests
	if h.fusion.logger != nil {
		h.fusion.logger.Warning("Pagination authentication not implemented in multi-tenant mode")
	}

	// Set headers
	req.Header.Set("Accept", "application/json")

	// Execute the request - for pagination, we'll use a simplified approach
	resp, err := h.fusion.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to execute next page request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read next page response body: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("next page request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, "", fmt.Errorf("failed to parse next page JSON response: %w", err)
	}

	// Extract pagination info
	mapper := NewMapper(h.fusion.logger)
	config := h.endpoint.Response.PaginationConfig
	nextToken, pageData, err := mapper.ExtractPaginationInfo(data, *config)
	if err != nil {
		return nil, "", fmt.Errorf("failed to extract next page pagination info: %w", err)
	}

	return pageData, nextToken, nil
}

// generateCacheKey generates a cache key for the request
func (h *HTTPHandler) generateCacheKey(args map[string]interface{}) string {
	// Use custom cache key template if provided
	if h.endpoint.Response.Caching != nil && h.endpoint.Response.Caching.Key != "" {
		// Simple template replacement - could be enhanced with a template engine
		key := h.endpoint.Response.Caching.Key
		for _, value := range args {
			key = fmt.Sprintf(key, value) // Simple approach for now
		}
		return fmt.Sprintf("fusion:%s:%s:%s", h.service.Name, h.endpoint.ID, key)
	}

	// Generate a hash-based cache key from the arguments
	hasher := sha256.New()

	// Include service and endpoint info
	hasher.Write([]byte(h.service.Name))
	hasher.Write([]byte(":"))
	hasher.Write([]byte(h.endpoint.ID))
	hasher.Write([]byte(":"))

	// Include all argument values in a deterministic way
	argData, err := json.Marshal(args)
	if err != nil {
		// Fallback if marshaling fails
		hasher.Write([]byte(fmt.Sprintf("%v", args)))
	} else {
		hasher.Write(argData)
	}

	// Generate hash
	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	return fmt.Sprintf("fusion:%s:%s:%s", h.service.Name, h.endpoint.ID, hash[:16])
}

// wrapNetworkError wraps network errors in NetworkError type
func (h *HTTPHandler) wrapNetworkError(err error, req *http.Request) error {
	if err == nil {
		return nil
	}

	// Check if it's already a NetworkError (avoid double wrapping)
	if _, ok := err.(*NetworkError); ok {
		return err
	}

	// Determine if it's a timeout error
	timeout := false
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		timeout = true
	}

	// Determine if it's retryable
	retryable := true

	// Check for specific error types
	if urlErr, ok := err.(*url.Error); ok {
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
