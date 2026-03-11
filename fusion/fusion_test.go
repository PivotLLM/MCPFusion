/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"testing"

	"github.com/PivotLLM/MCPFusion/global"
	"github.com/PivotLLM/MCPFusion/mlogger"
)

func TestNew(t *testing.T) {
	// Test that creating a Fusion instance automatically creates multi-tenant auth
	fusion := New()

	if fusion.multiTenantAuth == nil {
		t.Error("Expected multi-tenant auth to be auto-created")
	}

	if fusion.cache == nil {
		t.Error("Expected cache to be set from multi-tenant auth manager")
	}
}

func TestNewWithOptions(t *testing.T) {
	// Create a memory logger
	memLogger := mlogger.NewMemoryLogger()

	// Test creating with options
	fusion := New(
		WithLogger(memLogger),
	)

	if fusion.logger != memLogger {
		t.Error("Logger not set correctly")
	}

	// Should always use database cache with multi-tenant auth (auto-created)
	if _, ok := fusion.cache.(*DatabaseCache); !ok {
		t.Error("Expected DatabaseCache when multi-tenant auth is configured")
	}

	// Multi-tenant auth should be auto-created
	if fusion.multiTenantAuth == nil {
		t.Error("Multi-tenant auth manager should be auto-created")
	}
}

func TestNewWithJSONConfig(t *testing.T) {
	jsonConfig := `{
		"services": {
			"test": {
				"name": "Test Service",
				"baseURL": "https://api.example.com",
				"auth": {
					"type": "bearer",
					"config": {
						"token": "test-token"
					}
				},
				"endpoints": [
					{
						"id": "test_endpoint",
						"name": "Test Endpoint",
						"description": "A test endpoint",
						"method": "GET",
						"path": "/test",
						"response": {
							"type": "json"
						}
					}
				]
			}
		}
	}`

	fusion := New(
		WithJSONConfigData([]byte(jsonConfig), "test-config.json"),
	)

	if fusion.config == nil {
		t.Fatal("Config should be loaded")
	}

	if len(fusion.config.Services) != 1 {
		t.Error("Expected 1 service")
	}

	if _, exists := fusion.config.Services["test"]; !exists {
		t.Error("Test service not found")
	}
}

func TestRegisterTools(t *testing.T) {
	jsonConfig := `{
		"services": {
			"test": {
				"name": "Test Service",
				"baseURL": "https://api.example.com",
				"auth": {
					"type": "bearer",
					"config": {
						"token": "test-token"
					}
				},
				"endpoints": [
					{
						"id": "test_endpoint",
						"name": "Test Endpoint",
						"description": "A test endpoint",
						"method": "GET",
						"path": "/test",
						"parameters": [
							{
								"name": "id",
								"description": "ID parameter",
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

	fusion := New(
		WithJSONConfigData([]byte(jsonConfig), "test-config.json"),
	)

	tools := fusion.RegisterTools()

	// Find the tool by name rather than relying on position or count,
	// since additional built-in tools (e.g. health_status) may also be registered.
	var targetTool *global.ToolDefinition
	for i := range tools {
		if tools[i].Name == "test_test_endpoint" {
			targetTool = &tools[i]
			break
		}
	}

	if targetTool == nil {
		t.Fatalf("Expected to find tool 'test_test_endpoint', got tools: %v",
			func() []string {
				names := make([]string, len(tools))
				for i, tool := range tools {
					names[i] = tool.Name
				}
				return names
			}())
	}

	if len(targetTool.Parameters) != 1 {
		t.Errorf("Expected 1 parameter, got %d", len(targetTool.Parameters))
	}

	if targetTool.Parameters[0].Name != "id" {
		t.Errorf("Expected parameter name 'id', got '%s'", targetTool.Parameters[0].Name)
	}
}

func TestInterfaceImplementation(t *testing.T) {
	fusion := New()

	// Test that Fusion implements all required interfaces
	var _ global.ToolProvider = fusion
	var _ global.ResourceProvider = fusion
	var _ global.PromptProvider = fusion
}

func TestGetServiceNames(t *testing.T) {
	jsonConfig := `{
		"services": {
			"service1": {
				"name": "Service 1",
				"baseURL": "https://api1.example.com",
				"auth": {"type": "bearer", "config": {"token": "token1"}},
				"endpoints": [{"id": "e1", "name": "E1", "description": "E1", "method": "GET", "path": "/", "response": {"type": "json"}}]
			},
			"service2": {
				"name": "Service 2",
				"baseURL": "https://api2.example.com",
				"auth": {"type": "bearer", "config": {"token": "token2"}},
				"endpoints": [{"id": "e2", "name": "E2", "description": "E2", "method": "GET", "path": "/", "response": {"type": "json"}}]
			}
		}
	}`

	fusion := New(
		WithJSONConfigData([]byte(jsonConfig), "test-config.json"),
	)

	names := fusion.GetServiceNames()

	if len(names) != 2 {
		t.Errorf("Expected 2 service names, got %d", len(names))
	}

	// Check that both services are present (order doesn't matter)
	found1, found2 := false, false
	for _, name := range names {
		if name == "service1" {
			found1 = true
		}
		if name == "service2" {
			found2 = true
		}
	}

	if !found1 || !found2 {
		t.Error("Expected to find both service1 and service2")
	}
}
