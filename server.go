/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/PivotLLM/MCPFusion/app"
	"github.com/PivotLLM/MCPFusion/config"
	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/PivotLLM/MCPFusion/hub"
	"github.com/PivotLLM/MCPFusion/mcpserver"
	"github.com/PivotLLM/MCPFusion/metrics"
	"github.com/PivotLLM/MCPFusion/providers/health"
	"github.com/PivotLLM/MCPFusion/providers/knowledge"
	"github.com/PivotLLM/MCPFusion/providers/perf"
)

// server holds what is needed to wire and run the MCP server.
type server struct {
	logger           global.Logger
	database         db.Database
	listen           string
	configFiles      []string
	debug            bool
	noAuth           bool
	knowledgeEnabled bool
	perfEnabled      bool
}

// run wires the providers and the MCP server, serves until ctx is cancelled,
// then shuts down. It closes the database before it returns, on every path.
func (s server) run(ctx context.Context) error {
	// Auto-migrate unlinked API keys to user accounts on startup
	if err := s.database.AutoMigrateKeys(); err != nil {
		s.logger.Warningf("API key auto-migration had issues: %v", err)
	}

	// Initialize database-backed cache
	dbCache := fusion.NewDatabaseCache(s.database, fusion.WithLogger(s.logger))

	// Create multi-tenant authentication manager. It registers the full canonical
	// set of auth strategies (see fusion.defaultStrategies), so no manual
	// per-strategy registration is needed here.
	multiTenantAuth := fusion.NewMultiTenantAuthManager(s.database, dbCache, fusion.WithLogger(s.logger))

	// Initialize config manager with all configuration files
	configManager := config.New(
		config.WithLogger(s.logger),
		config.WithConfigFiles(s.configFiles...),
	)

	// Load all configurations
	if err := configManager.LoadConfigs(); err != nil {
		s.logger.Errorf("Failed to load configurations: %v", err)
		// Continue anyway - server can run without configs
	}

	// Log what was loaded
	serviceCount := configManager.ServiceCount()
	commandCount := configManager.CommandCount()

	if serviceCount > 0 || commandCount > 0 {
		if serviceCount > 0 && commandCount > 0 {
			s.logger.Infof("Loaded %d services and %d command groups from configuration files",
				serviceCount, commandCount)
		} else if serviceCount > 0 {
			s.logger.Infof("Loaded %d services from configuration files", serviceCount)
		} else {
			s.logger.Infof("Loaded %d command groups from configuration files", commandCount)
		}
	} else {
		s.logger.Warning("No services or commands loaded from configuration files")
	}

	s.logger.Info("Multi-tenant authentication system initialized")

	// Create shared metrics collector for cross-package health reporting
	sharedCollector := metrics.New()

	// Create a slice (list) of tool providers
	var providers []global.ToolProvider

	// Add fusion provider if configurations were loaded
	var fusionProvider *fusion.Fusion
	if serviceCount > 0 || commandCount > 0 {
		s.logger.Infof("Creating fusion provider with %d services and %d command groups",
			serviceCount, commandCount)

		// Configure fusion provider with config manager
		fusionOpts := []fusion.Option{
			fusion.WithLogger(s.logger),
			fusion.WithConfigManager(configManager),
			fusion.WithSharedCollector(sharedCollector),
		}

		// Set external URL for auth setup tools
		if externalURL := os.Getenv("MCP_FUSION_EXTERNAL_URL"); externalURL != "" {
			fusionOpts = append(fusionOpts, fusion.WithExternalURL(externalURL))
			s.logger.Infof("External URL for auth setup: %s", externalURL)
		} else {
			fusionOpts = append(fusionOpts, fusion.WithExternalURL("http://"+s.listen))
			s.logger.Warningf("MCP_FUSION_EXTERNAL_URL not set, using http://%s (may not be reachable externally)", s.listen)
		}

		// Set download directory for binary responses
		if dlDir := os.Getenv("MCP_FUSION_DL_DIR"); dlDir != "" {
			fusionOpts = append(fusionOpts, fusion.WithDownloadDir(dlDir))
			s.logger.Infof("Download directory: %s", dlDir)
		}

		// Add multi-tenant support if available
		if multiTenantAuth != nil {
			fusionOpts = append(fusionOpts, fusion.WithMultiTenantAuth(multiTenantAuth))
		}

		// Provide database for native tools (e.g., knowledge store)
		fusionOpts = append(fusionOpts, fusion.WithDatabase(s.database))

		var err error
		fusionProvider, err = fusion.New(fusionOpts...)
		if err != nil {
			s.release(nil, nil)
			return fmt.Errorf("unable to create fusion provider: %w", err)
		}
		providers = append(providers, fusionProvider)
	} else {
		s.logger.Warning("No fusion provider created - no configurations loaded")
	}

	// Register native tool prefixes with the config manager so the auth middleware
	// recognises health, knowledge, and perf as valid service names.
	// health is always enabled; knowledge and perf are registered conditionally below.
	configManager.RegisterNativeToolPrefix("health")

	// Health provider (always enabled).
	healthOpts := []health.Option{
		health.WithLogger(s.logger),
		health.WithCollector(sharedCollector),
	}
	if fusionProvider != nil {
		healthOpts = append(healthOpts, health.WithCircuitBreakerSource(fusionProvider.CircuitBreakerSource()))
	}
	healthProvider := health.New(healthOpts...)
	providers = append(providers, healthProvider)

	// Knowledge provider (enabled unless MCP_FUSION_KNOWLEDGE=false/0/no).
	if s.knowledgeEnabled {
		configManager.RegisterNativeToolPrefix("knowledge")
		knowledgeProvider := knowledge.New(
			knowledge.WithLogger(s.logger),
			knowledge.WithDatabase(s.database),
			knowledge.WithCollector(sharedCollector),
			knowledge.WithUserIDExtractor(func(ctx context.Context) (string, error) {
				tc, ok := ctx.Value(global.TenantContextKey).(*fusion.TenantContext)
				if !ok || tc == nil {
					return "", fmt.Errorf("no tenant context available")
				}
				if tc.UserID == "" {
					return "", fmt.Errorf("no user ID linked to this API key (link with mcpfusion -user-link <user_id>:<key_hash>)")
				}
				return tc.UserID, nil
			}),
		)
		// Register knowledge service with the shared metrics collector.
		knowledgeToolCount := knowledgeProvider.ToolCount()
		sharedCollector.RegisterService("knowledge", global.TransportInternal, &knowledgeToolCount)
		providers = append(providers, knowledgeProvider)
	}

	// Perf provider (only when explicitly enabled via --perf or MCP_FUSION_PERF).
	if s.perfEnabled {
		configManager.RegisterNativeToolPrefix("perf")
		perfProvider := perf.New(perf.WithLogger(s.logger))
		providers = append(providers, perfProvider)
	}

	// Identify hub services and create hub provider
	var hubProvider *hub.HubProvider
	hubConfigs := make(map[string]*fusion.ServiceConfig)
	for name, svc := range configManager.Services() {
		if svc.IsHubService() {
			hubConfigs[name] = svc
		}
	}
	if len(hubConfigs) > 0 {
		s.logger.Infof("Found %d hub service(s) to connect", len(hubConfigs))
		hubOpts := []hub.HubOption{
			hub.WithLogger(s.logger),
			hub.WithSharedCollector(sharedCollector),
		}
		if dlDir := os.Getenv("MCP_FUSION_DL_DIR"); dlDir != "" {
			hubOpts = append(hubOpts, hub.WithDownloadDir(dlDir))
		}
		hubProvider = hub.NewHubProvider(hubConfigs, hubOpts...)
		providers = append(providers, hubProvider)
	}

	// Create MCP server, passing in the logger and tool providers
	// as well as setting other options
	mcpOpts := []mcpserver.Option{
		mcpserver.WithListen(s.listen),
		mcpserver.WithDebug(s.debug),
		mcpserver.WithLogger(s.logger),
		mcpserver.WithName(app.Name()),
		mcpserver.WithVersion(app.SemVer()),

		// Pass in the tool providers
		mcpserver.WithToolProviders(providers),
	}

	// Setup resource and prompt providers (only if fusionProvider is initialized)
	if fusionProvider != nil {
		mcpOpts = append(mcpOpts,
			mcpserver.WithResourceProviders([]global.ResourceProvider{fusionProvider}),
			mcpserver.WithPromptProviders([]global.PromptProvider{fusionProvider}),
		)
	}

	// Add OAuth API support components
	mcpOpts = append(mcpOpts, mcpserver.WithDatabase(s.database.(*db.DB)))
	mcpOpts = append(mcpOpts, mcpserver.WithAuthManager(multiTenantAuth))
	mcpOpts = append(mcpOpts, mcpserver.WithConfigManager(configManager))

	// The fusion engine serves the OAuth token-management HTTP API routes.
	// Only registered when a fusion provider was created (services/commands loaded).
	if fusionProvider != nil {
		mcpOpts = append(mcpOpts, mcpserver.WithOAuthEngine(fusionProvider))
	}

	// Add multi-tenant authentication middleware
	authMiddleware := mcpserver.NewAuthMiddleware(multiTenantAuth, configManager,
		mcpserver.WithAuthLogger(s.logger),
		mcpserver.WithRequireAuth(!s.noAuth),
		mcpserver.WithSkipPaths("/health", "/metrics", "/status", "/capabilities"),
	)
	mcpOpts = append(mcpOpts, mcpserver.WithAuthMiddleware(authMiddleware))
	if s.noAuth {
		s.logger.Warning("Multi-tenant authentication middleware in NO-AUTH mode (insecure)")
	} else {
		s.logger.Info("Multi-tenant authentication middleware enabled")
	}
	s.logger.Info("OAuth API endpoints will be available at /api/v1/oauth/*")

	mcp, err := mcpserver.New(mcpOpts...)
	if err != nil {
		s.release(nil, fusionProvider)
		return fmt.Errorf("unable to create MCP server: %w", err)
	}

	// Start hub provider after MCP server is created
	if hubProvider != nil {
		hubProvider.SetMCPServer(mcp.Server())
		hubProvider.Start(ctx)
	}

	// Start MCP server
	if err = mcp.Start(); err != nil {
		s.release(hubProvider, fusionProvider)
		return fmt.Errorf("MCP server failed to start: %w", err)
	}

	// Wait for termination
	<-ctx.Done()
	s.logger.Infof("Shutting down...")

	// Stop the MCP server. On failure the remaining shutdown still runs so the
	// providers and the database are closed; the error is returned at the end.
	stopErr := mcp.Stop()
	s.release(hubProvider, fusionProvider)

	if stopErr != nil {
		return fmt.Errorf("error stopping MCP server: %w", stopErr)
	}

	s.logger.Infof("MCP server stopped successfully")
	return nil
}

// release shuts down the providers run has started and closes the database, in
// reverse order of creation. A nil provider was not started and is skipped.
func (s server) release(hubProvider *hub.HubProvider, fusionProvider *fusion.Fusion) {
	if hubProvider != nil {
		hubProvider.Shutdown()
	}

	if fusionProvider != nil {
		fusionProvider.Shutdown()
	}

	if s.database != nil {
		if err := s.database.Close(); err != nil {
			s.logger.Errorf("Error closing database: %v", err)
		} else {
			s.logger.Info("Database connection closed successfully")
		}
	}
}
