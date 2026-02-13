/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSessionJWTStrategy_GetAuthType(t *testing.T) {
	strategy := NewSessionJWTStrategy(&http.Client{}, nil)
	if strategy.GetAuthType() != AuthTypeSessionJWT {
		t.Errorf("GetAuthType() = %v, want %v", strategy.GetAuthType(), AuthTypeSessionJWT)
	}
}

func TestSessionJWTStrategy_SupportsRefresh(t *testing.T) {
	strategy := NewSessionJWTStrategy(&http.Client{}, nil)
	if !strategy.SupportsRefresh() {
		t.Error("SupportsRefresh() should return true")
	}
}

func TestSessionJWTStrategy_extractValueByPath(t *testing.T) {
	strategy := NewSessionJWTStrategy(&http.Client{}, nil)

	tests := []struct {
		name      string
		data      map[string]interface{}
		path      string
		want      interface{}
		wantError bool
	}{
		{
			name: "simple path",
			data: map[string]interface{}{
				"token": "abc123",
			},
			path: "token",
			want: "abc123",
		},
		{
			name: "nested path",
			data: map[string]interface{}{
				"datas": map[string]interface{}{
					"token": "xyz789",
				},
			},
			path: "datas.token",
			want: "xyz789",
		},
		{
			name: "deeply nested path",
			data: map[string]interface{}{
				"response": map[string]interface{}{
					"data": map[string]interface{}{
						"auth": map[string]interface{}{
							"token": "deep_token",
						},
					},
				},
			},
			path: "response.data.auth.token",
			want: "deep_token",
		},
		{
			name: "numeric value",
			data: map[string]interface{}{
				"expires_in": float64(3600),
			},
			path: "expires_in",
			want: float64(3600),
		},
		{
			name: "key not found",
			data: map[string]interface{}{
				"other": "value",
			},
			path:      "token",
			wantError: true,
		},
		{
			name: "nested key not found",
			data: map[string]interface{}{
				"datas": map[string]interface{}{
					"other": "value",
				},
			},
			path:      "datas.token",
			wantError: true,
		},
		{
			name: "path through non-object",
			data: map[string]interface{}{
				"token": "string_value",
			},
			path:      "token.nested",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := strategy.extractValueByPath(tt.data, tt.path)
			if (err != nil) != tt.wantError {
				t.Errorf("extractValueByPath() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && got != tt.want {
				t.Errorf("extractValueByPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSessionJWTStrategy_ApplyAuth_Header(t *testing.T) {
	strategy := NewSessionJWTStrategy(&http.Client{}, nil)

	tests := []struct {
		name           string
		tokenInfo      *TokenInfo
		wantHeaderName string
		wantHeaderVal  string
	}{
		{
			name: "default header format",
			tokenInfo: &TokenInfo{
				AccessToken: "test_token",
				TokenType:   "Bearer",
				Metadata: map[string]string{
					"tokenLocation": "header",
				},
			},
			wantHeaderName: "Authorization",
			wantHeaderVal:  "Bearer test_token",
		},
		{
			name: "custom header name",
			tokenInfo: &TokenInfo{
				AccessToken: "test_token",
				TokenType:   "Bearer",
				Metadata: map[string]string{
					"tokenLocation": "header",
					"headerName":    "X-Auth-Token",
				},
			},
			wantHeaderName: "X-Auth-Token",
			wantHeaderVal:  "Bearer test_token",
		},
		{
			name: "custom header format",
			tokenInfo: &TokenInfo{
				AccessToken: "test_token",
				TokenType:   "JWT",
				Metadata: map[string]string{
					"tokenLocation": "header",
					"headerFormat":  "Token {token}",
				},
			},
			wantHeaderName: "Authorization",
			wantHeaderVal:  "Token test_token",
		},
		{
			name: "custom format with token type",
			tokenInfo: &TokenInfo{
				AccessToken: "test_token",
				TokenType:   "CustomType",
				Metadata: map[string]string{
					"tokenLocation": "header",
					"headerFormat":  "Auth-{tokenType}: {token}",
				},
			},
			wantHeaderName: "Authorization",
			wantHeaderVal:  "Auth-CustomType: test_token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com/api", nil)
			err := strategy.ApplyAuth(req, tt.tokenInfo, nil)
			if err != nil {
				t.Errorf("ApplyAuth() error = %v", err)
				return
			}
			if got := req.Header.Get(tt.wantHeaderName); got != tt.wantHeaderVal {
				t.Errorf("Header %s = %v, want %v", tt.wantHeaderName, got, tt.wantHeaderVal)
			}
		})
	}
}

func TestSessionJWTStrategy_ApplyAuth_Cookie(t *testing.T) {
	strategy := NewSessionJWTStrategy(&http.Client{}, nil)

	tests := []struct {
		name           string
		tokenInfo      *TokenInfo
		wantCookieName string
		wantCookieVal  string
	}{
		{
			name: "default cookie format",
			tokenInfo: &TokenInfo{
				AccessToken: "test_token",
				TokenType:   "Bearer",
				Metadata: map[string]string{
					"tokenLocation": "cookie",
				},
			},
			wantCookieName: "token",
			wantCookieVal:  "test_token",
		},
		{
			name: "custom cookie name",
			tokenInfo: &TokenInfo{
				AccessToken: "test_token",
				TokenType:   "Bearer",
				Metadata: map[string]string{
					"tokenLocation": "cookie",
					"cookieName":    "session_token",
				},
			},
			wantCookieName: "session_token",
			wantCookieVal:  "test_token",
		},
		{
			name: "custom cookie format with JWT prefix",
			tokenInfo: &TokenInfo{
				AccessToken: "test_token",
				TokenType:   "Bearer",
				Metadata: map[string]string{
					"tokenLocation": "cookie",
					"cookieName":    "token",
					"cookieFormat":  "JWT {token}",
				},
			},
			wantCookieName: "token",
			wantCookieVal:  "JWT test_token",
		},
		{
			name: "cookie format with token type",
			tokenInfo: &TokenInfo{
				AccessToken: "test_token",
				TokenType:   "Bearer",
				Metadata: map[string]string{
					"tokenLocation": "cookie",
					"cookieFormat":  "{tokenType} {token}",
				},
			},
			wantCookieName: "token",
			wantCookieVal:  "Bearer test_token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com/api", nil)
			err := strategy.ApplyAuth(req, tt.tokenInfo, nil)
			if err != nil {
				t.Errorf("ApplyAuth() error = %v", err)
				return
			}
			cookie, err := req.Cookie(tt.wantCookieName)
			if err != nil {
				t.Errorf("Cookie %s not found: %v", tt.wantCookieName, err)
				return
			}
			if cookie.Value != tt.wantCookieVal {
				t.Errorf("Cookie %s value = %v, want %v", tt.wantCookieName, cookie.Value, tt.wantCookieVal)
			}
		})
	}
}

func TestSessionJWTStrategy_ApplyAuth_Query(t *testing.T) {
	strategy := NewSessionJWTStrategy(&http.Client{}, nil)

	tests := []struct {
		name           string
		tokenInfo      *TokenInfo
		wantQueryParam string
		wantQueryVal   string
	}{
		{
			name: "default query parameter",
			tokenInfo: &TokenInfo{
				AccessToken: "test_token",
				TokenType:   "Bearer",
				Metadata: map[string]string{
					"tokenLocation": "query",
				},
			},
			wantQueryParam: "token",
			wantQueryVal:   "test_token",
		},
		{
			name: "custom query parameter",
			tokenInfo: &TokenInfo{
				AccessToken: "test_token",
				TokenType:   "Bearer",
				Metadata: map[string]string{
					"tokenLocation": "query",
					"queryParam":    "access_token",
				},
			},
			wantQueryParam: "access_token",
			wantQueryVal:   "test_token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com/api", nil)
			err := strategy.ApplyAuth(req, tt.tokenInfo, nil)
			if err != nil {
				t.Errorf("ApplyAuth() error = %v", err)
				return
			}
			if got := req.URL.Query().Get(tt.wantQueryParam); got != tt.wantQueryVal {
				t.Errorf("Query param %s = %v, want %v", tt.wantQueryParam, got, tt.wantQueryVal)
			}
		})
	}
}

func TestSessionJWTStrategy_ApplyAuth_Errors(t *testing.T) {
	strategy := NewSessionJWTStrategy(&http.Client{}, nil)

	tests := []struct {
		name      string
		tokenInfo *TokenInfo
		wantError string
	}{
		{
			name:      "nil token info",
			tokenInfo: nil,
			wantError: "token info is nil",
		},
		{
			name: "unsupported token location",
			tokenInfo: &TokenInfo{
				AccessToken: "test_token",
				Metadata: map[string]string{
					"tokenLocation": "unsupported",
				},
			},
			wantError: "unsupported token location",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com/api", nil)
			err := strategy.ApplyAuth(req, tt.tokenInfo, nil)
			if err == nil {
				t.Error("ApplyAuth() expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("ApplyAuth() error = %v, want error containing %v", err, tt.wantError)
			}
		})
	}
}

func TestSessionJWTStrategy_Authenticate(t *testing.T) {
	// Create a mock server for login
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/users/token" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Return a mock token response
		response := map[string]interface{}{
			"status": "success",
			"datas": map[string]interface{}{
				"token": "mock_jwt_token_12345",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	strategy := NewSessionJWTStrategy(server.Client(), nil)

	config := map[string]interface{}{
		"loginURL":  server.URL + "/api/users/token",
		"tokenPath": "datas.token",
		"loginBody": map[string]interface{}{
			"username": "testuser",
			"password": "testpass",
		},
		"tokenLocation": "cookie",
		"cookieName":    "token",
		"cookieFormat":  "JWT {token}",
	}

	tokenInfo, err := strategy.Authenticate(context.Background(), config)
	if err != nil {
		t.Errorf("Authenticate() error = %v", err)
		return
	}

	if tokenInfo.AccessToken != "mock_jwt_token_12345" {
		t.Errorf("AccessToken = %v, want mock_jwt_token_12345", tokenInfo.AccessToken)
	}

	if tokenInfo.Metadata["tokenLocation"] != "cookie" {
		t.Errorf("Metadata[tokenLocation] = %v, want cookie", tokenInfo.Metadata["tokenLocation"])
	}

	if tokenInfo.Metadata["cookieName"] != "token" {
		t.Errorf("Metadata[cookieName] = %v, want token", tokenInfo.Metadata["cookieName"])
	}
}

func TestSessionJWTStrategy_Authenticate_Errors(t *testing.T) {
	strategy := NewSessionJWTStrategy(&http.Client{}, nil)

	tests := []struct {
		name      string
		config    map[string]interface{}
		wantError string
	}{
		{
			name:      "missing loginURL",
			config:    map[string]interface{}{},
			wantError: "loginURL is required",
		},
		{
			name: "missing tokenPath",
			config: map[string]interface{}{
				"loginURL": "http://example.com/login",
			},
			wantError: "tokenPath is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := strategy.Authenticate(context.Background(), tt.config)
			if err == nil {
				t.Error("Authenticate() expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("Authenticate() error = %v, want error containing %v", err, tt.wantError)
			}
		})
	}
}

func TestSessionJWTStrategy_Authenticate_WithExpiration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"datas": map[string]interface{}{
				"token":      "token_with_expiry",
				"expires_in": float64(3600),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	strategy := NewSessionJWTStrategy(server.Client(), nil)

	config := map[string]interface{}{
		"loginURL":      server.URL + "/login",
		"tokenPath":     "datas.token",
		"expiresInPath": "datas.expires_in",
		"tokenLocation": "header",
	}

	tokenInfo, err := strategy.Authenticate(context.Background(), config)
	if err != nil {
		t.Errorf("Authenticate() error = %v", err)
		return
	}

	if tokenInfo.ExpiresAt == nil {
		t.Error("ExpiresAt should not be nil when expiresInPath is configured")
	}
}

func TestSessionJWTStrategy_Authenticate_WithRefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"datas": map[string]interface{}{
				"token":        "access_token_here",
				"refreshToken": "refresh_token_here",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	strategy := NewSessionJWTStrategy(server.Client(), nil)

	config := map[string]interface{}{
		"loginURL":         server.URL + "/login",
		"tokenPath":        "datas.token",
		"refreshTokenPath": "datas.refreshToken",
		"tokenLocation":    "header",
	}

	tokenInfo, err := strategy.Authenticate(context.Background(), config)
	if err != nil {
		t.Errorf("Authenticate() error = %v", err)
		return
	}

	if tokenInfo.RefreshToken != "refresh_token_here" {
		t.Errorf("RefreshToken = %v, want refresh_token_here", tokenInfo.RefreshToken)
	}
}

func TestSessionJWTStrategy_Authenticate_LoginFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid credentials"}`))
	}))
	defer server.Close()

	strategy := NewSessionJWTStrategy(server.Client(), nil)

	config := map[string]interface{}{
		"loginURL":      server.URL + "/login",
		"tokenPath":     "datas.token",
		"tokenLocation": "header",
	}

	_, err := strategy.Authenticate(context.Background(), config)
	if err == nil {
		t.Error("Authenticate() expected error for failed login, got nil")
		return
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("Authenticate() error should contain status code 401, got: %v", err)
	}
}

func TestSessionJWTStrategy_RefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/refresh" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		response := map[string]interface{}{
			"datas": map[string]interface{}{
				"token": "new_access_token",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	strategy := NewSessionJWTStrategy(server.Client(), nil)

	config := map[string]interface{}{
		"refreshURL":    server.URL + "/refresh",
		"tokenPath":     "datas.token",
		"tokenLocation": "header",
	}

	oldTokenInfo := &TokenInfo{
		AccessToken:  "old_access_token",
		RefreshToken: "refresh_token",
		TokenType:    "Bearer",
		Metadata: map[string]string{
			"tokenLocation": "header",
		},
	}

	newTokenInfo, err := strategy.RefreshToken(context.Background(), oldTokenInfo, config)
	if err != nil {
		t.Errorf("RefreshToken() error = %v", err)
		return
	}

	if newTokenInfo.AccessToken != "new_access_token" {
		t.Errorf("AccessToken = %v, want new_access_token", newTokenInfo.AccessToken)
	}

	// Verify metadata is preserved
	if newTokenInfo.Metadata["tokenLocation"] != "header" {
		t.Errorf("Metadata[tokenLocation] = %v, want header", newTokenInfo.Metadata["tokenLocation"])
	}
}

func TestSessionJWTStrategy_RefreshToken_NoRefreshURL(t *testing.T) {
	strategy := NewSessionJWTStrategy(&http.Client{}, nil)

	config := map[string]interface{}{
		"tokenPath": "datas.token",
	}

	oldTokenInfo := &TokenInfo{
		AccessToken:  "old_access_token",
		RefreshToken: "refresh_token",
	}

	_, err := strategy.RefreshToken(context.Background(), oldTokenInfo, config)
	if err == nil {
		t.Error("RefreshToken() expected error when refreshURL not configured, got nil")
		return
	}
	if !strings.Contains(err.Error(), "refreshURL not configured") {
		t.Errorf("RefreshToken() error = %v, want error about refreshURL", err)
	}
}
