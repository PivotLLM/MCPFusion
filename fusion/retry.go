/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
)

// RetryExecutor handles retry logic for HTTP requests
type RetryExecutor struct {
	config *RetryConfig
	logger global.Logger
	rand   *rand.Rand
	mu     sync.Mutex
}

// NewRetryExecutor creates a new retry executor
func NewRetryExecutor(config *RetryConfig, logger global.Logger) *RetryExecutor {
	return &RetryExecutor{
		config: config,
		logger: logger,
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Execute executes an HTTP request with retry logic
func (r *RetryExecutor) Execute(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	if !r.config.Enabled {
		// No retry, execute once
		resp, err := client.Do(req)
		if err != nil {
			err = r.wrapNetworkError(err, req)
		}
		return resp, err
	}

	var lastErr error
	var lastResp *http.Response

	for attempt := 0; attempt < r.config.MaxAttempts; attempt++ {
		// Clone the request for retry attempts (in case body needs to be read again)
		clonedReq := r.cloneRequest(req)

		if r.logger != nil && attempt > 0 {
			r.logger.Infof("Retry attempt %d/%d for %s %s",
				attempt+1, r.config.MaxAttempts, req.Method, req.URL.String())
		}

		// Execute the request
		resp, err := client.Do(clonedReq)

		// Wrap network errors in NetworkError type
		if err != nil {
			err = r.wrapNetworkError(err, clonedReq)
		}

		// Check if we should retry
		shouldRetry, retryReason := r.shouldRetry(err, resp, attempt)

		if !shouldRetry {
			if r.logger != nil && attempt > 0 {
				r.logger.Infof("Request succeeded after %d attempts", attempt+1)
			}
			return resp, err
		}

		// Store the last error and response for potential return
		lastErr = err
		if lastResp != nil {
			lastResp.Body.Close()
		}
		lastResp = resp

		// Log retry reason
		if r.logger != nil {
			r.logger.Warningf("Request failed (attempt %d/%d): %s. Will retry after delay.",
				attempt+1, r.config.MaxAttempts, retryReason)
		}

		// Don't wait after the last attempt
		if attempt < r.config.MaxAttempts-1 {
			delay := r.calculateDelay(attempt)
			if r.logger != nil {
				r.logger.Debugf("Waiting %v before retry", delay)
			}

			select {
			case <-time.After(delay):
				// Continue to next attempt
			case <-ctx.Done():
				if resp != nil {
					resp.Body.Close()
				}
				return nil, ctx.Err()
			}
		}
	}

	// All retries exhausted
	if r.logger != nil {
		r.logger.Errorf("All %d retry attempts exhausted for %s %s",
			r.config.MaxAttempts, req.Method, req.URL.String())
	}

	// Return the last response and error
	return lastResp, lastErr
}

// shouldRetry determines if a request should be retried
func (r *RetryExecutor) shouldRetry(err error, resp *http.Response, attempt int) (bool, string) {
	// Don't retry if we've reached max attempts
	if attempt >= r.config.MaxAttempts-1 {
		return false, "max attempts reached"
	}

	// Check for network errors
	if err != nil {
		errorType := r.categorizeError(err)
		if r.isRetryableError(errorType) {
			return true, fmt.Sprintf("network error: %s (%s)", err.Error(), errorType)
		}
		return false, fmt.Sprintf("non-retryable error: %s", err.Error())
	}

	// Check HTTP status codes
	if resp != nil {
		switch {
		case resp.StatusCode >= 500: // Server errors
			return true, fmt.Sprintf("server error: HTTP %d", resp.StatusCode)
		case resp.StatusCode == 429: // Rate limited
			return true, fmt.Sprintf("rate limited: HTTP %d", resp.StatusCode)
		case resp.StatusCode == 408: // Request timeout
			return true, fmt.Sprintf("request timeout: HTTP %d", resp.StatusCode)
		case resp.StatusCode >= 400: // Other client errors
			return false, fmt.Sprintf("client error: HTTP %d", resp.StatusCode)
		default:
			return false, fmt.Sprintf("success: HTTP %d", resp.StatusCode)
		}
	}

	return false, "unknown condition"
}

// categorizeError categorizes an error for retry decision
func (r *RetryExecutor) categorizeError(err error) string {
	if err == nil {
		return "none"
	}

	errStr := strings.ToLower(err.Error())

	// Network connectivity issues
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "network unreachable") {
		return "network_error"
	}

	// Timeout issues
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") {
		return "timeout"
	}

	// TLS/SSL issues (might be temporary)
	if strings.Contains(errStr, "tls") ||
		strings.Contains(errStr, "certificate") {
		return "tls_error"
	}

	// DNS issues
	if strings.Contains(errStr, "dns") ||
		strings.Contains(errStr, "lookup") {
		return "dns_error"
	}

	// Generic I/O errors
	if strings.Contains(errStr, "i/o") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection aborted") {
		return "io_error"
	}

	return "unknown_error"
}

// isRetryableError checks if an error type is retryable based on configuration
func (r *RetryExecutor) isRetryableError(errorType string) bool {
	if len(r.config.RetryableErrors) == 0 {
		// Default retryable error types
		defaultRetryable := []string{
			"network_error", "timeout", "dns_error", "io_error",
		}
		for _, retryableType := range defaultRetryable {
			if errorType == retryableType {
				return true
			}
		}
		return false
	}

	// Check against configured retryable error types
	for _, retryableType := range r.config.RetryableErrors {
		if errorType == retryableType {
			return true
		}
	}
	return false
}

// calculateDelay calculates the delay before the next retry attempt
func (r *RetryExecutor) calculateDelay(attempt int) time.Duration {
	var delay time.Duration

	switch r.config.Strategy {
	case RetryStrategyFixed:
		delay = r.config.BaseDelay

	case RetryStrategyLinear:
		delay = r.config.BaseDelay * time.Duration(attempt+1)

	case RetryStrategyExponential:
		fallthrough
	default:
		// Exponential backoff: baseDelay * (backoffFactor ^ attempt)
		multiplier := math.Pow(r.config.BackoffFactor, float64(attempt))
		delay = time.Duration(float64(r.config.BaseDelay) * multiplier)
	}

	// Apply maximum delay limit
	if r.config.MaxDelay > 0 && delay > r.config.MaxDelay {
		delay = r.config.MaxDelay
	}

	// Apply jitter to prevent thundering herd
	if r.config.Jitter {
		jitter := r.generateJitter(delay)
		delay = time.Duration(float64(delay) * (0.5 + jitter*0.5)) // ±50% jitter
	}

	return delay
}

// generateJitter generates a random jitter value between 0 and 1
func (r *RetryExecutor) generateJitter(delay time.Duration) float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rand.Float64()
}

// cloneRequest creates a copy of an HTTP request for retry
func (r *RetryExecutor) cloneRequest(req *http.Request) *http.Request {
	// Clone the request
	cloned := req.Clone(req.Context())

	// Note: We don't clone the body here because it's more complex
	// In practice, most retryable requests are GET requests without body
	// For POST/PUT requests with body, the caller should handle body cloning

	return cloned
}

// wrapNetworkError wraps network errors in NetworkError type
func (r *RetryExecutor) wrapNetworkError(err error, req *http.Request) error {
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
	if errors.As(err, &urlErr) {
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

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

// String returns the string representation of circuit breaker state
func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerClosed:
		return "CLOSED"
	case CircuitBreakerOpen:
		return "OPEN"
	case CircuitBreakerHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config        *CircuitBreakerConfig
	logger        global.Logger
	mu            sync.RWMutex
	state         CircuitBreakerState
	failureCount  int
	successCount  int
	lastFailure   time.Time
	nextRetry     time.Time
	halfOpenCalls int
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig, logger global.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		logger: logger,
		state:  CircuitBreakerClosed,
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if !cb.config.Enabled {
		// Circuit breaker disabled, execute directly
		return fn()
	}

	// Check if we can execute
	if err := cb.beforeCall(); err != nil {
		return err
	}

	// Execute the function
	err := fn()

	// Record the result
	cb.afterCall(err)

	return err
}

// beforeCall checks if a call can be made
func (cb *CircuitBreaker) beforeCall() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitBreakerClosed:
		// Calls are allowed
		return nil

	case CircuitBreakerOpen:
		// Check if we should transition to half-open
		if time.Now().After(cb.nextRetry) {
			cb.state = CircuitBreakerHalfOpen
			cb.halfOpenCalls = 0
			if cb.logger != nil {
				cb.logger.Infof("Circuit breaker transitioning to HALF_OPEN state")
			}
			return nil
		}
		// Still in open state, reject the call
		return NewCircuitBreakerError("circuit breaker is OPEN", cb.nextRetry)

	case CircuitBreakerHalfOpen:
		// Allow limited calls in half-open state
		if cb.halfOpenCalls >= cb.config.HalfOpenMaxCalls {
			return NewCircuitBreakerError("circuit breaker HALF_OPEN: max calls reached", cb.nextRetry)
		}
		cb.halfOpenCalls++
		return nil

	default:
		return fmt.Errorf("unknown circuit breaker state: %v", cb.state)
	}
}

// afterCall records the result of a call
func (cb *CircuitBreaker) afterCall(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
}

// recordFailure records a failed call
func (cb *CircuitBreaker) recordFailure() {
	cb.failureCount++
	cb.lastFailure = time.Now()

	switch cb.state {
	case CircuitBreakerClosed:
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.state = CircuitBreakerOpen
			cb.nextRetry = time.Now().Add(cb.config.ResetTimeout)
			if cb.logger != nil {
				cb.logger.Warningf("Circuit breaker OPENED: %d failures reached threshold (%d)",
					cb.failureCount, cb.config.FailureThreshold)
			}
		}

	case CircuitBreakerHalfOpen:
		// Failure in half-open state, go back to open
		cb.state = CircuitBreakerOpen
		cb.nextRetry = time.Now().Add(cb.config.ResetTimeout)
		if cb.logger != nil {
			cb.logger.Warningf("Circuit breaker back to OPEN: failure in HALF_OPEN state")
		}
	}
}

// recordSuccess records a successful call
func (cb *CircuitBreaker) recordSuccess() {
	switch cb.state {
	case CircuitBreakerClosed:
		// Reset failure count on success
		if cb.failureCount > 0 {
			cb.failureCount = 0
			if cb.logger != nil {
				cb.logger.Debugf("Circuit breaker: failure count reset after success")
			}
		}

	case CircuitBreakerHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.config.SuccessThreshold {
			// Transition back to closed
			cb.state = CircuitBreakerClosed
			cb.failureCount = 0
			cb.successCount = 0
			if cb.logger != nil {
				cb.logger.Infof("Circuit breaker CLOSED: %d consecutive successes in HALF_OPEN",
					cb.successCount)
			}
		}
	}
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetMetrics returns circuit breaker metrics
func (cb *CircuitBreaker) GetMetrics() CircuitBreakerMetrics {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerMetrics{
		State:        cb.state,
		FailureCount: cb.failureCount,
		SuccessCount: cb.successCount,
		LastFailure:  cb.lastFailure,
		NextRetry:    cb.nextRetry,
	}
}

// CircuitBreakerMetrics contains circuit breaker metrics
type CircuitBreakerMetrics struct {
	State        CircuitBreakerState
	FailureCount int
	SuccessCount int
	LastFailure  time.Time
	NextRetry    time.Time
}

// CircuitBreakerError represents a circuit breaker error
type CircuitBreakerError struct {
	Message   string    `json:"message"`
	NextRetry time.Time `json:"next_retry"`
}

// Error implements the error interface
func (e *CircuitBreakerError) Error() string {
	if !e.NextRetry.IsZero() {
		return fmt.Sprintf("%s (next retry at %s)", e.Message, e.NextRetry.Format(time.RFC3339))
	}
	return e.Message
}

// NewCircuitBreakerError creates a new circuit breaker error
func NewCircuitBreakerError(message string, nextRetry time.Time) *CircuitBreakerError {
	return &CircuitBreakerError{
		Message:   message,
		NextRetry: nextRetry,
	}
}
