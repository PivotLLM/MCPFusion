/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tenebris-tech/mlogger/testlogger"
)

const (
	testTenantHash  = "abc123def456abc123def456abc123def456abc123def456abc123def456abcd"
	otherTenantHash = "1111111111111111111111111111111111111111111111111111111111111111"
	testUser        = "alice"
	testPassword    = "s3cret-pa55"
)

// captureLogger records every formatted log line so tests can prove that
// credentials never reach the logs.
type captureLogger struct {
	*testlogger.Logger
	mu    sync.Mutex
	lines []string
}

func newCaptureLogger(t *testing.T) *captureLogger {
	return &captureLogger{Logger: testlogger.New(t)}
}

func (c *captureLogger) record(line string) {
	c.mu.Lock()
	c.lines = append(c.lines, line)
	c.mu.Unlock()
}

func (c *captureLogger) Debug(m string)   { c.record(m); c.Logger.Debug(m) }
func (c *captureLogger) Info(m string)    { c.record(m); c.Logger.Info(m) }
func (c *captureLogger) Warning(m string) { c.record(m); c.Logger.Warning(m) }
func (c *captureLogger) Error(m string)   { c.record(m); c.Logger.Error(m) }
func (c *captureLogger) Debugf(f string, v ...any) {
	c.record(fmt.Sprintf(f, v...))
	c.Logger.Debugf(f, v...)
}
func (c *captureLogger) Infof(f string, v ...any) {
	c.record(fmt.Sprintf(f, v...))
	c.Logger.Infof(f, v...)
}
func (c *captureLogger) Warningf(f string, v ...any) {
	c.record(fmt.Sprintf(f, v...))
	c.Logger.Warningf(f, v...)
}
func (c *captureLogger) Errorf(f string, v ...any) {
	c.record(fmt.Sprintf(f, v...))
	c.Logger.Errorf(f, v...)
}

func (c *captureLogger) contains(s string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, line := range c.lines {
		if strings.Contains(line, s) {
			return true
		}
	}
	return false
}

// fakeUEM imitates the parts of a UnifyEM-style API the auth flow touches:
// a login endpoint that issues bearer tokens and a ping endpoint that only
// accepts the token most recently issued (or whatever acceptedToken is set to).
type fakeUEM struct {
	*httptest.Server
	mu            sync.Mutex
	loginCount    int
	lastLogin     map[string]any
	lastLoginPath string
	lastHeaders   http.Header
	acceptedToken string
	pingCount     int
}

func newFakeUEM(t *testing.T) *fakeUEM {
	t.Helper()
	f := &fakeUEM{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/login/", f.login)
	mux.HandleFunc("/api/v1/login", f.login)
	mux.HandleFunc("/api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.pingCount++
		if r.Header.Get("Authorization") != "Bearer "+f.acceptedToken || f.acceptedToken == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"status":"error","code":401,"details":"authentication failed"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok","code":200,"details":"pong"}`))
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func (f *fakeUEM) login(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loginCount++
	f.lastLoginPath = r.URL.Path
	f.lastHeaders = r.Header.Clone()
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.lastLogin = body
	if body["username"] != testUser || body["password"] != testPassword {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status":"error","code":401,"details":"authentication failed"}`))
		return
	}
	f.acceptedToken = fmt.Sprintf("tok-%d", f.loginCount)
	_, _ = fmt.Fprintf(w, `{"status":"ok","code":200,"access_token":"%s","refresh_token":"refresh-%d"}`, f.acceptedToken, f.loginCount)
}

func (f *fakeUEM) logins() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loginCount
}

func (f *fakeUEM) revoke() {
	f.mu.Lock()
	f.acceptedToken = "revoked"
	f.mu.Unlock()
}

// sessionService builds a UnifyEM-like service config pointing at the fake server.
func sessionService(baseURL, store string) *ServiceConfig {
	cfg := sessionCredsConfig(store)
	cfg["loginHeaders"] = map[string]any{"X-Client": "{{credentials.username}}-client"}
	cfg["headerFormat"] = "Bearer {token}"
	return &ServiceConfig{
		Name:       "UnifyEM",
		ServiceKey: "uem",
		BaseURL:    baseURL,
		Auth: AuthConfig{
			Type:              AuthTypeSessionJWT,
			Config:            cfg,
			TokenInvalidation: &TokenInvalidationConfig{StatusCodes: []int{401}, RetryOnInvalidation: true},
		},
		Endpoints: []EndpointConfig{{
			ID: "ping", Name: "Ping", Description: "Ping", Method: "GET", Path: "/api/v1/ping",
			Response: ResponseConfig{Type: ResponseTypeJSON},
		}},
	}
}

func newBoltTokenStore(t *testing.T, logger global.Logger) TokenStore {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "session-creds-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })
	database, err := db.New(db.WithLogger(logger), db.WithDataDir(tempDir))
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })
	return database.(*db.DB)
}

func newAuthManager(t *testing.T, store TokenStore, logger global.Logger) *MultiTenantAuthManager {
	t.Helper()
	return NewMultiTenantAuthManager(store, NewDatabaseCache(store, WithLogger(logger)), WithLogger(logger))
}

func tenant(service string) *TenantContext {
	return &TenantContext{TenantHash: testTenantHash, ServiceName: service}
}

func TestAcquireToken_CredentialsMode_LoginCacheAndRelogin(t *testing.T) {
	logger := newCaptureLogger(t)
	uem := newFakeUEM(t)
	service := sessionService(uem.URL, CredentialStoreCredentials)
	mtam := newAuthManager(t, newBoltTokenStore(t, logger), logger)
	ctx := context.Background()
	tc := tenant("uem")

	require.NoError(t, mtam.StoreUserCredentials(testTenantHash, "uem", map[string]string{
		"username": testUser, "password": testPassword,
	}))

	// First call logs in with the stored values substituted into body and headers.
	tok, err := mtam.AcquireToken(ctx, tc, service.AuthConfigForRequest())
	require.NoError(t, err)
	assert.Equal(t, "tok-1", tok.AccessToken)
	assert.Equal(t, 1, uem.logins())
	assert.Equal(t, testUser, uem.lastLogin["username"])
	assert.Equal(t, testPassword, uem.lastLogin["password"])
	assert.Equal(t, "alice-client", uem.lastHeaders.Get("X-Client"))
	assert.Equal(t, "application/json", uem.lastHeaders.Get("Content-Type"))

	// Credentials never land in the token record.
	for k, v := range tok.Metadata {
		assert.NotEqual(t, testPassword, v, "metadata key %s leaks the password", k)
		assert.NotEqual(t, testUser, v, "metadata key %s leaks the username", k)
	}
	_, hasRuntime := tok.Metadata[sessionCredentialsRuntimeKey]
	assert.False(t, hasRuntime)

	// Second call is served from the cached token.
	tok2, err := mtam.AcquireToken(ctx, tc, service.AuthConfigForRequest())
	require.NoError(t, err)
	assert.Equal(t, "tok-1", tok2.AccessToken)
	assert.Equal(t, 1, uem.logins())

	// Invalidating the token (what a 401 does) keeps the credentials, so the next call logs in again.
	mtam.InvalidateToken(tc)
	tok3, err := mtam.AcquireToken(ctx, tc, service.AuthConfigForRequest())
	require.NoError(t, err)
	assert.Equal(t, "tok-2", tok3.AccessToken)
	assert.Equal(t, 2, uem.logins())
	creds, err := mtam.LoadUserCredentials(testTenantHash, "uem")
	require.NoError(t, err)
	assert.Equal(t, testUser, creds["username"])

	// Removing the credentials sends the tenant back to auth_setup.
	mtam.InvalidateCredentials(tc)
	mtam.InvalidateToken(tc)
	_, err = mtam.AcquireToken(ctx, tc, service.AuthConfigForRequest())
	require.Error(t, err)
	authErr, ok := AsAuthenticationError(err)
	require.True(t, ok, "expected AuthenticationError, got %T", err)
	assert.Contains(t, authErr.Message, "uem_auth_setup")
	assert.Equal(t, 2, uem.logins(), "no login attempted without credentials")

	// The password must never have been logged.
	assert.False(t, logger.contains(testPassword), "password appeared in logs")
}

func TestAcquireToken_CredentialsMode_OtherTenantIsolated(t *testing.T) {
	logger := testlogger.New(t)
	uem := newFakeUEM(t)
	service := sessionService(uem.URL, CredentialStoreCredentials)
	mtam := newAuthManager(t, newBoltTokenStore(t, logger), logger)

	require.NoError(t, mtam.StoreUserCredentials(testTenantHash, "uem", map[string]string{
		"username": testUser, "password": testPassword,
	}))
	_, err := mtam.AcquireToken(context.Background(), &TenantContext{TenantHash: otherTenantHash, ServiceName: "uem"},
		service.AuthConfigForRequest())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uem_auth_setup")
	assert.Equal(t, 0, uem.logins())
}

func TestAcquireToken_CredentialsMode_WrongPassword(t *testing.T) {
	logger := newCaptureLogger(t)
	uem := newFakeUEM(t)
	service := sessionService(uem.URL, CredentialStoreCredentials)
	mtam := newAuthManager(t, newBoltTokenStore(t, logger), logger)

	require.NoError(t, mtam.StoreUserCredentials(testTenantHash, "uem", map[string]string{
		"username": testUser, "password": "wrong-pass",
	}))
	_, err := mtam.AcquireToken(context.Background(), tenant("uem"), service.AuthConfigForRequest())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.NotContains(t, err.Error(), "wrong-pass")
	assert.Equal(t, 1, uem.logins())
	assert.False(t, logger.contains("wrong-pass"))

	// Credentials remain so the user can fix them via auth_setup; no token was stored.
	_, err = mtam.LoadUserCredentials(testTenantHash, "uem")
	assert.NoError(t, err)
	_, err = mtam.db.LoadOAuthToken(testTenantHash, "uem")
	assert.Error(t, err)
}

func TestAcquireToken_CredentialsMode_PlaceholderInLoginURL(t *testing.T) {
	logger := testlogger.New(t)
	uem := newFakeUEM(t)
	service := sessionService(uem.URL, CredentialStoreCredentials)
	service.Auth.Config["loginURL"] = "/api/v1/login/{{credentials.username}}"
	mtam := newAuthManager(t, newBoltTokenStore(t, logger), logger)

	require.NoError(t, mtam.StoreUserCredentials(testTenantHash, "uem", map[string]string{
		"username": testUser, "password": testPassword,
	}))
	tok, err := mtam.AcquireToken(context.Background(), tenant("uem"), service.AuthConfigForRequest())
	require.NoError(t, err)
	assert.Equal(t, "tok-1", tok.AccessToken)
	assert.Equal(t, "/api/v1/login/"+testUser, uem.lastLoginPath)
	// The template, not the resolved URL, is what the config still holds.
	assert.Equal(t, "/api/v1/login/{{credentials.username}}", service.Auth.Config["loginURL"])
}

func TestTokenMode_ExchangeStoresOnlyToken(t *testing.T) {
	logger := newCaptureLogger(t)
	uem := newFakeUEM(t)
	service := sessionService(uem.URL, CredentialStoreToken)
	mtam := newAuthManager(t, newBoltTokenStore(t, logger), logger)
	ctx := context.Background()
	tc := tenant("uem")

	// Nothing stored yet: the tenant is told to run auth_setup and no login happens.
	_, err := mtam.AcquireToken(ctx, tc, service.AuthConfigForRequest())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uem_auth_setup")
	assert.Equal(t, 0, uem.logins())

	// Exchange at submission time.
	tok, err := mtam.AuthenticateWithCredentials(ctx, tc, service.AuthConfigForRequest(), map[string]string{
		"username": testUser, "password": testPassword,
	})
	require.NoError(t, err)
	assert.Equal(t, "tok-1", tok.AccessToken)
	assert.Equal(t, 1, uem.logins())

	_, err = mtam.LoadUserCredentials(testTenantHash, "uem")
	assert.Error(t, err, "token mode must not persist credentials")

	// Subsequent calls use the stored token without logging in.
	tok2, err := mtam.AcquireToken(ctx, tc, service.AuthConfigForRequest())
	require.NoError(t, err)
	assert.Equal(t, "tok-1", tok2.AccessToken)
	assert.Equal(t, 1, uem.logins())

	// Once the token is gone (401 invalidation), there is nothing to log in with.
	mtam.InvalidateToken(tc)
	_, err = mtam.AcquireToken(ctx, tc, service.AuthConfigForRequest())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uem_auth_setup")
	assert.Equal(t, 1, uem.logins())

	assert.False(t, logger.contains(testPassword))
}

func TestTokenMode_ExpiredTokenWithoutRefreshRequiresSetup(t *testing.T) {
	logger := testlogger.New(t)
	uem := newFakeUEM(t)
	service := sessionService(uem.URL, CredentialStoreToken)
	mtam := newAuthManager(t, newBoltTokenStore(t, logger), logger)

	past := time.Now().Add(-time.Minute)
	require.NoError(t, mtam.StoreOAuthToken(testTenantHash, "uem", &db.OAuthTokenData{
		AccessToken: "stale", TokenType: "Bearer", ExpiresAt: &past,
	}))
	_, err := mtam.AcquireToken(context.Background(), tenant("uem"), service.AuthConfigForRequest())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uem_auth_setup")
	assert.Equal(t, 0, uem.logins())
}

func TestTokenMode_FailedExchangeStoresNothing(t *testing.T) {
	logger := testlogger.New(t)
	uem := newFakeUEM(t)
	service := sessionService(uem.URL, CredentialStoreToken)
	mtam := newAuthManager(t, newBoltTokenStore(t, logger), logger)

	_, err := mtam.AuthenticateWithCredentials(context.Background(), tenant("uem"), service.AuthConfigForRequest(),
		map[string]string{"username": testUser, "password": "nope"})
	require.Error(t, err)
	authErr, ok := AsAuthenticationError(err)
	require.True(t, ok)
	assert.Equal(t, "login failed", authErr.Message)
	assert.Equal(t, 1, uem.logins())
	_, err = mtam.db.LoadOAuthToken(testTenantHash, "uem")
	assert.Error(t, err, "no token must be stored after a failed exchange")
	_, err = mtam.LoadUserCredentials(testTenantHash, "uem")
	assert.Error(t, err)
}

func TestAuthenticateWithCredentials_Guards(t *testing.T) {
	logger := testlogger.New(t)
	mtam := newAuthManager(t, newBoltTokenStore(t, logger), logger)
	_, err := mtam.AuthenticateWithCredentials(context.Background(), nil, AuthConfig{Type: AuthTypeSessionJWT}, nil)
	assert.Error(t, err)
	_, err = mtam.AuthenticateWithCredentials(context.Background(), tenant("uem"), AuthConfig{Type: "bogus"}, nil)
	assert.Error(t, err)
}

func TestCredentialStore_WithoutDatabase(t *testing.T) {
	mtam := NewMultiTenantAuthManager(nil, NewDatabaseCache(nil))
	assert.Error(t, mtam.StoreUserCredentials(testTenantHash, "uem", map[string]string{"a": "b"}))
	_, err := mtam.LoadUserCredentials(testTenantHash, "uem")
	assert.Error(t, err)
	mtam.InvalidateCredentials(tenant("uem")) // must not panic
	mtam.InvalidateCredentials(nil)
}

// The handler-level flow: a 401 from the API invalidates the session token
// and the retry logs in again with the stored credentials.
func TestHTTPHandler_SessionCredentials_ReloginOn401(t *testing.T) {
	logger := newCaptureLogger(t)
	uem := newFakeUEM(t)
	service := sessionService(uem.URL, CredentialStoreCredentials)
	mtam := newAuthManager(t, newBoltTokenStore(t, logger), logger)
	f, err := New(WithLogger(logger), WithMultiTenantAuth(mtam),
		WithConfig(&Config{Services: map[string]*ServiceConfig{"uem": service}}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	handler := NewHTTPHandler(f, service, &service.Endpoints[0])

	require.NoError(t, mtam.StoreUserCredentials(testTenantHash, "uem", map[string]string{
		"username": testUser, "password": testPassword,
	}))
	ctx := context.WithValue(context.Background(), global.TenantContextKey, tenant("uem"))

	result, err := handler.Handle(ctx, map[string]any{})
	require.NoError(t, err)
	assert.Contains(t, result, "pong")
	assert.Equal(t, 1, uem.logins())

	// The upstream session dies; the next call gets a 401, re-logs-in and succeeds.
	uem.revoke()
	result, err = handler.Handle(ctx, map[string]any{})
	require.NoError(t, err)
	assert.Contains(t, result, "pong")
	assert.Equal(t, 2, uem.logins())
	assert.False(t, logger.contains(testPassword))
}

func TestHTTPHandler_SessionCredentials_NoCredentials(t *testing.T) {
	logger := testlogger.New(t)
	uem := newFakeUEM(t)
	service := sessionService(uem.URL, CredentialStoreCredentials)
	mtam := newAuthManager(t, newBoltTokenStore(t, logger), logger)
	f, err := New(WithLogger(logger), WithMultiTenantAuth(mtam),
		WithConfig(&Config{Services: map[string]*ServiceConfig{"uem": service}}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	handler := NewHTTPHandler(f, service, &service.Endpoints[0])

	ctx := context.WithValue(context.Background(), global.TenantContextKey, tenant("uem"))
	_, err = handler.Handle(ctx, map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uem_auth_setup")
	assert.Equal(t, 0, uem.logins())
	assert.Equal(t, 0, uem.pingCount)
}

// Library embedding: the same flow through a generic DataStore-backed token
// store, which is how hosts that embed fusion (rather than running the
// standalone server) persist tokens and credentials.
func TestAcquireToken_CredentialsMode_DataStoreBacked(t *testing.T) {
	logger := testlogger.New(t)
	uem := newFakeUEM(t)
	service := sessionService(uem.URL, CredentialStoreCredentials)
	store, _ := newTestTokenStore(t)
	mtam := newAuthManager(t, store, logger)
	ctx := context.Background()
	tc := tenant("uem")

	_, err := mtam.AcquireToken(ctx, tc, service.AuthConfigForRequest())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uem_auth_setup")

	require.NoError(t, mtam.StoreUserCredentials(testTenantHash, "uem", map[string]string{
		"username": testUser, "password": testPassword,
	}))
	tok, err := mtam.AcquireToken(ctx, tc, service.AuthConfigForRequest())
	require.NoError(t, err)
	assert.Equal(t, "tok-1", tok.AccessToken)

	mtam.InvalidateToken(tc)
	tok, err = mtam.AcquireToken(ctx, tc, service.AuthConfigForRequest())
	require.NoError(t, err)
	assert.Equal(t, "tok-2", tok.AccessToken)

	mtam.InvalidateCredentials(tc)
	_, err = mtam.LoadUserCredentials(testTenantHash, "uem")
	assert.Error(t, err)
}

func TestTokenMode_DataStoreBacked(t *testing.T) {
	logger := testlogger.New(t)
	uem := newFakeUEM(t)
	service := sessionService(uem.URL, CredentialStoreToken)
	store, ds := newTestTokenStore(t)
	mtam := newAuthManager(t, store, logger)
	tc := tenant("uem")

	_, err := mtam.AuthenticateWithCredentials(context.Background(), tc, service.AuthConfigForRequest(),
		map[string]string{"username": testUser, "password": testPassword})
	require.NoError(t, err)
	tok, err := mtam.AcquireToken(context.Background(), tc, service.AuthConfigForRequest())
	require.NoError(t, err)
	assert.Equal(t, "tok-1", tok.AccessToken)
	assert.Empty(t, ds.data[dsCollectionCreds], "token mode must not write credentials")
}

// A session_jwt service without a credentials block keeps its env-driven behaviour.
func TestAcquireToken_PlainSessionJWT_Unchanged(t *testing.T) {
	logger := testlogger.New(t)
	uem := newFakeUEM(t)
	service := &ServiceConfig{
		Name: "UEM", ServiceKey: "uem", BaseURL: uem.URL,
		Auth: AuthConfig{Type: AuthTypeSessionJWT, Config: map[string]any{
			"loginURL": "/api/v1/login", "tokenPath": "access_token", "tokenLocation": "header",
			"loginBody": map[string]any{"username": testUser, "password": testPassword},
		}},
	}
	mtam := newAuthManager(t, newBoltTokenStore(t, logger), logger)
	tok, err := mtam.AcquireToken(context.Background(), tenant("uem"), service.AuthConfigForRequest())
	require.NoError(t, err)
	assert.Equal(t, "tok-1", tok.AccessToken)
}
