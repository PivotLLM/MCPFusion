/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package db

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/PivotLLM/MCPFusion/db/internal"
	"go.etcd.io/bbolt"
)

// GetTenantInfo retrieves information about a specific tenant
func (d *DB) GetTenantInfo(hash string) (*TenantInfo, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	// Validate hash format
	if err := internal.ValidateHash(hash); err != nil {
		return nil, NewValidationError("hash", hash, err.Error())
	}

	var tenantInfo *TenantInfo

	err := d.db.View(func(tx *bbolt.Tx) error {
		// Get tenants bucket
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket == nil {
			return NewDatabaseError("get_tenant_info", fmt.Errorf("tenants bucket not found"))
		}

		// Check if tenant bucket exists
		tenantBucket := tenantsBucket.Bucket([]byte(hash))
		if tenantBucket == nil {
			return NewDatabaseError("get_tenant_info", ErrTenantNotFound)
		}

		// Initialize tenant info
		tenantInfo = &TenantInfo{
			Hash: hash,
		}

		// Check for tenant metadata (if exists)
		metadataBytes := tenantBucket.Get([]byte(internal.KeyMetadata))
		if metadataBytes != nil {
			var metadata map[string]interface{}
			if err := json.Unmarshal(metadataBytes, &metadata); err == nil {
				if desc, ok := metadata["description"].(string); ok {
					tenantInfo.Description = desc
				}
				if createdStr, ok := metadata["created_at"].(string); ok {
					if created, err := time.Parse(time.RFC3339, createdStr); err == nil {
						tenantInfo.CreatedAt = created
					}
				}
				if lastUsedStr, ok := metadata["last_used"].(string); ok {
					if lastUsed, err := time.Parse(time.RFC3339, lastUsedStr); err == nil {
						tenantInfo.LastUsed = lastUsed
					}
				}
			}
		}

		// If no tenant description, check if there's an API token with this hash
		if tenantInfo.Description == "" {
			apiTokensBucket := tx.Bucket([]byte(internal.BucketAPITokens))
			if apiTokensBucket != nil {
				if tokenMetadataBytes := apiTokensBucket.Get([]byte(hash)); tokenMetadataBytes != nil {
					var tokenMetadata APITokenMetadata
					if err := json.Unmarshal(tokenMetadataBytes, &tokenMetadata); err == nil {
						tenantInfo.Description = tokenMetadata.Description
						tenantInfo.CreatedAt = tokenMetadata.CreatedAt
						tenantInfo.LastUsed = tokenMetadata.LastUsed
					}
				}
			}
		}

		// If no metadata found, use current time as placeholder
		if tenantInfo.CreatedAt.IsZero() {
			tenantInfo.CreatedAt = time.Now()
		}
		if tenantInfo.LastUsed.IsZero() {
			tenantInfo.LastUsed = time.Now()
		}

		// Count OAuth tokens
		oauthBucket := tenantBucket.Bucket([]byte(internal.BucketOAuthTokens))
		if oauthBucket != nil {
			oauthCount := 0
			if err := oauthBucket.ForEach(func(k, v []byte) error {
				oauthCount++
				return nil
			}); err != nil {
				d.logger.Warningf("Failed to iterate OAuth tokens for tenant %s: %v", hash[:12], err)
				// Continue with partial data - don't fail the entire operation
			}
			tenantInfo.OAuthCount = oauthCount
		}

		// Count service credentials
		credentialsBucket := tenantBucket.Bucket([]byte(internal.BucketServiceCredentials))
		if credentialsBucket != nil {
			credCount := 0
			if err := credentialsBucket.ForEach(func(k, v []byte) error {
				credCount++
				return nil
			}); err != nil {
				d.logger.Warningf("Failed to iterate credentials for tenant %s: %v", hash[:12], err)
				// Continue with partial data - don't fail the entire operation
			}
			tenantInfo.CredCount = credCount
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	d.logger.Debugf("Retrieved tenant info for %s (OAuth: %d, Creds: %d)", hash[:12], tenantInfo.OAuthCount, tenantInfo.CredCount)
	return tenantInfo, nil
}

// ListTenants returns information about all tenants in the database
func (d *DB) ListTenants() ([]TenantInfo, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	var tenants []TenantInfo

	err := d.db.View(func(tx *bbolt.Tx) error {
		// Get tenants bucket
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket == nil {
			return NewDatabaseError("list_tenants", fmt.Errorf("tenants bucket not found"))
		}

		// Iterate through all tenant buckets
		return tenantsBucket.ForEach(func(k, v []byte) error {
			// Skip if this is not a bucket (it's a direct API token)
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

			// Create tenant info
			tenantInfo := TenantInfo{
				Hash: tenantHash,
			}

			// Get tenant metadata if available
			metadataBytes := tenantBucket.Get([]byte(internal.KeyMetadata))
			if metadataBytes != nil {
				var metadata map[string]interface{}
				if err := json.Unmarshal(metadataBytes, &metadata); err == nil {
					if desc, ok := metadata["description"].(string); ok {
						tenantInfo.Description = desc
					}
					if createdStr, ok := metadata["created_at"].(string); ok {
						if created, err := time.Parse(time.RFC3339, createdStr); err == nil {
							tenantInfo.CreatedAt = created
						}
					}
					if lastUsedStr, ok := metadata["last_used"].(string); ok {
						if lastUsed, err := time.Parse(time.RFC3339, lastUsedStr); err == nil {
							tenantInfo.LastUsed = lastUsed
						}
					}
				}
			}

			// If no tenant description, check if there's an API token with this hash
			if tenantInfo.Description == "" {
				apiTokensBucket := tx.Bucket([]byte(internal.BucketAPITokens))
				if apiTokensBucket != nil {
					if tokenMetadataBytes := apiTokensBucket.Get([]byte(tenantHash)); tokenMetadataBytes != nil {
						var tokenMetadata APITokenMetadata
						if err := json.Unmarshal(tokenMetadataBytes, &tokenMetadata); err == nil {
							tenantInfo.Description = tokenMetadata.Description
							tenantInfo.CreatedAt = tokenMetadata.CreatedAt
							tenantInfo.LastUsed = tokenMetadata.LastUsed
						}
					}
				}
			}

			// Set defaults if metadata not found
			if tenantInfo.CreatedAt.IsZero() {
				tenantInfo.CreatedAt = time.Now()
			}
			if tenantInfo.LastUsed.IsZero() {
				tenantInfo.LastUsed = time.Now()
			}

			// Count OAuth tokens
			oauthBucket := tenantBucket.Bucket([]byte(internal.BucketOAuthTokens))
			if oauthBucket != nil {
				oauthCount := 0
				if err := oauthBucket.ForEach(func(k, v []byte) error {
					oauthCount++
					return nil
				}); err != nil {
					d.logger.Warningf("Failed to iterate OAuth tokens for tenant %s: %v", tenantHash[:12], err)
					// Continue with partial data - don't fail the entire operation
				}
				tenantInfo.OAuthCount = oauthCount
			}

			// Count service credentials
			credentialsBucket := tenantBucket.Bucket([]byte(internal.BucketServiceCredentials))
			if credentialsBucket != nil {
				credCount := 0
				if err := credentialsBucket.ForEach(func(k, v []byte) error {
					credCount++
					return nil
				}); err != nil {
					d.logger.Warningf("Failed to iterate credentials for tenant %s: %v", tenantHash[:12], err)
					// Continue with partial data - don't fail the entire operation
				}
				tenantInfo.CredCount = credCount
			}

			tenants = append(tenants, tenantInfo)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	d.logger.Debugf("Listed %d tenants", len(tenants))
	return tenants, nil
}

// UpdateTenantMetadata updates metadata for a tenant
func (d *DB) UpdateTenantMetadata(tenantHash, description string) error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	// Validate inputs
	if err := internal.ValidateHash(tenantHash); err != nil {
		return NewValidationError("tenant_hash", tenantHash, err.Error())
	}

	if err := internal.ValidateDescription(description); err != nil {
		return NewValidationError("description", description, err.Error())
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		// Get tenants bucket
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket == nil {
			return NewDatabaseError("update_tenant_metadata", fmt.Errorf("tenants bucket not found"))
		}

		// Get or create tenant bucket
		tenantBucket, err := tenantsBucket.CreateBucketIfNotExists([]byte(tenantHash))
		if err != nil {
			return NewDatabaseError("update_tenant_metadata", fmt.Errorf("failed to create tenant bucket: %w", err))
		}

		// Get existing metadata or create new
		var metadata map[string]interface{}
		metadataBytes := tenantBucket.Get([]byte(internal.KeyMetadata))
		if metadataBytes != nil {
			if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
				// If unmarshal fails, start with fresh metadata
				metadata = make(map[string]interface{})
			}
		} else {
			metadata = make(map[string]interface{})
		}

		// Update metadata
		now := time.Now()
		metadata["description"] = description
		metadata["updated_at"] = now.Format(time.RFC3339)

		// Set created_at if not already set
		if _, exists := metadata["created_at"]; !exists {
			metadata["created_at"] = now.Format(time.RFC3339)
		}

		// Marshal and store updated metadata
		updatedBytes, err := json.Marshal(metadata)
		if err != nil {
			return NewDatabaseError("update_tenant_metadata", fmt.Errorf("failed to marshal metadata: %w", err))
		}

		if err := tenantBucket.Put([]byte(internal.KeyMetadata), updatedBytes); err != nil {
			return NewDatabaseError("update_tenant_metadata", fmt.Errorf("failed to store metadata: %w", err))
		}

		return nil
	})

	if err != nil {
		return err
	}

	d.logger.Infof("Updated metadata for tenant %s", tenantHash[:12])
	return nil
}

// UpdateTenantLastUsed updates the last used timestamp for a tenant
func (d *DB) UpdateTenantLastUsed(tenantHash string) error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	// Validate input
	if err := internal.ValidateHash(tenantHash); err != nil {
		return NewValidationError("tenant_hash", tenantHash, err.Error())
	}

	// Run this asynchronously to avoid impacting performance
	go func() {
		err := d.db.Update(func(tx *bbolt.Tx) error {
			// Get tenants bucket
			tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
			if tenantsBucket == nil {
				return fmt.Errorf("tenants bucket not found")
			}

			// Get tenant bucket
			tenantBucket := tenantsBucket.Bucket([]byte(tenantHash))
			if tenantBucket == nil {
				return fmt.Errorf("tenant not found")
			}

			// Get existing metadata or create new
			var metadata map[string]interface{}
			metadataBytes := tenantBucket.Get([]byte(internal.KeyMetadata))
			if metadataBytes != nil {
				if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
					metadata = make(map[string]interface{})
				}
			} else {
				metadata = make(map[string]interface{})
			}

			// Update last used timestamp
			now := time.Now()
			metadata["last_used"] = now.Format(time.RFC3339)

			// Set created_at if not already set
			if _, exists := metadata["created_at"]; !exists {
				metadata["created_at"] = now.Format(time.RFC3339)
			}

			// Marshal and store updated metadata
			updatedBytes, err := json.Marshal(metadata)
			if err != nil {
				return fmt.Errorf("failed to marshal metadata: %w", err)
			}

			return tenantBucket.Put([]byte(internal.KeyMetadata), updatedBytes)
		})

		if err != nil {
			d.logger.Warningf("Failed to update tenant last used timestamp: %v", err)
		}
	}()

	return nil
}

// DeleteTenant removes a tenant and all associated data
func (d *DB) DeleteTenant(tenantHash string) error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	// Validate input
	if err := internal.ValidateHash(tenantHash); err != nil {
		return NewValidationError("tenant_hash", tenantHash, err.Error())
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		// Get tenants bucket
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket == nil {
			return NewDatabaseError("delete_tenant", fmt.Errorf("tenants bucket not found"))
		}

		// Check if tenant exists
		tenantBucket := tenantsBucket.Bucket([]byte(tenantHash))
		if tenantBucket == nil {
			return NewDatabaseError("delete_tenant", ErrTenantNotFound)
		}

		// Count items for logging
		oauthCount := 0
		credCount := 0

		oauthBucket := tenantBucket.Bucket([]byte(internal.BucketOAuthTokens))
		if oauthBucket != nil {
			if err := oauthBucket.ForEach(func(k, v []byte) error {
				oauthCount++
				return nil
			}); err != nil {
				d.logger.Warningf("Failed to count OAuth tokens for tenant deletion %s: %v", tenantHash[:12], err)
				// Continue with deletion - count may be inaccurate in logs
			}
		}

		credentialsBucket := tenantBucket.Bucket([]byte(internal.BucketServiceCredentials))
		if credentialsBucket != nil {
			if err := credentialsBucket.ForEach(func(k, v []byte) error {
				credCount++
				return nil
			}); err != nil {
				d.logger.Warningf("Failed to count credentials for tenant deletion %s: %v", tenantHash[:12], err)
				// Continue with deletion - count may be inaccurate in logs
			}
		}

		// Delete the entire tenant bucket
		if err := tenantsBucket.DeleteBucket([]byte(tenantHash)); err != nil {
			return NewDatabaseError("delete_tenant", fmt.Errorf("failed to delete tenant bucket: %w", err))
		}

		d.logger.Infof("Deleted tenant %s (OAuth tokens: %d, credentials: %d)", tenantHash[:12], oauthCount, credCount)
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

// TenantExists checks if a tenant exists in the database
func (d *DB) TenantExists(tenantHash string) (bool, error) {
	if err := d.checkClosed(); err != nil {
		return false, err
	}

	// Validate input
	if err := internal.ValidateHash(tenantHash); err != nil {
		return false, NewValidationError("tenant_hash", tenantHash, err.Error())
	}

	var exists bool

	err := d.db.View(func(tx *bbolt.Tx) error {
		// Get tenants bucket
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket == nil {
			return NewDatabaseError("tenant_exists", fmt.Errorf("tenants bucket not found"))
		}

		// Check if tenant bucket exists
		exists = tenantsBucket.Bucket([]byte(tenantHash)) != nil
		return nil
	})

	if err != nil {
		return false, err
	}

	return exists, nil
}

// GetTenantResourceCount returns the total count of resources for a tenant
func (d *DB) GetTenantResourceCount(tenantHash string) (int, int, error) {
	if err := d.checkClosed(); err != nil {
		return 0, 0, err
	}

	// Validate input
	if err := internal.ValidateHash(tenantHash); err != nil {
		return 0, 0, NewValidationError("tenant_hash", tenantHash, err.Error())
	}

	var oauthCount, credCount int

	err := d.db.View(func(tx *bbolt.Tx) error {
		// Get tenants bucket
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket == nil {
			return NewDatabaseError("get_tenant_resource_count", fmt.Errorf("tenants bucket not found"))
		}

		// Get tenant bucket
		tenantBucket := tenantsBucket.Bucket([]byte(tenantHash))
		if tenantBucket == nil {
			return NewDatabaseError("get_tenant_resource_count", ErrTenantNotFound)
		}

		// Count OAuth tokens
		oauthBucket := tenantBucket.Bucket([]byte(internal.BucketOAuthTokens))
		if oauthBucket != nil {
			if err := oauthBucket.ForEach(func(k, v []byte) error {
				oauthCount++
				return nil
			}); err != nil {
				return NewDatabaseError("get_tenant_resource_count", fmt.Errorf("failed to iterate OAuth tokens: %w", err))
			}
		}

		// Count service credentials
		credentialsBucket := tenantBucket.Bucket([]byte(internal.BucketServiceCredentials))
		if credentialsBucket != nil {
			if err := credentialsBucket.ForEach(func(k, v []byte) error {
				credCount++
				return nil
			}); err != nil {
				return NewDatabaseError("get_tenant_resource_count", fmt.Errorf("failed to iterate credentials: %w", err))
			}
		}

		return nil
	})

	if err != nil {
		return 0, 0, err
	}

	return oauthCount, credCount, nil
}
