/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"time"

	"github.com/PivotLLM/MCPFusion/db"
)

// TokenStore is the narrow persistence contract the fusion auth path needs. It
// is a strict subset of db.Database covering only OAuth tokens, service
// credentials, and single-use auth codes — the state the multi-tenant auth
// manager and database cache read and write. The concrete *db.DB (bbolt) used by
// standalone MCPFusion already satisfies it, so passing *db.DB where a TokenStore
// is expected is a type widening with no behavior change. Embedded callers
// (e.g. ClawEh) can instead supply a toolspec.DataStore-backed implementation
// via WithDataStore, keeping them free of the full db.Database surface.
type TokenStore interface {
	// OAuth token management
	StoreOAuthToken(tenantHash, serviceName string, tokenData *db.OAuthTokenData) error
	LoadOAuthToken(tenantHash, serviceName string) (*db.OAuthTokenData, error)
	DeleteOAuthToken(tenantHash, serviceName string) error
	ListOAuthTokens(tenantHash string) (map[string]*db.OAuthTokenData, error)

	// Service credentials management
	StoreCredentials(tenantHash, serviceName string, credentials *db.ServiceCredentials) error
	LoadCredentials(tenantHash, serviceName string) (*db.ServiceCredentials, error)
	DeleteCredentials(tenantHash, serviceName string) error
	ListCredentials(tenantHash string) (map[string]*db.ServiceCredentials, error)

	// Single-use auth code management
	CreateAuthCode(tenantHash, service string, ttl time.Duration) (string, error)
	ValidateAuthCode(code string) (tenantHash, service string, err error)
	CleanupExpiredAuthCodes() error
}

// *db.DB satisfies TokenStore. Enforced at compile time so the standalone seam
// stays a pure interface widening.
var _ TokenStore = (*db.DB)(nil)
