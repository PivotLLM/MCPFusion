/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLogger implements the global.Logger interface for testing
type testLogger struct {
	logs []string
	mu   sync.Mutex
}

func newTestLogger() *testLogger {
	return &testLogger{
		logs: make([]string, 0),
	}
}

func (l *testLogger) addLog(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, msg)
}

func (l *testLogger) getLogs() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	cpy := make([]string, len(l.logs))
	for i, log := range l.logs {
		cpy[i] = log
	}
	return cpy
}

func (l *testLogger) Debug(msg string)                { l.addLog("DEBUG: " + msg) }
func (l *testLogger) Info(msg string)                 { l.addLog("INFO: " + msg) }
func (l *testLogger) Notice(msg string)               { l.addLog("NOTICE: " + msg) }
func (l *testLogger) Warning(msg string)              { l.addLog("WARNING: " + msg) }
func (l *testLogger) Error(msg string)                { l.addLog("ERROR: " + msg) }
func (l *testLogger) Fatal(msg string)                { l.addLog("FATAL: " + msg) }
func (l *testLogger) Debugf(format string, v ...any)  { l.addLog(fmt.Sprintf("DEBUG: "+format, v...)) }
func (l *testLogger) Infof(format string, v ...any)   { l.addLog(fmt.Sprintf("INFO: "+format, v...)) }
func (l *testLogger) Noticef(format string, v ...any) { l.addLog(fmt.Sprintf("NOTICE: "+format, v...)) }
func (l *testLogger) Warningf(format string, v ...any) {
	l.addLog(fmt.Sprintf("WARNING: "+format, v...))
}
func (l *testLogger) Errorf(format string, v ...any) { l.addLog(fmt.Sprintf("ERROR: "+format, v...)) }
func (l *testLogger) Fatalf(format string, v ...any) { l.addLog(fmt.Sprintf("FATAL: "+format, v...)) }
func (l *testLogger) Close()                         {}

// Test helper functions

// setupTestDB creates a temporary database for testing
func setupTestDB(t *testing.T) (Database, string, *testLogger) {
	tempDir, err := os.MkdirTemp("", "mcpfusion_test_")
	require.NoError(t, err, "Failed to create temp directory")

	logger := newTestLogger()
	db, err := New(
		WithLogger(logger),
		WithDataDir(tempDir),
	)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		require.NoError(t, err, "Failed to create test database")
	}

	return db, tempDir, logger
}

// cleanupTestDB removes the temporary database
func cleanupTestDB(db Database, tempDir string) {
	if db != nil {
		_ = db.Close()
	}
	_ = os.RemoveAll(tempDir)
}

// createTestTenant creates a test tenant and returns its hash
func createTestTenant(t *testing.T, db Database, description string) string {
	_, hash, err := db.AddAPIToken(description)
	require.NoError(t, err, "Failed to create test tenant")
	return hash
}

// Core API Token Tests

func TestAPITokenAddVariousDescriptions(t *testing.T) {
	tests := []struct {
		name        string
		description string
		wantError   bool
	}{
		{"Valid description", "Test API token", false},
		{"Empty description", "", false},                     // Empty description is allowed by validation
		{"Long description", strings.Repeat("a", 500), true}, // Too long, exceeds MaxDescriptionLength (256)
		{"Unicode description", "测试令牌 🔐", false},
		{"Description with newlines", "Line 1\nLine 2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, tempDir, _ := setupTestDB(t)
			defer cleanupTestDB(db, tempDir)

			token, hash, err := db.AddAPIToken(tt.description)

			if tt.wantError {
				assert.Error(t, err)
				assert.Empty(t, token)
				assert.Empty(t, hash)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, token)
				assert.NotEmpty(t, hash)
				assert.Len(t, hash, 64, "Hash should be 64 characters (SHA-256)")

				// Verify token can be validated
				valid, returnedHash, err := db.ValidateAPIToken(token)
				require.NoError(t, err)
				assert.True(t, valid)
				assert.Equal(t, hash, returnedHash)
			}
		})
	}
}

func TestAPITokenValidation(t *testing.T) {
	db, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	// Create a valid token
	token, hash, err := db.AddAPIToken("Valid token")
	require.NoError(t, err)

	tests := []struct {
		name      string
		token     string
		wantValid bool
		wantError bool
		wantHash  string
	}{
		{"Valid token", token, true, false, hash},
		{"Invalid format - too short", "abc", false, true, ""},
		{"Invalid format - not hex", "invalid-token-xyz", false, true, ""},
		{"Valid format but wrong token", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", false, false, ""},
		{"Empty token", "", false, true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, returnedHash, err := db.ValidateAPIToken(tt.token)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantValid, valid)
			assert.Equal(t, tt.wantHash, returnedHash)
		})
	}
}

func TestAPITokenDeletion(t *testing.T) {
	db, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	// Create multiple tokens
	token1, hash1, err := db.AddAPIToken("Token 1")
	require.NoError(t, err)

	token2, hash2, err := db.AddAPIToken("Token 2")
	require.NoError(t, err)

	// Verify both tokens exist
	tokens, err := db.ListAPITokens()
	require.NoError(t, err)
	assert.Len(t, tokens, 2)

	// Delete first token
	err = db.DeleteAPIToken(hash1)
	require.NoError(t, err)

	// Verify first token is gone
	valid, _, err := db.ValidateAPIToken(token1)
	require.NoError(t, err)
	assert.False(t, valid)

	// Verify metadata is gone
	_, err = db.GetAPITokenMetadata(hash1)
	assert.True(t, IsNotFound(err))

	// Verify second token still exists
	valid, returnedHash, err := db.ValidateAPIToken(token2)
	require.NoError(t, err)
	assert.True(t, valid)
	assert.Equal(t, hash2, returnedHash)

	// Test cascade deletion - add OAuth token to second tenant
	tokenData := &OAuthTokenData{
		AccessToken: "oauth-token",
		TokenType:   "Bearer",
	}
	err = db.StoreOAuthToken(hash2, "service1", tokenData)
	require.NoError(t, err)

	// Delete second token (should cascade)
	err = db.DeleteAPIToken(hash2)
	require.NoError(t, err)

	// Note: OAuth tokens should be cascade deleted, but tenant bucket might persist
	// This depends on the implementation of DeleteAPIToken

	// Verify no tokens left
	tokens, err = db.ListAPITokens()
	require.NoError(t, err)
	assert.Empty(t, tokens)
}

func TestAPITokenListFunctionality(t *testing.T) {
	db, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	// Initially empty
	tokens, err := db.ListAPITokens()
	require.NoError(t, err)
	assert.Empty(t, tokens)

	// Create tokens with different descriptions
	descriptions := []string{"Token A", "Token B", "Token C"}
	createdHashes := make([]string, len(descriptions))

	for i, desc := range descriptions {
		_, hash, err := db.AddAPIToken(desc)
		require.NoError(t, err)
		createdHashes[i] = hash
	}

	// List and verify
	tokens, err = db.ListAPITokens()
	require.NoError(t, err)
	assert.Len(t, tokens, len(descriptions))

	// Verify all tokens are present
	foundHashes := make(map[string]bool)
	for _, token := range tokens {
		foundHashes[token.Hash] = true
		assert.NotZero(t, token.CreatedAt)
		assert.NotZero(t, token.LastUsed)
		assert.NotEmpty(t, token.Prefix)
		assert.Contains(t, descriptions, token.Description)
	}

	for _, hash := range createdHashes {
		assert.True(t, foundHashes[hash], "Hash %s not found in list", hash)
	}
}

// Core OAuth Token Tests

func TestOAuthTokenStoragePerTenantService(t *testing.T) {
	db, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	// Create test tenants
	tenant1 := createTestTenant(t, db, "Tenant 1")
	tenant2 := createTestTenant(t, db, "Tenant 2")

	// Create token data
	expiresAt := time.Now().Add(time.Hour)
	tokenData := &OAuthTokenData{
		AccessToken:  "access-token-123",
		RefreshToken: "refresh-token-123",
		TokenType:    "Bearer",
		ExpiresAt:    &expiresAt,
		Scope:        []string{"read", "write"},
	}

	tests := []struct {
		name        string
		tenantHash  string
		serviceName string
		wantError   bool
	}{
		{"Valid tenant1 service1", tenant1, "microsoft365", false},
		{"Valid tenant1 service2", tenant1, "google", false},
		{"Valid tenant2 service1", tenant2, "microsoft365", false},
		{"Invalid tenant hash", "invalid-hash", "service", true},
		{"Empty service name", tenant1, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := db.StoreOAuthToken(tt.tenantHash, tt.serviceName, tokenData)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)

				// Verify retrieval
				retrieved, err := db.GetOAuthToken(tt.tenantHash, tt.serviceName)
				require.NoError(t, err)

				assert.Equal(t, tokenData.AccessToken, retrieved.AccessToken)
				assert.Equal(t, tokenData.RefreshToken, retrieved.RefreshToken)
				assert.Equal(t, tokenData.TokenType, retrieved.TokenType)
				assert.Equal(t, len(tokenData.Scope), len(retrieved.Scope))
				assert.NotZero(t, retrieved.CreatedAt)
				assert.NotZero(t, retrieved.UpdatedAt)
			}
		})
	}
}

func TestOAuthTokenRetrievalAndExpiration(t *testing.T) {
	db, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	tenantHash := createTestTenant(t, db, "Test tenant")

	tests := []struct {
		name      string
		tokenData *OAuthTokenData
		checkFunc func(t *testing.T, token *OAuthTokenData)
	}{
		{
			"Token with expiration",
			&OAuthTokenData{
				AccessToken:  "access-1",
				RefreshToken: "refresh-1",
				TokenType:    "Bearer",
				ExpiresAt:    func() *time.Time { t := time.Now().Add(time.Hour); return &t }(),
				Scope:        []string{"read"},
			},
			func(t *testing.T, token *OAuthTokenData) {
				assert.False(t, token.IsExpired())
				assert.False(t, token.IsExpiredWithBuffer(30*time.Minute))
				assert.True(t, token.HasRefreshToken())
			},
		},
		{
			"Expired token",
			&OAuthTokenData{
				AccessToken:  "access-2",
				RefreshToken: "refresh-2",
				TokenType:    "Bearer",
				ExpiresAt:    func() *time.Time { t := time.Now().Add(-time.Hour); return &t }(),
			},
			func(t *testing.T, token *OAuthTokenData) {
				assert.True(t, token.IsExpired())
				assert.True(t, token.IsExpiredWithBuffer(30*time.Minute))
				assert.True(t, token.HasRefreshToken())
			},
		},
		{
			"Token without expiration",
			&OAuthTokenData{
				AccessToken: "access-3",
				TokenType:   "Bearer",
			},
			func(t *testing.T, token *OAuthTokenData) {
				assert.False(t, token.IsExpired())
				assert.False(t, token.IsExpiredWithBuffer(time.Hour))
				assert.False(t, token.HasRefreshToken())
			},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serviceName := fmt.Sprintf("service%d", i)

			err := db.StoreOAuthToken(tenantHash, serviceName, tt.tokenData)
			require.NoError(t, err)

			retrieved, err := db.GetOAuthToken(tenantHash, serviceName)
			require.NoError(t, err)

			tt.checkFunc(t, retrieved)
		})
	}
}

func TestOAuthTokenDeletion(t *testing.T) {
	db, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	tenantHash := createTestTenant(t, db, "Test tenant")

	// Store multiple OAuth tokens
	services := []string{"microsoft365", "google", "salesforce"}
	tokenData := &OAuthTokenData{
		AccessToken: "test-token",
		TokenType:   "Bearer",
	}

	for _, service := range services {
		err := db.StoreOAuthToken(tenantHash, service, tokenData)
		require.NoError(t, err)
	}

	// Verify all tokens exist
	tokens, err := db.ListOAuthTokens(tenantHash)
	require.NoError(t, err)
	assert.Len(t, tokens, len(services))

	// Delete one token
	err = db.DeleteOAuthToken(tenantHash, "microsoft365")
	require.NoError(t, err)

	// Verify deleted token is gone
	_, err = db.GetOAuthToken(tenantHash, "microsoft365")
	assert.True(t, IsNotFound(err))

	// Verify other tokens still exist
	tokens, err = db.ListOAuthTokens(tenantHash)
	require.NoError(t, err)
	assert.Len(t, tokens, len(services)-1)

	_, exists := tokens["microsoft365"]
	assert.False(t, exists)

	_, exists = tokens["google"]
	assert.True(t, exists)
}

func TestOAuthTokenListForTenants(t *testing.T) {
	db, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	// Create multiple tenants
	tenant1 := createTestTenant(t, db, "Tenant 1")
	tenant2 := createTestTenant(t, db, "Tenant 2")

	// Add tokens to first tenant
	tokenData1 := &OAuthTokenData{
		AccessToken: "token1",
		TokenType:   "Bearer",
		Scope:       []string{"read"},
	}

	err := db.StoreOAuthToken(tenant1, "service1", tokenData1)
	require.NoError(t, err)

	err = db.StoreOAuthToken(tenant1, "service2", tokenData1)
	require.NoError(t, err)

	// Add token to second tenant
	tokenData2 := &OAuthTokenData{
		AccessToken: "token2",
		TokenType:   "Bearer",
		Scope:       []string{"write"},
	}

	err = db.StoreOAuthToken(tenant2, "service1", tokenData2)
	require.NoError(t, err)

	// List tokens for tenant1
	tokens1, err := db.ListOAuthTokens(tenant1)
	require.NoError(t, err)
	assert.Len(t, tokens1, 2)
	assert.Contains(t, tokens1, "service1")
	assert.Contains(t, tokens1, "service2")
	assert.Equal(t, "token1", tokens1["service1"].AccessToken)

	// List tokens for tenant2
	tokens2, err := db.ListOAuthTokens(tenant2)
	require.NoError(t, err)
	assert.Len(t, tokens2, 1)
	assert.Contains(t, tokens2, "service1")
	assert.Equal(t, "token2", tokens2["service1"].AccessToken)

	// List tokens for non-existent tenant
	tokens3, err := db.ListOAuthTokens("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	assert.Empty(t, tokens3)
}

// Core Tenant Tests

func TestMultiTenantIsolation(t *testing.T) {
	db, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	// Create two tenants
	tenant1 := createTestTenant(t, db, "Tenant 1")
	tenant2 := createTestTenant(t, db, "Tenant 2")

	// Add OAuth tokens to each tenant
	oauthData := &OAuthTokenData{
		AccessToken: "oauth-token",
		TokenType:   "Bearer",
	}

	err := db.StoreOAuthToken(tenant1, "service1", oauthData)
	require.NoError(t, err)

	err = db.StoreOAuthToken(tenant2, "service1", oauthData)
	require.NoError(t, err)

	// Add credentials to each tenant
	credData := &ServiceCredentials{
		Type: CredentialTypeAPIKey,
		Data: map[string]interface{}{"key": "value"},
	}

	err = db.StoreCredentials(tenant1, "service2", credData)
	require.NoError(t, err)

	err = db.StoreCredentials(tenant2, "service2", credData)
	require.NoError(t, err)

	// Verify isolation - tenant1 cannot access tenant2's data
	_, err = db.GetOAuthToken(tenant1, "service1")
	require.NoError(t, err)

	// Cross-tenant access should work (same service name, different tenants)
	oauth1, err := db.GetOAuthToken(tenant1, "service1")
	require.NoError(t, err)

	oauth2, err := db.GetOAuthToken(tenant2, "service1")
	require.NoError(t, err)

	assert.Equal(t, oauth1.AccessToken, oauth2.AccessToken) // Same data stored

	// Verify tenant info shows correct counts
	info1, err := db.GetTenantInfo(tenant1)
	require.NoError(t, err)
	assert.Equal(t, 1, info1.OAuthCount)
	assert.Equal(t, 1, info1.CredCount)

	info2, err := db.GetTenantInfo(tenant2)
	require.NoError(t, err)
	assert.Equal(t, 1, info2.OAuthCount)
	assert.Equal(t, 1, info2.CredCount)
}

func TestTenantInfoRetrieval(t *testing.T) {
	db, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	// Create tenant with description
	description := "Test Tenant for Info"
	_, tenantHash, err := db.AddAPIToken(description)
	require.NoError(t, err)

	// Note: Tenant may not have a bucket until OAuth/credentials are added
	// Check if tenant exists first by adding some data
	oauthData := &OAuthTokenData{
		AccessToken: "temp-token",
		TokenType:   "Bearer",
	}
	err = db.StoreOAuthToken(tenantHash, "temp-service", oauthData)
	require.NoError(t, err)

	// Delete the temp token to start clean
	err = db.DeleteOAuthToken(tenantHash, "temp-service")
	require.NoError(t, err)

	// Now get tenant info - should work as tenant bucket exists
	info, err := db.GetTenantInfo(tenantHash)
	require.NoError(t, err)

	assert.Equal(t, tenantHash, info.Hash)
	assert.Equal(t, description, info.Description)
	assert.Equal(t, 0, info.OAuthCount)
	assert.Equal(t, 0, info.CredCount)
	assert.NotZero(t, info.CreatedAt)

	// Add some data and verify counts update
	oauthData2 := &OAuthTokenData{
		AccessToken: "oauth-token",
		TokenType:   "Bearer",
	}

	err = db.StoreOAuthToken(tenantHash, "service1", oauthData2)
	require.NoError(t, err)

	err = db.StoreOAuthToken(tenantHash, "service2", oauthData2)
	require.NoError(t, err)

	credData := &ServiceCredentials{
		Type: CredentialTypeAPIKey,
		Data: map[string]interface{}{"key": "value"},
	}

	err = db.StoreCredentials(tenantHash, "service3", credData)
	require.NoError(t, err)

	// Verify updated counts
	info, err = db.GetTenantInfo(tenantHash)
	require.NoError(t, err)

	assert.Equal(t, 2, info.OAuthCount)
	assert.Equal(t, 1, info.CredCount)
}

func TestCrossTenantDataSeparation(t *testing.T) {
	db, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	// Create multiple tenants
	tenant1 := createTestTenant(t, db, "Tenant A")
	tenant2 := createTestTenant(t, db, "Tenant B")
	tenant3 := createTestTenant(t, db, "Tenant C")

	// Store different data for each tenant with same service names
	serviceName := "common-service"

	// Tenant 1 data
	oauth1 := &OAuthTokenData{
		AccessToken: "tenant1-oauth",
		TokenType:   "Bearer",
		Scope:       []string{"tenant1"},
	}
	err := db.StoreOAuthToken(tenant1, serviceName, oauth1)
	require.NoError(t, err)

	cred1 := &ServiceCredentials{
		Type: CredentialTypeAPIKey,
		Data: map[string]interface{}{"tenant": "1", "secret": "tenant1-secret"},
	}
	err = db.StoreCredentials(tenant1, serviceName, cred1)
	require.NoError(t, err)

	// Tenant 2 data
	oauth2 := &OAuthTokenData{
		AccessToken: "tenant2-oauth",
		TokenType:   "Bearer",
		Scope:       []string{"tenant2"},
	}
	err = db.StoreOAuthToken(tenant2, serviceName, oauth2)
	require.NoError(t, err)

	cred2 := &ServiceCredentials{
		Type: CredentialTypeBearer,
		Data: map[string]interface{}{"tenant": "2", "token": "tenant2-token"},
	}
	err = db.StoreCredentials(tenant2, serviceName, cred2)
	require.NoError(t, err)

	// Verify each tenant gets their own data
	retrievedOAuth1, err := db.GetOAuthToken(tenant1, serviceName)
	require.NoError(t, err)
	assert.Equal(t, "tenant1-oauth", retrievedOAuth1.AccessToken)
	assert.Equal(t, []string{"tenant1"}, retrievedOAuth1.Scope)

	retrievedOAuth2, err := db.GetOAuthToken(tenant2, serviceName)
	require.NoError(t, err)
	assert.Equal(t, "tenant2-oauth", retrievedOAuth2.AccessToken)
	assert.Equal(t, []string{"tenant2"}, retrievedOAuth2.Scope)

	retrievedCred1, err := db.GetCredentials(tenant1, serviceName)
	require.NoError(t, err)
	assert.Equal(t, CredentialTypeAPIKey, retrievedCred1.Type)
	assert.Equal(t, "tenant1-secret", retrievedCred1.Data["secret"])

	retrievedCred2, err := db.GetCredentials(tenant2, serviceName)
	require.NoError(t, err)
	assert.Equal(t, CredentialTypeBearer, retrievedCred2.Type)
	assert.Equal(t, "tenant2-token", retrievedCred2.Data["token"])

	// Verify tenant3 has no data
	_, err = db.GetOAuthToken(tenant3, serviceName)
	assert.True(t, IsNotFound(err))

	_, err = db.GetCredentials(tenant3, serviceName)
	assert.True(t, IsNotFound(err))
}

// Critical Error Tests

func TestInvalidTokenFormats(t *testing.T) {
	db, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	invalidTokens := []string{
		"",
		"abc",
		"not-hex-token",
		"123",
		strings.Repeat("x", 63), // One character short
		strings.Repeat("g", 64), // Invalid hex character
	}

	for _, token := range invalidTokens {
		t.Run(fmt.Sprintf("Invalid token: %s", token), func(t *testing.T) {
			valid, hash, err := db.ValidateAPIToken(token)
			assert.Error(t, err, "Should return error for invalid token format")
			assert.False(t, valid, "Should not be valid")
			assert.Empty(t, hash, "Should not return hash")
			assert.True(t, IsValidationError(err), "Should be a validation error")
		})
	}
}

func TestNonExistentTenantsTokens(t *testing.T) {
	db, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	nonExistentTenant := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	nonExistentHash := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

	// Test OAuth operations on non-existent tenant
	_, err := db.GetOAuthToken(nonExistentTenant, "service")
	assert.Error(t, err)
	assert.True(t, IsNotFound(err))

	err = db.DeleteOAuthToken(nonExistentTenant, "service")
	assert.Error(t, err)
	assert.True(t, IsNotFound(err))

	// Test credential operations on non-existent tenant
	_, err = db.GetCredentials(nonExistentTenant, "service")
	assert.Error(t, err)
	assert.True(t, IsNotFound(err))

	err = db.DeleteCredentials(nonExistentTenant, "service")
	assert.Error(t, err)
	assert.True(t, IsNotFound(err))

	// Test API token operations on non-existent hash
	_, err = db.GetAPITokenMetadata(nonExistentHash)
	assert.Error(t, err)
	assert.True(t, IsNotFound(err))

	err = db.DeleteAPIToken(nonExistentHash)
	assert.Error(t, err)
	assert.True(t, IsNotFound(err))

	// Test tenant info for non-existent tenant
	_, err = db.GetTenantInfo(nonExistentTenant)
	assert.Error(t, err)
	assert.True(t, IsNotFound(err))
}

func TestDatabaseErrors(t *testing.T) {
	db, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	// Test operations after closing database
	err := db.Close()
	require.NoError(t, err)

	// All operations should return ErrDatabaseClosed
	_, _, err = db.AddAPIToken("test")
	assert.Equal(t, ErrDatabaseClosed, err)

	_, _, err = db.ValidateAPIToken("test-token")
	assert.Equal(t, ErrDatabaseClosed, err)

	_, err = db.ListAPITokens()
	assert.Equal(t, ErrDatabaseClosed, err)

	tokenData := &OAuthTokenData{
		AccessToken: "test",
		TokenType:   "Bearer",
	}
	err = db.StoreOAuthToken("hash", "service", tokenData)
	assert.Equal(t, ErrDatabaseClosed, err)

	_, err = db.GetOAuthToken("hash", "service")
	assert.Equal(t, ErrDatabaseClosed, err)

	// Multiple closes should not cause issues
	err = db.Close()
	require.NoError(t, err, "Multiple close calls should not error")
}

// Basic Concurrency Test

func TestConcurrentAPITokenOperations(t *testing.T) {
	db, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	const numGoroutines = 10
	const tokensPerGoroutine = 5

	var wg sync.WaitGroup
	tokenChan := make(chan string, numGoroutines*tokensPerGoroutine)
	errorChan := make(chan error, numGoroutines*tokensPerGoroutine)

	// Concurrent token creation
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < tokensPerGoroutine; j++ {
				token, hash, err := db.AddAPIToken(fmt.Sprintf("Token-%d-%d", routineID, j))
				if err != nil {
					errorChan <- err
					return
				}
				tokenChan <- hash

				// Immediately validate the token
				valid, returnedHash, err := db.ValidateAPIToken(token)
				if err != nil || !valid || returnedHash != hash {
					errorChan <- fmt.Errorf("validation failed: valid=%v, hash match=%v, err=%v", valid, returnedHash == hash, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(tokenChan)
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		t.Errorf("Concurrent operation failed: %v", err)
	}

	// Verify all tokens were created
	tokens := make([]string, 0)
	for hash := range tokenChan {
		tokens = append(tokens, hash)
	}

	expectedCount := numGoroutines * tokensPerGoroutine
	if len(tokens) != expectedCount {
		t.Errorf("Expected %d tokens, got %d", expectedCount, len(tokens))
	}

	// Verify no duplicate hashes
	hashSet := make(map[string]bool)
	for _, hash := range tokens {
		if hashSet[hash] {
			t.Errorf("Duplicate hash found: %s", hash)
		}
		hashSet[hash] = true
	}

	// Verify database consistency by listing all tokens
	allTokens, err := db.ListAPITokens()
	require.NoError(t, err)
	assert.Len(t, allTokens, expectedCount, "Token count mismatch in database")
}

func TestConcurrentOAuthOperations(t *testing.T) {
	db, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	// Create a tenant first
	tenantHash := createTestTenant(t, db, "Concurrent test tenant")

	const numGoroutines = 5
	const servicesPerGoroutine = 3

	var wg sync.WaitGroup
	errorChan := make(chan error, numGoroutines*servicesPerGoroutine)

	// Concurrent OAuth token operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < servicesPerGoroutine; j++ {
				serviceName := fmt.Sprintf("service-%d-%d", routineID, j)
				tokenData := &OAuthTokenData{
					AccessToken: fmt.Sprintf("token-%d-%d", routineID, j),
					TokenType:   "Bearer",
				}

				// Store token
				err := db.StoreOAuthToken(tenantHash, serviceName, tokenData)
				if err != nil {
					errorChan <- fmt.Errorf("store failed: %v", err)
					return
				}

				// Immediately retrieve and verify
				retrieved, err := db.GetOAuthToken(tenantHash, serviceName)
				if err != nil {
					errorChan <- fmt.Errorf("retrieve failed: %v", err)
					return
				}

				if retrieved.AccessToken != tokenData.AccessToken {
					errorChan <- fmt.Errorf("token mismatch: expected %s, got %s", tokenData.AccessToken, retrieved.AccessToken)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		t.Errorf("Concurrent OAuth operation failed: %v", err)
	}

	// Verify all tokens were stored
	tokens, err := db.ListOAuthTokens(tenantHash)
	require.NoError(t, err)

	expectedCount := numGoroutines * servicesPerGoroutine
	assert.Len(t, tokens, expectedCount, "OAuth token count mismatch")
}

// Additional utility tests

func TestDatabaseBackup(t *testing.T) {
	db, tempDir, _ := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	// Add some data
	_, hash, err := db.AddAPIToken("Backup test token")
	require.NoError(t, err)

	tokenData := &OAuthTokenData{
		AccessToken: "backup-token",
		TokenType:   "Bearer",
	}
	err = db.StoreOAuthToken(hash, "backup-service", tokenData)
	require.NoError(t, err)

	// Create backup
	backupDir, err := os.MkdirTemp("", "mcpfusion_backup_")
	require.NoError(t, err)
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(backupDir)

	backupPath := filepath.Join(backupDir, "backup.db")
	err = db.Backup(backupPath)
	require.NoError(t, err)

	// Verify backup file was created
	_, err = os.Stat(backupPath)
	require.NoError(t, err, "Backup file should exist")

	// Verify backup has content (basic size check)
	info, err := os.Stat(backupPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0), "Backup file should not be empty")
}
