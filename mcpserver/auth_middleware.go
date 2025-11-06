/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/global"
)

// ServiceProvider interface for getting available services
type ServiceProvider interface {
	GetAvailableServices() []string
}

// AuthMiddleware provides bearer token authentication and tenant context extraction
type AuthMiddleware struct {
	authManager     *fusion.MultiTenantAuthManager
	serviceProvider ServiceProvider
	logger          global.Logger
	skipPaths       []string // Paths that should skip authentication
	requireAuth     bool     // Whether authentication is required for all requests
}

// AuthMiddlewareOption represents configuration options for the auth middleware
type AuthMiddlewareOption func(*AuthMiddleware)

// WithSkipPaths sets paths that should skip authentication
func WithSkipPaths(paths ...string) AuthMiddlewareOption {
	return func(am *AuthMiddleware) {
		am.skipPaths = paths
	}
}

// WithRequireAuth sets whether authentication is required for all requests
func WithRequireAuth(required bool) AuthMiddlewareOption {
	return func(am *AuthMiddleware) {
		am.requireAuth = required
	}
}

// WithAuthLogger sets the logger for the auth middleware
func WithAuthLogger(logger global.Logger) AuthMiddlewareOption {
	return func(am *AuthMiddleware) {
		am.logger = logger
	}
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(authManager *fusion.MultiTenantAuthManager, serviceProvider ServiceProvider,
	options ...AuthMiddlewareOption) *AuthMiddleware {

	am := &AuthMiddleware{
		authManager:     authManager,
		serviceProvider: serviceProvider,
		skipPaths:       []string{"/health", "/metrics", "/status", "/capabilities"},
		requireAuth:     true,
	}

	// Apply options
	for _, option := range options {
		option(am)
	}

	if am.logger != nil {
		am.logger.Info("Initialized authentication middleware")
		am.logger.Debugf("Auth middleware skip paths: %v", am.skipPaths)
		am.logger.Debugf("Auth middleware require auth: %t", am.requireAuth)
	}

	return am
}

// Middleware returns the HTTP middleware function
func (am *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this path should skip authentication
		if am.shouldSkipAuth(r.URL.Path) {
			if am.logger != nil {
				am.logger.Debugf("Skipping authentication for path: %s", r.URL.Path)
			}
			next.ServeHTTP(w, r)
			return
		}

		// Try to extract tool name from MCP request body if this is an MCP call
		var toolName string
		if r.Method == "POST" && r.Header.Get("Content-Type") == "application/json" {
			// Read and buffer the body so we can parse it and still pass it downstream
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil {
				// Restore the body for downstream handlers
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				// Try to parse as MCP request
				var mcpRequest struct {
					Method string `json:"method"`
					Params struct {
						Name string `json:"name"`
					} `json:"params"`
				}
				if err := json.Unmarshal(bodyBytes, &mcpRequest); err == nil {
					if mcpRequest.Method == "tools/call" && mcpRequest.Params.Name != "" {
						toolName = mcpRequest.Params.Name
						if am.logger != nil {
							am.logger.Debugf("Extracted tool name from MCP request: %s", toolName)
						}
						// Add tool name to request context for resolveServiceName to use
						ctx := context.WithValue(r.Context(), global.ToolNameKey, toolName)
						r = r.WithContext(ctx)
					}
				}
			}
		}

		// Extract and validate bearer token
		token := am.extractBearerToken(r)
		if token == "" {
			if am.requireAuth {
				if am.logger != nil {
					am.logger.Warningf("Missing bearer token for authenticated request to %s", r.URL.Path)
				}
				am.writeErrorResponse(w, http.StatusUnauthorized, "Invalid token")
				return
			} else {
				// Auth not required, create NOAUTH tenant context
				if am.logger != nil {
					am.logger.Debugf("No bearer token provided, using NOAUTH tenant context for %s", r.URL.Path)
				}
				// Create NOAUTH tenant context using ExtractTenantFromToken with empty token
				tenantContext, err := am.authManager.ExtractTenantFromToken("")
				if err != nil {
					if am.logger != nil {
						am.logger.Errorf("Failed to create NOAUTH tenant context: %v", err)
					}
					am.writeErrorResponse(w, http.StatusInternalServerError, "Internal error")
					return
				}

				// Add request ID to tenant context
				tenantContext.RequestID = am.generateRequestID(r)

				// Add tenant context to request context
				ctx := context.WithValue(r.Context(), global.TenantContextKey, tenantContext)
				r = r.WithContext(ctx)

				next.ServeHTTP(w, r)
				return
			}
		}

		// Extract tenant context from token
		tenantContext, err := am.authManager.ExtractTenantFromToken(token)
		if err != nil {
			if am.logger != nil {
				am.logger.Errorf("Failed to extract tenant context from token: %v", err)
			}
			am.writeErrorResponse(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		// Add request ID to tenant context
		tenantContext.RequestID = am.generateRequestID(r)

		if am.logger != nil {
			am.logger.Debugf("Extracted tenant context: %s for request %s %s",
				tenantContext.String(), r.Method, r.URL.Path)
		}

		// Try to resolve service name from the request
		serviceName, err := am.resolveServiceName(r, tenantContext)
		if err != nil {
			// If this is a tool call and we couldn't resolve the service, it's an error
			if toolName != "" {
				if am.logger != nil {
					am.logger.Errorf("Failed to resolve service for tool %s: %v", toolName, err)
				}
				am.writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Unknown tool: %s", toolName))
				return
			}
			if am.logger != nil {
				am.logger.Warningf("Failed to resolve service name for request %s %s: %v",
					r.Method, r.URL.Path, err)
			}
			// Continue with default service name for non-tool requests
			serviceName = "default"
		}

		// Update tenant context with resolved service name
		tenantContext.ServiceName = serviceName

		// Validate tenant access to the service
		if err := am.authManager.ValidateTenantAccess(tenantContext, serviceName); err != nil {
			if am.logger != nil {
				am.logger.Errorf("Tenant access validation failed for %s service %s: %v",
					tenantContext.TenantHash[:12]+"...", serviceName, err)
			}
			am.writeErrorResponse(w, http.StatusForbidden, "Access denied to service")
			return
		}

		if am.logger != nil {
			am.logger.Debugf("Successfully authenticated tenant %s for service %s (request %s)",
				tenantContext.TenantHash[:12]+"...", serviceName, tenantContext.RequestID)
		}

		// Add tenant context and service name to request context
		ctx := context.WithValue(r.Context(), global.TenantContextKey, tenantContext)
		ctx = context.WithValue(ctx, global.ServiceNameKey, serviceName)
		r = r.WithContext(ctx)

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}

// shouldSkipAuth checks if authentication should be skipped for a given path
func (am *AuthMiddleware) shouldSkipAuth(path string) bool {
	for _, skipPath := range am.skipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}
	return false
}

// extractBearerToken extracts the bearer token from the Authorization header
func (am *AuthMiddleware) extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	// Check if it's a Bearer token
	if !strings.HasPrefix(authHeader, "Bearer ") {
		if am.logger != nil {
			am.logger.Debugf("Authorization header found but not a Bearer token: %s", authHeader[:min(len(authHeader), 20)]+"...")
		}
		return ""
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		if am.logger != nil {
			am.logger.Debug("Empty bearer token found")
		}
		return ""
	}

	if am.logger != nil {
		am.logger.Debugf("Extracted bearer token (length: %d)", len(token))
	}

	return token
}

// resolveServiceName attempts to resolve the service name from the request
func (am *AuthMiddleware) resolveServiceName(r *http.Request, _ *fusion.TenantContext) (string, error) {
	// Try multiple strategies to determine the service name

	// Strategy 1: Extract from MCP tool name if this is a tool call
	// Check if tool name is in the context (set by MCP server for tool calls)
	if toolName, ok := r.Context().Value(global.ToolNameKey).(string); ok && toolName != "" {
		serviceName, err := global.ExtractServiceFromToolName(toolName)
		if err != nil {
			if am.logger != nil {
				am.logger.Errorf("Failed to extract service from tool name %s: %v", toolName, err)
			}
			return "", fmt.Errorf("invalid tool name %s: %w", toolName, err)
		}

		// Validate that this service exists in our configuration
		if am.serviceProvider != nil {
			availableServices := am.serviceProvider.GetAvailableServices()
			serviceFound := false
			for _, availableService := range availableServices {
				if availableService == serviceName {
					serviceFound = true
					break
				}
			}
			if !serviceFound {
				if am.logger != nil {
					am.logger.Errorf("Service '%s' from tool '%s' not found in available services: %v",
						serviceName, toolName, availableServices)
				}
				return "", fmt.Errorf("service '%s' not configured", serviceName)
			}
		}

		if am.logger != nil {
			am.logger.Debugf("Resolved service name '%s' from tool name: %s", serviceName, toolName)
		}
		return serviceName, nil
	}

	// Strategy 2: Check for service name in query parameters (for non-tool requests)
	if serviceName := r.URL.Query().Get("service"); serviceName != "" {
		if am.logger != nil {
			am.logger.Debugf("Resolved service name from query parameter: %s", serviceName)
		}
		return serviceName, nil
	}

	// Strategy 3: Check for service name in headers (for non-tool requests)
	if serviceName := r.Header.Get("X-Service-Name"); serviceName != "" {
		if am.logger != nil {
			am.logger.Debugf("Resolved service name from header: %s", serviceName)
		}
		return serviceName, nil
	}

	// Strategy 4: Check if there's only one available service for this tenant
	// This is only used when there's no tool name and no explicit service specification
	if am.serviceProvider != nil {
		availableServices := am.serviceProvider.GetAvailableServices()
		if len(availableServices) == 1 {
			serviceName := availableServices[0]
			if am.logger != nil {
				am.logger.Debugf("Using single available service: %s", serviceName)
			}
			return serviceName, nil
		}
	}

	if am.logger != nil {
		am.logger.Debug("Could not resolve service name from request, using default")
	}

	// Default fallback
	return "default", fmt.Errorf("could not determine service name from request")
}

// generateRequestID generates a unique request ID for tracking
func (am *AuthMiddleware) generateRequestID(r *http.Request) string {
	// Check if request ID already exists in headers
	if existingID := r.Header.Get("X-Request-ID"); existingID != "" {
		return existingID
	}

	// Generate a simple request ID based on timestamp and remote address
	timestamp := time.Now().Unix()
	remoteAddr := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		remoteAddr = strings.Split(forwarded, ",")[0]
	}

	return fmt.Sprintf("req_%d_%s", timestamp, strings.ReplaceAll(remoteAddr, ":", "_"))
}

// writeErrorResponse writes a JSON error response
func (am *AuthMiddleware) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    statusCode,
			"message": message,
			"type":    "authentication_error",
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	// Don't log the error if JSON encoding fails - just write a simple response
	if jsonBytes, err := json.Marshal(errorResponse); err == nil {
		_, _ = w.Write(jsonBytes)
	} else {
		_, _ = w.Write([]byte(fmt.Sprintf(`{"error":{"code":%d,"message":"%s"}}`, statusCode, message)))
	}
}

// AuthValidationMiddleware is a simpler middleware that only validates authentication without tenant resolution
type AuthValidationMiddleware struct {
	authManager *fusion.MultiTenantAuthManager
	logger      global.Logger
	skipPaths   []string
}

// Middleware returns the HTTP middleware function for simple auth validation
func (avm *AuthValidationMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this path should skip authentication
		for _, skipPath := range avm.skipPaths {
			if strings.HasPrefix(r.URL.Path, skipPath) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Extract bearer token
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			if avm.logger != nil {
				avm.logger.Warningf("Missing or invalid authorization header for %s %s", r.Method, r.URL.Path)
			}
			avm.writeError(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			if avm.logger != nil {
				avm.logger.Warning("Empty bearer token provided")
			}
			avm.writeError(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		// Validate token format and extract tenant context
		_, err := avm.authManager.ExtractTenantFromToken(token)
		if err != nil {
			if avm.logger != nil {
				avm.logger.Errorf("Token validation failed: %v", err)
			}
			avm.writeError(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		if avm.logger != nil {
			avm.logger.Debugf("Token validated successfully for %s %s", r.Method, r.URL.Path)
		}

		next.ServeHTTP(w, r)
	})
}

// writeError writes a simple error response
func (avm *AuthValidationMiddleware) writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"error":{"code":%d,"message":"%s"}}`, statusCode, message)))
}

// SimpleMiddleware provides a simplified middleware that ONLY validates bearer tokens
// and extracts tenant context without service resolution or tool-specific logic.
// This is intended for use at the HTTP transport level where MCP-level authentication
// will handle tool-specific validation.
func (am *AuthMiddleware) SimpleMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this path should skip authentication
		if am.shouldSkipAuth(r.URL.Path) {
			if am.logger != nil {
				am.logger.Debugf("Simple Auth: Skipping authentication for path: %s", r.URL.Path)
			}
			next.ServeHTTP(w, r)
			return
		}

		// Extract and validate bearer token
		token := am.extractBearerToken(r)
		if token == "" {
			if am.requireAuth {
				if am.logger != nil {
					am.logger.Warningf("Simple Auth: Missing bearer token for authenticated request to %s", r.URL.Path)
				}
				am.writeErrorResponse(w, http.StatusUnauthorized, "Invalid token")
				return
			} else {
				// Auth not required, create NOAUTH tenant context
				if am.logger != nil {
					am.logger.Debugf("Simple Auth: No bearer token provided, using NOAUTH tenant context for %s", r.URL.Path)
				}
				// Create NOAUTH tenant context using ExtractTenantFromToken with empty token
				tenantContext, err := am.authManager.ExtractTenantFromToken("")
				if err != nil {
					if am.logger != nil {
						am.logger.Errorf("Simple Auth: Failed to create NOAUTH tenant context: %v", err)
					}
					am.writeErrorResponse(w, http.StatusInternalServerError, "Internal error")
					return
				}

				// Add request ID to tenant context
				tenantContext.RequestID = am.generateRequestID(r)

				// Add tenant context to request context
				ctx := context.WithValue(r.Context(), global.TenantContextKey, tenantContext)
				r = r.WithContext(ctx)

				next.ServeHTTP(w, r)
				return
			}
		}

		// Extract tenant context from token
		tenantContext, err := am.authManager.ExtractTenantFromToken(token)
		if err != nil {
			if am.logger != nil {
				am.logger.Errorf("Simple Auth: Failed to extract tenant context from token: %v", err)
			}
			am.writeErrorResponse(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		// Add request ID to tenant context
		tenantContext.RequestID = am.generateRequestID(r)

		if am.logger != nil {
			am.logger.Debugf("Simple Auth: Extracted tenant context: %s for request %s %s",
				tenantContext.String(), r.Method, r.URL.Path)
		}

		// Add tenant context to request context without service resolution
		ctx := context.WithValue(r.Context(), global.TenantContextKey, tenantContext)
		r = r.WithContext(ctx)

		// Continue to next handler - service validation will happen at MCP level
		next.ServeHTTP(w, r)
	})
}
