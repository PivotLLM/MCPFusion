/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

// Package main demonstrates how to integrate the Fusion package with MCPFusion server
// for production deployments. This example shows comprehensive configuration including
// authentication, monitoring, error handling, and graceful shutdown.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/PivotLLM/MCPFusion/mcpserver"
	"github.com/PivotLLM/MCPFusion/mlogger"
)

// main demonstrates production-ready integration of Fusion with MCPFusion server
func main() {
	// Create structured logger for production
	logger, err := mlogger.New(
		mlogger.WithPrefix("FUSION"),
		mlogger.WithDateFormat("2006-01-02 15:04:05"),
		mlogger.WithLogFile("logs/mcpfusion.log"), // Log to file
		mlogger.WithLogStdout(true),
		mlogger.WithDebug(false), // Production log level
	)
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}
	defer logger.Close()

	logger.Info("Starting MCPFusion server with Fusion provider...")

	// Create production-ready Fusion provider
	fusionProvider := fusion.New(
		// Load multiple service configurations
		fusion.WithJSONConfig("configs/microsoft365.json"),
		fusion.WithJSONConfig("configs/google.json"),
		fusion.WithJSONConfig("configs/custom-apis.json"),

		// Configure production features
		fusion.WithLogger(logger),  // Structured logging
		fusion.WithInMemoryCache(), // Token and response caching
	)

	// Validate configuration at startup
	if err := fusionProvider.Validate(); err != nil {
		logger.Fatalf("Configuration validation failed: %v", err)
	}

	// Log configuration summary
	serviceNames := fusionProvider.GetServiceNames()
	logger.Infof("Configured services: %v", serviceNames)

	tools := fusionProvider.RegisterTools()
	logger.Infof("Generated %d API tools", len(tools))

	// Create MCP server with production configuration
	server, err := mcpserver.New(
		mcpserver.WithListen("localhost:8080"), // Server address
		mcpserver.WithLogger(logger),           // Use same logger
		mcpserver.WithDebug(false),             // Production mode
		mcpserver.WithName("MCPFusion"),        // Server name
		mcpserver.WithVersion("1.0.0"),         // Server version
		mcpserver.WithToolProviders([]global.ToolProvider{fusionProvider}),
	)
	if err != nil {
		logger.Fatalf("Failed to create MCP server: %v", err)
	}

	// Fusion provider is already registered via WithToolProviders

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		logger.Info("MCPFusion server starting...")
		logger.Infof("Server will be available at http://localhost:8080")

		// Log service names
		serviceNames := fusionProvider.GetServiceNames()
		logger.Infof("Configured services: %v", serviceNames)

		if err := server.Start(); err != nil {
			serverErrChan <- err
		}
	}()

	// Server is now running

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigChan:
		logger.Infof("Received signal %v, initiating graceful shutdown...", sig)
	case err := <-serverErrChan:
		logger.Errorf("Server error: %v", err)
		cancel()
	case <-ctx.Done():
		logger.Info("Context cancelled, shutting down...")
	}

	logger.Info("Shutting down server...")
	if err := server.Stop(); err != nil {
		logger.Errorf("Server shutdown error: %v", err)
	} else {
		logger.Info("Server shutdown completed successfully")
	}

	logger.Info("MCPFusion server stopped")
}

// Example environment setup for production
func init() {
	// Load environment from file if it exists
	if _, err := os.Stat(".env"); err == nil {
		// Note: In production, use proper environment loading
		log.Println("Loading environment from .env file...")
	}

	// Validate required environment variables
	requiredVars := []string{
		"MS365_CLIENT_ID",
		"MS365_TENANT_ID",
		"GOOGLE_CLIENT_ID",
		"GOOGLE_CLIENT_SECRET",
	}

	missing := []string{}
	for _, varName := range requiredVars {
		if os.Getenv(varName) == "" {
			missing = append(missing, varName)
		}
	}

	if len(missing) > 0 {
		log.Printf("Warning: Missing environment variables: %v", missing)
		log.Println("Some OAuth2 integrations may not work without proper configuration")
	}
}
