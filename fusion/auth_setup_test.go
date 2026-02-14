/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// authSetupTestLogger implements global.Logger for auth setup tests
type authSetupTestLogger struct {
	t *testing.T
}

func (l *authSetupTestLogger) Debug(msg string)                          { l.t.Log("DEBUG:", msg) }
func (l *authSetupTestLogger) Debugf(format string, args ...interface{}) { l.t.Logf("DEBUG: "+format, args...) }
func (l *authSetupTestLogger) Info(msg string)                           { l.t.Log("INFO:", msg) }
func (l *authSetupTestLogger) Infof(format string, args ...interface{})  { l.t.Logf("INFO: "+format, args...) }
func (l *authSetupTestLogger) Notice(msg string)                         { l.t.Log("NOTICE:", msg) }
func (l *authSetupTestLogger) Noticef(format string, args ...interface{}) {
	l.t.Logf("NOTICE: "+format, args...)
}
func (l *authSetupTestLogger) Warning(msg string)                          { l.t.Log("WARN:", msg) }
func (l *authSetupTestLogger) Warningf(format string, args ...interface{}) { l.t.Logf("WARN: "+format, args...) }
func (l *authSetupTestLogger) Error(msg string)                            { l.t.Log("ERROR:", msg) }
func (l *authSetupTestLogger) Errorf(format string, args ...interface{})   { l.t.Logf("ERROR: "+format, args...) }
func (l *authSetupTestLogger) Fatal(msg string)                            { l.t.Fatal("FATAL:", msg) }
func (l *authSetupTestLogger) Fatalf(format string, args ...interface{}) {
	l.t.Fatalf("FATAL: "+format, args...)
}
func (l *authSetupTestLogger) Close() {}

// newAuthSetupTestFusion creates a Fusion instance with a real database and MultiTenantAuthManager
// for auth setup handler tests. Returns the Fusion instance and a cleanup function.
func newAuthSetupTestFusion(t *testing.T, externalURL string) *Fusion {
	t.Helper()

	logger := &authSetupTestLogger{t: t}

	tempDir, err := os.MkdirTemp("", "auth-setup-test-*")
	require.NoError(t, err, "failed to create temp directory")
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	database, err := db.New(db.WithLogger(logger), db.WithDataDir(tempDir))
	require.NoError(t, err, "failed to create database")
	t.Cleanup(func() { database.Close() })

	mtam := NewMultiTenantAuthManager(database.(*db.DB), nil, logger)

	return &Fusion{
		config: &Config{
			Services: map[string]*ServiceConfig{
				"google": {Name: "Google Workspace", Auth: AuthConfig{Type: AuthTypeOAuth2External}},
				"trello": {Name: "Trello", Auth: AuthConfig{
					Type: AuthTypeUserCredentials,
					Config: map[string]interface{}{
						"instructions": "To use Trello with MCPFusion, you need a Trello API Key and Token.\n\n" +
							"1. Visit https://trello.com/power-ups/admin/ to get your API Key\n" +
							"2. Click 'Generate a Token' link on that page to get your Token\n" +
							"3. Enter both values when prompted below",
					},
				}},
				"basic_creds": {Name: "Basic Service", Auth: AuthConfig{
					Type:   AuthTypeUserCredentials,
					Config: map[string]interface{}{},
				}},
			},
		},
		multiTenantAuth: mtam,
		externalURL:     externalURL,
		logger:          logger,
	}
}

func TestCreateAuthSetupToolDefinition(t *testing.T) {
	f := newAuthSetupTestFusion(t, "http://localhost:8888")

	service := f.config.Services["google"]
	toolDef := f.createAuthSetupToolDefinition("google", service)

	assert.Equal(t, "google_auth_setup", toolDef.Name)
	assert.Contains(t, toolDef.Description, "Google Workspace")
	assert.Empty(t, toolDef.Parameters)
	assert.NotNil(t, toolDef.Handler)

	require.NotNil(t, toolDef.Hints, "hints should not be nil")
	assert.Equal(t, global.BoolPtr(false), toolDef.Hints.ReadOnly)
	assert.Equal(t, global.BoolPtr(false), toolDef.Hints.Destructive)
	assert.Equal(t, global.BoolPtr(true), toolDef.Hints.Idempotent)
	assert.Equal(t, global.BoolPtr(false), toolDef.Hints.OpenWorld)
}

func TestAuthSetupHandler_Success(t *testing.T) {
	f := newAuthSetupTestFusion(t, "http://localhost:8888")

	handler := f.createAuthSetupHandler("google", AuthTypeOAuth2External)

	// Build a context with a valid TenantContext
	tenantCtx := &TenantContext{
		TenantHash:  "abc123def456abc123def456abc123def456abc123def456abc123def456abcd",
		ServiceName: "google",
	}
	ctx := context.WithValue(context.Background(), global.TenantContextKey, tenantCtx)
	options := map[string]any{"__mcp_context": ctx}

	result, err := handler(options)
	require.NoError(t, err, "handler should succeed")

	// The result should contain the fusion-auth command with a base64-encoded blob
	assert.Contains(t, result, "fusion-auth")

	// Extract the base64 blob from the result (it follows "fusion-auth ")
	parts := strings.Split(result, "fusion-auth ")
	require.Len(t, parts, 2, "result should contain exactly one 'fusion-auth ' occurrence")
	blobLine := strings.TrimSpace(strings.Split(parts[1], "\n")[0])

	// Decode the blob
	decoded, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(blobLine)
	require.NoError(t, err, "blob should be valid base64url")

	var blob AuthCodeBlob
	err = json.Unmarshal(decoded, &blob)
	require.NoError(t, err, "decoded blob should be valid JSON")

	assert.Equal(t, "http://localhost:8888", blob.URL)
	assert.Equal(t, "google", blob.Service)
	assert.NotEmpty(t, blob.Code, "auth code should not be empty")
}

func TestAuthSetupHandler_NoTenantContext(t *testing.T) {
	f := newAuthSetupTestFusion(t, "http://localhost:8888")

	handler := f.createAuthSetupHandler("google", AuthTypeOAuth2External)

	// Call without any tenant context in options
	options := map[string]any{}
	_, err := handler(options)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tenant context found")
}

func TestAuthSetupHandler_NoExternalURL(t *testing.T) {
	f := newAuthSetupTestFusion(t, "")

	handler := f.createAuthSetupHandler("google", AuthTypeOAuth2External)

	tenantCtx := &TenantContext{
		TenantHash:  "abc123def456abc123def456abc123def456abc123def456abc123def456abcd",
		ServiceName: "google",
	}
	ctx := context.WithValue(context.Background(), global.TenantContextKey, tenantCtx)
	options := map[string]any{"__mcp_context": ctx}

	_, err := handler(options)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MCP_FUSION_EXTERNAL_URL")
}

func TestAuthCodeBlobRoundTrip(t *testing.T) {
	original := AuthCodeBlob{
		URL:     "https://mcp.example.com:9443",
		Code:    "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4",
		Service: "google",
	}

	// Marshal to JSON
	blobJSON, err := json.Marshal(original)
	require.NoError(t, err, "marshal should succeed")

	// Encode to base64url (no padding, matching auth_setup.go)
	encoded := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(blobJSON)
	assert.NotEmpty(t, encoded)

	// Decode from base64url
	decoded, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(encoded)
	require.NoError(t, err, "decode should succeed")

	// Unmarshal from JSON
	var roundTripped AuthCodeBlob
	err = json.Unmarshal(decoded, &roundTripped)
	require.NoError(t, err, "unmarshal should succeed")

	assert.Equal(t, original.URL, roundTripped.URL)
	assert.Equal(t, original.Code, roundTripped.Code)
	assert.Equal(t, original.Service, roundTripped.Service)
}

func TestAuthSetupHandler_UserCredentialsWithInstructions(t *testing.T) {
	f := newAuthSetupTestFusion(t, "http://localhost:8888")

	handler := f.createAuthSetupHandler("trello", AuthTypeUserCredentials)

	tenantCtx := &TenantContext{
		TenantHash:  "abc123def456abc123def456abc123def456abc123def456abc123def456abcd",
		ServiceName: "trello",
	}
	ctx := context.WithValue(context.Background(), global.TenantContextKey, tenantCtx)
	options := map[string]any{"__mcp_context": ctx}

	result, err := handler(options)
	require.NoError(t, err, "handler should succeed")

	// Verify the message includes the instructions
	assert.Contains(t, result, "Credentials are required for Trello.")
	assert.Contains(t, result, "To use Trello with MCPFusion, you need a Trello API Key and Token.")
	assert.Contains(t, result, "https://trello.com/power-ups/admin/")
	assert.Contains(t, result, "fusion-auth")
	assert.Contains(t, result, "This auth code expires in 15 minutes.")

	// Verify ordering: instructions come before the fusion-auth command
	instructionsIdx := strings.Index(result, "Trello API Key and Token")
	fusionAuthIdx := strings.Index(result, "fusion-auth")
	assert.Greater(t, fusionAuthIdx, instructionsIdx,
		"instructions should appear before the fusion-auth command")
}

func TestAuthSetupHandler_UserCredentialsWithoutInstructions(t *testing.T) {
	f := newAuthSetupTestFusion(t, "http://localhost:8888")

	handler := f.createAuthSetupHandler("basic_creds", AuthTypeUserCredentials)

	tenantCtx := &TenantContext{
		TenantHash:  "abc123def456abc123def456abc123def456abc123def456abc123def456abcd",
		ServiceName: "basic_creds",
	}
	ctx := context.WithValue(context.Background(), global.TenantContextKey, tenantCtx)
	options := map[string]any{"__mcp_context": ctx}

	result, err := handler(options)
	require.NoError(t, err, "handler should succeed")

	// Verify the message works without instructions
	assert.Contains(t, result, "Credentials are required for Basic Service.")
	assert.Contains(t, result, "fusion-auth")
	assert.Contains(t, result, "This auth code expires in 15 minutes.")

	// The message should NOT contain double blank lines between
	// "Credentials are required" and "Please run" (no instructions block)
	assert.NotContains(t, result, "Credentials are required for Basic Service.\n\n\n\n")
}
