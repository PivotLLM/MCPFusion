/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/PivotLLM/MCPFusion/db/internal"
	"go.etcd.io/bbolt"
)

// GetStats returns comprehensive statistics about the database
func (d *DB) GetStats() (*TokenStats, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	var stats *TokenStats

	err := d.db.View(func(tx *bbolt.Tx) error {
		stats = &TokenStats{
			LastUpdated: time.Now(),
		}

		// Count API tokens
		apiTokensBucket := tx.Bucket([]byte(internal.BucketAPITokens))
		if apiTokensBucket != nil {
			apiTokensBucket.ForEach(func(k, v []byte) error {
				// If v is not nil, this is a direct API token (not a tenant bucket)
				if v != nil {
					stats.TotalAPITokens++
				}
				return nil
			})
		}

		// Count OAuth tokens and credentials by iterating through tenants
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket != nil {
			tenantsBucket.ForEach(func(k, v []byte) error {
				// Skip non-bucket entries
				if v != nil {
					return nil
				}

				tenantHash := string(k)

				// Validate that this looks like a tenant hash
				if err := internal.ValidateHash(tenantHash); err != nil {
					// Skip invalid hashes (might be other data)
					return nil
				}

				tenantBucket := tenantsBucket.Bucket(k)
				if tenantBucket == nil {
					return nil
				}

				// Count OAuth tokens for this tenant
				oauthBucket := tenantBucket.Bucket([]byte(internal.BucketOAuthTokens))
				if oauthBucket != nil {
					oauthBucket.ForEach(func(k, v []byte) error {
						stats.TotalOAuthTokens++
						return nil
					})
				}

				// Count credentials for this tenant
				credentialsBucket := tenantBucket.Bucket([]byte(internal.BucketServiceCredentials))
				if credentialsBucket != nil {
					credentialsBucket.ForEach(func(k, v []byte) error {
						stats.TotalCredentials++
						return nil
					})
				}

				return nil
			})
		}

		return nil
	})

	if err != nil {
		return nil, NewDatabaseError("get_stats", err)
	}

	d.logger.Debugf("Generated stats: API tokens: %d, OAuth tokens: %d, Credentials: %d",
		stats.TotalAPITokens, stats.TotalOAuthTokens, stats.TotalCredentials)

	return stats, nil
}

// GetDetailedStats returns more detailed statistics including per-tenant breakdown
func (d *DB) GetDetailedStats() (map[string]interface{}, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	var detailedStats map[string]interface{}

	err := d.db.View(func(tx *bbolt.Tx) error {
		detailedStats = make(map[string]interface{})

		// Basic stats
		totalAPITokens := int64(0)
		totalOAuthTokens := int64(0)
		totalCredentials := int64(0)
		totalTenants := int64(0)

		// Per-tenant breakdown
		tenantBreakdown := make(map[string]map[string]interface{})

		// Count API tokens
		apiTokensBucket := tx.Bucket([]byte(internal.BucketAPITokens))
		if apiTokensBucket != nil {
			apiTokensBucket.ForEach(func(k, v []byte) error {
				// If v is not nil, this is a direct API token
				if v != nil {
					totalAPITokens++
				}
				return nil
			})
		}

		// Process tenants
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket != nil {
			tenantsBucket.ForEach(func(k, v []byte) error {
				// Skip non-bucket entries
				if v != nil {
					return nil
				}

				tenantHash := string(k)

				// Validate that this looks like a tenant hash
				if err := internal.ValidateHash(tenantHash); err != nil {
					// Skip invalid hashes
					return nil
				}

				tenantBucket := tenantsBucket.Bucket(k)
				if tenantBucket == nil {
					return nil
				}

				totalTenants++

				// Initialize tenant stats
				tenantStats := map[string]interface{}{
					"oauth_tokens": int64(0),
					"credentials":  int64(0),
					"services":     make([]string, 0),
					"created_at":   "",
					"last_used":    "",
					"description":  "",
				}

				// Get tenant metadata
				metadataBytes := tenantBucket.Get([]byte(internal.KeyMetadata))
				if metadataBytes != nil {
					var metadata map[string]interface{}
					if err := json.Unmarshal(metadataBytes, &metadata); err == nil {
						if desc, ok := metadata["description"].(string); ok {
							tenantStats["description"] = desc
						}
						if createdAt, ok := metadata["created_at"].(string); ok {
							tenantStats["created_at"] = createdAt
						}
						if lastUsed, ok := metadata["last_used"].(string); ok {
							tenantStats["last_used"] = lastUsed
						}
					}
				}

				// Track unique services for this tenant
				serviceSet := make(map[string]bool)

				// Count OAuth tokens
				oauthCount := int64(0)
				oauthBucket := tenantBucket.Bucket([]byte(internal.BucketOAuthTokens))
				if oauthBucket != nil {
					oauthBucket.ForEach(func(k, v []byte) error {
						oauthCount++
						totalOAuthTokens++
						serviceSet[string(k)] = true
						return nil
					})
				}
				tenantStats["oauth_tokens"] = oauthCount

				// Count credentials
				credCount := int64(0)
				credentialsBucket := tenantBucket.Bucket([]byte(internal.BucketServiceCredentials))
				if credentialsBucket != nil {
					credentialsBucket.ForEach(func(k, v []byte) error {
						credCount++
						totalCredentials++
						serviceSet[string(k)] = true
						return nil
					})
				}
				tenantStats["credentials"] = credCount

				// Convert service set to slice
				services := make([]string, 0, len(serviceSet))
				for service := range serviceSet {
					services = append(services, service)
				}
				tenantStats["services"] = services

				// Use a truncated hash for the breakdown key
				truncatedHash := tenantHash
				if len(truncatedHash) > 12 {
					truncatedHash = truncatedHash[:12] + "..."
				}
				tenantBreakdown[truncatedHash] = tenantStats

				return nil
			})
		}

		// Assemble final stats
		detailedStats["summary"] = map[string]interface{}{
			"total_api_tokens":   totalAPITokens,
			"total_oauth_tokens": totalOAuthTokens,
			"total_credentials":  totalCredentials,
			"total_tenants":      totalTenants,
			"generated_at":       time.Now().Format(time.RFC3339),
		}
		detailedStats["tenants"] = tenantBreakdown

		// Add system info
		systemBucket := tx.Bucket([]byte(internal.BucketSystem))
		if systemBucket != nil {
			systemInfo := make(map[string]interface{})

			// Get schema version
			if schemaVersion := systemBucket.Get([]byte(internal.KeySchemaVersion)); schemaVersion != nil {
				systemInfo["schema_version"] = string(schemaVersion)
			}

			detailedStats["system"] = systemInfo
		}

		return nil
	})

	if err != nil {
		return nil, NewDatabaseError("get_detailed_stats", err)
	}

	d.logger.Debugf("Generated detailed stats for %d tenants", len(detailedStats["tenants"].(map[string]map[string]interface{})))
	return detailedStats, nil
}

// GetSystemHealth returns health information about the database
func (d *DB) GetSystemHealth() (map[string]interface{}, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	healthInfo := make(map[string]interface{})

	// Basic connectivity test
	err := d.db.View(func(tx *bbolt.Tx) error {
		// Check that core buckets exist
		healthInfo["database_accessible"] = true

		// Check schema integrity
		systemBucket := tx.Bucket([]byte(internal.BucketSystem))
		if systemBucket != nil {
			healthInfo["system_bucket_exists"] = true

			schemaVersion := systemBucket.Get([]byte(internal.KeySchemaVersion))
			if schemaVersion != nil {
				healthInfo["schema_version"] = string(schemaVersion)
				healthInfo["schema_valid"] = string(schemaVersion) == internal.SchemaVersion
			} else {
				healthInfo["schema_version"] = "unknown"
				healthInfo["schema_valid"] = false
			}
		} else {
			healthInfo["system_bucket_exists"] = false
			healthInfo["schema_valid"] = false
		}

		// Check other critical buckets
		healthInfo["api_tokens_bucket_exists"] = tx.Bucket([]byte(internal.BucketAPITokens)) != nil
		healthInfo["token_index_bucket_exists"] = tx.Bucket([]byte(internal.BucketTokenIndex)) != nil

		return nil
	})

	if err != nil {
		healthInfo["database_accessible"] = false
		healthInfo["error"] = err.Error()
	}

	// Add system metadata
	healthInfo["timestamp"] = time.Now().Format(time.RFC3339)
	healthInfo["data_directory"] = d.dataDir

	// Database file stats (if accessible)
	if d.db != nil {
		stats := d.db.Stats()
		dbStats := map[string]interface{}{
			"free_page_count":    stats.FreePageN,
			"pending_page_count": stats.PendingPageN,
			"free_alloc":         stats.FreeAlloc,
			"free_list_inuse":    stats.FreelistInuse,
			"tx_count":           stats.TxN,
			"open_tx_count":      stats.OpenTxN,
		}
		healthInfo["database_stats"] = dbStats
	}

	d.logger.Debugf("Generated system health report")
	return healthInfo, nil
}

// UpdateStoredStats updates the stored statistics in the database
// This is used for caching frequently accessed stats
func (d *DB) UpdateStoredStats() error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	// Get current stats
	stats, err := d.GetStats()
	if err != nil {
		return NewDatabaseError("update_stored_stats", fmt.Errorf("failed to generate stats: %w", err))
	}

	// Store in system bucket
	err = d.db.Update(func(tx *bbolt.Tx) error {
		systemBucket := tx.Bucket([]byte(internal.BucketSystem))
		if systemBucket == nil {
			return NewDatabaseError("update_stored_stats", fmt.Errorf("system bucket not found"))
		}

		// Marshal stats
		statsBytes, err := json.Marshal(stats)
		if err != nil {
			return NewDatabaseError("update_stored_stats", fmt.Errorf("failed to marshal stats: %w", err))
		}

		// Store stats
		if err := systemBucket.Put([]byte(internal.KeyStats), statsBytes); err != nil {
			return NewDatabaseError("update_stored_stats", fmt.Errorf("failed to store stats: %w", err))
		}

		return nil
	})

	if err != nil {
		return err
	}

	d.logger.Debugf("Updated stored statistics")
	return nil
}

// GetStoredStats retrieves cached statistics from the database
func (d *DB) GetStoredStats() (*TokenStats, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	var stats *TokenStats

	err := d.db.View(func(tx *bbolt.Tx) error {
		systemBucket := tx.Bucket([]byte(internal.BucketSystem))
		if systemBucket == nil {
			return NewDatabaseError("get_stored_stats", fmt.Errorf("system bucket not found"))
		}

		// Get stored stats
		statsBytes := systemBucket.Get([]byte(internal.KeyStats))
		if statsBytes == nil {
			// No cached stats available, generate fresh ones
			return ErrTokenNotFound
		}

		// Unmarshal stats
		stats = &TokenStats{}
		if err := json.Unmarshal(statsBytes, stats); err != nil {
			return NewDatabaseError("get_stored_stats", fmt.Errorf("failed to unmarshal stats: %w", err))
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			// No cached stats, generate fresh ones
			return d.GetStats()
		}
		return nil, err
	}

	d.logger.Debugf("Retrieved cached statistics")
	return stats, nil
}

// GetStatsWithTTL retrieves statistics, using cached version if within TTL
func (d *DB) GetStatsWithTTL(maxAge time.Duration) (*TokenStats, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	// Try to get cached stats first
	cachedStats, err := d.GetStoredStats()
	if err == nil && cachedStats != nil {
		// Check if cached stats are still fresh
		age := time.Since(cachedStats.LastUpdated)
		if age <= maxAge {
			d.logger.Debugf("Using cached stats (age: %v)", age)
			return cachedStats, nil
		}
	}

	// Cached stats are stale or unavailable, generate fresh ones
	freshStats, err := d.GetStats()
	if err != nil {
		return nil, err
	}

	// Update cached stats asynchronously
	go func() {
		if err := d.UpdateStoredStats(); err != nil {
			d.logger.Warningf("Failed to update cached stats: %v", err)
		}
	}()

	d.logger.Debugf("Generated fresh statistics")
	return freshStats, nil
}

// ClearStatsCache clears the cached statistics from the database
// This forces the next stats request to generate fresh statistics
func (d *DB) ClearStatsCache() error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		systemBucket := tx.Bucket([]byte(internal.BucketSystem))
		if systemBucket == nil {
			return NewDatabaseError("clear_stats_cache", fmt.Errorf("system bucket not found"))
		}

		// Remove cached stats
		if err := systemBucket.Delete([]byte(internal.KeyStats)); err != nil {
			return NewDatabaseError("clear_stats_cache", fmt.Errorf("failed to clear stats cache: %w", err))
		}

		return nil
	})

	if err != nil {
		return err
	}

	d.logger.Debugf("Cleared statistics cache")
	return nil
}
