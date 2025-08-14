/*=============================================================================
= Copyright (c) 2025 Tenebris Technologies Inc.                              =
= All rights reserved.                                                       =
=============================================================================*/

package fusion

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
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

// AuthManager manages authentication for multiple services
type AuthManager struct {
	strategies map[AuthType]AuthStrategy
	tokens     map[string]*TokenInfo // key: service name
	cache      Cache
	logger     global.Logger
	mu         sync.RWMutex
}

// NewAuthManager creates a new AuthManager
func NewAuthManager(cache Cache, logger global.Logger) *AuthManager {
	return &AuthManager{
		strategies: make(map[AuthType]AuthStrategy),
		tokens:     make(map[string]*TokenInfo),
		cache:      cache,
		logger:     logger,
	}
}

// RegisterStrategy registers an authentication strategy
func (am *AuthManager) RegisterStrategy(strategy AuthStrategy) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.strategies[strategy.GetAuthType()] = strategy
	if am.logger != nil {
		am.logger.Infof("Registered auth strategy: %s", strategy.GetAuthType())
	}
}

// GetToken gets a valid token for a service, performing authentication if necessary
func (am *AuthManager) GetToken(ctx context.Context, serviceName string, authConfig AuthConfig) (*TokenInfo, error) {
	if am.logger != nil {
		am.logger.Debugf("Getting token for service %s (auth type: %s)", serviceName, authConfig.Type)
	}

	am.mu.RLock()
	strategy, exists := am.strategies[authConfig.Type]
	am.mu.RUnlock()

	if !exists {
		if am.logger != nil {
			am.logger.Errorf("Unsupported authentication type for service %s: %s", serviceName, authConfig.Type)
		}
		return nil, NewAuthenticationError(authConfig.Type, serviceName,
			"unsupported authentication type", nil)
	}

	// Check if we have a cached token
	if am.logger != nil {
		am.logger.Debugf("Checking cached token for service: %s", serviceName)
	}
	if tokenInfo := am.getCachedToken(serviceName); tokenInfo != nil {
		if am.logger != nil {
			am.logger.Debugf("Found cached token for service %s", serviceName)
		}

		// Check if token is expired (with 5-minute buffer)
		if !tokenInfo.IsExpiredWithBuffer(5 * time.Minute) {
			if am.logger != nil {
				expiryInfo := "no expiry"
				if tokenInfo.ExpiresAt != nil {
					expiryInfo = fmt.Sprintf("expires at %s", tokenInfo.ExpiresAt.Format(time.RFC3339))
				}
				am.logger.Debugf("Using valid cached token for service %s (%s)", serviceName, expiryInfo)
			}
			return tokenInfo, nil
		}

		if am.logger != nil {
			am.logger.Debugf("Cached token for service %s is expired, attempting refresh", serviceName)
		}

		// Try to refresh if supported and we have a refresh token
		if strategy.SupportsRefresh() && tokenInfo.HasRefreshToken() {
			if am.logger != nil {
				am.logger.Debugf("Attempting to refresh token for service: %s", serviceName)
			}
			if refreshedToken, err := strategy.RefreshToken(ctx, tokenInfo); err == nil {
				am.cacheToken(serviceName, refreshedToken)
				if am.logger != nil {
					am.logger.Infof("Successfully refreshed token for service: %s", serviceName)
				}
				return refreshedToken, nil
			} else {
				if am.logger != nil {
					am.logger.Warningf("Failed to refresh token for service %s: %v", serviceName, err)
				}
			}
		} else {
			if am.logger != nil {
				if !strategy.SupportsRefresh() {
					am.logger.Debugf("Token refresh not supported for auth type %s", authConfig.Type)
				} else {
					am.logger.Debugf("No refresh token available for service %s", serviceName)
				}
			}
		}
	} else {
		if am.logger != nil {
			am.logger.Debugf("No cached token found for service: %s", serviceName)
		}
	}

	// Perform new authentication
	if am.logger != nil {
		am.logger.Infof("Performing new authentication for service %s using %s", serviceName, authConfig.Type)
	}

	tokenInfo, err := strategy.Authenticate(ctx, authConfig.Config)
	if err != nil {
		// Check if it's a DeviceCodeError - don't wrap it
		if _, ok := err.(*DeviceCodeError); ok {
			if am.logger != nil {
				am.logger.Infof("Device code authentication required for service %s", serviceName)
			}
			return nil, err // Return DeviceCodeError directly
		}

		if am.logger != nil {
			am.logger.Errorf("Authentication failed for service %s: %v", serviceName, err)
		}
		return nil, NewAuthenticationError(authConfig.Type, serviceName,
			"authentication failed", err)
	}

	// Cache the new token
	am.cacheToken(serviceName, tokenInfo)

	if am.logger != nil {
		expiryInfo := "no expiry"
		if tokenInfo.ExpiresAt != nil {
			expiryInfo = fmt.Sprintf("expires at %s", tokenInfo.ExpiresAt.Format(time.RFC3339))
		}
		am.logger.Infof("Successfully authenticated service %s (%s)", serviceName, expiryInfo)
	}

	return tokenInfo, nil
}

// ApplyAuthentication applies authentication to an HTTP request
func (am *AuthManager) ApplyAuthentication(ctx context.Context, req *http.Request,
	serviceName string, authConfig AuthConfig) error {

	if am.logger != nil {
		am.logger.Debugf("Applying authentication for service %s to %s %s", serviceName, req.Method, req.URL.String())
	}

	tokenInfo, err := am.GetToken(ctx, serviceName, authConfig)
	if err != nil {
		if am.logger != nil {
			am.logger.Errorf("Failed to get token for service %s: %v", serviceName, err)
		}
		return err
	}

	am.mu.RLock()
	strategy, exists := am.strategies[authConfig.Type]
	am.mu.RUnlock()

	if !exists {
		if am.logger != nil {
			am.logger.Errorf("Strategy not found for auth type %s on service %s", authConfig.Type, serviceName)
		}
		return NewAuthenticationError(authConfig.Type, serviceName,
			"strategy not found", nil)
	}

	if am.logger != nil {
		am.logger.Debugf("Applying %s authentication to request for service %s", authConfig.Type, serviceName)
	}

	if err := strategy.ApplyAuth(req, tokenInfo); err != nil {
		if am.logger != nil {
			am.logger.Errorf("Failed to apply authentication for service %s: %v", serviceName, err)
		}
		return err
	}

	if am.logger != nil {
		am.logger.Debugf("Successfully applied authentication for service %s", serviceName)
	}

	return nil
}

// InvalidateToken removes a token from cache
func (am *AuthManager) InvalidateToken(serviceName string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	delete(am.tokens, serviceName)

	if am.cache != nil {
		cacheKey := "token:" + serviceName
		if err := am.cache.Delete(cacheKey); err != nil && am.logger != nil {
			am.logger.Warningf("Failed to delete token from cache for service %s: %v", serviceName, err)
		}
	}

	if am.logger != nil {
		am.logger.Infof("Invalidated token for service: %s", serviceName)
	}
}

// getCachedToken retrieves a token from cache
func (am *AuthManager) getCachedToken(serviceName string) *TokenInfo {
	am.mu.RLock()
	defer am.mu.RUnlock()

	// Check in-memory cache first
	if tokenInfo, exists := am.tokens[serviceName]; exists {
		if am.logger != nil {
			am.logger.Debugf("Found token in memory cache for service: %s", serviceName)
		}
		return tokenInfo
	}

	// Check persistent cache if available
	if am.cache != nil {
		cacheKey := "token:" + serviceName
		if am.logger != nil {
			am.logger.Debugf("Checking persistent cache for service: %s", serviceName)
		}
		if data, err := am.cache.Get(cacheKey); err == nil {
			if tokenInfo, ok := data.(*TokenInfo); ok {
				if am.logger != nil {
					am.logger.Debugf("Found token in persistent cache for service: %s", serviceName)
				}
				am.tokens[serviceName] = tokenInfo
				return tokenInfo
			} else {
				if am.logger != nil {
					am.logger.Warningf("Invalid token data in cache for service %s", serviceName)
				}
			}
		} else {
			if am.logger != nil {
				am.logger.Debugf("No token found in persistent cache for service %s: %v", serviceName, err)
			}
		}
	}

	if am.logger != nil {
		am.logger.Debugf("No cached token found for service: %s", serviceName)
	}

	return nil
}

// cacheToken stores a token in cache
func (am *AuthManager) cacheToken(serviceName string, tokenInfo *TokenInfo) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.logger != nil {
		expiryInfo := "no expiry"
		if tokenInfo.ExpiresAt != nil {
			expiryInfo = fmt.Sprintf("expires at %s", tokenInfo.ExpiresAt.Format(time.RFC3339))
		}
		am.logger.Debugf("Caching token for service %s (%s)", serviceName, expiryInfo)
	}

	// Store in memory
	am.tokens[serviceName] = tokenInfo

	// Store in persistent cache if available
	if am.cache != nil {
		cacheKey := "token:" + serviceName
		var ttl time.Duration
		if tokenInfo.ExpiresAt != nil {
			ttl = time.Until(*tokenInfo.ExpiresAt)
			if am.logger != nil {
				am.logger.Debugf("Setting cache TTL for service %s: %v", serviceName, ttl)
			}
		} else {
			ttl = 24 * time.Hour // Default TTL if no expiration
			if am.logger != nil {
				am.logger.Debugf("Using default cache TTL for service %s: %v", serviceName, ttl)
			}
		}

		if err := am.cache.Set(cacheKey, tokenInfo, ttl); err != nil {
			if am.logger != nil {
				am.logger.Warningf("Failed to cache token for service %s: %v", serviceName, err)
			}
		} else {
			if am.logger != nil {
				am.logger.Debugf("Successfully cached token for service %s", serviceName)
			}
		}
	} else {
		if am.logger != nil {
			am.logger.Debugf("No persistent cache available, token stored in memory only for service: %s", serviceName)
		}
	}
}

// GetRegisteredStrategies returns a list of registered authentication types
func (am *AuthManager) GetRegisteredStrategies() []AuthType {
	am.mu.RLock()
	defer am.mu.RUnlock()

	types := make([]AuthType, 0, len(am.strategies))
	for authType := range am.strategies {
		types = append(types, authType)
	}
	return types
}

// HasStrategy checks if a strategy is registered for the given auth type
func (am *AuthManager) HasStrategy(authType AuthType) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()

	_, exists := am.strategies[authType]
	return exists
}

// PendingDeviceCode tracks pending device code authentication
type PendingDeviceCode struct {
	DeviceCodeErr *DeviceCodeError
	CreatedAt     time.Time
	ExpiresAt     time.Time
}

// IsExpired checks if the pending device code has expired
func (pdc *PendingDeviceCode) IsExpired() bool {
	return time.Now().After(pdc.ExpiresAt)
}

// OAuth2DeviceFlowStrategy implements OAuth2 device flow authentication
type OAuth2DeviceFlowStrategy struct {
	httpClient         *http.Client
	logger             global.Logger
	pendingDeviceCodes map[string]*PendingDeviceCode // key: serviceName
	mu                 sync.RWMutex
}

// NewOAuth2DeviceFlowStrategy creates a new OAuth2 device flow strategy
func NewOAuth2DeviceFlowStrategy(httpClient *http.Client, logger global.Logger) *OAuth2DeviceFlowStrategy {
	return &OAuth2DeviceFlowStrategy{
		httpClient:         httpClient,
		logger:             logger,
		pendingDeviceCodes: make(map[string]*PendingDeviceCode),
	}
}

// GetAuthType returns the authentication type
func (s *OAuth2DeviceFlowStrategy) GetAuthType() AuthType {
	return AuthTypeOAuth2Device
}

// SupportsRefresh returns true as OAuth2 supports refresh tokens
func (s *OAuth2DeviceFlowStrategy) SupportsRefresh() bool {
	return true
}

// Authenticate performs OAuth2 device flow authentication with non-blocking two-phase flow
func (s *OAuth2DeviceFlowStrategy) Authenticate(ctx context.Context, config map[string]interface{}) (*TokenInfo, error) {
	if s.logger != nil {
		s.logger.Debug("Starting OAuth2 device flow authentication")
	}

	// Extract required configuration
	clientID, ok := config["clientId"].(string)
	if !ok || clientID == "" {
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			"clientId is required for OAuth2 device flow", nil)
	}

	authorizationURL, ok := config["authorizationURL"].(string)
	if !ok || authorizationURL == "" {
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			"authorizationURL is required for OAuth2 device flow", nil)
	}

	tokenURL, ok := config["tokenURL"].(string)
	if !ok || tokenURL == "" {
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			"tokenURL is required for OAuth2 device flow", nil)
	}

	// Handle optional parameters
	tenantID, _ := config["tenantId"].(string)
	scopeInterface, _ := config["scope"]

	var scopes []string
	switch v := scopeInterface.(type) {
	case []string:
		scopes = v
	case []interface{}:
		for _, s := range v {
			if str, ok := s.(string); ok {
				scopes = append(scopes, str)
			}
		}
	}

	// Replace {tenantId} in URLs if provided
	if tenantID != "" {
		authorizationURL = replaceURLPlaceholder(authorizationURL, "tenantId", tenantID)
		tokenURL = replaceURLPlaceholder(tokenURL, "tenantId", tenantID)
	}

	// Use clientID as key to track pending device codes (should be unique per service)
	cacheKey := clientID

	// Phase 1: Check for pending device code authentication
	s.mu.RLock()
	pendingCode, hasPending := s.pendingDeviceCodes[cacheKey]
	s.mu.RUnlock()

	if hasPending && !pendingCode.IsExpired() {
		// Phase 2: We have a pending device code, try one poll attempt
		if s.logger != nil {
			s.logger.Debug("Found pending device code, checking authentication status")
		}

		// Try to exchange the device code for tokens
		tokenInfo, err := s.exchangeDeviceCode(ctx, pendingCode.DeviceCodeErr)
		if err == nil {
			// Success! Clean up pending code and return token
			s.cleanupPendingCode(cacheKey)
			if s.logger != nil {
				s.logger.Info("Device flow authentication completed successfully")
			}
			return tokenInfo, nil
		}

		// Check if it's still authorization pending
		if authErr, ok := err.(*AuthenticationError); ok {
			if authErr.OriginalError != nil && authErr.OriginalError.Error() == "authorization_pending" {
				// Still pending, return the device code error again for client to display
				if s.logger != nil {
					s.logger.Debug("Authorization still pending, returning device code error")
				}
				return nil, pendingCode.DeviceCodeErr
			}
		}

		// Other error occurred, clean up and let it fall through to request new code
		if s.logger != nil {
			s.logger.Warningf("Device code exchange failed, will request new code: %v", err)
		}
		s.cleanupPendingCode(cacheKey)
	} else if hasPending {
		// Pending code is expired, clean it up
		if s.logger != nil {
			s.logger.Debug("Pending device code expired, requesting new code")
		}
		s.cleanupPendingCode(cacheKey)
	}

	// Phase 1: Request new device code
	if s.logger != nil {
		s.logger.Debug("Requesting new device code")
	}

	deviceCode, err := s.requestDeviceCode(ctx, authorizationURL, clientID, scopes)
	if err != nil {
		return nil, err
	}

	// Create device code error struct
	deviceCodeErr := &DeviceCodeError{
		VerificationURL: deviceCode.VerificationURI,
		UserCode:        deviceCode.UserCode,
		Message:         deviceCode.Message,
		DeviceCode:      deviceCode.DeviceCode,
		Interval:        deviceCode.Interval,
		ExpiresIn:       deviceCode.ExpiresIn,
		ClientID:        clientID,
		TokenURL:        tokenURL,
		Scopes:          scopes,
	}

	// Cache the device code information
	pendingDeviceCode := &PendingDeviceCode{
		DeviceCodeErr: deviceCodeErr,
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second),
	}

	s.mu.Lock()
	s.pendingDeviceCodes[cacheKey] = pendingDeviceCode
	s.mu.Unlock()

	if s.logger != nil {
		s.logger.Infof("Device code authentication initiated. Please visit %s and enter code: %s",
			deviceCode.VerificationURI, deviceCode.UserCode)
	}

	// Return device code error immediately so client sees it and can retry
	return nil, deviceCodeErr
}

// requestDeviceCode requests a device code from the authorization server
func (s *OAuth2DeviceFlowStrategy) requestDeviceCode(ctx context.Context, authURL string, clientID string, scopes []string) (*deviceCodeResponse, error) {
	if s.logger != nil {
		s.logger.Debugf("Requesting device code from: %s", authURL)
	}

	data := make(map[string][]string)
	data["client_id"] = []string{clientID}
	if len(scopes) > 0 {
		data["scope"] = []string{joinScopes(scopes)}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", authURL, strings.NewReader(encodeFormData(data)))
	if err != nil {
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			"failed to create device code request", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			"failed to request device code", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if s.logger != nil {
			s.logger.Errorf("Device code request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			fmt.Sprintf("device code request failed with status %d", resp.StatusCode), nil)
	}

	var deviceCode deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceCode); err != nil {
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			"failed to parse device code response", err)
	}

	if s.logger != nil {
		s.logger.Infof("Device code received. User code: %s", deviceCode.UserCode)
	}

	return &deviceCode, nil
}

// PollForToken polls the token endpoint until the user completes authentication
func (s *OAuth2DeviceFlowStrategy) PollForToken(ctx context.Context, deviceCodeErr *DeviceCodeError) (*TokenInfo, error) {
	if s.logger != nil {
		s.logger.Debug("Starting to poll for token")
	}

	interval := deviceCodeErr.Interval
	if interval == 0 {
		interval = 5 // Default to 5 seconds
	}

	expiry := time.Now().Add(time.Duration(deviceCodeErr.ExpiresIn) * time.Second)
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
				"polling cancelled", ctx.Err())
		case <-ticker.C:
			if time.Now().After(expiry) {
				return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
					"device code expired", nil)
			}

			tokenInfo, err := s.exchangeDeviceCode(ctx, deviceCodeErr)
			if err == nil {
				return tokenInfo, nil
			}

			// Check if it's an authorization pending error
			if authErr, ok := err.(*AuthenticationError); ok {
				if authErr.OriginalError != nil && authErr.OriginalError.Error() == "authorization_pending" {
					// Continue polling
					if s.logger != nil {
						s.logger.Debug("Authorization pending, continuing to poll")
					}
					continue
				}
			}

			// Any other error stops polling
			return nil, err
		}
	}
}

// exchangeDeviceCode exchanges a device code for an access token
func (s *OAuth2DeviceFlowStrategy) exchangeDeviceCode(ctx context.Context, deviceCodeErr *DeviceCodeError) (*TokenInfo, error) {
	data := make(map[string][]string)
	data["client_id"] = []string{deviceCodeErr.ClientID}
	data["device_code"] = []string{deviceCodeErr.DeviceCode}
	data["grant_type"] = []string{"urn:ietf:params:oauth:grant-type:device_code"}

	req, err := http.NewRequestWithContext(ctx, "POST", deviceCodeErr.TokenURL, strings.NewReader(encodeFormData(data)))
	if err != nil {
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			"failed to create token request", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			"failed to exchange device code", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			"failed to read token response", err)
	}

	// Check for authorization_pending error
	if resp.StatusCode == http.StatusBadRequest {
		var errorResp struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error == "authorization_pending" {
			return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
				"authorization pending", fmt.Errorf("authorization_pending"))
		}
	}

	if resp.StatusCode != http.StatusOK {
		if s.logger != nil {
			s.logger.Errorf("Token exchange failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			fmt.Sprintf("token exchange failed with status %d", resp.StatusCode), nil)
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			"failed to parse token response", err)
	}

	tokenInfo := &TokenInfo{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: tokenResp.RefreshToken,
		Scope:        splitScopes(tokenResp.Scope),
		Metadata: map[string]string{
			"clientID": deviceCodeErr.ClientID,
			"tokenURL": deviceCodeErr.TokenURL,
		},
	}

	if tokenResp.ExpiresIn > 0 {
		expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		tokenInfo.ExpiresAt = &expiresAt
	}

	if s.logger != nil {
		s.logger.Info("Successfully obtained access token via device flow")
	}

	return tokenInfo, nil
}

// RefreshToken refreshes an OAuth2 token
func (s *OAuth2DeviceFlowStrategy) RefreshToken(ctx context.Context, tokenInfo *TokenInfo) (*TokenInfo, error) {
	if s.logger != nil {
		s.logger.Debug("Refreshing OAuth2 token")
	}

	if !tokenInfo.HasRefreshToken() {
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			"no refresh token available", nil)
	}

	// Get the token URL from metadata (should be stored during initial auth)
	tokenURL, ok := tokenInfo.Metadata["tokenURL"]
	if !ok || tokenURL == "" {
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			"token URL not found in token metadata", nil)
	}

	clientID, ok := tokenInfo.Metadata["clientID"]
	if !ok || clientID == "" {
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			"client ID not found in token metadata", nil)
	}

	data := make(map[string][]string)
	data["client_id"] = []string{clientID}
	data["refresh_token"] = []string{tokenInfo.RefreshToken}
	data["grant_type"] = []string{"refresh_token"}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(encodeFormData(data)))
	if err != nil {
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			"failed to create refresh request", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			"failed to refresh token", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if s.logger != nil {
			s.logger.Errorf("Token refresh failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			fmt.Sprintf("token refresh failed with status %d", resp.StatusCode), nil)
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, NewAuthenticationError(AuthTypeOAuth2Device, "",
			"failed to parse refresh response", err)
	}

	newTokenInfo := &TokenInfo{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: tokenResp.RefreshToken,
		Scope:        splitScopes(tokenResp.Scope),
		Metadata:     tokenInfo.Metadata, // Preserve metadata
	}

	// Use new refresh token if provided, otherwise keep the old one
	if newTokenInfo.RefreshToken == "" {
		newTokenInfo.RefreshToken = tokenInfo.RefreshToken
	}

	if tokenResp.ExpiresIn > 0 {
		expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		newTokenInfo.ExpiresAt = &expiresAt
	}

	if s.logger != nil {
		s.logger.Info("Successfully refreshed OAuth2 token")
	}

	return newTokenInfo, nil
}

// ApplyAuth applies OAuth2 authentication to a request
func (s *OAuth2DeviceFlowStrategy) ApplyAuth(req *http.Request, tokenInfo *TokenInfo) error {
	req.Header.Set("Authorization", tokenInfo.GetAuthorizationHeader())
	return nil
}

// cleanupPendingCode removes a pending device code from the cache
func (s *OAuth2DeviceFlowStrategy) cleanupPendingCode(cacheKey string) {
	s.mu.Lock()
	delete(s.pendingDeviceCodes, cacheKey)
	s.mu.Unlock()

	if s.logger != nil {
		s.logger.Debugf("Cleaned up pending device code for key: %s", cacheKey)
	}
}

// cleanupExpiredCodes removes expired pending device codes (should be called periodically)
func (s *OAuth2DeviceFlowStrategy) cleanupExpiredCodes() {
	s.mu.Lock()
	defer s.mu.Unlock()

	var expired []string
	for key, pending := range s.pendingDeviceCodes {
		if pending.IsExpired() {
			expired = append(expired, key)
		}
	}

	for _, key := range expired {
		delete(s.pendingDeviceCodes, key)
	}

	if len(expired) > 0 && s.logger != nil {
		s.logger.Debugf("Cleaned up %d expired device codes", len(expired))
	}
}

// BearerTokenStrategy implements bearer token authentication
type BearerTokenStrategy struct {
	logger global.Logger
}

// NewBearerTokenStrategy creates a new bearer token strategy
func NewBearerTokenStrategy(logger global.Logger) *BearerTokenStrategy {
	return &BearerTokenStrategy{
		logger: logger,
	}
}

// GetAuthType returns the authentication type
func (s *BearerTokenStrategy) GetAuthType() AuthType {
	return AuthTypeBearer
}

// SupportsRefresh returns false as bearer tokens typically don't refresh
func (s *BearerTokenStrategy) SupportsRefresh() bool {
	return false
}

// Authenticate creates a token info from bearer token configuration
func (s *BearerTokenStrategy) Authenticate(ctx context.Context, config map[string]interface{}) (*TokenInfo, error) {
	if s.logger != nil {
		s.logger.Debug("Authenticating with bearer token strategy")
	}

	var token string
	var tokenSource string

	// Try to get token directly
	if tokenValue, ok := config["token"]; ok {
		if tokenStr, ok := tokenValue.(string); ok && tokenStr != "" {
			token = tokenStr
			tokenSource = "direct configuration"
			if s.logger != nil {
				s.logger.Debug("Bearer token found in direct configuration")
			}
		}
	}

	// Try to get token from environment variable
	if token == "" {
		if envVarName, ok := config["tokenEnvVar"]; ok {
			if envVarStr, ok := envVarName.(string); ok && envVarStr != "" {
				if s.logger != nil {
					s.logger.Debugf("Looking for bearer token in environment variable: %s", envVarStr)
				}
				if envToken := getEnvVar(envVarStr); envToken != "" {
					token = envToken
					tokenSource = fmt.Sprintf("environment variable %s", envVarStr)
					if s.logger != nil {
						s.logger.Debugf("Bearer token found in environment variable: %s", envVarStr)
					}
				} else {
					if s.logger != nil {
						s.logger.Warningf("Environment variable %s is empty or not set", envVarStr)
					}
				}
			}
		}
	}

	if token == "" {
		if s.logger != nil {
			s.logger.Error("No bearer token found in config or environment")
		}
		return nil, NewAuthenticationError(AuthTypeBearer, "",
			"no bearer token found in config or environment", nil)
	}

	if s.logger != nil {
		s.logger.Infof("Bearer token authentication successful (source: %s)", tokenSource)
	}

	return &TokenInfo{
		AccessToken: token,
		TokenType:   "Bearer",
	}, nil
}

// RefreshToken is not supported for bearer tokens
func (s *BearerTokenStrategy) RefreshToken(ctx context.Context, tokenInfo *TokenInfo) (*TokenInfo, error) {
	return nil, NewAuthenticationError(AuthTypeBearer, "",
		"bearer token refresh not supported", nil)
}

// ApplyAuth applies bearer token authentication to a request
func (s *BearerTokenStrategy) ApplyAuth(req *http.Request, tokenInfo *TokenInfo) error {
	req.Header.Set("Authorization", tokenInfo.GetAuthorizationHeader())
	return nil
}

// APIKeyStrategy implements API key authentication
type APIKeyStrategy struct {
	logger global.Logger
}

// NewAPIKeyStrategy creates a new API key strategy
func NewAPIKeyStrategy(logger global.Logger) *APIKeyStrategy {
	return &APIKeyStrategy{
		logger: logger,
	}
}

// GetAuthType returns the authentication type
func (s *APIKeyStrategy) GetAuthType() AuthType {
	return AuthTypeAPIKey
}

// SupportsRefresh returns false as API keys typically don't refresh
func (s *APIKeyStrategy) SupportsRefresh() bool {
	return false
}

// Authenticate creates a token info from API key configuration
func (s *APIKeyStrategy) Authenticate(ctx context.Context, config map[string]interface{}) (*TokenInfo, error) {
	if s.logger != nil {
		s.logger.Debug("Authenticating with API key strategy")
	}

	var apiKey string
	var keySource string

	// Try to get API key directly
	if keyValue, ok := config["apiKey"]; ok {
		if keyStr, ok := keyValue.(string); ok && keyStr != "" {
			apiKey = keyStr
			keySource = "direct configuration"
			if s.logger != nil {
				s.logger.Debug("API key found in direct configuration")
			}
		}
	}

	// Try to get API key from environment variable
	if apiKey == "" {
		if envVarName, ok := config["apiKeyEnvVar"]; ok {
			if envVarStr, ok := envVarName.(string); ok && envVarStr != "" {
				if s.logger != nil {
					s.logger.Debugf("Looking for API key in environment variable: %s", envVarStr)
				}
				if envKey := getEnvVar(envVarStr); envKey != "" {
					apiKey = envKey
					keySource = fmt.Sprintf("environment variable %s", envVarStr)
					if s.logger != nil {
						s.logger.Debugf("API key found in environment variable: %s", envVarStr)
					}
				} else {
					if s.logger != nil {
						s.logger.Warningf("Environment variable %s is empty or not set", envVarStr)
					}
				}
			}
		}
	}

	if apiKey == "" {
		if s.logger != nil {
			s.logger.Error("No API key found in config or environment")
		}
		return nil, NewAuthenticationError(AuthTypeAPIKey, "",
			"no API key found in config or environment", nil)
	}

	// Store header name in metadata for later use
	metadata := make(map[string]string)
	if headerName, ok := config["headerName"]; ok {
		if headerStr, ok := headerName.(string); ok {
			metadata["headerName"] = headerStr
			if s.logger != nil {
				s.logger.Debugf("Using custom header name for API key: %s", headerStr)
			}
		}
	}
	if metadata["headerName"] == "" {
		metadata["headerName"] = "X-API-Key" // Default header name
		if s.logger != nil {
			s.logger.Debug("Using default header name for API key: X-API-Key")
		}
	}

	if s.logger != nil {
		s.logger.Infof("API key authentication successful (source: %s, header: %s)", keySource, metadata["headerName"])
	}

	return &TokenInfo{
		AccessToken: apiKey,
		TokenType:   "ApiKey",
		Metadata:    metadata,
	}, nil
}

// RefreshToken is not supported for API keys
func (s *APIKeyStrategy) RefreshToken(ctx context.Context, tokenInfo *TokenInfo) (*TokenInfo, error) {
	return nil, NewAuthenticationError(AuthTypeAPIKey, "",
		"API key refresh not supported", nil)
}

// ApplyAuth applies API key authentication to a request
func (s *APIKeyStrategy) ApplyAuth(req *http.Request, tokenInfo *TokenInfo) error {
	headerName := "X-API-Key"
	if tokenInfo.Metadata != nil {
		if name, ok := tokenInfo.Metadata["headerName"]; ok && name != "" {
			headerName = name
		}
	}

	req.Header.Set(headerName, tokenInfo.AccessToken)
	return nil
}

// BasicAuthStrategy implements basic authentication
type BasicAuthStrategy struct {
	logger global.Logger
}

// NewBasicAuthStrategy creates a new basic auth strategy
func NewBasicAuthStrategy(logger global.Logger) *BasicAuthStrategy {
	return &BasicAuthStrategy{
		logger: logger,
	}
}

// GetAuthType returns the authentication type
func (s *BasicAuthStrategy) GetAuthType() AuthType {
	return AuthTypeBasic
}

// SupportsRefresh returns false as basic auth doesn't use tokens
func (s *BasicAuthStrategy) SupportsRefresh() bool {
	return false
}

// Authenticate creates a token info from basic auth configuration
func (s *BasicAuthStrategy) Authenticate(ctx context.Context, config map[string]interface{}) (*TokenInfo, error) {
	if s.logger != nil {
		s.logger.Debug("Authenticating with basic auth strategy")
	}

	username, ok := config["username"].(string)
	if !ok || username == "" {
		if s.logger != nil {
			s.logger.Error("Username is required for basic auth but not provided")
		}
		return nil, NewAuthenticationError(AuthTypeBasic, "",
			"username is required for basic auth", nil)
	}

	password, ok := config["password"].(string)
	if !ok || password == "" {
		if s.logger != nil {
			s.logger.Error("Password is required for basic auth but not provided")
		}
		return nil, NewAuthenticationError(AuthTypeBasic, "",
			"password is required for basic auth", nil)
	}

	if s.logger != nil {
		s.logger.Debugf("Basic auth configured for username: %s", username)
	}

	// Store credentials in metadata
	metadata := map[string]string{
		"username": username,
		"password": password,
	}

	if s.logger != nil {
		s.logger.Infof("Basic auth authentication successful for username: %s", username)
	}

	return &TokenInfo{
		AccessToken: username + ":" + password, // This will be base64 encoded when applied
		TokenType:   "Basic",
		Metadata:    metadata,
	}, nil
}

// RefreshToken is not supported for basic auth
func (s *BasicAuthStrategy) RefreshToken(ctx context.Context, tokenInfo *TokenInfo) (*TokenInfo, error) {
	return nil, NewAuthenticationError(AuthTypeBasic, "",
		"basic auth refresh not supported", nil)
}

// ApplyAuth applies basic authentication to a request
func (s *BasicAuthStrategy) ApplyAuth(req *http.Request, tokenInfo *TokenInfo) error {
	if tokenInfo.Metadata == nil {
		return NewAuthenticationError(AuthTypeBasic, "",
			"basic auth credentials not found in token metadata", nil)
	}

	username, ok := tokenInfo.Metadata["username"]
	if !ok {
		return NewAuthenticationError(AuthTypeBasic, "",
			"username not found in token metadata", nil)
	}

	password, ok := tokenInfo.Metadata["password"]
	if !ok {
		return NewAuthenticationError(AuthTypeBasic, "",
			"password not found in token metadata", nil)
	}

	req.SetBasicAuth(username, password)
	return nil
}

// getEnvVar is a helper function to get environment variables
func getEnvVar(name string) string {
	return os.Getenv(name)
}

// deviceCodeResponse represents the response from a device code request
type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Message         string `json:"message,omitempty"`
}

// tokenResponse represents the response from a token request
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// replaceURLPlaceholder replaces placeholders in URLs
func replaceURLPlaceholder(urlStr, placeholder, value string) string {
	return strings.ReplaceAll(urlStr, "{"+placeholder+"}", value)
}

// joinScopes joins OAuth2 scopes into a space-separated string
func joinScopes(scopes []string) string {
	return strings.Join(scopes, " ")
}

// splitScopes splits a space-separated scope string into a slice
func splitScopes(scope string) []string {
	if scope == "" {
		return nil
	}
	return strings.Split(scope, " ")
}

// encodeFormData encodes form data for HTTP requests
func encodeFormData(data map[string][]string) string {
	values := url.Values(data)
	return values.Encode()
}
