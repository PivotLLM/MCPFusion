/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshOAuthToken_UpdatesAccessTokenPreservesRefreshToken(t *testing.T) {
	iface, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(iface, tempDir)

	// RefreshOAuthToken is defined on *DB, not on the Database interface.
	// The test is in package db (internal), so the type assertion is valid.
	database, ok := iface.(*DB)
	require.True(t, ok, "expected *DB from setupTestDB")

	tenantHash := createTestTenant(t, iface, "refresh test tenant")

	originalExpiry := time.Now().Add(time.Hour)
	original := &OAuthTokenData{
		AccessToken:  "old-access",
		RefreshToken: "my-refresh-token",
		TokenType:    "Bearer",
		ExpiresAt:    &originalExpiry,
	}

	err := iface.StoreOAuthToken(tenantHash, "svc", original)
	require.NoError(t, err)

	// Capture the UpdatedAt written by StoreOAuthToken.
	stored, err := iface.GetOAuthToken(tenantHash, "svc")
	require.NoError(t, err)
	originalUpdatedAt := stored.UpdatedAt

	// Small sleep so the new UpdatedAt is strictly after the original.
	time.Sleep(2 * time.Millisecond)

	newExpiry := time.Now().Add(2 * time.Hour)
	err = database.RefreshOAuthToken(tenantHash, "svc", "new-access", &newExpiry)
	require.NoError(t, err)

	refreshed, err := iface.GetOAuthToken(tenantHash, "svc")
	require.NoError(t, err)

	assert.Equal(t, "new-access", refreshed.AccessToken, "access token should be updated")
	assert.Equal(t, "my-refresh-token", refreshed.RefreshToken, "refresh token must be preserved")
	require.NotNil(t, refreshed.ExpiresAt, "ExpiresAt should not be nil")
	assert.WithinDuration(t, newExpiry, *refreshed.ExpiresAt, time.Second, "ExpiresAt should reflect the new value")
	assert.True(t, refreshed.UpdatedAt.After(originalUpdatedAt), "UpdatedAt should be later than original")
}

func TestRefreshOAuthToken_ErrorWhenNotFound(t *testing.T) {
	iface, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(iface, tempDir)

	database, ok := iface.(*DB)
	require.True(t, ok, "expected *DB from setupTestDB")

	tenantHash := createTestTenant(t, iface, "no-token tenant")

	newExpiry := time.Now().Add(time.Hour)
	err := database.RefreshOAuthToken(tenantHash, "nonexistent-service", "some-token", &newExpiry)
	require.Error(t, err, "should return an error when no token exists for the service")
	assert.True(t, IsNotFound(err), "error should be a not-found error")
}
