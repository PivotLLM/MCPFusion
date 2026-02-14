/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// TokenInfo represents information about an authentication token
type TokenInfo struct {
	AccessToken  string            `json:"access_token"`
	TokenType    string            `json:"token_type,omitempty"`
	RefreshToken string            `json:"refresh_token,omitempty"`
	ExpiresAt    *time.Time        `json:"expires_at,omitempty"`
	Scope        []string          `json:"scope,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// IsExpired checks if the token is expired
func (t *TokenInfo) IsExpired() bool {
	if t.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*t.ExpiresAt)
}

// IsExpiredWithBuffer checks if the token is expired with a buffer time
func (t *TokenInfo) IsExpiredWithBuffer(buffer time.Duration) bool {
	if t.ExpiresAt == nil {
		return false
	}
	return time.Now().Add(buffer).After(*t.ExpiresAt)
}

// HasRefreshToken returns true if the token has a refresh token
func (t *TokenInfo) HasRefreshToken() bool {
	return t.RefreshToken != ""
}

// GetAuthorizationHeader returns the authorization header value
func (t *TokenInfo) GetAuthorizationHeader() string {
	if t.TokenType != "" {
		return t.TokenType + " " + t.AccessToken
	}
	return "Bearer " + t.AccessToken
}

// String returns a safe string representation that doesn't expose token values
func (t *TokenInfo) String() string {
	if t == nil {
		return "TokenInfo(nil)"
	}

	// Create safe representation of access token
	var accessTokenInfo string
	if len(t.AccessToken) == 0 {
		accessTokenInfo = "empty"
	} else if len(t.AccessToken) <= 8 {
		accessTokenInfo = "[REDACTED]"
	} else {
		accessTokenInfo = t.AccessToken[:8] + "...[REDACTED]"
	}

	// Create safe representation of refresh token
	var refreshTokenInfo string
	if len(t.RefreshToken) == 0 {
		refreshTokenInfo = "none"
	} else {
		refreshTokenInfo = "present"
	}

	// Format expiry information
	var expiryInfo string
	if t.ExpiresAt == nil {
		expiryInfo = "no_expiry"
	} else if t.IsExpired() {
		expiryInfo = "expired"
	} else {
		timeUntilExpiry := time.Until(*t.ExpiresAt).Round(time.Minute)
		expiryInfo = fmt.Sprintf("expires_in=%v", timeUntilExpiry)
	}

	return fmt.Sprintf("TokenInfo(type=%s, access_token=%s, refresh_token=%s, %s, scope_count=%d)",
		t.TokenType, accessTokenInfo, refreshTokenInfo, expiryInfo, len(t.Scope))
}

// AuthStrategy defines the interface for authentication strategies
type AuthStrategy interface {
	// Authenticate performs the initial authentication and returns token info
	Authenticate(ctx context.Context, config map[string]interface{}) (*TokenInfo, error)

	// RefreshToken refreshes an existing token
	RefreshToken(ctx context.Context, tokenInfo *TokenInfo, config map[string]interface{}) (*TokenInfo, error)

	// GetAuthType returns the authentication type this strategy handles
	GetAuthType() AuthType

	// SupportsRefresh returns true if this strategy supports token refresh
	SupportsRefresh() bool

	// ApplyAuth applies authentication to an HTTP request
	ApplyAuth(req *http.Request, tokenInfo *TokenInfo, config map[string]interface{}) error
}

// AuthType and AuthConfig are defined in config.go
