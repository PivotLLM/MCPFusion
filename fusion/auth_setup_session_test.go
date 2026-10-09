/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"testing"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tenebris-tech/mlogger/testlogger"
)

func TestRegisterTools_AuthSetupForSessionCredentialsOnly(t *testing.T) {
	logger := testlogger.New(t)
	uem := newFakeUEM(t)
	plain := &ServiceConfig{
		Name: "Plain", ServiceKey: "plain", BaseURL: uem.URL,
		Auth: AuthConfig{Type: AuthTypeSessionJWT, Config: map[string]any{
			"loginURL": "/api/v1/login", "tokenPath": "access_token", "tokenLocation": "header",
		}},
	}
	f, err := New(WithLogger(logger), WithConfig(&Config{Services: map[string]*ServiceConfig{
		"uem":   sessionService(uem.URL, CredentialStoreCredentials),
		"plain": plain,
	}}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	names := map[string]global.ToolDefinition{}
	for _, tool := range f.RegisterTools() {
		names[tool.Name] = tool
	}
	setup, ok := names["uem_auth_setup"]
	require.True(t, ok, "session_jwt with credentials must register an auth setup tool")
	assert.Contains(t, setup.Description, "credentials")
	_, ok = names["plain_auth_setup"]
	assert.False(t, ok, "plain session_jwt must not register an auth setup tool")
	_, ok = names["uem_ping"]
	assert.True(t, ok)
}

func TestAuthSetupHandler_SessionCredentials_ClearsStoredValues(t *testing.T) {
	logger := testlogger.New(t)
	uem := newFakeUEM(t)
	mtam := newAuthManager(t, newBoltTokenStore(t, logger), logger)
	f := &Fusion{
		config:          &Config{Services: map[string]*ServiceConfig{"uem": sessionService(uem.URL, CredentialStoreCredentials)}},
		multiTenantAuth: mtam,
		externalURL:     "http://localhost:8888",
		logger:          logger,
	}

	require.NoError(t, mtam.StoreUserCredentials(testTenantHash, "uem", map[string]string{"username": testUser, "password": testPassword}))
	require.NoError(t, mtam.StoreOAuthToken(testTenantHash, "uem", &db.OAuthTokenData{AccessToken: "tok"}))

	handler := f.createAuthSetupHandler("uem", AuthTypeSessionJWT)
	ctx := context.WithValue(context.Background(), global.TenantContextKey, tenant("uem"))
	result, err := handler(map[string]any{"__mcp_context": ctx})
	require.NoError(t, err)
	assert.Contains(t, result, "Credentials are required for UnifyEM")
	assert.Contains(t, result, "Enter your UnifyEM administrator credentials.")
	assert.Contains(t, result, "fusion-auth ")
	assert.NotContains(t, result, "web browser")

	_, err = mtam.LoadUserCredentials(testTenantHash, "uem")
	assert.Error(t, err, "old credentials must be cleared before the user enters new ones")
	_, err = mtam.db.LoadOAuthToken(testTenantHash, "uem")
	assert.Error(t, err, "old token must be cleared")
}

func TestAuthSetupHandler_UserCredentials_DoesNotTouchCredentialStore(t *testing.T) {
	// Regression guard: user_credentials services keep their values in the
	// token record, and InvalidateCredentials is only called for session_jwt.
	f := newAuthSetupTestFusion(t, "http://localhost:8888")
	mtam := f.multiTenantAuth
	require.NoError(t, mtam.StoreUserCredentials(testTenantHash, "trello", map[string]string{"key": "k"}))

	handler := f.createAuthSetupHandler("trello", AuthTypeUserCredentials)
	ctx := context.WithValue(context.Background(), global.TenantContextKey, tenant("trello"))
	_, err := handler(map[string]any{"__mcp_context": ctx})
	require.NoError(t, err)
	_, err = mtam.LoadUserCredentials(testTenantHash, "trello")
	assert.NoError(t, err)
}
