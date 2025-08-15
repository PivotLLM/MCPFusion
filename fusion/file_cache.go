/*=============================================================================
= Copyright (c) 2025 Tenebris Technologies Inc.                              =
= All rights reserved.                                                       =
=============================================================================*/

package fusion

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
)

// FileCacheItem represents an item stored in the file cache
type FileCacheItem struct {
	Value     json.RawMessage `json:"value"`
	ExpiresAt time.Time       `json:"expires_at"`
	CreatedAt time.Time       `json:"created_at"`
}

// FileCache implements a file-based persistent cache
type FileCache struct {
	basePath string
	mu       sync.RWMutex
	logger   global.Logger
}

// NewFileCache creates a new file-based cache
func NewFileCache(logger global.Logger) *FileCache {
	cache := &FileCache{
		logger: logger,
	}
	
	// Determine cache directory
	cache.basePath = cache.determineCacheDirectory()
	
	// Ensure cache directory exists
	if err := os.MkdirAll(cache.basePath, 0755); err != nil {
		if logger != nil {
			logger.Errorf("Failed to create cache directory %s: %v", cache.basePath, err)
		}
	}
	
	if logger != nil {
		logger.Infof("File cache initialized at: %s", cache.basePath)
	}
	
	// Start cleanup goroutine
	go cache.cleanup()
	
	return cache
}

// determineCacheDirectory determines the best cache directory to use
func (c *FileCache) determineCacheDirectory() string {
	// Try /opt/mcpfusion/cache first
	systemCacheDir := "/opt/mcpfusion/cache"
	
	// Check if we can create or access the system cache directory
	if err := os.MkdirAll(systemCacheDir, 0755); err == nil {
		// Test if we can write to it
		testFile := filepath.Join(systemCacheDir, ".test")
		if err := ioutil.WriteFile(testFile, []byte("test"), 0644); err == nil {
			os.Remove(testFile)
			if c.logger != nil {
				c.logger.Debugf("Using system cache directory: %s", systemCacheDir)
			}
			return systemCacheDir
		}
	}
	
	// Fall back to user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Last resort: use temp directory
		tempDir := filepath.Join(os.TempDir(), "mcpfusion", "cache")
		if c.logger != nil {
			c.logger.Warningf("Cannot determine home directory, using temp: %s", tempDir)
		}
		return tempDir
	}
	
	userCacheDir := filepath.Join(homeDir, ".mcpfusion", "cache")
	if c.logger != nil {
		c.logger.Debugf("Using user cache directory: %s", userCacheDir)
	}
	return userCacheDir
}

// getCacheFilePath returns the file path for a cache key
func (c *FileCache) getCacheFilePath(key string) string {
	// Sanitize key to be filesystem-safe
	safeKey := sanitizeKey(key)
	return filepath.Join(c.basePath, safeKey+".json")
}

// Get retrieves a value from the cache
func (c *FileCache) Get(key string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.logger != nil {
		c.logger.Debugf("File cache GET operation for key: %s", key)
	}
	
	filePath := c.getCacheFilePath(key)
	
	// Read file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			if c.logger != nil {
				c.logger.Debugf("Cache MISS - file not found for key: %s", key)
			}
			return nil, &CacheError{Operation: "get", Key: key, Message: "key not found"}
		}
		if c.logger != nil {
			c.logger.Errorf("Failed to read cache file for key %s: %v", key, err)
		}
		return nil, &CacheError{Operation: "get", Key: key, Message: fmt.Sprintf("read error: %v", err)}
	}
	
	// Unmarshal cache item
	var item FileCacheItem
	if err := json.Unmarshal(data, &item); err != nil {
		if c.logger != nil {
			c.logger.Errorf("Failed to unmarshal cache data for key %s: %v", key, err)
		}
		return nil, &CacheError{Operation: "get", Key: key, Message: fmt.Sprintf("unmarshal error: %v", err)}
	}
	
	// Check expiration
	if time.Now().After(item.ExpiresAt) {
		if c.logger != nil {
			c.logger.Debugf("Cache MISS - expired entry for key: %s", key)
		}
		// Remove expired file
		os.Remove(filePath)
		return nil, &CacheError{Operation: "get", Key: key, Message: "key expired"}
	}
	
	// Special handling for TokenInfo objects
	if key[:6] == "token:" {
		var tokenInfo TokenInfo
		if err := json.Unmarshal(item.Value, &tokenInfo); err != nil {
			if c.logger != nil {
				c.logger.Errorf("Failed to unmarshal TokenInfo for key %s: %v", key, err)
			}
			return nil, &CacheError{Operation: "get", Key: key, Message: fmt.Sprintf("token unmarshal error: %v", err)}
		}
		if c.logger != nil {
			c.logger.Debugf("Cache HIT - found valid token for key: %s", key)
		}
		return &tokenInfo, nil
	}
	
	// For other types, return the raw JSON
	var value interface{}
	if err := json.Unmarshal(item.Value, &value); err != nil {
		if c.logger != nil {
			c.logger.Errorf("Failed to unmarshal value for key %s: %v", key, err)
		}
		return nil, &CacheError{Operation: "get", Key: key, Message: fmt.Sprintf("value unmarshal error: %v", err)}
	}
	
	if c.logger != nil {
		c.logger.Debugf("Cache HIT - found valid entry for key: %s", key)
	}
	
	return value, nil
}

// Set stores a value in the cache with the given TTL
func (c *FileCache) Set(key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.logger != nil {
		c.logger.Debugf("File cache SET operation for key: %s (TTL: %v)", key, ttl)
	}
	
	// Marshal the value
	valueData, err := json.Marshal(value)
	if err != nil {
		if c.logger != nil {
			c.logger.Errorf("Failed to marshal value for key %s: %v", key, err)
		}
		return &CacheError{Operation: "set", Key: key, Message: fmt.Sprintf("marshal error: %v", err)}
	}
	
	// Create cache item
	item := FileCacheItem{
		Value:     valueData,
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}
	
	// Marshal cache item
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		if c.logger != nil {
			c.logger.Errorf("Failed to marshal cache item for key %s: %v", key, err)
		}
		return &CacheError{Operation: "set", Key: key, Message: fmt.Sprintf("item marshal error: %v", err)}
	}
	
	// Write to file
	filePath := c.getCacheFilePath(key)
	if err := ioutil.WriteFile(filePath, data, 0600); err != nil {
		if c.logger != nil {
			c.logger.Errorf("Failed to write cache file for key %s: %v", key, err)
		}
		return &CacheError{Operation: "set", Key: key, Message: fmt.Sprintf("write error: %v", err)}
	}
	
	if c.logger != nil {
		c.logger.Debugf("Successfully cached entry for key: %s", key)
	}
	
	return nil
}

// Delete removes a value from the cache
func (c *FileCache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.logger != nil {
		c.logger.Debugf("File cache DELETE operation for key: %s", key)
	}
	
	filePath := c.getCacheFilePath(key)
	
	if err := os.Remove(filePath); err != nil {
		if !os.IsNotExist(err) {
			if c.logger != nil {
				c.logger.Errorf("Failed to delete cache file for key %s: %v", key, err)
			}
			return &CacheError{Operation: "delete", Key: key, Message: fmt.Sprintf("delete error: %v", err)}
		}
	}
	
	if c.logger != nil {
		c.logger.Debugf("Successfully deleted cache entry for key: %s", key)
	}
	
	return nil
}

// Clear removes all values from the cache
func (c *FileCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.logger != nil {
		c.logger.Debug("Clearing all cache entries")
	}
	
	// List all cache files
	files, err := ioutil.ReadDir(c.basePath)
	if err != nil {
		if c.logger != nil {
			c.logger.Errorf("Failed to read cache directory: %v", err)
		}
		return &CacheError{Operation: "clear", Message: fmt.Sprintf("read directory error: %v", err)}
	}
	
	// Delete all JSON files
	var deleteCount int
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			filePath := filepath.Join(c.basePath, file.Name())
			if err := os.Remove(filePath); err != nil {
				if c.logger != nil {
					c.logger.Warningf("Failed to delete cache file %s: %v", file.Name(), err)
				}
			} else {
				deleteCount++
			}
		}
	}
	
	if c.logger != nil {
		c.logger.Infof("Cleared %d cache entries", deleteCount)
	}
	
	return nil
}

// Has checks if a key exists in the cache
func (c *FileCache) Has(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	filePath := c.getCacheFilePath(key)
	
	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		return false
	}
	
	// Check if not expired
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return false
	}
	
	var item FileCacheItem
	if err := json.Unmarshal(data, &item); err != nil {
		return false
	}
	
	return time.Now().Before(item.ExpiresAt)
}

// cleanup periodically removes expired cache entries
func (c *FileCache) cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for range ticker.C {
		c.cleanupExpired()
	}
}

// cleanupExpired removes expired cache files
func (c *FileCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.logger != nil {
		c.logger.Debug("Running cache cleanup")
	}
	
	files, err := ioutil.ReadDir(c.basePath)
	if err != nil {
		if c.logger != nil {
			c.logger.Errorf("Failed to read cache directory during cleanup: %v", err)
		}
		return
	}
	
	var cleanedCount int
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}
		
		filePath := filepath.Join(c.basePath, file.Name())
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			continue
		}
		
		var item FileCacheItem
		if err := json.Unmarshal(data, &item); err != nil {
			// Remove corrupted file
			os.Remove(filePath)
			cleanedCount++
			continue
		}
		
		if time.Now().After(item.ExpiresAt) {
			// Remove expired file
			os.Remove(filePath)
			cleanedCount++
		}
	}
	
	if cleanedCount > 0 && c.logger != nil {
		c.logger.Debugf("Cleaned up %d expired cache entries", cleanedCount)
	}
}

// sanitizeKey converts a cache key to a filesystem-safe name
func sanitizeKey(key string) string {
	// Replace problematic characters with underscores
	result := key
	problematicChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", " "}
	
	for _, char := range problematicChars {
		result = strings.ReplaceAll(result, char, "_")
	}
	
	// Ensure the result is not empty and not too long
	if result == "" {
		result = "default"
	}
	if len(result) > 200 {
		result = result[:200]
	}
	
	return result
}