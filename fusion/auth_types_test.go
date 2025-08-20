/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"strings"
	"testing"
	"time"
)

func TestTokenInfo_String(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	tests := []struct {
		name        string
		token       *TokenInfo
		contains    []string // Strings that should be present in output
		notContains []string // Strings that should NOT be present in output
	}{
		{
			name:     "nil token",
			token:    nil,
			contains: []string{"TokenInfo(nil)"},
		},
		{
			name: "token with all fields",
			token: &TokenInfo{
				AccessToken:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
				RefreshToken: "refresh_token_here",
				TokenType:    "Bearer",
				ExpiresAt:    &future,
				Scope:        []string{"read", "write"},
			},
			contains: []string{
				"TokenInfo(",
				"type=Bearer",
				"access_token=eyJhbGci...[REDACTED]",
				"refresh_token=present",
				"expires_in=",
				"scope_count=2",
			},
			notContains: []string{
				"eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ", // Should not contain full token
				"refresh_token_here", // Should not contain actual refresh token
			},
		},
		{
			name: "empty access token",
			token: &TokenInfo{
				AccessToken: "",
				TokenType:   "Bearer",
			},
			contains: []string{
				"access_token=empty",
				"refresh_token=none",
				"no_expiry",
				"scope_count=0",
			},
		},
		{
			name: "short access token",
			token: &TokenInfo{
				AccessToken: "short",
				TokenType:   "Bearer",
			},
			contains: []string{
				"access_token=[REDACTED]",
			},
			notContains: []string{
				"short", // Should not contain actual short token
			},
		},
		{
			name: "expired token",
			token: &TokenInfo{
				AccessToken: "expired_token_value_here",
				TokenType:   "Bearer",
				ExpiresAt:   &past,
			},
			contains: []string{
				"expired",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.token.String()

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("String() result should contain '%s', got: %s", expected, result)
				}
			}

			for _, notExpected := range tt.notContains {
				if strings.Contains(result, notExpected) {
					t.Errorf("String() result should NOT contain '%s', got: %s", notExpected, result)
				}
			}
		})
	}
}
