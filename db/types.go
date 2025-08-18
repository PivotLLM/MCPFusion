/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package db

import (
	"time"
)

// APITokenMetadata represents metadata for an API token
type APITokenMetadata struct {
	Hash        string    `json:"hash"`        // SHA-256 hash of the original token
	CreatedAt   time.Time `json:"created_at"`  // When the token was created
	LastUsed    time.Time `json:"last_used"`   // When the token was last used
	Description string    `json:"description"` // Optional description
	Prefix      string    `json:"prefix"`      // First 8 chars for identification
}

// OAuthTokenData represents stored OAuth token information
type OAuthTokenData struct {
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token,omitempty"`
	TokenType    string     `json:"token_type"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Scope        []string   `json:"scope,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// IsExpired checks if the OAuth token is expired
func (o *OAuthTokenData) IsExpired() bool {
	if o.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*o.ExpiresAt)
}

// IsExpiredWithBuffer checks if the OAuth token is expired with a buffer time
func (o *OAuthTokenData) IsExpiredWithBuffer(buffer time.Duration) bool {
	if o.ExpiresAt == nil {
		return false
	}
	return time.Now().Add(buffer).After(*o.ExpiresAt)
}

// HasRefreshToken checks if the OAuth token has a refresh token
func (o *OAuthTokenData) HasRefreshToken() bool {
	return o.RefreshToken != ""
}

// CredentialType represents the type of service credential
type CredentialType string

const (
	CredentialTypeAPIKey    CredentialType = "api_key"
	CredentialTypeBearer    CredentialType = "bearer"
	CredentialTypeBasicAuth CredentialType = "basic_auth"
	CredentialTypeCustom    CredentialType = "custom"
)

// ServiceCredentials represents other service authentication data
type ServiceCredentials struct {
	Type      CredentialType         `json:"type"` // Type of credential
	Data      map[string]interface{} `json:"data"` // Service-specific credential data
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// TenantInfo represents information about a tenant
type TenantInfo struct {
	Hash        string    `json:"hash"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsed    time.Time `json:"last_used"`
	OAuthCount  int       `json:"oauth_count"`
	CredCount   int       `json:"credential_count"`
}
