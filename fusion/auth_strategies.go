/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
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

// OAuth2DeviceFlowStrategy implements OAuth2 device flow authentication
type OAuth2DeviceFlowStrategy struct {
	httpClient  *http.Client
	logger      global.Logger
	authManager *MultiTenantAuthManager // Reference to auth manager for token storage
}

// NewOAuth2DeviceFlowStrategy creates a new OAuth2 device flow strategy
func NewOAuth2DeviceFlowStrategy(httpClient *http.Client, logger global.Logger) *OAuth2DeviceFlowStrategy {
	return &OAuth2DeviceFlowStrategy{
		httpClient:  httpClient,
		logger:      logger,
		authManager: nil, // Will be set by the auth manager when registering
	}
}

// SetAuthManager sets the auth manager reference (called by MultiTenantAuthManager)
func (s *OAuth2DeviceFlowStrategy) SetAuthManager(authManager *MultiTenantAuthManager) {
	s.authManager = authManager
}

func (s *OAuth2DeviceFlowStrategy) GetAuthType() AuthType {
	return AuthTypeOAuth2Device
}

func (s *OAuth2DeviceFlowStrategy) SupportsRefresh() bool {
	return true
}

// PollingContext holds the context needed for background token polling
type PollingContext struct {
	TenantHash  string
	ServiceName string
	AuthConfig  map[string]interface{}
}

func (s *OAuth2DeviceFlowStrategy) Authenticate(ctx context.Context, config map[string]interface{}) (*TokenInfo, error) {
	if s.logger != nil {
		s.logger.Infof("Starting OAuth2 device flow authentication")
	}

	// Extract tenant context from the request context
	var pollingCtx *PollingContext
	if tenantContextValue := ctx.Value(global.TenantContextKey); tenantContextValue != nil {
		if tenantContext, ok := tenantContextValue.(*TenantContext); ok {
			pollingCtx = &PollingContext{
				TenantHash:  tenantContext.TenantHash,
				ServiceName: tenantContext.ServiceName,
				AuthConfig:  config,
			}
			if s.logger != nil {
				s.logger.Debugf("Extracted tenant context for background polling: tenant=%s, service=%s",
					pollingCtx.TenantHash[:12]+"...", pollingCtx.ServiceName)
			}
		}
	}

	// Extract configuration parameters (check both camelCase and snake_case)
	clientID, ok := config["clientId"].(string)
	if !ok || clientID == "" {
		clientID, ok = config["client_id"].(string)
		if !ok || clientID == "" {
			return nil, fmt.Errorf("clientId is required for OAuth2 device flow")
		}
	}

	// Look for device endpoint - check multiple possible field names
	deviceEndpoint, ok := config["authorizationURL"].(string)
	if !ok || deviceEndpoint == "" {
		deviceEndpoint, ok = config["device_endpoint"].(string)
		if !ok || deviceEndpoint == "" {
			deviceEndpoint, ok = config["deviceCodeURL"].(string)
			if !ok || deviceEndpoint == "" {
				return nil, fmt.Errorf("authorizationURL (device endpoint) is required for OAuth2 device flow")
			}
		}
	}

	tokenEndpoint, ok := config["tokenURL"].(string)
	if !ok || tokenEndpoint == "" {
		tokenEndpoint, ok = config["token_endpoint"].(string)
		if !ok || tokenEndpoint == "" {
			return nil, fmt.Errorf("tokenURL is required for OAuth2 device flow")
		}
	}

	scope, _ := config["scope"].(string)
	if scope == "" {
		scope = "https://graph.microsoft.com/.default"
	}

	// Handle tenant ID replacement in URLs
	tenantID, _ := config["tenantId"].(string)
	if tenantID == "" {
		tenantID, _ = config["tenant_id"].(string)
	}
	if tenantID == "" {
		tenantID = "common" // Default to common endpoint
	}

	// Replace {tenantId} placeholder in URLs
	deviceEndpoint = strings.ReplaceAll(deviceEndpoint, "{tenantId}", tenantID)
	tokenEndpoint = strings.ReplaceAll(tokenEndpoint, "{tenantId}", tenantID)

	if s.logger != nil {
		s.logger.Debugf("OAuth2 device flow config: client_id=%s, tenant_id=%s, device_endpoint=%s, token_endpoint=%s, scope=%s",
			clientID, tenantID, deviceEndpoint, tokenEndpoint, scope)
	}

	// Step 1: Request device code
	deviceCodeResp, err := s.requestDeviceCode(ctx, deviceEndpoint, clientID, scope)
	if err != nil {
		if s.logger != nil {
			s.logger.Errorf("Failed to request device code: %v", err)
		}
		return nil, fmt.Errorf("failed to request device code: %w", err)
	}

	if s.logger != nil {
		s.logger.Infof("Device code obtained: device_code=%s, user_code=%s, verification_uri=%s, expires_in=%d",
			deviceCodeResp.DeviceCode[:12]+"...", deviceCodeResp.UserCode, deviceCodeResp.VerificationURI, deviceCodeResp.ExpiresIn)
	}

	// Launch background goroutine to poll for token completion
	if s.logger != nil {
		s.logger.Infof("Launching background token polling for device code %s (expires in %d seconds)",
			deviceCodeResp.DeviceCode[:12]+"...", deviceCodeResp.ExpiresIn)
	}

	// Start background polling with a timeout of 10 minutes (or device expiry, whichever is shorter)
	pollTimeout := time.Duration(deviceCodeResp.ExpiresIn) * time.Second
	if pollTimeout > 10*time.Minute {
		pollTimeout = 10 * time.Minute
	}

	go s.backgroundTokenPolling(context.Background(), tokenEndpoint, clientID, deviceCodeResp.DeviceCode,
		deviceCodeResp.Interval, pollTimeout, pollingCtx)

	// Return a DeviceCodeError to signal that user authentication is required
	deviceCodeError := &DeviceCodeError{
		DeviceCode:              deviceCodeResp.DeviceCode,
		UserCode:                deviceCodeResp.UserCode,
		VerificationURL:         deviceCodeResp.VerificationURI,
		VerificationURLComplete: deviceCodeResp.VerificationURIComplete,
		ExpiresIn:               deviceCodeResp.ExpiresIn,
		Interval:                deviceCodeResp.Interval,
		// Don't set Message to avoid duplication - Error() method will generate the message
	}

	if s.logger != nil {
		s.logger.Infof("Returning device code authentication request to client")
	}

	return nil, deviceCodeError
}

func (s *OAuth2DeviceFlowStrategy) RefreshToken(ctx context.Context, tokenInfo *TokenInfo, config map[string]interface{}) (*TokenInfo, error) {
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

	scope, _ := config["scope"].(string)
	if scope == "" {
		scope = "https://graph.microsoft.com/.default"
	}

	// Handle tenant ID replacement in URLs
	tenantID, _ := config["tenantId"].(string)
	if tenantID == "" {
		tenantID, _ = config["tenant_id"].(string)
	}
	if tenantID == "" {
		tenantID = "common" // Default to common endpoint
	}

	// Replace {tenantId} placeholder in URL
	tokenEndpoint = strings.ReplaceAll(tokenEndpoint, "{tenantId}", tenantID)

	if s.logger != nil {
		s.logger.Debugf("Refreshing OAuth2 token: client_id=%s, tenant_id=%s, token_endpoint=%s, scope=%s",
			clientID, tenantID, tokenEndpoint, scope)
	}

	// Prepare form data for refresh token request
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("scope", scope)
	data.Set("client_id", clientID)
	data.Set("refresh_token", tokenInfo.RefreshToken)

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
	defer resp.Body.Close()

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
		Scope:        strings.Split(tokenResp.Scope, " "), // Split space-separated scope string
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
		s.logger.Infof("Successfully refreshed OAuth2 token (%s)", expiryInfo)
	}

	return newTokenInfo, nil
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

func (s *BearerTokenStrategy) RefreshToken(ctx context.Context, tokenInfo *TokenInfo, config map[string]interface{}) (*TokenInfo, error) {
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

func (s *APIKeyStrategy) RefreshToken(ctx context.Context, tokenInfo *TokenInfo, config map[string]interface{}) (*TokenInfo, error) {
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

func (s *BasicAuthStrategy) RefreshToken(ctx context.Context, tokenInfo *TokenInfo, config map[string]interface{}) (*TokenInfo, error) {
	return nil, fmt.Errorf("basic auth refresh not supported")
}

func (s *BasicAuthStrategy) ApplyAuth(req *http.Request, tokenInfo *TokenInfo) error {
	if tokenInfo == nil {
		return fmt.Errorf("token info is nil")
	}
	req.Header.Set("Authorization", tokenInfo.GetAuthorizationHeader())
	return nil
}

// DeviceCodeResponse represents the response from a device code request
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// TokenResponse represents the response from a token request
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// requestDeviceCode requests a device code from the OAuth2 provider
func (s *OAuth2DeviceFlowStrategy) requestDeviceCode(ctx context.Context, deviceEndpoint, clientID, scope string) (*DeviceCodeResponse, error) {
	if s.logger != nil {
		s.logger.Debugf("Requesting device code from %s", deviceEndpoint)
	}

	// Prepare form data
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("scope", scope)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", deviceEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create device code request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	if s.logger != nil {
		s.logger.Debugf("Sending device code request: client_id=%s, scope=%s", clientID, scope)
	}

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send device code request: %w", err)
	}
	defer resp.Body.Close()

	if s.logger != nil {
		s.logger.Debugf("Device code response status: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read device code response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if s.logger != nil {
			s.logger.Errorf("Device code request failed: status=%d, body=%s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("device code request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var deviceCodeResp DeviceCodeResponse
	if err := json.Unmarshal(body, &deviceCodeResp); err != nil {
		if s.logger != nil {
			s.logger.Errorf("Failed to parse device code response: %v, body: %s", err, string(body))
		}
		return nil, fmt.Errorf("failed to parse device code response: %w", err)
	}

	if s.logger != nil {
		s.logger.Debugf("Device code response parsed successfully: user_code=%s, verification_uri=%s",
			deviceCodeResp.UserCode, deviceCodeResp.VerificationURI)
	}

	return &deviceCodeResp, nil
}

// pollForToken polls the token endpoint until the user completes authentication
func (s *OAuth2DeviceFlowStrategy) pollForToken(ctx context.Context, tokenEndpoint, clientID, deviceCode string, interval int) (*TokenInfo, error) {
	if s.logger != nil {
		s.logger.Debugf("Starting token polling with interval %d seconds", interval)
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if s.logger != nil {
				s.logger.Debugf("Polling for token...")
			}

			tokenInfo, err := s.requestToken(ctx, tokenEndpoint, clientID, deviceCode)
			if err != nil {
				// Check if it's a pending authorization error
				if strings.Contains(err.Error(), "authorization_pending") {
					if s.logger != nil {
						s.logger.Debugf("Authorization still pending, continuing to poll...")
					}
					continue
				}
				if strings.Contains(err.Error(), "slow_down") {
					if s.logger != nil {
						s.logger.Debugf("Rate limited, increasing polling interval...")
					}
					// Increase polling interval
					ticker.Reset(time.Duration(interval+5) * time.Second)
					continue
				}
				return nil, err
			}

			if s.logger != nil {
				s.logger.Infof("Token obtained successfully")
			}
			return tokenInfo, nil
		}
	}
}

// requestToken requests an access token using the device code
func (s *OAuth2DeviceFlowStrategy) requestToken(ctx context.Context, tokenEndpoint, clientID, deviceCode string) (*TokenInfo, error) {
	// Prepare form data
	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	data.Set("client_id", clientID)
	data.Set("device_code", deviceCode)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send token request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Parse error response
		var errorResp map[string]interface{}
		if err := json.Unmarshal(body, &errorResp); err == nil {
			if errorStr, ok := errorResp["error"].(string); ok {
				return nil, fmt.Errorf("%s", errorStr)
			}
		}
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Convert to TokenInfo
	tokenInfo := &TokenInfo{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: tokenResp.RefreshToken,
		Scope:        strings.Split(tokenResp.Scope, " "), // Split space-separated scope string
		Metadata:     make(map[string]string),
	}

	// Calculate expiration time
	if tokenResp.ExpiresIn > 0 {
		expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		tokenInfo.ExpiresAt = &expiresAt
	}

	return tokenInfo, nil
}

// backgroundTokenPolling polls for token completion in the background
func (s *OAuth2DeviceFlowStrategy) backgroundTokenPolling(ctx context.Context, tokenEndpoint, clientID, deviceCode string, interval int, timeout time.Duration, pollingCtx *PollingContext) {
	if s.logger != nil {
		s.logger.Infof("Starting background token polling for device code %s (timeout: %v)", deviceCode[:12]+"...", timeout)
	}

	// Create a context with timeout
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Start polling with exponential backoff for rate limiting
	currentInterval := interval
	ticker := time.NewTicker(time.Duration(currentInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-pollCtx.Done():
			if s.logger != nil {
				s.logger.Infof("Token polling timeout reached for device code %s", deviceCode[:12]+"...")
			}
			return

		case <-ticker.C:
			if s.logger != nil {
				s.logger.Debugf("Polling for token completion...")
			}

			tokenInfo, err := s.requestToken(pollCtx, tokenEndpoint, clientID, deviceCode)
			if err != nil {
				// Check for specific OAuth2 error responses
				if strings.Contains(err.Error(), "authorization_pending") {
					if s.logger != nil {
						s.logger.Debugf("Authorization still pending, continuing to poll...")
					}
					continue
				}

				if strings.Contains(err.Error(), "slow_down") {
					if s.logger != nil {
						s.logger.Debugf("Rate limited, increasing polling interval...")
					}
					// Increase polling interval by 5 seconds
					currentInterval += 5
					ticker.Reset(time.Duration(currentInterval) * time.Second)
					continue
				}

				if strings.Contains(err.Error(), "expired_token") || strings.Contains(err.Error(), "authorization_declined") {
					if s.logger != nil {
						s.logger.Warningf("Token polling failed permanently: %v", err)
					}
					return
				}

				// Other errors - log and continue for a few attempts
				if s.logger != nil {
					s.logger.Warningf("Token polling error: %v", err)
				}
				continue
			}

			// Successfully obtained token - store it
			if s.logger != nil {
				s.logger.Infof("Token obtained successfully via background polling")
			}

			// Store the token in the database using multi-tenant auth manager
			if err := s.storeTokenFromPolling(pollCtx, tokenInfo, pollingCtx); err != nil {
				if s.logger != nil {
					s.logger.Errorf("Failed to store token from background polling: %v", err)
				}
			} else {
				if s.logger != nil {
					s.logger.Infof("Token successfully stored from background polling")
				}
			}
			return
		}
	}
}

// storeTokenFromPolling stores the token obtained from background polling
func (s *OAuth2DeviceFlowStrategy) storeTokenFromPolling(ctx context.Context, tokenInfo *TokenInfo, pollingCtx *PollingContext) error {
	if tokenInfo == nil {
		return fmt.Errorf("token info is nil")
	}

	if pollingCtx == nil {
		if s.logger != nil {
			s.logger.Warningf("No polling context available - cannot store token")
		}
		return fmt.Errorf("polling context is nil")
	}

	if s.authManager == nil {
		if s.logger != nil {
			s.logger.Warningf("No auth manager available - cannot store token")
		}
		return fmt.Errorf("auth manager not available")
	}

	// Create tenant context for token storage
	tenantContext := &TenantContext{
		TenantHash:  pollingCtx.TenantHash,
		ServiceName: pollingCtx.ServiceName,
		CreatedAt:   time.Now(),
	}

	if s.logger != nil {
		s.logger.Infof("Storing token from background polling for tenant %s service %s: type=%s, expires_at=%v, has_refresh=%v",
			tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName,
			tokenInfo.TokenType, tokenInfo.ExpiresAt, tokenInfo.HasRefreshToken())
	}

	// Use the auth manager's CacheToken method to store in both database and cache
	s.authManager.CacheToken(tenantContext, tokenInfo)

	if s.logger != nil {
		s.logger.Infof("Successfully stored token from background polling for tenant %s service %s",
			tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
	}

	return nil
}
