/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
)

// OAuth2ExternalStrategy implements OAuth2 authentication for services where tokens
// are provided externally by the fusion-auth helper tool (e.g., Google APIs that
// cannot use device flow).
type OAuth2ExternalStrategy struct {
	httpClient *http.Client
	logger     global.Logger
}

// NewOAuth2ExternalStrategy creates a new OAuth2 external strategy
func NewOAuth2ExternalStrategy(httpClient *http.Client, logger global.Logger) *OAuth2ExternalStrategy {
	return &OAuth2ExternalStrategy{
		httpClient: httpClient,
		logger:     logger,
	}
}

func (s *OAuth2ExternalStrategy) GetAuthType() AuthType {
	return AuthTypeOAuth2External
}

func (s *OAuth2ExternalStrategy) SupportsRefresh() bool {
	return true
}

func (s *OAuth2ExternalStrategy) Authenticate(_ context.Context, _ map[string]interface{}) (*TokenInfo, error) {
	return nil, fmt.Errorf("no stored token found for this service. Please run fusion-auth to authenticate.")
}

func (s *OAuth2ExternalStrategy) ApplyAuth(req *http.Request, tokenInfo *TokenInfo) error {
	if tokenInfo == nil {
		return fmt.Errorf("token info is nil")
	}
	req.Header.Set("Authorization", tokenInfo.GetAuthorizationHeader())
	return nil
}

func (s *OAuth2ExternalStrategy) RefreshToken(ctx context.Context, tokenInfo *TokenInfo, config map[string]interface{}) (*TokenInfo, error) {
	if tokenInfo == nil {
		return nil, fmt.Errorf("token info is nil")
	}

	if tokenInfo.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	if config == nil {
		return nil, fmt.Errorf("authentication configuration is required")
	}

	// Extract configuration parameters (check both camelCase and snake_case)
	clientID, ok := config["clientId"].(string)
	if !ok || clientID == "" {
		clientID, ok = config["client_id"].(string)
		if !ok || clientID == "" {
			return nil, fmt.Errorf("clientId is required for OAuth2 token refresh")
		}
	}

	tokenEndpoint, ok := config["tokenURL"].(string)
	if !ok || tokenEndpoint == "" {
		tokenEndpoint, ok = config["token_endpoint"].(string)
		if !ok || tokenEndpoint == "" {
			return nil, fmt.Errorf("tokenURL is required for OAuth2 token refresh")
		}
	}

	// Extract client secret (check both camelCase and snake_case)
	clientSecret, _ := config["clientSecret"].(string)
	if clientSecret == "" {
		clientSecret, _ = config["client_secret"].(string)
	}

	scope, _ := config["scope"].(string)

	if s.logger != nil {
		s.logger.Debugf("Refreshing OAuth2 external token: client_id=%s, token_endpoint=%s, scope=%s",
			clientID, tokenEndpoint, scope)
	}

	// Prepare form data for refresh token request
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", clientID)
	data.Set("refresh_token", tokenInfo.RefreshToken)

	if scope != "" {
		data.Set("scope", scope)
	}

	if clientSecret != "" {
		data.Set("client_secret", clientSecret)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	if s.logger != nil {
		s.logger.Debugf("Sending token refresh request: client_id=%s, scope=%s", clientID, scope)
	}

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send token refresh request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
		}
	}(resp.Body)

	if s.logger != nil {
		s.logger.Debugf("Token refresh response status: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if s.logger != nil {
			s.logger.Errorf("Token refresh request failed: status=%d, body=%s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("token refresh request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		if s.logger != nil {
			s.logger.Errorf("Failed to parse token refresh response: %v, body: %s", err, string(body))
		}
		return nil, fmt.Errorf("failed to parse token refresh response: %w", err)
	}

	// Convert to TokenInfo
	newTokenInfo := &TokenInfo{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: tokenResp.RefreshToken,
		Scope:        strings.Split(tokenResp.Scope, " "),
		Metadata:     make(map[string]string),
	}

	// If no new refresh token is provided, keep the old one (some providers don't rotate refresh tokens)
	if newTokenInfo.RefreshToken == "" {
		newTokenInfo.RefreshToken = tokenInfo.RefreshToken
		if s.logger != nil {
			s.logger.Debugf("No new refresh token provided, keeping existing refresh token")
		}
	}

	// Calculate expiration time
	if tokenResp.ExpiresIn > 0 {
		expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		newTokenInfo.ExpiresAt = &expiresAt
	}

	if s.logger != nil {
		expiryInfo := "no expiry"
		if newTokenInfo.ExpiresAt != nil {
			expiryInfo = fmt.Sprintf("expires at %s", newTokenInfo.ExpiresAt.Format(time.RFC3339))
		}
		s.logger.Infof("Successfully refreshed OAuth2 external token (%s)", expiryInfo)
	}

	return newTokenInfo, nil
}
