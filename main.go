/*=============================================================================
= Copyright (c) 2025 Tenebris Technologies Inc.                              =
= All rights reserved.                                                       =
=============================================================================*/

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

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

	APIAuthHeader := os.Getenv("API_AUTH_HEADER")
	if APIAuthHeader == "" {
		APIAuthHeader = "X-API-Key"
		logger.Warningf("API_AUTH_HEADER environment variable is not set, defaulting to %s", APIAuthHeader)
	}

	APIAuthKey := os.Getenv("API_AUTH_KEY")
	if APIAuthKey == "" {
		APIAuthKey = "1234567890ABCDEFGHIJKLMONPQRSTUVWXYZ"
		logger.Warningf("API_AUTH_KEY environment variable is not set, defaulting to %s", APIAuthKey)
	}

	// Create a slice (list) of tool providers
	providers := []global.ToolProvider{}

	// Add fusion provider if configuration is provided
	var fusionProvider *fusion.Fusion
	if fusionConfig != "" {
		logger.Infof("Loading fusion provider with config file: %s", fusionConfig)
		fusionProvider = fusion.New(
			fusion.WithLogger(logger),
			fusion.WithJSONConfig(fusionConfig),
		)
		providers = append(providers, fusionProvider)
	}

	// Create MCP server, passing in the logger and tool providers
	// as well as setting other options
	mcp, err := mcpserver.New(
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
	)
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

	logger.Infof("MCP server stopped successfully")

	// Exit with success
	os.Exit(0)
}
