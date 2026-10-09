/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package mcpserver

import (
	"context"
	"net"
	"net/http"

	"github.com/PivotLLM/MCPFusion/global"
)

// OAuthRouteProvider registers the OAuth token-management HTTP API routes onto a
// mux. It is implemented by *fusion.Fusion, letting the transport mount the exact
// endpoints the fusion-auth utility calls without owning the handler logic.
type OAuthRouteProvider interface {
	RegisterOAuthRoutes(mux *http.ServeMux)
}

// ExtendedTransport wraps multiple MCP transports and adds custom API endpoints
type ExtendedTransport struct {
	sseTransport  MCPServerTransport
	httpTransport MCPServerTransport
	server        *http.Server
	logger        global.Logger
}

// NewExtendedTransport creates a transport that combines both MCP transports with custom API endpoints
func NewExtendedTransport(sseTransport, httpTransport MCPServerTransport, oauthEngine OAuthRouteProvider,
	authMiddleware func(http.Handler) http.Handler, logger global.Logger) *ExtendedTransport {

	// Create a new ServeMux for routing
	mux := http.NewServeMux()

	// Register OAuth API routes (from the fusion engine) with authentication
	// middleware (if available)
	tempMux := http.NewServeMux()
	oauthEngine.RegisterOAuthRoutes(tempMux)
	if authMiddleware != nil {
		mux.Handle("/api/", authMiddleware(tempMux))
		mux.Handle("/ping", authMiddleware(tempMux))
	} else {
		mux.Handle("/api/", tempMux)
		mux.Handle("/ping", tempMux)
	}

	// Mount SSE transport endpoints (/sse and /message)
	// The SSEServer handles both internally when it gets requests to these paths
	if sseHandler, ok := sseTransport.(http.Handler); ok {
		// SSEServer's ServeHTTP will route between /sse and /message internally
		mux.Handle("/sse", sseHandler)
		mux.Handle("/message", sseHandler)
		logger.Info("Mounted SSE transport at /sse and /message")
	} else {
		logger.Error("SSE transport does not implement http.Handler")
	}

	// Mount Streamable HTTP transport at /mcp (per MCP specification)
	if httpHandler, ok := httpTransport.(http.Handler); ok {
		mux.Handle("/mcp", httpHandler)
		logger.Info("Mounted Streamable HTTP transport at /mcp")
	} else {
		logger.Error("HTTP transport does not implement http.Handler")
	}

	return &ExtendedTransport{
		sseTransport:  sseTransport,
		httpTransport: httpTransport,
		logger:        logger,
		server: &http.Server{
			Handler: mux,
		},
	}
}

// Start listens on addr and serves both MCP transports and the API functionality.
func (et *ExtendedTransport) Start(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return et.Serve(ln)
}

// Serve serves both MCP transports and the API functionality on an already open listener.
func (et *ExtendedTransport) Serve(ln net.Listener) error {
	if et.logger != nil {
		et.logger.Infof("Starting extended transport with both MCP transports and OAuth API on %s", ln.Addr())
	}
	return et.server.Serve(ln)
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
