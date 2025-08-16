/*=============================================================================
= Copyright (c) 2025 Tenebris Technologies Inc.                              =
= All rights reserved.                                                       =
=============================================================================*/

package fusion

import (
	"context"
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

// AuthStrategy defines the interface for authentication strategies
type AuthStrategy interface {
	// Authenticate performs the initial authentication and returns token info
	Authenticate(ctx context.Context, config map[string]interface{}) (*TokenInfo, error)

	// RefreshToken refreshes an existing token
	RefreshToken(ctx context.Context, tokenInfo *TokenInfo) (*TokenInfo, error)

	// GetAuthType returns the authentication type this strategy handles
	GetAuthType() AuthType

	// SupportsRefresh returns true if this strategy supports token refresh
	SupportsRefresh() bool

	// ApplyAuth applies authentication to an HTTP request
	ApplyAuth(req *http.Request, tokenInfo *TokenInfo) error
}

// AuthType and AuthConfig are defined in config.go