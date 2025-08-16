/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PivotLLM/MCPFusion/mlogger"
)

func TestRetryExecutor(t *testing.T) {
	logger, _ := mlogger.New() // Use default logger

	tests := []struct {
		name           string
		config         *RetryConfig
		serverResponse func(attempts *int) http.HandlerFunc
		expectSuccess  bool
		expectAttempts int
		expectError    string
	}{
		{
			name: "retry disabled - single success",
			config: &RetryConfig{
				Enabled:     false,
				MaxAttempts: 3,
			},
			serverResponse: func(attempts *int) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					*attempts++
					w.WriteHeader(200)
					w.Write([]byte("success"))
				}
			},
			expectSuccess:  true,
			expectAttempts: 1,
		},
		{
			name: "retry disabled - single failure",
			config: &RetryConfig{
				Enabled:     false,
				MaxAttempts: 3,
			},
			serverResponse: func(attempts *int) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					*attempts++
					w.WriteHeader(500)
					w.Write([]byte("server error"))
				}
			},
			expectSuccess:  false,
			expectAttempts: 1,
		},
		{
			name: "retry enabled - success after retries",
			config: &RetryConfig{
				Enabled:       true,
				MaxAttempts:   3,
				Strategy:      RetryStrategyFixed,
				BaseDelay:     10 * time.Millisecond,
				Jitter:        false,
				BackoffFactor: 2.0,
			},
			serverResponse: func(attempts *int) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					*attempts++
					if *attempts < 3 {
						w.WriteHeader(500) // Fail first two attempts
					} else {
						w.WriteHeader(200) // Success on third attempt
						w.Write([]byte("success"))
					}
				}
			},
			expectSuccess:  true,
			expectAttempts: 3,
		},
		{
			name: "retry enabled - all attempts fail",
			config: &RetryConfig{
				Enabled:       true,
				MaxAttempts:   3,
				Strategy:      RetryStrategyFixed,
				BaseDelay:     10 * time.Millisecond,
				Jitter:        false,
				BackoffFactor: 2.0,
			},
			serverResponse: func(attempts *int) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					*attempts++
					w.WriteHeader(500)
					w.Write([]byte("server error"))
				}
			},
			expectSuccess:  false,
			expectAttempts: 3,
		},
		{
			name: "retry enabled - client error (no retry)",
			config: &RetryConfig{
				Enabled:       true,
				MaxAttempts:   3,
				Strategy:      RetryStrategyFixed,
				BaseDelay:     10 * time.Millisecond,
				Jitter:        false,
				BackoffFactor: 2.0,
			},
			serverResponse: func(attempts *int) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					*attempts++
					w.WriteHeader(400) // Client error - should not retry
					w.Write([]byte("bad request"))
				}
			},
			expectSuccess:  false,
			expectAttempts: 1,
		},
		{
			name: "retry enabled - rate limited (should retry)",
			config: &RetryConfig{
				Enabled:       true,
				MaxAttempts:   3,
				Strategy:      RetryStrategyFixed,
				BaseDelay:     10 * time.Millisecond,
				Jitter:        false,
				BackoffFactor: 2.0,
			},
			serverResponse: func(attempts *int) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					*attempts++
					if *attempts < 3 {
						w.WriteHeader(429) // Rate limited - should retry
					} else {
						w.WriteHeader(200) // Success on third attempt
						w.Write([]byte("success"))
					}
				}
			},
			expectSuccess:  true,
			expectAttempts: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempts := 0
			server := httptest.NewServer(tt.serverResponse(&attempts))
			defer server.Close()

			retryExecutor := NewRetryExecutor(tt.config, logger)
			client := &http.Client{Timeout: 5 * time.Second}

			req, err := http.NewRequest("GET", server.URL, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			resp, err := retryExecutor.Execute(ctx, client, req)

			if tt.expectSuccess {
				if err != nil {
					t.Errorf("Expected success but got error: %v", err)
				}
				if resp == nil {
					t.Errorf("Expected response but got nil")
				} else if resp.StatusCode != 200 {
					t.Errorf("Expected status 200 but got %d", resp.StatusCode)
				}
			} else {
				if err == nil && (resp == nil || resp.StatusCode < 400) {
					t.Errorf("Expected failure but got success")
				}
			}

			if attempts != tt.expectAttempts {
				t.Errorf("Expected %d attempts but got %d", tt.expectAttempts, attempts)
			}

			if resp != nil {
				resp.Body.Close()
			}
		})
	}
}

func TestRetryStrategies(t *testing.T) {
	logger, _ := mlogger.New()

	tests := []struct {
		name     string
		strategy RetryStrategy
		attempts []int
		minDelay time.Duration
		maxDelay time.Duration
	}{
		{
			name:     "fixed strategy",
			strategy: RetryStrategyFixed,
			attempts: []int{0, 1, 2},
			minDelay: 100 * time.Millisecond,
			maxDelay: 100 * time.Millisecond, // Should be same for fixed
		},
		{
			name:     "linear strategy",
			strategy: RetryStrategyLinear,
			attempts: []int{0, 1, 2},
			minDelay: 100 * time.Millisecond, // First retry
			maxDelay: 300 * time.Millisecond, // Third retry (3x base)
		},
		{
			name:     "exponential strategy",
			strategy: RetryStrategyExponential,
			attempts: []int{0, 1, 2},
			minDelay: 100 * time.Millisecond, // First retry
			maxDelay: 400 * time.Millisecond, // Third retry (2^2 * 100ms)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &RetryConfig{
				Enabled:       true,
				MaxAttempts:   3,
				Strategy:      tt.strategy,
				BaseDelay:     100 * time.Millisecond,
				Jitter:        false, // Disable jitter for predictable testing
				BackoffFactor: 2.0,
			}

			executor := NewRetryExecutor(config, logger)

			for _, attempt := range tt.attempts {
				delay := executor.calculateDelay(attempt)

				switch tt.strategy {
				case RetryStrategyFixed:
					if delay != config.BaseDelay {
						t.Errorf("Fixed strategy attempt %d: expected %v, got %v", attempt, config.BaseDelay, delay)
					}
				case RetryStrategyLinear:
					expected := config.BaseDelay * time.Duration(attempt+1)
					if delay != expected {
						t.Errorf("Linear strategy attempt %d: expected %v, got %v", attempt, expected, delay)
					}
				case RetryStrategyExponential:
					// For exponential: baseDelay * (backoffFactor ^ attempt)
					expectedMultiplier := 1.0
					for i := 0; i < attempt; i++ {
						expectedMultiplier *= config.BackoffFactor
					}
					expected := time.Duration(float64(config.BaseDelay) * expectedMultiplier)
					if delay != expected {
						t.Errorf("Exponential strategy attempt %d: expected %v, got %v", attempt, expected, delay)
					}
				}
			}
		})
	}
}

func TestCircuitBreaker(t *testing.T) {
	logger, _ := mlogger.New()

	t.Run("circuit breaker states", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 3,
			SuccessThreshold: 2,
			Timeout:          30 * time.Second,
			HalfOpenMaxCalls: 2,
			ResetTimeout:     100 * time.Millisecond,
		}

		cb := NewCircuitBreaker(config, logger)

		// Initially closed
		if cb.GetState() != CircuitBreakerClosed {
			t.Errorf("Expected initial state CLOSED, got %v", cb.GetState())
		}

		// Test failures - should transition to OPEN after threshold
		ctx := context.Background()
		for i := 0; i < 3; i++ {
			err := cb.Execute(ctx, func() error {
				return errors.New("test failure")
			})
			if err == nil {
				t.Errorf("Expected error on failure %d", i+1)
			}
		}

		// Should be OPEN now
		if cb.GetState() != CircuitBreakerOpen {
			t.Errorf("Expected state OPEN after failures, got %v", cb.GetState())
		}

		// Calls should be rejected immediately
		err := cb.Execute(ctx, func() error {
			return nil // This shouldn't be called
		})
		if err == nil || !strings.Contains(err.Error(), "circuit breaker is OPEN") {
			t.Errorf("Expected circuit breaker OPEN error, got: %v", err)
		}

		// Wait for reset timeout
		time.Sleep(150 * time.Millisecond)

		// Should transition to HALF_OPEN on next call
		cb.Execute(ctx, func() error {
			return nil // Success
		})

		if cb.GetState() != CircuitBreakerHalfOpen {
			t.Errorf("Expected state HALF_OPEN after reset timeout, got %v", cb.GetState())
		}

		// One more success should close the circuit
		cb.Execute(ctx, func() error {
			return nil // Success
		})

		if cb.GetState() != CircuitBreakerClosed {
			t.Errorf("Expected state CLOSED after successes, got %v", cb.GetState())
		}
	})

	t.Run("circuit breaker metrics", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 2,
			SuccessThreshold: 2,
			Timeout:          30 * time.Second,
			HalfOpenMaxCalls: 2,
			ResetTimeout:     100 * time.Millisecond,
		}

		cb := NewCircuitBreaker(config, logger)
		ctx := context.Background()

		// Test some failures
		cb.Execute(ctx, func() error { return errors.New("fail") })
		cb.Execute(ctx, func() error { return errors.New("fail") })

		metrics := cb.GetMetrics()
		if metrics.FailureCount != 2 {
			t.Errorf("Expected 2 failures, got %d", metrics.FailureCount)
		}
		if metrics.State != CircuitBreakerOpen {
			t.Errorf("Expected OPEN state, got %v", metrics.State)
		}
	})

	t.Run("circuit breaker disabled", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			Enabled: false,
		}

		cb := NewCircuitBreaker(config, logger)
		ctx := context.Background()

		// Should execute directly without protection
		callCount := 0
		err := cb.Execute(ctx, func() error {
			callCount++
			return errors.New("test error")
		})

		if err == nil || !strings.Contains(err.Error(), "test error") {
			t.Errorf("Expected test error, got: %v", err)
		}
		if callCount != 1 {
			t.Errorf("Expected 1 call, got %d", callCount)
		}
	})
}

func TestErrorCategorization(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   ErrorCategory
	}{
		{200, ErrorCategoryPermanent}, // Success codes default to permanent
		{400, ErrorCategoryClient},
		{401, ErrorCategoryAuth},
		{403, ErrorCategoryAuth},
		{404, ErrorCategoryClient},
		{408, ErrorCategoryTimeout},
		{429, ErrorCategoryRateLimit},
		{500, ErrorCategoryServer},
		{502, ErrorCategoryServer},
		{503, ErrorCategoryServer},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.statusCode), func(t *testing.T) {
			category := categorizeHTTPError(tt.statusCode)
			if category != tt.expected {
				t.Errorf("Status %d: expected %v, got %v", tt.statusCode, tt.expected, category)
			}
		})
	}
}

func TestAPIErrorWithCorrelation(t *testing.T) {
	correlationID := "test-correlation-123"
	apiErr := NewAPIErrorWithCorrelation("test-service", "test-endpoint", 500, "test message", "response body", true, correlationID)

	if apiErr.CorrelationID != correlationID {
		t.Errorf("Expected correlation ID %s, got %s", correlationID, apiErr.CorrelationID)
	}

	if apiErr.Category != ErrorCategoryServer {
		t.Errorf("Expected category %v, got %v", ErrorCategoryServer, apiErr.Category)
	}

	if !apiErr.IsRetryable() {
		t.Errorf("Expected error to be retryable")
	}

	if !apiErr.IsTransient() {
		t.Errorf("Expected server error to be transient")
	}

	errorMsg := apiErr.Error()
	if !strings.Contains(errorMsg, correlationID) {
		t.Errorf("Expected error message to contain correlation ID, got: %s", errorMsg)
	}
}

func TestNetworkErrorWithCorrelation(t *testing.T) {
	correlationID := "test-correlation-456"
	netErr := NewNetworkErrorWithCorrelation("https://example.com", "GET", "connection failed", nil, false, true, correlationID)

	if netErr.CorrelationID != correlationID {
		t.Errorf("Expected correlation ID %s, got %s", correlationID, netErr.CorrelationID)
	}

	if netErr.Category != ErrorCategoryNetwork {
		t.Errorf("Expected category %v, got %v", ErrorCategoryNetwork, netErr.Category)
	}

	if !netErr.IsRetryable() {
		t.Errorf("Expected error to be retryable")
	}

	if netErr.IsTimeout() {
		t.Errorf("Expected error to not be timeout")
	}
}

func TestRetryConfigDefaults(t *testing.T) {
	configJSON := `{
		"enabled": true
	}`

	var config RetryConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// Check defaults
	if config.MaxAttempts != 3 {
		t.Errorf("Expected default MaxAttempts 3, got %d", config.MaxAttempts)
	}
	if config.Strategy != RetryStrategyExponential {
		t.Errorf("Expected default strategy %v, got %v", RetryStrategyExponential, config.Strategy)
	}
	if config.BaseDelay != time.Second {
		t.Errorf("Expected default BaseDelay %v, got %v", time.Second, config.BaseDelay)
	}
	if config.BackoffFactor != 2.0 {
		t.Errorf("Expected default BackoffFactor 2.0, got %f", config.BackoffFactor)
	}
}

func TestCircuitBreakerConfigDefaults(t *testing.T) {
	configJSON := `{
		"enabled": true
	}`

	var config CircuitBreakerConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// Check defaults
	if config.FailureThreshold != 5 {
		t.Errorf("Expected default FailureThreshold 5, got %d", config.FailureThreshold)
	}
	if config.SuccessThreshold != 3 {
		t.Errorf("Expected default SuccessThreshold 3, got %d", config.SuccessThreshold)
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("Expected default Timeout %v, got %v", 30*time.Second, config.Timeout)
	}
	if config.HalfOpenMaxCalls != 3 {
		t.Errorf("Expected default HalfOpenMaxCalls 3, got %d", config.HalfOpenMaxCalls)
	}
	if config.ResetTimeout != 60*time.Second {
		t.Errorf("Expected default ResetTimeout %v, got %v", 60*time.Second, config.ResetTimeout)
	}
}
