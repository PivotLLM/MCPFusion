/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/global"
)

// NoAuthTenantHash is the special tenant hash used when authentication is disabled
// This is INSECURE and should only be used for testing purposes
const NoAuthTenantHash = "NOAUTH"

// TenantContext represents tenant-specific context information
type TenantContext struct {
	TenantHash  string            `json:"tenant_hash"`
	ServiceName string            `json:"service_name"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	RequestID   string            `json:"request_id,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// ShortHash returns a truncated version of the tenant hash for logging
// Safely handles hashes shorter than 12 characters (e.g., "NOAUTH")
func (tc *TenantContext) ShortHash() string {
	if tc == nil || tc.TenantHash == "" {
		return "unknown"
	}
	if len(tc.TenantHash) <= 12 {
		return tc.TenantHash
	}
	return tc.TenantHash[:12] + "..."
}

// String returns a string representation of the tenant context
func (tc *TenantContext) String() string {
	if tc == nil {
		return "TenantContext(nil)"
	}
	return fmt.Sprintf("TenantContext(tenant=%s, service=%s, request=%s)",
		tc.ShortHash(), tc.ServiceName, tc.RequestID)
}

// MultiTenantAuthManager manages authentication for multiple tenants
type MultiTenantAuthManager struct {
	db                *db.DB
	strategies        map[AuthType]AuthStrategy
	cache             Cache
	logger            global.Logger
	mu                sync.RWMutex
	invalidationLocks sync.Map // Per-tenant token invalidation locks (key: string, value: *sync.Mutex)
}

// NewMultiTenantAuthManager creates a new multi-tenant authentication manager
func NewMultiTenantAuthManager(database *db.DB, cache Cache, logger global.Logger) *MultiTenantAuthManager {
	return &MultiTenantAuthManager{
		db:         database,
		strategies: make(map[AuthType]AuthStrategy),
		cache:      cache,
		logger:     logger,
		// invalidationLocks is a sync.Map and doesn't need initialization
	}
}

// RegisterStrategy registers an authentication strategy
func (mtam *MultiTenantAuthManager) RegisterStrategy(strategy AuthStrategy) {
	mtam.mu.Lock()
	defer mtam.mu.Unlock()

	mtam.strategies[strategy.GetAuthType()] = strategy

	// Set auth manager reference for OAuth2 device flow strategies
	if oauth2Strategy, ok := strategy.(*OAuth2DeviceFlowStrategy); ok {
		oauth2Strategy.SetAuthManager(mtam)
	}

	if mtam.logger != nil {
		mtam.logger.Infof("Registered multi-tenant auth strategy: %s", strategy.GetAuthType())
	}
}

// GetToken gets a valid token for a tenant and service, performing authentication if necessary
func (mtam *MultiTenantAuthManager) GetToken(ctx context.Context, tenantContext *TenantContext,
	authConfig AuthConfig) (*TokenInfo, error) {

	if tenantContext == nil {
		return nil, NewAuthenticationError("", "", "tenant context is required", nil)
	}

	if mtam.logger != nil {
		mtam.logger.Debugf("Getting token for tenant %s service %s (auth type: %s)",
			tenantContext.ShortHash(), tenantContext.ServiceName, authConfig.Type)
	}

	// For "none" auth type, return nil token (no authentication needed)
	if authConfig.Type == AuthTypeNone {
		if mtam.logger != nil {
			mtam.logger.Debugf("No authentication required for tenant %s service %s",
				tenantContext.ShortHash(), tenantContext.ServiceName)
		}
		return nil, nil
	}

	mtam.mu.RLock()
	strategy, exists := mtam.strategies[authConfig.Type]
	mtam.mu.RUnlock()

	if !exists {
		if mtam.logger != nil {
			mtam.logger.Errorf("Unsupported authentication type for tenant %s service %s: %s",
				tenantContext.ShortHash(), tenantContext.ServiceName, authConfig.Type)
		}
		return nil, NewAuthenticationError(authConfig.Type, tenantContext.ServiceName,
			"unsupported authentication type", nil)
	}

	// Check if we have a cached token
	if mtam.logger != nil {
		mtam.logger.Debugf("Checking cached token for tenant %s service: %s",
			tenantContext.ShortHash(), tenantContext.ServiceName)
	}

	if tokenInfo := mtam.getCachedToken(tenantContext); tokenInfo != nil {
		if mtam.logger != nil {
			mtam.logger.Debugf("Found cached token for tenant %s service %s",
				tenantContext.ShortHash(), tenantContext.ServiceName)
		}

		// Check if token is expired (with 5-minute buffer)
		if !tokenInfo.IsExpiredWithBuffer(5 * time.Minute) {
			if mtam.logger != nil {
				expiryInfo := "no expiry"
				if tokenInfo.ExpiresAt != nil {
					expiryInfo = fmt.Sprintf("expires at %s", tokenInfo.ExpiresAt.Format(time.RFC3339))
				}
				mtam.logger.Debugf("Using valid cached token for tenant %s service %s (%s)",
					tenantContext.ShortHash(), tenantContext.ServiceName, expiryInfo)
			}
			return tokenInfo, nil
		}

		if mtam.logger != nil {
			mtam.logger.Debugf("Cached token for tenant %s service %s is expired, attempting refresh",
				tenantContext.ShortHash(), tenantContext.ServiceName)
		}

		// Try to refresh if supported and we have a refresh token
		if strategy.SupportsRefresh() && tokenInfo.HasRefreshToken() {
			if mtam.logger != nil {
				mtam.logger.Debugf("Attempting to refresh token for tenant %s service: %s",
					tenantContext.ShortHash(), tenantContext.ServiceName)
			}
			if refreshedToken, err := strategy.RefreshToken(ctx, tokenInfo, authConfig.Config); err == nil {
				mtam.CacheToken(tenantContext, refreshedToken)
				if mtam.logger != nil {
					mtam.logger.Infof("Successfully refreshed token for tenant %s service: %s",
						tenantContext.ShortHash(), tenantContext.ServiceName)
				}
				return refreshedToken, nil
			} else {
				if mtam.logger != nil {
					mtam.logger.Warningf("Failed to refresh token for tenant %s service %s: %v",
						tenantContext.ShortHash(), tenantContext.ServiceName, err)
				}
			}
		} else {
			if mtam.logger != nil {
				if !strategy.SupportsRefresh() {
					mtam.logger.Debugf("Token refresh not supported for auth type %s", authConfig.Type)
				} else {
					mtam.logger.Debugf("No refresh token available for tenant %s service %s",
						tenantContext.ShortHash(), tenantContext.ServiceName)
				}
			}
		}
	} else {
		if mtam.logger != nil {
			mtam.logger.Debugf("No cached token found for tenant %s service: %s",
				tenantContext.ShortHash(), tenantContext.ServiceName)
		}
	}

	// Perform new authentication
	if mtam.logger != nil {
		mtam.logger.Infof("Performing new authentication for tenant %s service %s using %s",
			tenantContext.ShortHash(), tenantContext.ServiceName, authConfig.Type)
	}

	tokenInfo, err := strategy.Authenticate(ctx, authConfig.Config)
	if err != nil {
		// Check if it's a DeviceCodeError - don't wrap it
		if _, ok := AsDeviceCodeError(err); ok {
			if mtam.logger != nil {
				mtam.logger.Infof("Device code authentication required for tenant %s service %s",
					tenantContext.ShortHash(), tenantContext.ServiceName)
			}
			return nil, err // Return DeviceCodeError directly
		}

		if mtam.logger != nil {
			mtam.logger.Errorf("Authentication failed for tenant %s service %s: %v",
				tenantContext.ShortHash(), tenantContext.ServiceName, err)
		}
		return nil, NewAuthenticationError(authConfig.Type, tenantContext.ServiceName,
			"authentication failed", err)
	}

	// Cache the new token
	mtam.CacheToken(tenantContext, tokenInfo)

	if mtam.logger != nil {
		expiryInfo := "no expiry"
		if tokenInfo.ExpiresAt != nil {
			expiryInfo = fmt.Sprintf("expires at %s", tokenInfo.ExpiresAt.Format(time.RFC3339))
		}
		mtam.logger.Infof("Successfully authenticated tenant %s service %s (%s)",
			tenantContext.ShortHash(), tenantContext.ServiceName, expiryInfo)
	}

	return tokenInfo, nil
}

// ApplyAuthentication applies authentication to an HTTP request for a specific tenant
func (mtam *MultiTenantAuthManager) ApplyAuthentication(ctx context.Context, req *http.Request,
	tenantContext *TenantContext, authConfig AuthConfig) error {

	if tenantContext == nil {
		return NewAuthenticationError("", "", "tenant context is required", nil)
	}

	if mtam.logger != nil {
		mtam.logger.Debugf("Applying authentication for tenant %s service %s to %s %s",
			tenantContext.ShortHash(), tenantContext.ServiceName, req.Method, req.URL.String())
	}

	tokenInfo, err := mtam.GetToken(ctx, tenantContext, authConfig)
	if err != nil {
		if mtam.logger != nil {
			mtam.logger.Errorf("Failed to get token for tenant %s service %s: %v",
				tenantContext.ShortHash(), tenantContext.ServiceName, err)
		}
		return err
	}

	// For "none" auth type, skip authentication entirely
	if authConfig.Type == AuthTypeNone {
		if mtam.logger != nil {
			mtam.logger.Debugf("Skipping authentication for tenant %s service %s (auth type: none)",
				tenantContext.ShortHash(), tenantContext.ServiceName)
		}
		return nil
	}

	mtam.mu.RLock()
	strategy, exists := mtam.strategies[authConfig.Type]
	mtam.mu.RUnlock()

	if !exists {
		if mtam.logger != nil {
			mtam.logger.Errorf("Strategy not found for auth type %s on tenant %s service %s",
				authConfig.Type, tenantContext.ShortHash(), tenantContext.ServiceName)
		}
		return NewAuthenticationError(authConfig.Type, tenantContext.ServiceName,
			"strategy not found", nil)
	}

	if mtam.logger != nil {
		mtam.logger.Debugf("Applying %s authentication to request for tenant %s service %s",
			authConfig.Type, tenantContext.ShortHash(), tenantContext.ServiceName)
	}

	if err := strategy.ApplyAuth(req, tokenInfo); err != nil {
		if mtam.logger != nil {
			mtam.logger.Errorf("Failed to apply authentication for tenant %s service %s: %v",
				tenantContext.ShortHash(), tenantContext.ServiceName, err)
		}
		return err
	}

	if mtam.logger != nil {
		mtam.logger.Debugf("Successfully applied authentication for tenant %s service %s",
			tenantContext.ShortHash(), tenantContext.ServiceName)
	}

	return nil
}

// InvalidateToken removes a token from cache for a specific tenant and service
func (mtam *MultiTenantAuthManager) InvalidateToken(tenantContext *TenantContext) {
	if tenantContext == nil {
		return
	}

	// Get or create a per-tenant lock to prevent concurrent invalidation attempts
	lockKey := fmt.Sprintf("%s:%s", tenantContext.TenantHash, tenantContext.ServiceName)

	// Use LoadOrStore for atomic get-or-create
	lockValue, _ := mtam.invalidationLocks.LoadOrStore(lockKey, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)

	// Lock for this specific tenant+service combination
	lock.Lock()
	defer lock.Unlock()

	// Delete from database
	if mtam.db != nil {
		if err := mtam.db.DeleteOAuthToken(tenantContext.TenantHash, tenantContext.ServiceName); err != nil {
			if mtam.logger != nil {
				mtam.logger.Warningf("Failed to delete token from database for tenant %s service %s: %v",
					tenantContext.ShortHash(), tenantContext.ServiceName, err)
			}
		}
	}

	// Delete from cache
	if mtam.cache != nil {
		cacheKey := mtam.buildCacheKey(tenantContext)
		if err := mtam.cache.Delete(cacheKey); err != nil && mtam.logger != nil {
			mtam.logger.Warningf("Failed to delete token from cache for tenant %s service %s: %v",
				tenantContext.ShortHash(), tenantContext.ServiceName, err)
		}
	}

	// Log at INFO level for security audit trail
	if mtam.logger != nil {
		mtam.logger.Infof("Invalidated token for tenant %s service: %s",
			tenantContext.ShortHash(), tenantContext.ServiceName)
	}
}

// RefreshIfPossible attempts to refresh an existing token without falling back to
// full re-authentication. It returns the refreshed token on success or an error
// describing why the refresh could not be performed. The caller decides whether
// to invalidate the token on failure.
func (mtam *MultiTenantAuthManager) RefreshIfPossible(ctx context.Context, tenantContext *TenantContext,
	authConfig AuthConfig) (*TokenInfo, error) {

	if tenantContext == nil {
		return nil, NewAuthenticationError("", "", "tenant context is required", nil)
	}

	if mtam.logger != nil {
		mtam.logger.Debugf("Attempting token refresh for tenant %s service %s (auth type: %s)",
			tenantContext.ShortHash(), tenantContext.ServiceName, authConfig.Type)
	}

	// Retrieve the current token from cache or DB without deleting it
	tokenInfo := mtam.getCachedToken(tenantContext)
	if tokenInfo == nil {
		if mtam.logger != nil {
			mtam.logger.Debugf("No token found to refresh for tenant %s service %s",
				tenantContext.ShortHash(), tenantContext.ServiceName)
		}
		return nil, NewAuthenticationError(authConfig.Type, tenantContext.ServiceName,
			"no token found to refresh", nil)
	}

	// Look up the strategy for this auth type
	mtam.mu.RLock()
	strategy, exists := mtam.strategies[authConfig.Type]
	mtam.mu.RUnlock()

	if !exists {
		if mtam.logger != nil {
			mtam.logger.Errorf("Unsupported authentication type for refresh: %s (tenant %s service %s)",
				authConfig.Type, tenantContext.ShortHash(), tenantContext.ServiceName)
		}
		return nil, NewAuthenticationError(authConfig.Type, tenantContext.ServiceName,
			"unsupported authentication type", nil)
	}

	// Verify that the strategy supports refresh
	if !strategy.SupportsRefresh() {
		if mtam.logger != nil {
			mtam.logger.Debugf("Strategy %s does not support token refresh for tenant %s service %s",
				authConfig.Type, tenantContext.ShortHash(), tenantContext.ServiceName)
		}
		return nil, NewAuthenticationError(authConfig.Type, tenantContext.ServiceName,
			"token refresh not supported by authentication strategy", nil)
	}

	// Verify that the token has a refresh token
	if !tokenInfo.HasRefreshToken() {
		if mtam.logger != nil {
			mtam.logger.Debugf("No refresh token available for tenant %s service %s",
				tenantContext.ShortHash(), tenantContext.ServiceName)
		}
		return nil, NewAuthenticationError(authConfig.Type, tenantContext.ServiceName,
			"no refresh token available", nil)
	}

	// Attempt the refresh
	if mtam.logger != nil {
		mtam.logger.Debugf("Refreshing token for tenant %s service %s using strategy %s",
			tenantContext.ShortHash(), tenantContext.ServiceName, authConfig.Type)
	}

	refreshedToken, err := strategy.RefreshToken(ctx, tokenInfo, authConfig.Config)
	if err != nil {
		if mtam.logger != nil {
			mtam.logger.Warningf("Token refresh failed for tenant %s service %s: %v",
				tenantContext.ShortHash(), tenantContext.ServiceName, err)
		}
		return nil, err
	}

	// Cache the refreshed token
	mtam.CacheToken(tenantContext, refreshedToken)

	if mtam.logger != nil {
		expiryInfo := "no expiry"
		if refreshedToken.ExpiresAt != nil {
			expiryInfo = fmt.Sprintf("expires at %s", refreshedToken.ExpiresAt.Format(time.RFC3339))
		}
		mtam.logger.Infof("Successfully refreshed token for tenant %s service %s (%s)",
			tenantContext.ShortHash(), tenantContext.ServiceName, expiryInfo)
	}

	return refreshedToken, nil
}

// getCachedToken retrieves a token from cache or database
func (mtam *MultiTenantAuthManager) getCachedToken(tenantContext *TenantContext) *TokenInfo {
	// Check cache first
	if mtam.cache != nil {
		cacheKey := mtam.buildCacheKey(tenantContext)
		if mtam.logger != nil {
			mtam.logger.Debugf("Checking cache for tenant %s service: %s",
				tenantContext.ShortHash(), tenantContext.ServiceName)
		}
		if data, err := mtam.cache.Get(cacheKey); err == nil {
			if tokenInfo, ok := data.(*TokenInfo); ok {
				if mtam.logger != nil {
					mtam.logger.Debugf("Found token in cache for tenant %s service: %s",
						tenantContext.ShortHash(), tenantContext.ServiceName)
				}
				return tokenInfo
			} else {
				if mtam.logger != nil {
					mtam.logger.Warningf("Invalid token data in cache for tenant %s service %s",
						tenantContext.ShortHash(), tenantContext.ServiceName)
				}
			}
		} else {
			if mtam.logger != nil {
				mtam.logger.Debugf("No token found in cache for tenant %s service %s: %v",
					tenantContext.ShortHash(), tenantContext.ServiceName, err)
			}
		}
	}

	// Check database
	if mtam.db != nil {
		if mtam.logger != nil {
			mtam.logger.Debugf("Checking database for tenant %s service: %s",
				tenantContext.ShortHash(), tenantContext.ServiceName)
		}
		if tokenData, err := mtam.db.GetOAuthToken(tenantContext.TenantHash, tenantContext.ServiceName); err == nil {
			tokenInfo := mtam.convertOAuthTokenDataToTokenInfo(tokenData)
			if mtam.logger != nil {
				mtam.logger.Debugf("Found token in database for tenant %s service: %s",
					tenantContext.ShortHash(), tenantContext.ServiceName)
			}
			// Cache it for future use
			if mtam.cache != nil {
				cacheKey := mtam.buildCacheKey(tenantContext)
				var ttl time.Duration
				if tokenInfo.ExpiresAt != nil {
					ttl = time.Until(*tokenInfo.ExpiresAt)
				} else {
					ttl = 24 * time.Hour // Default TTL
				}
				_ = mtam.cache.Set(cacheKey, tokenInfo, ttl)
			}
			return tokenInfo
		} else {
			if mtam.logger != nil {
				mtam.logger.Debugf("No token found in database for tenant %s service %s: %v",
					tenantContext.ShortHash(), tenantContext.ServiceName, err)
			}
		}
	}

	if mtam.logger != nil {
		mtam.logger.Debugf("No cached token found for tenant %s service: %s",
			tenantContext.ShortHash(), tenantContext.ServiceName)
	}

	return nil
}

// CacheToken stores a token in cache and database
func (mtam *MultiTenantAuthManager) CacheToken(tenantContext *TenantContext, tokenInfo *TokenInfo) {
	if tokenInfo == nil || tenantContext == nil {
		return
	}

	if mtam.logger != nil {
		expiryInfo := "no expiry"
		if tokenInfo.ExpiresAt != nil {
			expiryInfo = fmt.Sprintf("expires at %s", tokenInfo.ExpiresAt.Format(time.RFC3339))
		}
		mtam.logger.Debugf("Caching token for tenant %s service %s (%s)",
			tenantContext.ShortHash(), tenantContext.ServiceName, expiryInfo)
	}

	// Store in database
	if mtam.db != nil {
		tokenData := mtam.convertTokenInfoToOAuthTokenData(tokenInfo)
		if err := mtam.db.StoreOAuthToken(tenantContext.TenantHash, tenantContext.ServiceName, tokenData); err != nil {
			if mtam.logger != nil {
				mtam.logger.Warningf("Failed to store token in database for tenant %s service %s: %v",
					tenantContext.ShortHash(), tenantContext.ServiceName, err)
			}
		} else {
			if mtam.logger != nil {
				mtam.logger.Debugf("Successfully stored token in database for tenant %s service %s",
					tenantContext.ShortHash(), tenantContext.ServiceName)
			}
		}
	}

	// Store in cache
	if mtam.cache != nil {
		cacheKey := mtam.buildCacheKey(tenantContext)
		var ttl time.Duration
		if tokenInfo.ExpiresAt != nil {
			ttl = time.Until(*tokenInfo.ExpiresAt)
			if mtam.logger != nil {
				mtam.logger.Debugf("Setting cache TTL for tenant %s service %s: %v",
					tenantContext.ShortHash(), tenantContext.ServiceName, ttl)
			}
		} else {
			ttl = 24 * time.Hour // Default TTL if no expiration
			if mtam.logger != nil {
				mtam.logger.Debugf("Using default cache TTL for tenant %s service %s: %v",
					tenantContext.ShortHash(), tenantContext.ServiceName, ttl)
			}
		}

		if err := mtam.cache.Set(cacheKey, tokenInfo, ttl); err != nil {
			if mtam.logger != nil {
				mtam.logger.Warningf("Failed to cache token for tenant %s service %s: %v",
					tenantContext.ShortHash(), tenantContext.ServiceName, err)
			}
		} else {
			if mtam.logger != nil {
				mtam.logger.Debugf("Successfully cached token for tenant %s service %s",
					tenantContext.ShortHash(), tenantContext.ServiceName)
			}
		}
	}
}

// buildCacheKey builds a cache key for a tenant and service
// Format: "tenant:{tenantHash}:token:{serviceName}"
func (mtam *MultiTenantAuthManager) buildCacheKey(tenantContext *TenantContext) string {
	return fmt.Sprintf("tenant:%s:token:%s", tenantContext.TenantHash, tenantContext.ServiceName)
}

// parseCacheKey parses a cache key to extract tenant hash and service name
// Returns tenantHash, serviceName, error
func (mtam *MultiTenantAuthManager) parseCacheKey(cacheKey string) (string, string, error) {
	parts := strings.Split(cacheKey, ":")
	if len(parts) != 4 || parts[0] != "tenant" || parts[2] != "token" {
		return "", "", fmt.Errorf("invalid cache key format: %s", cacheKey)
	}
	return parts[1], parts[3], nil
}

// convertTokenInfoToOAuthTokenData converts TokenInfo to OAuthTokenData
func (mtam *MultiTenantAuthManager) convertTokenInfoToOAuthTokenData(tokenInfo *TokenInfo) *db.OAuthTokenData {
	if tokenInfo == nil {
		return nil
	}

	// Copy metadata if present
	var metadata map[string]string
	if len(tokenInfo.Metadata) > 0 {
		metadata = make(map[string]string, len(tokenInfo.Metadata))
		for k, v := range tokenInfo.Metadata {
			metadata[k] = v
		}
	}

	return &db.OAuthTokenData{
		AccessToken:  tokenInfo.AccessToken,
		RefreshToken: tokenInfo.RefreshToken,
		TokenType:    tokenInfo.TokenType,
		ExpiresAt:    tokenInfo.ExpiresAt,
		Scope:        tokenInfo.Scope,
		Metadata:     metadata,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// convertOAuthTokenDataToTokenInfo converts OAuthTokenData to TokenInfo
func (mtam *MultiTenantAuthManager) convertOAuthTokenDataToTokenInfo(tokenData *db.OAuthTokenData) *TokenInfo {
	if tokenData == nil {
		return nil
	}

	// Copy metadata if present, otherwise initialize empty map
	metadata := make(map[string]string)
	if len(tokenData.Metadata) > 0 {
		for k, v := range tokenData.Metadata {
			metadata[k] = v
		}
	}

	return &TokenInfo{
		AccessToken:  tokenData.AccessToken,
		RefreshToken: tokenData.RefreshToken,
		TokenType:    tokenData.TokenType,
		ExpiresAt:    tokenData.ExpiresAt,
		Scope:        tokenData.Scope,
		Metadata:     metadata,
	}
}

// ExtractTenantFromToken extracts tenant information from a bearer token
// If token is empty, returns a NOAUTH tenant context for no-auth mode
func (mtam *MultiTenantAuthManager) ExtractTenantFromToken(token string) (*TenantContext, error) {
	// Remove "Bearer " prefix if present
	if strings.HasPrefix(token, "Bearer ") {
		token = strings.TrimPrefix(token, "Bearer ")
	}

	// If token is empty, create a NOAUTH tenant context (for no-auth mode)
	if token == "" {
		tenantContext := &TenantContext{
			TenantHash:  NoAuthTenantHash,
			ServiceName: "default",
			Description: "No Authentication Mode (INSECURE)",
			Metadata:    make(map[string]string),
			CreatedAt:   time.Now(),
		}

		if mtam.logger != nil {
			mtam.logger.Debugf("Created NOAUTH tenant context (no-auth mode)")
		}

		return tenantContext, nil
	}

	// Validate the token against the database
	if mtam.db != nil {
		valid, hash, err := mtam.db.ValidateAPIToken(token)
		if err != nil {
			if mtam.logger != nil {
				mtam.logger.Errorf("Token validation error: %v", err)
			}
			return nil, fmt.Errorf("invalid token")
		}

		if !valid {
			if mtam.logger != nil {
				mtam.logger.Warning("Invalid token provided")
			}
			return nil, fmt.Errorf("invalid token")
		}

		// Get token metadata for additional context
		metadata, err := mtam.db.GetAPITokenMetadata(hash)
		if err != nil {
			if mtam.logger != nil {
				mtam.logger.Warningf("Failed to get token metadata: %v", err)
			}
		}

		tenantContext := &TenantContext{
			TenantHash:  hash,
			ServiceName: "default", // Service name will be resolved from request context
			Metadata:    make(map[string]string),
			CreatedAt:   time.Now(),
		}

		if metadata != nil {
			tenantContext.Description = metadata.Description
		}

		if mtam.logger != nil {
			mtam.logger.Debugf("Validated and extracted tenant context: %s", tenantContext.String())
		}

		return tenantContext, nil
	}

	// Fallback for when database is not available (testing/development mode)
	// Hash the token to create a tenant identifier
	hasher := sha256.New()
	hasher.Write([]byte(token))
	tenantHash := hex.EncodeToString(hasher.Sum(nil))

	tenantContext := &TenantContext{
		TenantHash:  tenantHash,
		ServiceName: "default",
		Metadata:    make(map[string]string),
		CreatedAt:   time.Now(),
	}

	if mtam.logger != nil {
		mtam.logger.Debugf("Extracted tenant context from token (testing/development mode): %s", tenantContext.String())
	}

	return tenantContext, nil
}

// ExtractTenantFromAuthCode validates an auth code and returns a TenantContext
// with the tenant hash stored at code creation time
func (mtam *MultiTenantAuthManager) ExtractTenantFromAuthCode(code string) (*TenantContext, error) {
	if mtam.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	tenantHash, service, err := mtam.db.ValidateAuthCode(code)
	if err != nil {
		if mtam.logger != nil {
			mtam.logger.Debugf("Auth code validation failed: %v", err)
		}
		return nil, fmt.Errorf("invalid auth code")
	}

	tenantContext := &TenantContext{
		TenantHash:  tenantHash,
		ServiceName: service,
		Description: "Auth code authentication",
		Metadata:    make(map[string]string),
		CreatedAt:   time.Now(),
	}

	if mtam.logger != nil {
		mtam.logger.Infof("Validated auth code for tenant %s service %s",
			tenantContext.ShortHash(), service)
	}

	return tenantContext, nil
}

// CreateAuthCode creates a time-limited auth code for the given tenant and service.
// Delegates to the underlying database.
func (mtam *MultiTenantAuthManager) CreateAuthCode(tenantHash, service string, ttl time.Duration) (string, error) {
	if mtam.db == nil {
		return "", fmt.Errorf("database not available")
	}
	return mtam.db.CreateAuthCode(tenantHash, service, ttl)
}

// ValidateTenantAccess validates that a tenant has access to a specific service
func (mtam *MultiTenantAuthManager) ValidateTenantAccess(tenantContext *TenantContext, serviceName string) error {
	if tenantContext == nil {
		return fmt.Errorf("tenant context is required")
	}

	if serviceName == "" {
		return fmt.Errorf("service name is required")
	}

	// For now, all tenants have access to all services
	// In a real implementation, this would check against tenant permissions
	if mtam.logger != nil {
		mtam.logger.Debugf("Validated tenant %s access to service %s",
			tenantContext.ShortHash(), serviceName)
	}

	return nil
}

// GetRegisteredStrategies returns a list of registered authentication types
func (mtam *MultiTenantAuthManager) GetRegisteredStrategies() []AuthType {
	mtam.mu.RLock()
	defer mtam.mu.RUnlock()

	types := make([]AuthType, 0, len(mtam.strategies))
	for authType := range mtam.strategies {
		types = append(types, authType)
	}
	return types
}

// HasStrategy checks if a strategy is registered for the given auth type
func (mtam *MultiTenantAuthManager) HasStrategy(authType AuthType) bool {
	mtam.mu.RLock()
	defer mtam.mu.RUnlock()

	_, exists := mtam.strategies[authType]
	return exists
}

// GetTenantTokens returns all tokens for a specific tenant
func (mtam *MultiTenantAuthManager) GetTenantTokens(tenantHash string) (map[string]*TokenInfo, error) {
	if mtam.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	tokenDataMap, err := mtam.db.ListOAuthTokens(tenantHash)
	if err != nil {
		return nil, fmt.Errorf("failed to list tokens for tenant: %w", err)
	}

	tokenInfoMap := make(map[string]*TokenInfo)
	for serviceName, tokenData := range tokenDataMap {
		tokenInfoMap[serviceName] = mtam.convertOAuthTokenDataToTokenInfo(tokenData)
	}

	if mtam.logger != nil {
		mtam.logger.Debugf("Retrieved %d tokens for tenant %s",
			len(tokenInfoMap), tenantHash[:12]+"...")
	}

	return tokenInfoMap, nil
}

// ListTenants returns a list of all tenants (this would need to be implemented based on your tenant management strategy)
func (mtam *MultiTenantAuthManager) ListTenants() ([]string, error) {
	// This is a placeholder implementation
	// In a real system, you'd query your database or tenant registry
	if mtam.logger != nil {
		mtam.logger.Debug("Listing tenants (placeholder implementation)")
	}
	return []string{}, nil
}
