// Copyright (c) 2025 Tenebris Technologies Inc.
// Please see LICENSE for details.

package fusion

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildRequest_BasicGET(t *testing.T) {
	fusion := New()
	
	service := &ServiceConfig{
		BaseURL: "https://api.example.com",
	}
	
	endpoint := &EndpointConfig{
		Method: "GET",
		Path:   "/users",
		Parameters: []ParameterConfig{
			{
				Name:     "limit",
				Type:     ParameterTypeNumber,
				Location: ParameterLocationQuery,
				Required: false,
			},
		},
	}
	
	options := map[string]any{
		"limit": 10,
	}
	
	ctx := context.Background()
	req, err := fusion.buildRequest(ctx, "test", service, endpoint, options)
	if err != nil {
		t.Fatalf("buildRequest failed: %v", err)
	}
	
	if req.Method != "GET" {
		t.Errorf("Expected method GET, got %s", req.Method)
	}
	
	expectedURL := "https://api.example.com/users?limit=10"
	if req.URL.String() != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, req.URL.String())
	}
}

func TestBuildRequest_POSTWithBody(t *testing.T) {
	fusion := New()
	
	service := &ServiceConfig{
		BaseURL: "https://api.example.com",
	}
	
	endpoint := &EndpointConfig{
		Method: "POST",
		Path:   "/users",
		Parameters: []ParameterConfig{
			{
				Name:     "name",
				Type:     ParameterTypeString,
				Location: ParameterLocationBody,
				Required: true,
			},
			{
				Name:     "email",
				Type:     ParameterTypeString,
				Location: ParameterLocationBody,
				Required: true,
			},
		},
	}
	
	options := map[string]any{
		"name":  "John Doe",
		"email": "john@example.com",
	}
	
	ctx := context.Background()
	req, err := fusion.buildRequest(ctx, "test", service, endpoint, options)
	if err != nil {
		t.Fatalf("buildRequest failed: %v", err)
	}
	
	if req.Method != "POST" {
		t.Errorf("Expected method POST, got %s", req.Method)
	}
	
	if req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", req.Header.Get("Content-Type"))
	}
	
	// Verify body content
	var bodyData map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&bodyData); err != nil {
		t.Fatalf("Failed to decode request body: %v", err)
	}
	
	if bodyData["name"] != "John Doe" {
		t.Errorf("Expected name 'John Doe', got %v", bodyData["name"])
	}
	
	if bodyData["email"] != "john@example.com" {
		t.Errorf("Expected email 'john@example.com', got %v", bodyData["email"])
	}
}

func TestBuildRequest_PathParameters(t *testing.T) {
	fusion := New()
	
	service := &ServiceConfig{
		BaseURL: "https://api.example.com",
	}
	
	endpoint := &EndpointConfig{
		Method: "GET",
		Path:   "/users/{id}",
		Parameters: []ParameterConfig{
			{
				Name:     "id",
				Type:     ParameterTypeString,
				Location: ParameterLocationPath,
				Required: true,
			},
		},
	}
	
	options := map[string]any{
		"id": "123",
	}
	
	ctx := context.Background()
	req, err := fusion.buildRequest(ctx, "test", service, endpoint, options)
	if err != nil {
		t.Fatalf("buildRequest failed: %v", err)
	}
	
	expectedURL := "https://api.example.com/users/123"
	if req.URL.String() != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, req.URL.String())
	}
}

func TestBuildRequest_HeaderParameters(t *testing.T) {
	fusion := New()
	
	service := &ServiceConfig{
		BaseURL: "https://api.example.com",
	}
	
	endpoint := &EndpointConfig{
		Method: "GET",
		Path:   "/users",
		Parameters: []ParameterConfig{
			{
				Name:     "api-version",
				Type:     ParameterTypeString,
				Location: ParameterLocationHeader,
				Required: false,
			},
		},
	}
	
	options := map[string]any{
		"api-version": "v1",
	}
	
	ctx := context.Background()
	req, err := fusion.buildRequest(ctx, "test", service, endpoint, options)
	if err != nil {
		t.Fatalf("buildRequest failed: %v", err)
	}
	
	if req.Header.Get("api-version") != "v1" {
		t.Errorf("Expected header 'api-version: v1', got %s", req.Header.Get("api-version"))
	}
}

func TestBuildRequest_RequiredParameterMissing(t *testing.T) {
	fusion := New()
	
	service := &ServiceConfig{
		BaseURL: "https://api.example.com",
	}
	
	endpoint := &EndpointConfig{
		Method: "GET",
		Path:   "/users/{id}",
		Parameters: []ParameterConfig{
			{
				Name:     "id",
				Type:     ParameterTypeString,
				Location: ParameterLocationPath,
				Required: true,
			},
		},
	}
	
	options := map[string]any{} // Missing required parameter
	
	ctx := context.Background()
	_, err := fusion.buildRequest(ctx, "test", service, endpoint, options)
	if err == nil {
		t.Fatal("Expected error for missing required parameter")
	}
	
	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Errorf("Expected ValidationError, got %T", err)
	} else if validationErr.Parameter != "id" {
		t.Errorf("Expected parameter 'id' in error, got %s", validationErr.Parameter)
	}
}

func TestBuildRequest_DefaultValue(t *testing.T) {
	fusion := New()
	
	service := &ServiceConfig{
		BaseURL: "https://api.example.com",
	}
	
	endpoint := &EndpointConfig{
		Method: "GET",
		Path:   "/users",
		Parameters: []ParameterConfig{
			{
				Name:     "limit",
				Type:     ParameterTypeNumber,
				Location: ParameterLocationQuery,
				Required: false,
				Default:  50,
			},
		},
	}
	
	options := map[string]any{} // No limit provided, should use default
	
	ctx := context.Background()
	req, err := fusion.buildRequest(ctx, "test", service, endpoint, options)
	if err != nil {
		t.Fatalf("buildRequest failed: %v", err)
	}
	
	expectedURL := "https://api.example.com/users?limit=50"
	if req.URL.String() != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, req.URL.String())
	}
}

func TestProcessResponse_JSON(t *testing.T) {
	fusion := New()
	
	endpoint := &EndpointConfig{
		Response: ResponseConfig{
			Type: ResponseTypeJSON,
		},
	}
	
	// Create a mock response
	responseData := map[string]interface{}{
		"id":   123,
		"name": "John Doe",
	}
	jsonData, _ := json.Marshal(responseData)
	
	// Mock the response body
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)
	}))
	defer server.Close()
	
	// Create actual response
	realResp, _ := http.Get(server.URL)
	defer realResp.Body.Close()
	
	result, err := fusion.processResponse(realResp, endpoint, "test")
	if err != nil {
		t.Fatalf("processResponse failed: %v", err)
	}
	
	// Parse the result to verify it's valid JSON
	var parsedResult map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsedResult); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}
	
	if parsedResult["id"].(float64) != 123 {
		t.Errorf("Expected id 123, got %v", parsedResult["id"])
	}
	
	if parsedResult["name"] != "John Doe" {
		t.Errorf("Expected name 'John Doe', got %v", parsedResult["name"])
	}
}

func TestProcessResponse_Text(t *testing.T) {
	fusion := New()
	
	endpoint := &EndpointConfig{
		Response: ResponseConfig{
			Type: ResponseTypeText,
		},
	}
	
	expectedText := "This is a text response"
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(expectedText))
	}))
	defer server.Close()
	
	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()
	
	result, err := fusion.processResponse(resp, endpoint, "test")
	if err != nil {
		t.Fatalf("processResponse failed: %v", err)
	}
	
	if result != expectedText {
		t.Errorf("Expected '%s', got '%s'", expectedText, result)
	}
}

func TestProcessResponse_HTTPError(t *testing.T) {
	fusion := New()
	
	endpoint := &EndpointConfig{
		ID: "test_endpoint",
		Response: ResponseConfig{
			Type: ResponseTypeJSON,
		},
	}
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"error": "Not found"}`))
	}))
	defer server.Close()
	
	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()
	
	_, err := fusion.processResponse(resp, endpoint, "test")
	if err == nil {
		t.Fatal("Expected error for 404 response")
	}
	
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Errorf("Expected APIError, got %T", err)
	} else {
		if apiErr.StatusCode != 404 {
			t.Errorf("Expected status code 404, got %d", apiErr.StatusCode)
		}
		if apiErr.Service != "test" {
			t.Errorf("Expected service 'test', got %s", apiErr.Service)
		}
		if apiErr.Endpoint != "test_endpoint" {
			t.Errorf("Expected endpoint 'test_endpoint', got %s", apiErr.Endpoint)
		}
	}
}

func TestValidateParameter_String(t *testing.T) {
	fusion := New()
	
	param := &ParameterConfig{
		Name: "test",
		Type: ParameterTypeString,
		Validation: &ValidationConfig{
			MinLength: 3,
			MaxLength: 10,
			Pattern:   "^[a-z]+$",
		},
	}
	
	// Valid string
	if err := fusion.validateParameter(param, "hello"); err != nil {
		t.Errorf("Valid string failed validation: %v", err)
	}
	
	// Too short
	if err := fusion.validateParameter(param, "hi"); err == nil {
		t.Error("Expected error for string too short")
	}
	
	// Too long
	if err := fusion.validateParameter(param, "verylongstring"); err == nil {
		t.Error("Expected error for string too long")
	}
	
	// Invalid pattern
	if err := fusion.validateParameter(param, "Hello"); err == nil {
		t.Error("Expected error for string not matching pattern")
	}
	
	// Wrong type
	if err := fusion.validateParameter(param, 123); err == nil {
		t.Error("Expected error for non-string value")
	}
}

func TestValidateParameter_Number(t *testing.T) {
	fusion := New()
	
	param := &ParameterConfig{
		Name: "test",
		Type: ParameterTypeNumber,
	}
	
	// Valid numbers
	if err := fusion.validateParameter(param, 123); err != nil {
		t.Errorf("Valid int failed validation: %v", err)
	}
	
	if err := fusion.validateParameter(param, 123.45); err != nil {
		t.Errorf("Valid float failed validation: %v", err)
	}
	
	if err := fusion.validateParameter(param, int64(123)); err != nil {
		t.Errorf("Valid int64 failed validation: %v", err)
	}
	
	// Invalid type
	if err := fusion.validateParameter(param, "123"); err == nil {
		t.Error("Expected error for string value")
	}
}

func TestValidateParameter_Enum(t *testing.T) {
	fusion := New()
	
	param := &ParameterConfig{
		Name: "test",
		Type: ParameterTypeString,
		Validation: &ValidationConfig{
			Enum: []interface{}{"red", "green", "blue"},
		},
	}
	
	// Valid enum value
	if err := fusion.validateParameter(param, "red"); err != nil {
		t.Errorf("Valid enum value failed validation: %v", err)
	}
	
	// Invalid enum value
	if err := fusion.validateParameter(param, "yellow"); err == nil {
		t.Error("Expected error for invalid enum value")
	}
}

func TestTransformParameter_BasicTransforms(t *testing.T) {
	fusion := New()
	
	tests := []struct {
		name       string
		transform  string
		input      interface{}
		expected   interface{}
		shouldFail bool
	}{
		{"toString", "toString", 123, "123", false},
		{"toInt valid", "toInt", "123", 123, false},
		{"toInt invalid", "toInt", "abc", nil, true},
		{"toLowerCase", "toLowerCase", "HELLO", "hello", false},
		{"toUpperCase", "toUpperCase", "hello", "HELLO", false},
		{"unknown", "unknown", "test", "test", false}, // Should return unchanged
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param := &ParameterConfig{
				Name: "test",
				Transform: &TransformConfig{
					Expression: tt.transform,
				},
			}
			
			result, err := fusion.transformParameter(param, tt.input)
			
			if tt.shouldFail {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExtractJSONPath(t *testing.T) {
	fusion := New()
	
	data := map[string]interface{}{
		"user": map[string]interface{}{
			"name": "John Doe",
			"age":  30,
		},
		"items": []interface{}{
			map[string]interface{}{"id": 1, "name": "Item 1"},
			map[string]interface{}{"id": 2, "name": "Item 2"},
		},
	}
	
	// Extract nested value
	result, err := fusion.extractJSONPath(data, "$.user.name")
	if err != nil {
		t.Fatalf("extractJSONPath failed: %v", err)
	}
	
	if result != "John Doe" {
		t.Errorf("Expected 'John Doe', got %v", result)
	}
	
	// Extract entire object
	userObj, err := fusion.extractJSONPath(data, "$.user")
	if err != nil {
		t.Fatalf("extractJSONPath failed: %v", err)
	}
	
	user := userObj.(map[string]interface{})
	if user["name"] != "John Doe" {
		t.Errorf("Expected user name 'John Doe', got %v", user["name"])
	}
	
	// Non-existent path
	_, err = fusion.extractJSONPath(data, "$.nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent path")
	}
}