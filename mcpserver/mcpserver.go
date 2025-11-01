/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package mcpserver

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/global"
)

// Option defines a function type for configuring the MCPServer.
type Option func(*MCPServer)

// MCPServerTransport is an interface that abstracts the different transport types
//
//goland:noinspection GoNameStartsWithPackageName
type MCPServerTransport interface {
	Start(addr string) error
	Shutdown(ctx context.Context) error
}

// AuthenticatedTransport wraps an underlying transport with authentication middleware
type AuthenticatedTransport struct {
	underlying MCPServerTransport
	handler    http.Handler
	server     *http.Server
	logger     global.Logger
}

// NewAuthenticatedTransport creates a new authenticated transport wrapper
func NewAuthenticatedTransport(underlying MCPServerTransport, middleware func(http.Handler) http.Handler, logger global.Logger) *AuthenticatedTransport {
	// Extract the http.Handler from the underlying transport
	var handler http.Handler
	if h, ok := underlying.(http.Handler); ok {
		handler = middleware(h)
	} else {
		logger.Error("Underlying transport does not implement http.Handler")
		return nil
	}

	return &AuthenticatedTransport{
		underlying: underlying,
		handler:    handler,
		logger:     logger,
	}
}

// Start starts the authenticated transport
func (at *AuthenticatedTransport) Start(addr string) error {
	if at.logger != nil {
		at.logger.Infof("Starting authenticated transport on %s", addr)
	}

	// Create HTTP server with our wrapped handler
	at.server = &http.Server{
		Addr:         addr,
		Handler:      at.handler,
		ReadTimeout:  0,                      // No timeout for reading request
		WriteTimeout: 3600 * time.Second,     // 1 hour timeout for writing response (allows long-running commands)
		IdleTimeout:  120 * time.Second,      // 2 minutes idle timeout
	}

	return at.server.ListenAndServe()
}

// Shutdown shuts down the authenticated transport
func (at *AuthenticatedTransport) Shutdown(ctx context.Context) error {
	if at.server != nil {
		return at.server.Shutdown(ctx)
	}
	return nil
}

// ServeHTTP implements http.Handler interface to allow this transport to be wrapped by other middleware
func (at *AuthenticatedTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if at.handler != nil {
		at.handler.ServeHTTP(w, r)
	} else {
		http.Error(w, "Handler not configured", http.StatusInternalServerError)
	}
}

// MCPServer represents the server instance.
type MCPServer struct {
	listen            string
	srv               *server.MCPServer
	sseServer         *server.SSEServer
	httpServer        *server.StreamableHTTPServer
	transport         MCPServerTransport
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	logger            global.Logger
	debug             bool
	name              string
	version           string
	noStreaming       bool
	toolProviders     []global.ToolProvider
	resourceProviders []global.ResourceProvider
	promptProviders   []global.PromptProvider
	authMiddleware    *AuthMiddleware
	database          *db.DB
	authManager       *fusion.MultiTenantAuthManager
	configManager     ServiceProvider
}

func WithListen(listen string) Option {
	return func(m *MCPServer) {
		m.listen = listen
	}
}

func WithLogger(logger global.Logger) Option {
	return func(m *MCPServer) {
		m.logger = logger
	}
}

func WithDebug(debug bool) Option {
	return func(m *MCPServer) {
		m.debug = debug
	}
}

func WithName(name string) Option {
	return func(m *MCPServer) {
		m.name = name
	}
}

func WithVersion(version string) Option {
	return func(m *MCPServer) {
		m.version = version
	}
}

func WithToolProviders(providers []global.ToolProvider) Option {
	return func(s *MCPServer) {
		s.toolProviders = providers
	}
}

func WithResourceProviders(providers []global.ResourceProvider) Option {
	return func(s *MCPServer) {
		s.resourceProviders = providers
	}
}

func WithPromptProviders(providers []global.PromptProvider) Option {
	return func(s *MCPServer) {
		s.promptProviders = providers
	}
}

func WithNoStreaming(noStreaming bool) Option {
	return func(m *MCPServer) {
		m.noStreaming = noStreaming
	}
}

func WithAuthMiddleware(authMiddleware *AuthMiddleware) Option {
	return func(m *MCPServer) {
		m.authMiddleware = authMiddleware
	}
}

func WithDatabase(database *db.DB) Option {
	return func(m *MCPServer) {
		m.database = database
	}
}

func WithAuthManager(authManager *fusion.MultiTenantAuthManager) Option {
	return func(m *MCPServer) {
		m.authManager = authManager
	}
}

func WithConfigManager(configManager ServiceProvider) Option {
	return func(m *MCPServer) {
		m.configManager = configManager
	}
}

// New creates a new MCPServer instance with the provided options.
func New(options ...Option) (*MCPServer, error) {

	// Create a new MCPServer instance with default values
	// This is a wrapper around the mcp-go server
	m := &MCPServer{
		listen:      "localhost:8080",
		srv:         nil,
		sseServer:   nil,
		httpServer:  nil,
		transport:   nil,
		ctx:         nil,
		cancel:      nil,
		logger:      nil,
		debug:       false,
		name:        "Generic-MCP",
		version:     "0.0.1",
		noStreaming: false,
		wg:          sync.WaitGroup{},
	}

	// Apply options
	for _, opt := range options {
		opt(m)
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

	// Create an MCP server using the mcp-go library with proper middleware ordering
	// 1. Basic server capabilities (logging, recovery)
	// 2. Request logging for debugging 
	// 3. MCP-level authentication for tool-specific validation
	// 4. Hooks for provider integration
	serverOptions := []server.ServerOption{
		server.WithLogging(),
		server.WithRecovery(),
		WithRequestLogging(m.logger), // Our custom request logging middleware
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

// Start runs the MCP server in a background goroutine and checks for a logger.
func (s *MCPServer) Start() error {
	if s.logger == nil {
		return fmt.Errorf("logger not set")
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		// Log the start
		if s.noStreaming {
			s.logger.Infof("MCP server listening on TCP port %s (HTTP mode)", s.listen)
		} else {
			s.logger.Infof("MCP server listening on TCP port %s (SSE mode)", s.listen)
		}

		// Create the appropriate server based on streaming preference
		if s.noStreaming {
			// Create HTTP server for non-streaming mode
			s.httpServer = server.NewStreamableHTTPServer(s.srv)
			s.transport = s.httpServer
		} else {
			// Create SSE server for streaming mode (default)
			s.sseServer = server.NewSSEServer(s.srv)
			s.transport = s.sseServer
		}

		// Apply HTTP-level authentication for all requests (simple token validation)
		// MCP-level authentication will handle tool-specific validation
		if s.authMiddleware != nil {
			if s.logger != nil {
				if s.noStreaming {
					s.logger.Info("Applying simple HTTP authentication middleware to HTTP server")
				} else {
					s.logger.Info("Applying simple HTTP authentication middleware to SSE server")
				}
			}
			s.transport = NewAuthenticatedTransport(s.transport, s.authMiddleware.SimpleMiddleware, s.logger)
			if s.transport == nil {
				if s.logger != nil {
					s.logger.Error("Failed to create authenticated transport, continuing without HTTP authentication")
				}
				if s.noStreaming {
					s.transport = s.httpServer
				} else {
					s.transport = s.sseServer
				}
			}
		}

		// Check if OAuth API functionality should be enabled
		if s.database != nil && s.authManager != nil && s.configManager != nil {
			if s.logger != nil {
				s.logger.Info("Enabling OAuth API endpoints with extended transport")
			}
			// Wrap with ExtendedTransport to add OAuth API endpoints
			// This should preserve the existing transport with simple auth
			originalTransport := s.transport
			s.transport = NewExtendedTransport(originalTransport, s.database, s.authManager, 
				s.configManager, nil, s.logger) // OAuth API uses its own auth for /api/v1/oauth/* routes
			if s.transport == nil {
				if s.logger != nil {
					s.logger.Error("Failed to create extended transport, falling back to original transport")
				}
				s.transport = originalTransport
			}
		}

		// Start the server
		err := s.transport.Start(s.listen)
		// We don't need to log anything here - if the server is shutting down,
		// this is expected behavior and not an error condition
		_ = err
		return
	}()
	return nil
}

// Stop signals the MCP server to shut down and waits for the goroutine to exit.
func (s *MCPServer) Stop() error {
	// First cancel the context to signal all operations to stop
	if s.cancel != nil {
		s.cancel()
	}

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
