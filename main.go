/*=============================================================================
= Copyright (c) 2025 Tenebris Technologies Inc.                              =
= All rights reserved.                                                       =
=============================================================================*/

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/PivotLLM/MCPFusion/mcpserver"
	"github.com/PivotLLM/MCPFusion/mlogger"
)

// Version information
const (
	AppName    = "MCPFusion"
	AppVersion = "0.0.2"
)

func main() {
	var err error
	var listen string

	// Define command line flags
	debugFlag := flag.Bool("debug", true, "Enable debug mode")
	portFlag := flag.Int("port", 8888, "Port to listen on")
	noStreamingFlag := flag.Bool("no-streaming", false, "Disable streaming (use plain HTTP instead of SSE)")
	configFlag := flag.String("config", "", "Path to fusion configuration file (optional)")
	helpFlag := flag.Bool("help", false, "Show help information")
	versionFlag := flag.Bool("version", false, "Show version information")

	// Set custom usage message
	flag.Usage = func() {
		fmt.Printf("Usage of %s:\n", os.Args[0])
		fmt.Printf("  %s [options]\n\n", os.Args[0])
		fmt.Printf("Options:\n")
		flag.PrintDefaults()
	}

	// Parse command line flags
	flag.Parse()

	// Show help and exit if requested
	if *helpFlag {
		flag.Usage()
		os.Exit(0)
	}

	// Show version and exit if requested
	if *versionFlag {
		fmt.Printf("%s version %s\n", AppName, AppVersion)
		os.Exit(0)
	}

	// Use the flag values
	debug := *debugFlag
	noStreaming := *noStreamingFlag

	// Create a temporary logger for early logging (before env vars are loaded)
	tempLogger, err := mlogger.New(
		mlogger.WithPrefix("MCP"),
		mlogger.WithDateFormat("2006-01-02 15:04:05"),
		mlogger.WithLogFile("mcp.log"),
		mlogger.WithLogStdout(true),
		mlogger.WithDebug(debug),
	)
	if err != nil {
		fmt.Printf("Unable to create logger: %v", err)
		os.Exit(1)
	}

	// Load environment variables from config files in priority order:
	// 1. /opt/mcpfusion/env
	// 2. ~/.mcpfusion
	// 3. ~/.mcp (for backwards compatibility)
	envFiles := []string{
		"/opt/mcpfusion/env",
	}

	// Add user-specific config files if home directory is available
	homeDir, err := os.UserHomeDir()
	if err == nil {
		envFiles = append(envFiles,
			homeDir+string(os.PathSeparator)+".mcpfusion",
			homeDir+string(os.PathSeparator)+".mcp",
		)
	}

	// Try to load each config file in order
	for _, envFile := range envFiles {
		if _, err := os.Stat(envFile); err == nil {
			err = godotenv.Load(envFile)
			if err == nil {
				tempLogger.Infof("Loaded environment variables from %s", envFile)
				break // Stop after loading the first successful file
			}
		}
	}

	// Now that env files are loaded, check for fusion config
	fusionConfig := *configFlag

	// Check for MCP_FUSION_CONFIG environment variable if no config flag was provided
	if fusionConfig == "" {
		if envConfig := os.Getenv("MCP_FUSION_CONFIG"); envConfig != "" {
			fusionConfig = envConfig
			tempLogger.Infof("Using fusion config from MCP_FUSION_CONFIG: %s", envConfig)
		}
	}

	// Determine listen address from environment or flag
	if envListen := os.Getenv("MCP_FUSION_LISTEN"); envListen != "" {
		listen = envListen
		tempLogger.Infof("Using listen address from MCP_FUSION_LISTEN: %s", envListen)
	} else if *portFlag > 0 && *portFlag < 65536 {
		listen = fmt.Sprintf("localhost:%d", *portFlag)
	} else {
		listen = "localhost:8888"
	}

	// Use the temporary logger as the main logger
	logger := tempLogger

	// Initialize database if configuration is available
	var database db.Database
	var multiTenantAuth *fusion.MultiTenantAuthManager
	var serviceResolver *fusion.ServiceConfigResolver
	
	// Check for database configuration
	dbDataDir := os.Getenv("MCP_DB_DATA_DIR")
	enableDatabase := os.Getenv("MCP_ENABLE_DATABASE") == "true"
	
	if enableDatabase || dbDataDir != "" {
		logger.Info("Initializing database for multi-tenant support")
		
		// Create database options
		dbOpts := []db.Option{
			db.WithLogger(logger),
		}
		if dbDataDir != "" {
			dbOpts = append(dbOpts, db.WithDataDir(dbDataDir))
		}
		
		// Initialize database
		var err error
		database, err = db.New(dbOpts...)
		if err != nil {
			logger.Errorf("Failed to initialize database: %v", err)
			logger.Warning("Continuing without database - multi-tenant features disabled")
		} else {
			logger.Info("Database initialized successfully")
			
			// Initialize database-backed cache
			dbCache := fusion.NewDatabaseCache(database.(*db.DB), logger)
			
			// Create multi-tenant authentication manager
			multiTenantAuth = fusion.NewMultiTenantAuthManager(database.(*db.DB), dbCache, logger)
			
			// Register authentication strategies
			oauthStrategy := fusion.NewOAuth2DeviceFlowStrategy(
				&http.Client{Timeout: 30 * time.Second}, logger)
			multiTenantAuth.RegisterStrategy(oauthStrategy)
			
			// Initialize service resolver
			serviceResolver = fusion.NewServiceConfigResolver(
				fusion.WithSRLogger(logger),
				fusion.WithAutoReload(5*time.Minute),
			)
			
			logger.Info("Multi-tenant authentication system initialized")
		}
	} else {
		logger.Info("Database not configured - running in single-tenant mode")
	}

	// API authentication configuration (legacy support)
	APIAuthHeader := os.Getenv("API_AUTH_HEADER")
	if APIAuthHeader == "" {
		APIAuthHeader = "X-API-Key"
		if database == nil {
			logger.Warningf("API_AUTH_HEADER environment variable is not set, defaulting to %s", APIAuthHeader)
		}
	}

	APIAuthKey := os.Getenv("API_AUTH_KEY")
	if APIAuthKey == "" {
		APIAuthKey = "1234567890ABCDEFGHIJKLMONPQRSTUVWXYZ"
		if database == nil {
			logger.Warningf("API_AUTH_KEY environment variable is not set, defaulting to %s", APIAuthKey)
		}
	}
	
	// Multi-tenant API token support
	bearerTokensEnabled := os.Getenv("MCP_ENABLE_BEARER_TOKENS") == "true"
	if bearerTokensEnabled && database != nil {
		logger.Info("Bearer token authentication enabled for multi-tenant access")
	}

	// Create a slice (list) of tool providers
	providers := []global.ToolProvider{}

	// Add fusion provider if configuration is provided
	var fusionProvider *fusion.Fusion
	if fusionConfig != "" {
		logger.Infof("Loading fusion provider with config file: %s", fusionConfig)
		
		// Configure fusion provider based on whether multi-tenant mode is enabled
		fusionOpts := []fusion.Option{
			fusion.WithLogger(logger),
			fusion.WithJSONConfig(fusionConfig),
		}
		
		// Add multi-tenant support if available
		if multiTenantAuth != nil {
			fusionOpts = append(fusionOpts, fusion.WithMultiTenantAuth(multiTenantAuth))
		}
		
		fusionProvider = fusion.New(fusionOpts...)
		providers = append(providers, fusionProvider)
	}

	// Create MCP server, passing in the logger and tool providers
	// as well as setting other options
	mcpOpts := []mcpserver.Option{
		mcpserver.WithListen(listen),
		mcpserver.WithDebug(debug),
		mcpserver.WithLogger(logger),
		mcpserver.WithName(AppName),
		mcpserver.WithVersion(AppVersion),
		mcpserver.WithNoStreaming(noStreaming),

		// Pass in the tool providers
		mcpserver.WithToolProviders(providers),

		// Setup resource and prompt providers
		mcpserver.WithResourceProviders([]global.ResourceProvider{fusionProvider}),
		mcpserver.WithPromptProviders([]global.PromptProvider{fusionProvider}),
	}
	
	// Add multi-tenant authentication middleware if enabled
	if bearerTokensEnabled && multiTenantAuth != nil && serviceResolver != nil {
		authMiddleware := mcpserver.NewAuthMiddleware(multiTenantAuth, serviceResolver,
			mcpserver.WithAuthLogger(logger),
			mcpserver.WithRequireAuth(true),
			mcpserver.WithSkipPaths("/health", "/metrics", "/status", "/capabilities"),
		)
		mcpOpts = append(mcpOpts, mcpserver.WithAuthMiddleware(authMiddleware))
		logger.Info("Multi-tenant authentication middleware enabled")
	}
	
	mcp, err := mcpserver.New(mcpOpts...)
	if err != nil {
		logger.Fatalf("Unable to create MCP server: %v", err)
		os.Exit(1)
	}

	// Start MCP server
	if err = mcp.Start(); err != nil {
		logger.Fatalf("MCP server failed to start: %v", err)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for termination signal
	<-sigChan
	logger.Infof("Shutting down...")

	// Stop the MCP server
	if err = mcp.Stop(); err != nil {
		logger.Errorf("Error stopping MCP server: %s", err.Error())
		os.Exit(1)
	}

	// Close database connection if initialized
	if database != nil {
		if err := database.Close(); err != nil {
			logger.Errorf("Error closing database: %v", err)
		} else {
			logger.Info("Database connection closed successfully")
		}
	}

	logger.Infof("MCP server stopped successfully")

	// Exit with success
	os.Exit(0)
}
