/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFromJSON_ValidConfig(t *testing.T) {
	validConfig := `{
		"services": {
			"test_service": {
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

	config, err := LoadConfigFromJSON([]byte(validConfig), "test-config.json")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config == nil {
		t.Fatal("Expected config to be non-nil")
	}

	if config.ConfigPath != "test-config.json" {
		t.Errorf("Expected ConfigPath to be 'test-config.json', got '%s'", config.ConfigPath)
	}

	if len(config.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(config.Services))
	}

	service, exists := config.Services["test_service"]
	if !exists {
		t.Fatal("Expected 'test_service' to exist")
	}

	if service.Name != "Test Service" {
		t.Errorf("Expected service name 'Test Service', got '%s'", service.Name)
	}

	if service.BaseURL != "https://api.example.com" {
		t.Errorf("Expected baseURL 'https://api.example.com', got '%s'", service.BaseURL)
	}

	if service.Auth.Type != AuthTypeBearer {
		t.Errorf("Expected auth type 'bearer', got '%s'", service.Auth.Type)
	}

	if len(service.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint, got %d", len(service.Endpoints))
	}

	endpoint := service.Endpoints[0]
	if endpoint.ID != "test_endpoint" {
		t.Errorf("Expected endpoint ID 'test_endpoint', got '%s'", endpoint.ID)
	}
}

func TestLoadConfigFromJSON_InvalidJSON(t *testing.T) {
	invalidJSON := `{
		"services": {
			"test_service": {
				"name": "Test Service"
				"missing_comma": true
			}
		}
	}`

	_, err := LoadConfigFromJSON([]byte(invalidJSON), "test-config.json")
	if err == nil {
		t.Fatal("Expected JSON parsing error")
	}

	if !containsError(err.Error(), "failed to parse JSON config") {
		t.Errorf("Expected JSON parsing error message, got: %v", err)
	}
}

func TestLoadConfigFromJSON_ValidationFailure(t *testing.T) {
	invalidConfig := `{
		"services": {
			"test_service": {
				"name": "",
				"baseURL": "https://api.example.com",
				"auth": {
					"type": "bearer",
					"config": {
						"token": "test-token"
					}
				},
				"endpoints": []
			}
		}
	}`

	_, err := LoadConfigFromJSON([]byte(invalidConfig), "test-config.json")
	if err == nil {
		t.Fatal("Expected validation error")
	}

	if !containsError(err.Error(), "configuration validation failed") {
		t.Errorf("Expected validation error message, got: %v", err)
	}
}

func TestLoadConfigFromFile_Success(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.json")

	validConfig := `{
		"services": {
			"test_service": {
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

	if err := os.WriteFile(configFile, []byte(validConfig), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	config, err := LoadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config == nil {
		t.Fatal("Expected config to be non-nil")
	}

	if config.ConfigPath != configFile {
		t.Errorf("Expected ConfigPath to be '%s', got '%s'", configFile, config.ConfigPath)
	}
}

func TestLoadConfigFromFile_FileNotFound(t *testing.T) {
	nonExistentFile := "/path/that/does/not/exist/config.json"

	_, err := LoadConfigFromFile(nonExistentFile)
	if err == nil {
		t.Fatal("Expected file not found error")
	}

	if !containsError(err.Error(), "failed to read config file") {
		t.Errorf("Expected file read error message, got: %v", err)
	}
}

func TestExpandEnvironmentVariables_Success(t *testing.T) {
	// Set up test environment variables
	os.Setenv("TEST_TOKEN", "secret-token-123")
	os.Setenv("TEST_URL", "https://api.test.com")
	defer func() {
		os.Unsetenv("TEST_TOKEN")
		os.Unsetenv("TEST_URL")
	}()

	input := `{
		"services": {
			"test": {
				"name": "Test Service",
				"baseURL": "${TEST_URL}",
				"auth": {
					"type": "bearer",
					"config": {
						"token": "${TEST_TOKEN}"
					}
				}
			}
		}
	}`

	result, err := expandEnvironmentVariables([]byte(input))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	resultStr := string(result)
	if !containsString(resultStr, "secret-token-123") {
		t.Error("Expected TEST_TOKEN to be expanded")
	}

	if !containsString(resultStr, "https://api.test.com") {
		t.Error("Expected TEST_URL to be expanded")
	}

	// Verify it's still valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Errorf("Result should be valid JSON: %v", err)
	}
}

func TestExpandEnvironmentVariables_MissingVar(t *testing.T) {
	input := `{
		"services": {
			"test": {
				"token": "${NONEXISTENT_VAR}"
			}
		}
	}`

	result, err := expandEnvironmentVariables([]byte(input))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	resultStr := string(result)
	// Should leave the original pattern when variable doesn't exist
	if !containsString(resultStr, "${NONEXISTENT_VAR}") {
		t.Error("Expected missing variable to remain unexpanded")
	}
}

func TestExpandEnvironmentVariables_EmptyVar(t *testing.T) {
	// Set empty environment variable
	os.Setenv("EMPTY_VAR", "")
	defer os.Unsetenv("EMPTY_VAR")

	input := `{
		"token": "${EMPTY_VAR}"
	}`

	result, err := expandEnvironmentVariables([]byte(input))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	resultStr := string(result)
	// Should leave the original pattern when variable is empty
	if !containsString(resultStr, "${EMPTY_VAR}") {
		t.Error("Expected empty variable to remain unexpanded")
	}
}

func TestExpandEnvironmentVariables_MultipleVars(t *testing.T) {
	os.Setenv("VAR1", "value1")
	os.Setenv("VAR2", "value2")
	os.Setenv("VAR3", "value3")
	defer func() {
		os.Unsetenv("VAR1")
		os.Unsetenv("VAR2")
		os.Unsetenv("VAR3")
	}()

	input := `{
		"field1": "${VAR1}",
		"field2": "${VAR2}",
		"field3": "${VAR3}",
		"combined": "${VAR1}-${VAR2}-${VAR3}"
	}`

	result, err := expandEnvironmentVariables([]byte(input))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	resultStr := string(result)
	if !containsString(resultStr, "value1") {
		t.Error("Expected VAR1 to be expanded")
	}
	if !containsString(resultStr, "value2") {
		t.Error("Expected VAR2 to be expanded")
	}
	if !containsString(resultStr, "value3") {
		t.Error("Expected VAR3 to be expanded")
	}
	if !containsString(resultStr, "value1-value2-value3") {
		t.Error("Expected combined expansion to work")
	}
}

func TestExpandEnvironmentVariables_WithDefaults(t *testing.T) {
	// Set one environment variable, leave others unset to test defaults
	os.Setenv("EXISTING_VAR", "existing-value")
	defer os.Unsetenv("EXISTING_VAR")

	input := `{
		"existingVar": "${EXISTING_VAR}",
		"missingWithDefault": "${MISSING_VAR:default-value}",
		"missingWithEmptyDefault": "${MISSING_VAR2:}",
		"missingNoDefault": "${MISSING_VAR3}",
		"complexDefault": "${MISSING_VAR4:https://api.example.com/v1}"
	}`

	result, err := expandEnvironmentVariables([]byte(input))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	resultStr := string(result)

	// Should expand existing variable
	if !containsString(resultStr, "existing-value") {
		t.Error("Expected EXISTING_VAR to be expanded")
	}

	// Should use default value for missing variable
	if !containsString(resultStr, "default-value") {
		t.Error("Expected MISSING_VAR to use default value")
	}

	// Should use empty string as default
	if !containsString(resultStr, `"missingWithEmptyDefault": ""`) {
		t.Errorf("Expected MISSING_VAR2 to use empty default. Got result: %s", resultStr)
	}

	// Should leave unexpanded when no default provided
	if !containsString(resultStr, "${MISSING_VAR3}") {
		t.Errorf("Expected MISSING_VAR3 to remain unexpanded. Got result: %s", resultStr)
	}

	// Should handle complex default values
	if !containsString(resultStr, "https://api.example.com/v1") {
		t.Error("Expected MISSING_VAR4 to use complex default")
	}

	// Verify it's still valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Errorf("Result should be valid JSON: %v", err)
	}
}

func TestExpandEnvironmentVariables_DefaultWithColon(t *testing.T) {
	input := `{
		"url": "${API_URL:http://localhost:8080}",
		"timeFormat": "${TIME_FORMAT:2006-01-02T15:04:05Z07:00}"
	}`

	result, err := expandEnvironmentVariables([]byte(input))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	resultStr := string(result)

	// Should handle default values that contain colons
	if !containsString(resultStr, "http://localhost:8080") {
		t.Error("Expected API_URL to use default with colon")
	}

	if !containsString(resultStr, "2006-01-02T15:04:05Z07:00") {
		t.Error("Expected TIME_FORMAT to use default with colon")
	}
}

func TestConfig_Validate_NoServices(t *testing.T) {
	config := &Config{
		Services: make(map[string]*ServiceConfig),
		Commands: make(map[string]*CommandGroupConfig),
	}

	err := config.Validate()
	if err == nil {
		t.Fatal("Expected validation error for no services or commands")
	}

	if !containsError(err.Error(), "no services or commands configured") {
		t.Errorf("Expected 'no services or commands configured' error, got: %v", err)
	}
}

func TestServiceConfig_Validate_MissingName(t *testing.T) {
	service := &ServiceConfig{
		Name:    "",
		BaseURL: "https://api.example.com",
		Auth: AuthConfig{
			Type:   AuthTypeBearer,
			Config: map[string]interface{}{"token": "test"},
		},
		Endpoints: []EndpointConfig{
			{
				ID:          "test",
				Name:        "Test",
				Description: "Test endpoint",
				Method:      "GET",
				Path:        "/test",
				Response:    ResponseConfig{Type: ResponseTypeJSON},
			},
		},
	}

	err := service.Validate()
	if err == nil {
		t.Fatal("Expected validation error for missing name")
	}

	if !containsError(err.Error(), "service name is required") {
		t.Errorf("Expected 'service name is required' error, got: %v", err)
	}
}

func TestServiceConfig_Validate_MissingBaseURL(t *testing.T) {
	service := &ServiceConfig{
		Name:    "Test Service",
		BaseURL: "",
		Auth: AuthConfig{
			Type:   AuthTypeBearer,
			Config: map[string]interface{}{"token": "test"},
		},
		Endpoints: []EndpointConfig{
			{
				ID:          "test",
				Name:        "Test",
				Description: "Test endpoint",
				Method:      "GET",
				Path:        "/test",
				Response:    ResponseConfig{Type: ResponseTypeJSON},
			},
		},
	}

	err := service.Validate()
	if err == nil {
		t.Fatal("Expected validation error for missing baseURL")
	}

	if !containsError(err.Error(), "service baseURL is required") {
		t.Errorf("Expected 'service baseURL is required' error, got: %v", err)
	}
}

func TestServiceConfig_Validate_NoEndpoints(t *testing.T) {
	service := &ServiceConfig{
		Name:    "Test Service",
		BaseURL: "https://api.example.com",
		Auth: AuthConfig{
			Type:   AuthTypeBearer,
			Config: map[string]interface{}{"token": "test"},
		},
		Endpoints: []EndpointConfig{},
	}

	err := service.Validate()
	if err == nil {
		t.Fatal("Expected validation error for no endpoints")
	}

	if !containsError(err.Error(), "at least one endpoint is required") {
		t.Errorf("Expected 'at least one endpoint is required' error, got: %v", err)
	}
}

func TestAuthConfig_Validate_OAuth2Device(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid oauth2_device config",
			config: map[string]interface{}{
				"clientId": "test-client-id",
				"tokenURL": "https://oauth.example.com/token",
			},
			expectError: false,
		},
		{
			name: "missing clientId",
			config: map[string]interface{}{
				"tokenURL": "https://oauth.example.com/token",
			},
			expectError: true,
			errorMsg:    "oauth2_device auth requires clientId",
		},
		{
			name: "missing tokenURL",
			config: map[string]interface{}{
				"clientId": "test-client-id",
			},
			expectError: true,
			errorMsg:    "oauth2_device auth requires tokenURL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &AuthConfig{
				Type:   AuthTypeOAuth2Device,
				Config: tt.config,
			}

			err := auth.Validate()
			if tt.expectError {
				if err == nil {
					t.Fatal("Expected validation error")
				}
				if !containsError(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestAuthConfig_Validate_Bearer(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid bearer with token",
			config: map[string]interface{}{
				"token": "test-token",
			},
			expectError: false,
		},
		{
			name: "valid bearer with tokenEnvVar",
			config: map[string]interface{}{
				"tokenEnvVar": "AUTH_TOKEN",
			},
			expectError: false,
		},
		{
			name:        "missing token and tokenEnvVar",
			config:      map[string]interface{}{},
			expectError: true,
			errorMsg:    "bearer auth requires either token or tokenEnvVar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &AuthConfig{
				Type:   AuthTypeBearer,
				Config: tt.config,
			}

			err := auth.Validate()
			if tt.expectError {
				if err == nil {
					t.Fatal("Expected validation error")
				}
				if !containsError(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestAuthConfig_Validate_APIKey(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid apikey with apiKey",
			config: map[string]interface{}{
				"apiKey": "test-key",
			},
			expectError: false,
		},
		{
			name: "valid apikey with apiKeyEnvVar",
			config: map[string]interface{}{
				"apiKeyEnvVar": "API_KEY",
			},
			expectError: false,
		},
		{
			name:        "missing apiKey and apiKeyEnvVar",
			config:      map[string]interface{}{},
			expectError: true,
			errorMsg:    "api_key auth requires either apiKey or apiKeyEnvVar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &AuthConfig{
				Type:   AuthTypeAPIKey,
				Config: tt.config,
			}

			err := auth.Validate()
			if tt.expectError {
				if err == nil {
					t.Fatal("Expected validation error")
				}
				if !containsError(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestAuthConfig_Validate_Basic(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid basic auth",
			config: map[string]interface{}{
				"username": "testuser",
				"password": "testpass",
			},
			expectError: false,
		},
		{
			name: "missing username",
			config: map[string]interface{}{
				"password": "testpass",
			},
			expectError: true,
			errorMsg:    "basic auth requires username",
		},
		{
			name: "missing password",
			config: map[string]interface{}{
				"username": "testuser",
			},
			expectError: true,
			errorMsg:    "basic auth requires password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &AuthConfig{
				Type:   AuthTypeBasic,
				Config: tt.config,
			}

			err := auth.Validate()
			if tt.expectError {
				if err == nil {
					t.Fatal("Expected validation error")
				}
				if !containsError(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestAuthConfig_Validate_UnsupportedType(t *testing.T) {
	auth := &AuthConfig{
		Type:   AuthType("unsupported"),
		Config: map[string]interface{}{},
	}

	err := auth.Validate()
	if err == nil {
		t.Fatal("Expected validation error for unsupported auth type")
	}

	if !containsError(err.Error(), "unsupported auth type") {
		t.Errorf("Expected 'unsupported auth type' error, got: %v", err)
	}
}

func TestEndpointConfig_Validate_MissingFields(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    EndpointConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "missing ID",
			endpoint: EndpointConfig{
				Name:        "Test",
				Description: "Test endpoint",
				Method:      "GET",
				Path:        "/test",
				Response:    ResponseConfig{Type: ResponseTypeJSON},
			},
			expectError: true,
			errorMsg:    "endpoint ID is required",
		},
		{
			name: "missing name",
			endpoint: EndpointConfig{
				ID:          "test",
				Description: "Test endpoint",
				Method:      "GET",
				Path:        "/test",
				Response:    ResponseConfig{Type: ResponseTypeJSON},
			},
			expectError: true,
			errorMsg:    "endpoint name is required",
		},
		{
			name: "missing method",
			endpoint: EndpointConfig{
				ID:          "test",
				Name:        "Test",
				Description: "Test endpoint",
				Path:        "/test",
				Response:    ResponseConfig{Type: ResponseTypeJSON},
			},
			expectError: true,
			errorMsg:    "endpoint method is required",
		},
		{
			name: "invalid method",
			endpoint: EndpointConfig{
				ID:          "test",
				Name:        "Test",
				Description: "Test endpoint",
				Method:      "INVALID",
				Path:        "/test",
				Response:    ResponseConfig{Type: ResponseTypeJSON},
			},
			expectError: true,
			errorMsg:    "invalid HTTP method",
		},
		{
			name: "missing path",
			endpoint: EndpointConfig{
				ID:          "test",
				Name:        "Test",
				Description: "Test endpoint",
				Method:      "GET",
				Response:    ResponseConfig{Type: ResponseTypeJSON},
			},
			expectError: true,
			errorMsg:    "endpoint path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.endpoint.Validate()
			if tt.expectError {
				if err == nil {
					t.Fatal("Expected validation error")
				}
				if !containsError(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestLoadConfigFromFile_RealExample(t *testing.T) {
	// Test loading the actual Google config file
	configPath := filepath.Join("..", "configs", "google.json")

	config, err := LoadConfigFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load Google config: %v", err)
	}

	if config == nil {
		t.Fatal("Config should not be nil")
	}

	// Verify the Google service is loaded correctly
	googleService, exists := config.Services["google"]
	if !exists {
		t.Fatal("Google service should exist")
	}

	if googleService.Name != "Google APIs" {
		t.Errorf("Expected Google service name 'Google APIs', got '%s'", googleService.Name)
	}

	if googleService.BaseURL != "https://www.googleapis.com" {
		t.Errorf("Expected Google baseURL 'https://www.googleapis.com', got '%s'", googleService.BaseURL)
	}

	if googleService.Auth.Type != AuthTypeOAuth2Device {
		t.Errorf("Expected Google auth type 'oauth2_device', got '%s'", googleService.Auth.Type)
	}

	if len(googleService.Endpoints) < 16 {
		t.Errorf("Expected at least 16 Google endpoints, got %d", len(googleService.Endpoints))
	}

	// Verify specific endpoints
	calendarEndpoint := googleService.GetEndpointByID("calendar_events_list")
	if calendarEndpoint == nil {
		t.Fatal("Calendar events list endpoint should exist")
	}

	if calendarEndpoint.Name != "List Calendar Events" {
		t.Errorf("Expected calendar endpoint name 'List Calendar Events', got '%s'", calendarEndpoint.Name)
	}

	if calendarEndpoint.Method != "GET" {
		t.Errorf("Expected calendar endpoint method 'GET', got '%s'", calendarEndpoint.Method)
	}

	if calendarEndpoint.Path != "/calendar/v3/calendars/primary/events" {
		t.Errorf("Expected calendar endpoint path '/calendar/v3/calendars/primary/events', got '%s'", calendarEndpoint.Path)
	}
}

func TestLoadConfigFromFile_WithEnvironmentVariables(t *testing.T) {
	// Set test environment variables
	os.Setenv("TEST_BASE_URL", "https://test-api.example.com")
	os.Setenv("TEST_TOKEN", "test-secret-token")
	defer func() {
		os.Unsetenv("TEST_BASE_URL")
		os.Unsetenv("TEST_TOKEN")
	}()

	// Create a test config with environment variables
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-env-config.json")

	configContent := `{
		"services": {
			"test_service": {
				"name": "Test Service",
				"baseURL": "${TEST_BASE_URL}",
				"auth": {
					"type": "bearer",
					"config": {
						"token": "${TEST_TOKEN}",
						"header": "${AUTH_HEADER:Authorization}"
					}
				},
				"endpoints": [
					{
						"id": "test_endpoint",
						"name": "Test Endpoint",
						"description": "A test endpoint with env vars",
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

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	config, err := LoadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("Failed to load config with env vars: %v", err)
	}

	service := config.Services["test_service"]
	if service.BaseURL != "https://test-api.example.com" {
		t.Errorf("Expected baseURL to be expanded, got '%s'", service.BaseURL)
	}

	if token, ok := service.Auth.Config["token"].(string); !ok || token != "test-secret-token" {
		t.Errorf("Expected token to be expanded, got '%v'", service.Auth.Config["token"])
	}

	// Should use default value for missing AUTH_HEADER env var
	if header, ok := service.Auth.Config["header"].(string); !ok || header != "Authorization" {
		t.Errorf("Expected header to use default value, got '%v'", service.Auth.Config["header"])
	}
}

func TestConfig_GetServiceByName(t *testing.T) {
	config := &Config{
		Services: map[string]*ServiceConfig{
			"service1": {Name: "Service One"},
			"service2": {Name: "Service Two"},
		},
	}

	// Test existing service
	service := config.GetServiceByName("Service One")
	if service == nil {
		t.Fatal("Expected to find Service One")
	}
	if service != config.Services["service1"] {
		t.Error("Expected to get service1")
	}

	// Test non-existent service
	service = config.GetServiceByName("Non-existent")
	if service != nil {
		t.Error("Expected nil for non-existent service")
	}
}

func TestConfig_GetAllEndpoints(t *testing.T) {
	config := &Config{
		Services: map[string]*ServiceConfig{
			"service1": {
				Name: "Service One",
				Endpoints: []EndpointConfig{
					{ID: "ep1", Name: "Endpoint 1"},
					{ID: "ep2", Name: "Endpoint 2"},
				},
			},
			"service2": {
				Name: "Service Two",
				Endpoints: []EndpointConfig{
					{ID: "ep3", Name: "Endpoint 3"},
				},
			},
		},
	}

	endpoints := config.GetAllEndpoints()
	if len(endpoints) != 3 {
		t.Errorf("Expected 3 endpoints, got %d", len(endpoints))
	}

	// Check that endpoints have correct service context
	for _, ep := range endpoints {
		if ep.ServiceName == "" {
			t.Error("Service name should not be empty")
		}
		if ep.Service == nil {
			t.Error("Service should not be nil")
		}
		if ep.Endpoint.ID == "" {
			t.Error("Endpoint ID should not be empty")
		}
	}
}

func TestConfig_ValidateServiceConfig(t *testing.T) {
	config := &Config{
		Services: map[string]*ServiceConfig{
			"valid_service": {
				Name:    "Valid Service",
				BaseURL: "https://api.example.com",
				Auth: AuthConfig{
					Type:   AuthTypeBearer,
					Config: map[string]interface{}{"token": "test"},
				},
				Endpoints: []EndpointConfig{
					{
						ID:          "test",
						Name:        "Test",
						Description: "Test endpoint",
						Method:      "GET",
						Path:        "/test",
						Response:    ResponseConfig{Type: ResponseTypeJSON},
					},
				},
			},
			"invalid_service": {
				Name:    "",
				BaseURL: "https://api.example.com",
			},
		},
	}

	// Test valid service
	err := config.ValidateServiceConfig("valid_service")
	if err != nil {
		t.Errorf("Expected valid service to pass validation, got: %v", err)
	}

	// Test invalid service
	err = config.ValidateServiceConfig("invalid_service")
	if err == nil {
		t.Error("Expected invalid service to fail validation")
	}

	// Test non-existent service
	err = config.ValidateServiceConfig("non_existent")
	if err == nil {
		t.Error("Expected non-existent service to return error")
	}
	var configErr *ConfigurationError
	if !errors.As(err, &configErr) {
		t.Error("Expected ConfigurationError for non-existent service")
	}
}

func TestConfig_GetRequiredEnvironmentVariables(t *testing.T) {
	validConfig := `{
		"services": {
			"test_service": {
				"name": "Test Service",
				"baseURL": "${API_URL}",
				"auth": {
					"type": "bearer",
					"config": {
						"token": "${API_TOKEN}",
						"header": "${AUTH_HEADER:Authorization}"
					}
				},
				"endpoints": [
					{
						"id": "test_endpoint",
						"name": "Test Endpoint",
						"description": "A test endpoint",
						"method": "GET",
						"path": "/test/${USER_ID}",
						"response": {
							"type": "json"
						}
					}
				]
			}
		}
	}`

	config, err := LoadConfigFromJSON([]byte(validConfig), "test-config.json")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	envVars := config.GetRequiredEnvironmentVariables()

	// AUTH_HEADER has a default, so it should not be considered "required"
	expectedVars := []string{"API_URL", "API_TOKEN", "USER_ID"}
	if len(envVars) != len(expectedVars) {
		t.Errorf("Expected %d environment variables, got %d: %v", len(expectedVars), len(envVars), envVars)
	}

	// Convert to map for easier checking
	varMap := make(map[string]bool)
	for _, v := range envVars {
		varMap[v] = true
	}

	for _, expected := range expectedVars {
		if !varMap[expected] {
			t.Errorf("Expected environment variable %s not found", expected)
		}
	}
}

func TestConfig_Clone(t *testing.T) {
	original := &Config{
		ConfigPath: "original-config.json",
		Services: map[string]*ServiceConfig{
			"service1": {
				Name:    "Service One",
				BaseURL: "https://api1.example.com",
				Auth: AuthConfig{
					Type:   AuthTypeBearer,
					Config: map[string]interface{}{"token": "test1"},
				},
				Endpoints: []EndpointConfig{
					{
						ID:          "ep1",
						Name:        "Endpoint 1",
						Description: "First endpoint",
						Method:      "GET",
						Path:        "/test1",
						Response:    ResponseConfig{Type: ResponseTypeJSON},
					},
				},
			},
		},
	}

	clone, err := original.Clone()
	if err != nil {
		t.Fatalf("Failed to clone config: %v", err)
	}

	if clone.ConfigPath != original.ConfigPath {
		t.Error("ConfigPath should be copied")
	}

	if len(clone.Services) != len(original.Services) {
		t.Error("Services should be copied")
	}

	// Modify clone to ensure it's a deep copy
	clone.Services["service1"].Name = "Modified Service"
	if original.Services["service1"].Name == "Modified Service" {
		t.Error("Clone should be independent of original")
	}
}

func TestConfig_MergeConfig(t *testing.T) {
	base := &Config{
		Services: map[string]*ServiceConfig{
			"service1": {
				Name:    "Service One",
				BaseURL: "https://api1.example.com",
				Auth: AuthConfig{
					Type:   AuthTypeBearer,
					Config: map[string]interface{}{"token": "test1"},
				},
				Endpoints: []EndpointConfig{
					{
						ID:          "ep1",
						Name:        "Endpoint 1",
						Description: "First endpoint",
						Method:      "GET",
						Path:        "/test1",
						Response:    ResponseConfig{Type: ResponseTypeJSON},
					},
				},
			},
		},
	}

	other := &Config{
		Services: map[string]*ServiceConfig{
			"service2": {
				Name:    "Service Two",
				BaseURL: "https://api2.example.com",
				Auth: AuthConfig{
					Type:   AuthTypeBearer,
					Config: map[string]interface{}{"token": "test2"},
				},
				Endpoints: []EndpointConfig{
					{
						ID:          "ep2",
						Name:        "Endpoint 2",
						Description: "Second endpoint",
						Method:      "GET",
						Path:        "/test2",
						Response:    ResponseConfig{Type: ResponseTypeJSON},
					},
				},
			},
		},
	}

	err := base.MergeConfig(other)
	if err != nil {
		t.Fatalf("Failed to merge configs: %v", err)
	}

	if len(base.Services) != 2 {
		t.Errorf("Expected 2 services after merge, got %d", len(base.Services))
	}

	if _, exists := base.Services["service2"]; !exists {
		t.Error("service2 should exist after merge")
	}

	// Test merging with conflict
	conflicting := &Config{
		Services: map[string]*ServiceConfig{
			"service1": {Name: "Conflicting Service"},
		},
	}

	err = base.MergeConfig(conflicting)
	if err == nil {
		t.Error("Expected error when merging conflicting services")
	}

	var configErr *ConfigurationError
	if !errors.As(err, &configErr) {
		t.Error("Expected ConfigurationError for merge conflict")
	}
}

// Helper functions for testing
func containsError(actual, expected string) bool {
	return containsString(actual, expected)
}

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle ||
			haystack[:len(needle)] == needle ||
			haystack[len(haystack)-len(needle):] == needle ||
			containsSubstring(haystack, needle))
}

func containsSubstring(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
