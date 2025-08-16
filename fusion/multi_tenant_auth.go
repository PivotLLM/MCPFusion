/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
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

// TenantContext represents tenant-specific context information
type TenantContext struct {
	TenantHash  string            `json:"tenant_hash"`
	ServiceName string            `json:"service_name"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	RequestID   string            `json:"request_id,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// String returns a string representation of the tenant context
func (tc *TenantContext) String() string {
	if tc == nil {
		return "TenantContext(nil)"
	}
	return fmt.Sprintf("TenantContext(tenant=%s, service=%s, request=%s)",
		tc.TenantHash[:12]+"...", tc.ServiceName, tc.RequestID)
}

// MultiTenantAuthManager manages authentication for multiple tenants
type MultiTenantAuthManager struct {
	db         *db.DB
	strategies map[AuthType]AuthStrategy
	cache      Cache
	logger     global.Logger
	mu         sync.RWMutex
}

// NewMultiTenantAuthManager creates a new multi-tenant authentication manager
func NewMultiTenantAuthManager(database *db.DB, cache Cache, logger global.Logger) *MultiTenantAuthManager {
	return &MultiTenantAuthManager{
		db:         database,
		strategies: make(map[AuthType]AuthStrategy),
		cache:      cache,
		logger:     logger,
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
			tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, authConfig.Type)
	}

	mtam.mu.RLock()
	strategy, exists := mtam.strategies[authConfig.Type]
	mtam.mu.RUnlock()

	if !exists {
		if mtam.logger != nil {
			mtam.logger.Errorf("Unsupported authentication type for tenant %s service %s: %s",
				tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, authConfig.Type)
		}
		return nil, NewAuthenticationError(authConfig.Type, tenantContext.ServiceName,
			"unsupported authentication type", nil)
	}

	// Check if we have a cached token
	if mtam.logger != nil {
		mtam.logger.Debugf("Checking cached token for tenant %s service: %s",
			tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
	}

	if tokenInfo := mtam.getCachedToken(tenantContext); tokenInfo != nil {
		if mtam.logger != nil {
			mtam.logger.Debugf("Found cached token for tenant %s service %s",
				tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
		}

		// Check if token is expired (with 5-minute buffer)
		if !tokenInfo.IsExpiredWithBuffer(5 * time.Minute) {
			if mtam.logger != nil {
				expiryInfo := "no expiry"
				if tokenInfo.ExpiresAt != nil {
					expiryInfo = fmt.Sprintf("expires at %s", tokenInfo.ExpiresAt.Format(time.RFC3339))
				}
				mtam.logger.Debugf("Using valid cached token for tenant %s service %s (%s)",
					tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, expiryInfo)
			}
			return tokenInfo, nil
		}

		if mtam.logger != nil {
			mtam.logger.Debugf("Cached token for tenant %s service %s is expired, attempting refresh",
				tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
		}

		// Try to refresh if supported and we have a refresh token
		if strategy.SupportsRefresh() && tokenInfo.HasRefreshToken() {
			if mtam.logger != nil {
				mtam.logger.Debugf("Attempting to refresh token for tenant %s service: %s",
					tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
			}
			if refreshedToken, err := strategy.RefreshToken(ctx, tokenInfo, authConfig.Config); err == nil {
				mtam.CacheToken(tenantContext, refreshedToken)
				if mtam.logger != nil {
					mtam.logger.Infof("Successfully refreshed token for tenant %s service: %s",
						tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
				}
				return refreshedToken, nil
			} else {
				if mtam.logger != nil {
					mtam.logger.Warningf("Failed to refresh token for tenant %s service %s: %v",
						tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, err)
				}
			}
		} else {
			if mtam.logger != nil {
				if !strategy.SupportsRefresh() {
					mtam.logger.Debugf("Token refresh not supported for auth type %s", authConfig.Type)
				} else {
					mtam.logger.Debugf("No refresh token available for tenant %s service %s",
						tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
				}
			}
		}
	} else {
		if mtam.logger != nil {
			mtam.logger.Debugf("No cached token found for tenant %s service: %s",
				tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
		}
	}

	// Perform new authentication
	if mtam.logger != nil {
		mtam.logger.Infof("Performing new authentication for tenant %s service %s using %s",
			tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, authConfig.Type)
	}

	tokenInfo, err := strategy.Authenticate(ctx, authConfig.Config)
	if err != nil {
		// Check if it's a DeviceCodeError - don't wrap it
		if _, ok := err.(*DeviceCodeError); ok {
			if mtam.logger != nil {
				mtam.logger.Infof("Device code authentication required for tenant %s service %s",
					tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
			}
			return nil, err // Return DeviceCodeError directly
		}

		if mtam.logger != nil {
			mtam.logger.Errorf("Authentication failed for tenant %s service %s: %v",
				tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, err)
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
			tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, expiryInfo)
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
			tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, req.Method, req.URL.String())
	}

	tokenInfo, err := mtam.GetToken(ctx, tenantContext, authConfig)
	if err != nil {
		if mtam.logger != nil {
			mtam.logger.Errorf("Failed to get token for tenant %s service %s: %v",
				tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, err)
		}
		return err
	}

	mtam.mu.RLock()
	strategy, exists := mtam.strategies[authConfig.Type]
	mtam.mu.RUnlock()

	if !exists {
		if mtam.logger != nil {
			mtam.logger.Errorf("Strategy not found for auth type %s on tenant %s service %s",
				authConfig.Type, tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
		}
		return NewAuthenticationError(authConfig.Type, tenantContext.ServiceName,
			"strategy not found", nil)
	}

	if mtam.logger != nil {
		mtam.logger.Debugf("Applying %s authentication to request for tenant %s service %s",
			authConfig.Type, tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
	}

	if err := strategy.ApplyAuth(req, tokenInfo); err != nil {
		if mtam.logger != nil {
			mtam.logger.Errorf("Failed to apply authentication for tenant %s service %s: %v",
				tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, err)
		}
		return err
	}

	if mtam.logger != nil {
		mtam.logger.Debugf("Successfully applied authentication for tenant %s service %s",
			tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
	}

	return nil
}

// InvalidateToken removes a token from cache for a specific tenant and service
func (mtam *MultiTenantAuthManager) InvalidateToken(tenantContext *TenantContext) {
	if tenantContext == nil {
		return
	}

	// Delete from database
	if mtam.db != nil {
		if err := mtam.db.DeleteOAuthToken(tenantContext.TenantHash, tenantContext.ServiceName); err != nil {
			if mtam.logger != nil {
				mtam.logger.Warningf("Failed to delete token from database for tenant %s service %s: %v",
					tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, err)
			}
		}
	}

	// Delete from cache
	if mtam.cache != nil {
		cacheKey := mtam.buildCacheKey(tenantContext)
		if err := mtam.cache.Delete(cacheKey); err != nil && mtam.logger != nil {
			mtam.logger.Warningf("Failed to delete token from cache for tenant %s service %s: %v",
				tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, err)
		}
	}

	if mtam.logger != nil {
		mtam.logger.Infof("Invalidated token for tenant %s service: %s",
			tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
	}
}

// getCachedToken retrieves a token from cache or database
func (mtam *MultiTenantAuthManager) getCachedToken(tenantContext *TenantContext) *TokenInfo {
	// Check cache first
	if mtam.cache != nil {
		cacheKey := mtam.buildCacheKey(tenantContext)
		if mtam.logger != nil {
			mtam.logger.Debugf("Checking cache for tenant %s service: %s",
				tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
		}
		if data, err := mtam.cache.Get(cacheKey); err == nil {
			if tokenInfo, ok := data.(*TokenInfo); ok {
				if mtam.logger != nil {
					mtam.logger.Debugf("Found token in cache for tenant %s service: %s",
						tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
				}
				return tokenInfo
			} else {
				if mtam.logger != nil {
					mtam.logger.Warningf("Invalid token data in cache for tenant %s service %s",
						tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
				}
			}
		} else {
			if mtam.logger != nil {
				mtam.logger.Debugf("No token found in cache for tenant %s service %s: %v",
					tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, err)
			}
		}
	}

	// Check database
	if mtam.db != nil {
		if mtam.logger != nil {
			mtam.logger.Debugf("Checking database for tenant %s service: %s",
				tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
		}
		if tokenData, err := mtam.db.GetOAuthToken(tenantContext.TenantHash, tenantContext.ServiceName); err == nil {
			tokenInfo := mtam.convertOAuthTokenDataToTokenInfo(tokenData)
			if mtam.logger != nil {
				mtam.logger.Debugf("Found token in database for tenant %s service: %s",
					tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
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
					tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, err)
			}
		}
	}

	if mtam.logger != nil {
		mtam.logger.Debugf("No cached token found for tenant %s service: %s",
			tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
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
			tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, expiryInfo)
	}

	// Store in database
	if mtam.db != nil {
		tokenData := mtam.convertTokenInfoToOAuthTokenData(tokenInfo)
		if err := mtam.db.StoreOAuthToken(tenantContext.TenantHash, tenantContext.ServiceName, tokenData); err != nil {
			if mtam.logger != nil {
				mtam.logger.Warningf("Failed to store token in database for tenant %s service %s: %v",
					tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, err)
			}
		} else {
			if mtam.logger != nil {
				mtam.logger.Debugf("Successfully stored token in database for tenant %s service %s",
					tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
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
					tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, ttl)
			}
		} else {
			ttl = 24 * time.Hour // Default TTL if no expiration
			if mtam.logger != nil {
				mtam.logger.Debugf("Using default cache TTL for tenant %s service %s: %v",
					tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, ttl)
			}
		}

		if err := mtam.cache.Set(cacheKey, tokenInfo, ttl); err != nil {
			if mtam.logger != nil {
				mtam.logger.Warningf("Failed to cache token for tenant %s service %s: %v",
					tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, err)
			}
		} else {
			if mtam.logger != nil {
				mtam.logger.Debugf("Successfully cached token for tenant %s service %s",
					tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
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

	return &db.OAuthTokenData{
		AccessToken:  tokenInfo.AccessToken,
		RefreshToken: tokenInfo.RefreshToken,
		TokenType:    tokenInfo.TokenType,
		ExpiresAt:    tokenInfo.ExpiresAt,
		Scope:        tokenInfo.Scope,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// convertOAuthTokenDataToTokenInfo converts OAuthTokenData to TokenInfo
func (mtam *MultiTenantAuthManager) convertOAuthTokenDataToTokenInfo(tokenData *db.OAuthTokenData) *TokenInfo {
	if tokenData == nil {
		return nil
	}

	return &TokenInfo{
		AccessToken:  tokenData.AccessToken,
		RefreshToken: tokenData.RefreshToken,
		TokenType:    tokenData.TokenType,
		ExpiresAt:    tokenData.ExpiresAt,
		Scope:        tokenData.Scope,
		Metadata:     make(map[string]string), // Initialize empty metadata
	}
}

// ExtractTenantFromToken extracts tenant information from a bearer token
func (mtam *MultiTenantAuthManager) ExtractTenantFromToken(token string) (*TenantContext, error) {
	if token == "" {
		return nil, fmt.Errorf("invalid token")
	}

	// Remove "Bearer " prefix if present
	if strings.HasPrefix(token, "Bearer ") {
		token = strings.TrimPrefix(token, "Bearer ")
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
			tenantContext.TenantHash[:12]+"...", serviceName)
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
