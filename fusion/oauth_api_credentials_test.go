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
	"strings"
	"testing"
	"time"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tenebris-tech/mlogger/testlogger"
)

// newOAuthAPITestFusion builds a Fusion with a UnifyEM-style session_jwt
// service in the requested store mode, a user_credentials service and a
// bbolt-backed auth manager, plus the OAuth API handler in front of it.
func newOAuthAPITestFusion(t *testing.T, uemURL, store string) (*Fusion, *oauthAPIHandler) {
	t.Helper()
	logger := testlogger.New(t)
	mtam := newAuthManager(t, newBoltTokenStore(t, logger), logger)
	f := &Fusion{
		config: &Config{Services: map[string]*ServiceConfig{
			"uem": sessionService(uemURL, store),
			"trello": {Name: "Trello", ServiceKey: "trello", Auth: AuthConfig{
				Type: AuthTypeUserCredentials,
				Config: map[string]any{
					"instructions": "Get a key.",
					"fields": []any{
						map[string]any{"name": "key", "location": "query"},
					},
				},
			}},
		}},
		multiTenantAuth: mtam,
		externalURL:     "http://localhost:8888",
		logger:          logger,
	}
	return f, &oauthAPIHandler{engine: f, logger: logger}
}

func postTokens(t *testing.T, h *oauthAPIHandler, service string, body map[string]any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/oauth/tokens", strings.NewReader(string(payload)))
	req = req.WithContext(context.WithValue(req.Context(), global.TenantContextKey,
		&TenantContext{TenantHash: testTenantHash, ServiceName: service}))
	rec := httptest.NewRecorder()
	h.handleOAuthTokens(rec, req)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &decoded), "response must be JSON: %s", rec.Body.String())
	return rec, decoded
}

func credentialPayload(username, password string) map[string]any {
	return map[string]any{
		"service":      "uem",
		"access_token": "user_credentials:uem",
		"metadata":     map[string]string{"username": username, "password": password},
	}
}

func TestHandleOAuthTokens_CredentialsMode_Stores(t *testing.T) {
	uem := newFakeUEM(t)
	f, h := newOAuthAPITestFusion(t, uem.URL, CredentialStoreCredentials)

	// A token from an earlier set of credentials must not survive the update.
	require.NoError(t, f.multiTenantAuth.StoreOAuthToken(testTenantHash, "uem", &db.OAuthTokenData{AccessToken: "old"}))

	rec, body := postTokens(t, h, "uem", credentialPayload(testUser, testPassword))
	assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, true, body["success"])
	assert.Equal(t, "Credentials stored successfully", body["message"])

	creds, err := f.multiTenantAuth.LoadUserCredentials(testTenantHash, "uem")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"username": testUser, "password": testPassword}, creds)
	_, err = f.multiTenantAuth.db.LoadOAuthToken(testTenantHash, "uem")
	assert.Error(t, err, "stale token must be removed")
	assert.Equal(t, 0, uem.logins(), "credentials mode does not log in at storage time")
}

func TestHandleOAuthTokens_CredentialsMode_IgnoresUndeclaredFields(t *testing.T) {
	uem := newFakeUEM(t)
	f, h := newOAuthAPITestFusion(t, uem.URL, CredentialStoreCredentials)
	payload := credentialPayload(testUser, testPassword)
	payload["metadata"] = map[string]string{"username": testUser, "password": testPassword, "extra": "x"}
	rec, _ := postTokens(t, h, "uem", payload)
	require.Equal(t, http.StatusCreated, rec.Code)
	creds, err := f.multiTenantAuth.LoadUserCredentials(testTenantHash, "uem")
	require.NoError(t, err)
	_, hasExtra := creds["extra"]
	assert.False(t, hasExtra)
}

func TestHandleOAuthTokens_MissingField(t *testing.T) {
	uem := newFakeUEM(t)
	f, h := newOAuthAPITestFusion(t, uem.URL, CredentialStoreCredentials)
	payload := credentialPayload(testUser, "   ")
	rec, body := postTokens(t, h, "uem", payload)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, body["error"].(map[string]any)["message"], "credential field 'password'")
	_, err := f.multiTenantAuth.LoadUserCredentials(testTenantHash, "uem")
	assert.Error(t, err)
}

func TestHandleOAuthTokens_TokenMode_ExchangesAndStoresToken(t *testing.T) {
	uem := newFakeUEM(t)
	f, h := newOAuthAPITestFusion(t, uem.URL, CredentialStoreToken)

	rec, body := postTokens(t, h, "uem", credentialPayload(testUser, testPassword))
	assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Contains(t, body["message"], "exchanged")
	assert.Equal(t, 1, uem.logins())

	tok, err := f.multiTenantAuth.db.LoadOAuthToken(testTenantHash, "uem")
	require.NoError(t, err)
	assert.Equal(t, "tok-1", tok.AccessToken)
	_, err = f.multiTenantAuth.LoadUserCredentials(testTenantHash, "uem")
	assert.Error(t, err, "token mode must not persist credentials")

	// The stored token serves tool calls without another login.
	got, err := f.multiTenantAuth.AcquireToken(context.Background(), tenant("uem"),
		f.config.Services["uem"].AuthConfigForRequest())
	require.NoError(t, err)
	assert.Equal(t, "tok-1", got.AccessToken)
	assert.Equal(t, 1, uem.logins())
}

func TestHandleOAuthTokens_TokenMode_FailedExchange(t *testing.T) {
	uem := newFakeUEM(t)
	f, h := newOAuthAPITestFusion(t, uem.URL, CredentialStoreToken)

	rec, body := postTokens(t, h, "uem", credentialPayload(testUser, "wrong"))
	assert.Equal(t, http.StatusBadGateway, rec.Code)
	msg := body["error"].(map[string]any)["message"].(string)
	assert.Contains(t, msg, "Login to UnifyEM failed")
	assert.NotContains(t, msg, "wrong", "error must not echo the password")
	_, err := f.multiTenantAuth.db.LoadOAuthToken(testTenantHash, "uem")
	assert.Error(t, err)
}

func TestHandleOAuthTokens_UserCredentialsServiceUnchanged(t *testing.T) {
	uem := newFakeUEM(t)
	f, h := newOAuthAPITestFusion(t, uem.URL, CredentialStoreCredentials)
	rec, body := postTokens(t, h, "trello", map[string]any{
		"service":      "trello",
		"access_token": "user_credentials:trello",
		"metadata":     map[string]string{"key": "k123"},
	})
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "Tokens stored successfully", body["message"])
	tok, err := f.multiTenantAuth.db.LoadOAuthToken(testTenantHash, "trello")
	require.NoError(t, err)
	assert.Equal(t, "k123", tok.Metadata["key"])
}

func TestHandleOAuthTokens_ExpiresInStillHonouredForPlainTokens(t *testing.T) {
	uem := newFakeUEM(t)
	f, h := newOAuthAPITestFusion(t, uem.URL, CredentialStoreCredentials)
	rec, _ := postTokens(t, h, "trello", map[string]any{
		"service": "trello", "access_token": "abc", "expires_in": 60,
	})
	require.Equal(t, http.StatusCreated, rec.Code)
	tok, err := f.multiTenantAuth.db.LoadOAuthToken(testTenantHash, "trello")
	require.NoError(t, err)
	require.NotNil(t, tok.ExpiresAt)
	assert.WithinDuration(t, time.Now().Add(time.Minute), *tok.ExpiresAt, 5*time.Second)
}

func getServiceConfig(t *testing.T, h *oauthAPIHandler, service string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/services/"+service+"/config", nil)
	rec := httptest.NewRecorder()
	h.handleServiceConfig(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var decoded struct {
		Config map[string]any `json:"config"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &decoded))
	return decoded.Config
}

func TestHandleServiceConfig_LiftsSessionCredentials(t *testing.T) {
	uem := newFakeUEM(t)
	_, h := newOAuthAPITestFusion(t, uem.URL, CredentialStoreCredentials)

	cfg := getServiceConfig(t, h, "uem")
	assert.Equal(t, "session_jwt", cfg["auth_type"])
	assert.Equal(t, "Enter your UnifyEM administrator credentials.", cfg["instructions"])
	fields, ok := cfg["fields"].([]any)
	require.True(t, ok, "fields should be an array: %v", cfg["fields"])
	require.Len(t, fields, 2)
	assert.Equal(t, map[string]any{"name": "username", "label": "Username"}, fields[0])
	assert.Equal(t, map[string]any{"name": "password", "label": "Password", "secret": true}, fields[1])
	_, leaksBody := cfg["loginBody"]
	assert.False(t, leaksBody, "login template is not part of the client-facing config")

	trello := getServiceConfig(t, h, "trello")
	assert.Equal(t, "user_credentials", trello["auth_type"])
	assert.Equal(t, "Get a key.", trello["instructions"])
	tf := trello["fields"].([]any)
	assert.Equal(t, "query", tf[0].(map[string]any)["location"])
}

func TestHandleOAuthTokens_RequestValidation(t *testing.T) {
	uem := newFakeUEM(t)
	_, h := newOAuthAPITestFusion(t, uem.URL, CredentialStoreCredentials)
	withTenant := func(r *http.Request) *http.Request {
		return r.WithContext(context.WithValue(r.Context(), global.TenantContextKey,
			&TenantContext{TenantHash: testTenantHash, ServiceName: "uem"}))
	}

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.handleOAuthTokens(rec, withTenant(httptest.NewRequest(http.MethodGet, "/api/v1/oauth/tokens", nil)))
		assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})
	t.Run("missing tenant context", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.handleOAuthTokens(rec, httptest.NewRequest(http.MethodPost, "/api/v1/oauth/tokens", strings.NewReader(`{}`)))
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
	t.Run("invalid body", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.handleOAuthTokens(rec, withTenant(httptest.NewRequest(http.MethodPost, "/api/v1/oauth/tokens", strings.NewReader(`{`))))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
	t.Run("missing service", func(t *testing.T) {
		rec, _ := postTokens(t, h, "uem", map[string]any{"access_token": "x"})
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
	t.Run("missing token and metadata", func(t *testing.T) {
		rec, _ := postTokens(t, h, "uem", map[string]any{"service": "uem"})
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
	t.Run("unknown service", func(t *testing.T) {
		rec, body := postTokens(t, h, "nope", map[string]any{"service": "nope", "access_token": "x"})
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, body["error"].(map[string]any)["message"], "Unknown service")
	})
}

func TestHandleServiceConfig_Errors(t *testing.T) {
	uem := newFakeUEM(t)
	_, h := newOAuthAPITestFusion(t, uem.URL, CredentialStoreCredentials)
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodPost, "/api/v1/services/uem/config", http.StatusMethodNotAllowed},
		{http.MethodGet, "/api/v1/services/uem", http.StatusNotFound},
		{http.MethodGet, "/api/v1/services//config", http.StatusBadRequest},
		{http.MethodGet, "/api/v1/services/nope/config", http.StatusNotFound},
	} {
		rec := httptest.NewRecorder()
		h.handleServiceConfig(rec, httptest.NewRequest(tc.method, tc.path, nil))
		assert.Equal(t, tc.want, rec.Code, "%s %s", tc.method, tc.path)
	}
}
