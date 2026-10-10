/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/global"
)

// Option configures the MCPServer.
type Option interface {
	applyToServer(*MCPServer)
}

// optionFunc adapts a function to the Option interface.
type optionFunc func(*MCPServer)

func (o optionFunc) applyToServer(m *MCPServer) { o(m) }

// MCPServerTransport is an interface that abstracts the different transport types
//
//goland:noinspection GoNameStartsWithPackageName
type MCPServerTransport interface {
	Start(addr string) error
	Shutdown(ctx context.Context) error
}

// AuthenticatedTransport wraps an underlying transport handler with authentication middleware
type AuthenticatedTransport struct {
	handler http.Handler
	server  *http.Server
	logger  global.Logger
}

// NewAuthenticatedTransport creates a new authenticated transport wrapper. The
// underlying transport is taken as an http.Handler so that every request it
// serves always passes through the middleware.
func NewAuthenticatedTransport(underlying http.Handler, middleware func(http.Handler) http.Handler, opts ...TransportOption) *AuthenticatedTransport {
	logger := newTransportOptions(opts).logger
	handler := middleware(underlying)
	return &AuthenticatedTransport{
		handler: handler,
		logger:  logger,
		server: &http.Server{
			Handler:      handler,
			ReadTimeout:  0,                  // No timeout for reading request
			WriteTimeout: 3600 * time.Second, // 1 hour timeout for writing response (allows long-running commands)
			IdleTimeout:  120 * time.Second,  // 2 minutes idle timeout
		},
	}
}

// Start listens on addr and serves the authenticated transport.
func (at *AuthenticatedTransport) Start(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return at.Serve(ln)
}

// Serve serves the authenticated transport on an already open listener.
func (at *AuthenticatedTransport) Serve(ln net.Listener) error {
	if at.logger != nil {
		at.logger.Infof("Starting authenticated transport on %s", ln.Addr())
	}
	return at.server.Serve(ln)
}

// Shutdown shuts down the authenticated transport
func (at *AuthenticatedTransport) Shutdown(ctx context.Context) error {
	return at.server.Shutdown(ctx)
}

// ServeHTTP implements http.Handler interface to allow this transport to be wrapped by other middleware
func (at *AuthenticatedTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	at.handler.ServeHTTP(w, r)
}

// listenerTransport is a transport that serves on a listener opened by MCPServer,
// so that a failure to listen is reported by Start rather than lost.
type listenerTransport interface {
	Serve(ln net.Listener) error
	Shutdown(ctx context.Context) error
}

// sseOnlyTransport serves a bare SSE transport when neither authentication nor
// the OAuth API is configured.
type sseOnlyTransport struct {
	sse    *server.SSEServer
	server *http.Server
}

// Serve serves the SSE transport on an already open listener.
func (t *sseOnlyTransport) Serve(ln net.Listener) error {
	return t.server.Serve(ln)
}

// Shutdown closes the SSE sessions and shuts down the HTTP server.
func (t *sseOnlyTransport) Shutdown(ctx context.Context) error {
	t.sse.CloseSessions()
	return t.server.Shutdown(ctx)
}

// MCPServer represents the server instance.
type MCPServer struct {
	listen            string
	srv               *server.MCPServer
	sseServer         *server.SSEServer
	httpServer        *server.StreamableHTTPServer
	transport         listenerTransport
	wg                sync.WaitGroup
	logger            global.Logger
	debug             bool
	name              string
	version           string
	toolProviders     []global.ToolProvider
	resourceProviders []global.ResourceProvider
	promptProviders   []global.PromptProvider
	authMiddleware    *AuthMiddleware
	database          *db.DB
	authManager       *fusion.MultiTenantAuthManager
	configManager     ServiceProvider
	oauthEngine       OAuthRouteProvider
	authorizer        global.Authorizer
}

func WithListen(listen string) Option {
	return optionFunc(func(m *MCPServer) {
		m.listen = listen
	})
}

func WithDebug(debug bool) Option {
	return optionFunc(func(m *MCPServer) {
		m.debug = debug
	})
}

func WithName(name string) Option {
	return optionFunc(func(m *MCPServer) {
		m.name = name
	})
}

func WithVersion(version string) Option {
	return optionFunc(func(m *MCPServer) {
		m.version = version
	})
}

func WithToolProviders(providers []global.ToolProvider) Option {
	return optionFunc(func(s *MCPServer) {
		s.toolProviders = providers
	})
}

func WithResourceProviders(providers []global.ResourceProvider) Option {
	return optionFunc(func(s *MCPServer) {
		s.resourceProviders = providers
	})
}

func WithPromptProviders(providers []global.PromptProvider) Option {
	return optionFunc(func(s *MCPServer) {
		s.promptProviders = providers
	})
}

func WithAuthMiddleware(authMiddleware *AuthMiddleware) Option {
	return optionFunc(func(m *MCPServer) {
		m.authMiddleware = authMiddleware
	})
}

func WithDatabase(database *db.DB) Option {
	return optionFunc(func(m *MCPServer) {
		m.database = database
	})
}

func WithAuthManager(authManager *fusion.MultiTenantAuthManager) Option {
	return optionFunc(func(m *MCPServer) {
		m.authManager = authManager
	})
}

func WithConfigManager(configManager ServiceProvider) Option {
	return optionFunc(func(m *MCPServer) {
		m.configManager = configManager
	})
}

// WithOAuthEngine sets the fusion engine that serves the OAuth token-management
// HTTP API routes. Required to enable the OAuth API endpoints on the extended transport.
func WithOAuthEngine(engine OAuthRouteProvider) Option {
	return optionFunc(func(m *MCPServer) {
		m.oauthEngine = engine
	})
}

func WithAuthorizer(authorizer global.Authorizer) Option {
	return optionFunc(func(m *MCPServer) {
		m.authorizer = authorizer
	})
}

// New creates a new MCPServer instance with the provided options.
func New(options ...Option) (*MCPServer, error) {

	// Create a new MCPServer instance with default values
	// This is a wrapper around the mcp-go server
	m := &MCPServer{
		listen:     "localhost:8080",
		srv:        nil,
		sseServer:  nil,
		httpServer: nil,
		transport:  nil,
		logger:     nil,
		debug:      false,
		name:       "Generic-MCP",
		version:    "0.0.1",
		wg:         sync.WaitGroup{},
	}

	// Apply options
	for _, opt := range options {
		opt.applyToServer(m)
	}

	// If there is no logger, create one
	if m.logger == nil {
		return nil, fmt.Errorf("logger not set")
	}

	// Create hooks
	hooks := &server.Hooks{}
	hooks.AddAfterListPrompts(m.hookAfterListPrompts)
	hooks.AddAfterListResources(m.hookAfterListResources)
	hooks.AddAfterListResourceTemplates(m.hookAfterListResourceTemplates)
	hooks.AddAfterListTools(m.hookAfterListTools)
	hooks.AddAfterCallTool(m.hookAfterCallTool)

	// Create an MCP server using the mcp-go library with proper middleware ordering
	// 1. Basic server capabilities (logging, recovery)
	// 2. Request logging for debugging
	// 3. MCP-level authentication for tool-specific validation
	// 4. Hooks for provider integration
	serverOptions := []server.ServerOption{
		server.WithLogging(),
		server.WithRecovery(),
		WithRequestLogging(m.logger),      // Our custom request logging middleware
		server.WithToolCapabilities(true), // Enable dynamic tool list change notifications
	}

	// Add MCP authentication middleware if configured
	if m.authManager != nil {
		authOptions := []MCPAuthOption{
			WithMCPAuthManager(m.authManager),
			WithMCPLogger(m.logger),
		}
		if m.configManager != nil {
			authOptions = append(authOptions, WithMCPServiceProvider(m.configManager))
		}
		if m.authorizer != nil {
			authOptions = append(authOptions, WithMCPAuthorizer(m.authorizer))
		}
		serverOptions = append(serverOptions, WithMCPAuthentication(authOptions...))
	}

	// Add hooks last to ensure they see the fully processed requests
	serverOptions = append(serverOptions, server.WithHooks(hooks))

	m.srv = server.NewMCPServer(m.name, m.version, serverOptions...)

	// Tools are in a separate file for better organization
	m.AddTools()
	m.AddResources()
	m.AddResourceTemplates()
	m.AddPrompts()

	// Return the MCPServer instance
	return m, nil
}

// Start opens the listener and serves the MCP transports in a background
// goroutine. It returns an error if the listener cannot be opened.
func (s *MCPServer) Start() error {
	if s.logger == nil {
		return fmt.Errorf("logger not set")
	}

	// Create both transports - clients can use either
	s.sseServer = server.NewSSEServer(s.srv) // Handles /sse and /message
	// Configure Streamable HTTP transport for /mcp
	// Disable GET streaming since MCPFusion doesn't send server-initiated notifications.
	// This returns 405 Method Not Allowed for GET /mcp (per MCP spec), which is cleaner
	// than opening an SSE stream that never sends data (causing client timeouts).
	// POST /mcp works normally for request/response operations.
	s.httpServer = server.NewStreamableHTTPServer(s.srv,
		server.WithDisableStreaming(true),
	) // Handles /mcp

	// Apply HTTP-level authentication to both transports
	var authenticatedSSE, authenticatedHTTP MCPServerTransport
	authenticatedSSE = s.sseServer
	authenticatedHTTP = s.httpServer

	var authSSE *AuthenticatedTransport
	if s.authMiddleware != nil {
		s.logger.Info("Applying HTTP authentication middleware to both transports")
		authSSE = NewAuthenticatedTransport(s.sseServer, s.authMiddleware.SimpleMiddleware, WithLogger(s.logger))
		authenticatedSSE = authSSE
		authenticatedHTTP = NewAuthenticatedTransport(s.httpServer, s.authMiddleware.SimpleMiddleware, WithLogger(s.logger))
	}

	switch {
	case s.database != nil && s.authManager != nil && s.configManager != nil && s.oauthEngine != nil:
		s.logger.Info("Enabling OAuth API endpoints with extended transport")
		// Build auth middleware for OAuth API routes (/ping, /api/*)
		var oauthAuthMiddleware func(http.Handler) http.Handler
		if s.authMiddleware != nil {
			oauthAuthMiddleware = s.authMiddleware.SimpleMiddleware
		}
		// Wrap both transports with ExtendedTransport to add OAuth API endpoints
		s.transport = NewExtendedTransport(authenticatedSSE, authenticatedHTTP, s.oauthEngine,
			oauthAuthMiddleware, WithLogger(s.logger))
	case authSSE != nil:
		// No OAuth API - just use SSE transport with both available through routing
		s.logger.Warning("OAuth API disabled - using SSE transport only")
		s.transport = authSSE
	default:
		s.logger.Warning("OAuth API disabled - using SSE transport only")
		s.transport = &sseOnlyTransport{sse: s.sseServer, server: &http.Server{Handler: s.sseServer}}
	}

	// Open the listener before returning so that a busy or invalid address
	// fails startup instead of leaving the process running with nothing listening.
	ln, err := net.Listen("tcp", s.listen)
	if err != nil {
		return fmt.Errorf("open listener: %w", err)
	}

	s.logger.Infof("MCP server listening on TCP port %s", s.listen)
	s.logger.Info("Available endpoints: /sse, /message (SSE mode), /mcp (Streamable HTTP mode)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// http.ErrServerClosed is the expected result of a shutdown.
		if err := s.transport.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Errorf("MCP server stopped serving: %v", err)
		}
	}()
	return nil
}

// Stop signals the MCP server to shut down and waits for the goroutine to exit.
func (s *MCPServer) Stop() error {
	if s.transport != nil {
		// Attempt graceful shutdown with a timeout
		// Use a shorter timeout to avoid the context deadline exceeded error
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Shutdown the server and ignore all errors during shutdown
		// This prevents both the ErrServerClosed and context deadline exceeded errors
		_ = s.transport.Shutdown(ctx)
	}

	// Wait for the server goroutine to exit with a timeout
	waitCh := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(waitCh)
	}()

	// Wait for either the waitgroup to finish or a timeout
	select {
	case <-waitCh:
		// Goroutine completed successfully
		return nil
	case <-time.After(1 * time.Second):
		// If we're still waiting after 1 second, continue anyway
		// This prevents the context deadline exceeded error
		return nil
	}
}

// Server returns the underlying mcp-go server for dynamic tool management
func (s *MCPServer) Server() *server.MCPServer {
	return s.srv
}

// WithRequestLogging is a middleware function that logs request details.
func WithRequestLogging(logger global.Logger) server.ServerOption {
	return server.WithToolHandlerMiddleware(func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

			// Log the request details
			logger.Debugf("Request: %+v", request)

			// Call the next handler in the chain
			return next(ctx, request)
		}
	})
}
