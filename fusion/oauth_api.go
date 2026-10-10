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
	"strings"
	"time"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/global"
)

// oauthAPIHandler serves the OAuth token-management HTTP endpoints that the
// fusion-auth / claw-auth utilities call. It is constructed from the engine so
// an embedding host (e.g. ClawEh) can mount the exact same endpoints without
// importing the standalone mcpserver package. All state (token store, service
// config, logger) is read from the engine, keeping this handler dependency-free
// beyond the engine itself.
type oauthAPIHandler struct {
	engine *Fusion
	logger global.Logger
}

// tokenRequest represents a request to store OAuth tokens.
type tokenRequest struct {
	Service      string            `json:"service"`
	AccessToken  string            `json:"access_token"`
	RefreshToken string            `json:"refresh_token"`
	ExpiresIn    int               `json:"expires_in,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// tokenResponse represents the response from storing OAuth tokens.
type tokenResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	TokenID string `json:"token_id,omitempty"`
}

// serviceConfigResponse represents the response from getting service config.
type serviceConfigResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	ServiceName string `json:"service_name,omitempty"`
	Config      any    `json:"config,omitempty"`
}

// authVerifyResponse represents the response from auth verification.
type authVerifyResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	TenantID  string `json:"tenant_id,omitempty"`
	ValidTill string `json:"valid_till,omitempty"`
}

// RegisterOAuthRoutes registers the OAuth token-management API routes on the given
// mux. The routes read the authenticated tenant from global.TenantContextKey, so
// the caller must wrap the mux with an authenticating middleware. Standalone
// MCPFusion supplies its own middleware; embedders can use OAuthHandler to get the
// routes plus the built-in auth-code middleware wired together.
func (f *Fusion) RegisterOAuthRoutes(mux *http.ServeMux) {
	f.newOAuthAPIHandler().registerRoutes(mux)
}

// OAuthHandler returns an http.Handler that serves the OAuth API routes wrapped
// with the auth-code -> tenant middleware. This is the one-call entry point for
// embedding hosts that do not have their own bearer/tenant middleware.
func (f *Fusion) OAuthHandler() http.Handler {
	h := f.newOAuthAPIHandler()
	mux := http.NewServeMux()
	h.registerRoutes(mux)
	return h.authMiddleware(mux)
}

func (f *Fusion) newOAuthAPIHandler() *oauthAPIHandler {
	return &oauthAPIHandler{engine: f, logger: f.logger}
}

// registerRoutes wires the OAuth API endpoints onto the mux.
func (h *oauthAPIHandler) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/ping", h.handlePing)
	mux.HandleFunc("/api/v1/oauth/tokens", h.handleOAuthTokens)
	mux.HandleFunc("/api/v1/auth/verify", h.handleAuthVerify)
	mux.HandleFunc("/api/v1/services/", h.handleServiceConfig)
	mux.HandleFunc("/api/v1/oauth/success", h.handleOAuthSuccess)
	mux.HandleFunc("/api/v1/oauth/error", h.handleOAuthError)
}

// authMiddleware validates the auth code presented as a bearer token (the "c"
// field of the fusion-auth blob), builds a TenantContext from it, and injects it
// under global.TenantContextKey so the handlers see the authenticated tenant.
// This mirrors the auth-code path the standalone mcpserver middleware provides.
func (h *oauthAPIHandler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			h.writeErrorResponse(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		tenantContext, err := h.engine.multiTenantAuth.ExtractTenantFromAuthCode(token)
		if err != nil {
			if h.logger != nil {
				h.logger.Errorf("OAuth API auth code validation failed: %v", err)
			}
			h.writeErrorResponse(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		if h.logger != nil {
			h.logger.Infof("OAuth API authenticated via auth code for tenant %s service %s",
				tenantContext.ShortHash(), tenantContext.ServiceName)
		}

		ctx := context.WithValue(r.Context(), global.TenantContextKey, tenantContext)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractBearerToken returns the bearer token from the Authorization header, or
// an empty string when absent or malformed.
func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(authHeader, "Bearer ")
}

// handlePing handles GET /ping - simple authenticated endpoint for connectivity testing
func (h *oauthAPIHandler) handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract tenant context from middleware - if it's present, the request is authenticated
	tenantContext, ok := r.Context().Value(global.TenantContextKey).(*TenantContext)
	if !ok {
		if h.logger != nil {
			h.logger.Error("Missing tenant context in ping request")
		}
		h.writeErrorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	if h.logger != nil {
		h.logger.Infof("Ping request from tenant %s", tenantContext.TenantHash)
	}

	// Return simple success response
	response := map[string]any{
		"success":   true,
		"message":   "pong",
		"tenant_id": tenantContext.TenantHash,
		"timestamp": time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		if h.logger != nil {
			h.logger.Errorf("Failed to encode ping response: %v", err)
		}
	}
}

// handleOAuthTokens handles POST /api/v1/oauth/tokens
func (h *oauthAPIHandler) handleOAuthTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract tenant context from middleware
	tenantContext, ok := r.Context().Value(global.TenantContextKey).(*TenantContext)
	if !ok {
		if h.logger != nil {
			h.logger.Error("Missing tenant context in OAuth token request")
		}
		h.writeErrorResponse(w, http.StatusUnauthorized, "Invalid authentication")
		return
	}

	// Parse request body
	var req tokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if h.logger != nil {
			h.logger.Errorf("Failed to decode OAuth token request: %v", err)
		}
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Service == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Service name is required")
		return
	}
	if req.AccessToken == "" && len(req.Metadata) == 0 {
		h.writeErrorResponse(w, http.StatusBadRequest, "Access token or metadata is required")
		return
	}

	// Validate service name against configured services
	if !h.engine.HasService(req.Service) {
		if h.logger != nil {
			h.logger.Errorf("Unknown service '%s' from tenant %s", req.Service, tenantContext.ShortHash())
		}
		h.writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Unknown service: %s", req.Service))
		return
	}

	// session_jwt services with a credentials block receive field values, not
	// a token: either persist them, or exchange them for a token right now.
	if service := h.engine.Service(req.Service); service != nil {
		if sc, ok := service.Auth.SessionCredentials(); ok {
			h.storeSessionCredentials(w, r, tenantContext, req.Service, service, sc, req.Metadata)
			return
		}
	}

	// Create OAuth token data
	tokenData := &db.OAuthTokenData{
		AccessToken:  req.AccessToken,
		RefreshToken: req.RefreshToken,
		TokenType:    "Bearer",
		Metadata:     req.Metadata,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Compute ExpiresAt from ExpiresIn if provided
	if req.ExpiresIn > 0 {
		expiresAt := time.Now().Add(time.Duration(req.ExpiresIn) * time.Second)
		tokenData.ExpiresAt = &expiresAt
	}

	// Store tokens via the multi-tenant auth token store
	if err := h.engine.multiTenantAuth.StoreOAuthToken(tenantContext.TenantHash, req.Service, tokenData); err != nil {
		if h.logger != nil {
			h.logger.Errorf("Failed to store OAuth token for tenant %s service %s: %v",
				tenantContext.ShortHash(), req.Service, err)
		}
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to store tokens")
		return
	}

	if h.logger != nil {
		h.logger.Infof("Successfully stored OAuth tokens for tenant %s service %s",
			tenantContext.ShortHash(), req.Service)
	}

	// Return success response
	response := tokenResponse{
		Success: true,
		Message: "Tokens stored successfully",
		TokenID: fmt.Sprintf("%s_%s", tenantContext.ShortHash(), req.Service),
	}

	h.writeJSONResponse(w, http.StatusCreated, response)
}

// storeSessionCredentials handles the token storage request for a session_jwt
// service that declares a credentials block. In credentials mode the field
// values are persisted for later logins; in token mode they are exchanged for
// a token immediately and discarded.
func (h *oauthAPIHandler) storeSessionCredentials(w http.ResponseWriter, r *http.Request,
	tenantContext *TenantContext, serviceName string, service *ServiceConfig,
	sc *SessionCredentialsConfig, values map[string]string) {

	filtered := make(map[string]string, len(sc.Fields))
	for _, field := range sc.Fields {
		value := strings.TrimSpace(values[field.Name])
		if value == "" {
			h.writeErrorResponse(w, http.StatusBadRequest,
				fmt.Sprintf("Missing value for credential field '%s'", field.Name))
			return
		}
		filtered[field.Name] = value
	}

	serviceContext := &TenantContext{
		TenantHash:  tenantContext.TenantHash,
		ServiceName: serviceName,
	}

	if sc.Store == CredentialStoreToken {
		if _, err := h.engine.multiTenantAuth.AuthenticateWithCredentials(r.Context(), serviceContext,
			service.AuthConfigForRequest(), filtered); err != nil {
			h.writeErrorResponse(w, http.StatusBadGateway,
				fmt.Sprintf("Login to %s failed: %v", service.Name, err))
			return
		}
		if h.logger != nil {
			h.logger.Infof("Exchanged credentials for a token for tenant %s service %s",
				tenantContext.ShortHash(), serviceName)
		}
		h.writeJSONResponse(w, http.StatusCreated, tokenResponse{
			Success: true,
			Message: "Credentials exchanged for a token and stored successfully",
			TokenID: fmt.Sprintf("%s_%s", tenantContext.ShortHash(), serviceName),
		})
		return
	}

	if err := h.engine.multiTenantAuth.StoreUserCredentials(tenantContext.TenantHash, serviceName, filtered); err != nil {
		if h.logger != nil {
			h.logger.Errorf("Failed to store credentials for tenant %s service %s: %v",
				tenantContext.ShortHash(), serviceName, err)
		}
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to store credentials")
		return
	}
	// Any token obtained with previous credentials must not outlive them.
	h.engine.multiTenantAuth.InvalidateToken(serviceContext)
	if h.logger != nil {
		h.logger.Infof("Stored credentials for tenant %s service %s", tenantContext.ShortHash(), serviceName)
	}
	h.writeJSONResponse(w, http.StatusCreated, tokenResponse{
		Success: true,
		Message: "Credentials stored successfully",
		TokenID: fmt.Sprintf("%s_%s", tenantContext.ShortHash(), serviceName),
	})
}

// handleAuthVerify handles GET /api/v1/auth/verify
func (h *oauthAPIHandler) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract tenant context from middleware - if we got here, auth was successful
	tenantContext, ok := r.Context().Value(global.TenantContextKey).(*TenantContext)
	if !ok {
		h.writeErrorResponse(w, http.StatusUnauthorized, "Invalid authentication")
		return
	}

	response := authVerifyResponse{
		Success:   true,
		Message:   "Authentication valid",
		TenantID:  tenantContext.ShortHash(),
		ValidTill: "Token-based authentication (no expiration)",
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// handleServiceConfig handles GET /api/v1/services/{service}/config
func (h *oauthAPIHandler) handleServiceConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract service name from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/services/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "config" {
		h.writeErrorResponse(w, http.StatusNotFound, "Invalid endpoint")
		return
	}

	serviceName := parts[0]
	if serviceName == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Service name is required")
		return
	}

	// Retrieve the service configuration from the engine
	service := h.engine.Service(serviceName)
	if service == nil {
		h.writeErrorResponse(w, http.StatusNotFound, fmt.Sprintf("Service '%s' not found", serviceName))
		return
	}

	// Build the OAuth config response from the service's auth configuration.
	// The JSON keys match the providers.ServiceConfig struct tags in cmd/auth
	// so fusion-auth can unmarshal this directly.
	oauthConfig := map[string]any{
		"service_name": serviceName,
	}

	// Include OAuth-specific config fields that fusion-auth needs
	if service.Auth.Config != nil {
		if clientID, ok := service.Auth.Config["clientId"].(string); ok && clientID != "" {
			oauthConfig["client_id"] = clientID
		}
		if clientSecret, ok := service.Auth.Config["clientSecret"].(string); ok && clientSecret != "" {
			oauthConfig["client_secret"] = clientSecret
		}
		if scope, ok := service.Auth.Config["scope"].(string); ok && scope != "" {
			oauthConfig["scopes"] = scope
		}
		if tokenURL, ok := service.Auth.Config["tokenURL"].(string); ok && tokenURL != "" {
			oauthConfig["token_url"] = tokenURL
		}
		if authURL, ok := service.Auth.Config["authorizationURL"].(string); ok && authURL != "" {
			oauthConfig["authorization_url"] = authURL
		}
	}

	// Add auth type and user_credentials config details from the service auth config
	oauthConfig["auth_type"] = string(service.Auth.Type)
	if instructions := service.Auth.SetupInstructions(); instructions != "" {
		oauthConfig["instructions"] = instructions
	}
	if fields := service.Auth.SetupFields(); fields != nil {
		oauthConfig["fields"] = fields
	}

	// Add standard endpoint info
	oauthConfig["oauth_available"] = true
	oauthConfig["endpoints"] = map[string]string{
		"token_storage": "/api/v1/oauth/tokens",
		"auth_verify":   "/api/v1/auth/verify",
	}

	response := serviceConfigResponse{
		Success:     true,
		Message:     "Service configuration retrieved",
		ServiceName: serviceName,
		Config:      oauthConfig,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// handleOAuthSuccess handles POST /api/v1/oauth/success
func (h *oauthAPIHandler) handleOAuthSuccess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract tenant context from middleware
	tenantContext, ok := r.Context().Value(global.TenantContextKey).(*TenantContext)
	if !ok {
		h.writeErrorResponse(w, http.StatusUnauthorized, "Invalid authentication")
		return
	}

	// Parse notification (we don't need to store it, just log it)
	var notification map[string]any
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		if h.logger != nil {
			h.logger.Errorf("Failed to decode success notification: %v", err)
		}
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	serviceName, _ := notification["service"].(string)
	if h.logger != nil {
		h.logger.Infof("OAuth success notification for tenant %s service %s",
			tenantContext.ShortHash(), serviceName)
	}

	h.writeJSONResponse(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Success notification received",
	})
}

// handleOAuthError handles POST /api/v1/oauth/error
func (h *oauthAPIHandler) handleOAuthError(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract tenant context from middleware
	tenantContext, ok := r.Context().Value(global.TenantContextKey).(*TenantContext)
	if !ok {
		h.writeErrorResponse(w, http.StatusUnauthorized, "Invalid authentication")
		return
	}

	// Parse notification (we don't need to store it, just log it)
	var notification map[string]any
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		if h.logger != nil {
			h.logger.Errorf("Failed to decode error notification: %v", err)
		}
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	serviceName, _ := notification["service"].(string)
	errorMsg, _ := notification["error"].(string)
	if h.logger != nil {
		h.logger.Warningf("OAuth error notification for tenant %s service %s: %s",
			tenantContext.ShortHash(), serviceName, errorMsg)
	}

	h.writeJSONResponse(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Error notification received",
	})
}

// writeJSONResponse writes a JSON response
func (h *oauthAPIHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		if h.logger != nil {
			h.logger.Errorf("Failed to encode JSON response: %v", err)
		}
	}
}

// writeErrorResponse writes a JSON error response
func (h *oauthAPIHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := map[string]any{
		"success": false,
		"error": map[string]any{
			"code":    statusCode,
			"message": message,
			"type":    "api_error",
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		if h.logger != nil {
			h.logger.Errorf("Failed to encode error response: %v", err)
		}
	}
}
