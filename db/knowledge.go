/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package db

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PivotLLM/MCPFusion/db/internal"
	"go.etcd.io/bbolt"
)

// SetKnowledge creates or updates a knowledge entry for a user within a domain
func (d *DB) SetKnowledge(userID string, entry *KnowledgeEntry) error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	// Validate inputs
	if strings.TrimSpace(userID) == "" {
		return NewValidationError("user_id", userID, "user ID cannot be empty")
	}

	if entry == nil {
		return NewValidationError("entry", nil, "knowledge entry cannot be nil")
	}

	if strings.TrimSpace(entry.Domain) == "" {
		return NewValidationError("domain", entry.Domain, "domain cannot be empty")
	}

	if strings.TrimSpace(entry.Key) == "" {
		return NewValidationError("key", entry.Key, "key cannot be empty")
	}

	if strings.TrimSpace(entry.Content) == "" {
		return NewValidationError("content", entry.Content, "content cannot be empty")
	}

	now := time.Now()

	// Work on a copy to avoid mutating the caller's struct
	stored := KnowledgeEntry{
		Domain:  entry.Domain,
		Key:     entry.Key,
		Content: entry.Content,
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		// Verify user exists
		usersBucket := tx.Bucket([]byte(internal.BucketUsers))
		if usersBucket == nil {
			return NewDatabaseError("set_knowledge", fmt.Errorf("users bucket not found"))
		}

		userBucket := usersBucket.Bucket([]byte(userID))
		if userBucket == nil {
			return NewDatabaseError("set_knowledge", ErrUserNotFound)
		}

		// Get or create knowledge bucket under the user
		knowledgeBucket, err := userBucket.CreateBucketIfNotExists([]byte(internal.BucketUserKnowledge))
		if err != nil {
			return NewDatabaseError("set_knowledge", fmt.Errorf("failed to create knowledge bucket: %w", err))
		}

		// Get or create domain sub-bucket
		domainBucket, err := knowledgeBucket.CreateBucketIfNotExists([]byte(stored.Domain))
		if err != nil {
			return NewDatabaseError("set_knowledge", fmt.Errorf("failed to create domain bucket: %w", err))
		}

		// Check if entry already exists to preserve CreatedAt
		existing := domainBucket.Get([]byte(stored.Key))
		if existing != nil {
			var existingEntry KnowledgeEntry
			if err := json.Unmarshal(existing, &existingEntry); err == nil {
				stored.CreatedAt = existingEntry.CreatedAt
			} else {
				stored.CreatedAt = now
			}
		} else {
			stored.CreatedAt = now
		}
		stored.UpdatedAt = now

		// Marshal and store
		entryBytes, err := json.Marshal(&stored)
		if err != nil {
			return NewDatabaseError("set_knowledge", fmt.Errorf("failed to marshal knowledge entry: %w", err))
		}

		if err := domainBucket.Put([]byte(stored.Key), entryBytes); err != nil {
			return NewDatabaseError("set_knowledge", fmt.Errorf("failed to store knowledge entry: %w", err))
		}

		return nil
	})

	if err != nil {
		return err
	}

	d.logger.Infof("Set knowledge entry for user %s domain %s key %s", userID, entry.Domain, entry.Key)
	return nil
}

// GetKnowledge retrieves a single knowledge entry for a user by domain and key
func (d *DB) GetKnowledge(userID, domain, key string) (*KnowledgeEntry, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	// Validate inputs
	if strings.TrimSpace(userID) == "" {
		return nil, NewValidationError("user_id", userID, "user ID cannot be empty")
	}

	if strings.TrimSpace(domain) == "" {
		return nil, NewValidationError("domain", domain, "domain cannot be empty")
	}

	if strings.TrimSpace(key) == "" {
		return nil, NewValidationError("key", key, "key cannot be empty")
	}

	var entry *KnowledgeEntry

	err := d.db.View(func(tx *bbolt.Tx) error {
		// Navigate to the user bucket
		usersBucket := tx.Bucket([]byte(internal.BucketUsers))
		if usersBucket == nil {
			return NewDatabaseError("get_knowledge", fmt.Errorf("users bucket not found"))
		}

		userBucket := usersBucket.Bucket([]byte(userID))
		if userBucket == nil {
			return NewDatabaseError("get_knowledge", ErrUserNotFound)
		}

		// Navigate to knowledge bucket
		knowledgeBucket := userBucket.Bucket([]byte(internal.BucketUserKnowledge))
		if knowledgeBucket == nil {
			return NewDatabaseError("get_knowledge", ErrKnowledgeNotFound)
		}

		// Navigate to domain bucket
		domainBucket := knowledgeBucket.Bucket([]byte(domain))
		if domainBucket == nil {
			return NewDatabaseError("get_knowledge", ErrKnowledgeNotFound)
		}

		// Get the entry
		entryBytes := domainBucket.Get([]byte(key))
		if entryBytes == nil {
			return NewDatabaseError("get_knowledge", ErrKnowledgeNotFound)
		}

		// Unmarshal
		entry = &KnowledgeEntry{}
		if err := json.Unmarshal(entryBytes, entry); err != nil {
			return NewDatabaseError("get_knowledge", fmt.Errorf("failed to unmarshal knowledge entry: %w", err))
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	d.logger.Debugf("Retrieved knowledge entry for user %s domain %s key %s", userID, domain, key)
	return entry, nil
}

// ListKnowledge returns knowledge entries for a user. If domain is non-empty,
// only entries in that domain are returned. If domain is empty, all entries
// across all domains are returned.
func (d *DB) ListKnowledge(userID, domain string) ([]KnowledgeEntry, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	// Validate inputs
	if strings.TrimSpace(userID) == "" {
		return nil, NewValidationError("user_id", userID, "user ID cannot be empty")
	}

	var entries []KnowledgeEntry

	err := d.db.View(func(tx *bbolt.Tx) error {
		// Navigate to the user bucket
		usersBucket := tx.Bucket([]byte(internal.BucketUsers))
		if usersBucket == nil {
			return NewDatabaseError("list_knowledge", fmt.Errorf("users bucket not found"))
		}

		userBucket := usersBucket.Bucket([]byte(userID))
		if userBucket == nil {
			return NewDatabaseError("list_knowledge", ErrUserNotFound)
		}

		// Navigate to knowledge bucket
		knowledgeBucket := userBucket.Bucket([]byte(internal.BucketUserKnowledge))
		if knowledgeBucket == nil {
			// User exists but has no knowledge entries
			return nil
		}

		if strings.TrimSpace(domain) != "" {
			// List entries for a specific domain
			domainBucket := knowledgeBucket.Bucket([]byte(domain))
			if domainBucket == nil {
				// Domain does not exist, return empty slice
				return nil
			}

			return domainBucket.ForEach(func(k, v []byte) error {
				var entry KnowledgeEntry
				if err := json.Unmarshal(v, &entry); err != nil {
					d.logger.Warningf("Failed to unmarshal knowledge entry %s/%s: %v", domain, string(k), err)
					return nil // Continue iteration
				}
				entries = append(entries, entry)
				return nil
			})
		}

		// List entries across all domains
		c := knowledgeBucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if v != nil {
				// Regular key-value pair, not a sub-bucket; skip
				continue
			}

			// k is a sub-bucket name (domain)
			domainBucket := knowledgeBucket.Bucket(k)
			if domainBucket == nil {
				continue
			}

			if err := domainBucket.ForEach(func(entryKey, entryValue []byte) error {
				var entry KnowledgeEntry
				if err := json.Unmarshal(entryValue, &entry); err != nil {
					d.logger.Warningf("Failed to unmarshal knowledge entry %s/%s: %v", string(k), string(entryKey), err)
					return nil // Continue iteration
				}
				entries = append(entries, entry)
				return nil
			}); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if entries == nil {
		entries = []KnowledgeEntry{}
	}

	d.logger.Debugf("Listed %d knowledge entries for user %s (domain: %q)", len(entries), userID, domain)
	return entries, nil
}

// DeleteKnowledge removes a knowledge entry for a user by domain and key.
// If the domain bucket becomes empty after deletion, it is also removed.
func (d *DB) DeleteKnowledge(userID, domain, key string) error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	// Validate inputs
	if strings.TrimSpace(userID) == "" {
		return NewValidationError("user_id", userID, "user ID cannot be empty")
	}

	if strings.TrimSpace(domain) == "" {
		return NewValidationError("domain", domain, "domain cannot be empty")
	}

	if strings.TrimSpace(key) == "" {
		return NewValidationError("key", key, "key cannot be empty")
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		// Navigate to the user bucket
		usersBucket := tx.Bucket([]byte(internal.BucketUsers))
		if usersBucket == nil {
			return NewDatabaseError("delete_knowledge", fmt.Errorf("users bucket not found"))
		}

		userBucket := usersBucket.Bucket([]byte(userID))
		if userBucket == nil {
			return NewDatabaseError("delete_knowledge", ErrUserNotFound)
		}

		// Navigate to knowledge bucket
		knowledgeBucket := userBucket.Bucket([]byte(internal.BucketUserKnowledge))
		if knowledgeBucket == nil {
			return NewDatabaseError("delete_knowledge", ErrKnowledgeNotFound)
		}

		// Navigate to domain bucket
		domainBucket := knowledgeBucket.Bucket([]byte(domain))
		if domainBucket == nil {
			return NewDatabaseError("delete_knowledge", ErrKnowledgeNotFound)
		}

		// Verify the entry exists before deleting
		if domainBucket.Get([]byte(key)) == nil {
			return NewDatabaseError("delete_knowledge", ErrKnowledgeNotFound)
		}

		// Delete the entry
		if err := domainBucket.Delete([]byte(key)); err != nil {
			return NewDatabaseError("delete_knowledge", fmt.Errorf("failed to delete knowledge entry: %w", err))
		}

		// Clean up empty domain bucket
		isEmpty := true
		c := domainBucket.Cursor()
		if k, _ := c.First(); k != nil {
			isEmpty = false
		}

		if isEmpty {
			if err := knowledgeBucket.DeleteBucket([]byte(domain)); err != nil {
				d.logger.Warningf("Failed to clean up empty domain bucket %s: %v", domain, err)
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	d.logger.Infof("Deleted knowledge entry for user %s domain %s key %s", userID, domain, key)
	return nil
}

// RenameKnowledge renames a knowledge entry's key within a domain, preserving its content and metadata.
func (d *DB) RenameKnowledge(userID, domain, oldKey, newKey string) error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	// Validate inputs
	if strings.TrimSpace(userID) == "" {
		return NewValidationError("user_id", userID, "user ID cannot be empty")
	}

	if strings.TrimSpace(domain) == "" {
		return NewValidationError("domain", domain, "domain cannot be empty")
	}

	if strings.TrimSpace(oldKey) == "" {
		return NewValidationError("old_key", oldKey, "old key cannot be empty")
	}

	if strings.TrimSpace(newKey) == "" {
		return NewValidationError("new_key", newKey, "new key cannot be empty")
	}

	if oldKey == newKey {
		return NewValidationError("new_key", newKey, "new key must be different from old key")
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		// Navigate to the user bucket
		usersBucket := tx.Bucket([]byte(internal.BucketUsers))
		if usersBucket == nil {
			return NewDatabaseError("rename_knowledge", fmt.Errorf("users bucket not found"))
		}

		userBucket := usersBucket.Bucket([]byte(userID))
		if userBucket == nil {
			return NewDatabaseError("rename_knowledge", ErrUserNotFound)
		}

		// Navigate to knowledge bucket
		knowledgeBucket := userBucket.Bucket([]byte(internal.BucketUserKnowledge))
		if knowledgeBucket == nil {
			return NewDatabaseError("rename_knowledge", ErrKnowledgeNotFound)
		}

		// Navigate to domain bucket
		domainBucket := knowledgeBucket.Bucket([]byte(domain))
		if domainBucket == nil {
			return NewDatabaseError("rename_knowledge", ErrKnowledgeNotFound)
		}

		// Get existing entry by oldKey
		entryBytes := domainBucket.Get([]byte(oldKey))
		if entryBytes == nil {
			return NewDatabaseError("rename_knowledge", ErrKnowledgeNotFound)
		}

		// Check that newKey does not already exist
		if domainBucket.Get([]byte(newKey)) != nil {
			return NewDatabaseError("rename_knowledge", fmt.Errorf("key %q already exists in domain %q", newKey, domain))
		}

		// Unmarshal the entry
		var entry KnowledgeEntry
		if err := json.Unmarshal(entryBytes, &entry); err != nil {
			return NewDatabaseError("rename_knowledge", fmt.Errorf("failed to unmarshal knowledge entry: %w", err))
		}

		// Update key and timestamp
		entry.Key = newKey
		entry.UpdatedAt = time.Now()

		// Marshal updated entry
		updatedBytes, err := json.Marshal(&entry)
		if err != nil {
			return NewDatabaseError("rename_knowledge", fmt.Errorf("failed to marshal knowledge entry: %w", err))
		}

		// Store under new key
		if err := domainBucket.Put([]byte(newKey), updatedBytes); err != nil {
			return NewDatabaseError("rename_knowledge", fmt.Errorf("failed to store renamed knowledge entry: %w", err))
		}

		// Delete old key
		if err := domainBucket.Delete([]byte(oldKey)); err != nil {
			return NewDatabaseError("rename_knowledge", fmt.Errorf("failed to delete old knowledge entry: %w", err))
		}

		return nil
	})

	if err != nil {
		return err
	}

	d.logger.Infof("Renamed knowledge entry for user %s domain %s: %s -> %s", userID, domain, oldKey, newKey)
	return nil
}

// SearchKnowledge searches knowledge entries for a user by performing a case-insensitive
// substring match of the query against each entry's Domain, Key, and Content fields.
func (d *DB) SearchKnowledge(userID, query string) ([]KnowledgeEntry, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	// Validate inputs
	if strings.TrimSpace(userID) == "" {
		return nil, NewValidationError("user_id", userID, "user ID cannot be empty")
	}

	if strings.TrimSpace(query) == "" {
		return nil, NewValidationError("query", query, "query cannot be empty")
	}

	lowerQuery := strings.ToLower(query)
	var entries []KnowledgeEntry

	err := d.db.View(func(tx *bbolt.Tx) error {
		// Navigate to the user bucket
		usersBucket := tx.Bucket([]byte(internal.BucketUsers))
		if usersBucket == nil {
			return NewDatabaseError("search_knowledge", fmt.Errorf("users bucket not found"))
		}

		userBucket := usersBucket.Bucket([]byte(userID))
		if userBucket == nil {
			return NewDatabaseError("search_knowledge", ErrUserNotFound)
		}

		// Navigate to knowledge bucket
		knowledgeBucket := userBucket.Bucket([]byte(internal.BucketUserKnowledge))
		if knowledgeBucket == nil {
			// User exists but has no knowledge entries
			return nil
		}

		// Iterate all domains
		c := knowledgeBucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if v != nil {
				// Regular key-value pair, not a sub-bucket; skip
				continue
			}

			// k is a sub-bucket name (domain)
			domainBucket := knowledgeBucket.Bucket(k)
			if domainBucket == nil {
				continue
			}

			if err := domainBucket.ForEach(func(entryKey, entryValue []byte) error {
				var entry KnowledgeEntry
				if err := json.Unmarshal(entryValue, &entry); err != nil {
					d.logger.Warningf("Failed to unmarshal knowledge entry %s/%s: %v", string(k), string(entryKey), err)
					return nil // Continue iteration
				}

				// Case-insensitive substring match against Domain, Key, and Content
				if strings.Contains(strings.ToLower(entry.Domain), lowerQuery) ||
					strings.Contains(strings.ToLower(entry.Key), lowerQuery) ||
					strings.Contains(strings.ToLower(entry.Content), lowerQuery) {
					entries = append(entries, entry)
				}

				return nil
			}); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if entries == nil {
		entries = []KnowledgeEntry{}
	}

	d.logger.Debugf("Search for %q returned %d knowledge entries for user %s", query, len(entries), userID)
	return entries, nil
}
