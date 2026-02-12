/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package db

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestUser is a helper that creates a user and returns its ID.
func createTestUser(t *testing.T, database Database, description string) string {
	t.Helper()
	user, err := database.CreateUser(description)
	require.NoError(t, err, "Failed to create test user")
	require.NotEmpty(t, user.UserID)
	return user.UserID
}

func TestSetKnowledge(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	userID := createTestUser(t, database, "test user")

	entry := &KnowledgeEntry{
		Domain:  "email",
		Key:     "preferences",
		Content: "User prefers HTML emails",
	}

	err := database.SetKnowledge(userID, entry)
	require.NoError(t, err)
}

func TestSetKnowledge_UpdateExisting(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	userID := createTestUser(t, database, "test user")

	// Create the initial entry
	entry := &KnowledgeEntry{
		Domain:  "email",
		Key:     "preferences",
		Content: "User prefers HTML emails",
	}
	err := database.SetKnowledge(userID, entry)
	require.NoError(t, err)

	// Retrieve to capture the original CreatedAt
	original, err := database.GetKnowledge(userID, "email", "preferences")
	require.NoError(t, err)
	originalCreatedAt := original.CreatedAt
	originalUpdatedAt := original.UpdatedAt

	// Sleep to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Update the entry with new content
	updatedEntry := &KnowledgeEntry{
		Domain:  "email",
		Key:     "preferences",
		Content: "User prefers plain text emails",
	}
	err = database.SetKnowledge(userID, updatedEntry)
	require.NoError(t, err)

	// Retrieve and verify timestamps
	retrieved, err := database.GetKnowledge(userID, "email", "preferences")
	require.NoError(t, err)

	assert.Equal(t, "User prefers plain text emails", retrieved.Content)
	assert.Equal(t, originalCreatedAt, retrieved.CreatedAt, "CreatedAt should be preserved on update")
	assert.True(t, retrieved.UpdatedAt.After(originalUpdatedAt), "UpdatedAt should be later than original")
}

func TestSetKnowledge_ValidationErrors(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	userID := createTestUser(t, database, "test user")

	tests := []struct {
		name    string
		userID  string
		entry   *KnowledgeEntry
		wantMsg string
	}{
		{
			name:    "Empty userID",
			userID:  "",
			entry:   &KnowledgeEntry{Domain: "email", Key: "k", Content: "c"},
			wantMsg: "user ID cannot be empty",
		},
		{
			name:    "Nil entry",
			userID:  userID,
			entry:   nil,
			wantMsg: "knowledge entry cannot be nil",
		},
		{
			name:    "Empty domain",
			userID:  userID,
			entry:   &KnowledgeEntry{Domain: "", Key: "k", Content: "c"},
			wantMsg: "domain cannot be empty",
		},
		{
			name:    "Empty key",
			userID:  userID,
			entry:   &KnowledgeEntry{Domain: "email", Key: "", Content: "c"},
			wantMsg: "key cannot be empty",
		},
		{
			name:    "Empty content",
			userID:  userID,
			entry:   &KnowledgeEntry{Domain: "email", Key: "k", Content: ""},
			wantMsg: "content cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := database.SetKnowledge(tt.userID, tt.entry)
			assert.Error(t, err)
			assert.True(t, IsValidationError(err), "expected validation error, got: %v", err)
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
}

func TestSetKnowledge_UserNotFound(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	entry := &KnowledgeEntry{
		Domain:  "email",
		Key:     "preferences",
		Content: "some content",
	}

	err := database.SetKnowledge("nonexistent-user-id", entry)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrUserNotFound), "expected ErrUserNotFound, got: %v", err)
}

func TestGetKnowledge(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	userID := createTestUser(t, database, "test user")

	entry := &KnowledgeEntry{
		Domain:  "calendar",
		Key:     "meeting-preferences",
		Content: "User prefers 30-minute meetings in the morning",
	}

	err := database.SetKnowledge(userID, entry)
	require.NoError(t, err)

	retrieved, err := database.GetKnowledge(userID, "calendar", "meeting-preferences")
	require.NoError(t, err)

	assert.Equal(t, "calendar", retrieved.Domain)
	assert.Equal(t, "meeting-preferences", retrieved.Key)
	assert.Equal(t, "User prefers 30-minute meetings in the morning", retrieved.Content)
	assert.False(t, retrieved.CreatedAt.IsZero(), "CreatedAt should be set")
	assert.False(t, retrieved.UpdatedAt.IsZero(), "UpdatedAt should be set")
}

func TestGetKnowledge_NotFound(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	userID := createTestUser(t, database, "test user")

	// Set one entry so the knowledge bucket exists
	entry := &KnowledgeEntry{
		Domain:  "email",
		Key:     "preferences",
		Content: "some content",
	}
	err := database.SetKnowledge(userID, entry)
	require.NoError(t, err)

	tests := []struct {
		name   string
		domain string
		key    string
	}{
		{"Nonexistent domain", "nonexistent-domain", "preferences"},
		{"Nonexistent key", "email", "nonexistent-key"},
		{"Both nonexistent", "no-domain", "no-key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := database.GetKnowledge(userID, tt.domain, tt.key)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrKnowledgeNotFound), "expected ErrKnowledgeNotFound, got: %v", err)
		})
	}
}

func TestGetKnowledge_UserNotFound(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	_, err := database.GetKnowledge("nonexistent-user-id", "email", "preferences")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrUserNotFound), "expected ErrUserNotFound, got: %v", err)
}

func TestListKnowledge_ByDomain(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	userID := createTestUser(t, database, "test user")

	// Create multiple entries in the same domain
	entries := []KnowledgeEntry{
		{Domain: "email", Key: "preferences", Content: "Prefers HTML"},
		{Domain: "email", Key: "signature", Content: "Best regards, Test User"},
		{Domain: "email", Key: "filters", Content: "Auto-archive newsletters"},
	}

	for i := range entries {
		err := database.SetKnowledge(userID, &entries[i])
		require.NoError(t, err)
	}

	// List by domain
	result, err := database.ListKnowledge(userID, "email")
	require.NoError(t, err)
	assert.Len(t, result, 3)

	// Verify all entries are present by building a map of keys
	keyMap := make(map[string]string)
	for _, e := range result {
		keyMap[e.Key] = e.Content
	}

	assert.Equal(t, "Prefers HTML", keyMap["preferences"])
	assert.Equal(t, "Best regards, Test User", keyMap["signature"])
	assert.Equal(t, "Auto-archive newsletters", keyMap["filters"])
}

func TestListKnowledge_AllDomains(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	userID := createTestUser(t, database, "test user")

	// Create entries in multiple domains
	entries := []KnowledgeEntry{
		{Domain: "email", Key: "preferences", Content: "Prefers HTML"},
		{Domain: "calendar", Key: "meeting-length", Content: "30 minutes"},
		{Domain: "contacts", Key: "groups", Content: "Team, Family, Friends"},
	}

	for i := range entries {
		err := database.SetKnowledge(userID, &entries[i])
		require.NoError(t, err)
	}

	// List all domains (empty domain parameter)
	result, err := database.ListKnowledge(userID, "")
	require.NoError(t, err)
	assert.Len(t, result, 3)

	// Verify entries from all domains are present
	domainKeyMap := make(map[string]string)
	for _, e := range result {
		domainKeyMap[e.Domain+"/"+e.Key] = e.Content
	}

	assert.Equal(t, "Prefers HTML", domainKeyMap["email/preferences"])
	assert.Equal(t, "30 minutes", domainKeyMap["calendar/meeting-length"])
	assert.Equal(t, "Team, Family, Friends", domainKeyMap["contacts/groups"])
}

func TestListKnowledge_EmptyResult(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	userID := createTestUser(t, database, "test user")

	// List with no knowledge entries created
	result, err := database.ListKnowledge(userID, "")
	require.NoError(t, err)
	assert.NotNil(t, result, "result should be an empty slice, not nil")
	assert.Empty(t, result)
}

func TestListKnowledge_NonexistentDomain(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	userID := createTestUser(t, database, "test user")

	// Add an entry in a different domain so the knowledge bucket exists
	entry := &KnowledgeEntry{
		Domain:  "email",
		Key:     "preferences",
		Content: "some content",
	}
	err := database.SetKnowledge(userID, entry)
	require.NoError(t, err)

	// List by a domain that does not exist
	result, err := database.ListKnowledge(userID, "nonexistent-domain")
	require.NoError(t, err)
	assert.NotNil(t, result, "result should be an empty slice, not nil")
	assert.Empty(t, result)
}

func TestListKnowledge_UserNotFound(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	_, err := database.ListKnowledge("nonexistent-user-id", "")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrUserNotFound), "expected ErrUserNotFound, got: %v", err)
}

func TestDeleteKnowledge(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	userID := createTestUser(t, database, "test user")

	entry := &KnowledgeEntry{
		Domain:  "email",
		Key:     "preferences",
		Content: "some content",
	}
	err := database.SetKnowledge(userID, entry)
	require.NoError(t, err)

	// Verify the entry exists
	_, err = database.GetKnowledge(userID, "email", "preferences")
	require.NoError(t, err)

	// Delete it
	err = database.DeleteKnowledge(userID, "email", "preferences")
	require.NoError(t, err)

	// Verify it is gone
	_, err = database.GetKnowledge(userID, "email", "preferences")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrKnowledgeNotFound), "expected ErrKnowledgeNotFound, got: %v", err)
}

func TestDeleteKnowledge_NotFound(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	userID := createTestUser(t, database, "test user")

	err := database.DeleteKnowledge(userID, "email", "nonexistent-key")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrKnowledgeNotFound), "expected ErrKnowledgeNotFound, got: %v", err)
}

func TestDeleteKnowledge_UserNotFound(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	err := database.DeleteKnowledge("nonexistent-user-id", "email", "preferences")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrUserNotFound), "expected ErrUserNotFound, got: %v", err)
}

func TestDeleteKnowledge_CleansEmptyDomain(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	userID := createTestUser(t, database, "test user")

	// Create a single entry in a domain
	entry := &KnowledgeEntry{
		Domain:  "temp-domain",
		Key:     "only-entry",
		Content: "this will be deleted",
	}
	err := database.SetKnowledge(userID, entry)
	require.NoError(t, err)

	// Delete the only entry in the domain (should clean up the domain bucket)
	err = database.DeleteKnowledge(userID, "temp-domain", "only-entry")
	require.NoError(t, err)

	// Create a new entry in the same domain (should succeed since bucket was cleaned up)
	newEntry := &KnowledgeEntry{
		Domain:  "temp-domain",
		Key:     "new-entry",
		Content: "new content after cleanup",
	}
	err = database.SetKnowledge(userID, newEntry)
	require.NoError(t, err)

	// Verify the new entry is retrievable
	retrieved, err := database.GetKnowledge(userID, "temp-domain", "new-entry")
	require.NoError(t, err)
	assert.Equal(t, "new content after cleanup", retrieved.Content)
}

func TestKnowledge_MultipleDomainsIsolation(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	userID := createTestUser(t, database, "test user")

	// Create entries with the same key in different domains
	emailEntry := &KnowledgeEntry{
		Domain:  "email",
		Key:     "preferences",
		Content: "Email preferences content",
	}
	calendarEntry := &KnowledgeEntry{
		Domain:  "calendar",
		Key:     "preferences",
		Content: "Calendar preferences content",
	}

	err := database.SetKnowledge(userID, emailEntry)
	require.NoError(t, err)

	err = database.SetKnowledge(userID, calendarEntry)
	require.NoError(t, err)

	// Verify each domain returns the correct content
	emailResult, err := database.GetKnowledge(userID, "email", "preferences")
	require.NoError(t, err)
	assert.Equal(t, "Email preferences content", emailResult.Content)

	calendarResult, err := database.GetKnowledge(userID, "calendar", "preferences")
	require.NoError(t, err)
	assert.Equal(t, "Calendar preferences content", calendarResult.Content)

	// Verify listing by domain returns only the correct entries
	emailList, err := database.ListKnowledge(userID, "email")
	require.NoError(t, err)
	assert.Len(t, emailList, 1)
	assert.Equal(t, "Email preferences content", emailList[0].Content)

	calendarList, err := database.ListKnowledge(userID, "calendar")
	require.NoError(t, err)
	assert.Len(t, calendarList, 1)
	assert.Equal(t, "Calendar preferences content", calendarList[0].Content)

	// Delete from one domain and verify the other is unaffected
	err = database.DeleteKnowledge(userID, "email", "preferences")
	require.NoError(t, err)

	_, err = database.GetKnowledge(userID, "email", "preferences")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrKnowledgeNotFound))

	calendarStillThere, err := database.GetKnowledge(userID, "calendar", "preferences")
	require.NoError(t, err)
	assert.Equal(t, "Calendar preferences content", calendarStillThere.Content)
}

func TestKnowledge_MultipleUsersIsolation(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(database, tempDir)

	user1 := createTestUser(t, database, "user one")
	user2 := createTestUser(t, database, "user two")

	// Create entries with the same domain and key for different users
	entry1 := &KnowledgeEntry{
		Domain:  "email",
		Key:     "preferences",
		Content: "User 1 prefers HTML",
	}
	entry2 := &KnowledgeEntry{
		Domain:  "email",
		Key:     "preferences",
		Content: "User 2 prefers plain text",
	}

	err := database.SetKnowledge(user1, entry1)
	require.NoError(t, err)

	err = database.SetKnowledge(user2, entry2)
	require.NoError(t, err)

	// Verify each user sees only their own data
	result1, err := database.GetKnowledge(user1, "email", "preferences")
	require.NoError(t, err)
	assert.Equal(t, "User 1 prefers HTML", result1.Content)

	result2, err := database.GetKnowledge(user2, "email", "preferences")
	require.NoError(t, err)
	assert.Equal(t, "User 2 prefers plain text", result2.Content)

	// Verify listing is isolated per user
	list1, err := database.ListKnowledge(user1, "email")
	require.NoError(t, err)
	assert.Len(t, list1, 1)
	assert.Equal(t, "User 1 prefers HTML", list1[0].Content)

	list2, err := database.ListKnowledge(user2, "email")
	require.NoError(t, err)
	assert.Len(t, list2, 1)
	assert.Equal(t, "User 2 prefers plain text", list2[0].Content)

	// Delete user1's entry and verify user2 is unaffected
	err = database.DeleteKnowledge(user1, "email", "preferences")
	require.NoError(t, err)

	_, err = database.GetKnowledge(user1, "email", "preferences")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrKnowledgeNotFound))

	stillThere, err := database.GetKnowledge(user2, "email", "preferences")
	require.NoError(t, err)
	assert.Equal(t, "User 2 prefers plain text", stillThere.Content)
}
