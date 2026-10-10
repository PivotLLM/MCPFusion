/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sessionCredsConfig returns a session_jwt auth config map with a credentials
// block. Callers mutate the returned map to build variants.
func sessionCredsConfig(store string) map[string]any {
	credentials := map[string]any{
		"instructions": "Enter your UnifyEM administrator credentials.",
		"fields": []any{
			map[string]any{"name": "username", "label": "Username"},
			map[string]any{"name": "password", "label": "Password", "secret": true},
		},
	}
	if store != "" {
		credentials["store"] = store
	}
	return map[string]any{
		"loginURL":      "/api/v1/login",
		"tokenPath":     "access_token",
		"tokenLocation": "header",
		"loginBody": map[string]any{
			"username": "{{credentials.username}}",
			"password": "{{credentials.password}}",
		},
		"credentials": credentials,
	}
}

func TestSessionCredentials_Parse(t *testing.T) {
	t.Run("absent for other auth types", func(t *testing.T) {
		a := &AuthConfig{Type: AuthTypeUserCredentials, Config: map[string]any{"credentials": map[string]any{}}}
		_, ok := a.SessionCredentials()
		assert.False(t, ok)
		_, ok = (*AuthConfig)(nil).SessionCredentials()
		assert.False(t, ok)
	})

	t.Run("absent for session_jwt without block", func(t *testing.T) {
		a := &AuthConfig{Type: AuthTypeSessionJWT, Config: map[string]any{"loginURL": "/login"}}
		_, ok := a.SessionCredentials()
		assert.False(t, ok)
		assert.False(t, a.RequiresUserSetup())
		assert.False(t, a.PromptsForCredentials())
		assert.Nil(t, a.SetupFields())
	})

	t.Run("defaults to credentials store", func(t *testing.T) {
		a := &AuthConfig{Type: AuthTypeSessionJWT, Config: sessionCredsConfig("")}
		sc, ok := a.SessionCredentials()
		require.True(t, ok)
		assert.Equal(t, CredentialStoreCredentials, sc.Store)
		assert.Equal(t, "Enter your UnifyEM administrator credentials.", sc.Instructions)
		require.Len(t, sc.Fields, 2)
		assert.Equal(t, SessionCredentialField{Name: "username", Label: "Username"}, sc.Fields[0])
		assert.Equal(t, SessionCredentialField{Name: "password", Label: "Password", Secret: true}, sc.Fields[1])
		assert.True(t, a.RequiresUserSetup())
		assert.True(t, a.PromptsForCredentials())
		assert.Equal(t, sc.Instructions, a.SetupInstructions())
	})

	t.Run("token store", func(t *testing.T) {
		a := &AuthConfig{Type: AuthTypeSessionJWT, Config: sessionCredsConfig(CredentialStoreToken)}
		sc, ok := a.SessionCredentials()
		require.True(t, ok)
		assert.Equal(t, CredentialStoreToken, sc.Store)
	})

	t.Run("setup fields are generic maps with secret flag", func(t *testing.T) {
		a := &AuthConfig{Type: AuthTypeSessionJWT, Config: sessionCredsConfig("")}
		fields, ok := a.SetupFields().([]map[string]any)
		require.True(t, ok)
		require.Len(t, fields, 2)
		assert.Equal(t, map[string]any{"name": "username", "label": "Username"}, fields[0])
		assert.Equal(t, map[string]any{"name": "password", "label": "Password", "secret": true}, fields[1])
	})

	t.Run("user_credentials helpers pass raw config through", func(t *testing.T) {
		raw := []any{map[string]any{"name": "key", "location": "query"}}
		a := &AuthConfig{Type: AuthTypeUserCredentials, Config: map[string]any{
			"instructions": "Get a key.", "fields": raw,
		}}
		assert.True(t, a.RequiresUserSetup())
		assert.True(t, a.PromptsForCredentials())
		assert.Equal(t, "Get a key.", a.SetupInstructions())
		assert.Equal(t, raw, a.SetupFields())
	})

	t.Run("oauth2_external requires setup but does not prompt", func(t *testing.T) {
		a := &AuthConfig{Type: AuthTypeOAuth2External}
		assert.True(t, a.RequiresUserSetup())
		assert.False(t, a.PromptsForCredentials())
		assert.Equal(t, "", a.SetupInstructions())
	})
}

func TestValidateSessionCredentials(t *testing.T) {
	withCreds := func(mutate func(cfg map[string]any, creds map[string]any)) map[string]any {
		cfg := sessionCredsConfig("")
		mutate(cfg, cfg["credentials"].(map[string]any))
		return cfg
	}

	tests := []struct {
		name    string
		config  map[string]any
		wantErr string
	}{
		{
			name:   "plain session_jwt without block is valid",
			config: map[string]any{"loginURL": "/login", "tokenPath": "t", "tokenLocation": "header"},
		},
		{
			name:   "valid credentials mode",
			config: sessionCredsConfig(CredentialStoreCredentials),
		},
		{
			name:   "valid token mode",
			config: sessionCredsConfig(CredentialStoreToken),
		},
		{
			name: "placeholders in URL headers and form body",
			config: map[string]any{
				"loginURL":      "/login/{{credentials.user}}",
				"tokenPath":     "t",
				"tokenLocation": "header",
				"loginHeaders":  map[string]any{"X-Key": "{{credentials.key}}"},
				"loginFormBody": map[string]any{"pw": "{{ credentials.pw }}"},
				"credentials": map[string]any{"fields": []any{
					map[string]any{"name": "user"},
					map[string]any{"name": "key"},
					map[string]any{"name": "pw"},
				}},
			},
		},
		{
			name: "placeholder without block",
			config: map[string]any{
				"loginURL": "/login", "tokenPath": "t", "tokenLocation": "header",
				"loginBody": map[string]any{"u": "{{credentials.username}}"},
			},
			wantErr: "references {{credentials.username}} but no credentials block",
		},
		{
			name:    "block must be object",
			config:  withCreds(func(cfg map[string]any, _ map[string]any) { cfg["credentials"] = "yes" }),
			wantErr: "credentials must be an object",
		},
		{
			name:    "invalid store",
			config:  withCreds(func(_ map[string]any, c map[string]any) { c["store"] = "vault" }),
			wantErr: "credentials.store must be",
		},
		{
			name:    "store must be string",
			config:  withCreds(func(_ map[string]any, c map[string]any) { c["store"] = true }),
			wantErr: "credentials.store must be",
		},
		{
			name:    "fields required",
			config:  withCreds(func(_ map[string]any, c map[string]any) { delete(c, "fields") }),
			wantErr: "requires 'fields'",
		},
		{
			name:    "fields non-empty",
			config:  withCreds(func(_ map[string]any, c map[string]any) { c["fields"] = []any{} }),
			wantErr: "non-empty array",
		},
		{
			name:    "field must be object",
			config:  withCreds(func(_ map[string]any, c map[string]any) { c["fields"] = []any{"username"} }),
			wantErr: "field 0 must be an object",
		},
		{
			name: "field requires name",
			config: withCreds(func(_ map[string]any, c map[string]any) {
				c["fields"] = []any{map[string]any{"label": "x"}}
			}),
			wantErr: "field 0 requires 'name'",
		},
		{
			name: "duplicate field",
			config: withCreds(func(_ map[string]any, c map[string]any) {
				c["fields"] = []any{
					map[string]any{"name": "username"},
					map[string]any{"name": "username"},
					map[string]any{"name": "password"},
				}
			}),
			wantErr: "defined more than once",
		},
		{
			name: "secret must be boolean",
			config: withCreds(func(_ map[string]any, c map[string]any) {
				c["fields"] = []any{
					map[string]any{"name": "username"},
					map[string]any{"name": "password", "secret": "yes"},
				}
			}),
			wantErr: "'secret' must be a boolean",
		},
		{
			name: "block without any placeholder",
			config: withCreds(func(cfg map[string]any, _ map[string]any) {
				cfg["loginBody"] = map[string]any{"username": "static"}
			}),
			wantErr: "uses no {{credentials.<field>}} placeholder",
		},
		{
			name: "placeholder for undeclared field",
			config: withCreds(func(cfg map[string]any, _ map[string]any) {
				cfg["loginBody"].(map[string]any)["otp"] = "{{credentials.otp}}"
			}),
			wantErr: "{{credentials.otp}} which is not a declared",
		},
		{
			name:    "loginHeaders must be object",
			config:  withCreds(func(cfg map[string]any, _ map[string]any) { cfg["loginHeaders"] = "X: y" }),
			wantErr: "loginHeaders must be an object",
		},
		{
			name: "loginHeaders values must be strings",
			config: withCreds(func(cfg map[string]any, _ map[string]any) {
				cfg["loginHeaders"] = map[string]any{"X-Count": 1}
			}),
			wantErr: "loginHeaders value for 'X-Count' must be a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSessionCredentials(tt.config)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestAuthConfigValidate_SessionCredentials(t *testing.T) {
	// The block is validated as part of AuthConfig.Validate for session_jwt.
	good := &AuthConfig{Type: AuthTypeSessionJWT, Config: sessionCredsConfig("")}
	assert.NoError(t, good.Validate())

	bad := &AuthConfig{Type: AuthTypeSessionJWT, Config: sessionCredsConfig("vault")}
	err := bad.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credentials.store")
}

func TestLoadConfigFromJSON_SessionCredentials(t *testing.T) {
	const template = `{
	  "services": {
	    "uem": {
	      "name": "UnifyEM",
	      "baseURL": "https://uem.example.com",
	      "auth": {
	        "type": "session_jwt",
	        "config": {
	          "loginURL": "/api/v1/login",
	          "loginBody": {"username": "{{credentials.username}}", "password": "{{credentials.password}}"},
	          "tokenPath": "access_token",
	          "tokenLocation": "header",
	          "credentials": {
	            "store": "STORE",
	            "fields": [{"name": "username"}, {"name": "password", "secret": true}]
	          }
	        }
	      },
	      "endpoints": [{"id": "ping", "name": "Ping", "description": "Ping", "method": "GET", "path": "/api/v1/ping", "response": {"type": "json"}}]
	    }
	  }
	}`

	for _, store := range []string{CredentialStoreCredentials, CredentialStoreToken} {
		cfg, err := LoadConfigFromJSON([]byte(strings.ReplaceAll(template, "STORE", store)), "test.json")
		require.NoError(t, err, "store %s should load", store)
		sc, ok := cfg.Services["uem"].Auth.SessionCredentials()
		require.True(t, ok)
		assert.Equal(t, store, sc.Store)
		assert.Equal(t, "uem", cfg.Services["uem"].ServiceKey)
	}

	_, err := LoadConfigFromJSON([]byte(strings.ReplaceAll(template, "STORE", "vault")), "test.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credentials.store")
}

func TestCredentialPlaceholdersIn(t *testing.T) {
	config := map[string]any{
		"loginURL":     "/login/{{credentials.user}}",
		"loginHeaders": map[string]any{"X-Key": "{{credentials.key}}"},
		"loginBody": map[string]any{
			"nested": map[string]any{"list": []any{"{{ credentials.pw }}", "{{credentials.user}}"}},
		},
		"tokenPath": "{{credentials.ignored}}", // not a login template key
	}
	assert.Equal(t, []string{"key", "pw", "user"}, credentialPlaceholdersIn(config))
	assert.Empty(t, credentialPlaceholdersIn(map[string]any{"loginURL": "/login"}))
}

func TestSubstituteCredentialPlaceholders(t *testing.T) {
	creds := map[string]string{"user": "alice", "pw": "s3cret"}

	t.Run("string", func(t *testing.T) {
		out, err := substituteCredentialPlaceholders("/login/{{credentials.user}}?x={{ credentials.pw }}", creds)
		require.NoError(t, err)
		assert.Equal(t, "/login/alice?x=s3cret", out)
	})

	t.Run("map and slice are copied not mutated", func(t *testing.T) {
		template := map[string]any{
			"username": "{{credentials.user}}",
			"nested":   map[string]any{"list": []any{"{{credentials.pw}}", 42, true}},
			"static":   "value",
		}
		out, err := substituteCredentialPlaceholders(template, creds)
		require.NoError(t, err)
		outMap := out.(map[string]any)
		assert.Equal(t, "alice", outMap["username"])
		assert.Equal(t, "value", outMap["static"])
		assert.Equal(t, []any{"s3cret", 42, true}, outMap["nested"].(map[string]any)["list"])
		// The shared template must keep its placeholders for the next tenant.
		assert.Equal(t, "{{credentials.user}}", template["username"])
		assert.Equal(t, "{{credentials.pw}}", template["nested"].(map[string]any)["list"].([]any)[0])
	})

	t.Run("missing value fails and names the field", func(t *testing.T) {
		_, err := substituteCredentialPlaceholders(map[string]any{"u": "{{credentials.user}}", "o": "{{credentials.otp}}"}, creds)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "{{credentials.otp}}")
		assert.NotContains(t, err.Error(), "alice", "error must not echo credential values")
	})

	t.Run("nil credentials with placeholder fails", func(t *testing.T) {
		_, err := substituteCredentialPlaceholders("{{credentials.user}}", nil)
		require.Error(t, err)
	})

	t.Run("no placeholder passes through untouched", func(t *testing.T) {
		out, err := substituteCredentialPlaceholders("plain", nil)
		require.NoError(t, err)
		assert.Equal(t, "plain", out)
		num, err := substituteCredentialPlaceholders(7, nil)
		require.NoError(t, err)
		assert.Equal(t, 7, num)
	})
}

func TestWithRuntimeCredentials(t *testing.T) {
	base := map[string]any{"loginURL": "/login"}
	creds := map[string]string{"user": "alice"}
	out := withRuntimeCredentials(base, creds)
	assert.Equal(t, creds, out[sessionCredentialsRuntimeKey])
	assert.Equal(t, "/login", out["loginURL"])
	_, leaked := base[sessionCredentialsRuntimeKey]
	assert.False(t, leaked, "original config must not receive credentials")
}

func TestAuthConfigForRequest(t *testing.T) {
	service := &ServiceConfig{
		BaseURL: "https://uem.example.com",
		Auth:    AuthConfig{Type: AuthTypeSessionJWT, Config: map[string]any{"loginURL": "/login"}},
	}
	got := service.AuthConfigForRequest()
	assert.Equal(t, "https://uem.example.com", got.Config["baseURL"])
	assert.Equal(t, "/login", got.Config["loginURL"])
	_, mutated := service.Auth.Config["baseURL"]
	assert.False(t, mutated, "service config must not be mutated")

	empty := &ServiceConfig{BaseURL: "https://x", Auth: AuthConfig{Type: AuthTypeNone}}
	assert.Equal(t, "https://x", empty.AuthConfigForRequest().Config["baseURL"])
}

func TestCredentialsRequiredError(t *testing.T) {
	err := credentialsRequiredError(AuthTypeSessionJWT, "uem")
	assert.Equal(t, AuthTypeSessionJWT, err.Type)
	assert.Equal(t, "uem", err.Service)
	assert.Contains(t, err.Error(), "uem_auth_setup")
}
