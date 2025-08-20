/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package db

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/PivotLLM/MCPFusion/db/internal"
	"go.etcd.io/bbolt"
)

// StoreOAuthToken stores OAuth token data for a tenant and service
func (d *DB) StoreOAuthToken(tenantHash, serviceName string, tokenData *OAuthTokenData) error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	// Validate inputs
	if err := internal.ValidateHash(tenantHash); err != nil {
		return NewValidationError("tenant_hash", tenantHash, err.Error())
	}

	if err := internal.ValidateServiceName(serviceName); err != nil {
		return NewValidationError("service_name", serviceName, err.Error())
	}

	if tokenData == nil {
		return NewValidationError("token_data", nil, "token data cannot be nil")
	}

	if tokenData.AccessToken == "" {
		return NewValidationError("access_token", "", "access token cannot be empty")
	}

	// Set timestamps
	now := time.Now()
	tokenData.UpdatedAt = now
	if tokenData.CreatedAt.IsZero() {
		tokenData.CreatedAt = now
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		// Get or create tenant bucket
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket == nil {
			return NewDatabaseError("store_oauth_token", fmt.Errorf("tenants bucket not found"))
		}

		tenantBucket, err := tenantsBucket.CreateBucketIfNotExists([]byte(tenantHash))
		if err != nil {
			return NewDatabaseErrorWithContext("store_oauth_token", fmt.Errorf("failed to create tenant bucket: %w", err), tenantHash, serviceName)
		}

		// Get or create OAuth tokens bucket for this tenant
		oauthBucket, err := tenantBucket.CreateBucketIfNotExists([]byte(internal.BucketOAuthTokens))
		if err != nil {
			return NewDatabaseErrorWithContext("store_oauth_token", fmt.Errorf("failed to create oauth bucket: %w", err), tenantHash, serviceName)
		}

		// Marshal token data
		tokenBytes, err := json.Marshal(tokenData)
		if err != nil {
			return NewDatabaseErrorWithContext("store_oauth_token", fmt.Errorf("failed to marshal token data: %w", err), tenantHash, serviceName)
		}

		// Store token data
		if err := oauthBucket.Put([]byte(serviceName), tokenBytes); err != nil {
			return NewDatabaseErrorWithContext("store_oauth_token", fmt.Errorf("failed to store token: %w", err), tenantHash, serviceName)
		}

		return nil
	})

	if err != nil {
		return err
	}

	d.logger.Infof("Stored OAuth token for tenant %s service %s", tenantHash[:12], serviceName)
	return nil
}

// GetOAuthToken retrieves OAuth token data for a tenant and service
func (d *DB) GetOAuthToken(tenantHash, serviceName string) (*OAuthTokenData, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	// Validate inputs
	if err := internal.ValidateHash(tenantHash); err != nil {
		return nil, NewValidationError("tenant_hash", tenantHash, err.Error())
	}

	if err := internal.ValidateServiceName(serviceName); err != nil {
		return nil, NewValidationError("service_name", serviceName, err.Error())
	}

	var tokenData *OAuthTokenData

	err := d.db.View(func(tx *bbolt.Tx) error {
		// Get tenant bucket
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket == nil {
			return NewDatabaseError("get_oauth_token", fmt.Errorf("tenants bucket not found"))
		}

		tenantBucket := tenantsBucket.Bucket([]byte(tenantHash))
		if tenantBucket == nil {
			return NewDatabaseErrorWithContext("get_oauth_token", ErrTenantNotFound, tenantHash, serviceName)
		}

		// Get OAuth tokens bucket
		oauthBucket := tenantBucket.Bucket([]byte(internal.BucketOAuthTokens))
		if oauthBucket == nil {
			return NewDatabaseErrorWithContext("get_oauth_token", ErrServiceNotFound, tenantHash, serviceName)
		}

		// Get token data
		tokenBytes := oauthBucket.Get([]byte(serviceName))
		if tokenBytes == nil {
			return NewDatabaseErrorWithContext("get_oauth_token", ErrServiceNotFound, tenantHash, serviceName)
		}

		// Unmarshal token data
		tokenData = &OAuthTokenData{}
		if err := json.Unmarshal(tokenBytes, tokenData); err != nil {
			return NewDatabaseErrorWithContext("get_oauth_token", fmt.Errorf("failed to unmarshal token data: %w", err), tenantHash, serviceName)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	d.logger.Debugf("Retrieved OAuth token for tenant %s service %s", tenantHash[:12], serviceName)
	return tokenData, nil
}

// DeleteOAuthToken removes OAuth token data for a tenant and service
func (d *DB) DeleteOAuthToken(tenantHash, serviceName string) error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	// Validate inputs
	if err := internal.ValidateHash(tenantHash); err != nil {
		return NewValidationError("tenant_hash", tenantHash, err.Error())
	}

	if err := internal.ValidateServiceName(serviceName); err != nil {
		return NewValidationError("service_name", serviceName, err.Error())
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		// Get tenant bucket
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket == nil {
			return NewDatabaseError("delete_oauth_token", fmt.Errorf("tenants bucket not found"))
		}

		tenantBucket := tenantsBucket.Bucket([]byte(tenantHash))
		if tenantBucket == nil {
			return NewDatabaseErrorWithContext("delete_oauth_token", ErrTenantNotFound, tenantHash, serviceName)
		}

		// Get OAuth tokens bucket
		oauthBucket := tenantBucket.Bucket([]byte(internal.BucketOAuthTokens))
		if oauthBucket == nil {
			return NewDatabaseErrorWithContext("delete_oauth_token", ErrServiceNotFound, tenantHash, serviceName)
		}

		// Check if token exists
		if oauthBucket.Get([]byte(serviceName)) == nil {
			return NewDatabaseErrorWithContext("delete_oauth_token", ErrServiceNotFound, tenantHash, serviceName)
		}

		// Delete token
		if err := oauthBucket.Delete([]byte(serviceName)); err != nil {
			return NewDatabaseErrorWithContext("delete_oauth_token", fmt.Errorf("failed to delete token: %w", err), tenantHash, serviceName)
		}

		return nil
	})

	if err != nil {
		return err
	}

	d.logger.Infof("Deleted OAuth token for tenant %s service %s", tenantHash[:12], serviceName)
	return nil
}

// ListOAuthTokens returns all OAuth tokens for a tenant
func (d *DB) ListOAuthTokens(tenantHash string) (map[string]*OAuthTokenData, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	// Validate input
	if err := internal.ValidateHash(tenantHash); err != nil {
		return nil, NewValidationError("tenant_hash", tenantHash, err.Error())
	}

	tokens := make(map[string]*OAuthTokenData)

	err := d.db.View(func(tx *bbolt.Tx) error {
		// Get tenant bucket
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket == nil {
			return NewDatabaseError("list_oauth_tokens", fmt.Errorf("tenants bucket not found"))
		}

		tenantBucket := tenantsBucket.Bucket([]byte(tenantHash))
		if tenantBucket == nil {
			// No tenant bucket means no tokens - return empty map
			return nil
		}

		// Get OAuth tokens bucket
		oauthBucket := tenantBucket.Bucket([]byte(internal.BucketOAuthTokens))
		if oauthBucket == nil {
			// No OAuth bucket means no tokens - return empty map
			return nil
		}

		// Iterate through all OAuth tokens
		return oauthBucket.ForEach(func(k, v []byte) error {
			serviceName := string(k)

			var tokenData OAuthTokenData
			if err := json.Unmarshal(v, &tokenData); err != nil {
				d.logger.Warningf("Failed to unmarshal OAuth token for service %s: %v", serviceName, err)
				return nil // Continue iteration
			}

			tokens[serviceName] = &tokenData
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	d.logger.Debugf("Listed %d OAuth tokens for tenant %s", len(tokens), tenantHash[:12])
	return tokens, nil
}

// RefreshOAuthToken updates an OAuth token's access token and expiration
// This is a convenience method for token refresh operations
func (d *DB) RefreshOAuthToken(tenantHash, serviceName, newAccessToken string, expiresAt *time.Time) error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	// Validate inputs
	if err := internal.ValidateHash(tenantHash); err != nil {
		return NewValidationError("tenant_hash", tenantHash, err.Error())
	}

	if err := internal.ValidateServiceName(serviceName); err != nil {
		return NewValidationError("service_name", serviceName, err.Error())
	}

	if newAccessToken == "" {
		return NewValidationError("access_token", "", "access token cannot be empty")
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		// Get tenant bucket
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket == nil {
			return NewDatabaseError("refresh_oauth_token", fmt.Errorf("tenants bucket not found"))
		}

		tenantBucket := tenantsBucket.Bucket([]byte(tenantHash))
		if tenantBucket == nil {
			return NewDatabaseErrorWithContext("refresh_oauth_token", ErrTenantNotFound, tenantHash, serviceName)
		}

		// Get OAuth tokens bucket
		oauthBucket := tenantBucket.Bucket([]byte(internal.BucketOAuthTokens))
		if oauthBucket == nil {
			return NewDatabaseErrorWithContext("refresh_oauth_token", ErrServiceNotFound, tenantHash, serviceName)
		}

		// Get existing token data
		tokenBytes := oauthBucket.Get([]byte(serviceName))
		if tokenBytes == nil {
			return NewDatabaseErrorWithContext("refresh_oauth_token", ErrServiceNotFound, tenantHash, serviceName)
		}

		var tokenData OAuthTokenData
		if err := json.Unmarshal(tokenBytes, &tokenData); err != nil {
			return NewDatabaseErrorWithContext("refresh_oauth_token", fmt.Errorf("failed to unmarshal existing token data: %w", err), tenantHash, serviceName)
		}

		// Update token data
		tokenData.AccessToken = newAccessToken
		tokenData.ExpiresAt = expiresAt
		tokenData.UpdatedAt = time.Now()

		// Marshal and store updated data
		updatedBytes, err := json.Marshal(&tokenData)
		if err != nil {
			return NewDatabaseErrorWithContext("refresh_oauth_token", fmt.Errorf("failed to marshal updated token data: %w", err), tenantHash, serviceName)
		}

		if err := oauthBucket.Put([]byte(serviceName), updatedBytes); err != nil {
			return NewDatabaseErrorWithContext("refresh_oauth_token", fmt.Errorf("failed to store updated token: %w", err), tenantHash, serviceName)
		}

		return nil
	})

	if err != nil {
		return err
	}

	d.logger.Infof("Refreshed OAuth token for tenant %s service %s", tenantHash[:12], serviceName)
	return nil
}
