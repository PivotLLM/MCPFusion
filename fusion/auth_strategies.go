/*=============================================================================
= Copyright (c) 2025 Tenebris Technologies Inc.                              =
= All rights reserved.                                                       =
=============================================================================*/

package fusion

import (
	"context"
	"fmt"
	"net/http"

	"github.com/PivotLLM/MCPFusion/global"
)

// OAuth2DeviceFlowStrategy implements OAuth2 device flow authentication
type OAuth2DeviceFlowStrategy struct {
	httpClient *http.Client
	logger     global.Logger
}

// NewOAuth2DeviceFlowStrategy creates a new OAuth2 device flow strategy
func NewOAuth2DeviceFlowStrategy(httpClient *http.Client, logger global.Logger) *OAuth2DeviceFlowStrategy {
	return &OAuth2DeviceFlowStrategy{
		httpClient: httpClient,
		logger:     logger,
	}
}

func (s *OAuth2DeviceFlowStrategy) GetAuthType() AuthType {
	return AuthTypeOAuth2Device
}

func (s *OAuth2DeviceFlowStrategy) SupportsRefresh() bool {
	return true
}

func (s *OAuth2DeviceFlowStrategy) Authenticate(ctx context.Context, config map[string]interface{}) (*TokenInfo, error) {
	return nil, fmt.Errorf("OAuth2 device flow authentication not implemented in database-only mode")
}

func (s *OAuth2DeviceFlowStrategy) RefreshToken(ctx context.Context, tokenInfo *TokenInfo) (*TokenInfo, error) {
	return nil, fmt.Errorf("OAuth2 token refresh not implemented in database-only mode")
}

func (s *OAuth2DeviceFlowStrategy) ApplyAuth(req *http.Request, tokenInfo *TokenInfo) error {
	if tokenInfo == nil {
		return fmt.Errorf("token info is nil")
	}
	req.Header.Set("Authorization", tokenInfo.GetAuthorizationHeader())
	return nil
}

// BearerTokenStrategy implements bearer token authentication
type BearerTokenStrategy struct {
	logger global.Logger
}

// NewBearerTokenStrategy creates a new bearer token strategy
func NewBearerTokenStrategy(logger global.Logger) *BearerTokenStrategy {
	return &BearerTokenStrategy{logger: logger}
}

func (s *BearerTokenStrategy) GetAuthType() AuthType {
	return AuthTypeBearer
}

func (s *BearerTokenStrategy) SupportsRefresh() bool {
	return false
}

func (s *BearerTokenStrategy) Authenticate(ctx context.Context, config map[string]interface{}) (*TokenInfo, error) {
	return nil, fmt.Errorf("bearer token authentication not implemented in database-only mode")
}

func (s *BearerTokenStrategy) RefreshToken(ctx context.Context, tokenInfo *TokenInfo) (*TokenInfo, error) {
	return nil, fmt.Errorf("bearer token refresh not supported")
}

func (s *BearerTokenStrategy) ApplyAuth(req *http.Request, tokenInfo *TokenInfo) error {
	if tokenInfo == nil {
		return fmt.Errorf("token info is nil")
	}
	req.Header.Set("Authorization", tokenInfo.GetAuthorizationHeader())
	return nil
}

// APIKeyStrategy implements API key authentication
type APIKeyStrategy struct {
	logger global.Logger
}

// NewAPIKeyStrategy creates a new API key strategy
func NewAPIKeyStrategy(logger global.Logger) *APIKeyStrategy {
	return &APIKeyStrategy{logger: logger}
}

func (s *APIKeyStrategy) GetAuthType() AuthType {
	return AuthTypeAPIKey
}

func (s *APIKeyStrategy) SupportsRefresh() bool {
	return false
}

func (s *APIKeyStrategy) Authenticate(ctx context.Context, config map[string]interface{}) (*TokenInfo, error) {
	return nil, fmt.Errorf("API key authentication not implemented in database-only mode")
}

func (s *APIKeyStrategy) RefreshToken(ctx context.Context, tokenInfo *TokenInfo) (*TokenInfo, error) {
	return nil, fmt.Errorf("API key refresh not supported")
}

func (s *APIKeyStrategy) ApplyAuth(req *http.Request, tokenInfo *TokenInfo) error {
	if tokenInfo == nil {
		return fmt.Errorf("token info is nil")
	}
	// This would typically set an API key header
	req.Header.Set("Authorization", tokenInfo.GetAuthorizationHeader())
	return nil
}

// BasicAuthStrategy implements basic authentication
type BasicAuthStrategy struct {
	logger global.Logger
}

// NewBasicAuthStrategy creates a new basic auth strategy
func NewBasicAuthStrategy(logger global.Logger) *BasicAuthStrategy {
	return &BasicAuthStrategy{logger: logger}
}

func (s *BasicAuthStrategy) GetAuthType() AuthType {
	return AuthTypeBasic
}

func (s *BasicAuthStrategy) SupportsRefresh() bool {
	return false
}

func (s *BasicAuthStrategy) Authenticate(ctx context.Context, config map[string]interface{}) (*TokenInfo, error) {
	return nil, fmt.Errorf("basic authentication not implemented in database-only mode")
}

func (s *BasicAuthStrategy) RefreshToken(ctx context.Context, tokenInfo *TokenInfo) (*TokenInfo, error) {
	return nil, fmt.Errorf("basic auth refresh not supported")
}

func (s *BasicAuthStrategy) ApplyAuth(req *http.Request, tokenInfo *TokenInfo) error {
	if tokenInfo == nil {
		return fmt.Errorf("token info is nil")
	}
	req.Header.Set("Authorization", tokenInfo.GetAuthorizationHeader())
	return nil
}