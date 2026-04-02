/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package mcpserver

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tenebris-tech/mlogger"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/global"
)

// mockServiceProvider implements ServiceProvider for tests.
type mockServiceProvider struct {
	services []string
}

func (m *mockServiceProvider) GetAvailableServices() []string {
	return m.services
}

func (m *mockServiceProvider) GetService(name string) (*fusion.ServiceConfig, error) {
	return nil, nil
}

func (m *mockServiceProvider) GetServiceAuthConfig(name string) (*fusion.AuthConfig, error) {
	return nil, nil
}

// newTestAuthManager creates a MultiTenantAuthManager with no database (testing mode).
// ExtractTenantFromToken with a non-empty token SHA256-hashes the token as the tenant ID.
// ExtractTenantFromToken with empty token returns the NOAUTH context.
func newTestAuthManager() *fusion.MultiTenantAuthManager {
	return fusion.NewMultiTenantAuthManager(nil, nil, nil)
}

// newTestAuthManagerWithDB creates a MultiTenantAuthManager backed by a real BoltDB
// in a temporary directory. The caller must close the database and remove the tempDir.
func newTestAuthManagerWithDB(t *testing.T) (*fusion.MultiTenantAuthManager, db.Database, string) {
	t.Helper()
	logger := mlogger.NewMemoryLogger()
	tempDir, err := os.MkdirTemp("", "mcpfusion_auth_test_")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	database, err := db.New(db.WithLogger(logger), db.WithDataDir(tempDir))
	if err != nil {
		_ = os.RemoveAll(tempDir)
		t.Fatalf("failed to create test database: %v", err)
	}
	manager := fusion.NewMultiTenantAuthManager(database.(*db.DB), nil, nil)
	return manager, database, tempDir
}

// okHandler is a trivial downstream HTTP handler that writes 200 OK.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// ---------------------------------------------------------------------------
// P0-1: Bearer token validation in AuthMiddleware.SimpleMiddleware
// ---------------------------------------------------------------------------

// TestSimpleMiddleware_MissingAuthHeader ensures requests without an Authorization
// header are rejected with 401 when requireAuth is true.
func TestSimpleMiddleware_MissingAuthHeader(t *testing.T) {
	am := NewAuthMiddleware(newTestAuthManager(), nil, WithRequireAuth(true))
	handler := am.SimpleMiddleware(okHandler)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// TestSimpleMiddleware_WrongScheme ensures requests using non-Bearer auth (e.g. Basic)
// are rejected with 401 when requireAuth is true.
func TestSimpleMiddleware_WrongScheme(t *testing.T) {
	am := NewAuthMiddleware(newTestAuthManager(), nil, WithRequireAuth(true))
	handler := am.SimpleMiddleware(okHandler)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for Basic auth, got %d", rr.Code)
	}
}

// TestSimpleMiddleware_EmptyBearerToken ensures "Authorization: Bearer " (empty token)
// is rejected with 401 when requireAuth is true.
func TestSimpleMiddleware_EmptyBearerToken(t *testing.T) {
	am := NewAuthMiddleware(newTestAuthManager(), nil, WithRequireAuth(true))
	handler := am.SimpleMiddleware(okHandler)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer ")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for empty bearer token, got %d", rr.Code)
	}
}

// TestSimpleMiddleware_ValidToken ensures a valid token (any non-empty token in
// no-DB mode) passes through and the tenant context is set in the downstream handler.
func TestSimpleMiddleware_ValidToken(t *testing.T) {
	am := NewAuthMiddleware(newTestAuthManager(), nil, WithRequireAuth(true))

	var capturedTenant *fusion.TenantContext
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenant, _ = r.Context().Value(global.TenantContextKey).(*fusion.TenantContext)
		w.WriteHeader(http.StatusOK)
	})

	handler := am.SimpleMiddleware(captureHandler)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer valid-test-token-123")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if capturedTenant == nil {
		t.Fatal("expected tenant context in downstream handler, got nil")
	}
	if capturedTenant.TenantHash == "" {
		t.Error("expected non-empty tenant hash")
	}
}

// TestSimpleMiddleware_RevokedToken simulates a revoked token by using a real DB,
// adding a token, then deleting it, and verifying subsequent requests get 401.
func TestSimpleMiddleware_RevokedToken(t *testing.T) {
	manager, database, tempDir := newTestAuthManagerWithDB(t)
	defer func() {
		_ = database.Close()
		_ = os.RemoveAll(tempDir)
	}()

	// Add a token.
	token, _, err := database.AddAPIToken("test token")
	if err != nil {
		t.Fatalf("failed to add API token: %v", err)
	}

	// Validate it works first.
	valid, hash, err := database.ValidateAPIToken(token)
	if err != nil || !valid {
		t.Fatalf("expected token to be valid before deletion: valid=%v err=%v", valid, err)
	}

	// Delete the token.
	if err := database.DeleteAPIToken(hash); err != nil {
		t.Fatalf("failed to delete API token: %v", err)
	}

	am := NewAuthMiddleware(manager, nil, WithRequireAuth(true))
	handler := am.SimpleMiddleware(okHandler)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for revoked token, got %d", rr.Code)
	}
}

// TestSimpleMiddleware_SkipPath ensures paths in the skip list bypass auth entirely.
func TestSimpleMiddleware_SkipPath(t *testing.T) {
	am := NewAuthMiddleware(newTestAuthManager(), nil,
		WithRequireAuth(true),
		WithSkipPaths("/health", "/metrics"),
	)
	handler := am.SimpleMiddleware(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	// No Authorization header intentionally.
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for skipped path, got %d", rr.Code)
	}
}

// TestSimpleMiddleware_NoAuthMode ensures that when requireAuth is false and no
// Authorization header is provided, the request passes with a NOAUTH tenant context.
func TestSimpleMiddleware_NoAuthMode(t *testing.T) {
	am := NewAuthMiddleware(newTestAuthManager(), nil, WithRequireAuth(false))

	var capturedTenant *fusion.TenantContext
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenant, _ = r.Context().Value(global.TenantContextKey).(*fusion.TenantContext)
		w.WriteHeader(http.StatusOK)
	})

	handler := am.SimpleMiddleware(captureHandler)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 in no-auth mode, got %d", rr.Code)
	}
	if capturedTenant == nil {
		t.Fatal("expected NOAUTH tenant context, got nil")
	}
	if capturedTenant.TenantHash != fusion.NoAuthTenantHash {
		t.Errorf("expected NOAUTH tenant hash %q, got %q",
			fusion.NoAuthTenantHash, capturedTenant.TenantHash)
	}
}

// ---------------------------------------------------------------------------
// P0-2: Auth code fallback path in SimpleMiddleware
// ---------------------------------------------------------------------------

// TestSimpleMiddleware_AuthCodeFallback_Success verifies that when the bearer
// token fails (unknown token against a real DB) but a valid auth code exists,
// the request is authenticated using the auth code.
func TestSimpleMiddleware_AuthCodeFallback_Success(t *testing.T) {
	manager, database, tempDir := newTestAuthManagerWithDB(t)
	defer func() {
		_ = database.Close()
		_ = os.RemoveAll(tempDir)
	}()

	// Create an API token to get a tenant hash.
	_, hash, err := database.AddAPIToken("test tenant")
	if err != nil {
		t.Fatalf("failed to add API token: %v", err)
	}

	// Create an auth code for that tenant.
	authCode, err := database.CreateAuthCode(hash, "google", 5*time.Minute)
	if err != nil {
		t.Fatalf("failed to create auth code: %v", err)
	}

	am := NewAuthMiddleware(manager, nil, WithRequireAuth(true))

	var capturedTenant *fusion.TenantContext
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenant, _ = r.Context().Value(global.TenantContextKey).(*fusion.TenantContext)
		w.WriteHeader(http.StatusOK)
	})

	handler := am.SimpleMiddleware(captureHandler)

	// Use the auth code as the Bearer value (a token that fails regular DB validation
	// but succeeds as an auth code in the fallback path).
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+authCode)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for auth code fallback, got %d (body: %s)",
			rr.Code, rr.Body.String())
	}
	if capturedTenant == nil {
		t.Fatal("expected tenant context from auth code, got nil")
	}
	if capturedTenant.TenantHash != hash {
		t.Errorf("expected tenant hash %q from auth code, got %q", hash, capturedTenant.TenantHash)
	}
}

// TestSimpleMiddleware_AuthCodeFallback_BothFail verifies that when both the bearer
// token and the auth code fail, the request is rejected with 401.
func TestSimpleMiddleware_AuthCodeFallback_BothFail(t *testing.T) {
	manager, database, tempDir := newTestAuthManagerWithDB(t)
	defer func() {
		_ = database.Close()
		_ = os.RemoveAll(tempDir)
	}()

	am := NewAuthMiddleware(manager, nil, WithRequireAuth(true))
	handler := am.SimpleMiddleware(okHandler)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer completely-invalid-token-and-not-an-auth-code")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when both token and auth code fail, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// P1-10: peekJSONRPCMethod body restore
// ---------------------------------------------------------------------------

// TestPeekJSONRPCMethod_ValidBody verifies that peekJSONRPCMethod extracts the
// "method" field from a JSON-RPC request body and restores the body for downstream
// handlers to read again.
func TestPeekJSONRPCMethod_ValidBody(t *testing.T) {
	bodyJSON := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"test_tool"}}`
	bodyBytes := []byte(bodyJSON)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(bodyBytes))

	method := peekJSONRPCMethod(req)

	if method != "tools/call" {
		t.Errorf("expected method %q, got %q", "tools/call", method)
	}

	// Body must be fully readable again after the peek.
	restored, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("failed to read restored body: %v", err)
	}
	if string(restored) != bodyJSON {
		t.Errorf("body not fully restored\nwant: %s\ngot:  %s", bodyJSON, string(restored))
	}
}

// TestPeekJSONRPCMethod_EmptyBody verifies that peekJSONRPCMethod handles a nil
// body gracefully, returning an empty string without panicking.
func TestPeekJSONRPCMethod_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	// httptest.NewRequest sets Body to http.NoBody when passed nil; set to nil explicitly.
	req.Body = nil

	method := peekJSONRPCMethod(req)

	if method != "" {
		t.Errorf("expected empty method for nil body, got %q", method)
	}
}

// TestPeekJSONRPCMethod_EmptyBodyReader verifies handling of an empty (zero-byte) body.
func TestPeekJSONRPCMethod_EmptyBodyReader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(""))

	method := peekJSONRPCMethod(req)

	if method != "" {
		t.Errorf("expected empty method for empty body, got %q", method)
	}
}

// TestPeekJSONRPCMethod_InvalidJSON verifies that peekJSONRPCMethod returns an
// empty string for malformed JSON without panicking. The body is still restored.
func TestPeekJSONRPCMethod_InvalidJSON(t *testing.T) {
	badJSON := `not json at all {{{{`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(badJSON))

	method := peekJSONRPCMethod(req)

	if method != "" {
		t.Errorf("expected empty method for invalid JSON, got %q", method)
	}

	// Body should still be restored even for invalid JSON.
	restored, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("failed to read body after invalid JSON peek: %v", err)
	}
	if string(restored) != badJSON {
		t.Errorf("body not restored after invalid JSON\nwant: %s\ngot:  %s", badJSON, string(restored))
	}
}

// TestPeekJSONRPCMethod_NoMethodField verifies that JSON without a "method" field
// returns an empty string.
func TestPeekJSONRPCMethod_NoMethodField(t *testing.T) {
	bodyJSON := `{"jsonrpc":"2.0","params":{"name":"test_tool"}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(bodyJSON))

	method := peekJSONRPCMethod(req)

	if method != "" {
		t.Errorf("expected empty method when no method field present, got %q", method)
	}
}
