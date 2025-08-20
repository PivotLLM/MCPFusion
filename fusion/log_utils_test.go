/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"testing"
	"time"
)

func TestSanitizeTokenForLogging(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected string
	}{
		{
			name:     "empty token",
			token:    "",
			expected: "[EMPTY]",
		},
		{
			name:     "short token",
			token:    "abc123",
			expected: "[REDACTED]",
		},
		{
			name:     "long token",
			token:    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expected: "eyJhbGci...[REDACTED]",
		},
		{
			name:     "bearer token",
			token:    "AbCdEfGhIjKlMnOpQrStUvWxYz123456",
			expected: "AbCdEfGh...[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeTokenForLogging(tt.token)
			if result != tt.expected {
				t.Errorf("SanitizeTokenForLogging() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSanitizeStringForLogging(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{
			name:     "normal string",
			value:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "bearer token",
			value:    "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected: "Bearer e...[REDACTED]",
		},
		{
			name:     "token parameter",
			value:    "token=abc123def456ghi789",
			expected: "token=ab...[REDACTED]",
		},
		{
			name:     "jwt token",
			value:    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expected: "eyJhbGci...[REDACTED]",
		},
		{
			name:     "long alphanumeric string",
			value:    "abcdef1234567890ABCDEF1234567890_-",
			expected: "abcdef12...[REDACTED]",
		},
		{
			name:     "normal URL",
			value:    "https://example.com/api/endpoint",
			expected: "https://example.com/api/endpoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeStringForLogging(tt.value)
			if result != tt.expected {
				t.Errorf("SanitizeStringForLogging() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSanitizeHeaderForLogging(t *testing.T) {
	tests := []struct {
		name        string
		headerName  string
		headerValue string
		expected    string
	}{
		{
			name:        "authorization header",
			headerName:  "Authorization",
			headerValue: "Bearer abc123def456",
			expected:    "[REDACTED - length 19]",
		},
		{
			name:        "api key header",
			headerName:  "X-API-Key",
			headerValue: "secret123",
			expected:    "[REDACTED - length 9]",
		},
		{
			name:        "content type header",
			headerName:  "Content-Type",
			headerValue: "application/json",
			expected:    "application/json",
		},
		{
			name:        "user agent header",
			headerName:  "User-Agent",
			headerValue: "MCPFusion/1.0",
			expected:    "MCPFusion/1.0",
		},
		{
			name:        "empty authorization header",
			headerName:  "Authorization",
			headerValue: "",
			expected:    "[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeHeaderForLogging(tt.headerName, tt.headerValue)
			if result != tt.expected {
				t.Errorf("SanitizeHeaderForLogging() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSanitizeCacheKeyForLogging(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "empty key",
			key:      "",
			expected: "",
		},
		{
			name:     "tenant cache key",
			key:      "tenant:ec9076a84218935af35f7fd33d93d46ce2ddf8201da74d173bdcce430fd17e3b:token:Microsoft 365",
			expected: "tenant:ec9076a84218...:token:Microsoft 365",
		},
		{
			name:     "short tenant hash",
			key:      "tenant:short123:token:Service",
			expected: "tenant:short123:token:Service",
		},
		{
			name:     "long service name",
			key:      "tenant:ec9076a84218935af35f7fd33d93d46ce2ddf8201da74d173bdcce430fd17e3b:token:Very Long Service Name That Should Be Truncated",
			expected: "tenant:ec9076a84218...:token:Very Long Service Na...",
		},
		{
			name:     "non-tenant key",
			key:      "some:other:key:format",
			expected: "some:other:key:format",
		},
		{
			name:     "very long non-tenant key",
			key:      "this-is-a-very-long-key-that-should-be-truncated-because-it-exceeds-the-50-character-limit",
			expected: "this-is-a-very-long-key-that-should-be-truncated-b...[TRUNCATED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeCacheKeyForLogging(tt.key)
			if result != tt.expected {
				t.Errorf("SanitizeCacheKeyForLogging() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatExpiryForLogging(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour + 30*time.Minute)
	past := now.Add(-time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		expected  string
	}{
		{
			name:      "nil expiry",
			expiresAt: nil,
			expected:  "no_expiry",
		},
		{
			name:      "expired token",
			expiresAt: &past,
			expected:  "expired",
		},
		{
			name:      "future expiry",
			expiresAt: &future,
			expected:  "expires_in=1h30m0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatExpiryForLogging(tt.expiresAt)
			if result != tt.expected {
				t.Errorf("FormatExpiryForLogging() = %v, want %v", result, tt.expected)
			}
		})
	}
}
