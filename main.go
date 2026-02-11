/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/PivotLLM/MCPFusion/config"
	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/PivotLLM/MCPFusion/mcpserver"
	"github.com/PivotLLM/MCPFusion/mlogger"
)

func main() {
	var err error
	var listen string

	// Define command line flags
	debugFlag := flag.Bool("debug", true, "Enable debug mode")
	portFlag := flag.Int("port", 8888, "Port to listen on")
	noAuthFlag := flag.Bool("no-auth", false, "Disable authentication (INSECURE - testing only)")
	configFlag := flag.String("config", "", "Comma-separated list of configuration files (optional)")
	helpFlag := flag.Bool("help", false, "Show help information")
	versionFlag := flag.Bool("version", false, "Show version information")

	// Token management subcommands
	tokenAddFlag := flag.String("token-add", "", "Add new API token with description")
	tokenListFlag := flag.Bool("token-list", false, "List all API tokens")
	tokenDeleteFlag := flag.String("token-del", "", "Delete API token by prefix or hash")

	// Auth code generation
	authCodeFlag := flag.String("auth-code", "", "Generate auth code for a service (e.g., google)")
	authURLFlag := flag.String("auth-url", "", "External URL of this server (required with -auth-code)")
	authTokenFlag := flag.String("auth-token", "", "API token prefix/hash to identify tenant (for multi-token setups)")

	// Set custom usage message
	flag.Usage = func() {
		fmt.Printf("MCPFusion - Multi-Tenant Model Context Protocol Server\n\n")
		fmt.Printf("Usage:\n")
		fmt.Printf("  %s [options]\n\n", os.Args[0])
		fmt.Printf("Server Options:\n")
		fmt.Printf("  -config string\n")
		fmt.Printf("        Comma-separated list of configuration files (optional)\n")
		fmt.Printf("        Can also use MCP_FUSION_CONFIGS environment variable\n")
		fmt.Printf("  -debug\n")
		fmt.Printf("        Enable debug mode (default true)\n")
		fmt.Printf("  -help\n")
		fmt.Printf("        Show help information\n")
		fmt.Printf("  -no-auth\n")
		fmt.Printf("        Disable authentication (INSECURE - testing only)\n")
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
		fmt.Printf("Auth Code Commands:\n")
		fmt.Printf("  -auth-code string\n")
		fmt.Printf("        Generate auth code for a service (e.g., google)\n")
		fmt.Printf("  -auth-url string\n")
		fmt.Printf("        External URL of this server (required with -auth-code)\n")
		fmt.Printf("  -auth-token string\n")
		fmt.Printf("        API token prefix/hash to identify tenant (for multi-token setups)\n\n")
		fmt.Printf("Environment Variables:\n")
		fmt.Printf("  MCP_FUSION_DB_DIR   Custom database directory (default: /opt/mcpfusion or ~/.mcpfusion)\n\n")
		fmt.Printf("Examples:\n")
		fmt.Printf("  # Start server with configuration\n")
		fmt.Printf("  %s -config configs/microsoft365.json -port 8888\n\n", os.Args[0])
		fmt.Printf("  # Token management examples\n")
		fmt.Printf("  %s -token-add \"Production token\"\n", os.Args[0])
		fmt.Printf("  %s -token-list\n", os.Args[0])
		fmt.Printf("  %s -token-del abc12345\n\n", os.Args[0])
		fmt.Printf("  # Generate auth code for fusion-auth\n")
		fmt.Printf("  %s -auth-code google -auth-url http://10.0.0.1:8888\n\n", os.Args[0])
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
		fmt.Printf("%s version %s\n", global.AppName, global.AppVersion)
		os.Exit(0)
	}

	// Use the flag values
	debug := *debugFlag
	noAuth := *noAuthFlag

	// Load environment variables from config files in priority order:
	// 1. /opt/mcpfusion/env
	// 2. ~/.mcpfusion
	envFiles := []string{
		"/opt/mcpfusion/env",
	}

	// Add user-specific config files if home directory is available
	homeDir, err := os.UserHomeDir()
	if err == nil {
		envFiles = append(envFiles, homeDir+string(os.PathSeparator)+".mcpfusion")
	}

	// Track which environment file was loaded
	var loadedEnvFile string

	// Try to load each config file in order
	for _, envFile := range envFiles {
		if _, err := os.Stat(envFile); err == nil {
			err = godotenv.Load(envFile)
			if err == nil {
				// Stop after loading the first successful file. Note that logger is not configured yet.
				loadedEnvFile = envFile
				break
			}
		}
	}

	// My default log in the current directory
	logfile := "mcpfusion.log"

	// If MCP_FUSION_LOGFILE is set, use it instead
	value, exists := os.LookupEnv("MCP_FUSION_LOGFILE")
	if exists {
		// Environment variable is set (could be empty)
		logfile = value
	}

	// Create the logger
	logger, err := mlogger.New(
		mlogger.WithPrefix("MCPFusion"),
		mlogger.WithDateFormat("2006-01-02 15:04:05"),
		mlogger.WithLogFile(logfile),
		mlogger.WithLogStdout(true),
		mlogger.WithDebug(debug),
	)
	if err != nil {
		fmt.Printf("Unable to create logger: %v", err)
		os.Exit(1)
	}

	// Log startup banner
	logger.Infof("Starting %s v%s", global.AppName, global.AppVersion)

	// Log warning if no-auth mode is enabled
	if noAuth {
		logger.Warning("**************************************************************")
		logger.Warning("* SECURITY WARNING: Authentication is DISABLED              *")
		logger.Warning("* This mode is INSECURE and should ONLY be used for testing *")
		logger.Warning("* All requests will use the 'NOAUTH' tenant context         *")
		logger.Warning("**************************************************************")
	}

	// Log environment file loading status
	if loadedEnvFile != "" {
		logger.Infof("Loaded environment from: %s", loadedEnvFile)
	} else {
		logger.Debug("No environment file loaded (searched: /opt/mcpfusion/env, ~/.mcpfusion, ~/.mcp)")
	}

	// Now that env files are loaded, check for fusion configs
	configFiles := getConfigFiles(*configFlag, logger)

	// Determine listen address from environment or flag
	if envListen := os.Getenv("MCP_FUSION_LISTEN"); envListen != "" {
		listen = envListen
		logger.Infof("Using listen address from MCP_FUSION_LISTEN: %s", envListen)
	} else if *portFlag > 0 && *portFlag < 65536 {
		listen = fmt.Sprintf("localhost:%d", *portFlag)
	} else {
		listen = "localhost:8888"
	}

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

	// Handle auth code generation if specified
	if *authCodeFlag != "" {
		if err := handleAuthCode(database, *authCodeFlag, *authURLFlag, *authTokenFlag, logger); err != nil {
			logger.Fatalf("Auth code generation failed: %v", err)
		}
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

	sessionJWTStrategy := fusion.NewSessionJWTStrategy(
		&http.Client{Timeout: 30 * time.Second}, logger)
	multiTenantAuth.RegisterStrategy(sessionJWTStrategy)

	oauth2ExternalStrategy := fusion.NewOAuth2ExternalStrategy(
		&http.Client{Timeout: 30 * time.Second}, logger)
	multiTenantAuth.RegisterStrategy(oauth2ExternalStrategy)

	// Initialize config manager with all configuration files
	configManager := config.New(
		config.WithLogger(logger),
		config.WithConfigFiles(configFiles...),
	)

	// Load all configurations
	if err := configManager.LoadConfigs(); err != nil {
		logger.Errorf("Failed to load configurations: %v", err)
		// Continue anyway - server can run without configs
	}

	// Log what was loaded
	serviceCount := configManager.ServiceCount()
	commandCount := configManager.CommandCount()

	if serviceCount > 0 || commandCount > 0 {
		if serviceCount > 0 && commandCount > 0 {
			logger.Infof("Loaded %d services and %d command groups from configuration files",
				serviceCount, commandCount)
		} else if serviceCount > 0 {
			logger.Infof("Loaded %d services from configuration files", serviceCount)
		} else {
			logger.Infof("Loaded %d command groups from configuration files", commandCount)
		}
	} else {
		logger.Warning("No services or commands loaded from configuration files")
	}

	logger.Info("Multi-tenant authentication system initialized")

	// Create a slice (list) of tool providers
	var providers []global.ToolProvider

	// Add fusion provider if configurations were loaded
	var fusionProvider *fusion.Fusion
	if serviceCount > 0 || commandCount > 0 {
		logger.Infof("Creating fusion provider with %d services and %d command groups",
			serviceCount, commandCount)

		// Configure fusion provider with config manager
		fusionOpts := []fusion.Option{
			fusion.WithLogger(logger),
			fusion.WithConfigManager(configManager),
		}

		// Set external URL for auth setup tools
		if externalURL := os.Getenv("MCP_FUSION_EXTERNAL_URL"); externalURL != "" {
			fusionOpts = append(fusionOpts, fusion.WithExternalURL(externalURL))
			logger.Infof("External URL for auth setup: %s", externalURL)
		} else {
			fusionOpts = append(fusionOpts, fusion.WithExternalURL("http://"+listen))
			logger.Warningf("MCP_FUSION_EXTERNAL_URL not set, using http://%s (may not be reachable externally)", listen)
		}

		// Add multi-tenant support if available
		if multiTenantAuth != nil {
			fusionOpts = append(fusionOpts, fusion.WithMultiTenantAuth(multiTenantAuth))
		}

		fusionProvider = fusion.New(fusionOpts...)
		providers = append(providers, fusionProvider)
	} else {
		logger.Warning("No fusion provider created - no configurations loaded")
	}

	// Create MCP server, passing in the logger and tool providers
	// as well as setting other options
	mcpOpts := []mcpserver.Option{
		mcpserver.WithListen(listen),
		mcpserver.WithDebug(debug),
		mcpserver.WithLogger(logger),
		mcpserver.WithName(global.AppName),
		mcpserver.WithVersion(global.AppVersion),

		// Pass in the tool providers
		mcpserver.WithToolProviders(providers),

		// Setup resource and prompt providers
		mcpserver.WithResourceProviders([]global.ResourceProvider{fusionProvider}),
		mcpserver.WithPromptProviders([]global.PromptProvider{fusionProvider}),
	}

	// Add OAuth API support components
	mcpOpts = append(mcpOpts, mcpserver.WithDatabase(database.(*db.DB)))
	mcpOpts = append(mcpOpts, mcpserver.WithAuthManager(multiTenantAuth))
	mcpOpts = append(mcpOpts, mcpserver.WithConfigManager(configManager))

	// Add multi-tenant authentication middleware
	authMiddleware := mcpserver.NewAuthMiddleware(multiTenantAuth, configManager,
		mcpserver.WithAuthLogger(logger),
		mcpserver.WithRequireAuth(!noAuth),
		mcpserver.WithSkipPaths("/health", "/metrics", "/status", "/capabilities"),
	)
	mcpOpts = append(mcpOpts, mcpserver.WithAuthMiddleware(authMiddleware))
	if noAuth {
		logger.Warning("Multi-tenant authentication middleware in NO-AUTH mode (insecure)")
	} else {
		logger.Info("Multi-tenant authentication middleware enabled")
	}
	logger.Info("OAuth API endpoints will be available at /api/v1/oauth/*")

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

	// Shutdown Fusion provider if initialized
	if fusionProvider != nil {
		fusionProvider.Shutdown()
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
func handleTokenAdd(database db.Database, description string, _ global.Logger) error {
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
func handleTokenList(database db.Database, _ global.Logger) error {
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
func handleTokenDelete(database db.Database, identifier string, _ global.Logger) error {
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
	_, err = fmt.Scanln(&response)
	if err != nil {
		return err
	}

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

// handleAuthCode generates an auth code for use with fusion-auth
func handleAuthCode(database db.Database, service, authURL, authToken string, logger global.Logger) error {
	if authURL == "" {
		return fmt.Errorf("-auth-url is required with -auth-code")
	}

	// Resolve the tenant hash from API tokens
	tokens, err := database.ListAPITokens()
	if err != nil {
		return fmt.Errorf("failed to list API tokens: %w", err)
	}

	if len(tokens) == 0 {
		return fmt.Errorf("no API tokens found. Create one with: %s -token-add \"Description\"", os.Args[0])
	}

	var tenantHash string
	if len(tokens) == 1 {
		tenantHash = tokens[0].Hash
	} else {
		// Multiple tokens — require -auth-token to disambiguate
		if authToken == "" {
			return fmt.Errorf("multiple API tokens found. Use -auth-token to specify which token's tenant to use")
		}
		resolvedHash, err := database.ResolveAPIToken(authToken)
		if err != nil {
			return fmt.Errorf("failed to resolve API token '%s': %w", authToken, err)
		}
		tenantHash = resolvedHash
	}

	// Create the auth code with 15-minute TTL
	code, err := database.CreateAuthCode(tenantHash, service, 15*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to create auth code: %w", err)
	}

	// Build the blob
	blob := fusion.AuthCodeBlob{
		URL:     authURL,
		Code:    code,
		Service: service,
	}

	blobJSON, err := json.Marshal(blob)
	if err != nil {
		return fmt.Errorf("failed to marshal auth code blob: %w", err)
	}

	encoded := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(blobJSON)

	fmt.Printf("\nAuth code generated successfully\n")
	fmt.Printf("\n")
	fmt.Printf("Service:  %s\n", service)
	fmt.Printf("Server:   %s\n", authURL)
	fmt.Printf("Expires:  15 minutes\n")
	fmt.Printf("\n")
	fmt.Printf("Run fusion-auth with:\n")
	fmt.Printf("  ./fusion-auth %s\n", encoded)
	fmt.Printf("\n")

	logger.Infof("Generated auth code for service %s (tenant %s)", service, tenantHash[:12])
	return nil
}

// getConfigFiles parses comma-separated config files from command line or environment
func getConfigFiles(configFlag string, logger global.Logger) []string {
	configPaths := configFlag

	// If not provided via command line, check environment variables
	if configPaths == "" {
		// Check new environment variable first
		configPaths = os.Getenv("MCP_FUSION_CONFIGS")
		if configPaths != "" && logger != nil {
			logger.Infof("Using config files from MCP_FUSION_CONFIGS: %s", configPaths)
		}
	}

	// Fall back to old single config environment variable for backward compatibility
	if configPaths == "" {
		configPaths = os.Getenv("MCP_FUSION_CONFIG")
		if configPaths != "" && logger != nil {
			logger.Infof("Using config file from MCP_FUSION_CONFIG: %s", configPaths)
		}
	}

	// If still empty, return empty list (no configs)
	if configPaths == "" {
		return []string{}
	}

	// Split comma-separated list and trim whitespace
	files := strings.Split(configPaths, ",")
	cleanFiles := make([]string, 0, len(files))

	for _, file := range files {
		trimmed := strings.TrimSpace(file)
		if trimmed != "" {
			cleanFiles = append(cleanFiles, trimmed)
		}
	}

	if logger != nil && len(cleanFiles) > 0 {
		logger.Infof("Found %d configuration file(s) to load", len(cleanFiles))
	}

	return cleanFiles
}
