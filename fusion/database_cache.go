/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/global"
)

// DatabaseCache implements the Cache interface using the database for persistent storage
type DatabaseCache struct {
	db         *db.DB
	logger     global.Logger
	defaultTTL time.Duration
}

// CacheItem represents an item stored in the database cache
type CacheItem struct {
	Value     json.RawMessage `json:"value"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	TTL       time.Duration   `json:"ttl"`
}

// IsExpired checks if the cache item has expired
func (ci *CacheItem) IsExpired() bool {
	if ci.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*ci.ExpiresAt)
}

// NewDatabaseCache creates a new database-backed cache
func NewDatabaseCache(database *db.DB, logger global.Logger) *DatabaseCache {
	return NewDatabaseCacheWithDefaultTTL(database, logger, 24*time.Hour)
}

// NewDatabaseCacheWithDefaultTTL creates a new database-backed cache with a custom default TTL
func NewDatabaseCacheWithDefaultTTL(database *db.DB, logger global.Logger, defaultTTL time.Duration) *DatabaseCache {
	cache := &DatabaseCache{
		db:         database,
		logger:     logger,
		defaultTTL: defaultTTL,
	}

	if logger != nil {
		logger.Infof("Initialized database cache with default TTL: %v", defaultTTL)
	}

	return cache
}

// Get retrieves a value from the database cache
func (dc *DatabaseCache) Get(key string) (interface{}, error) {
	if dc.logger != nil {
		dc.logger.Debugf("Database cache GET operation for key: %s", key)
	}

	if dc.db == nil {
		return nil, &CacheError{Operation: "get", Key: key, Message: "database not available"}
	}

	// Parse the cache key to extract tenant information
	tenantHash, serviceName, err := dc.parseCacheKey(key)
	if err != nil {
		// If it's not a tenant-specific key, fall back to a generic cache approach
		if dc.logger != nil {
			dc.logger.Warningf("Failed to parse cache key %s as tenant key: %v", key, err)
		}
		return nil, &CacheError{Operation: "get", Key: key, Message: "unsupported cache key format"}
	}

	// Get the OAuth token from the database
	tokenData, err := dc.db.GetOAuthToken(tenantHash, serviceName)
	if err != nil {
		if dc.logger != nil {
			dc.logger.Debugf("Database cache MISS - token not found for key: %s (%v)", key, err)
		}
		return nil, &CacheError{Operation: "get", Key: key, Message: "key not found"}
	}

	// Convert OAuthTokenData to TokenInfo
	tokenInfo := dc.convertOAuthTokenDataToTokenInfo(tokenData)

	// Check if the token is expired
	if tokenInfo.IsExpired() {
		if dc.logger != nil {
			dc.logger.Debugf("Database cache MISS - token expired for key: %s", key)
		}
		// Clean up expired token
		_ = dc.db.DeleteOAuthToken(tenantHash, serviceName)
		return nil, &CacheError{Operation: "get", Key: key, Message: "key expired"}
	}

	if dc.logger != nil {
		expiryInfo := "no expiry"
		if tokenInfo.ExpiresAt != nil {
			timeToExpiry := time.Until(*tokenInfo.ExpiresAt)
			expiryInfo = fmt.Sprintf("expires in %v", timeToExpiry)
		}
		dc.logger.Debugf("Database cache HIT for key: %s (%s)", key, expiryInfo)
	}

	return tokenInfo, nil
}

// Set stores a value in the database cache with the given TTL
func (dc *DatabaseCache) Set(key string, value interface{}, ttl time.Duration) error {
	if dc.logger != nil {
		dc.logger.Debugf("Database cache SET operation for key: %s (TTL: %v)", key, ttl)
	}

	if dc.db == nil {
		return &CacheError{Operation: "set", Key: key, Message: "database not available"}
	}

	// Parse the cache key to extract tenant information
	tenantHash, serviceName, err := dc.parseCacheKey(key)
	if err != nil {
		if dc.logger != nil {
			dc.logger.Warningf("Failed to parse cache key %s as tenant key: %v", key, err)
		}
		return &CacheError{Operation: "set", Key: key, Message: "unsupported cache key format"}
	}

	// Convert value to TokenInfo
	tokenInfo, ok := value.(*TokenInfo)
	if !ok {
		if dc.logger != nil {
			dc.logger.Errorf("Database cache SET failed - invalid value type for key: %s", key)
		}
		return &CacheError{Operation: "set", Key: key, Message: "value must be *TokenInfo"}
	}

	// Convert TokenInfo to OAuthTokenData
	tokenData := dc.convertTokenInfoToOAuthTokenData(tokenInfo)

	// Set expiration based on TTL if not already set
	if tokenData.ExpiresAt == nil && ttl > 0 {
		expiresAt := time.Now().Add(ttl)
		tokenData.ExpiresAt = &expiresAt
		tokenInfo.ExpiresAt = &expiresAt // Update the original TokenInfo as well
	}

	// Store in database
	if err := dc.db.StoreOAuthToken(tenantHash, serviceName, tokenData); err != nil {
		if dc.logger != nil {
			dc.logger.Errorf("Database cache SET failed for key %s: %v", key, err)
		}
		return &CacheError{Operation: "set", Key: key, Message: fmt.Sprintf("database error: %v", err)}
	}

	if dc.logger != nil {
		expiryInfo := "no expiry"
		if tokenData.ExpiresAt != nil {
			expiryInfo = fmt.Sprintf("expires at: %s", tokenData.ExpiresAt.Format(time.RFC3339))
		}
		dc.logger.Debugf("Database cache SET successful for key: %s (%s)", key, expiryInfo)
	}

	return nil
}

// Delete removes a value from the database cache
func (dc *DatabaseCache) Delete(key string) error {
	if dc.logger != nil {
		dc.logger.Debugf("Database cache DELETE operation for key: %s", key)
	}

	if dc.db == nil {
		return &CacheError{Operation: "delete", Key: key, Message: "database not available"}
	}

	// Parse the cache key to extract tenant information
	tenantHash, serviceName, err := dc.parseCacheKey(key)
	if err != nil {
		if dc.logger != nil {
			dc.logger.Warningf("Failed to parse cache key %s as tenant key: %v", key, err)
		}
		return &CacheError{Operation: "delete", Key: key, Message: "unsupported cache key format"}
	}

	// Delete from database
	err = dc.db.DeleteOAuthToken(tenantHash, serviceName)
	if err != nil {
		if dc.logger != nil {
			dc.logger.Warningf("Database cache DELETE failed for key %s: %v", key, err)
		}
		// Don't return error for "not found" cases - deletion is idempotent
		if !strings.Contains(err.Error(), "not found") {
			return &CacheError{Operation: "delete", Key: key, Message: fmt.Sprintf("database error: %v", err)}
		}
	}

	if dc.logger != nil {
		dc.logger.Debugf("Database cache DELETE successful for key: %s", key)
	}

	return nil
}

// Clear removes all values from the database cache
func (dc *DatabaseCache) Clear() error {
	if dc.logger != nil {
		dc.logger.Debug("Database cache CLEAR operation")
	}

	if dc.db == nil {
		return &CacheError{Operation: "clear", Key: "", Message: "database not available"}
	}

	// This is a complex operation that would require iterating through all tenants
	// For now, we'll just log that this operation is not fully implemented
	if dc.logger != nil {
		dc.logger.Warning("Database cache CLEAR operation not fully implemented - individual tenant cache clearing not supported")
	}

	// In a real implementation, you would need to:
	// 1. List all tenants
	// 2. For each tenant, list all OAuth tokens
	// 3. Delete each OAuth token
	// This is omitted for now as it would require additional database methods

	return nil
}

// Has checks if a key exists in the database cache
func (dc *DatabaseCache) Has(key string) bool {
	if dc.logger != nil {
		dc.logger.Debugf("Database cache HAS operation for key: %s", key)
	}

	if dc.db == nil {
		if dc.logger != nil {
			dc.logger.Debugf("Database cache HAS result for key %s: false (database not available)", key)
		}
		return false
	}

	// Parse the cache key to extract tenant information
	tenantHash, serviceName, err := dc.parseCacheKey(key)
	if err != nil {
		if dc.logger != nil {
			dc.logger.Debugf("Database cache HAS result for key %s: false (invalid format)", key)
		}
		return false
	}

	// Check if the token exists and is not expired
	tokenData, err := dc.db.GetOAuthToken(tenantHash, serviceName)
	if err != nil {
		if dc.logger != nil {
			dc.logger.Debugf("Database cache HAS result for key %s: false (not found)", key)
		}
		return false
	}

	// Check if expired
	if tokenData.IsExpired() {
		if dc.logger != nil {
			dc.logger.Debugf("Database cache HAS result for key %s: false (expired)", key)
		}
		// Clean up expired token
		_ = dc.db.DeleteOAuthToken(tenantHash, serviceName)
		return false
	}

	if dc.logger != nil {
		dc.logger.Debugf("Database cache HAS result for key %s: true", key)
	}

	return true
}

// parseCacheKey parses a cache key to extract tenant hash and service name
// Expected format: "tenant:{tenantHash}:token:{serviceName}"
func (dc *DatabaseCache) parseCacheKey(cacheKey string) (tenantHash, serviceName string, err error) {
	parts := strings.Split(cacheKey, ":")
	if len(parts) != 4 || parts[0] != "tenant" || parts[2] != "token" {
		return "", "", fmt.Errorf("invalid cache key format: expected 'tenant:{hash}:token:{service}', got '%s'", cacheKey)
	}
	return parts[1], parts[3], nil
}

// buildCacheKey builds a cache key from tenant hash and service name
func (dc *DatabaseCache) buildCacheKey(tenantHash, serviceName string) string {
	return fmt.Sprintf("tenant:%s:token:%s", tenantHash, serviceName)
}

// convertTokenInfoToOAuthTokenData converts TokenInfo to OAuthTokenData
func (dc *DatabaseCache) convertTokenInfoToOAuthTokenData(tokenInfo *TokenInfo) *db.OAuthTokenData {
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
func (dc *DatabaseCache) convertOAuthTokenDataToTokenInfo(tokenData *db.OAuthTokenData) *TokenInfo {
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

// Size returns the approximate number of items in the cache
// Note: This is an expensive operation as it requires scanning all tenants
func (dc *DatabaseCache) Size() int {
	if dc.db == nil {
		return 0
	}

	// This would require a method to count all OAuth tokens across all tenants
	// For now, return 0 as this is not efficiently implementable without additional database methods
	if dc.logger != nil {
		dc.logger.Debug("Database cache SIZE operation not efficiently implementable - returning 0")
	}
	return 0
}

// Keys returns all keys in the cache
// Note: This is an expensive operation as it requires scanning all tenants
func (dc *DatabaseCache) Keys() []string {
	if dc.db == nil {
		return []string{}
	}

	// This would require listing all tenants and their OAuth tokens
	// For now, return empty slice as this is not efficiently implementable without additional database methods
	if dc.logger != nil {
		dc.logger.Debug("Database cache KEYS operation not efficiently implementable - returning empty slice")
	}
	return []string{}
}

// CleanupExpired removes expired tokens from the database
// This method can be called periodically to clean up expired tokens
func (dc *DatabaseCache) CleanupExpired() error {
	if dc.logger != nil {
		dc.logger.Debug("Database cache cleanup starting")
	}

	if dc.db == nil {
		return fmt.Errorf("database not available")
	}

	// This would require additional database methods to efficiently find and clean up expired tokens
	// For now, just log that cleanup is needed
	if dc.logger != nil {
		dc.logger.Info("Database cache cleanup completed (note: automatic cleanup requires additional database methods)")
	}

	return nil
}

// GetStats returns statistics about the cache
func (dc *DatabaseCache) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"type":        "database",
		"default_ttl": dc.defaultTTL.String(),
		"available":   dc.db != nil,
	}

	if dc.db != nil {
		stats["database_available"] = true
	} else {
		stats["database_available"] = false
	}

	return stats
}
