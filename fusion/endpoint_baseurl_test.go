/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"encoding/json"
	"testing"
)

func TestEndpointConfig_BaseURL_JSONSerialization(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       EndpointConfig
		expectInJSON   bool   // whether "baseURL" should appear in the serialized JSON
		expectedURL    string // expected BaseURL value after round-trip
	}{
		{
			name: "without baseURL omits field from JSON",
			endpoint: EndpointConfig{
				ID:          "test_endpoint",
				Name:        "Test Endpoint",
				Description: "An endpoint without baseURL override",
				Method:      "GET",
				Path:        "/v1/test",
				Response:    ResponseConfig{Type: ResponseTypeJSON},
			},
			expectInJSON: false,
			expectedURL:  "",
		},
		{
			name: "with baseURL includes field in JSON",
			endpoint: EndpointConfig{
				ID:          "test_endpoint",
				Name:        "Test Endpoint",
				Description: "An endpoint with baseURL override",
				Method:      "GET",
				Path:        "/v1/test",
				BaseURL:     "https://override.example.com",
				Response:    ResponseConfig{Type: ResponseTypeJSON},
			},
			expectInJSON: true,
			expectedURL:  "https://override.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.endpoint)
			if err != nil {
				t.Fatalf("Failed to marshal EndpointConfig: %v", err)
			}

			// Check whether the raw JSON contains the "baseURL" key
			var raw map[string]interface{}
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("Failed to unmarshal raw JSON: %v", err)
			}

			_, hasBaseURL := raw["baseURL"]
			if tt.expectInJSON && !hasBaseURL {
				t.Errorf("Expected 'baseURL' key in JSON, but it was not present: %s", string(data))
			}
			if !tt.expectInJSON && hasBaseURL {
				t.Errorf("Expected 'baseURL' key to be omitted from JSON, but it was present: %s", string(data))
			}

			// Round-trip: unmarshal back and verify the BaseURL field
			var decoded EndpointConfig
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal EndpointConfig: %v", err)
			}

			if decoded.BaseURL != tt.expectedURL {
				t.Errorf("Expected BaseURL '%s' after round-trip, got '%s'", tt.expectedURL, decoded.BaseURL)
			}
		})
	}
}

func TestEndpointConfig_BaseURL_DeserializeFromJSON(t *testing.T) {
	tests := []struct {
		name        string
		jsonInput   string
		expectedURL string
	}{
		{
			name: "JSON without baseURL field",
			jsonInput: `{
				"id": "ep1",
				"name": "Endpoint One",
				"description": "No override",
				"method": "GET",
				"path": "/test"
			}`,
			expectedURL: "",
		},
		{
			name: "JSON with baseURL field",
			jsonInput: `{
				"id": "ep2",
				"name": "Endpoint Two",
				"description": "With override",
				"method": "POST",
				"path": "/test",
				"baseURL": "https://different-host.example.com"
			}`,
			expectedURL: "https://different-host.example.com",
		},
		{
			name: "JSON with empty baseURL field",
			jsonInput: `{
				"id": "ep3",
				"name": "Endpoint Three",
				"description": "Empty override",
				"method": "GET",
				"path": "/test",
				"baseURL": ""
			}`,
			expectedURL: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ep EndpointConfig
			if err := json.Unmarshal([]byte(tt.jsonInput), &ep); err != nil {
				t.Fatalf("Failed to unmarshal JSON: %v", err)
			}

			if ep.BaseURL != tt.expectedURL {
				t.Errorf("Expected BaseURL '%s', got '%s'", tt.expectedURL, ep.BaseURL)
			}
		})
	}
}

func TestMapper_BuildURL_EndpointBaseURLOverride(t *testing.T) {
	mapper := NewMapper(nil)

	tests := []struct {
		name        string
		baseURL     string
		path        string
		params      []ParameterConfig
		args        map[string]interface{}
		expectedURL string
		expectError bool
	}{
		{
			name:        "service-level baseURL with simple path",
			baseURL:     "https://graph.microsoft.com/v1.0",
			path:        "/me/messages",
			params:      nil,
			args:        nil,
			expectedURL: "https://graph.microsoft.com/v1.0/me/messages",
			expectError: false,
		},
		{
			name:        "endpoint-level baseURL override with simple path",
			baseURL:     "https://www.googleapis.com",
			path:        "/calendar/v3/calendars/primary/events",
			params:      nil,
			args:        nil,
			expectedURL: "https://www.googleapis.com/calendar/v3/calendars/primary/events",
			expectError: false,
		},
		{
			name:        "trailing slash on baseURL is trimmed",
			baseURL:     "https://api.example.com/",
			path:        "/v1/resources",
			params:      nil,
			args:        nil,
			expectedURL: "https://api.example.com/v1/resources",
			expectError: false,
		},
		{
			name:        "no leading slash on path is handled",
			baseURL:     "https://api.example.com",
			path:        "v1/resources",
			params:      nil,
			args:        nil,
			expectedURL: "https://api.example.com/v1/resources",
			expectError: false,
		},
		{
			name:    "baseURL override with path parameters",
			baseURL: "https://override.example.com/api",
			path:    "/users/{userId}/items",
			params: []ParameterConfig{
				{
					Name:     "userId",
					Type:     "string",
					Location: "path",
					Required: true,
				},
			},
			args:        map[string]interface{}{"userId": "abc123"},
			expectedURL: "https://override.example.com/api/users/abc123/items",
			expectError: false,
		},
		{
			name:    "different baseURLs produce different URLs for same path",
			baseURL: "https://people.googleapis.com",
			path:    "/v1/people/me/connections",
			params:  nil,
			args:    nil,
			expectedURL: "https://people.googleapis.com/v1/people/me/connections",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mapper.BuildURL(tt.baseURL, tt.path, tt.params, tt.args)
			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result != tt.expectedURL {
				t.Errorf("Expected URL '%s', got '%s'", tt.expectedURL, result)
			}
		})
	}
}

func TestBuildURL_ServiceVsEndpointBaseURL(t *testing.T) {
	// This test simulates the logic in handler.go buildRequest(), verifying that
	// the endpoint-level baseURL takes precedence over the service-level baseURL.
	mapper := NewMapper(nil)

	serviceBaseURL := "https://www.googleapis.com"
	endpointBaseURL := "https://people.googleapis.com"
	path := "/v1/people/me/connections"

	// Simulate service-level fallback (endpoint.BaseURL is empty)
	effectiveBaseURL := serviceBaseURL
	endpointOverride := ""
	if endpointOverride != "" {
		effectiveBaseURL = endpointOverride
	}

	resultWithService, err := mapper.BuildURL(effectiveBaseURL, path, nil, nil)
	if err != nil {
		t.Fatalf("Failed to build URL with service baseURL: %v", err)
	}

	expectedServiceURL := "https://www.googleapis.com/v1/people/me/connections"
	if resultWithService != expectedServiceURL {
		t.Errorf("Service baseURL: expected '%s', got '%s'", expectedServiceURL, resultWithService)
	}

	// Simulate endpoint-level override (endpoint.BaseURL is set)
	effectiveBaseURL = serviceBaseURL
	endpointOverride = endpointBaseURL
	if endpointOverride != "" {
		effectiveBaseURL = endpointOverride
	}

	resultWithEndpoint, err := mapper.BuildURL(effectiveBaseURL, path, nil, nil)
	if err != nil {
		t.Fatalf("Failed to build URL with endpoint baseURL: %v", err)
	}

	expectedEndpointURL := "https://people.googleapis.com/v1/people/me/connections"
	if resultWithEndpoint != expectedEndpointURL {
		t.Errorf("Endpoint baseURL: expected '%s', got '%s'", expectedEndpointURL, resultWithEndpoint)
	}

	// Verify they are different
	if resultWithService == resultWithEndpoint {
		t.Error("Service-level and endpoint-level URLs should differ when baseURLs differ")
	}
}

func TestLoadConfigFromJSON_EndpointBaseURLOverride(t *testing.T) {
	configJSON := `{
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
						"id": "default_base",
						"name": "Default Base",
						"description": "Uses service baseURL",
						"method": "GET",
						"path": "/v1/default",
						"response": { "type": "json" }
					},
					{
						"id": "custom_base",
						"name": "Custom Base",
						"description": "Uses endpoint baseURL override",
						"method": "GET",
						"path": "/v1/custom",
						"baseURL": "https://other-api.example.com",
						"response": { "type": "json" }
					}
				]
			}
		}
	}`

	config, err := LoadConfigFromJSON([]byte(configJSON), "test-endpoint-baseurl.json")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	service, exists := config.Services["test_service"]
	if !exists {
		t.Fatal("Expected 'test_service' to exist")
	}

	if len(service.Endpoints) != 2 {
		t.Fatalf("Expected 2 endpoints, got %d", len(service.Endpoints))
	}

	// First endpoint should have no BaseURL override
	defaultEP := service.GetEndpointByID("default_base")
	if defaultEP == nil {
		t.Fatal("Expected 'default_base' endpoint to exist")
	}
	if defaultEP.BaseURL != "" {
		t.Errorf("Expected empty BaseURL for default endpoint, got '%s'", defaultEP.BaseURL)
	}

	// Second endpoint should have the override
	customEP := service.GetEndpointByID("custom_base")
	if customEP == nil {
		t.Fatal("Expected 'custom_base' endpoint to exist")
	}
	if customEP.BaseURL != "https://other-api.example.com" {
		t.Errorf("Expected BaseURL 'https://other-api.example.com', got '%s'", customEP.BaseURL)
	}
}
