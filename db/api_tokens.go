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

// AddAPIToken creates a new API token with metadata
func (d *DB) AddAPIToken(description string) (string, string, error) {
	if err := d.checkClosed(); err != nil {
		return "", "", err
	}

	// Validate description
	if err := internal.ValidateDescription(description); err != nil {
		return "", "", NewValidationError("description", description, err.Error())
	}

	// Generate secure token
	token, err := d.generateSecureToken()
	if err != nil {
		return "", "", err
	}

	// Create hash and prefix
	hash := d.hashAPIToken(token)
	prefix := d.generatePrefix(token)

	// Create metadata
	now := time.Now()
	metadata := &APITokenMetadata{
		Hash:        hash,
		CreatedAt:   now,
		LastUsed:    now,
		Description: description,
		Prefix:      prefix,
	}

	// Store in database
	err = d.db.Update(func(tx *bbolt.Tx) error {
		// Get buckets
		tokensBucket := tx.Bucket([]byte(internal.BucketAPITokens))
		if tokensBucket == nil {
			return NewDatabaseError("add_api_token", fmt.Errorf("tokens bucket not found"))
		}

		indexBucket := tx.Bucket([]byte(internal.BucketTokenIndex))
		if indexBucket == nil {
			return NewDatabaseError("add_api_token", fmt.Errorf("index bucket not found"))
		}

		// Check for duplicate hash
		if tokensBucket.Get([]byte(hash)) != nil {
			return NewDatabaseError("add_api_token", ErrDuplicateToken)
		}

		// Store metadata
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return NewDatabaseError("add_api_token", fmt.Errorf("failed to marshal metadata: %w", err))
		}

		if err := tokensBucket.Put([]byte(hash), metadataBytes); err != nil {
			return NewDatabaseError("add_api_token", fmt.Errorf("failed to store metadata: %w", err))
		}

		// Update indexes
		hashIndexBucket := indexBucket.Bucket([]byte(internal.BucketIndexByHash))
		if hashIndexBucket == nil {
			return NewDatabaseError("add_api_token", fmt.Errorf("hash index bucket not found"))
		}

		prefixIndexBucket := indexBucket.Bucket([]byte(internal.BucketIndexByPrefix))
		if prefixIndexBucket == nil {
			return NewDatabaseError("add_api_token", fmt.Errorf("prefix index bucket not found"))
		}

		// Store hash index (hash -> hash for fast validation)
		if err := hashIndexBucket.Put([]byte(hash), []byte(hash)); err != nil {
			return NewDatabaseError("add_api_token", fmt.Errorf("failed to update hash index: %w", err))
		}

		// Store prefix index (prefix -> hash for identification)
		if err := prefixIndexBucket.Put([]byte(prefix), []byte(hash)); err != nil {
			return NewDatabaseError("add_api_token", fmt.Errorf("failed to update prefix index: %w", err))
		}

		return nil
	})

	if err != nil {
		return "", "", err
	}

	d.logger.Infof("Created API token with hash %s (prefix: %s)", hash[:12], prefix)
	return token, hash, nil
}

// ValidateAPIToken validates an API token and returns its hash
func (d *DB) ValidateAPIToken(token string) (bool, string, error) {
	if err := d.checkClosed(); err != nil {
		return false, "", err
	}

	// Validate token format
	if err := internal.ValidateToken(token); err != nil {
		return false, "", NewValidationError("token", token, err.Error())
	}

	// Generate hash from token
	hash := d.hashAPIToken(token)

	var valid bool
	var metadata *APITokenMetadata

	err := d.db.View(func(tx *bbolt.Tx) error {
		// Check hash index first for fast validation
		indexBucket := tx.Bucket([]byte(internal.BucketTokenIndex))
		if indexBucket == nil {
			return NewDatabaseError("validate_api_token", fmt.Errorf("index bucket not found"))
		}

		hashIndexBucket := indexBucket.Bucket([]byte(internal.BucketIndexByHash))
		if hashIndexBucket == nil {
			return NewDatabaseError("validate_api_token", fmt.Errorf("hash index bucket not found"))
		}

		// Check if hash exists in index
		if hashIndexBucket.Get([]byte(hash)) == nil {
			valid = false
			return nil
		}

		// Get full metadata from tokens bucket
		tokensBucket := tx.Bucket([]byte(internal.BucketAPITokens))
		if tokensBucket == nil {
			return NewDatabaseError("validate_api_token", fmt.Errorf("tokens bucket not found"))
		}

		metadataBytes := tokensBucket.Get([]byte(hash))
		if metadataBytes == nil {
			valid = false
			return nil
		}

		// Parse metadata
		metadata = &APITokenMetadata{}
		if err := json.Unmarshal(metadataBytes, metadata); err != nil {
			return NewDatabaseError("validate_api_token", fmt.Errorf("failed to unmarshal metadata: %w", err))
		}

		valid = true
		return nil
	})

	if err != nil {
		return false, "", err
	}

	// Update last used timestamp if valid
	if valid && metadata != nil {
		d.updateTokenLastUsed(hash)
	}

	if !valid {
		d.logger.Debugf("Invalid API token validation attempt")
		return false, "", nil
	}

	d.logger.Debugf("Valid API token validated: %s", hash[:12])
	return true, hash, nil
}

// updateTokenLastUsed updates the last used timestamp for a token (async operation)
func (d *DB) updateTokenLastUsed(hash string) {
	go func() {
		err := d.db.Update(func(tx *bbolt.Tx) error {
			tokensBucket := tx.Bucket([]byte(internal.BucketAPITokens))
			if tokensBucket == nil {
				return fmt.Errorf("tokens bucket not found")
			}

			metadataBytes := tokensBucket.Get([]byte(hash))
			if metadataBytes == nil {
				return fmt.Errorf("token not found")
			}

			var metadata APITokenMetadata
			if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
				return fmt.Errorf("failed to unmarshal metadata: %w", err)
			}

			// Update last used timestamp
			metadata.LastUsed = time.Now()

			updatedBytes, err := json.Marshal(&metadata)
			if err != nil {
				return fmt.Errorf("failed to marshal updated metadata: %w", err)
			}

			return tokensBucket.Put([]byte(hash), updatedBytes)
		})

		if err != nil {
			d.logger.Warningf("Failed to update token last used timestamp: %v", err)
		}
	}()
}

// DeleteAPIToken removes an API token by hash
func (d *DB) DeleteAPIToken(hash string) error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	// Validate hash format
	if err := internal.ValidateHash(hash); err != nil {
		return NewValidationError("hash", hash, err.Error())
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		// Get buckets
		tokensBucket := tx.Bucket([]byte(internal.BucketAPITokens))
		if tokensBucket == nil {
			return NewDatabaseError("delete_api_token", fmt.Errorf("tokens bucket not found"))
		}

		indexBucket := tx.Bucket([]byte(internal.BucketTokenIndex))
		if indexBucket == nil {
			return NewDatabaseError("delete_api_token", fmt.Errorf("index bucket not found"))
		}

		// Check if token exists and get metadata
		metadataBytes := tokensBucket.Get([]byte(hash))
		if metadataBytes == nil {
			return NewTokenError("api", hash, ErrTokenNotFound)
		}

		var metadata APITokenMetadata
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			return NewDatabaseError("delete_api_token", fmt.Errorf("failed to unmarshal metadata: %w", err))
		}

		// Delete from main bucket
		if err := tokensBucket.Delete([]byte(hash)); err != nil {
			return NewDatabaseError("delete_api_token", fmt.Errorf("failed to delete token: %w", err))
		}

		// Remove from indexes
		hashIndexBucket := indexBucket.Bucket([]byte(internal.BucketIndexByHash))
		if hashIndexBucket != nil {
			if err := hashIndexBucket.Delete([]byte(hash)); err != nil {
				d.logger.Warningf("Failed to remove hash index: %v", err)
			}
		}

		prefixIndexBucket := indexBucket.Bucket([]byte(internal.BucketIndexByPrefix))
		if prefixIndexBucket != nil {
			if err := prefixIndexBucket.Delete([]byte(metadata.Prefix)); err != nil {
				d.logger.Warningf("Failed to remove prefix index: %v", err)
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	d.logger.Infof("Deleted API token with hash %s", hash[:12])
	return nil
}

// ListAPITokens returns all API token metadata
func (d *DB) ListAPITokens() ([]APITokenMetadata, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	var tokens []APITokenMetadata

	err := d.db.View(func(tx *bbolt.Tx) error {
		tokensBucket := tx.Bucket([]byte(internal.BucketAPITokens))
		if tokensBucket == nil {
			return NewDatabaseError("list_api_tokens", fmt.Errorf("tokens bucket not found"))
		}

		return tokensBucket.ForEach(func(k, v []byte) error {
			var metadata APITokenMetadata
			if err := json.Unmarshal(v, &metadata); err != nil {
				d.logger.Warningf("Failed to unmarshal token metadata for key %s: %v", string(k), err)
				return nil // Continue iteration
			}

			tokens = append(tokens, metadata)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	d.logger.Debugf("Listed %d API tokens", len(tokens))
	return tokens, nil
}

// GetAPITokenMetadata retrieves metadata for a specific API token by hash
func (d *DB) GetAPITokenMetadata(hash string) (*APITokenMetadata, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	// Validate hash format
	if err := internal.ValidateHash(hash); err != nil {
		return nil, NewValidationError("hash", hash, err.Error())
	}

	var metadata *APITokenMetadata

	err := d.db.View(func(tx *bbolt.Tx) error {
		tokensBucket := tx.Bucket([]byte(internal.BucketAPITokens))
		if tokensBucket == nil {
			return NewDatabaseError("get_api_token_metadata", fmt.Errorf("tokens bucket not found"))
		}

		metadataBytes := tokensBucket.Get([]byte(hash))
		if metadataBytes == nil {
			return NewTokenError("api", hash, ErrTokenNotFound)
		}

		metadata = &APITokenMetadata{}
		if err := json.Unmarshal(metadataBytes, metadata); err != nil {
			return NewDatabaseError("get_api_token_metadata", fmt.Errorf("failed to unmarshal metadata: %w", err))
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	d.logger.Debugf("Retrieved API token metadata for hash %s", hash[:12])
	return metadata, nil
}

// ResolveAPIToken resolves a token identifier (hash or prefix) to its full hash
func (d *DB) ResolveAPIToken(identifier string) (string, error) {
	if err := d.checkClosed(); err != nil {
		return "", err
	}

	if identifier == "" {
		return "", NewValidationError("identifier", identifier, "identifier cannot be empty")
	}

	var resolvedHash string

	err := d.db.View(func(tx *bbolt.Tx) error {
		// First check if it's a full hash
		if len(identifier) == 64 { // SHA-256 hash length
			tokensBucket := tx.Bucket([]byte(internal.BucketAPITokens))
			if tokensBucket == nil {
				return NewDatabaseError("resolve_api_token", fmt.Errorf("tokens bucket not found"))
			}

			if tokensBucket.Get([]byte(identifier)) != nil {
				resolvedHash = identifier
				return nil
			}
		}

		// Check if it's a prefix
		indexBucket := tx.Bucket([]byte(internal.BucketTokenIndex))
		if indexBucket == nil {
			return NewDatabaseError("resolve_api_token", fmt.Errorf("index bucket not found"))
		}

		prefixIndexBucket := indexBucket.Bucket([]byte(internal.BucketIndexByPrefix))
		if prefixIndexBucket == nil {
			return NewDatabaseError("resolve_api_token", fmt.Errorf("prefix index bucket not found"))
		}

		// Look for exact prefix match
		if hashBytes := prefixIndexBucket.Get([]byte(identifier)); hashBytes != nil {
			resolvedHash = string(hashBytes)
			return nil
		}

		// If not exact prefix match, look for prefixes that start with the identifier
		var matches []string
		cursor := prefixIndexBucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			prefix := string(k)
			if len(identifier) <= len(prefix) && prefix[:len(identifier)] == identifier {
				matches = append(matches, string(v))
			}
		}

		if len(matches) == 0 {
			return NewTokenError("api", identifier, ErrTokenNotFound)
		}

		if len(matches) > 1 {
			return NewValidationError("identifier", identifier,
				fmt.Sprintf("ambiguous identifier matches %d tokens", len(matches)))
		}

		resolvedHash = matches[0]
		return nil
	})

	if err != nil {
		return "", err
	}

	if resolvedHash == "" {
		return "", NewTokenError("api", identifier, ErrTokenNotFound)
	}

	d.logger.Debugf("Resolved identifier %s to hash %s", identifier, resolvedHash[:12])
	return resolvedHash, nil
}
