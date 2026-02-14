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

// ShortHash returns a truncated version of the tenant hash for logging
// Safely handles hashes shorter than 12 characters (e.g., "NOAUTH")
func (pc *PollingContext) ShortHash() string {
	if pc == nil || pc.TenantHash == "" {
		return "unknown"
	}
	if len(pc.TenantHash) <= 12 {
		return pc.TenantHash
	}
	return pc.TenantHash[:12] + "..."
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
					pollingCtx.ShortHash(), pollingCtx.ServiceName)
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

func (s *OAuth2DeviceFlowStrategy) ApplyAuth(req *http.Request, tokenInfo *TokenInfo, _ map[string]interface{}) error {
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

func (s *BearerTokenStrategy) Authenticate(_ context.Context, _ map[string]interface{}) (*TokenInfo, error) {
	return nil, fmt.Errorf("bearer token authentication not implemented in database-only mode")
}

func (s *BearerTokenStrategy) RefreshToken(_ context.Context, _ *TokenInfo, _ map[string]interface{}) (*TokenInfo, error) {
	return nil, fmt.Errorf("bearer token refresh not supported")
}

func (s *BearerTokenStrategy) ApplyAuth(req *http.Request, tokenInfo *TokenInfo, _ map[string]interface{}) error {
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

func (s *APIKeyStrategy) Authenticate(_ context.Context, _ map[string]interface{}) (*TokenInfo, error) {
	return nil, fmt.Errorf("API key authentication not implemented in database-only mode")
}

func (s *APIKeyStrategy) RefreshToken(_ context.Context, _ *TokenInfo, _ map[string]interface{}) (*TokenInfo, error) {
	return nil, fmt.Errorf("API key refresh not supported")
}

func (s *APIKeyStrategy) ApplyAuth(req *http.Request, tokenInfo *TokenInfo, _ map[string]interface{}) error {
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

func (s *BasicAuthStrategy) Authenticate(_ context.Context, _ map[string]interface{}) (*TokenInfo, error) {
	return nil, fmt.Errorf("basic authentication not implemented in database-only mode")
}

func (s *BasicAuthStrategy) RefreshToken(_ context.Context, _ *TokenInfo, _ map[string]interface{}) (*TokenInfo, error) {
	return nil, fmt.Errorf("basic auth refresh not supported")
}

func (s *BasicAuthStrategy) ApplyAuth(req *http.Request, tokenInfo *TokenInfo, _ map[string]interface{}) error {
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
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
		}
	}(resp.Body)

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
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
		}
	}(resp.Body)

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
func (s *OAuth2DeviceFlowStrategy) storeTokenFromPolling(_ context.Context, tokenInfo *TokenInfo, pollingCtx *PollingContext) error {
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
		var expiryInfo string
		if tokenInfo.ExpiresAt != nil {
			timeUntilExpiry := time.Until(*tokenInfo.ExpiresAt).Round(time.Minute)
			if timeUntilExpiry > 0 {
				expiryInfo = fmt.Sprintf("expires_in=%v", timeUntilExpiry)
			} else {
				expiryInfo = "expired"
			}
		} else {
			expiryInfo = "no_expiry"
		}
		s.logger.Infof("Storing token from background polling for tenant %s service %s: type=%s, %s, has_refresh=%v",
			tenantContext.ShortHash(), tenantContext.ServiceName,
			tokenInfo.TokenType, expiryInfo, tokenInfo.HasRefreshToken())
	}

	// Use the auth manager's CacheToken method to store in both database and cache
	s.authManager.CacheToken(tenantContext, tokenInfo)

	if s.logger != nil {
		s.logger.Infof("Successfully stored token from background polling for tenant %s service %s",
			tenantContext.ShortHash(), tenantContext.ServiceName)
	}

	return nil
}

// ============================================================================
// Session JWT Strategy
// ============================================================================

// SessionJWTStrategy implements session-based JWT authentication
// This strategy supports login-based authentication where:
// 1. Credentials are POSTed to a login endpoint
// 2. A JWT token is extracted from the JSON response
// 3. The token is applied to requests as a header, cookie, or query parameter
// 4. Optional token refresh via a separate endpoint
type SessionJWTStrategy struct {
	httpClient *http.Client
	logger     global.Logger
}

// NewSessionJWTStrategy creates a new session JWT strategy
func NewSessionJWTStrategy(httpClient *http.Client, logger global.Logger) *SessionJWTStrategy {
	return &SessionJWTStrategy{
		httpClient: httpClient,
		logger:     logger,
	}
}

func (s *SessionJWTStrategy) GetAuthType() AuthType {
	return AuthTypeSessionJWT
}

func (s *SessionJWTStrategy) SupportsRefresh() bool {
	return true // We support refresh if refreshURL is configured
}

func (s *SessionJWTStrategy) Authenticate(ctx context.Context, config map[string]interface{}) (*TokenInfo, error) {
	if s.logger != nil {
		s.logger.Infof("Starting session JWT authentication")
	}

	// Extract configuration
	loginURL, _ := config["loginURL"].(string)
	if loginURL == "" {
		return nil, fmt.Errorf("loginURL is required for session_jwt auth")
	}

	baseURL, _ := config["baseURL"].(string)
	if baseURL != "" {
		// Combine baseURL with loginURL if loginURL is relative
		if !strings.HasPrefix(loginURL, "http://") && !strings.HasPrefix(loginURL, "https://") {
			loginURL = strings.TrimSuffix(baseURL, "/") + "/" + strings.TrimPrefix(loginURL, "/")
		}
	}

	loginMethod := "POST"
	if m, ok := config["loginMethod"].(string); ok && m != "" {
		loginMethod = strings.ToUpper(m)
	}

	contentType := "application/json"
	if ct, ok := config["loginContentType"].(string); ok && ct != "" {
		contentType = ct
	}

	tokenPath, _ := config["tokenPath"].(string)
	if tokenPath == "" {
		return nil, fmt.Errorf("tokenPath is required for session_jwt auth")
	}

	// Build request body
	var bodyReader io.Reader
	if loginBody, ok := config["loginBody"].(map[string]interface{}); ok {
		bodyBytes, err := json.Marshal(loginBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal login body: %w", err)
		}
		bodyReader = strings.NewReader(string(bodyBytes))
		if s.logger != nil {
			s.logger.Debugf("Login request body prepared (JSON)")
		}
	} else if formBody, ok := config["loginFormBody"].(map[string]interface{}); ok {
		formData := url.Values{}
		for k, v := range formBody {
			formData.Set(k, fmt.Sprintf("%v", v))
		}
		bodyReader = strings.NewReader(formData.Encode())
		contentType = "application/x-www-form-urlencoded"
		if s.logger != nil {
			s.logger.Debugf("Login request body prepared (form-urlencoded)")
		}
	}

	if s.logger != nil {
		s.logger.Debugf("Session JWT login: %s %s", loginMethod, loginURL)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, loginMethod, loginURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create login request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send login request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if s.logger != nil {
		s.logger.Debugf("Login response status: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read login response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if s.logger != nil {
			s.logger.Errorf("Login request failed: status=%d, body=%s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("login request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var responseData map[string]interface{}
	if err := json.Unmarshal(body, &responseData); err != nil {
		return nil, fmt.Errorf("failed to parse login response: %w", err)
	}

	// Extract token using path
	token, err := s.extractValueByPath(responseData, tokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract token from response: %w", err)
	}

	tokenStr, ok := token.(string)
	if !ok {
		return nil, fmt.Errorf("token at path '%s' is not a string", tokenPath)
	}

	if s.logger != nil {
		s.logger.Infof("Successfully extracted token from login response")
	}

	// Build TokenInfo
	tokenInfo := &TokenInfo{
		AccessToken: tokenStr,
		TokenType:   "Bearer",
		Metadata:    make(map[string]string),
	}

	// Override token type if specified
	if tt, ok := config["tokenType"].(string); ok && tt != "" {
		tokenInfo.TokenType = tt
	}

	// Store token location info in metadata for ApplyAuth
	if tokenLocation, ok := config["tokenLocation"].(string); ok {
		tokenInfo.Metadata["tokenLocation"] = tokenLocation
	}
	if headerName, ok := config["headerName"].(string); ok {
		tokenInfo.Metadata["headerName"] = headerName
	}
	if headerFormat, ok := config["headerFormat"].(string); ok {
		tokenInfo.Metadata["headerFormat"] = headerFormat
	}
	if cookieName, ok := config["cookieName"].(string); ok {
		tokenInfo.Metadata["cookieName"] = cookieName
	}
	if cookieFormat, ok := config["cookieFormat"].(string); ok {
		tokenInfo.Metadata["cookieFormat"] = cookieFormat
	}
	if queryParam, ok := config["queryParam"].(string); ok {
		tokenInfo.Metadata["queryParam"] = queryParam
	}

	// Handle expiration
	if expiresInPath, ok := config["expiresInPath"].(string); ok && expiresInPath != "" {
		if expiresInVal, err := s.extractValueByPath(responseData, expiresInPath); err == nil {
			if expiresIn, ok := expiresInVal.(float64); ok {
				expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
				tokenInfo.ExpiresAt = &expiresAt
			}
		}
	} else if expiresIn, ok := config["expiresIn"].(float64); ok && expiresIn > 0 {
		expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
		tokenInfo.ExpiresAt = &expiresAt
	}

	// Extract refresh token if configured
	if refreshTokenPath, ok := config["refreshTokenPath"].(string); ok && refreshTokenPath != "" {
		if refreshToken, err := s.extractValueByPath(responseData, refreshTokenPath); err == nil {
			if rt, ok := refreshToken.(string); ok {
				tokenInfo.RefreshToken = rt
				if s.logger != nil {
					s.logger.Debugf("Refresh token extracted from response body")
				}
			}
		}
	}

	// Check for refresh token in cookies
	if refreshTokenLocation, ok := config["refreshTokenLocation"].(string); ok && refreshTokenLocation == "cookie" {
		cookieName := "refreshToken"
		if rtn, ok := config["refreshTokenCookieName"].(string); ok && rtn != "" {
			cookieName = rtn
		}
		for _, cookie := range resp.Cookies() {
			if cookie.Name == cookieName {
				tokenInfo.RefreshToken = cookie.Value
				if s.logger != nil {
					s.logger.Debugf("Refresh token extracted from cookie: %s", cookieName)
				}
				break
			}
		}
	}

	if s.logger != nil {
		expiryInfo := "no expiry"
		if tokenInfo.ExpiresAt != nil {
			expiryInfo = fmt.Sprintf("expires at %s", tokenInfo.ExpiresAt.Format(time.RFC3339))
		}
		s.logger.Infof("Session JWT authentication successful (%s, has_refresh=%v)",
			expiryInfo, tokenInfo.HasRefreshToken())
	}

	return tokenInfo, nil
}

func (s *SessionJWTStrategy) RefreshToken(ctx context.Context, tokenInfo *TokenInfo, config map[string]interface{}) (*TokenInfo, error) {
	refreshURL, ok := config["refreshURL"].(string)
	if !ok || refreshURL == "" {
		return nil, fmt.Errorf("refreshURL not configured for session_jwt auth")
	}

	baseURL, _ := config["baseURL"].(string)
	if baseURL != "" {
		if !strings.HasPrefix(refreshURL, "http://") && !strings.HasPrefix(refreshURL, "https://") {
			refreshURL = strings.TrimSuffix(baseURL, "/") + "/" + strings.TrimPrefix(refreshURL, "/")
		}
	}

	refreshMethod := "POST"
	if m, ok := config["refreshMethod"].(string); ok && m != "" {
		refreshMethod = strings.ToUpper(m)
	}

	if s.logger != nil {
		s.logger.Debugf("Refreshing session JWT token: %s %s", refreshMethod, refreshURL)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, refreshMethod, refreshURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	// Add refresh token as cookie if that's where it came from
	if refreshTokenLocation, ok := config["refreshTokenLocation"].(string); ok && refreshTokenLocation == "cookie" {
		cookieName := "refreshToken"
		if rtn, ok := config["refreshTokenCookieName"].(string); ok && rtn != "" {
			cookieName = rtn
		}
		req.AddCookie(&http.Cookie{
			Name:  cookieName,
			Value: tokenInfo.RefreshToken,
		})
		if s.logger != nil {
			s.logger.Debugf("Added refresh token cookie: %s", cookieName)
		}
	}

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send refresh request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if s.logger != nil {
		s.logger.Debugf("Refresh response status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if s.logger != nil {
			s.logger.Errorf("Refresh request failed: status=%d, body=%s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("refresh request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var responseData map[string]interface{}
	if err := json.Unmarshal(body, &responseData); err != nil {
		return nil, fmt.Errorf("failed to parse refresh response: %w", err)
	}

	// Extract new token
	tokenPath, _ := config["tokenPath"].(string)
	token, err := s.extractValueByPath(responseData, tokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract token from refresh response: %w", err)
	}

	tokenStr, ok := token.(string)
	if !ok {
		return nil, fmt.Errorf("token at path '%s' is not a string", tokenPath)
	}

	// Build new TokenInfo, preserving metadata from original
	newTokenInfo := &TokenInfo{
		AccessToken:  tokenStr,
		TokenType:    tokenInfo.TokenType,
		RefreshToken: tokenInfo.RefreshToken, // Keep old refresh token
		Metadata:     tokenInfo.Metadata,
	}

	// Check for new refresh token in response
	if refreshTokenPath, ok := config["refreshTokenPath"].(string); ok && refreshTokenPath != "" {
		if refreshToken, err := s.extractValueByPath(responseData, refreshTokenPath); err == nil {
			if rt, ok := refreshToken.(string); ok && rt != "" {
				newTokenInfo.RefreshToken = rt
			}
		}
	}

	// Check for new refresh token in cookies
	if refreshTokenLocation, ok := config["refreshTokenLocation"].(string); ok && refreshTokenLocation == "cookie" {
		cookieName := "refreshToken"
		if rtn, ok := config["refreshTokenCookieName"].(string); ok && rtn != "" {
			cookieName = rtn
		}
		for _, cookie := range resp.Cookies() {
			if cookie.Name == cookieName {
				newTokenInfo.RefreshToken = cookie.Value
				break
			}
		}
	}

	// Handle expiration
	if expiresInPath, ok := config["expiresInPath"].(string); ok && expiresInPath != "" {
		if expiresInVal, err := s.extractValueByPath(responseData, expiresInPath); err == nil {
			if expiresIn, ok := expiresInVal.(float64); ok {
				expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
				newTokenInfo.ExpiresAt = &expiresAt
			}
		}
	} else if expiresIn, ok := config["expiresIn"].(float64); ok && expiresIn > 0 {
		expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
		newTokenInfo.ExpiresAt = &expiresAt
	}

	if s.logger != nil {
		expiryInfo := "no expiry"
		if newTokenInfo.ExpiresAt != nil {
			expiryInfo = fmt.Sprintf("expires at %s", newTokenInfo.ExpiresAt.Format(time.RFC3339))
		}
		s.logger.Infof("Session JWT token refreshed successfully (%s)", expiryInfo)
	}

	return newTokenInfo, nil
}

func (s *SessionJWTStrategy) ApplyAuth(req *http.Request, tokenInfo *TokenInfo, _ map[string]interface{}) error {
	if tokenInfo == nil {
		return fmt.Errorf("token info is nil")
	}

	tokenLocation := "header" // default
	if loc, ok := tokenInfo.Metadata["tokenLocation"]; ok && loc != "" {
		tokenLocation = loc
	}

	switch tokenLocation {
	case "header":
		headerName := "Authorization"
		if hn, ok := tokenInfo.Metadata["headerName"]; ok && hn != "" {
			headerName = hn
		}

		headerFormat := "{tokenType} {token}"
		if hf, ok := tokenInfo.Metadata["headerFormat"]; ok && hf != "" {
			headerFormat = hf
		}

		// Apply format
		headerValue := headerFormat
		headerValue = strings.ReplaceAll(headerValue, "{tokenType}", tokenInfo.TokenType)
		headerValue = strings.ReplaceAll(headerValue, "{token}", tokenInfo.AccessToken)

		req.Header.Set(headerName, headerValue)
		if s.logger != nil {
			s.logger.Debugf("Applied session JWT token as header: %s", headerName)
		}

	case "cookie":
		cookieName := "token"
		if cn, ok := tokenInfo.Metadata["cookieName"]; ok && cn != "" {
			cookieName = cn
		}

		cookieFormat := "{token}"
		if cf, ok := tokenInfo.Metadata["cookieFormat"]; ok && cf != "" {
			cookieFormat = cf
		}

		// Apply format
		cookieValue := cookieFormat
		cookieValue = strings.ReplaceAll(cookieValue, "{tokenType}", tokenInfo.TokenType)
		cookieValue = strings.ReplaceAll(cookieValue, "{token}", tokenInfo.AccessToken)

		req.AddCookie(&http.Cookie{
			Name:  cookieName,
			Value: cookieValue,
		})
		if s.logger != nil {
			s.logger.Debugf("Applied session JWT token as cookie: %s", cookieName)
		}

	case "query":
		queryParam := "token"
		if qp, ok := tokenInfo.Metadata["queryParam"]; ok && qp != "" {
			queryParam = qp
		}

		q := req.URL.Query()
		q.Set(queryParam, tokenInfo.AccessToken)
		req.URL.RawQuery = q.Encode()
		if s.logger != nil {
			s.logger.Debugf("Applied session JWT token as query parameter: %s", queryParam)
		}

	default:
		return fmt.Errorf("unsupported token location: %s", tokenLocation)
	}

	return nil
}

// ============================================================================
// User Credentials Strategy
// ============================================================================

// UserCredentialsStrategy implements user-provided credential authentication
// This strategy supports services that require per-user credentials (e.g., API key + token)
// injected as query parameters, headers, or cookies. The credentials are stored in
// TokenInfo.Metadata and applied per the field definitions in the auth config.
type UserCredentialsStrategy struct {
	logger global.Logger
}

// NewUserCredentialsStrategy creates a new user credentials strategy
func NewUserCredentialsStrategy(logger global.Logger) *UserCredentialsStrategy {
	return &UserCredentialsStrategy{logger: logger}
}

func (s *UserCredentialsStrategy) GetAuthType() AuthType {
	return AuthTypeUserCredentials
}

func (s *UserCredentialsStrategy) SupportsRefresh() bool {
	return false
}

func (s *UserCredentialsStrategy) Authenticate(_ context.Context, _ map[string]interface{}) (*TokenInfo, error) {
	return nil, fmt.Errorf("user_credentials authentication requires running fusion-auth to provide credentials")
}

func (s *UserCredentialsStrategy) RefreshToken(_ context.Context, _ *TokenInfo, _ map[string]interface{}) (*TokenInfo, error) {
	return nil, fmt.Errorf("user_credentials does not support token refresh")
}

func (s *UserCredentialsStrategy) ApplyAuth(req *http.Request, tokenInfo *TokenInfo, config map[string]interface{}) error {
	if tokenInfo == nil {
		return fmt.Errorf("token info is nil")
	}

	if config == nil {
		return fmt.Errorf("auth config is required for user_credentials")
	}

	// Parse fields from config
	fieldsRaw, ok := config["fields"]
	if !ok {
		return fmt.Errorf("user_credentials config missing 'fields'")
	}

	fields, ok := fieldsRaw.([]interface{})
	if !ok {
		return fmt.Errorf("user_credentials 'fields' must be an array")
	}

	q := req.URL.Query()

	for _, fieldRaw := range fields {
		field, ok := fieldRaw.(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := field["name"].(string)
		location, _ := field["location"].(string)
		paramName, _ := field["paramName"].(string)
		if paramName == "" {
			paramName = name
		}

		// Look up the value in tokenInfo.Metadata
		value, exists := tokenInfo.Metadata[name]
		if !exists || value == "" {
			return fmt.Errorf("missing credential value for field '%s'", name)
		}

		// NOTE: Only log field/param names here, never credential values.
		switch location {
		case "query":
			q.Set(paramName, value)
			if s.logger != nil {
				s.logger.Debugf("Applied user credential '%s' as query parameter '%s'", name, paramName)
			}
		case "header":
			req.Header.Set(paramName, value)
			if s.logger != nil {
				s.logger.Debugf("Applied user credential '%s' as header '%s'", name, paramName)
			}
		case "cookie":
			req.AddCookie(&http.Cookie{
				Name:  paramName,
				Value: value,
			})
			if s.logger != nil {
				s.logger.Debugf("Applied user credential '%s' as cookie '%s'", name, paramName)
			}
		default:
			return fmt.Errorf("unsupported credential location '%s' for field '%s'", location, name)
		}
	}

	req.URL.RawQuery = q.Encode()
	return nil
}

// extractValueByPath extracts a value from a nested map using dot notation
// e.g., "datas.token" extracts responseData["datas"]["token"]
func (s *SessionJWTStrategy) extractValueByPath(data map[string]interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("key '%s' not found in path '%s'", part, path)
			}
			current = val
		default:
			return nil, fmt.Errorf("cannot navigate into non-object at '%s' in path '%s'", part, path)
		}
	}

	return current, nil
}
