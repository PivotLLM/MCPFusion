/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// P0-3: BearerTokenStrategy unit tests
// ---------------------------------------------------------------------------

func TestBearerTokenStrategy_GetAuthType(t *testing.T) {
	s := NewBearerTokenStrategy(nil)
	if s.GetAuthType() != AuthTypeBearer {
		t.Errorf("GetAuthType() = %v, want %v", s.GetAuthType(), AuthTypeBearer)
	}
}

func TestBearerTokenStrategy_SupportsRefresh(t *testing.T) {
	s := NewBearerTokenStrategy(nil)
	if s.SupportsRefresh() {
		t.Error("SupportsRefresh() should return false for BearerTokenStrategy")
	}
}

func TestBearerTokenStrategy_Authenticate_WithToken(t *testing.T) {
	s := NewBearerTokenStrategy(nil)
	config := map[string]interface{}{
		"token": "my-static-bearer-token",
	}

	tokenInfo, err := s.Authenticate(context.Background(), config)
	if err != nil {
		t.Fatalf("Authenticate() returned unexpected error: %v", err)
	}
	if tokenInfo == nil {
		t.Fatal("Authenticate() returned nil tokenInfo, want non-nil")
	}
	if tokenInfo.AccessToken != "my-static-bearer-token" {
		t.Errorf("AccessToken = %q, want %q", tokenInfo.AccessToken, "my-static-bearer-token")
	}
	if tokenInfo.TokenType != "Bearer" {
		t.Errorf("TokenType = %q, want %q", tokenInfo.TokenType, "Bearer")
	}
}

func TestBearerTokenStrategy_Authenticate_EmptyToken(t *testing.T) {
	s := NewBearerTokenStrategy(nil)
	config := map[string]interface{}{
		"token": "",
	}

	_, err := s.Authenticate(context.Background(), config)
	if err == nil {
		t.Fatal("Authenticate() expected error for empty token, got nil")
	}
}

func TestBearerTokenStrategy_Authenticate_MissingToken(t *testing.T) {
	s := NewBearerTokenStrategy(nil)
	config := map[string]interface{}{}

	_, err := s.Authenticate(context.Background(), config)
	if err == nil {
		t.Fatal("Authenticate() expected error for missing token, got nil")
	}
}

func TestBearerTokenStrategy_ApplyAuth_SetsHeader(t *testing.T) {
	s := NewBearerTokenStrategy(nil)
	tokenInfo := &TokenInfo{
		AccessToken: "test-bearer-value",
		TokenType:   "Bearer",
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/api", nil)
	if err := s.ApplyAuth(req, tokenInfo, nil); err != nil {
		t.Fatalf("ApplyAuth() returned unexpected error: %v", err)
	}

	got := req.Header.Get("Authorization")
	want := "Bearer test-bearer-value"
	if got != want {
		t.Errorf("Authorization header = %q, want %q", got, want)
	}
}

func TestBearerTokenStrategy_ApplyAuth_NilTokenInfo(t *testing.T) {
	s := NewBearerTokenStrategy(nil)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/api", nil)
	err := s.ApplyAuth(req, nil, nil)
	if err == nil {
		t.Fatal("ApplyAuth() expected error for nil tokenInfo, got nil")
	}
}

// ---------------------------------------------------------------------------
// P1-1: ApplyAuthentication end-to-end
// ---------------------------------------------------------------------------

func TestApplyAuthentication_AuthTypeNone(t *testing.T) {
	manager := NewMultiTenantAuthManager(nil, newMockCache(), nil)

	tenantCtx := &TenantContext{
		TenantHash:  "abc123def456abc1",
		ServiceName: "test_service",
		CreatedAt:   time.Now(),
	}
	authConfig := AuthConfig{
		Type:   AuthTypeNone,
		Config: map[string]interface{}{},
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/api", nil)
	err := manager.ApplyAuthentication(context.Background(), req, tenantCtx, authConfig)
	if err != nil {
		t.Fatalf("ApplyAuthentication() with auth type 'none' returned unexpected error: %v", err)
	}

	// No Authorization header should be set
	if got := req.Header.Get("Authorization"); got != "" {
		t.Errorf("Authorization header = %q, want empty for auth type 'none'", got)
	}
}

func TestApplyAuthentication_BearerWithStaticToken(t *testing.T) {
	manager := NewMultiTenantAuthManager(nil, newMockCache(), nil)

	tenantCtx := &TenantContext{
		TenantHash:  "abc123def456abc1",
		ServiceName: "test_service",
		CreatedAt:   time.Now(),
	}
	authConfig := AuthConfig{
		Type: AuthTypeBearer,
		Config: map[string]interface{}{
			"token": "static-bearer-token",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/api", nil)
	err := manager.ApplyAuthentication(context.Background(), req, tenantCtx, authConfig)
	if err != nil {
		t.Fatalf("ApplyAuthentication() with bearer token returned unexpected error: %v", err)
	}

	got := req.Header.Get("Authorization")
	want := "Bearer static-bearer-token"
	if got != want {
		t.Errorf("Authorization header = %q, want %q", got, want)
	}
}

func TestApplyAuthentication_NilTenantContext(t *testing.T) {
	manager := NewMultiTenantAuthManager(nil, newMockCache(), nil)

	authConfig := AuthConfig{
		Type:   AuthTypeNone,
		Config: map[string]interface{}{},
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/api", nil)
	err := manager.ApplyAuthentication(context.Background(), req, nil, authConfig)
	if err == nil {
		t.Fatal("ApplyAuthentication() with nil tenant context expected error, got nil")
	}
	if !strings.Contains(err.Error(), "tenant context is required") {
		t.Errorf("error = %q, want to contain 'tenant context is required'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// P0-6: ExtractTenantFromToken and ExtractTenantFromAuthCode
//
// NOTE: MultiTenantAuthManager accepts *db.DB (concrete type), so the DB code
// paths can only be exercised via integration tests that create a real DB.
// These unit tests cover the no-DB fallback paths which are well-defined and
// testable without a database dependency.
// ---------------------------------------------------------------------------

func TestExtractTenantFromToken_EmptyToken_UsesNoAuth(t *testing.T) {
	manager := NewMultiTenantAuthManager(nil, newMockCache(), nil)

	tenantCtx, err := manager.ExtractTenantFromToken("")
	if err != nil {
		t.Fatalf("ExtractTenantFromToken(\"\") returned unexpected error: %v", err)
	}
	if tenantCtx == nil {
		t.Fatal("ExtractTenantFromToken(\"\") returned nil TenantContext")
	}
	if tenantCtx.TenantHash != NoAuthTenantHash {
		t.Errorf("TenantHash = %q, want %q", tenantCtx.TenantHash, NoAuthTenantHash)
	}
}

func TestExtractTenantFromToken_BearerPrefix_Stripped(t *testing.T) {
	// With no DB, the code hashes the token after stripping the prefix.
	// Both "Bearer mytoken" and "mytoken" should produce the same tenant hash.
	manager := NewMultiTenantAuthManager(nil, newMockCache(), nil)

	tc1, err := manager.ExtractTenantFromToken("Bearer mytoken")
	if err != nil {
		t.Fatalf("ExtractTenantFromToken(\"Bearer mytoken\") error: %v", err)
	}

	tc2, err := manager.ExtractTenantFromToken("mytoken")
	if err != nil {
		t.Fatalf("ExtractTenantFromToken(\"mytoken\") error: %v", err)
	}

	if tc1.TenantHash != tc2.TenantHash {
		t.Errorf("TenantHash with 'Bearer ' prefix (%q) != without prefix (%q)",
			tc1.TenantHash, tc2.TenantHash)
	}
}

func TestExtractTenantFromToken_NoDB_HashesToken(t *testing.T) {
	// Without a DB, the fallback path hashes the token using SHA-256.
	manager := NewMultiTenantAuthManager(nil, newMockCache(), nil)

	tenantCtx, err := manager.ExtractTenantFromToken("some-api-key-value")
	if err != nil {
		t.Fatalf("ExtractTenantFromToken() returned unexpected error: %v", err)
	}
	if tenantCtx == nil {
		t.Fatal("ExtractTenantFromToken() returned nil TenantContext")
	}
	// SHA-256 hex is 64 characters
	if len(tenantCtx.TenantHash) != 64 {
		t.Errorf("TenantHash length = %d, want 64 (SHA-256 hex)", len(tenantCtx.TenantHash))
	}
}

func TestExtractTenantFromToken_NoDB_DifferentTokensDifferentHashes(t *testing.T) {
	manager := NewMultiTenantAuthManager(nil, newMockCache(), nil)

	tc1, err := manager.ExtractTenantFromToken("token-alpha")
	if err != nil {
		t.Fatalf("first ExtractTenantFromToken() error: %v", err)
	}

	tc2, err := manager.ExtractTenantFromToken("token-beta")
	if err != nil {
		t.Fatalf("second ExtractTenantFromToken() error: %v", err)
	}

	if tc1.TenantHash == tc2.TenantHash {
		t.Error("Different tokens produced the same tenant hash (collision)")
	}
}

func TestExtractTenantFromAuthCode_NoDB_ReturnsError(t *testing.T) {
	manager := NewMultiTenantAuthManager(nil, newMockCache(), nil)

	_, err := manager.ExtractTenantFromAuthCode("some-auth-code")
	if err == nil {
		t.Fatal("ExtractTenantFromAuthCode() expected error when db is nil, got nil")
	}
	if !strings.Contains(err.Error(), "database not available") {
		t.Errorf("error = %q, want to contain 'database not available'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// P1-9: OAuth2DeviceFlowStrategy.Authenticate spawns background goroutine
// ---------------------------------------------------------------------------

func TestOAuth2DeviceFlowStrategy_Authenticate_ReturnsDeviceCodeError(t *testing.T) {
	// Start a test server that responds to the device code request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Respond to device code request with a very short expiry so the
		// background goroutine exits quickly.
		resp := DeviceCodeResponse{
			DeviceCode:      "test-device-code-1234567890",
			UserCode:        "ABCD-1234",
			VerificationURI: "https://example.com/activate",
			ExpiresIn:       1, // 1 second so background goroutine exits fast
			Interval:        1,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	strategy := NewOAuth2DeviceFlowStrategy(server.Client(), nil)

	config := map[string]interface{}{
		"clientId":         "test-client-id",
		"authorizationURL": server.URL + "/device",
		"tokenURL":         server.URL + "/token",
		"scope":            "openid",
	}

	baselineGoroutines := runtime.NumGoroutine()

	ctx := context.Background()
	_, err := strategy.Authenticate(ctx, config)

	// Must return a non-nil error
	if err == nil {
		t.Fatal("Authenticate() expected a DeviceCodeError, got nil error")
	}

	// Must be a *DeviceCodeError
	dcErr, ok := AsDeviceCodeError(err)
	if !ok {
		t.Fatalf("Authenticate() error type = %T, want *DeviceCodeError", err)
	}

	// UserCode and VerificationURL must be populated
	if dcErr.UserCode == "" {
		t.Error("DeviceCodeError.UserCode is empty, want populated")
	}
	if dcErr.VerificationURL == "" {
		t.Error("DeviceCodeError.VerificationURL is empty, want populated")
	}
	if dcErr.UserCode != "ABCD-1234" {
		t.Errorf("UserCode = %q, want %q", dcErr.UserCode, "ABCD-1234")
	}
	if dcErr.VerificationURL != "https://example.com/activate" {
		t.Errorf("VerificationURL = %q, want %q", dcErr.VerificationURL, "https://example.com/activate")
	}

	// A background goroutine should have been spawned
	afterGoroutines := runtime.NumGoroutine()
	if afterGoroutines <= baselineGoroutines {
		t.Logf("Warning: goroutine count did not increase (before=%d, after=%d); "+
			"goroutine may have already exited", baselineGoroutines, afterGoroutines)
	}

	// The background goroutine uses ExpiresIn=1s timeout derived from context.Background().
	// Wait up to 5 seconds for it to exit naturally.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= baselineGoroutines+1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	// We don't assert the final goroutine count strictly because other test
	// goroutines may be running; just ensure the test completes without hanging.
}

func TestOAuth2DeviceFlowStrategy_Authenticate_MissingClientID(t *testing.T) {
	strategy := NewOAuth2DeviceFlowStrategy(http.DefaultClient, nil)
	config := map[string]interface{}{
		"authorizationURL": "http://example.com/device",
		"tokenURL":         "http://example.com/token",
	}

	_, err := strategy.Authenticate(context.Background(), config)
	if err == nil {
		t.Fatal("Authenticate() expected error for missing clientId, got nil")
	}
	if !strings.Contains(err.Error(), "clientId") {
		t.Errorf("error = %q, want to contain 'clientId'", err.Error())
	}
}

func TestOAuth2DeviceFlowStrategy_Authenticate_MissingAuthorizationURL(t *testing.T) {
	strategy := NewOAuth2DeviceFlowStrategy(http.DefaultClient, nil)
	config := map[string]interface{}{
		"clientId": "test-client",
		"tokenURL": "http://example.com/token",
	}

	_, err := strategy.Authenticate(context.Background(), config)
	if err == nil {
		t.Fatal("Authenticate() expected error for missing authorizationURL, got nil")
	}
}

func TestOAuth2DeviceFlowStrategy_GetAuthType(t *testing.T) {
	strategy := NewOAuth2DeviceFlowStrategy(http.DefaultClient, nil)
	if strategy.GetAuthType() != AuthTypeOAuth2Device {
		t.Errorf("GetAuthType() = %v, want %v", strategy.GetAuthType(), AuthTypeOAuth2Device)
	}
}

func TestOAuth2DeviceFlowStrategy_SupportsRefresh(t *testing.T) {
	strategy := NewOAuth2DeviceFlowStrategy(http.DefaultClient, nil)
	if !strategy.SupportsRefresh() {
		t.Error("SupportsRefresh() should return true for OAuth2DeviceFlowStrategy")
	}
}
