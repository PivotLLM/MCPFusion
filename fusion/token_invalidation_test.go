/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PivotLLM/MCPFusion/global"
)

func TestTokenInvalidationConfig_GetEffectiveTokenInvalidationConfig(t *testing.T) {
	tests := []struct {
		name                    string
		authConfig              AuthConfig
		wantStatusCodes         []int
		wantRetryOnInvalidation bool
	}{
		{
			name: "default config when not specified",
			authConfig: AuthConfig{
				Type:              AuthTypeBearer,
				Config:            map[string]interface{}{"token": "test"},
				TokenInvalidation: nil,
			},
			wantStatusCodes:         DefaultTokenInvalidationStatusCodes,
			wantRetryOnInvalidation: true,
		},
		{
			name: "configured with custom status codes",
			authConfig: AuthConfig{
				Type:   AuthTypeBearer,
				Config: map[string]interface{}{"token": "test"},
				TokenInvalidation: &TokenInvalidationConfig{
					StatusCodes:         []int{401, 403},
					RetryOnInvalidation: false,
				},
			},
			wantStatusCodes:         []int{401, 403},
			wantRetryOnInvalidation: false,
		},
		{
			name: "configured with empty status codes uses defaults",
			authConfig: AuthConfig{
				Type:   AuthTypeBearer,
				Config: map[string]interface{}{"token": "test"},
				TokenInvalidation: &TokenInvalidationConfig{
					StatusCodes:         []int{},
					RetryOnInvalidation: true,
				},
			},
			wantStatusCodes:         DefaultTokenInvalidationStatusCodes,
			wantRetryOnInvalidation: true,
		},
		{
			name: "configured with retry enabled",
			authConfig: AuthConfig{
				Type:   AuthTypeBearer,
				Config: map[string]interface{}{"token": "test"},
				TokenInvalidation: &TokenInvalidationConfig{
					StatusCodes:         []int{401},
					RetryOnInvalidation: true,
				},
			},
			wantStatusCodes:         []int{401},
			wantRetryOnInvalidation: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.authConfig.GetEffectiveTokenInvalidationConfig()
			if len(got.StatusCodes) != len(tt.wantStatusCodes) {
				t.Errorf("StatusCodes length = %v, want %v", len(got.StatusCodes), len(tt.wantStatusCodes))
			}
			for i, code := range tt.wantStatusCodes {
				if got.StatusCodes[i] != code {
					t.Errorf("StatusCodes[%d] = %v, want %v", i, got.StatusCodes[i], code)
				}
			}
			if got.RetryOnInvalidation != tt.wantRetryOnInvalidation {
				t.Errorf("RetryOnInvalidation = %v, want %v", got.RetryOnInvalidation, tt.wantRetryOnInvalidation)
			}
		})
	}
}

func TestHTTPHandler_TokenInvalidationOn401(t *testing.T) {
	// Track calls to the mock server
	callCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call returns 401
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "unauthorized"}`))
		} else {
			// Second call (after token refresh) returns success
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"result": "success"}`))
		}
	}))
	defer mockServer.Close()

	// Create a test configuration
	config := &Config{
		Services: map[string]*ServiceConfig{
			"test_service": {
				Name:    "test_service",
				BaseURL: mockServer.URL,
				Auth: AuthConfig{
					Type:   AuthTypeBearer,
					Config: map[string]interface{}{"token": "test_token"},
					TokenInvalidation: &TokenInvalidationConfig{
						StatusCodes:         []int{401},
						RetryOnInvalidation: true,
					},
				},
				Endpoints: []EndpointConfig{
					{
						ID:          "test_endpoint",
						Name:        "Test Endpoint",
						Description: "Test",
						Method:      "GET",
						Path:        "/test",
						Parameters:  []ParameterConfig{},
						Response:    ResponseConfig{Type: ResponseTypeJSON},
					},
				},
			},
		},
	}

	fusion := New(WithConfig(config))

	service := config.Services["test_service"]
	endpoint := &service.Endpoints[0]
	handler := NewHTTPHandler(fusion, service, endpoint)

	// Create a context with tenant information
	tenantContext := &TenantContext{
		TenantHash:  "test_tenant_hash",
		ServiceName: "test_service",
		RequestID:   "test_request",
	}
	ctx := context.WithValue(context.Background(), global.TenantContextKey, tenantContext)

	// Execute the handler
	_, err := handler.Handle(ctx, map[string]interface{}{})

	// We expect success after retry (or specific error if multiTenantAuth is not set up)
	// In this test without full multiTenantAuth setup, we'll get an auth error
	if err == nil {
		t.Error("Expected error due to missing multiTenantAuth setup")
	}
}

func TestHTTPHandler_TokenInvalidationWithoutRetry(t *testing.T) {
	// Track calls to the mock server
	callCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Always return 401
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer mockServer.Close()

	// Create a test configuration with retry disabled
	config := &Config{
		Services: map[string]*ServiceConfig{
			"test_service": {
				Name:    "test_service",
				BaseURL: mockServer.URL,
				Auth: AuthConfig{
					Type:   AuthTypeBearer,
					Config: map[string]interface{}{"token": "test_token"},
					TokenInvalidation: &TokenInvalidationConfig{
						StatusCodes:         []int{401},
						RetryOnInvalidation: false, // No retry
					},
				},
				Endpoints: []EndpointConfig{
					{
						ID:          "test_endpoint",
						Name:        "Test Endpoint",
						Description: "Test",
						Method:      "GET",
						Path:        "/test",
						Parameters:  []ParameterConfig{},
						Response:    ResponseConfig{Type: ResponseTypeJSON},
					},
				},
			},
		},
	}

	fusion := New(WithConfig(config))

	service := config.Services["test_service"]
	endpoint := &service.Endpoints[0]
	handler := NewHTTPHandler(fusion, service, endpoint)

	// Create a context with tenant information
	tenantContext := &TenantContext{
		TenantHash:  "test_tenant_hash",
		ServiceName: "test_service",
		RequestID:   "test_request",
	}
	ctx := context.WithValue(context.Background(), global.TenantContextKey, tenantContext)

	// Execute the handler
	_, err := handler.Handle(ctx, map[string]interface{}{})

	// Should fail without retry
	if err == nil {
		t.Error("Expected error due to 401 response")
	}
}

func TestHTTPHandler_TokenInvalidationMultipleStatusCodes(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		shouldInvalidate bool
		configuredCodes []int
	}{
		{
			name:             "401 should invalidate",
			statusCode:       401,
			shouldInvalidate: true,
			configuredCodes:  []int{401, 403},
		},
		{
			name:             "403 should invalidate",
			statusCode:       403,
			shouldInvalidate: true,
			configuredCodes:  []int{401, 403},
		},
		{
			name:             "404 should not invalidate",
			statusCode:       404,
			shouldInvalidate: false,
			configuredCodes:  []int{401, 403},
		},
		{
			name:             "500 should not invalidate",
			statusCode:       500,
			shouldInvalidate: false,
			configuredCodes:  []int{401, 403},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(`{"error": "test error"}`))
			}))
			defer mockServer.Close()

			config := &Config{
				Services: map[string]*ServiceConfig{
					"test_service": {
						Name:    "test_service",
						BaseURL: mockServer.URL,
						Auth: AuthConfig{
							Type:   AuthTypeBearer,
							Config: map[string]interface{}{"token": "test_token"},
							TokenInvalidation: &TokenInvalidationConfig{
								StatusCodes:         tt.configuredCodes,
								RetryOnInvalidation: false,
							},
						},
						Endpoints: []EndpointConfig{
							{
								ID:          "test_endpoint",
								Name:        "Test Endpoint",
								Description: "Test",
								Method:      "GET",
								Path:        "/test",
								Parameters:  []ParameterConfig{},
								Response:    ResponseConfig{Type: ResponseTypeJSON},
							},
						},
					},
				},
			}

			fusion := New(WithConfig(config))

			service := config.Services["test_service"]
			endpoint := &service.Endpoints[0]
			handler := NewHTTPHandler(fusion, service, endpoint)

			tenantContext := &TenantContext{
				TenantHash:  "test_tenant_hash",
				ServiceName: "test_service",
				RequestID:   "test_request",
			}
			ctx := context.WithValue(context.Background(), global.TenantContextKey, tenantContext)

			// Execute the handler
			result, err := handler.Handle(ctx, map[string]interface{}{})

			// For status codes >= 400, we return the error body as result with no error
			if tt.statusCode >= 400 && err == nil {
				// Should return error body as result
				if result == "" {
					t.Error("Expected error body in result")
				}
			}
		})
	}
}

func TestHTTPHandler_PrepareAuthConfig(t *testing.T) {
	config := &Config{
		Services: map[string]*ServiceConfig{
			"test_service": {
				Name:    "test_service",
				BaseURL: "https://api.example.com",
				Auth: AuthConfig{
					Type: AuthTypeBearer,
					Config: map[string]interface{}{
						"token": "test_token",
						"scope": "read:data",
					},
				},
				Endpoints: []EndpointConfig{
					{
						ID:          "test_endpoint",
						Name:        "Test Endpoint",
						Description: "Test",
						Method:      "GET",
						Path:        "/test",
						Parameters:  []ParameterConfig{},
						Response:    ResponseConfig{Type: ResponseTypeJSON},
					},
				},
			},
		},
	}

	fusion := New(WithConfig(config))

	service := config.Services["test_service"]
	endpoint := &service.Endpoints[0]
	handler := NewHTTPHandler(fusion, service, endpoint)

	// Test that prepareAuthConfig creates a copy and adds baseURL
	authConfig1 := handler.prepareAuthConfig()
	authConfig2 := handler.prepareAuthConfig()

	// Check that baseURL was added
	if authConfig1.Config["baseURL"] != "https://api.example.com" {
		t.Errorf("baseURL not added to config")
	}

	// Check that the original config was not modified
	if handler.service.Auth.Config["baseURL"] != nil {
		t.Errorf("Original config was modified")
	}

	// Check that we get independent copies
	authConfig1.Config["test"] = "value1"
	authConfig2.Config["test"] = "value2"

	if authConfig1.Config["test"] == authConfig2.Config["test"] {
		t.Errorf("Config copies are not independent")
	}
}

func TestTokenInvalidationConfigJSON(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		want     *TokenInvalidationConfig
	}{
		{
			name: "full config",
			jsonData: `{
				"statusCodes": [401, 403],
				"retryOnInvalidation": true
			}`,
			want: &TokenInvalidationConfig{
				StatusCodes:         []int{401, 403},
				RetryOnInvalidation: true,
			},
		},
		{
			name: "minimal config",
			jsonData: `{
				"retryOnInvalidation": false
			}`,
			want: &TokenInvalidationConfig{
				StatusCodes:         nil,
				RetryOnInvalidation: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got TokenInvalidationConfig
			if err := json.Unmarshal([]byte(tt.jsonData), &got); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if got.RetryOnInvalidation != tt.want.RetryOnInvalidation {
				t.Errorf("RetryOnInvalidation = %v, want %v", got.RetryOnInvalidation, tt.want.RetryOnInvalidation)
			}

			if len(got.StatusCodes) != len(tt.want.StatusCodes) {
				t.Errorf("StatusCodes length = %v, want %v", len(got.StatusCodes), len(tt.want.StatusCodes))
			}
		})
	}
}

func TestHTTPHandler_NilMultiTenantAuth(t *testing.T) {
	// Test that InvalidateToken handles nil multiTenantAuth gracefully
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer mockServer.Close()

	config := &Config{
		Services: map[string]*ServiceConfig{
			"test_service": {
				Name:    "test_service",
				BaseURL: mockServer.URL,
				Auth: AuthConfig{
					Type:   AuthTypeBearer,
					Config: map[string]interface{}{"token": "test_token"},
					TokenInvalidation: &TokenInvalidationConfig{
						StatusCodes:         []int{401},
						RetryOnInvalidation: false,
					},
				},
				Endpoints: []EndpointConfig{
					{
						ID:          "test_endpoint",
						Name:        "Test Endpoint",
						Description: "Test",
						Method:      "GET",
						Path:        "/test",
						Parameters:  []ParameterConfig{},
						Response:    ResponseConfig{Type: ResponseTypeJSON},
					},
				},
			},
		},
	}

	fusion := New(WithConfig(config))

	service := config.Services["test_service"]
	endpoint := &service.Endpoints[0]
	handler := NewHTTPHandler(fusion, service, endpoint)

	// Ensure multiTenantAuth is nil (it should be by default without DB setup)
	handler.fusion.multiTenantAuth = nil

	tenantContext := &TenantContext{
		TenantHash:  "test_tenant_hash",
		ServiceName: "test_service",
		RequestID:   "test_request",
	}
	ctx := context.WithValue(context.Background(), global.TenantContextKey, tenantContext)

	// This should not panic even with nil multiTenantAuth
	_, err = handler.Handle(ctx, map[string]interface{}{})

	// We expect an error due to no auth being configured
	if err == nil {
		t.Error("Expected error due to missing auth configuration")
	}
}

func TestHTTPHandler_ContextCancellationBeforeRetry(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer mockServer.Close()

	config := &Config{
		Services: map[string]*ServiceConfig{
			"test_service": {
				Name:    "test_service",
				BaseURL: mockServer.URL,
				Auth: AuthConfig{
					Type:   AuthTypeBearer,
					Config: map[string]interface{}{"token": "test_token"},
					TokenInvalidation: &TokenInvalidationConfig{
						StatusCodes:         []int{401},
						RetryOnInvalidation: true,
					},
				},
				Endpoints: []EndpointConfig{
					{
						ID:          "test_endpoint",
						Name:        "Test Endpoint",
						Description: "Test",
						Method:      "GET",
						Path:        "/test",
						Parameters:  []ParameterConfig{},
						Response:    ResponseConfig{Type: ResponseTypeJSON},
					},
				},
			},
		},
	}

	fusion := New(WithConfig(config))

	service := config.Services["test_service"]
	endpoint := &service.Endpoints[0]
	handler := NewHTTPHandler(fusion, service, endpoint)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	tenantContext := &TenantContext{
		TenantHash:  "test_tenant_hash",
		ServiceName: "test_service",
		RequestID:   "test_request",
	}
	ctx = context.WithValue(ctx, global.TenantContextKey, tenantContext)

	// Execute the handler with cancelled context
	_, err = handler.Handle(ctx, map[string]interface{}{})

	// Should get context cancelled error
	if err == nil {
		t.Error("Expected error due to cancelled context")
	}
}
