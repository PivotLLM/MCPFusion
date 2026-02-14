/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestUserCredentials_BasicAuth(t *testing.T) {
	tests := []struct {
		name     string
		field1   string
		value1   string
		field2   string
		value2   string
		validate func(t *testing.T, req *http.Request)
	}{
		{
			name:   "standard username and password",
			field1: "username",
			value1: "myuser",
			field2: "password",
			value2: "mypass",
			validate: func(t *testing.T, req *http.Request) {
				expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("myuser:mypass"))
				if got := req.Header.Get("Authorization"); got != expected {
					t.Errorf("Authorization header = %q, want %q", got, expected)
				}
			},
		},
		{
			name:   "values with special characters",
			field1: "api_key",
			value1: "key:with:colons",
			field2: "api_secret",
			value2: "secret/with+chars==",
			validate: func(t *testing.T, req *http.Request) {
				expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("key:with:colons:secret/with+chars=="))
				if got := req.Header.Get("Authorization"); got != expected {
					t.Errorf("Authorization header = %q, want %q", got, expected)
				}
			},
		},
		{
			name:   "empty-ish looking but valid values",
			field1: "user",
			value1: "a",
			field2: "pass",
			value2: "b",
			validate: func(t *testing.T, req *http.Request) {
				expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("a:b"))
				if got := req.Header.Get("Authorization"); got != expected {
					t.Errorf("Authorization header = %q, want %q", got, expected)
				}
			},
		},
	}

	strategy := NewUserCredentialsStrategy(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "https://api.example.com/v1/resource", nil)

			tokenInfo := &TokenInfo{
				Metadata: map[string]string{
					tt.field1: tt.value1,
					tt.field2: tt.value2,
				},
			}

			config := map[string]interface{}{
				"authMethod": "basic_auth",
				"fields": []interface{}{
					map[string]interface{}{"name": tt.field1},
					map[string]interface{}{"name": tt.field2},
				},
			}

			err := strategy.ApplyAuth(req, tokenInfo, config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.validate(t, req)
		})
	}
}

func TestUserCredentials_BasicAuth_MissingField(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]string
		wantErr  string
	}{
		{
			name:     "first field missing",
			metadata: map[string]string{"password": "pass123"},
			wantErr:  "missing credential value for field 'username'",
		},
		{
			name:     "second field missing",
			metadata: map[string]string{"username": "user123"},
			wantErr:  "missing credential value for field 'password'",
		},
		{
			name:     "both fields missing",
			metadata: map[string]string{},
			wantErr:  "missing credential value for field 'username'",
		},
		{
			name:     "first field empty string",
			metadata: map[string]string{"username": "", "password": "pass123"},
			wantErr:  "missing credential value for field 'username'",
		},
	}

	strategy := NewUserCredentialsStrategy(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "https://api.example.com/v1/resource", nil)

			tokenInfo := &TokenInfo{Metadata: tt.metadata}
			config := map[string]interface{}{
				"authMethod": "basic_auth",
				"fields": []interface{}{
					map[string]interface{}{"name": "username"},
					map[string]interface{}{"name": "password"},
				},
			}

			err := strategy.ApplyAuth(req, tokenInfo, config)
			if err == nil {
				t.Fatal("expected error but got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestUserCredentials_BasicAuth_WrongFieldCount(t *testing.T) {
	tests := []struct {
		name   string
		fields []interface{}
	}{
		{
			name: "one field",
			fields: []interface{}{
				map[string]interface{}{"name": "username"},
			},
		},
		{
			name: "three fields",
			fields: []interface{}{
				map[string]interface{}{"name": "username"},
				map[string]interface{}{"name": "password"},
				map[string]interface{}{"name": "extra"},
			},
		},
	}

	strategy := NewUserCredentialsStrategy(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "https://api.example.com/v1/resource", nil)

			tokenInfo := &TokenInfo{
				Metadata: map[string]string{
					"username": "user",
					"password": "pass",
					"extra":    "val",
				},
			}
			config := map[string]interface{}{
				"authMethod": "basic_auth",
				"fields":     tt.fields,
			}

			err := strategy.ApplyAuth(req, tokenInfo, config)
			if err == nil {
				t.Fatal("expected error but got nil")
			}
			if !strings.Contains(err.Error(), "basic_auth requires exactly 2 fields") {
				t.Errorf("error = %q, want it to contain 'basic_auth requires exactly 2 fields'", err.Error())
			}
		})
	}
}
