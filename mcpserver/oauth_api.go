/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package mcpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/global"
)

// OAuthAPIHandler provides HTTP endpoints for OAuth token management
type OAuthAPIHandler struct {
	database      *db.DB
	authManager   *fusion.MultiTenantAuthManager
	configManager ServiceProvider
	logger        global.Logger
}

// NewOAuthAPIHandler creates a new OAuth API handler
func NewOAuthAPIHandler(database *db.DB, authManager *fusion.MultiTenantAuthManager, 
	configManager ServiceProvider, logger global.Logger) *OAuthAPIHandler {
	return &OAuthAPIHandler{
		database:      database,
		authManager:   authManager,
		configManager: configManager,
		logger:        logger,
	}
}

// TokenRequest represents a request to store OAuth tokens
type TokenRequest struct {
	Service      string `json:"service"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// TokenResponse represents the response from storing OAuth tokens
type TokenResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	TokenID string `json:"token_id,omitempty"`
}

// ServiceConfigResponse represents the response from getting service config
type ServiceConfigResponse struct {
	Success     bool        `json:"success"`
	Message     string      `json:"message"`
	ServiceName string      `json:"service_name,omitempty"`
	Config      interface{} `json:"config,omitempty"`
}

// AuthVerifyResponse represents the response from auth verification
type AuthVerifyResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	TenantID  string `json:"tenant_id,omitempty"`
	ValidTill string `json:"valid_till,omitempty"`
}

// RegisterRoutes registers the OAuth API routes with the given mux
func (h *OAuthAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/ping", h.handlePing)
	mux.HandleFunc("/api/v1/oauth/tokens", h.handleOAuthTokens)
	mux.HandleFunc("/api/v1/auth/verify", h.handleAuthVerify)
	mux.HandleFunc("/api/v1/services/", h.handleServiceConfig)
	mux.HandleFunc("/api/v1/oauth/success", h.handleOAuthSuccess)
	mux.HandleFunc("/api/v1/oauth/error", h.handleOAuthError)
}

// handlePing handles GET /ping - simple authenticated endpoint for connectivity testing
func (h *OAuthAPIHandler) handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract tenant context from middleware - if it's present, the request is authenticated
	tenantContext, ok := r.Context().Value(global.TenantContextKey).(*fusion.TenantContext)
	if !ok {
		h.logger.Error("Missing tenant context in ping request")
		h.writeErrorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Log the ping request if needed
	h.logger.Infof("Ping request from tenant %s", tenantContext.TenantHash)

	// Return simple success response
	response := map[string]interface{}{
		"success": true,
		"message": "pong",
		"tenant_id": tenantContext.TenantHash,
		"timestamp": time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Errorf("Failed to encode ping response: %v", err)
	}
}

// handleOAuthTokens handles POST /api/v1/oauth/tokens
func (h *OAuthAPIHandler) handleOAuthTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract tenant context from middleware
	tenantContext, ok := r.Context().Value(global.TenantContextKey).(*fusion.TenantContext)
	if !ok {
		h.logger.Error("Missing tenant context in OAuth token request")
		h.writeErrorResponse(w, http.StatusUnauthorized, "Invalid authentication")
		return
	}

	// Parse request body
	var req TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Errorf("Failed to decode OAuth token request: %v", err)
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Service == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Service name is required")
		return
	}
	if req.AccessToken == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Access token is required")
		return
	}

	// Validate service name against available services
	if h.configManager != nil {
		availableServices := h.configManager.GetAvailableServices()
		serviceFound := false
		for _, service := range availableServices {
			if service == req.Service {
				serviceFound = true
				break
			}
		}
		if !serviceFound {
			h.logger.Errorf("Unknown service '%s' from tenant %s", req.Service, tenantContext.TenantHash[:12])
			h.writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Unknown service: %s", req.Service))
			return
		}
	}

	// Create OAuth token data
	tokenData := &db.OAuthTokenData{
		AccessToken:  req.AccessToken,
		RefreshToken: req.RefreshToken,
		TokenType:    "Bearer",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Store tokens in database
	if err := h.database.StoreOAuthToken(tenantContext.TenantHash, req.Service, tokenData); err != nil {
		h.logger.Errorf("Failed to store OAuth token for tenant %s service %s: %v", 
			tenantContext.TenantHash[:12], req.Service, err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to store tokens")
		return
	}

	h.logger.Infof("Successfully stored OAuth tokens for tenant %s service %s", 
		tenantContext.TenantHash[:12], req.Service)

	// Return success response
	response := TokenResponse{
		Success: true,
		Message: "Tokens stored successfully",
		TokenID: fmt.Sprintf("%s_%s", tenantContext.TenantHash[:12], req.Service),
	}

	h.writeJSONResponse(w, http.StatusCreated, response)
}

// handleAuthVerify handles GET /api/v1/auth/verify
func (h *OAuthAPIHandler) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract tenant context from middleware - if we got here, auth was successful
	tenantContext, ok := r.Context().Value(global.TenantContextKey).(*fusion.TenantContext)
	if !ok {
		h.writeErrorResponse(w, http.StatusUnauthorized, "Invalid authentication")
		return
	}

	response := AuthVerifyResponse{
		Success:   true,
		Message:   "Authentication valid",
		TenantID:  tenantContext.TenantHash[:12] + "...",
		ValidTill: "Token-based authentication (no expiration)",
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// handleServiceConfig handles GET /api/v1/services/{service}/config
func (h *OAuthAPIHandler) handleServiceConfig(w http.ResponseWriter, r *http.Request) {
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

	// Validate service exists
	if h.configManager != nil {
		availableServices := h.configManager.GetAvailableServices()
		serviceFound := false
		for _, service := range availableServices {
			if service == serviceName {
				serviceFound = true
				break
			}
		}
		if !serviceFound {
			h.writeErrorResponse(w, http.StatusNotFound, fmt.Sprintf("Service '%s' not found", serviceName))
			return
		}
	}

	// For now, return a basic success response indicating the service exists
	// In a full implementation, this would return actual OAuth configuration
	response := ServiceConfigResponse{
		Success:     true,
		Message:     "Service configuration retrieved",
		ServiceName: serviceName,
		Config: map[string]interface{}{
			"service_name": serviceName,
			"oauth_available": true,
			"endpoints": map[string]string{
				"token_storage": "/api/v1/oauth/tokens",
				"auth_verify":   "/api/v1/auth/verify",
			},
		},
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// handleOAuthSuccess handles POST /api/v1/oauth/success
func (h *OAuthAPIHandler) handleOAuthSuccess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract tenant context from middleware
	tenantContext, ok := r.Context().Value(global.TenantContextKey).(*fusion.TenantContext)
	if !ok {
		h.writeErrorResponse(w, http.StatusUnauthorized, "Invalid authentication")
		return
	}

	// Parse notification (we don't need to store it, just log it)
	var notification map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		h.logger.Errorf("Failed to decode success notification: %v", err)
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	serviceName, _ := notification["service"].(string)
	h.logger.Infof("OAuth success notification for tenant %s service %s", 
		tenantContext.TenantHash[:12], serviceName)

	h.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Success notification received",
	})
}

// handleOAuthError handles POST /api/v1/oauth/error
func (h *OAuthAPIHandler) handleOAuthError(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract tenant context from middleware
	tenantContext, ok := r.Context().Value(global.TenantContextKey).(*fusion.TenantContext)
	if !ok {
		h.writeErrorResponse(w, http.StatusUnauthorized, "Invalid authentication")
		return
	}

	// Parse notification (we don't need to store it, just log it)
	var notification map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		h.logger.Errorf("Failed to decode error notification: %v", err)
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	serviceName, _ := notification["service"].(string)
	errorMsg, _ := notification["error"].(string)
	h.logger.Warningf("OAuth error notification for tenant %s service %s: %s", 
		tenantContext.TenantHash[:12], serviceName, errorMsg)

	h.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Error notification received",
	})
}

// writeJSONResponse writes a JSON response
func (h *OAuthAPIHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Errorf("Failed to encode JSON response: %v", err)
	}
}

// writeErrorResponse writes a JSON error response
func (h *OAuthAPIHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := map[string]interface{}{
		"success": false,
		"error": map[string]interface{}{
			"code":    statusCode,
			"message": message,
			"type":    "api_error",
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		h.logger.Errorf("Failed to encode error response: %v", err)
	}
}