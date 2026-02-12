/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package db

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isValidUUID checks that the string has the format of a UUID v4:
// 8-4-4-4-12 hex characters (36 chars total, 4 hyphens).
func isValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	if strings.Count(s, "-") != 4 {
		return false
	}
	return true
}

// TestCreateUser verifies that CreateUser generates a properly formatted user
// with all metadata fields populated.
func TestCreateUser(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	user, err := database.CreateUser("Test user")
	require.NoError(t, err)
	require.NotNil(t, user)

	// Verify UserID is a valid UUID
	assert.True(t, isValidUUID(user.UserID), "UserID should be a valid UUID, got: %s", user.UserID)
	assert.Len(t, user.UserID, 36, "UserID should be 36 characters")
	assert.Equal(t, 4, strings.Count(user.UserID, "-"), "UserID should contain 4 hyphens")

	// Verify other fields
	assert.Equal(t, "Test user", user.Description)
	assert.False(t, user.CreatedAt.IsZero(), "CreatedAt should be set")
	assert.False(t, user.UpdatedAt.IsZero(), "UpdatedAt should be set")
	assert.Equal(t, user.CreatedAt, user.UpdatedAt, "CreatedAt and UpdatedAt should match on creation")
}

// TestCreateUser_EmptyDescription verifies that an empty description is rejected
// with a validation error.
func TestCreateUser_EmptyDescription(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	user, err := database.CreateUser("")
	assert.Error(t, err)
	assert.Nil(t, user)
	assert.True(t, IsValidationError(err), "should be a validation error")
}

// TestGetUser verifies that a created user can be retrieved and all fields match.
func TestGetUser(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	created, err := database.CreateUser("Retrievable user")
	require.NoError(t, err)

	retrieved, err := database.GetUser(created.UserID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, created.UserID, retrieved.UserID)
	assert.Equal(t, created.Description, retrieved.Description)
	assert.Equal(t, created.CreatedAt.Unix(), retrieved.CreatedAt.Unix())
	assert.Equal(t, created.UpdatedAt.Unix(), retrieved.UpdatedAt.Unix())
}

// TestGetUser_NotFound verifies that retrieving a nonexistent user returns
// ErrUserNotFound.
func TestGetUser_NotFound(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	user, err := database.GetUser("00000000-0000-0000-0000-000000000000")
	assert.Error(t, err)
	assert.Nil(t, user)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

// TestListUsers creates multiple users and verifies ListUsers returns all of them.
func TestListUsers(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	descriptions := []string{"User A", "User B", "User C"}
	createdIDs := make(map[string]string) // userID -> description

	for _, desc := range descriptions {
		user, err := database.CreateUser(desc)
		require.NoError(t, err)
		createdIDs[user.UserID] = desc
	}

	users, err := database.ListUsers()
	require.NoError(t, err)
	assert.Len(t, users, len(descriptions))

	// Verify all created users are present
	for _, u := range users {
		expectedDesc, found := createdIDs[u.UserID]
		assert.True(t, found, "unexpected user ID: %s", u.UserID)
		assert.Equal(t, expectedDesc, u.Description)
	}
}

// TestListUsers_Empty verifies that ListUsers returns an empty slice when no
// users exist.
func TestListUsers_Empty(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	users, err := database.ListUsers()
	require.NoError(t, err)
	assert.Empty(t, users)
}

// TestDeleteUser creates a user, deletes it, and verifies it is no longer
// retrievable.
func TestDeleteUser(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	user, err := database.CreateUser("Doomed user")
	require.NoError(t, err)

	err = database.DeleteUser(user.UserID)
	require.NoError(t, err)

	// Verify the user is gone
	retrieved, err := database.GetUser(user.UserID)
	assert.Error(t, err)
	assert.Nil(t, retrieved)
	assert.ErrorIs(t, err, ErrUserNotFound)

	// Verify it does not appear in ListUsers
	users, err := database.ListUsers()
	require.NoError(t, err)
	assert.Empty(t, users)
}

// TestDeleteUser_NotFound verifies that deleting a nonexistent user returns
// ErrUserNotFound.
func TestDeleteUser_NotFound(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	err := database.DeleteUser("00000000-0000-0000-0000-000000000000")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

// TestDeleteUser_CleansUpKeyLinks creates a user, links an API key, deletes the
// user, and verifies the key_to_user reverse index entry is cleaned up.
func TestDeleteUser_CleansUpKeyLinks(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	// Create a user
	user, err := database.CreateUser("User with key")
	require.NoError(t, err)

	// Create an API token to link
	_, hash, err := database.AddAPIToken("linked token")
	require.NoError(t, err)

	// Link the key to the user
	err = database.LinkAPIKey(user.UserID, hash)
	require.NoError(t, err)

	// Sanity check: lookup should work before deletion
	foundUserID, err := database.GetUserByAPIKey(hash)
	require.NoError(t, err)
	assert.Equal(t, user.UserID, foundUserID)

	// Delete the user
	err = database.DeleteUser(user.UserID)
	require.NoError(t, err)

	// Verify key_to_user mapping is cleaned up
	_, err = database.GetUserByAPIKey(hash)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

// TestLinkAPIKey creates a user and an API token, links them, and verifies the
// link works in both directions.
func TestLinkAPIKey(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	// Create user and token
	user, err := database.CreateUser("Linkable user")
	require.NoError(t, err)

	_, hash, err := database.AddAPIToken("linkable token")
	require.NoError(t, err)

	// Link the key
	err = database.LinkAPIKey(user.UserID, hash)
	require.NoError(t, err)

	// Verify forward lookup: key -> user
	foundUserID, err := database.GetUserByAPIKey(hash)
	require.NoError(t, err)
	assert.Equal(t, user.UserID, foundUserID)
}

// TestLinkAPIKey_UserNotFound verifies that linking a key to a nonexistent user
// returns ErrUserNotFound.
func TestLinkAPIKey_UserNotFound(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	// Create a real API token
	_, hash, err := database.AddAPIToken("orphan token")
	require.NoError(t, err)

	err = database.LinkAPIKey("00000000-0000-0000-0000-000000000000", hash)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

// TestLinkAPIKey_AlreadyLinked verifies that linking a key that is already linked
// returns ErrKeyAlreadyLinked.
func TestLinkAPIKey_AlreadyLinked(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	user1, err := database.CreateUser("First user")
	require.NoError(t, err)

	user2, err := database.CreateUser("Second user")
	require.NoError(t, err)

	_, hash, err := database.AddAPIToken("contested token")
	require.NoError(t, err)

	// First link should succeed
	err = database.LinkAPIKey(user1.UserID, hash)
	require.NoError(t, err)

	// Second link (same key, different user) should fail
	err = database.LinkAPIKey(user2.UserID, hash)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrKeyAlreadyLinked)
}

// TestLinkAPIKey_TokenNotFound verifies that linking a nonexistent API token
// returns an error wrapping ErrTokenNotFound.
func TestLinkAPIKey_TokenNotFound(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	user, err := database.CreateUser("User without key")
	require.NoError(t, err)

	fakeHash := strings.Repeat("ab", 32) // 64 hex chars
	err = database.LinkAPIKey(user.UserID, fakeHash)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTokenNotFound)
}

// TestUnlinkAPIKey links a key, unlinks it, and verifies the reverse lookup
// returns ErrUserNotFound.
func TestUnlinkAPIKey(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	user, err := database.CreateUser("Unlinkable user")
	require.NoError(t, err)

	_, hash, err := database.AddAPIToken("unlinkable token")
	require.NoError(t, err)

	err = database.LinkAPIKey(user.UserID, hash)
	require.NoError(t, err)

	// Unlink
	err = database.UnlinkAPIKey(hash)
	require.NoError(t, err)

	// Verify lookup returns not found
	_, err = database.GetUserByAPIKey(hash)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

// TestUnlinkAPIKey_NotLinked verifies that unlinking a key that is not linked
// returns ErrUserNotFound.
func TestUnlinkAPIKey_NotLinked(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	_, hash, err := database.AddAPIToken("never linked token")
	require.NoError(t, err)

	err = database.UnlinkAPIKey(hash)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

// TestGetUserByAPIKey creates a user, links a key, and verifies GetUserByAPIKey
// returns the correct user ID.
func TestGetUserByAPIKey(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	user, err := database.CreateUser("Lookup user")
	require.NoError(t, err)

	_, hash, err := database.AddAPIToken("lookup token")
	require.NoError(t, err)

	err = database.LinkAPIKey(user.UserID, hash)
	require.NoError(t, err)

	foundUserID, err := database.GetUserByAPIKey(hash)
	require.NoError(t, err)
	assert.Equal(t, user.UserID, foundUserID)
}

// TestGetUserByAPIKey_NotLinked verifies that looking up a key that has no user
// link returns ErrUserNotFound.
func TestGetUserByAPIKey_NotLinked(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	_, hash, err := database.AddAPIToken("unlinked token")
	require.NoError(t, err)

	_, err = database.GetUserByAPIKey(hash)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

// TestAutoMigrateKeys creates API tokens, calls AutoMigrateKeys, and verifies
// that users were created and keys were linked.
func TestAutoMigrateKeys(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	// Create API tokens without any user links
	_, hash1, err := database.AddAPIToken("migrate me 1")
	require.NoError(t, err)

	_, hash2, err := database.AddAPIToken("migrate me 2")
	require.NoError(t, err)

	// No users should exist yet
	users, err := database.ListUsers()
	require.NoError(t, err)
	assert.Empty(t, users)

	// Run migration
	err = database.AutoMigrateKeys()
	require.NoError(t, err)

	// Verify users were created
	users, err = database.ListUsers()
	require.NoError(t, err)
	assert.Len(t, users, 2)

	// Verify both keys are linked to users
	userID1, err := database.GetUserByAPIKey(hash1)
	require.NoError(t, err)
	assert.True(t, isValidUUID(userID1))

	userID2, err := database.GetUserByAPIKey(hash2)
	require.NoError(t, err)
	assert.True(t, isValidUUID(userID2))

	// Each key should be linked to a different user
	assert.NotEqual(t, userID1, userID2)
}

// TestAutoMigrateKeys_AlreadyLinked creates a token, manually links it to a user,
// then calls AutoMigrateKeys and verifies no duplicate migration occurs.
func TestAutoMigrateKeys_AlreadyLinked(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	// Create a token and manually link it
	_, hash, err := database.AddAPIToken("already linked")
	require.NoError(t, err)

	user, err := database.CreateUser("Pre-existing user")
	require.NoError(t, err)

	err = database.LinkAPIKey(user.UserID, hash)
	require.NoError(t, err)

	// Run migration
	err = database.AutoMigrateKeys()
	require.NoError(t, err)

	// Verify only the original user exists (no extra user created)
	users, err := database.ListUsers()
	require.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, user.UserID, users[0].UserID)

	// Verify the key is still linked to the original user
	foundUserID, err := database.GetUserByAPIKey(hash)
	require.NoError(t, err)
	assert.Equal(t, user.UserID, foundUserID)
}

// TestAutoMigrateKeys_NoTokens calls AutoMigrateKeys with no tokens in the
// database and verifies no error is returned.
func TestAutoMigrateKeys_NoTokens(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	err := database.AutoMigrateKeys()
	require.NoError(t, err)

	// No users should have been created
	users, err := database.ListUsers()
	require.NoError(t, err)
	assert.Empty(t, users)
}
