/*=============================================================================
= Copyright (c) 2025 Tenebris Technologies Inc.                              =
= All rights reserved.                                                       =
=============================================================================*/

package fusion

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFusionIntegration_EndToEnd(t *testing.T) {
	// Create a mock API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users":
			if r.Method == "GET" {
				// Handle GET /users?limit=X
				limit := r.URL.Query().Get("limit")
				users := []map[string]interface{}{
					{"id": 1, "name": "John Doe", "email": "john@example.com"},
					{"id": 2, "name": "Jane Smith", "email": "jane@example.com"},
				}

				if limit == "1" {
					users = users[:1]
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"users": users,
					"total": len(users),
				})
			} else if r.Method == "POST" {
				// Handle POST /users
				var requestBody map[string]interface{}
				json.NewDecoder(r.Body).Decode(&requestBody)

				// Verify auth header
				authHeader := r.Header.Get("Authorization")
				if authHeader != "Bearer test-token" {
					w.WriteHeader(401)
					json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
					return
				}

				user := map[string]interface{}{
					"id":    3,
					"name":  requestBody["name"],
					"email": requestBody["email"],
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(201)
				json.NewEncoder(w).Encode(user)
			}

		case "/users/123":
			if r.Method == "GET" {
				// Handle GET /users/{id}
				user := map[string]interface{}{
					"id":    123,
					"name":  "Specific User",
					"email": "specific@example.com",
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(user)
			}

		default:
			w.WriteHeader(404)
			json.NewEncoder(w).Encode(map[string]string{"error": "Not Found"})
		}
	}))
	defer server.Close()

	// Create configuration
	configJSON := `{
		"services": {
			"testapi": {
				"name": "Test API",
				"baseURL": "` + server.URL + `",
				"auth": {
					"type": "bearer",
					"config": {
						"token": "test-token"
					}
				},
				"endpoints": [
					{
						"id": "list_users",
						"name": "List Users",
						"description": "Get a list of users",
						"method": "GET",
						"path": "/users",
						"parameters": [
							{
								"name": "limit",
								"description": "Maximum number of users to return",
								"type": "number",
								"required": false,
								"location": "query",
								"default": 10
							}
						],
						"response": {
							"type": "json",
							"transform": "$.users"
						}
					},
					{
						"id": "get_user",
						"name": "Get User",
						"description": "Get a specific user by ID",
						"method": "GET",
						"path": "/users/{id}",
						"parameters": [
							{
								"name": "id",
								"description": "User ID",
								"type": "string",
								"required": true,
								"location": "path"
							}
						],
						"response": {
							"type": "json"
						}
					},
					{
						"id": "create_user",
						"name": "Create User",
						"description": "Create a new user",
						"method": "POST",
						"path": "/users",
						"parameters": [
							{
								"name": "name",
								"description": "User name",
								"type": "string",
								"required": true,
								"location": "body",
								"validation": {
									"minLength": 2,
									"maxLength": 50
								}
							},
							{
								"name": "email",
								"description": "User email",
								"type": "string",
								"required": true,
								"location": "body",
								"validation": {
									"pattern": "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
								}
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

	// Create Fusion instance
	fusion := New(
		WithJSONConfigData([]byte(configJSON), "test-config.json"),
		WithLogger(&mockLogger{}),
	)

	// Verify configuration loaded
	if fusion.config == nil {
		t.Fatal("Configuration not loaded")
	}

	// Get the registered tools
	tools := fusion.RegisterTools()
	if len(tools) != 3 {
		t.Fatalf("Expected 3 tools, got %d", len(tools))
	}

	// Test each tool
	for _, tool := range tools {
		switch tool.Name {
		case "testapi_list_users":
			t.Run("list_users", func(t *testing.T) {
				// Test with default limit
				result, err := tool.Handler(map[string]any{})
				if err != nil {
					t.Fatalf("Tool execution failed: %v", err)
				}

				// Parse result
				var users []map[string]interface{}
				if err := json.Unmarshal([]byte(result), &users); err != nil {
					t.Fatalf("Failed to parse result: %v", err)
				}

				if len(users) != 2 {
					t.Errorf("Expected 2 users, got %d", len(users))
				}

				// Test with limit parameter
				result, err = tool.Handler(map[string]any{"limit": 1})
				if err != nil {
					t.Fatalf("Tool execution with limit failed: %v", err)
				}

				var limitedUsers []map[string]interface{}
				if err := json.Unmarshal([]byte(result), &limitedUsers); err != nil {
					t.Fatalf("Failed to parse limited result: %v", err)
				}

				if len(limitedUsers) != 1 {
					t.Errorf("Expected 1 user with limit, got %d", len(limitedUsers))
				}
			})

		case "testapi_get_user":
			t.Run("get_user", func(t *testing.T) {
				result, err := tool.Handler(map[string]any{"id": "123"})
				if err != nil {
					t.Fatalf("Tool execution failed: %v", err)
				}

				var user map[string]interface{}
				if err := json.Unmarshal([]byte(result), &user); err != nil {
					t.Fatalf("Failed to parse result: %v", err)
				}

				if user["id"].(float64) != 123 {
					t.Errorf("Expected user ID 123, got %v", user["id"])
				}

				if user["name"] != "Specific User" {
					t.Errorf("Expected user name 'Specific User', got %v", user["name"])
				}

				// Test missing required parameter
				_, err = tool.Handler(map[string]any{})
				if err == nil {
					t.Error("Expected error for missing required parameter")
				}
			})

		case "testapi_create_user":
			t.Run("create_user", func(t *testing.T) {
				result, err := tool.Handler(map[string]any{
					"name":  "New User",
					"email": "newuser@example.com",
				})
				if err != nil {
					t.Fatalf("Tool execution failed: %v", err)
				}

				var user map[string]interface{}
				if err := json.Unmarshal([]byte(result), &user); err != nil {
					t.Fatalf("Failed to parse result: %v", err)
				}

				if user["name"] != "New User" {
					t.Errorf("Expected name 'New User', got %v", user["name"])
				}

				if user["email"] != "newuser@example.com" {
					t.Errorf("Expected email 'newuser@example.com', got %v", user["email"])
				}

				// Test validation - name too short
				_, err = tool.Handler(map[string]any{
					"name":  "X",
					"email": "test@example.com",
				})
				if err == nil {
					t.Error("Expected validation error for short name")
				}

				// Test validation - invalid email
				_, err = tool.Handler(map[string]any{
					"name":  "Valid Name",
					"email": "invalid-email",
				})
				if err == nil {
					t.Error("Expected validation error for invalid email")
				}
			})
		}
	}
}

func TestFusionIntegration_AuthenticationError(t *testing.T) {
	// Create a mock API server that requires authentication
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer valid-token" {
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Success"})
	}))
	defer server.Close()

	// Create configuration with invalid token
	configJSON := `{
		"services": {
			"testapi": {
				"name": "Test API",
				"baseURL": "` + server.URL + `",
				"auth": {
					"type": "bearer",
					"config": {
						"token": "invalid-token"
					}
				},
				"endpoints": [
					{
						"id": "test_endpoint",
						"name": "Test Endpoint",
						"description": "Test endpoint",
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
		WithJSONConfigData([]byte(configJSON), "test-config.json"),
		WithLogger(&mockLogger{}),
	)

	tools := fusion.RegisterTools()
	if len(tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(tools))
	}

	// Execute the tool - should get an API error
	_, err := tools[0].Handler(map[string]any{})
	if err == nil {
		t.Fatal("Expected API error for invalid authentication")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Errorf("Expected APIError, got %T", err)
	} else if apiErr.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", apiErr.StatusCode)
	}
}

func TestFusionIntegration_NetworkError(t *testing.T) {
	// Create configuration with invalid URL
	configJSON := `{
		"services": {
			"testapi": {
				"name": "Test API",
				"baseURL": "http://invalid-host-that-does-not-exist.local",
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
						"description": "Test endpoint",
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
		WithJSONConfigData([]byte(configJSON), "test-config.json"),
		WithLogger(&mockLogger{}),
	)

	tools := fusion.RegisterTools()
	if len(tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(tools))
	}

	// Execute the tool - should get a network error
	_, err := tools[0].Handler(map[string]any{})
	if err == nil {
		t.Fatal("Expected network error for invalid host")
	}

	networkErr, ok := err.(*NetworkError)
	if !ok {
		t.Errorf("Expected NetworkError, got %T: %v", err, err)
	} else if !networkErr.Retryable {
		t.Error("Expected network error to be retryable")
	}
}
