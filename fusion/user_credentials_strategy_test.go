/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserCredentialsStrategy_GetAuthType(t *testing.T) {
	strategy := NewUserCredentialsStrategy(nil)
	if strategy.GetAuthType() != AuthTypeUserCredentials {
		t.Errorf("GetAuthType() = %v, want %v", strategy.GetAuthType(), AuthTypeUserCredentials)
	}
}

func TestUserCredentialsStrategy_SupportsRefresh(t *testing.T) {
	strategy := NewUserCredentialsStrategy(nil)
	if strategy.SupportsRefresh() {
		t.Error("SupportsRefresh() should return false")
	}
}

func TestUserCredentialsStrategy_ApplyAuth(t *testing.T) {
	tests := []struct {
		name      string
		tokenInfo *TokenInfo
		config    map[string]interface{}
		wantErr   bool
		errMsg    string
		validate  func(t *testing.T, req *http.Request)
	}{
		{
			name: "query params - two fields",
			tokenInfo: &TokenInfo{
				Metadata: map[string]string{
					"api_key":  "key123",
					"api_token": "tok456",
				},
			},
			config: map[string]interface{}{
				"fields": []interface{}{
					map[string]interface{}{
						"name":      "api_key",
						"location":  "query",
						"paramName": "key",
					},
					map[string]interface{}{
						"name":      "api_token",
						"location":  "query",
						"paramName": "token",
					},
				},
			},
			validate: func(t *testing.T, req *http.Request) {
				if got := req.URL.Query().Get("key"); got != "key123" {
					t.Errorf("query param 'key' = %q, want %q", got, "key123")
				}
				if got := req.URL.Query().Get("token"); got != "tok456" {
					t.Errorf("query param 'token' = %q, want %q", got, "tok456")
				}
			},
		},
		{
			name: "header field",
			tokenInfo: &TokenInfo{
				Metadata: map[string]string{
					"x_api_key": "headerval789",
				},
			},
			config: map[string]interface{}{
				"fields": []interface{}{
					map[string]interface{}{
						"name":      "x_api_key",
						"location":  "header",
						"paramName": "X-Api-Key",
					},
				},
			},
			validate: func(t *testing.T, req *http.Request) {
				if got := req.Header.Get("X-Api-Key"); got != "headerval789" {
					t.Errorf("header 'X-Api-Key' = %q, want %q", got, "headerval789")
				}
			},
		},
		{
			name: "cookie field",
			tokenInfo: &TokenInfo{
				Metadata: map[string]string{
					"session_id": "sess_abc",
				},
			},
			config: map[string]interface{}{
				"fields": []interface{}{
					map[string]interface{}{
						"name":      "session_id",
						"location":  "cookie",
						"paramName": "sid",
					},
				},
			},
			validate: func(t *testing.T, req *http.Request) {
				cookie, err := req.Cookie("sid")
				if err != nil {
					t.Fatalf("expected cookie 'sid' but got error: %v", err)
				}
				if cookie.Value != "sess_abc" {
					t.Errorf("cookie 'sid' = %q, want %q", cookie.Value, "sess_abc")
				}
			},
		},
		{
			name: "mixed locations - query and header",
			tokenInfo: &TokenInfo{
				Metadata: map[string]string{
					"api_key":   "qkey111",
					"auth_token": "hval222",
				},
			},
			config: map[string]interface{}{
				"fields": []interface{}{
					map[string]interface{}{
						"name":      "api_key",
						"location":  "query",
						"paramName": "apikey",
					},
					map[string]interface{}{
						"name":      "auth_token",
						"location":  "header",
						"paramName": "Authorization",
					},
				},
			},
			validate: func(t *testing.T, req *http.Request) {
				if got := req.URL.Query().Get("apikey"); got != "qkey111" {
					t.Errorf("query param 'apikey' = %q, want %q", got, "qkey111")
				}
				if got := req.Header.Get("Authorization"); got != "hval222" {
					t.Errorf("header 'Authorization' = %q, want %q", got, "hval222")
				}
			},
		},
		{
			name: "paramName defaults to name when empty",
			tokenInfo: &TokenInfo{
				Metadata: map[string]string{
					"api_key": "defaultname_val",
				},
			},
			config: map[string]interface{}{
				"fields": []interface{}{
					map[string]interface{}{
						"name":     "api_key",
						"location": "query",
						// paramName intentionally omitted
					},
				},
			},
			validate: func(t *testing.T, req *http.Request) {
				if got := req.URL.Query().Get("api_key"); got != "defaultname_val" {
					t.Errorf("query param 'api_key' = %q, want %q", got, "defaultname_val")
				}
			},
		},
		{
			name: "missing field value in metadata",
			tokenInfo: &TokenInfo{
				Metadata: map[string]string{
					// "api_key" intentionally absent
				},
			},
			config: map[string]interface{}{
				"fields": []interface{}{
					map[string]interface{}{
						"name":      "api_key",
						"location":  "query",
						"paramName": "key",
					},
				},
			},
			wantErr: true,
			errMsg:  "missing credential value for field 'api_key'",
		},
		{
			name:      "nil tokenInfo",
			tokenInfo: nil,
			config: map[string]interface{}{
				"fields": []interface{}{},
			},
			wantErr: true,
			errMsg:  "token info is nil",
		},
		{
			name: "nil config",
			tokenInfo: &TokenInfo{
				Metadata: map[string]string{},
			},
			config:  nil,
			wantErr: true,
			errMsg:  "auth config is required for user_credentials",
		},
		{
			name: "no fields key in config",
			tokenInfo: &TokenInfo{
				Metadata: map[string]string{},
			},
			config:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "user_credentials config missing 'fields'",
		},
		{
			name: "fields is not an array",
			tokenInfo: &TokenInfo{
				Metadata: map[string]string{},
			},
			config: map[string]interface{}{
				"fields": "not_an_array",
			},
			wantErr: true,
			errMsg:  "user_credentials 'fields' must be an array",
		},
		{
			name: "unsupported location",
			tokenInfo: &TokenInfo{
				Metadata: map[string]string{
					"api_key": "val",
				},
			},
			config: map[string]interface{}{
				"fields": []interface{}{
					map[string]interface{}{
						"name":      "api_key",
						"location":  "body",
						"paramName": "key",
					},
				},
			},
			wantErr: true,
			errMsg:  "unsupported credential location 'body' for field 'api_key'",
		},
	}

	strategy := NewUserCredentialsStrategy(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a fresh request for each test case
			req := httptest.NewRequest(http.MethodGet, "https://api.example.com/v1/resource", nil)

			err := strategy.ApplyAuth(req, tt.tokenInfo, tt.config)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("error = %q, want %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, req)
			}
		})
	}
}
