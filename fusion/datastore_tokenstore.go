/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/PivotLLM/toolspec"
)

// DataStore collection names. A host that partitions by collection (a table,
// bucket, or directory per collection) keeps token, credential, auth-code, and
// index records apart.
const (
	dsCollectionOAuth     = "oauth"
	dsCollectionCreds     = "creds"
	dsCollectionAuthCodes = "authcodes"
	dsCollectionIndex     = "index"
)

// dataStoreTokenStore adapts a generic toolspec.DataStore (3-method opaque KV) to
// the TokenStore contract the fusion auth path needs. Embedded hosts (e.g. ClawEh)
// implement only DataStore; this adapter owns all fusion-specific encoding so the
// host never learns about OAuth tokens, credentials, or auth codes.
//
// Record layout:
//   - oauth/creds: key "<tenantHash>/<serviceName>", value JSON of the record.
//   - authcodes:   key is the generated code, value JSON of dsAuthCode (carries
//     its own expiry, enforced on read; the store needs no TTL support).
//   - index:       key "<tenantHash>/<collection>", value JSON []string of the
//     service names present, so List* can enumerate without a scan API (DataStore
//     offers only Get/Set/Delete).
type dataStoreTokenStore struct {
	ds     toolspec.DataStore
	logger global.Logger
}

// NewDataStoreTokenStore wraps a toolspec.DataStore as a TokenStore. ds must be
// non-nil; callers gate on host persistence before constructing one.
func NewDataStoreTokenStore(ds toolspec.DataStore, opts ...ComponentOption) TokenStore {
	logger := newComponentOptions(opts).logger
	return &dataStoreTokenStore{ds: ds, logger: logger}
}

var _ TokenStore = (*dataStoreTokenStore)(nil)

// recordKey builds the "<tenantHash>/<serviceName>" key used for oauth/creds.
func recordKey(tenantHash, serviceName string) string {
	return tenantHash + "/" + serviceName
}

// indexKey builds the per-tenant, per-collection index key.
func indexKey(tenantHash, collection string) string {
	return tenantHash + "/" + collection
}

// dsAuthCode is the value stored under the authcodes collection. It mirrors
// db.AuthCodeData but is defined locally so the adapter owns its own encoding.
type dsAuthCode struct {
	TenantHash string    `json:"tenant_hash"`
	Service    string    `json:"service"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// ---- OAuth tokens ----

func (a *dataStoreTokenStore) StoreOAuthToken(tenantHash, serviceName string, tokenData *db.OAuthTokenData) error {
	value, err := json.Marshal(tokenData)
	if err != nil {
		return fmt.Errorf("marshal oauth token: %w", err)
	}
	if err := a.ds.Set(context.Background(), dsCollectionOAuth, recordKey(tenantHash, serviceName), value); err != nil {
		return fmt.Errorf("store oauth token: %w", err)
	}
	return a.addToIndex(tenantHash, dsCollectionOAuth, serviceName)
}

func (a *dataStoreTokenStore) LoadOAuthToken(tenantHash, serviceName string) (*db.OAuthTokenData, error) {
	value, ok, err := a.ds.Get(context.Background(), dsCollectionOAuth, recordKey(tenantHash, serviceName))
	if err != nil {
		return nil, fmt.Errorf("get oauth token: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("oauth token not found for tenant %s service %s", tenantHash, serviceName)
	}
	var tokenData db.OAuthTokenData
	if err := json.Unmarshal(value, &tokenData); err != nil {
		return nil, fmt.Errorf("unmarshal oauth token: %w", err)
	}
	return &tokenData, nil
}

func (a *dataStoreTokenStore) DeleteOAuthToken(tenantHash, serviceName string) error {
	if err := a.ds.Delete(context.Background(), dsCollectionOAuth, recordKey(tenantHash, serviceName)); err != nil {
		return fmt.Errorf("delete oauth token: %w", err)
	}
	return a.removeFromIndex(tenantHash, dsCollectionOAuth, serviceName)
}

func (a *dataStoreTokenStore) ListOAuthTokens(tenantHash string) (map[string]*db.OAuthTokenData, error) {
	services, err := a.readIndex(tenantHash, dsCollectionOAuth)
	if err != nil {
		return nil, err
	}
	result := make(map[string]*db.OAuthTokenData, len(services))
	for _, serviceName := range services {
		tokenData, err := a.LoadOAuthToken(tenantHash, serviceName)
		if err != nil {
			// A stale index entry (record deleted out of band) is skipped, not fatal.
			continue
		}
		result[serviceName] = tokenData
	}
	return result, nil
}

// ---- Service credentials ----

func (a *dataStoreTokenStore) StoreCredentials(tenantHash, serviceName string, credentials *db.ServiceCredentials) error {
	value, err := json.Marshal(credentials)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	if err := a.ds.Set(context.Background(), dsCollectionCreds, recordKey(tenantHash, serviceName), value); err != nil {
		return fmt.Errorf("store credentials: %w", err)
	}
	return a.addToIndex(tenantHash, dsCollectionCreds, serviceName)
}

func (a *dataStoreTokenStore) LoadCredentials(tenantHash, serviceName string) (*db.ServiceCredentials, error) {
	value, ok, err := a.ds.Get(context.Background(), dsCollectionCreds, recordKey(tenantHash, serviceName))
	if err != nil {
		return nil, fmt.Errorf("get credentials: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("credentials not found for tenant %s service %s", tenantHash, serviceName)
	}
	var credentials db.ServiceCredentials
	if err := json.Unmarshal(value, &credentials); err != nil {
		return nil, fmt.Errorf("unmarshal credentials: %w", err)
	}
	return &credentials, nil
}

func (a *dataStoreTokenStore) DeleteCredentials(tenantHash, serviceName string) error {
	if err := a.ds.Delete(context.Background(), dsCollectionCreds, recordKey(tenantHash, serviceName)); err != nil {
		return fmt.Errorf("delete credentials: %w", err)
	}
	return a.removeFromIndex(tenantHash, dsCollectionCreds, serviceName)
}

func (a *dataStoreTokenStore) ListCredentials(tenantHash string) (map[string]*db.ServiceCredentials, error) {
	services, err := a.readIndex(tenantHash, dsCollectionCreds)
	if err != nil {
		return nil, err
	}
	result := make(map[string]*db.ServiceCredentials, len(services))
	for _, serviceName := range services {
		credentials, err := a.LoadCredentials(tenantHash, serviceName)
		if err != nil {
			continue
		}
		result[serviceName] = credentials
	}
	return result, nil
}

// ---- Auth codes ----

func (a *dataStoreTokenStore) CreateAuthCode(tenantHash, service string, ttl time.Duration) (string, error) {
	if tenantHash == "" {
		return "", fmt.Errorf("tenant hash cannot be empty")
	}
	if service == "" {
		return "", fmt.Errorf("service cannot be empty")
	}
	if ttl <= 0 {
		return "", fmt.Errorf("ttl must be positive")
	}

	codeBytes := make([]byte, 16)
	if _, err := rand.Read(codeBytes); err != nil {
		return "", fmt.Errorf("generate auth code: %w", err)
	}
	code := hex.EncodeToString(codeBytes)

	now := time.Now()
	value, err := json.Marshal(dsAuthCode{
		TenantHash: tenantHash,
		Service:    service,
		CreatedAt:  now,
		ExpiresAt:  now.Add(ttl),
	})
	if err != nil {
		return "", fmt.Errorf("marshal auth code: %w", err)
	}
	if err := a.ds.Set(context.Background(), dsCollectionAuthCodes, code, value); err != nil {
		return "", fmt.Errorf("store auth code: %w", err)
	}
	return code, nil
}

// ValidateAuthCode looks up an auth code and enforces expiry on read. It does NOT
// consume the code on success: the auth utility (fusion-auth / claw-auth) completes
// a multi-request flow — ping, then GET the service config, then POST the token —
// each call re-presenting the same code as its bearer. Deleting on first validation
// would 401 every call after the first (the observed embedded-host failure). The
// code stays valid until its TTL expires, matching the standalone bbolt store
// (db.ValidateAuthCode). Expired codes are best-effort deleted here, which is why
// CleanupExpiredAuthCodes is a no-op.
func (a *dataStoreTokenStore) ValidateAuthCode(code string) (string, string, error) {
	if code == "" {
		return "", "", fmt.Errorf("auth code cannot be empty")
	}
	value, ok, err := a.ds.Get(context.Background(), dsCollectionAuthCodes, code)
	if err != nil {
		return "", "", fmt.Errorf("get auth code: %w", err)
	}
	if !ok {
		return "", "", fmt.Errorf("auth code not found")
	}
	var data dsAuthCode
	if err := json.Unmarshal(value, &data); err != nil {
		return "", "", fmt.Errorf("unmarshal auth code: %w", err)
	}
	if time.Now().After(data.ExpiresAt) {
		// Best-effort cleanup of the expired code; report expiry regardless.
		_ = a.ds.Delete(context.Background(), dsCollectionAuthCodes, code)
		return "", "", fmt.Errorf("auth code expired")
	}
	return data.TenantHash, data.Service, nil
}

// CleanupExpiredAuthCodes is a no-op: expiry is enforced on read in
// ValidateAuthCode, and DataStore exposes no scan/iterate API to sweep codes.
func (a *dataStoreTokenStore) CleanupExpiredAuthCodes() error {
	return nil
}

// ---- Per-tenant index maintenance ----
//
// DataStore has no list/scan primitive, so List* is served from a small index
// value (a JSON []string of service names) maintained per tenant per collection.

func (a *dataStoreTokenStore) readIndex(tenantHash, collection string) ([]string, error) {
	value, ok, err := a.ds.Get(context.Background(), dsCollectionIndex, indexKey(tenantHash, collection))
	if err != nil {
		return nil, fmt.Errorf("read %s index: %w", collection, err)
	}
	if !ok {
		return nil, nil
	}
	var services []string
	if err := json.Unmarshal(value, &services); err != nil {
		return nil, fmt.Errorf("unmarshal %s index: %w", collection, err)
	}
	return services, nil
}

func (a *dataStoreTokenStore) addToIndex(tenantHash, collection, serviceName string) error {
	services, err := a.readIndex(tenantHash, collection)
	if err != nil {
		return err
	}
	for _, s := range services {
		if s == serviceName {
			return nil // already present
		}
	}
	services = append(services, serviceName)
	return a.writeIndex(tenantHash, collection, services)
}

func (a *dataStoreTokenStore) removeFromIndex(tenantHash, collection, serviceName string) error {
	services, err := a.readIndex(tenantHash, collection)
	if err != nil {
		return err
	}
	filtered := services[:0]
	for _, s := range services {
		if s != serviceName {
			filtered = append(filtered, s)
		}
	}
	if len(filtered) == len(services) {
		return nil // nothing removed
	}
	return a.writeIndex(tenantHash, collection, filtered)
}

func (a *dataStoreTokenStore) writeIndex(tenantHash, collection string, services []string) error {
	value, err := json.Marshal(services)
	if err != nil {
		return fmt.Errorf("marshal %s index: %w", collection, err)
	}
	if err := a.ds.Set(context.Background(), dsCollectionIndex, indexKey(tenantHash, collection), value); err != nil {
		return fmt.Errorf("write %s index: %w", collection, err)
	}
	return nil
}
