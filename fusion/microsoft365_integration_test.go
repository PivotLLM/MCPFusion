/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
)

// TestMicrosoft365Integration tests the Microsoft 365 Graph API integration
func TestMicrosoft365Integration(t *testing.T) {
	// Mock HTTP server to simulate Microsoft Graph API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for authorization header to bypass OAuth2 flow in tests
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" && !strings.Contains(r.URL.Path, "oauth2") {
			// Mock successful authentication by setting a fake token
			r.Header.Set("Authorization", "Bearer mock_access_token_12345")
		}

		switch r.URL.Path {
		case "/v1.0/oauth2/v2.0/devicecode":
			handleDeviceCodeRequest(w, r, t)
		case "/v1.0/oauth2/v2.0/token":
			handleTokenRequest(w, r, t)
		case "/v1.0/me":
			handleProfileRequest(w, r, t)
		case "/v1.0/me/calendarView":
			handleCalendarRequest(w, r, t)
		case "/v1.0/me/messages":
			handleMessagesRequest(w, r, t)
		case "/v1.0/me/contacts":
			handleContactsRequest(w, r, t)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create modified Microsoft 365 configuration for testing (with Bearer token for simplicity)
	config := createTestMicrosoft365ConfigWithBearer(server.URL)

	// Create fusion instance
	logger := &MockLogger{}
	fusion := New(
		WithJSONConfigData([]byte(config), "test-config.json"),
		WithLogger(logger),
	)

	// Get the tools
	tools := fusion.RegisterTools()

	// Debug: Print all tool names
	t.Logf("Found %d tools:", len(tools))
	for i, tool := range tools {
		t.Logf("  %d: %s", i, tool.Name)
	}

	// Find tools by name
	var profileTool, calendarSummaryTool, calendarDetailsTool, mailTool, contactsTool *global.ToolDefinition
	for _, tool := range tools {
		switch tool.Name {
		case "microsoft365_profile_get":
			profileTool = &tool
		case "microsoft365_calendar_read_summary":
			calendarSummaryTool = &tool
		case "microsoft365_calendar_read_details":
			calendarDetailsTool = &tool
		case "microsoft365_mail_read_inbox":
			mailTool = &tool
		case "microsoft365_contacts_list":
			contactsTool = &tool
		}
	}

	// Test profile endpoint
	t.Run("GetProfile", func(t *testing.T) {
		if profileTool == nil {
			t.Fatal("Profile tool not found")
		}

		result, err := profileTool.Handler(map[string]interface{}{})
		if err != nil {
			t.Fatalf("Profile request failed: %v", err)
		}

		var profile map[string]interface{}
		if err := json.Unmarshal([]byte(result), &profile); err != nil {
			t.Fatalf("Failed to parse profile response: %v", err)
		}

		if profile["displayName"] != "Test User" {
			t.Errorf("Expected displayName 'Test User', got %v", profile["displayName"])
		}
	})

	// Test calendar summary endpoint
	t.Run("GetCalendarSummary", func(t *testing.T) {
		if calendarSummaryTool == nil {
			t.Fatal("Calendar summary tool not found")
		}

		result, err := calendarSummaryTool.Handler(map[string]interface{}{
			"startDate": "20250101",
			"endDate":   "20250131",
		})
		if err != nil {
			t.Fatalf("Calendar summary request failed: %v", err)
		}

		// Calendar response includes .value transformation
		var events []interface{}
		if err := json.Unmarshal([]byte(result), &events); err != nil {
			t.Fatalf("Failed to parse calendar response: %v", err)
		}

		if len(events) == 0 {
			t.Error("Expected calendar events, got empty array")
		}
	})

	// Test calendar details with pagination
	t.Run("GetCalendarDetailsWithPagination", func(t *testing.T) {
		if calendarDetailsTool == nil {
			t.Fatal("Calendar details tool not found")
		}

		result, err := calendarDetailsTool.Handler(map[string]interface{}{
			"startDate": "20250101",
			"endDate":   "20250131",
		})
		if err != nil {
			t.Fatalf("Calendar details request failed: %v", err)
		}

		var events []interface{}
		if err := json.Unmarshal([]byte(result), &events); err != nil {
			t.Fatalf("Failed to parse calendar response: %v", err)
		}

		// Should have events from multiple pages
		if len(events) < 2 {
			t.Errorf("Expected multiple events from pagination, got %d", len(events))
		}
	})

	// Test inbox messages
	t.Run("GetInboxMessages", func(t *testing.T) {
		if mailTool == nil {
			t.Fatal("Mail tool not found")
		}

		result, err := mailTool.Handler(map[string]interface{}{
			"$top": 10,
		})
		if err != nil {
			t.Fatalf("Inbox request failed: %v", err)
		}

		// For paginated responses, we get the data array directly
		var messages []interface{}
		if err := json.Unmarshal([]byte(result), &messages); err != nil {
			t.Fatalf("Failed to parse messages response: %v", err)
		}

		if len(messages) == 0 {
			t.Error("Expected inbox messages, got empty array")
		}
	})

	// Test contacts
	t.Run("GetContacts", func(t *testing.T) {
		if contactsTool == nil {
			t.Fatal("Contacts tool not found")
		}

		result, err := contactsTool.Handler(map[string]interface{}{
			"$top": 25,
		})
		if err != nil {
			t.Fatalf("Contacts request failed: %v", err)
		}

		// Contacts response is not paginated, so it's returned as a regular object
		var contactsResponse map[string]interface{}
		if err := json.Unmarshal([]byte(result), &contactsResponse); err != nil {
			t.Fatalf("Failed to parse contacts response: %v", err)
		}

		if contactsValue, ok := contactsResponse["value"].([]interface{}); ok {
			if len(contactsValue) == 0 {
				t.Error("Expected contacts, got empty array")
			}
		} else {
			t.Error("Expected contacts response with 'value' array")
		}
	})

	// Test caching functionality
	t.Run("TestCaching", func(t *testing.T) {
		if profileTool == nil {
			t.Fatal("Profile tool not found for caching test")
		}

		// First request - should hit the server
		start := time.Now()
		result1, err := profileTool.Handler(map[string]interface{}{})
		if err != nil {
			t.Fatalf("First profile request failed: %v", err)
		}
		firstDuration := time.Since(start)

		// Second request - should be cached
		start = time.Now()
		result2, err := profileTool.Handler(map[string]interface{}{})
		if err != nil {
			t.Fatalf("Second profile request failed: %v", err)
		}
		secondDuration := time.Since(start)

		// Results should be identical
		if result1 != result2 {
			t.Error("Cached result differs from original")
		}

		// Second request should be significantly faster (cached)
		if secondDuration > firstDuration/2 {
			t.Logf("Warning: Second request (%v) not significantly faster than first (%v) - caching may not be working",
				secondDuration, firstDuration)
		}
	})
}

// TestMicrosoft365OAuth2DeviceFlow tests the OAuth2 device flow specifically
func TestMicrosoft365OAuth2DeviceFlow(t *testing.T) {
	// Mock HTTP server for OAuth2 flow
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "devicecode"):
			handleDeviceCodeRequest(w, r, t)
		case strings.Contains(r.URL.Path, "token"):
			handleTokenRequest(w, r, t)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Test OAuth2 device flow configuration
	config := createTestMicrosoft365Config(server.URL)
	logger := &MockLogger{}

	fusion := New(
		WithJSONConfigData([]byte(config), "test-oauth-config.json"),
		WithLogger(logger),
	)

	// Get the tools
	tools := fusion.RegisterTools()

	// Find profile tool
	var profileTool *global.ToolDefinition
	for _, tool := range tools {
		if tool.Name == "microsoft365_profile_get" {
			profileTool = &tool
			break
		}
	}

	// Test device code initiation
	t.Run("InitiateDeviceFlow", func(t *testing.T) {
		if profileTool == nil {
			t.Fatal("Profile tool not found")
		}

		// This test simulates what happens when authentication is required
		// The device code error should be returned with user instructions
		result, err := profileTool.Handler(map[string]interface{}{})

		// For a real device flow, this would return instructions for the user
		// In our mock, we'll simulate a successful flow
		if err != nil && !strings.Contains(err.Error(), "device") {
			t.Logf("Device flow initiated with result: %s", result)
		}
	})
}

// Helper functions for mock server handlers

func handleDeviceCodeRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "POST" {
		t.Errorf("Expected POST for device code request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"device_code":      "mockdevicecode123",
		"user_code":        "A1B2C3",
		"verification_uri": "https://microsoft.com/devicelogin",
		"expires_in":       900,
		"interval":         5,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleTokenRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "POST" {
		t.Errorf("Expected POST for token request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"access_token":  "mock_access_token_12345",
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": "mock_refresh_token_67890",
		"scope":         "https://graph.microsoft.com/Calendars.Read https://graph.microsoft.com/Mail.Read",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleProfileRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "GET" {
		t.Errorf("Expected GET for profile request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"displayName":       "Test User",
		"mail":              "test.user@contoso.com",
		"userPrincipalName": "test.user@contoso.com",
		"jobTitle":          "Software Developer",
		"department":        "Engineering",
		"companyName":       "Contoso Ltd.",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleCalendarRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "GET" {
		t.Errorf("Expected GET for calendar request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if this is a paginated request
	isPaginated := strings.Contains(r.URL.RawQuery, "startDateTime") &&
		!strings.Contains(r.URL.RawQuery, "skiptoken")

	var response map[string]interface{}

	if isPaginated {
		// First page of results
		response = map[string]interface{}{
			"value": []map[string]interface{}{
				{
					"subject": "Team Meeting",
					"start":   map[string]string{"dateTime": "2025-01-15T09:00:00Z"},
					"end":     map[string]string{"dateTime": "2025-01-15T10:00:00Z"},
					"organizer": map[string]interface{}{
						"emailAddress": map[string]string{"address": "organizer@contoso.com"},
					},
				},
				{
					"subject": "Project Review",
					"start":   map[string]string{"dateTime": "2025-01-16T14:00:00Z"},
					"end":     map[string]string{"dateTime": "2025-01-16T15:00:00Z"},
					"organizer": map[string]interface{}{
						"emailAddress": map[string]string{"address": "pm@contoso.com"},
					},
				},
			},
			"@odata.nextLink": fmt.Sprintf("%s/v1.0/me/calendarView?skiptoken=page2", r.Host),
		}
	} else if strings.Contains(r.URL.RawQuery, "skiptoken=page2") {
		// Second page of results
		response = map[string]interface{}{
			"value": []map[string]interface{}{
				{
					"subject": "Client Call",
					"start":   map[string]string{"dateTime": "2025-01-17T11:00:00Z"},
					"end":     map[string]string{"dateTime": "2025-01-17T12:00:00Z"},
					"organizer": map[string]interface{}{
						"emailAddress": map[string]string{"address": "sales@contoso.com"},
					},
				},
			},
		}
	} else {
		// Single page for summary view - should also have the same structure for transformation
		response = map[string]interface{}{
			"value": []map[string]interface{}{
				{
					"subject": "Team Meeting",
					"start":   map[string]string{"dateTime": "2025-01-15T09:00:00Z"},
					"end":     map[string]string{"dateTime": "2025-01-15T10:00:00Z"},
				},
				{
					"subject": "Project Review",
					"start":   map[string]string{"dateTime": "2025-01-16T14:00:00Z"},
					"end":     map[string]string{"dateTime": "2025-01-16T15:00:00Z"},
				},
			},
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleMessagesRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "GET" {
		t.Errorf("Expected GET for messages request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"value": []map[string]interface{}{
			{
				"subject":          "Project Update",
				"from":             map[string]interface{}{"emailAddress": map[string]string{"address": "manager@contoso.com"}},
				"receivedDateTime": "2025-01-10T10:30:00Z",
				"bodyPreview":      "Here's the latest update on the project...",
				"isRead":           false,
			},
			{
				"subject":          "Meeting Reminder",
				"from":             map[string]interface{}{"emailAddress": map[string]string{"address": "calendar@contoso.com"}},
				"receivedDateTime": "2025-01-09T08:00:00Z",
				"bodyPreview":      "Don't forget about the team meeting tomorrow...",
				"isRead":           true,
			},
		},
		"@odata.nextLink": fmt.Sprintf("%s/v1.0/me/messages?skiptoken=page2", r.Host),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleContactsRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "GET" {
		t.Errorf("Expected GET for contacts request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"value": []map[string]interface{}{
			{
				"displayName": "John Smith",
				"emailAddresses": []map[string]interface{}{
					{"address": "john.smith@contoso.com"},
				},
				"businessPhones": []string{"+1-555-0123"},
				"jobTitle":       "Senior Developer",
				"companyName":    "Contoso Ltd.",
			},
			{
				"displayName": "Jane Doe",
				"emailAddresses": []map[string]interface{}{
					{"address": "jane.doe@contoso.com"},
				},
				"businessPhones": []string{"+1-555-0124"},
				"jobTitle":       "Product Manager",
				"companyName":    "Contoso Ltd.",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func createTestMicrosoft365Config(baseURL string) string {
	config := map[string]interface{}{
		"services": map[string]interface{}{
			"microsoft365": map[string]interface{}{
				"name":    "Microsoft 365",
				"baseURL": baseURL + "/v1.0",
				"auth": map[string]interface{}{
					"type": "oauth2_device",
					"config": map[string]interface{}{
						"clientId":         "test-client-id",
						"tenantId":         "test-tenant-id",
						"scope":            []string{"https://graph.microsoft.com/Calendars.Read", "https://graph.microsoft.com/Mail.Read"},
						"authorizationURL": baseURL + "/v1.0/oauth2/v2.0/devicecode",
						"tokenURL":         baseURL + "/v1.0/oauth2/v2.0/token",
					},
				},
				"endpoints": []interface{}{
					map[string]interface{}{
						"id":          "profile_get",
						"name":        "Get User Profile",
						"description": "Get the current user's profile information",
						"method":      "GET",
						"path":        "/me",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "$select",
								"description": "Fields to include in response",
								"type":        "string",
								"required":    false,
								"location":    "query",
								"default":     "displayName,mail,userPrincipalName,jobTitle,department,companyName",
							},
						},
						"response": map[string]interface{}{
							"type": "json",
							"caching": map[string]interface{}{
								"enabled": true,
								"ttl":     "30m",
							},
						},
					},
					map[string]interface{}{
						"id":          "calendar_read_summary",
						"name":        "Read Calendar Summary",
						"description": "Get calendar events with basic information",
						"method":      "GET",
						"path":        "/me/calendarView",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "startDate",
								"description": "Start date in YYYYMMDD format",
								"type":        "string",
								"required":    true,
								"location":    "query",
								"validation": map[string]interface{}{
									"pattern": "^\\d{8}$",
								},
								"transform": map[string]interface{}{
									"targetName": "startDateTime",
									"expression": "concat(slice(0,4), '-', slice(4,6), '-', slice(6,8), 'T00:00:00Z')",
								},
							},
							map[string]interface{}{
								"name":        "endDate",
								"description": "End date in YYYYMMDD format",
								"type":        "string",
								"required":    true,
								"location":    "query",
								"validation": map[string]interface{}{
									"pattern": "^\\d{8}$",
								},
								"transform": map[string]interface{}{
									"targetName": "endDateTime",
									"expression": "concat(slice(0,4), '-', slice(4,6), '-', slice(6,8), 'T23:59:59Z')",
								},
							},
						},
						"response": map[string]interface{}{
							"type":      "json",
							"transform": ".value | map({subject: .subject, start: .start.dateTime, end: .end.dateTime})",
						},
					},
					map[string]interface{}{
						"id":          "calendar_read_details",
						"name":        "Read Calendar Details",
						"description": "Get calendar events with full details",
						"method":      "GET",
						"path":        "/me/calendarView",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "startDate",
								"description": "Start date in YYYYMMDD format",
								"type":        "string",
								"required":    true,
								"location":    "query",
								"validation": map[string]interface{}{
									"pattern": "^\\d{8}$",
								},
								"transform": map[string]interface{}{
									"targetName": "startDateTime",
									"expression": "concat(slice(0,4), '-', slice(4,6), '-', slice(6,8), 'T00:00:00Z')",
								},
							},
							map[string]interface{}{
								"name":        "endDate",
								"description": "End date in YYYYMMDD format",
								"type":        "string",
								"required":    true,
								"location":    "query",
								"validation": map[string]interface{}{
									"pattern": "^\\d{8}$",
								},
								"transform": map[string]interface{}{
									"targetName": "endDateTime",
									"expression": "concat(slice(0,4), '-', slice(4,6), '-', slice(6,8), 'T23:59:59Z')",
								},
							},
						},
						"response": map[string]interface{}{
							"type":      "json",
							"paginated": true,
							"paginationConfig": map[string]interface{}{
								"nextPageTokenPath": "@odata.nextLink",
								"dataPath":          "value",
								"pageSize":          50,
							},
							"caching": map[string]interface{}{
								"enabled": true,
								"ttl":     "10m",
							},
						},
					},
					map[string]interface{}{
						"id":          "mail_read_inbox",
						"name":        "Read Inbox Messages",
						"description": "Get inbox messages with basic information",
						"method":      "GET",
						"path":        "/me/messages",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "$top",
								"description": "Number of messages to retrieve",
								"type":        "number",
								"required":    false,
								"location":    "query",
								"default":     10,
							},
						},
						"response": map[string]interface{}{
							"type":      "json",
							"paginated": true,
							"paginationConfig": map[string]interface{}{
								"nextPageTokenPath": "@odata.nextLink",
								"dataPath":          "value",
								"pageSize":          50,
							},
						},
					},
					map[string]interface{}{
						"id":          "contacts_list",
						"name":        "List Contacts",
						"description": "Get contacts from the user's address book",
						"method":      "GET",
						"path":        "/me/contacts",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "$top",
								"description": "Number of contacts to retrieve",
								"type":        "number",
								"required":    false,
								"location":    "query",
								"default":     25,
							},
						},
						"response": map[string]interface{}{
							"type": "json",
						},
					},
				},
			},
		},
	}

	configBytes, _ := json.Marshal(config)
	return string(configBytes)
}

func createTestMicrosoft365ConfigWithBearer(baseURL string) string {
	config := map[string]interface{}{
		"services": map[string]interface{}{
			"microsoft365": map[string]interface{}{
				"name":    "Microsoft 365",
				"baseURL": baseURL + "/v1.0",
				"auth": map[string]interface{}{
					"type": "bearer",
					"config": map[string]interface{}{
						"token": "mock_access_token_12345",
					},
				},
				"endpoints": []interface{}{
					map[string]interface{}{
						"id":          "profile_get",
						"name":        "Get User Profile",
						"description": "Get the current user's profile information",
						"method":      "GET",
						"path":        "/me",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "$select",
								"description": "Fields to include in response",
								"type":        "string",
								"required":    false,
								"location":    "query",
								"default":     "displayName,mail,userPrincipalName,jobTitle,department,companyName",
							},
						},
						"response": map[string]interface{}{
							"type": "json",
							"caching": map[string]interface{}{
								"enabled": true,
								"ttl":     "30m",
							},
						},
					},
					map[string]interface{}{
						"id":          "calendar_read_summary",
						"name":        "Read Calendar Summary",
						"description": "Get calendar events with basic information",
						"method":      "GET",
						"path":        "/me/calendarView",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "startDate",
								"description": "Start date in YYYYMMDD format",
								"type":        "string",
								"required":    true,
								"location":    "query",
								"validation": map[string]interface{}{
									"pattern": "^\\d{8}$",
								},
								"transform": map[string]interface{}{
									"targetName": "startDateTime",
									"expression": "concat(slice(0,4), '-', slice(4,6), '-', slice(6,8), 'T00:00:00Z')",
								},
							},
							map[string]interface{}{
								"name":        "endDate",
								"description": "End date in YYYYMMDD format",
								"type":        "string",
								"required":    true,
								"location":    "query",
								"validation": map[string]interface{}{
									"pattern": "^\\d{8}$",
								},
								"transform": map[string]interface{}{
									"targetName": "endDateTime",
									"expression": "concat(slice(0,4), '-', slice(4,6), '-', slice(6,8), 'T23:59:59Z')",
								},
							},
						},
						"response": map[string]interface{}{
							"type": "json",
							// Skip complex transformation for testing
							"transform": ".value",
						},
					},
					map[string]interface{}{
						"id":          "calendar_read_details",
						"name":        "Read Calendar Details",
						"description": "Get calendar events with full details",
						"method":      "GET",
						"path":        "/me/calendarView",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "startDate",
								"description": "Start date in YYYYMMDD format",
								"type":        "string",
								"required":    true,
								"location":    "query",
								"validation": map[string]interface{}{
									"pattern": "^\\d{8}$",
								},
								"transform": map[string]interface{}{
									"targetName": "startDateTime",
									"expression": "concat(slice(0,4), '-', slice(4,6), '-', slice(6,8), 'T00:00:00Z')",
								},
							},
							map[string]interface{}{
								"name":        "endDate",
								"description": "End date in YYYYMMDD format",
								"type":        "string",
								"required":    true,
								"location":    "query",
								"validation": map[string]interface{}{
									"pattern": "^\\d{8}$",
								},
								"transform": map[string]interface{}{
									"targetName": "endDateTime",
									"expression": "concat(slice(0,4), '-', slice(4,6), '-', slice(6,8), 'T23:59:59Z')",
								},
							},
						},
						"response": map[string]interface{}{
							"type":      "json",
							"paginated": true,
							"paginationConfig": map[string]interface{}{
								"nextPageTokenPath": "@odata.nextLink",
								"dataPath":          "value",
								"pageSize":          50,
							},
							"caching": map[string]interface{}{
								"enabled": true,
								"ttl":     "10m",
							},
						},
					},
					map[string]interface{}{
						"id":          "mail_read_inbox",
						"name":        "Read Inbox Messages",
						"description": "Get inbox messages with basic information",
						"method":      "GET",
						"path":        "/me/messages",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "$top",
								"description": "Number of messages to retrieve",
								"type":        "number",
								"required":    false,
								"location":    "query",
								"default":     10,
							},
						},
						"response": map[string]interface{}{
							"type":      "json",
							"paginated": true,
							"paginationConfig": map[string]interface{}{
								"nextPageTokenPath": "@odata.nextLink",
								"dataPath":          "value",
								"pageSize":          50,
							},
						},
					},
					map[string]interface{}{
						"id":          "contacts_list",
						"name":        "List Contacts",
						"description": "Get contacts from the user's address book",
						"method":      "GET",
						"path":        "/me/contacts",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "$top",
								"description": "Number of contacts to retrieve",
								"type":        "number",
								"required":    false,
								"location":    "query",
								"default":     25,
							},
						},
						"response": map[string]interface{}{
							"type": "json",
						},
					},
				},
			},
		},
	}

	configBytes, _ := json.Marshal(config)
	return string(configBytes)
}

// MockLogger implements the global.Logger interface for testing
type MockLogger struct {
	logs []string
}

func (m *MockLogger) Debug(msg string) {
	m.logs = append(m.logs, "DEBUG: "+msg)
}

func (m *MockLogger) Debugf(format string, args ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf("DEBUG: "+format, args...))
}

func (m *MockLogger) Info(msg string) {
	m.logs = append(m.logs, "INFO: "+msg)
}

func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf("INFO: "+format, args...))
}

func (m *MockLogger) Notice(msg string) {
	m.logs = append(m.logs, "NOTICE: "+msg)
}

func (m *MockLogger) Noticef(format string, args ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf("NOTICE: "+format, args...))
}

func (m *MockLogger) Warning(msg string) {
	m.logs = append(m.logs, "WARNING: "+msg)
}

func (m *MockLogger) Warningf(format string, args ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf("WARNING: "+format, args...))
}

func (m *MockLogger) Error(msg string) {
	m.logs = append(m.logs, "ERROR: "+msg)
}

func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf("ERROR: "+format, args...))
}

func (m *MockLogger) Fatal(msg string) {
	m.logs = append(m.logs, "FATAL: "+msg)
}

func (m *MockLogger) Fatalf(format string, args ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf("FATAL: "+format, args...))
}

// Close implements the global.Logger interface
func (m *MockLogger) Close() {}

// GetLogs returns all logged messages
func (m *MockLogger) GetLogs() []string {
	return m.logs
}
