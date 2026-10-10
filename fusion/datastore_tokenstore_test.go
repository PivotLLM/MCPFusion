/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tenebris-tech/mlogger/testlogger"
)

// memDataStore is an in-memory toolspec.DataStore for exercising the
// DataStore-backed TokenStore adapter without a real host backend.
type memDataStore struct {
	mu   sync.Mutex
	data map[string]map[string][]byte // collection -> key -> value
}

func newMemDataStore() *memDataStore {
	return &memDataStore{data: make(map[string]map[string][]byte)}
}

func (m *memDataStore) Get(_ context.Context, collection, key string) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.data[collection]; ok {
		if v, ok := c[key]; ok {
			cp := make([]byte, len(v))
			copy(cp, v)
			return cp, true, nil
		}
	}
	return nil, false, nil
}

func (m *memDataStore) Set(_ context.Context, collection, key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[collection]; !ok {
		m.data[collection] = make(map[string][]byte)
	}
	cp := make([]byte, len(value))
	copy(cp, value)
	m.data[collection][key] = cp
	return nil
}

func (m *memDataStore) Delete(_ context.Context, collection, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.data[collection]; ok {
		delete(c, key)
	}
	return nil
}

func newTestTokenStore(t *testing.T) (TokenStore, *memDataStore) {
	t.Helper()
	ds := newMemDataStore()
	return NewDataStoreTokenStore(ds, WithLogger(testlogger.New(t))), ds
}

func TestDataStoreTokenStore_OAuthRoundTrip(t *testing.T) {
	store, _ := newTestTokenStore(t)
	const tenant = "tenant-a"
	const service = "google"

	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	token := &db.OAuthTokenData{
		AccessToken:  "access-123",
		RefreshToken: "refresh-456",
		TokenType:    "Bearer",
		ExpiresAt:    &expires,
		Scope:        []string{"calendar", "mail"},
		Metadata:     map[string]string{"k": "v"},
	}

	require.NoError(t, store.StoreOAuthToken(tenant, service, token))

	got, err := store.LoadOAuthToken(tenant, service)
	require.NoError(t, err)
	assert.Equal(t, token.AccessToken, got.AccessToken)
	assert.Equal(t, token.RefreshToken, got.RefreshToken)
	assert.Equal(t, token.TokenType, got.TokenType)
	require.NotNil(t, got.ExpiresAt)
	assert.True(t, expires.Equal(*got.ExpiresAt))
	assert.Equal(t, token.Scope, got.Scope)
	assert.Equal(t, token.Metadata, got.Metadata)

	// List reflects the stored token via the per-tenant index.
	list, err := store.ListOAuthTokens(tenant)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "access-123", list[service].AccessToken)

	// Delete removes both the record and the index entry.
	require.NoError(t, store.DeleteOAuthToken(tenant, service))
	_, err = store.LoadOAuthToken(tenant, service)
	assert.Error(t, err, "token should be gone after delete")

	list, err = store.ListOAuthTokens(tenant)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestDataStoreTokenStore_MultipleServicesAndTenantIsolation(t *testing.T) {
	store, _ := newTestTokenStore(t)

	require.NoError(t, store.StoreOAuthToken("tenant-a", "google", &db.OAuthTokenData{AccessToken: "a-google"}))
	require.NoError(t, store.StoreOAuthToken("tenant-a", "microsoft365", &db.OAuthTokenData{AccessToken: "a-ms"}))
	require.NoError(t, store.StoreOAuthToken("tenant-b", "google", &db.OAuthTokenData{AccessToken: "b-google"}))

	listA, err := store.ListOAuthTokens("tenant-a")
	require.NoError(t, err)
	assert.Len(t, listA, 2)
	assert.Equal(t, "a-google", listA["google"].AccessToken)
	assert.Equal(t, "a-ms", listA["microsoft365"].AccessToken)

	listB, err := store.ListOAuthTokens("tenant-b")
	require.NoError(t, err)
	assert.Len(t, listB, 1)
	assert.Equal(t, "b-google", listB["google"].AccessToken)
}

func TestDataStoreTokenStore_CredentialsRoundTrip(t *testing.T) {
	store, _ := newTestTokenStore(t)
	const tenant = "tenant-a"
	const service = "trello"

	creds := &db.ServiceCredentials{
		Type: db.CredentialTypeAPIKey,
		Data: map[string]any{"apiKey": "secret", "token": "tok"},
	}
	require.NoError(t, store.StoreCredentials(tenant, service, creds))

	got, err := store.LoadCredentials(tenant, service)
	require.NoError(t, err)
	assert.Equal(t, db.CredentialTypeAPIKey, got.Type)
	assert.Equal(t, "secret", got.Data["apiKey"])
	assert.Equal(t, "tok", got.Data["token"])

	list, err := store.ListCredentials(tenant)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, db.CredentialTypeAPIKey, list[service].Type)

	require.NoError(t, store.DeleteCredentials(tenant, service))
	_, err = store.LoadCredentials(tenant, service)
	assert.Error(t, err)

	list, err = store.ListCredentials(tenant)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestDataStoreTokenStore_AuthCodeValidReusableUntilTTL(t *testing.T) {
	store, _ := newTestTokenStore(t)

	code, err := store.CreateAuthCode("tenant-a", "google", 15*time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, code)

	// The auth utility re-presents the same code across several calls (ping,
	// service config, token store), so a valid code must keep validating until it
	// expires — not be consumed on first use (matches the standalone bbolt store).
	for i := 0; i < 3; i++ {
		tenant, service, err := store.ValidateAuthCode(code)
		require.NoError(t, err, "validation %d should succeed within TTL", i)
		assert.Equal(t, "tenant-a", tenant)
		assert.Equal(t, "google", service)
	}
}

func TestDataStoreTokenStore_AuthCodeExpiry(t *testing.T) {
	store, _ := newTestTokenStore(t)

	code, err := store.CreateAuthCode("tenant-a", "google", time.Nanosecond)
	require.NoError(t, err)

	time.Sleep(2 * time.Millisecond)

	_, _, err = store.ValidateAuthCode(code)
	assert.Error(t, err, "expired auth code should not validate")

	// After an expired read the code is cleaned up and stays invalid.
	_, _, err = store.ValidateAuthCode(code)
	assert.Error(t, err)
}

func TestDataStoreTokenStore_AuthCodeValidationErrors(t *testing.T) {
	store, _ := newTestTokenStore(t)

	_, _, err := store.ValidateAuthCode("")
	assert.Error(t, err, "empty code must error")

	_, _, err = store.ValidateAuthCode("does-not-exist")
	assert.Error(t, err, "unknown code must error")

	_, err = store.CreateAuthCode("", "google", time.Minute)
	assert.Error(t, err, "empty tenant must error")

	_, err = store.CreateAuthCode("tenant-a", "", time.Minute)
	assert.Error(t, err, "empty service must error")

	_, err = store.CreateAuthCode("tenant-a", "google", 0)
	assert.Error(t, err, "non-positive ttl must error")
}

func TestDataStoreTokenStore_CleanupExpiredIsNoOp(t *testing.T) {
	store, _ := newTestTokenStore(t)
	assert.NoError(t, store.CleanupExpiredAuthCodes())
}

func TestDataStoreTokenStore_GetMissing(t *testing.T) {
	store, _ := newTestTokenStore(t)

	_, err := store.LoadOAuthToken("tenant-a", "missing")
	assert.Error(t, err)

	list, err := store.ListOAuthTokens("tenant-a")
	require.NoError(t, err)
	assert.Empty(t, list, "listing an unknown tenant yields an empty map, not an error")
}
