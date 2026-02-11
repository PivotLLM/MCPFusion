/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package db

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PivotLLM/MCPFusion/db/internal"
	"go.etcd.io/bbolt"
)

// CreateAuthCode generates a new auth code for the given tenant and service.
// The code is stored in the auth_codes bucket with the specified TTL and returned
// as a 32-character hex string.
func (d *DB) CreateAuthCode(tenantHash, service string, ttl time.Duration) (string, error) {
	if err := d.checkClosed(); err != nil {
		return "", err
	}

	// Validate inputs
	if tenantHash == "" {
		return "", NewValidationError("tenantHash", tenantHash, "tenant hash cannot be empty")
	}
	if service == "" {
		return "", NewValidationError("service", service, "service cannot be empty")
	}
	if ttl <= 0 {
		return "", NewValidationError("ttl", ttl, "TTL must be positive")
	}

	// Generate 16 random bytes -> 32-char hex code
	codeBytes := make([]byte, 16)
	if _, err := rand.Read(codeBytes); err != nil {
		return "", NewDatabaseError("create_auth_code", fmt.Errorf("failed to generate random bytes: %w", err))
	}
	code := hex.EncodeToString(codeBytes)

	now := time.Now()
	data := &AuthCodeData{
		TenantHash: tenantHash,
		Service:    service,
		CreatedAt:  now,
		ExpiresAt:  now.Add(ttl),
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(internal.BucketAuthCodes))
		if bucket == nil {
			return NewDatabaseError("create_auth_code", fmt.Errorf("auth_codes bucket not found"))
		}

		dataBytes, err := json.Marshal(data)
		if err != nil {
			return NewDatabaseError("create_auth_code", fmt.Errorf("failed to marshal auth code data: %w", err))
		}

		if err := bucket.Put([]byte(code), dataBytes); err != nil {
			return NewDatabaseError("create_auth_code", fmt.Errorf("failed to store auth code: %w", err))
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	d.logger.Infof("Created auth code for service %s (expires: %s)", service, data.ExpiresAt.Format(time.RFC3339))
	return code, nil
}

// ValidateAuthCode looks up an auth code and returns the associated tenant hash and service.
// It returns an error if the code is not found or has expired. The code is not deleted;
// auth codes expire naturally after their TTL.
func (d *DB) ValidateAuthCode(code string) (string, string, error) {
	if err := d.checkClosed(); err != nil {
		return "", "", err
	}

	if code == "" {
		return "", "", NewValidationError("code", code, "auth code cannot be empty")
	}

	var data AuthCodeData

	err := d.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(internal.BucketAuthCodes))
		if bucket == nil {
			return NewDatabaseError("validate_auth_code", fmt.Errorf("auth_codes bucket not found"))
		}

		dataBytes := bucket.Get([]byte(code))
		if dataBytes == nil {
			return NewTokenError("auth_code", code, ErrTokenNotFound)
		}

		if err := json.Unmarshal(dataBytes, &data); err != nil {
			return NewDatabaseError("validate_auth_code", fmt.Errorf("failed to unmarshal auth code data: %w", err))
		}

		return nil
	})

	if err != nil {
		return "", "", err
	}

	// Check expiry
	if time.Now().After(data.ExpiresAt) {
		d.logger.Debugf("Auth code expired at %s", data.ExpiresAt.Format(time.RFC3339))
		return "", "", NewTokenError("auth_code", code, fmt.Errorf("auth code expired"))
	}

	d.logger.Debugf("Validated auth code for service %s (tenant: %s)", data.Service, data.TenantHash[:8])
	return data.TenantHash, data.Service, nil
}

// CleanupExpiredAuthCodes iterates the auth_codes bucket and deletes all expired entries.
func (d *DB) CleanupExpiredAuthCodes() error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	var deletedCount int

	err := d.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(internal.BucketAuthCodes))
		if bucket == nil {
			return NewDatabaseError("cleanup_auth_codes", fmt.Errorf("auth_codes bucket not found"))
		}

		now := time.Now()
		var expiredKeys [][]byte

		// Collect expired keys first to avoid modifying the bucket during iteration
		err := bucket.ForEach(func(k, v []byte) error {
			var data AuthCodeData
			if err := json.Unmarshal(v, &data); err != nil {
				// Delete corrupted entries as well
				d.logger.Warningf("Failed to unmarshal auth code %s, marking for deletion: %v", string(k), err)
				expiredKeys = append(expiredKeys, append([]byte{}, k...))
				return nil
			}

			if now.After(data.ExpiresAt) {
				expiredKeys = append(expiredKeys, append([]byte{}, k...))
			}
			return nil
		})
		if err != nil {
			return NewDatabaseError("cleanup_auth_codes", fmt.Errorf("failed to iterate auth codes: %w", err))
		}

		// Delete expired entries
		for _, key := range expiredKeys {
			if err := bucket.Delete(key); err != nil {
				return NewDatabaseError("cleanup_auth_codes", fmt.Errorf("failed to delete expired auth code: %w", err))
			}
			deletedCount++
		}

		return nil
	})

	if err != nil {
		return err
	}

	if deletedCount > 0 {
		d.logger.Infof("Cleaned up %d expired auth codes", deletedCount)
	} else {
		d.logger.Debugf("No expired auth codes to clean up")
	}

	return nil
}
