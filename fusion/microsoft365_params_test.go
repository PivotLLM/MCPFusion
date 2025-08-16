/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"strings"
	"testing"
)

func TestMicrosoft365ParameterAliases(t *testing.T) {
	// Load the Microsoft365 configuration
	config, err := LoadConfigFromFile("../configs/microsoft365.json")
	if err != nil {
		t.Skipf("Skipping test - could not load microsoft365.json: %v", err)
	}

	// Check that all $ parameters have aliases
	for serviceName, service := range config.Services {
		for _, endpoint := range service.Endpoints {
			for _, param := range endpoint.Parameters {
				if strings.HasPrefix(param.Name, "$") {
					if param.Alias == "" {
						t.Errorf("Parameter '%s' in endpoint %s.%s has no alias", 
							param.Name, serviceName, endpoint.ID)
					} else {
						// Verify the alias is MCP-compliant
						if !IsValidMCPParameterName(param.Alias) {
							t.Errorf("Alias '%s' for parameter '%s' in %s.%s is not MCP-compliant",
								param.Alias, param.Name, serviceName, endpoint.ID)
						}
						
						// Verify the alias is the parameter name without $
						expectedAlias := strings.TrimPrefix(param.Name, "$")
						if param.Alias != expectedAlias {
							t.Logf("Note: Alias '%s' differs from expected '%s' for parameter '%s' in %s.%s",
								param.Alias, expectedAlias, param.Name, serviceName, endpoint.ID)
						}
					}
				}
			}
		}
	}
}

func TestMicrosoft365ToolGeneration(t *testing.T) {
	// Load the configuration
	config, err := LoadConfigFromFile("../configs/microsoft365.json")
	if err != nil {
		t.Skipf("Skipping test - could not load config: %v", err)
	}
	
	// Create a Fusion instance with Microsoft365 config
	mockAuth := NewMultiTenantAuthManager(nil, NewDatabaseCache(nil, nil), nil)
	fusion := New(
		WithConfig(config),
		WithMultiTenantAuth(mockAuth),
	)

	// Get the registered tools
	tools := fusion.RegisterTools()
	
	// Check that tools were generated
	if len(tools) == 0 {
		t.Error("No tools were generated from Microsoft365 configuration")
	}

	// Check that all tool parameters are MCP-compliant
	for _, tool := range tools {
		for _, param := range tool.Parameters {
			if !IsValidMCPParameterName(param.Name) {
				t.Errorf("Tool '%s' has non-MCP-compliant parameter '%s'", 
					tool.Name, param.Name)
			}
			
			// Verify no $ prefixes in parameter names
			if strings.HasPrefix(param.Name, "$") {
				t.Errorf("Tool '%s' has parameter with $ prefix: '%s'", 
					tool.Name, param.Name)
			}
		}
	}
	
	t.Logf("Successfully generated %d tools with MCP-compliant parameters", len(tools))
}