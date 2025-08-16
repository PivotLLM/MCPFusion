/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
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

	// Token management subcommands
	tokenAddFlag := flag.String("token-add", "", "Add new API token with description")
	tokenListFlag := flag.Bool("token-list", false, "List all API tokens")
	tokenDeleteFlag := flag.String("token-del", "", "Delete API token by prefix or hash")

	// Set custom usage message
	flag.Usage = func() {
		fmt.Printf("MCPFusion - Multi-Tenant Model Context Protocol Server\n\n")
		fmt.Printf("Usage:\n")
		fmt.Printf("  %s [options]\n\n", os.Args[0])
		fmt.Printf("Server Options:\n")
		fmt.Printf("  -config string\n")
		fmt.Printf("        Path to fusion configuration file (optional)\n")
		fmt.Printf("  -debug\n")
		fmt.Printf("        Enable debug mode (default true)\n")
		fmt.Printf("  -help\n")
		fmt.Printf("        Show help information\n")
		fmt.Printf("  -no-streaming\n")
		fmt.Printf("        Disable streaming (use plain HTTP instead of SSE)\n")
		fmt.Printf("  -port int\n")
		fmt.Printf("        Port to listen on (default 8888)\n")
		fmt.Printf("  -version\n")
		fmt.Printf("        Show version information\n\n")
		fmt.Printf("Token Management Commands:\n")
		fmt.Printf("  -token-add string\n")
		fmt.Printf("        Add new API token with description\n")
		fmt.Printf("  -token-list\n")
		fmt.Printf("        List all API tokens\n")
		fmt.Printf("  -token-del string\n")
		fmt.Printf("        Delete API token by prefix or hash\n\n")
		fmt.Printf("Environment Variables:\n")
		fmt.Printf("  MCP_FUSION_DB_DIR   Custom database directory (default: /opt/mcpfusion or ~/.mcpfusion)\n\n")
		fmt.Printf("Examples:\n")
		fmt.Printf("  # Start server with configuration\n")
		fmt.Printf("  %s -config configs/microsoft365.json -port 8888\n\n", os.Args[0])
		fmt.Printf("  # Token management examples\n")
		fmt.Printf("  %s -token-add \"Production token\"\n", os.Args[0])
		fmt.Printf("  %s -token-list\n", os.Args[0])
		fmt.Printf("  %s -token-del abc12345\n\n", os.Args[0])
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
		mlogger.WithPrefix("MCPFusion"),
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

	// Initialize database
	logger.Info("Initializing database")

	// Database configuration
	dbDataDir := os.Getenv("MCP_FUSION_DB_DIR")
	dbOpts := []db.Option{
		db.WithLogger(logger),
	}
	if dbDataDir != "" {
		dbOpts = append(dbOpts, db.WithDataDir(dbDataDir))
	}

	// Initialize database (required)
	database, err := db.New(dbOpts...)
	if err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}
	logger.Info("Database initialized successfully")

	// Handle token management commands if specified
	if *tokenAddFlag != "" || *tokenListFlag || *tokenDeleteFlag != "" {
		if err := handleTokenCommands(database, *tokenAddFlag, *tokenListFlag, *tokenDeleteFlag, logger); err != nil {
			logger.Fatalf("Token management failed: %v", err)
		}
		// Exit after token management - don't start server
		os.Exit(0)
	}

	// Initialize database-backed cache
	dbCache := fusion.NewDatabaseCache(database.(*db.DB), logger)

	// Create multi-tenant authentication manager
	multiTenantAuth := fusion.NewMultiTenantAuthManager(database.(*db.DB), dbCache, logger)

	// Register authentication strategies
	oauthStrategy := fusion.NewOAuth2DeviceFlowStrategy(
		&http.Client{Timeout: 30 * time.Second}, logger)
	multiTenantAuth.RegisterStrategy(oauthStrategy)

	// Register other auth strategies
	bearerStrategy := fusion.NewBearerTokenStrategy(logger)
	multiTenantAuth.RegisterStrategy(bearerStrategy)

	apiKeyStrategy := fusion.NewAPIKeyStrategy(logger)
	multiTenantAuth.RegisterStrategy(apiKeyStrategy)

	basicStrategy := fusion.NewBasicAuthStrategy(logger)
	multiTenantAuth.RegisterStrategy(basicStrategy)

	// Initialize service resolver
	serviceResolver := fusion.NewServiceConfigResolver(
		fusion.WithSRLogger(logger),
		fusion.WithAutoReload(5*time.Minute),
	)

	logger.Info("Multi-tenant authentication system initialized")

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

	// Add multi-tenant authentication middleware (always enabled)
	authMiddleware := mcpserver.NewAuthMiddleware(multiTenantAuth, serviceResolver,
		mcpserver.WithAuthLogger(logger),
		mcpserver.WithRequireAuth(true),
		mcpserver.WithSkipPaths("/health", "/metrics", "/status", "/capabilities"),
	)
	mcpOpts = append(mcpOpts, mcpserver.WithAuthMiddleware(authMiddleware))
	logger.Info("Multi-tenant authentication middleware enabled")

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

// handleTokenCommands processes token management commands
func handleTokenCommands(database db.Database, tokenAdd string, tokenList bool, tokenDelete string, logger global.Logger) error {
	if tokenAdd != "" {
		return handleTokenAdd(database, tokenAdd, logger)
	}

	if tokenList {
		return handleTokenList(database, logger)
	}

	if tokenDelete != "" {
		return handleTokenDelete(database, tokenDelete, logger)
	}

	return nil
}

// handleTokenAdd creates a new API token
func handleTokenAdd(database db.Database, description string, logger global.Logger) error {
	if description == "" {
		description = "API Token"
	}

	// Validate description length
	if len(description) > 255 {
		return fmt.Errorf("description too long (max 255 characters)")
	}

	fmt.Printf("Generating new API token...\n")

	token, hash, err := database.AddAPIToken(description)
	if err != nil {
		return fmt.Errorf("failed to create API token: %w", err)
	}

	// Show the token only once with security warning
	fmt.Printf("\n")
	fmt.Printf("API Token created successfully\n")
	fmt.Printf("\n")
	fmt.Printf("SECURITY WARNING: This token will only be displayed once!\n")
	fmt.Printf("   Copy it now and store it securely.\n")
	fmt.Printf("\n")
	fmt.Printf("Token:       %s\n", token)
	fmt.Printf("Hash:        %s\n", hash[:12])
	fmt.Printf("Description: %s\n", description)
	fmt.Printf("\n")
	fmt.Printf("Use this token in the Authorization header:\n")
	fmt.Printf("  Authorization: Bearer %s\n", token)
	fmt.Printf("\n")

	return nil
}

// handleTokenList displays all API tokens
func handleTokenList(database db.Database, logger global.Logger) error {
	tokens, err := database.ListAPITokens()
	if err != nil {
		return fmt.Errorf("failed to list API tokens: %w", err)
	}

	if len(tokens) == 0 {
		fmt.Printf("No API tokens found.\n")
		fmt.Printf("Create one with: %s -token-add \"Description\"\n", os.Args[0])
		return nil
	}

	fmt.Printf("API Tokens:\n")
	fmt.Printf("%-10s %-20s %-20s %-20s %s\n", "PREFIX", "HASH", "CREATED", "LAST USED", "DESCRIPTION")
	fmt.Printf("%-10s %-20s %-20s %-20s %s\n", "------", "----", "-------", "---------", "-----------")

	for _, token := range tokens {
		prefix := token.Hash[:8]
		shortHash := token.Hash[:12]

		createdAt := token.CreatedAt.Format("2006-01-02 15:04:05")

		lastUsed := "Never used"
		if !token.LastUsed.IsZero() {
			lastUsed = token.LastUsed.Format("2006-01-02 15:04:05")
		}

		description := token.Description
		if len(description) > 30 {
			description = description[:27] + "..."
		}

		fmt.Printf("%-10s %-20s %-20s %-20s %s\n", prefix, shortHash, createdAt, lastUsed, description)
	}

	fmt.Printf("\nTotal: %d tokens\n", len(tokens))
	return nil
}

// handleTokenDelete removes an API token
func handleTokenDelete(database db.Database, identifier string, logger global.Logger) error {
	if identifier == "" {
		return fmt.Errorf("token identifier is required")
	}

	// List tokens to find matching one
	tokens, err := database.ListAPITokens()
	if err != nil {
		return fmt.Errorf("failed to list API tokens: %w", err)
	}

	var matchedToken *db.APITokenMetadata
	for _, token := range tokens {
		if token.Hash == identifier || strings.HasPrefix(token.Hash, identifier) {
			if matchedToken != nil {
				return fmt.Errorf("multiple tokens match '%s'. Please use a longer prefix", identifier)
			}
			matchedToken = &token
		}
	}

	if matchedToken == nil {
		return fmt.Errorf("no API token found matching '%s'", identifier)
	}

	// Show token details and confirm deletion
	fmt.Printf("Token Details:\n")
	fmt.Printf("  Hash: %s\n", matchedToken.Hash[:12])
	fmt.Printf("  Description: %s\n", matchedToken.Description)
	fmt.Printf("  Created: %s\n", matchedToken.CreatedAt.Format("2006-01-02 15:04:05"))

	fmt.Printf("Are you sure you want to delete this token? (y/N): ")
	var response string
	fmt.Scanln(&response)

	if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
		fmt.Printf("Token deletion cancelled.\n")
		return nil
	}

	if err := database.DeleteAPIToken(matchedToken.Hash); err != nil {
		return fmt.Errorf("failed to delete API token: %w", err)
	}

	fmt.Printf("Token deleted successfully.\n")
	return nil
}
