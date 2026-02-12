/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package db

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAuthCode(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	tenantHash := createTestTenant(t, database, "auth code test tenant")

	code, err := database.CreateAuthCode(tenantHash, "google", 5*time.Minute)
	require.NoError(t, err)
	assert.Len(t, code, 32, "Auth code should be a 32-character hex string")

	// Verify the code is valid hex by checking character set
	for _, c := range code {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"Auth code should contain only lowercase hex characters, got: %c", c)
	}
}

func TestCreateAuthCode_InvalidInputs(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	tenantHash := createTestTenant(t, database, "invalid input test tenant")

	tests := []struct {
		name       string
		tenantHash string
		service    string
		ttl        time.Duration
	}{
		{
			name:       "Empty tenantHash",
			tenantHash: "",
			service:    "google",
			ttl:        5 * time.Minute,
		},
		{
			name:       "Empty service",
			tenantHash: tenantHash,
			service:    "",
			ttl:        5 * time.Minute,
		},
		{
			name:       "Zero TTL",
			tenantHash: tenantHash,
			service:    "google",
			ttl:        0,
		},
		{
			name:       "Negative TTL",
			tenantHash: tenantHash,
			service:    "google",
			ttl:        -1 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := database.CreateAuthCode(tt.tenantHash, tt.service, tt.ttl)
			assert.Error(t, err, "Expected error for invalid input")
			assert.Empty(t, code, "Code should be empty on error")
		})
	}
}

func TestValidateAuthCode_Success(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	tenantHash := createTestTenant(t, database, "validate success test tenant")
	service := "microsoft365"

	code, err := database.CreateAuthCode(tenantHash, service, 5*time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, code)

	// Validate the auth code
	returnedTenantHash, returnedService, err := database.ValidateAuthCode(code)
	require.NoError(t, err)
	assert.Equal(t, tenantHash, returnedTenantHash, "Returned tenantHash should match")
	assert.Equal(t, service, returnedService, "Returned service should match")
}

func TestValidateAuthCode_Expired(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	tenantHash := createTestTenant(t, database, "validate expired test tenant")

	// Create an auth code with a very short TTL
	code, err := database.CreateAuthCode(tenantHash, "google", 1*time.Millisecond)
	require.NoError(t, err)
	require.NotEmpty(t, code)

	// Wait for the code to expire
	time.Sleep(10 * time.Millisecond)

	// Validate should fail because the code has expired
	_, _, err = database.ValidateAuthCode(code)
	assert.Error(t, err, "Expected error for expired auth code")
}

func TestValidateAuthCode_NotFound(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	// Validate a code that was never created
	_, _, err := database.ValidateAuthCode("nonexistent_code_1234567890")
	assert.Error(t, err, "Expected error for non-existent auth code")
}

func TestCleanupExpiredAuthCodes(t *testing.T) {
	database, tempDir, _ := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	defer database.Close()

	tenantHash := createTestTenant(t, database, "cleanup test tenant")

	// Create several auth codes with short TTLs
	codes := make([]string, 3)
	for i := 0; i < 3; i++ {
		code, err := database.CreateAuthCode(tenantHash, "google", 1*time.Millisecond)
		require.NoError(t, err)
		require.NotEmpty(t, code)
		codes[i] = code
	}

	// Wait for all codes to expire
	time.Sleep(10 * time.Millisecond)

	// Run cleanup
	err := database.CleanupExpiredAuthCodes()
	require.NoError(t, err)

	// Verify all expired codes have been removed
	for _, code := range codes {
		_, _, err := database.ValidateAuthCode(code)
		assert.Error(t, err, "Expected error for cleaned up auth code: %s", code)
	}
}
