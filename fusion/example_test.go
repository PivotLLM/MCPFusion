// Copyright (c) 2025 Tenebris Technologies Inc.
// Please see LICENSE for details.

package fusion_test

import (
	"fmt"
	"log"
	"os"

	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/mlogger"
)

// ExampleNew demonstrates how to create a new Fusion instance
func ExampleNew() {
	// Create a logger
	logger, err := mlogger.New()
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Close()

	// Create a Fusion instance with default options
	fusionProvider := fusion.New(
		fusion.WithLogger(logger),
		fusion.WithInMemoryCache(),
	)

	fmt.Printf("Fusion provider created with %d configured services\n", len(fusionProvider.GetServiceNames()))
	// Output: Fusion provider created with 0 configured services
}

// ExampleNew_withConfig demonstrates loading configuration from JSON
func ExampleNew_withConfig() {
	// Create a simple configuration
	configJSON := `{
		"services": {
			"example": {
				"name": "Example API",
				"baseURL": "https://api.example.com",
				"auth": {
					"type": "bearer",
					"config": {
						"tokenEnvVar": "EXAMPLE_TOKEN"
					}
				},
				"endpoints": [
					{
						"id": "get_user",
						"name": "Get User",
						"description": "Get user information",
						"method": "GET",
						"path": "/users/{userId}",
						"parameters": [
							{
								"name": "userId",
								"description": "User ID",
								"type": "string",
								"required": true,
								"location": "path"
							}
						],
						"response": {
							"type": "json"
						}
					}
				]
			}
		}
	}`

	// Create Fusion instance with configuration
	fusionProvider := fusion.New(
		fusion.WithJSONConfigData([]byte(configJSON), "example-config.json"),
	)

	// Get tools that would be registered
	tools := fusionProvider.RegisterTools()

	fmt.Printf("Loaded %d services\n", len(fusionProvider.GetServiceNames()))
	fmt.Printf("Generated %d tools\n", len(tools))
	if len(tools) > 0 {
		fmt.Printf("First tool: %s\n", tools[0].Name)
	}

	// Output:
	// Loaded 1 services
	// Generated 1 tools
	// First tool: example_get_user
}

// ExampleFusion_GetSupportedAuthTypes demonstrates getting supported auth types
func ExampleFusion_GetSupportedAuthTypes() {
	fusionProvider := fusion.New()

	authTypes := fusionProvider.GetSupportedAuthTypes()
	fmt.Printf("Supported auth types: %d\n", len(authTypes))

	// The exact number may vary based on registered strategies
	// Output: Supported auth types: 4
}

// ExampleLoadConfigFromFile demonstrates loading configuration from a file
func ExampleLoadConfigFromFile() {
	// This example assumes you have a config file
	// In practice, you'd use a real file path
	configPath := "configs/microsoft365.json"

	// Check if the file exists (for example purposes)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("Config file %s not found (this is expected in tests)\n", configPath)
		return
	}

	config, err := fusion.LoadConfigFromFile(configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	fmt.Printf("Loaded configuration with %d services\n", len(config.Services))
}

// ExampleLoadConfigFromJSON_validation demonstrates configuration validation
func ExampleLoadConfigFromJSON_validation() {
	// Example of invalid configuration (missing required fields)
	invalidJSON := `{
		"services": {
			"invalid": {
				"name": "",
				"baseURL": "",
				"auth": {
					"type": "invalid_type",
					"config": {}
				},
				"endpoints": []
			}
		}
	}`

	_, err := fusion.LoadConfigFromJSON([]byte(invalidJSON), "invalid-config.json")
	if err != nil {
		fmt.Printf("Configuration validation failed (as expected): %v\n", err)
	}

	// This will output an error message about validation failure
}