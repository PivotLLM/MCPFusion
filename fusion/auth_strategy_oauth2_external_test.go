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

func TestOAuth2ExternalStrategy_GetAuthType(t *testing.T) {
	strategy := NewOAuth2ExternalStrategy(&http.Client{}, nil)
	if strategy.GetAuthType() != AuthTypeOAuth2External {
		t.Errorf("GetAuthType() = %v, want %v", strategy.GetAuthType(), AuthTypeOAuth2External)
	}
}

func TestOAuth2ExternalStrategy_SupportsRefresh(t *testing.T) {
	strategy := NewOAuth2ExternalStrategy(&http.Client{}, nil)
	if !strategy.SupportsRefresh() {
		t.Error("SupportsRefresh() should return true")
	}
}

func TestOAuth2ExternalStrategy_Authenticate(t *testing.T) {
	strategy := NewOAuth2ExternalStrategy(&http.Client{}, nil)

	_, err := strategy.Authenticate(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Authenticate() expected error, got nil")
		return
	}
	if !strings.Contains(err.Error(), "fusion-auth") {
		t.Errorf("Authenticate() error = %v, want error containing 'fusion-auth'", err)
	}
}

func TestOAuth2ExternalStrategy_ApplyAuth(t *testing.T) {
	strategy := NewOAuth2ExternalStrategy(&http.Client{}, nil)

	tests := []struct {
		name      string
		tokenInfo *TokenInfo
		wantAuth  string
		wantError bool
		errorMsg  string
	}{
		{
			name: "sets bearer authorization header",
			tokenInfo: &TokenInfo{
				AccessToken: "test_access_token",
				TokenType:   "Bearer",
			},
			wantAuth: "Bearer test_access_token",
		},
		{
			name: "sets authorization header with custom token type",
			tokenInfo: &TokenInfo{
				AccessToken: "test_access_token",
				TokenType:   "CustomType",
			},
			wantAuth: "CustomType test_access_token",
		},
		{
			name: "defaults to Bearer when token type is empty",
			tokenInfo: &TokenInfo{
				AccessToken: "test_access_token",
			},
			wantAuth: "Bearer test_access_token",
		},
		{
			name:      "nil token info returns error",
			tokenInfo: nil,
			wantError: true,
			errorMsg:  "token info is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com/api", nil)
			err := strategy.ApplyAuth(req, tt.tokenInfo, nil)

			if tt.wantError {
				if err == nil {
					t.Error("ApplyAuth() expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ApplyAuth() error = %v, want error containing %v", err, tt.errorMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("ApplyAuth() unexpected error: %v", err)
				return
			}

			if got := req.Header.Get("Authorization"); got != tt.wantAuth {
				t.Errorf("Authorization header = %v, want %v", got, tt.wantAuth)
			}
		})
	}
}

func TestOAuth2ExternalStrategy_RefreshToken(t *testing.T) {
	tests := []struct {
		name           string
		tokenInfo      *TokenInfo
		config         map[string]interface{}
		serverHandler  http.HandlerFunc
		wantError      bool
		errorMsg       string
		wantToken      string
		wantRefresh    string
		wantExpiry     bool
		checkSecret    bool
		checkScope     bool
		wantScope      string
		wantSecretVal  string
	}{
		{
			name: "successful refresh",
			tokenInfo: &TokenInfo{
				AccessToken:  "old_token",
				RefreshToken: "refresh_token_123",
			},
			config: map[string]interface{}{
				"clientId": "test-client-id",
				"tokenURL": "", // will be replaced with server URL
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
					return
				}
				if err := r.ParseForm(); err != nil {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				if r.FormValue("grant_type") != "refresh_token" {
					http.Error(w, "invalid grant_type", http.StatusBadRequest)
					return
				}
				if r.FormValue("client_id") != "test-client-id" {
					http.Error(w, "invalid client_id", http.StatusBadRequest)
					return
				}
				if r.FormValue("refresh_token") != "refresh_token_123" {
					http.Error(w, "invalid refresh_token", http.StatusBadRequest)
					return
				}
				resp := TokenResponse{
					AccessToken:  "new_access_token",
					TokenType:    "Bearer",
					ExpiresIn:    3600,
					RefreshToken: "new_refresh_token",
					Scope:        "openid email",
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			},
			wantToken:   "new_access_token",
			wantRefresh: "new_refresh_token",
			wantExpiry:  true,
		},
		{
			name: "refresh with client_secret included",
			tokenInfo: &TokenInfo{
				AccessToken:  "old_token",
				RefreshToken: "refresh_token_123",
			},
			config: map[string]interface{}{
				"clientId":     "test-client-id",
				"clientSecret": "test-secret",
				"tokenURL":     "", // will be replaced with server URL
				"scope":        "openid profile",
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				if r.FormValue("client_secret") != "test-secret" {
					http.Error(w, "missing client_secret", http.StatusBadRequest)
					return
				}
				if r.FormValue("scope") != "openid profile" {
					http.Error(w, "missing scope", http.StatusBadRequest)
					return
				}
				resp := TokenResponse{
					AccessToken:  "refreshed_token",
					TokenType:    "Bearer",
					ExpiresIn:    7200,
					RefreshToken: "",
					Scope:        "openid profile",
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			},
			wantToken:   "refreshed_token",
			wantRefresh: "refresh_token_123", // old refresh token kept since server returned empty
			wantExpiry:  true,
		},
		{
			name: "refresh with snake_case config keys",
			tokenInfo: &TokenInfo{
				AccessToken:  "old_token",
				RefreshToken: "refresh_token_123",
			},
			config: map[string]interface{}{
				"client_id":     "test-client-id",
				"client_secret": "test-secret-snake",
				"token_endpoint": "", // will be replaced with server URL
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				if r.FormValue("client_id") != "test-client-id" {
					http.Error(w, "invalid client_id", http.StatusBadRequest)
					return
				}
				if r.FormValue("client_secret") != "test-secret-snake" {
					http.Error(w, "missing client_secret", http.StatusBadRequest)
					return
				}
				resp := TokenResponse{
					AccessToken: "refreshed_token_snake",
					TokenType:   "Bearer",
					ExpiresIn:   3600,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			},
			wantToken:   "refreshed_token_snake",
			wantRefresh: "refresh_token_123",
			wantExpiry:  true,
		},
		{
			name:      "nil token info",
			tokenInfo: nil,
			config: map[string]interface{}{
				"clientId": "test-client-id",
				"tokenURL": "http://example.com/token",
			},
			wantError: true,
			errorMsg:  "token info is nil",
		},
		{
			name: "missing refresh token",
			tokenInfo: &TokenInfo{
				AccessToken:  "old_token",
				RefreshToken: "",
			},
			config: map[string]interface{}{
				"clientId": "test-client-id",
				"tokenURL": "http://example.com/token",
			},
			wantError: true,
			errorMsg:  "no refresh token available",
		},
		{
			name: "nil config",
			tokenInfo: &TokenInfo{
				AccessToken:  "old_token",
				RefreshToken: "refresh_token_123",
			},
			config:    nil,
			wantError: true,
			errorMsg:  "authentication configuration is required",
		},
		{
			name: "missing clientId in config",
			tokenInfo: &TokenInfo{
				AccessToken:  "old_token",
				RefreshToken: "refresh_token_123",
			},
			config: map[string]interface{}{
				"tokenURL": "http://example.com/token",
			},
			wantError: true,
			errorMsg:  "clientId is required",
		},
		{
			name: "missing tokenURL in config",
			tokenInfo: &TokenInfo{
				AccessToken:  "old_token",
				RefreshToken: "refresh_token_123",
			},
			config: map[string]interface{}{
				"clientId": "test-client-id",
			},
			wantError: true,
			errorMsg:  "tokenURL is required",
		},
		{
			name: "server error",
			tokenInfo: &TokenInfo{
				AccessToken:  "old_token",
				RefreshToken: "refresh_token_123",
			},
			config: map[string]interface{}{
				"clientId": "test-client-id",
				"tokenURL": "", // will be replaced with server URL
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error": "server_error"}`))
			},
			wantError: true,
			errorMsg:  "token refresh request failed with status 500",
		},
		{
			name: "invalid JSON response",
			tokenInfo: &TokenInfo{
				AccessToken:  "old_token",
				RefreshToken: "refresh_token_123",
			},
			config: map[string]interface{}{
				"clientId": "test-client-id",
				"tokenURL": "", // will be replaced with server URL
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`not valid json`))
			},
			wantError: true,
			errorMsg:  "failed to parse token refresh response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.serverHandler != nil {
				server = httptest.NewServer(tt.serverHandler)
				defer server.Close()

				// Replace empty tokenURL/token_endpoint with server URL
				if v, ok := tt.config["tokenURL"]; ok && v == "" {
					tt.config["tokenURL"] = server.URL + "/token"
				}
				if v, ok := tt.config["token_endpoint"]; ok && v == "" {
					tt.config["token_endpoint"] = server.URL + "/token"
				}
			}

			var httpClient *http.Client
			if server != nil {
				httpClient = server.Client()
			} else {
				httpClient = &http.Client{}
			}

			strategy := NewOAuth2ExternalStrategy(httpClient, nil)
			newTokenInfo, err := strategy.RefreshToken(context.Background(), tt.tokenInfo, tt.config)

			if tt.wantError {
				if err == nil {
					t.Error("RefreshToken() expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("RefreshToken() error = %v, want error containing %v", err, tt.errorMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("RefreshToken() unexpected error: %v", err)
				return
			}

			if newTokenInfo.AccessToken != tt.wantToken {
				t.Errorf("AccessToken = %v, want %v", newTokenInfo.AccessToken, tt.wantToken)
			}

			if newTokenInfo.RefreshToken != tt.wantRefresh {
				t.Errorf("RefreshToken = %v, want %v", newTokenInfo.RefreshToken, tt.wantRefresh)
			}

			if tt.wantExpiry && newTokenInfo.ExpiresAt == nil {
				t.Error("ExpiresAt should not be nil")
			}
		})
	}
}

func TestOAuth2ExternalStrategy_RefreshToken_NoScope(t *testing.T) {
	// Verify that when scope is empty, it is not included in the request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// Verify scope is NOT in the form data
		if r.FormValue("scope") != "" {
			http.Error(w, "scope should not be included when empty", http.StatusBadRequest)
			return
		}
		resp := TokenResponse{
			AccessToken: "token_no_scope",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	strategy := NewOAuth2ExternalStrategy(server.Client(), nil)

	tokenInfo := &TokenInfo{
		AccessToken:  "old_token",
		RefreshToken: "refresh_token_123",
	}
	config := map[string]interface{}{
		"clientId": "test-client-id",
		"tokenURL": server.URL + "/token",
		// No scope configured
	}

	newTokenInfo, err := strategy.RefreshToken(context.Background(), tokenInfo, config)
	if err != nil {
		t.Errorf("RefreshToken() unexpected error: %v", err)
		return
	}

	if newTokenInfo.AccessToken != "token_no_scope" {
		t.Errorf("AccessToken = %v, want token_no_scope", newTokenInfo.AccessToken)
	}
}

func TestOAuth2ExternalStrategy_RefreshToken_UsesTokenURLDirectly(t *testing.T) {
	// Verify that tokenURL is used as-is without any {tenantId} replacement
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		resp := TokenResponse{
			AccessToken: "direct_url_token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	strategy := NewOAuth2ExternalStrategy(server.Client(), nil)

	tokenInfo := &TokenInfo{
		AccessToken:  "old_token",
		RefreshToken: "refresh_token_123",
	}
	config := map[string]interface{}{
		"clientId": "test-client-id",
		"tokenURL": server.URL + "/oauth2/v4/token",
	}

	_, err := strategy.RefreshToken(context.Background(), tokenInfo, config)
	if err != nil {
		t.Errorf("RefreshToken() unexpected error: %v", err)
		return
	}

	if receivedPath != "/oauth2/v4/token" {
		t.Errorf("Server received path = %v, want /oauth2/v4/token", receivedPath)
	}
}
