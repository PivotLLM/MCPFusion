/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
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
type InMemoryCache struct {
	items  map[string]*cacheItem
	mu     sync.RWMutex
	logger global.Logger
}

// NewInMemoryCacheWithLogger creates a new in-memory cache with logging support
func NewInMemoryCacheWithLogger(logger global.Logger) *InMemoryCache {
	cache := &InMemoryCache{
		items:  make(map[string]*cacheItem),
		logger: logger,
	}

	if logger != nil {
		logger.Debug("Initializing in-memory cache")
	}

	// Start cleanup goroutine
	go cache.cleanup()

	if logger != nil {
		logger.Debug("In-memory cache initialized successfully")
	}

	return cache
}

// NewInMemoryCache creates a new in-memory cache
func NewInMemoryCache() *InMemoryCache {
	return NewInMemoryCacheWithLogger(nil)
}

// Get retrieves a value from the cache
func (c *InMemoryCache) Get(key string) (interface{}, error) {
	if c.logger != nil {
		c.logger.Debugf("Cache GET operation for key: %s", key)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		if c.logger != nil {
			c.logger.Debugf("Cache MISS - key not found: %s", key)
		}
		return nil, &CacheError{Operation: "get", Key: key, Message: "key not found"}
	}

	if item.isExpired() {
		if c.logger != nil {
			c.logger.Debugf("Cache MISS - key expired: %s", key)
		}
		// Remove expired item
		delete(c.items, key)
		return nil, &CacheError{Operation: "get", Key: key, Message: "key expired"}
	}

	if c.logger != nil {
		timeToExpiry := time.Until(item.expiresAt)
		c.logger.Debugf("Cache HIT for key: %s (expires in %v)", key, timeToExpiry)
	}

	return item.value, nil
}

// Set stores a value in the cache with the given TTL
func (c *InMemoryCache) Set(key string, value interface{}, ttl time.Duration) error {
	if c.logger != nil {
		c.logger.Debugf("Cache SET operation for key: %s (TTL: %v)", key, ttl)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	expiresAt := time.Now().Add(ttl)
	c.items[key] = &cacheItem{
		value:     value,
		expiresAt: expiresAt,
	}

	if c.logger != nil {
		c.logger.Debugf("Cache SET successful for key: %s (expires at: %s)", key, expiresAt.Format(time.RFC3339))
	}

	return nil
}

// Delete removes a value from the cache
func (c *InMemoryCache) Delete(key string) error {
	if c.logger != nil {
		c.logger.Debugf("Cache DELETE operation for key: %s", key)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	_, existed := c.items[key]
	delete(c.items, key)

	if c.logger != nil {
		if existed {
			c.logger.Debugf("Cache DELETE successful for key: %s", key)
		} else {
			c.logger.Debugf("Cache DELETE - key did not exist: %s", key)
		}
	}

	return nil
}

// Clear removes all values from the cache
func (c *InMemoryCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	itemCount := len(c.items)
	c.items = make(map[string]*cacheItem)

	if c.logger != nil {
		c.logger.Infof("Cache CLEAR operation completed - removed %d items", itemCount)
	}

	return nil
}

// Has checks if a key exists in the cache
func (c *InMemoryCache) Has(key string) bool {
	if c.logger != nil {
		c.logger.Debugf("Cache HAS operation for key: %s", key)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		if c.logger != nil {
			c.logger.Debugf("Cache HAS result for key %s: false (not found)", key)
		}
		return false
	}

	if item.isExpired() {
		if c.logger != nil {
			c.logger.Debugf("Cache HAS result for key %s: false (expired)", key)
		}
		// Remove expired item
		delete(c.items, key)
		return false
	}

	if c.logger != nil {
		c.logger.Debugf("Cache HAS result for key %s: true", key)
	}

	return true
}

// cleanup removes expired items from the cache
func (c *InMemoryCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	defer ticker.Stop()

	if c.logger != nil {
		c.logger.Debug("Cache cleanup goroutine started (runs every 5 minutes)")
	}

	for range ticker.C {
		c.mu.Lock()

		initialCount := len(c.items)
		expiredCount := 0

		for key, item := range c.items {
			if item.isExpired() {
				delete(c.items, key)
				expiredCount++
			}
		}

		c.mu.Unlock()

		if c.logger != nil {
			if expiredCount > 0 {
				c.logger.Infof("Cache cleanup completed - removed %d expired items (%d remaining)", expiredCount, initialCount-expiredCount)
			} else {
				c.logger.Debugf("Cache cleanup completed - no expired items found (%d items in cache)", initialCount)
			}
		}
	}
}

// Size returns the number of items in the cache
func (c *InMemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.items)
}

// Keys returns all keys in the cache
func (c *InMemoryCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.items))
	for key, item := range c.items {
		if !item.isExpired() {
			keys = append(keys, key)
		}
	}

	return keys
}

// NoOpCache implements a cache that doesn't actually cache anything
type NoOpCache struct{}

// NewNoOpCache creates a new no-op cache
func NewNoOpCache() *NoOpCache {
	return &NoOpCache{}
}

// Get always returns a "not found" error
func (c *NoOpCache) Get(key string) (interface{}, error) {
	return nil, &CacheError{Operation: "get", Key: key, Message: "no-op cache does not store values"}
}

// Set does nothing and returns nil
func (c *NoOpCache) Set(key string, value interface{}, ttl time.Duration) error {
	return nil
}

// Delete does nothing and returns nil
func (c *NoOpCache) Delete(key string) error {
	return nil
}

// Clear does nothing and returns nil
func (c *NoOpCache) Clear() error {
	return nil
}

// Has always returns false
func (c *NoOpCache) Has(key string) bool {
	return false
}
