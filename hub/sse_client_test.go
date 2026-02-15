/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package hub

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PivotLLM/MCPFusion/fusion"
)

// ---------------------------------------------------------------------------
// NewSSEClient
// ---------------------------------------------------------------------------

func TestNewSSEClient_DefaultBackoff(t *testing.T) {
	cfg := &fusion.ServiceConfig{
		ServiceKey: "test_sse",
		Name:       "Test SSE",
		Transport:  fusion.TransportTypeSSE,
		BaseURL:    "http://localhost:9999/sse",
	}

	c := NewSSEClient(cfg, newTestLogger(t))
	require.NotNil(t, c)

	assert.Equal(t, time.Second, c.backoff.currentDelay,
		"default base delay should be 1s")
	assert.Equal(t, 60*time.Second, c.backoff.maxDelay,
		"default max delay should be 60s")
	assert.Equal(t, 2.0, c.backoff.factor,
		"default backoff factor should be 2.0")
}

func TestNewSSEClient_CustomRetryConfig(t *testing.T) {
	cfg := &fusion.ServiceConfig{
		ServiceKey: "test_sse_custom",
		Name:       "Test SSE Custom",
		Transport:  fusion.TransportTypeSSE,
		BaseURL:    "http://localhost:9999/sse",
		Retry: &fusion.RetryConfig{
			BaseDelay:     500 * time.Millisecond,
			MaxDelay:      10 * time.Second,
			BackoffFactor: 3.0,
		},
	}

	c := NewSSEClient(cfg, newTestLogger(t))
	require.NotNil(t, c)

	assert.Equal(t, 500*time.Millisecond, c.backoff.baseDelay,
		"custom base delay should be 500ms")
	assert.Equal(t, 10*time.Second, c.backoff.maxDelay,
		"custom max delay should be 10s")
	assert.Equal(t, 3.0, c.backoff.factor,
		"custom backoff factor should be 3.0")
}

func TestNewSSEClient_PartialRetryConfig(t *testing.T) {
	cfg := &fusion.ServiceConfig{
		ServiceKey: "test_sse_partial",
		Name:       "Test SSE Partial",
		Transport:  fusion.TransportTypeSSE,
		BaseURL:    "http://localhost:9999/sse",
		Retry: &fusion.RetryConfig{
			BaseDelay: 2 * time.Second,
			// MaxDelay and BackoffFactor left at zero — should use defaults
		},
	}

	c := NewSSEClient(cfg, newTestLogger(t))
	require.NotNil(t, c)

	assert.Equal(t, 2*time.Second, c.backoff.baseDelay,
		"base delay should use provided value")
	assert.Equal(t, 60*time.Second, c.backoff.maxDelay,
		"max delay should fall back to default")
	assert.Equal(t, 2.0, c.backoff.factor,
		"backoff factor should fall back to default")
}

// ---------------------------------------------------------------------------
// Manager
// ---------------------------------------------------------------------------

func TestSSEClient_Manager_ReturnsNonNil(t *testing.T) {
	cfg := &fusion.ServiceConfig{
		ServiceKey: "test_mgr",
		Name:       "Test Manager",
		Transport:  fusion.TransportTypeSSE,
		BaseURL:    "http://localhost:9999/sse",
	}

	c := NewSSEClient(cfg, newTestLogger(t))
	mgr := c.Manager()
	require.NotNil(t, mgr, "Manager() must return a non-nil MCPClientManager")
}

// ---------------------------------------------------------------------------
// buildAuthHeaders
// ---------------------------------------------------------------------------

func TestSSEClient_BuildAuthHeaders(t *testing.T) {
	tests := []struct {
		name     string
		auth     fusion.AuthConfig
		expected map[string]string
	}{
		{
			name: "bearer token",
			auth: fusion.AuthConfig{
				Type:   fusion.AuthTypeBearer,
				Config: map[string]interface{}{"token": "my-secret-token"},
			},
			expected: map[string]string{
				"Authorization": "Bearer my-secret-token",
			},
		},
		{
			name: "basic auth",
			auth: fusion.AuthConfig{
				Type: fusion.AuthTypeBasic,
				Config: map[string]interface{}{
					"username": "admin",
					"password": "hunter2",
				},
			},
			expected: map[string]string{
				"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:hunter2")),
			},
		},
		{
			name: "api_key default header",
			auth: fusion.AuthConfig{
				Type:   fusion.AuthTypeAPIKey,
				Config: map[string]interface{}{"apiKey": "key-12345"},
			},
			expected: map[string]string{
				"X-API-Key": "key-12345",
			},
		},
		{
			name: "api_key custom header name",
			auth: fusion.AuthConfig{
				Type: fusion.AuthTypeAPIKey,
				Config: map[string]interface{}{
					"apiKey":     "key-67890",
					"headerName": "X-Custom-Auth",
				},
			},
			expected: map[string]string{
				"X-Custom-Auth": "key-67890",
			},
		},
		{
			name: "auth type none",
			auth: fusion.AuthConfig{
				Type: fusion.AuthTypeNone,
			},
			expected: map[string]string{},
		},
		{
			name: "auth type empty string",
			auth: fusion.AuthConfig{
				Type: "",
			},
			expected: map[string]string{},
		},
		{
			name: "bearer missing token",
			auth: fusion.AuthConfig{
				Type:   fusion.AuthTypeBearer,
				Config: map[string]interface{}{},
			},
			expected: map[string]string{},
		},
		{
			name: "bearer empty token",
			auth: fusion.AuthConfig{
				Type:   fusion.AuthTypeBearer,
				Config: map[string]interface{}{"token": ""},
			},
			expected: map[string]string{},
		},
		{
			name: "basic missing username",
			auth: fusion.AuthConfig{
				Type: fusion.AuthTypeBasic,
				Config: map[string]interface{}{
					"password": "secret",
				},
			},
			expected: map[string]string{},
		},
		{
			name: "basic empty username",
			auth: fusion.AuthConfig{
				Type: fusion.AuthTypeBasic,
				Config: map[string]interface{}{
					"username": "",
					"password": "secret",
				},
			},
			expected: map[string]string{},
		},
		{
			name: "api_key missing key",
			auth: fusion.AuthConfig{
				Type:   fusion.AuthTypeAPIKey,
				Config: map[string]interface{}{},
			},
			expected: map[string]string{},
		},
		{
			name: "api_key empty key",
			auth: fusion.AuthConfig{
				Type:   fusion.AuthTypeAPIKey,
				Config: map[string]interface{}{"apiKey": ""},
			},
			expected: map[string]string{},
		},
		{
			name: "bearer nil config map",
			auth: fusion.AuthConfig{
				Type:   fusion.AuthTypeBearer,
				Config: nil,
			},
			expected: map[string]string{},
		},
		{
			name: "basic password only encoded correctly",
			auth: fusion.AuthConfig{
				Type: fusion.AuthTypeBasic,
				Config: map[string]interface{}{
					"username": "user",
					"password": "",
				},
			},
			expected: map[string]string{
				"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("user:")),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &fusion.ServiceConfig{
				ServiceKey: "auth_test",
				Name:       "Auth Test",
				Transport:  fusion.TransportTypeSSE,
				BaseURL:    "http://localhost:9999/sse",
				Auth:       tc.auth,
			}
			c := NewSSEClient(cfg, newTestLogger(t))
			headers := c.buildAuthHeaders()
			assert.Equal(t, tc.expected, headers)
		})
	}
}

// ---------------------------------------------------------------------------
// Connect — error handling for invalid URL
// ---------------------------------------------------------------------------

func TestSSEClient_Connect_InvalidURL(t *testing.T) {
	cfg := &fusion.ServiceConfig{
		ServiceKey: "bad_url",
		Name:       "Bad URL",
		Transport:  fusion.TransportTypeSSE,
		BaseURL:    "http://127.0.0.1:0/nonexistent-sse-endpoint",
	}

	c := NewSSEClient(cfg, newTestLogger(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.Connect(ctx)
	assert.Error(t, err, "Connect to an invalid URL should return an error")
}

// ---------------------------------------------------------------------------
// Close — safe to call when never connected
// ---------------------------------------------------------------------------

func TestSSEClient_Close_NeverConnected(t *testing.T) {
	cfg := &fusion.ServiceConfig{
		ServiceKey: "close_test",
		Name:       "Close Test",
		Transport:  fusion.TransportTypeSSE,
		BaseURL:    "http://localhost:9999/sse",
	}

	c := NewSSEClient(cfg, newTestLogger(t))
	assert.NotPanics(t, func() {
		err := c.Close()
		assert.NoError(t, err, "Close on a never-connected client should not error")
	}, "Close should not panic when never connected")
}

// ---------------------------------------------------------------------------
// RunWithReconnect — context cancellation stops the loop
// ---------------------------------------------------------------------------

func TestSSEClient_RunWithReconnect_ContextCancellation(t *testing.T) {
	cfg := &fusion.ServiceConfig{
		ServiceKey: "reconnect_test",
		Name:       "Reconnect Test",
		Transport:  fusion.TransportTypeSSE,
		BaseURL:    "http://127.0.0.1:0/nonexistent-sse-endpoint",
		Retry: &fusion.RetryConfig{
			BaseDelay:     50 * time.Millisecond,
			MaxDelay:      100 * time.Millisecond,
			BackoffFactor: 1.5,
		},
	}

	c := NewSSEClient(cfg, newTestLogger(t))

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		c.RunWithReconnect(ctx, nil)
		close(done)
	}()

	// Let the reconnect loop run briefly, then cancel
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// RunWithReconnect returned as expected
	case <-time.After(5 * time.Second):
		t.Fatal("RunWithReconnect did not stop after context cancellation")
	}
}
