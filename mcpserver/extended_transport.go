/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package mcpserver

import (
	"context"
	"net/http"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/global"
)

// ExtendedTransport wraps an MCPServerTransport and adds custom API endpoints
type ExtendedTransport struct {
	underlying    MCPServerTransport
	server        *http.Server
	logger        global.Logger
	oauthHandler  *OAuthAPIHandler
}

// NewExtendedTransport creates a transport that combines MCP functionality with custom API endpoints
func NewExtendedTransport(underlying MCPServerTransport, database *db.DB, 
	authManager *fusion.MultiTenantAuthManager, configManager ServiceProvider,
	authMiddleware func(http.Handler) http.Handler, logger global.Logger) *ExtendedTransport {

	// Create OAuth API handler
	oauthHandler := NewOAuthAPIHandler(database, authManager, configManager, logger)

	// Create a new ServeMux for routing
	mux := http.NewServeMux()

	// Register OAuth API routes with authentication middleware (if available)
	tempMux := http.NewServeMux()
	oauthHandler.RegisterRoutes(tempMux)
	if authMiddleware != nil {
		mux.Handle("/api/", authMiddleware(tempMux))
		mux.Handle("/ping", authMiddleware(tempMux))
	} else {
		mux.Handle("/api/", tempMux)
		mux.Handle("/ping", tempMux)
	}

	// Handle MCP requests by delegating to the underlying transport
	// The underlying transport should implement http.Handler
	if mcpHandler, ok := underlying.(http.Handler); ok {
		// For all other paths, delegate to the MCP handler
		mux.Handle("/", mcpHandler)
	} else {
		logger.Error("Underlying transport does not implement http.Handler")
		return nil
	}

	return &ExtendedTransport{
		underlying:   underlying,
		logger:       logger,
		oauthHandler: oauthHandler,
		server: &http.Server{
			Handler: mux,
		},
	}
}

// Start starts the extended transport with both MCP and API functionality
func (et *ExtendedTransport) Start(addr string) error {
	if et.logger != nil {
		et.logger.Infof("Starting extended transport with OAuth API on %s", addr)
	}

	et.server.Addr = addr
	return et.server.ListenAndServe()
}

// Shutdown shuts down the extended transport
func (et *ExtendedTransport) Shutdown(ctx context.Context) error {
	if et.server != nil {
		return et.server.Shutdown(ctx)
	}
	return nil
}

// ServeHTTP implements http.Handler to allow this transport to be wrapped by other middleware
func (et *ExtendedTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	et.server.Handler.ServeHTTP(w, r)
}