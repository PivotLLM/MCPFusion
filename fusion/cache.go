/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"sync"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
)

// Cache defines the interface for caching operations
type Cache interface {
	// Get retrieves a value from the cache
	Get(key string) (interface{}, error)

	// Set stores a value in the cache with the given TTL
	Set(key string, value interface{}, ttl time.Duration) error

	// Delete removes a value from the cache
	Delete(key string) error

	// Clear removes all values from the cache
	Clear() error

	// Has checks if a key exists in the cache
	Has(key string) bool
}

// cacheItem represents an item stored in the cache
type cacheItem struct {
	value     interface{}
	expiresAt time.Time
}

// isExpired checks if the cache item has expired
func (ci *cacheItem) isExpired() bool {
	return time.Now().After(ci.expiresAt)
}

// InMemoryCache implements a simple in-memory cache
// DEPRECATED: Use DatabaseCache with multi-tenant authentication instead.
// This cache implementation is only kept for compatibility and will be removed.
type InMemoryCache struct {
	items  map[string]*cacheItem
	mu     sync.RWMutex
	logger global.Logger
}
