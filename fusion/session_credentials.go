/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	"github.com/PivotLLM/MCPFusion/db"
)

// Per-user credentials for session_jwt.
//
// A session_jwt auth config may declare a "credentials" block. Each tenant then
// supplies the declared field values through fusion-auth, and the login
// request references them with {{credentials.<name>}} placeholders in
// loginURL, loginHeaders, loginBody or loginFormBody. What is persisted per
// tenant depends on credentials.store:
//
//   - "credentials" (default): the field values are stored and a login is
//     performed whenever a token is needed.
//   - "token": the login is performed once when the values are submitted and
//     only the resulting token is stored.

const (
	// CredentialStoreCredentials keeps the prompted values and logs in whenever a token is needed.
	CredentialStoreCredentials = "credentials"
	// CredentialStoreToken exchanges the prompted values once and keeps only the resulting token.
	CredentialStoreToken = "token"

	sessionCredentialsConfigKey  = "credentials"
	sessionCredentialsRuntimeKey = "__credentials"
	sessionCredentialType        = db.CredentialTypeCustom
)

var credentialPlaceholderRegex = regexp.MustCompile(`\{\{\s*credentials\.([A-Za-z0-9_.-]+)\s*\}\}`)

// loginTemplateKeys are the session_jwt config keys that may contain credential placeholders.
var loginTemplateKeys = []string{"loginURL", "loginHeaders", "loginBody", "loginFormBody"}

// SessionCredentialField describes one value fusion-auth prompts the user for.
type SessionCredentialField struct {
	Name        string `json:"name"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
	Secret      bool   `json:"secret,omitempty"`
}

// SessionCredentialsConfig is the parsed "credentials" block of a session_jwt auth config.
type SessionCredentialsConfig struct {
	Store        string
	Instructions string
	Fields       []SessionCredentialField
}

// SessionCredentials returns the parsed credentials block when this is a
// session_jwt auth config that declares one.
func (a *AuthConfig) SessionCredentials() (*SessionCredentialsConfig, bool) {
	if a == nil || a.Type != AuthTypeSessionJWT || a.Config == nil {
		return nil, false
	}
	raw, ok := a.Config[sessionCredentialsConfigKey].(map[string]any)
	if !ok {
		return nil, false
	}
	cfg := &SessionCredentialsConfig{Store: CredentialStoreCredentials}
	if store, ok := raw["store"].(string); ok && store != "" {
		cfg.Store = store
	}
	cfg.Instructions, _ = raw["instructions"].(string)
	if fields, ok := raw["fields"].([]any); ok {
		for _, entry := range fields {
			fm, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			field := SessionCredentialField{}
			field.Name, _ = fm["name"].(string)
			field.Label, _ = fm["label"].(string)
			field.Description, _ = fm["description"].(string)
			field.Secret, _ = fm["secret"].(bool)
			cfg.Fields = append(cfg.Fields, field)
		}
	}
	return cfg, true
}

// RequiresUserSetup reports whether tenants must go through fusion-auth before
// the service can be used, which is when an auth setup tool is registered.
func (a *AuthConfig) RequiresUserSetup() bool {
	if a == nil {
		return false
	}
	if a.Type == AuthTypeOAuth2External || a.Type == AuthTypeUserCredentials {
		return true
	}
	_, ok := a.SessionCredentials()
	return ok
}

// PromptsForCredentials reports whether fusion-auth collects field values for
// this service rather than running an OAuth flow.
func (a *AuthConfig) PromptsForCredentials() bool {
	if a == nil {
		return false
	}
	if a.Type == AuthTypeUserCredentials {
		return true
	}
	_, ok := a.SessionCredentials()
	return ok
}

// SetupInstructions returns the text shown to the user before fusion-auth prompts.
func (a *AuthConfig) SetupInstructions() string {
	if sc, ok := a.SessionCredentials(); ok {
		return sc.Instructions
	}
	if a == nil || a.Config == nil {
		return ""
	}
	instructions, _ := a.Config["instructions"].(string)
	return instructions
}

// SetupFields returns the field definitions fusion-auth prompts for, in the
// generic form served by the service config API, or nil when there are none.
func (a *AuthConfig) SetupFields() any {
	if sc, ok := a.SessionCredentials(); ok {
		out := make([]map[string]any, 0, len(sc.Fields))
		for _, f := range sc.Fields {
			m := map[string]any{"name": f.Name}
			if f.Label != "" {
				m["label"] = f.Label
			}
			if f.Description != "" {
				m["description"] = f.Description
			}
			if f.Secret {
				m["secret"] = true
			}
			out = append(out, m)
		}
		return out
	}
	if a == nil || a.Config == nil {
		return nil
	}
	if fields, ok := a.Config["fields"]; ok {
		return fields
	}
	return nil
}

// AuthConfigForRequest returns a copy of the service's auth config with the
// service base URL injected, so strategies can resolve relative login URLs.
// The copy is shallow: nested maps are shared and must not be mutated.
func (s *ServiceConfig) AuthConfigForRequest() AuthConfig {
	authConfig := s.Auth
	configCopy := make(map[string]any, len(authConfig.Config)+1)
	for k, v := range authConfig.Config {
		configCopy[k] = v
	}
	configCopy["baseURL"] = s.BaseURL
	authConfig.Config = configCopy
	return authConfig
}

// validateSessionCredentials checks the credentials block, loginHeaders and
// placeholder usage of a session_jwt auth config.
func validateSessionCredentials(config map[string]any) error {
	if headers, ok := config["loginHeaders"]; ok {
		hm, ok := headers.(map[string]any)
		if !ok {
			return fmt.Errorf("session_jwt loginHeaders must be an object")
		}
		for name, value := range hm {
			if _, ok := value.(string); !ok {
				return fmt.Errorf("session_jwt loginHeaders value for '%s' must be a string", name)
			}
		}
	}

	referenced := credentialPlaceholdersIn(config)
	raw, present := config[sessionCredentialsConfigKey]
	if !present {
		if len(referenced) > 0 {
			return fmt.Errorf("session_jwt login configuration references {{credentials.%s}} but no credentials block is defined", referenced[0])
		}
		return nil
	}

	block, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("session_jwt credentials must be an object")
	}
	if store, ok := block["store"]; ok {
		s, isString := store.(string)
		if !isString || (s != CredentialStoreCredentials && s != CredentialStoreToken) {
			return fmt.Errorf("session_jwt credentials.store must be '%s' or '%s'", CredentialStoreCredentials, CredentialStoreToken)
		}
	}
	fieldsRaw, ok := block["fields"]
	if !ok {
		return fmt.Errorf("session_jwt credentials requires 'fields'")
	}
	fields, ok := fieldsRaw.([]any)
	if !ok || len(fields) == 0 {
		return fmt.Errorf("session_jwt credentials 'fields' must be a non-empty array")
	}
	names := make(map[string]bool, len(fields))
	for i, entry := range fields {
		fm, ok := entry.(map[string]any)
		if !ok {
			return fmt.Errorf("session_jwt credentials field %d must be an object", i)
		}
		name, _ := fm["name"].(string)
		if name == "" {
			return fmt.Errorf("session_jwt credentials field %d requires 'name'", i)
		}
		if names[name] {
			return fmt.Errorf("session_jwt credentials field '%s' is defined more than once", name)
		}
		names[name] = true
		if secret, ok := fm["secret"]; ok {
			if _, isBool := secret.(bool); !isBool {
				return fmt.Errorf("session_jwt credentials field '%s': 'secret' must be a boolean", name)
			}
		}
	}
	if len(referenced) == 0 {
		return fmt.Errorf("session_jwt credentials block is defined but the login configuration uses no {{credentials.<field>}} placeholder")
	}
	for _, ref := range referenced {
		if !names[ref] {
			return fmt.Errorf("session_jwt login configuration references {{credentials.%s}} which is not a declared credentials field", ref)
		}
	}
	return nil
}

// credentialPlaceholdersIn returns the sorted, de-duplicated field names
// referenced by {{credentials.<name>}} placeholders in the login template keys.
func credentialPlaceholdersIn(config map[string]any) []string {
	seen := make(map[string]bool)
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			for _, m := range credentialPlaceholderRegex.FindAllStringSubmatch(t, -1) {
				seen[m[1]] = true
			}
		case map[string]any:
			for _, x := range t {
				walk(x)
			}
		case []any:
			for _, x := range t {
				walk(x)
			}
		}
	}
	for _, key := range loginTemplateKeys {
		if v, ok := config[key]; ok {
			walk(v)
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// substituteCredentialPlaceholders returns a copy of v with every
// {{credentials.<name>}} placeholder replaced from creds. Strings are
// replaced; maps and slices are walked and copied, never mutated in place.
// It fails on the first placeholder that has no value.
func substituteCredentialPlaceholders(v any, creds map[string]string) (any, error) {
	switch t := v.(type) {
	case string:
		var firstErr error
		out := credentialPlaceholderRegex.ReplaceAllStringFunc(t, func(match string) string {
			name := credentialPlaceholderRegex.FindStringSubmatch(match)[1]
			value, ok := creds[name]
			if !ok || value == "" {
				if firstErr == nil {
					firstErr = fmt.Errorf("login configuration references {{credentials.%s}} but no stored value was supplied", name)
				}
				return match
			}
			return value
		})
		if firstErr != nil {
			return nil, firstErr
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, x := range t {
			replaced, err := substituteCredentialPlaceholders(x, creds)
			if err != nil {
				return nil, err
			}
			out[k] = replaced
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			replaced, err := substituteCredentialPlaceholders(x, creds)
			if err != nil {
				return nil, err
			}
			out[i] = replaced
		}
		return out, nil
	default:
		return v, nil
	}
}

// withRuntimeCredentials returns a shallow copy of config carrying creds under
// the reserved runtime key that SessionJWTStrategy.Authenticate reads.
func withRuntimeCredentials(config map[string]any, creds map[string]string) map[string]any {
	out := make(map[string]any, len(config)+1)
	for k, v := range config {
		out[k] = v
	}
	out[sessionCredentialsRuntimeKey] = creds
	return out
}

// credentialsRequiredError is returned when a tenant has no usable stored
// credentials or token; the message tells the LLM which tool to call next.
func credentialsRequiredError(authType AuthType, serviceName string) *AuthenticationError {
	return NewAuthenticationError(authType, serviceName,
		fmt.Sprintf("no stored credentials for service %s: call the %s_auth_setup tool and have the user run the command it returns, then retry",
			serviceName, serviceName), nil)
}

// StoreUserCredentials persists the field values a tenant supplied for a service.
func (mtam *MultiTenantAuthManager) StoreUserCredentials(tenantHash, serviceName string, values map[string]string) error {
	if mtam.db == nil {
		return fmt.Errorf("token store not available")
	}
	data := make(map[string]any, len(values))
	for k, v := range values {
		data[k] = v
	}
	return mtam.db.StoreCredentials(tenantHash, serviceName, &db.ServiceCredentials{
		Type: sessionCredentialType,
		Data: data,
	})
}

// LoadUserCredentials returns the field values a tenant supplied for a service.
// Non-string values in the stored record are ignored.
func (mtam *MultiTenantAuthManager) LoadUserCredentials(tenantHash, serviceName string) (map[string]string, error) {
	if mtam.db == nil {
		return nil, fmt.Errorf("token store not available")
	}
	record, err := mtam.db.LoadCredentials(tenantHash, serviceName)
	if err != nil {
		return nil, err
	}
	values := make(map[string]string, len(record.Data))
	for k, v := range record.Data {
		if s, ok := v.(string); ok {
			values[k] = s
		}
	}
	return values, nil
}

// InvalidateCredentials removes a tenant's stored field values for the service
// named in tenantContext. Any cached token is left to InvalidateToken.
func (mtam *MultiTenantAuthManager) InvalidateCredentials(tenantContext *TenantContext) {
	if tenantContext == nil || mtam.db == nil {
		return
	}
	if err := mtam.db.DeleteCredentials(tenantContext.TenantHash, tenantContext.ServiceName); err != nil {
		if mtam.logger != nil && !db.IsNotFound(err) {
			mtam.logger.Warningf("Failed to delete credentials for tenant %s service %s: %v",
				tenantContext.ShortHash(), tenantContext.ServiceName, err)
		}
		return
	}
	if mtam.logger != nil {
		mtam.logger.Infof("Deleted stored credentials for tenant %s service %s",
			tenantContext.ShortHash(), tenantContext.ServiceName)
	}
}

// AuthenticateWithCredentials performs the service login with the supplied
// field values and caches the resulting token for the tenant. It is used by
// the token storage API when credentials.store is "token", so the values are
// never persisted.
func (mtam *MultiTenantAuthManager) AuthenticateWithCredentials(ctx context.Context, tenantContext *TenantContext,
	authConfig AuthConfig, creds map[string]string) (*TokenInfo, error) {
	if tenantContext == nil {
		return nil, NewAuthenticationError("", "", "tenant context is required", nil)
	}
	mtam.mu.RLock()
	strategy, exists := mtam.strategies[authConfig.Type]
	mtam.mu.RUnlock()
	if !exists {
		return nil, NewAuthenticationError(authConfig.Type, tenantContext.ServiceName,
			"unsupported authentication type", nil)
	}
	tokenInfo, err := strategy.Authenticate(ctx, withRuntimeCredentials(authConfig.Config, creds))
	if err != nil {
		if mtam.logger != nil {
			mtam.logger.Warningf("Credential exchange failed for tenant %s service %s: %v",
				tenantContext.ShortHash(), tenantContext.ServiceName, err)
		}
		return nil, NewAuthenticationError(authConfig.Type, tenantContext.ServiceName, "login failed", err)
	}
	mtam.CacheToken(tenantContext, tokenInfo)
	if mtam.logger != nil {
		mtam.logger.Infof("Exchanged credentials for a token for tenant %s service %s",
			tenantContext.ShortHash(), tenantContext.ServiceName)
	}
	return tokenInfo, nil
}

// loginConfigForTenant returns the auth config to pass to Authenticate for a
// tenant. For session_jwt with a credentials block it loads the tenant's
// stored values (credentials mode) or reports that the user must re-run
// fusion-auth (token mode, or nothing stored).
func (mtam *MultiTenantAuthManager) loginConfigForTenant(tenantContext *TenantContext,
	authConfig AuthConfig) (map[string]any, error) {
	sc, ok := authConfig.SessionCredentials()
	if !ok {
		return authConfig.Config, nil
	}
	if sc.Store == CredentialStoreToken {
		return nil, credentialsRequiredError(authConfig.Type, tenantContext.ServiceName)
	}
	creds, err := mtam.LoadUserCredentials(tenantContext.TenantHash, tenantContext.ServiceName)
	if err != nil || len(creds) == 0 {
		if mtam.logger != nil {
			mtam.logger.Debugf("No stored credentials for tenant %s service %s: %v",
				tenantContext.ShortHash(), tenantContext.ServiceName, err)
		}
		return nil, credentialsRequiredError(authConfig.Type, tenantContext.ServiceName)
	}
	return withRuntimeCredentials(authConfig.Config, creds), nil
}
