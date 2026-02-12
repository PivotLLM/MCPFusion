/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package db

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PivotLLM/MCPFusion/db/internal"
	"go.etcd.io/bbolt"
)

// truncateHash safely truncates a hash string for logging
func truncateHash(hash string, maxLen int) string {
	if len(hash) <= maxLen {
		return hash
	}
	return hash[:maxLen]
}

// generateUUID generates a random UUID v4 using crypto/rand
func generateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// CreateUser creates a new user with a generated UUID and the given description
func (d *DB) CreateUser(description string) (*UserMetadata, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	// Validate description
	if description == "" {
		return nil, NewValidationError("description", description, "description cannot be empty")
	}

	// Generate UUID
	userID, err := generateUUID()
	if err != nil {
		return nil, NewDatabaseError("create_user", fmt.Errorf("failed to generate UUID: %w", err))
	}

	// Create metadata
	now := time.Now()
	metadata := &UserMetadata{
		UserID:      userID,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = d.db.Update(func(tx *bbolt.Tx) error {
		// Get users root bucket
		usersBucket := tx.Bucket([]byte(internal.BucketUsers))
		if usersBucket == nil {
			return NewDatabaseError("create_user", fmt.Errorf("users bucket not found"))
		}

		// Check if user already exists (collision check)
		if usersBucket.Bucket([]byte(userID)) != nil {
			return NewDatabaseError("create_user", ErrUserExists)
		}

		// Create the user sub-bucket
		userBucket, err := usersBucket.CreateBucket([]byte(userID))
		if err != nil {
			return NewDatabaseError("create_user", fmt.Errorf("failed to create user bucket: %w", err))
		}

		// Create api_keys sub-bucket
		if _, err := userBucket.CreateBucket([]byte(internal.BucketUserAPIKeys)); err != nil {
			return NewDatabaseError("create_user", fmt.Errorf("failed to create api_keys bucket: %w", err))
		}

		// Create knowledge sub-bucket
		if _, err := userBucket.CreateBucket([]byte(internal.BucketUserKnowledge)); err != nil {
			return NewDatabaseError("create_user", fmt.Errorf("failed to create knowledge bucket: %w", err))
		}

		// Store metadata as JSON under the "metadata" key
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return NewDatabaseError("create_user", fmt.Errorf("failed to marshal metadata: %w", err))
		}

		if err := userBucket.Put([]byte(internal.KeyUserMetadata), metadataBytes); err != nil {
			return NewDatabaseError("create_user", fmt.Errorf("failed to store metadata: %w", err))
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	d.logger.Infof("Created user %s (%s)", userID, description)
	return metadata, nil
}

// GetUser retrieves user metadata by user ID
func (d *DB) GetUser(userID string) (*UserMetadata, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	if userID == "" {
		return nil, NewValidationError("user_id", userID, "user ID cannot be empty")
	}

	var metadata *UserMetadata

	err := d.db.View(func(tx *bbolt.Tx) error {
		usersBucket := tx.Bucket([]byte(internal.BucketUsers))
		if usersBucket == nil {
			return NewDatabaseError("get_user", fmt.Errorf("users bucket not found"))
		}

		userBucket := usersBucket.Bucket([]byte(userID))
		if userBucket == nil {
			return ErrUserNotFound
		}

		metadataBytes := userBucket.Get([]byte(internal.KeyUserMetadata))
		if metadataBytes == nil {
			return NewDatabaseError("get_user", fmt.Errorf("metadata not found for user %s", userID))
		}

		metadata = &UserMetadata{}
		if err := json.Unmarshal(metadataBytes, metadata); err != nil {
			return NewDatabaseError("get_user", fmt.Errorf("failed to unmarshal metadata: %w", err))
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	d.logger.Debugf("Retrieved user %s", userID)
	return metadata, nil
}

// ListUsers returns metadata for all users
func (d *DB) ListUsers() ([]UserMetadata, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	var users []UserMetadata

	err := d.db.View(func(tx *bbolt.Tx) error {
		usersBucket := tx.Bucket([]byte(internal.BucketUsers))
		if usersBucket == nil {
			return NewDatabaseError("list_users", fmt.Errorf("users bucket not found"))
		}

		// Iterate over all keys in the users bucket
		// Sub-buckets have nil values, key/value pairs have non-nil values
		return usersBucket.ForEach(func(k, v []byte) error {
			// Sub-buckets have nil values; skip regular key/value pairs
			if v != nil {
				return nil
			}

			userBucket := usersBucket.Bucket(k)
			if userBucket == nil {
				return nil
			}

			metadataBytes := userBucket.Get([]byte(internal.KeyUserMetadata))
			if metadataBytes == nil {
				d.logger.Warningf("User %s has no metadata, skipping", string(k))
				return nil
			}

			var metadata UserMetadata
			if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
				d.logger.Warningf("Failed to unmarshal metadata for user %s: %v", string(k), err)
				return nil // Continue iteration
			}

			users = append(users, metadata)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	if users == nil {
		users = []UserMetadata{}
	}

	d.logger.Debugf("Listed %d users", len(users))
	return users, nil
}

// DeleteUser removes a user and cleans up all associated key_to_user entries
func (d *DB) DeleteUser(userID string) error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	if userID == "" {
		return NewValidationError("user_id", userID, "user ID cannot be empty")
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		usersBucket := tx.Bucket([]byte(internal.BucketUsers))
		if usersBucket == nil {
			return NewDatabaseError("delete_user", fmt.Errorf("users bucket not found"))
		}

		userBucket := usersBucket.Bucket([]byte(userID))
		if userBucket == nil {
			return ErrUserNotFound
		}

		// Collect API key hashes linked to this user for cleanup
		var linkedKeys []string
		apiKeysBucket := userBucket.Bucket([]byte(internal.BucketUserAPIKeys))
		if apiKeysBucket != nil {
			_ = apiKeysBucket.ForEach(func(k, v []byte) error {
				linkedKeys = append(linkedKeys, string(k))
				return nil
			})
		}

		// Clean up key_to_user reverse index entries
		keyToUserBucket := tx.Bucket([]byte(internal.BucketKeyToUser))
		if keyToUserBucket != nil {
			for _, keyHash := range linkedKeys {
				if err := keyToUserBucket.Delete([]byte(keyHash)); err != nil {
					d.logger.Warningf("Failed to remove key_to_user entry for key %s: %v", truncateHash(keyHash, 12), err)
				}
			}
		}

		// Delete the entire user sub-bucket
		if err := usersBucket.DeleteBucket([]byte(userID)); err != nil {
			return NewDatabaseError("delete_user", fmt.Errorf("failed to delete user bucket: %w", err))
		}

		if len(linkedKeys) > 0 {
			d.logger.Infof("Cleaned up %d key_to_user entries for deleted user %s", len(linkedKeys), userID)
		}

		return nil
	})

	if err != nil {
		return err
	}

	d.logger.Infof("Deleted user %s", userID)
	return nil
}

// LinkAPIKey associates an API key hash with a user
func (d *DB) LinkAPIKey(userID, keyHash string) error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	if userID == "" {
		return NewValidationError("user_id", userID, "user ID cannot be empty")
	}
	if keyHash == "" {
		return NewValidationError("key_hash", keyHash, "key hash cannot be empty")
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		// Verify user exists
		usersBucket := tx.Bucket([]byte(internal.BucketUsers))
		if usersBucket == nil {
			return NewDatabaseError("link_api_key", fmt.Errorf("users bucket not found"))
		}

		userBucket := usersBucket.Bucket([]byte(userID))
		if userBucket == nil {
			return ErrUserNotFound
		}

		// Check if key is already linked to a user
		keyToUserBucket := tx.Bucket([]byte(internal.BucketKeyToUser))
		if keyToUserBucket == nil {
			return NewDatabaseError("link_api_key", fmt.Errorf("key_to_user bucket not found"))
		}

		if existing := keyToUserBucket.Get([]byte(keyHash)); existing != nil {
			return ErrKeyAlreadyLinked
		}

		// Verify the key hash exists in the api_tokens bucket
		tokensBucket := tx.Bucket([]byte(internal.BucketAPITokens))
		if tokensBucket == nil {
			return NewDatabaseError("link_api_key", fmt.Errorf("api_tokens bucket not found"))
		}

		if tokensBucket.Get([]byte(keyHash)) == nil {
			return NewTokenError("api", keyHash, ErrTokenNotFound)
		}

		// Set key_to_user/{keyHash} -> userID
		if err := keyToUserBucket.Put([]byte(keyHash), []byte(userID)); err != nil {
			return NewDatabaseError("link_api_key", fmt.Errorf("failed to store key_to_user mapping: %w", err))
		}

		// Create entry in users/{userID}/api_keys/{keyHash}
		apiKeysBucket := userBucket.Bucket([]byte(internal.BucketUserAPIKeys))
		if apiKeysBucket == nil {
			return NewDatabaseError("link_api_key", fmt.Errorf("api_keys bucket not found for user %s", userID))
		}

		if err := apiKeysBucket.Put([]byte(keyHash), []byte{}); err != nil {
			return NewDatabaseError("link_api_key", fmt.Errorf("failed to store api_key entry: %w", err))
		}

		return nil
	})

	if err != nil {
		return err
	}

	d.logger.Infof("Linked API key %s to user %s", truncateHash(keyHash, 12), userID)
	return nil
}

// UnlinkAPIKey removes the association between an API key hash and its user
func (d *DB) UnlinkAPIKey(keyHash string) error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	if keyHash == "" {
		return NewValidationError("key_hash", keyHash, "key hash cannot be empty")
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		// Look up the user for this key
		keyToUserBucket := tx.Bucket([]byte(internal.BucketKeyToUser))
		if keyToUserBucket == nil {
			return NewDatabaseError("unlink_api_key", fmt.Errorf("key_to_user bucket not found"))
		}

		userIDBytes := keyToUserBucket.Get([]byte(keyHash))
		if userIDBytes == nil {
			return ErrUserNotFound
		}
		userID := string(userIDBytes)

		// Delete key_to_user/{keyHash}
		if err := keyToUserBucket.Delete([]byte(keyHash)); err != nil {
			return NewDatabaseError("unlink_api_key", fmt.Errorf("failed to delete key_to_user mapping: %w", err))
		}

		// Delete users/{userID}/api_keys/{keyHash}
		usersBucket := tx.Bucket([]byte(internal.BucketUsers))
		if usersBucket == nil {
			return NewDatabaseError("unlink_api_key", fmt.Errorf("users bucket not found"))
		}

		userBucket := usersBucket.Bucket([]byte(userID))
		if userBucket != nil {
			apiKeysBucket := userBucket.Bucket([]byte(internal.BucketUserAPIKeys))
			if apiKeysBucket != nil {
				if err := apiKeysBucket.Delete([]byte(keyHash)); err != nil {
					d.logger.Warningf("Failed to delete api_key entry for key %s: %v", truncateHash(keyHash, 12), err)
				}
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	d.logger.Infof("Unlinked API key %s", truncateHash(keyHash, 12))
	return nil
}

// GetUserByAPIKey looks up the user ID associated with an API key hash
func (d *DB) GetUserByAPIKey(keyHash string) (string, error) {
	if err := d.checkClosed(); err != nil {
		return "", err
	}

	if keyHash == "" {
		return "", NewValidationError("key_hash", keyHash, "key hash cannot be empty")
	}

	var userID string

	err := d.db.View(func(tx *bbolt.Tx) error {
		keyToUserBucket := tx.Bucket([]byte(internal.BucketKeyToUser))
		if keyToUserBucket == nil {
			return NewDatabaseError("get_user_by_api_key", fmt.Errorf("key_to_user bucket not found"))
		}

		userIDBytes := keyToUserBucket.Get([]byte(keyHash))
		if userIDBytes == nil {
			return ErrUserNotFound
		}

		userID = string(userIDBytes)
		return nil
	})

	if err != nil {
		return "", err
	}

	d.logger.Debugf("Resolved API key %s to user %s", truncateHash(keyHash, 12), userID)
	return userID, nil
}

// AutoMigrateKeys iterates all API tokens and creates users for any that are
// not yet linked via key_to_user. This should be called on server startup to
// ensure backward compatibility with tokens created before user management.
func (d *DB) AutoMigrateKeys() error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	// Collect token hashes that need migration
	var unmigrated []string

	err := d.db.View(func(tx *bbolt.Tx) error {
		tokensBucket := tx.Bucket([]byte(internal.BucketAPITokens))
		if tokensBucket == nil {
			return NewDatabaseError("auto_migrate_keys", fmt.Errorf("api_tokens bucket not found"))
		}

		keyToUserBucket := tx.Bucket([]byte(internal.BucketKeyToUser))
		if keyToUserBucket == nil {
			return NewDatabaseError("auto_migrate_keys", fmt.Errorf("key_to_user bucket not found"))
		}

		return tokensBucket.ForEach(func(k, v []byte) error {
			hash := string(k)
			if keyToUserBucket.Get([]byte(hash)) == nil {
				unmigrated = append(unmigrated, hash)
			}
			return nil
		})
	})

	if err != nil {
		return err
	}

	if len(unmigrated) == 0 {
		d.logger.Debugf("No API keys require migration")
		return nil
	}

	d.logger.Infof("Found %d API key(s) requiring user migration", len(unmigrated))

	// Migrate each key: create a user and link the key
	for _, hash := range unmigrated {
		truncated := truncateHash(hash, 12)

		description := fmt.Sprintf("Auto-migrated from token %s", truncated)

		user, err := d.CreateUser(description)
		if err != nil {
			d.logger.Warningf("Failed to create user for key %s: %v", truncated, err)
			continue
		}

		if err := d.LinkAPIKey(user.UserID, hash); err != nil {
			d.logger.Warningf("Failed to link key %s to user %s: %v", truncated, user.UserID, err)
			// Clean up the orphaned user
			if delErr := d.DeleteUser(user.UserID); delErr != nil {
				d.logger.Warningf("Failed to clean up orphaned user %s: %v", user.UserID, delErr)
			}
			continue
		}

		d.logger.Infof("Migrated key %s to new user %s", truncated, user.UserID)
	}

	return nil
}
