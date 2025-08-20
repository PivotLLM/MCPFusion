/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PivotLLM/MCPFusion/cmd/fusion-oauth/providers"
)

// Provider implements OAuth for GitHub
type Provider struct {
	httpClient *http.Client
}

// NewProvider creates a new GitHub OAuth provider
func NewProvider() *Provider {
	return &Provider{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *Provider) GetServiceName() string {
	return "github"
}

func (p *Provider) GetDisplayName() string {
	return "GitHub"
}

func (p *Provider) GetRequiredScopes() []string {
	return []string{
		"user:email",
		"read:user",
	}
}

func (p *Provider) GetAuthorizationEndpoint() string {
	return "https://github.com/login/oauth/authorize"
}

func (p *Provider) GetTokenEndpoint() string {
	return "https://github.com/login/oauth/access_token"
}

func (p *Provider) GetDeviceCodeEndpoint() string {
	return "https://github.com/login/device/code"
}

func (p *Provider) SupportsDeviceFlow() bool {
	return false // Temporarily disabled to test auth code flow
}

func (p *Provider) SupportsAuthorizationCode() bool {
	return true
}

func (p *Provider) ValidateConfiguration(config *providers.ServiceConfig) error {
	if config.ClientID == "" {
		return fmt.Errorf("client_id is required for GitHub OAuth")
	}

	// Client secret is required for GitHub OAuth flows
	if config.ClientSecret == "" {
		return fmt.Errorf("client_secret is required for GitHub OAuth")
	}

	// Validate scopes - GitHub scopes are more flexible than Google
	if config.Scopes != "" {
		scopes := strings.Fields(config.Scopes)
		for _, scope := range scopes {
			if scope == "" {
				return fmt.Errorf("empty scope found in configuration")
			}
			// Basic scope validation - GitHub has many valid scopes
			if !p.isValidGitHubScope(scope) {
				return fmt.Errorf("invalid GitHub scope: %s", scope)
			}
		}
	}

	return nil
}

func (p *Provider) isValidGitHubScope(scope string) bool {
	validScopes := []string{
		"repo", "repo:status", "repo_deployment", "public_repo", "repo:invite",
		"security_events", "admin:repo_hook", "write:repo_hook", "read:repo_hook",
		"admin:org", "write:org", "read:org", "admin:public_key", "write:public_key",
		"read:public_key", "admin:org_hook", "gist", "notifications", "user",
		"read:user", "user:email", "user:follow", "delete_repo", "write:discussion",
		"read:discussion", "write:packages", "read:packages", "delete:packages",
		"admin:gpg_key", "write:gpg_key", "read:gpg_key", "codespace",
		"workflow", "admin:enterprise", "manage_billing:enterprise", "read:enterprise",
	}

	for _, validScope := range validScopes {
		if scope == validScope {
			return true
		}
	}

	// Check for scope prefixes
	prefixes := []string{"admin:", "write:", "read:", "delete:", "manage_billing:"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(scope, prefix) {
			return true
		}
	}

	return false
}

func (p *Provider) CustomizeDeviceRequest(params map[string]string, config *providers.ServiceConfig) error {
	params["client_id"] = config.ClientID
	
	// Set default scopes if none provided
	scopes := config.Scopes
	if scopes == "" {
		scopes = strings.Join(p.GetRequiredScopes(), " ")
	}
	params["scope"] = scopes

	return nil
}

func (p *Provider) CustomizeTokenRequest(params map[string]string, config *providers.ServiceConfig) error {
	params["client_id"] = config.ClientID
	params["client_secret"] = config.ClientSecret

	return nil
}

func (p *Provider) ProcessTokenResponse(response map[string]interface{}) (*providers.TokenInfo, error) {
	tokenInfo := &providers.TokenInfo{}

	// Extract access token (required)
	if accessToken, ok := response["access_token"].(string); ok {
		tokenInfo.AccessToken = accessToken
	} else {
		return nil, fmt.Errorf("access_token not found in response")
	}

	// GitHub tokens are always Bearer type
	tokenInfo.TokenType = "Bearer"

	// Extract refresh token (GitHub doesn't typically provide refresh tokens)
	if refreshToken, ok := response["refresh_token"].(string); ok {
		tokenInfo.RefreshToken = refreshToken
	}

	// GitHub doesn't always provide expiration, tokens are long-lived
	if expiresIn, ok := response["expires_in"].(float64); ok {
		tokenInfo.ExpiresIn = int(expiresIn)
		expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
		tokenInfo.ExpiresAt = &expiresAt
	}

	// Extract scope
	if scope, ok := response["scope"].(string); ok {
		tokenInfo.Scope = strings.Split(scope, ",")
		// Trim whitespace from scopes
		for i, s := range tokenInfo.Scope {
			tokenInfo.Scope[i] = strings.TrimSpace(s)
		}
	}

	// GitHub-specific token type info
	if tokenInfo.Metadata == nil {
		tokenInfo.Metadata = make(map[string]string)
	}
	
	if tokenType, ok := response["token_type"].(string); ok {
		tokenInfo.Metadata["github_token_type"] = tokenType
	}

	return tokenInfo, nil
}

func (p *Provider) GetUserInfo(ctx context.Context, token *providers.TokenInfo) (*providers.UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user info request: %w", err)
	}

	req.Header.Set("Authorization", token.GetAuthorizationHeader())
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "fusion-oauth/1.0")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user info request failed with status: %d", resp.StatusCode)
	}

	var userInfoResp struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
		Company   string `json:"company"`
		Location  string `json:"location"`
		Bio       string `json:"bio"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&userInfoResp); err != nil {
		return nil, fmt.Errorf("failed to decode user info response: %w", err)
	}

	// If primary email is not public, try to get it from the emails endpoint
	email := userInfoResp.Email
	if email == "" {
		email, _ = p.getPrimaryEmail(ctx, token)
	}

	displayName := userInfoResp.Name
	if displayName == "" {
		displayName = userInfoResp.Login
	}

	return &providers.UserInfo{
		ID:          fmt.Sprintf("%d", userInfoResp.ID),
		Email:       email,
		Name:        userInfoResp.Login,
		DisplayName: displayName,
	}, nil
}

func (p *Provider) getPrimaryEmail(ctx context.Context, token *providers.TokenInfo) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", token.GetAuthorizationHeader())
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "fusion-oauth/1.0")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("emails request failed with status: %d", resp.StatusCode)
	}

	var emails []struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
		Verified bool  `json:"verified"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	// Find primary email
	for _, email := range emails {
		if email.Primary && email.Verified {
			return email.Email, nil
		}
	}

	// Fall back to first verified email
	for _, email := range emails {
		if email.Verified {
			return email.Email, nil
		}
	}

	return "", fmt.Errorf("no verified email found")
}

// GetExtendedScopes returns common extended scopes for GitHub
func (p *Provider) GetExtendedScopes() map[string][]string {
	return map[string][]string{
		"repositories": {
			"repo",
			"public_repo",
			"repo:status",
			"repo_deployment",
		},
		"organizations": {
			"read:org",
			"write:org",
			"admin:org",
		},
		"gists": {
			"gist",
		},
		"notifications": {
			"notifications",
		},
		"packages": {
			"read:packages",
			"write:packages",
			"delete:packages",
		},
		"actions": {
			"workflow",
		},
	}
}

// ValidateToken validates a GitHub OAuth token by making a test API call
func (p *Provider) ValidateToken(ctx context.Context, token *providers.TokenInfo) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return fmt.Errorf("failed to create token validation request: %w", err)
	}

	req.Header.Set("Authorization", token.GetAuthorizationHeader())
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "fusion-oauth/1.0")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("token is invalid or expired")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token validation failed with status: %d", resp.StatusCode)
	}

	return nil
}