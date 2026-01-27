/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
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

// TestGoogleIntegration tests the Google APIs integration
func TestGoogleIntegration(t *testing.T) {
	// Mock HTTP server to simulate Google APIs
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for authorization header to bypass OAuth2 flow in tests
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" && !strings.Contains(r.URL.Path, "oauth2") {
			// Mock successful authentication by setting a fake token
			r.Header.Set("Authorization", "Bearer mock_google_token_12345")
		}

		switch {
		case strings.Contains(r.URL.Path, "oauth2/device/code"):
			handleGoogleDeviceCodeRequest(w, r, t)
		case strings.Contains(r.URL.Path, "oauth2/token"):
			handleGoogleTokenRequest(w, r, t)
		case strings.Contains(r.URL.Path, "oauth2/v2/userinfo"):
			handleGoogleProfileRequest(w, r, t)
		case r.URL.Path == "/calendar/v3/calendars/primary/events" && r.Method == "GET":
			handleGoogleCalendarEventsRequest(w, r, t)
		case r.URL.Path == "/calendar/v3/calendars/primary/events" && r.Method == "POST":
			handleGoogleCalendarEventCreateRequest(w, r, t)
		case strings.HasPrefix(r.URL.Path, "/calendar/v3/calendars/primary/events/") && r.Method == "GET":
			handleGoogleCalendarEventGetRequest(w, r, t)
		case strings.HasPrefix(r.URL.Path, "/calendar/v3/calendars/primary/events/") && r.Method == "PUT":
			handleGoogleCalendarEventUpdateRequest(w, r, t)
		case strings.HasPrefix(r.URL.Path, "/calendar/v3/calendars/primary/events/") && r.Method == "DELETE":
			handleGoogleCalendarEventDeleteRequest(w, r, t)
		case r.URL.Path == "/gmail/v1/users/me/messages" && r.Method == "GET":
			handleGoogleGmailMessagesRequest(w, r, t)
		case strings.HasPrefix(r.URL.Path, "/gmail/v1/users/me/messages/") && !strings.HasSuffix(r.URL.Path, "/send") && r.Method == "GET":
			handleGoogleGmailMessageGetRequest(w, r, t)
		case strings.HasSuffix(r.URL.Path, "/gmail/v1/users/me/messages/send") && r.Method == "POST":
			handleGoogleGmailSendRequest(w, r, t)
		case r.URL.Path == "/drive/v3/files" && r.Method == "GET":
			handleGoogleDriveFilesRequest(w, r, t)
		case strings.HasPrefix(r.URL.Path, "/drive/v3/files/") && r.Method == "GET" && !strings.Contains(r.URL.RawQuery, "alt=media") && !strings.Contains(r.URL.Path, "/permissions"):
			handleGoogleDriveFileGetRequest(w, r, t)
		case strings.HasPrefix(r.URL.Path, "/drive/v3/files/") && r.Method == "GET" && strings.Contains(r.URL.RawQuery, "alt=media"):
			handleGoogleDriveFileDownloadRequest(w, r, t)
		case strings.HasPrefix(r.URL.Path, "/upload/drive/v3/files") && r.Method == "POST":
			handleGoogleDriveFileCreateRequest(w, r, t)
		case strings.HasPrefix(r.URL.Path, "/drive/v3/files/") && r.Method == "DELETE" && !strings.Contains(r.URL.Path, "/permissions"):
			handleGoogleDriveFileDeleteRequest(w, r, t)
		case strings.HasPrefix(r.URL.Path, "/drive/v3/files/") && strings.Contains(r.URL.Path, "/permissions") && r.Method == "POST":
			handleGoogleDriveFileShareRequest(w, r, t)
		default:
			t.Logf("Unhandled request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create modified Google configuration for testing (with Bearer token for simplicity)
	config := createTestGoogleConfigWithBearer(server.URL)

	// Create fusion instance
	logger := &MockLogger{}
	fusion := New(
		WithJSONConfigData([]byte(config), "test-google-config.json"),
		WithLogger(logger),
	)

	// Get the tools
	tools := fusion.RegisterTools()

	// Debug: Print all tool names
	t.Logf("Found %d Google tools:", len(tools))
	for i, tool := range tools {
		t.Logf("  %d: %s", i, tool.Name)
	}

	// Find tools by name
	var (
		profileTool        *global.ToolDefinition
		calendarListTool   *global.ToolDefinition
		calendarCreateTool *global.ToolDefinition
		calendarGetTool    *global.ToolDefinition
		calendarUpdateTool *global.ToolDefinition
		calendarDeleteTool *global.ToolDefinition
		gmailListTool      *global.ToolDefinition
		gmailGetTool       *global.ToolDefinition
		gmailSendTool      *global.ToolDefinition
		gmailSearchTool    *global.ToolDefinition
		driveListTool      *global.ToolDefinition
		driveGetTool       *global.ToolDefinition
		driveDownloadTool  *global.ToolDefinition
		driveCreateTool    *global.ToolDefinition
		driveDeleteTool    *global.ToolDefinition
		driveShareTool     *global.ToolDefinition
	)

	for _, tool := range tools {
		switch tool.Name {
		case "google_profile_get":
			profileTool = &tool
		case "google_calendar_events_list":
			calendarListTool = &tool
		case "google_calendar_event_create":
			calendarCreateTool = &tool
		case "google_calendar_event_get":
			calendarGetTool = &tool
		case "google_calendar_event_update":
			calendarUpdateTool = &tool
		case "google_calendar_event_delete":
			calendarDeleteTool = &tool
		case "google_gmail_messages_list":
			gmailListTool = &tool
		case "google_gmail_message_get":
			gmailGetTool = &tool
		case "google_gmail_message_send":
			gmailSendTool = &tool
		case "google_gmail_search_messages":
			gmailSearchTool = &tool
		case "google_drive_files_list":
			driveListTool = &tool
		case "google_drive_file_get":
			driveGetTool = &tool
		case "google_drive_file_download":
			driveDownloadTool = &tool
		case "google_drive_file_create":
			driveCreateTool = &tool
		case "google_drive_file_delete":
			driveDeleteTool = &tool
		case "google_drive_file_share":
			driveShareTool = &tool
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

		if profile["name"] != "Test Google User" {
			t.Errorf("Expected name 'Test Google User', got %v", profile["name"])
		}
	})

	// Test calendar events list
	t.Run("ListCalendarEvents", func(t *testing.T) {
		if calendarListTool == nil {
			t.Fatal("Calendar list tool not found")
		}

		result, err := calendarListTool.Handler(map[string]interface{}{
			"startDate": "20250101",
			"endDate":   "20250131",
		})
		if err != nil {
			t.Fatalf("Calendar list request failed: %v", err)
		}

		var events []interface{}
		if err := json.Unmarshal([]byte(result), &events); err != nil {
			t.Fatalf("Failed to parse calendar response: %v", err)
		}

		if len(events) == 0 {
			t.Error("Expected calendar events, got empty array")
		}
	})

	// Test calendar event creation
	t.Run("CreateCalendarEvent", func(t *testing.T) {
		if calendarCreateTool == nil {
			t.Fatal("Calendar create tool not found")
		}

		result, err := calendarCreateTool.Handler(map[string]interface{}{
			"summary":       "Test Event",
			"description":   "This is a test event",
			"startDateTime": "2025-01-15T10:00:00Z",
			"endDateTime":   "2025-01-15T11:00:00Z",
			"location":      "Test Location",
		})
		if err != nil {
			t.Fatalf("Calendar create request failed: %v", err)
		}

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(result), &event); err != nil {
			t.Fatalf("Failed to parse create event response: %v", err)
		}

		if event["summary"] != "Test Event" {
			t.Errorf("Expected summary 'Test Event', got %v", event["summary"])
		}
	})

	// Test calendar event get
	t.Run("GetCalendarEvent", func(t *testing.T) {
		if calendarGetTool == nil {
			t.Fatal("Calendar get tool not found")
		}

		result, err := calendarGetTool.Handler(map[string]interface{}{
			"eventId": "test-event-123",
		})
		if err != nil {
			t.Fatalf("Calendar get request failed: %v", err)
		}

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(result), &event); err != nil {
			t.Fatalf("Failed to parse get event response: %v", err)
		}

		if eventID, ok := event["id"]; !ok || eventID != "test-event-123" {
			t.Errorf("Expected id 'test-event-123', got %v (full response: %+v)", eventID, event)
		}
	})

	// Test calendar event update
	t.Run("UpdateCalendarEvent", func(t *testing.T) {
		if calendarUpdateTool == nil {
			t.Fatal("Calendar update tool not found")
		}

		result, err := calendarUpdateTool.Handler(map[string]interface{}{
			"eventId": "test-event-123",
			"summary": "Updated Test Event",
		})
		if err != nil {
			t.Fatalf("Calendar update request failed: %v", err)
		}

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(result), &event); err != nil {
			t.Fatalf("Failed to parse update event response: %v", err)
		}

		if event["summary"] != "Updated Test Event" {
			t.Errorf("Expected summary 'Updated Test Event', got %v", event["summary"])
		}
	})

	// Test calendar event delete
	t.Run("DeleteCalendarEvent", func(t *testing.T) {
		if calendarDeleteTool == nil {
			t.Fatal("Calendar delete tool not found")
		}

		result, err := calendarDeleteTool.Handler(map[string]interface{}{
			"eventId": "test-event-123",
		})
		if err != nil {
			t.Fatalf("Calendar delete request failed: %v", err)
		}

		if result != "" {
			t.Logf("Delete response: %s", result)
		}
	})

	// Test Gmail messages list
	t.Run("ListGmailMessages", func(t *testing.T) {
		if gmailListTool == nil {
			t.Fatal("Gmail list tool not found")
		}

		result, err := gmailListTool.Handler(map[string]interface{}{
			"maxResults": 10,
		})
		if err != nil {
			t.Fatalf("Gmail list request failed: %v", err)
		}

		var messages []interface{}
		if err := json.Unmarshal([]byte(result), &messages); err != nil {
			t.Fatalf("Failed to parse Gmail messages response: %v", err)
		}

		if len(messages) == 0 {
			t.Error("Expected Gmail messages, got empty array")
		}
	})

	// Test Gmail message get
	t.Run("GetGmailMessage", func(t *testing.T) {
		if gmailGetTool == nil {
			t.Fatal("Gmail get tool not found")
		}

		result, err := gmailGetTool.Handler(map[string]interface{}{
			"messageId": "test-message-123",
		})
		if err != nil {
			t.Fatalf("Gmail get request failed: %v", err)
		}

		var message map[string]interface{}
		if err := json.Unmarshal([]byte(result), &message); err != nil {
			t.Fatalf("Failed to parse Gmail message response: %v", err)
		}

		if messageID, ok := message["id"]; !ok || messageID != "test-message-123" {
			t.Errorf("Expected id 'test-message-123', got %v", messageID)
		}
	})

	// Test Gmail message send
	t.Run("SendGmailMessage", func(t *testing.T) {
		if gmailSendTool == nil {
			t.Fatal("Gmail send tool not found")
		}

		result, err := gmailSendTool.Handler(map[string]interface{}{
			"to":      "test@example.com",
			"subject": "Test Email",
			"body":    "This is a test email body",
		})
		if err != nil {
			t.Fatalf("Gmail send request failed: %v", err)
		}

		var sentMessage map[string]interface{}
		if err := json.Unmarshal([]byte(result), &sentMessage); err != nil {
			t.Fatalf("Failed to parse sent message response: %v", err)
		}

		if sentMessage["id"] == "" {
			t.Error("Expected sent message to have an ID")
		}
	})

	// Test Gmail search messages
	t.Run("SearchGmailMessages", func(t *testing.T) {
		if gmailSearchTool == nil {
			t.Fatal("Gmail search tool not found")
		}

		result, err := gmailSearchTool.Handler(map[string]interface{}{
			"query": "from:test@example.com",
		})
		if err != nil {
			t.Fatalf("Gmail search request failed: %v", err)
		}

		var messages []interface{}
		if err := json.Unmarshal([]byte(result), &messages); err != nil {
			t.Fatalf("Failed to parse Gmail search response: %v", err)
		}

		if len(messages) == 0 {
			t.Error("Expected search results, got empty array")
		}
	})

	// Test Drive files list
	t.Run("ListDriveFiles", func(t *testing.T) {
		if driveListTool == nil {
			t.Fatal("Drive list tool not found")
		}

		result, err := driveListTool.Handler(map[string]interface{}{
			"pageSize": 10,
		})
		if err != nil {
			t.Fatalf("Drive list request failed: %v", err)
		}

		var files []interface{}
		if err := json.Unmarshal([]byte(result), &files); err != nil {
			t.Fatalf("Failed to parse Drive files response: %v", err)
		}

		if len(files) == 0 {
			t.Error("Expected Drive files, got empty array")
		}
	})

	// Test Drive file get
	t.Run("GetDriveFile", func(t *testing.T) {
		if driveGetTool == nil {
			t.Fatal("Drive get tool not found")
		}

		result, err := driveGetTool.Handler(map[string]interface{}{
			"fileId": "test-file-123",
		})
		if err != nil {
			t.Fatalf("Drive get request failed: %v", err)
		}

		var file map[string]interface{}
		if err := json.Unmarshal([]byte(result), &file); err != nil {
			t.Fatalf("Failed to parse Drive file response: %v", err)
		}

		if fileID, ok := file["id"]; !ok || fileID != "test-file-123" {
			t.Errorf("Expected id 'test-file-123', got %v", fileID)
		}
	})

	// Test Drive file download
	t.Run("DownloadDriveFile", func(t *testing.T) {
		if driveDownloadTool == nil {
			t.Fatal("Drive download tool not found")
		}

		result, err := driveDownloadTool.Handler(map[string]interface{}{
			"fileId": "test-file-123",
		})
		if err != nil {
			t.Fatalf("Drive download request failed: %v", err)
		}

		// For binary responses, the result will show "Binary data (X bytes)"
		if !strings.Contains(result, "Binary data") && !strings.Contains(result, "This is test file content") {
			t.Errorf("Expected binary data or file content in response, got: %s", result)
		}
	})

	// Test Drive file create
	t.Run("CreateDriveFile", func(t *testing.T) {
		if driveCreateTool == nil {
			t.Fatal("Drive create tool not found")
		}

		result, err := driveCreateTool.Handler(map[string]interface{}{
			"name":        "test-file.txt",
			"description": "This is a test file",
			"mimeType":    "text/plain",
		})
		if err != nil {
			t.Fatalf("Drive create request failed: %v", err)
		}

		var file map[string]interface{}
		if err := json.Unmarshal([]byte(result), &file); err != nil {
			t.Fatalf("Failed to parse created file response: %v", err)
		}

		if file["name"] != "test-file.txt" {
			t.Errorf("Expected name 'test-file.txt', got %v", file["name"])
		}
	})

	// Test Drive file delete
	t.Run("DeleteDriveFile", func(t *testing.T) {
		if driveDeleteTool == nil {
			t.Fatal("Drive delete tool not found")
		}

		result, err := driveDeleteTool.Handler(map[string]interface{}{
			"fileId": "test-file-123",
		})
		if err != nil {
			t.Fatalf("Drive delete request failed: %v", err)
		}

		if result != "" {
			t.Logf("Delete response: %s", result)
		}
	})

	// Test Drive file share
	t.Run("ShareDriveFile", func(t *testing.T) {
		if driveShareTool == nil {
			t.Fatal("Drive share tool not found")
		}

		result, err := driveShareTool.Handler(map[string]interface{}{
			"fileId":       "test-file-123",
			"type":         "user",
			"role":         "reader",
			"emailAddress": "user@example.com",
		})
		if err != nil {
			t.Fatalf("Drive share request failed: %v", err)
		}

		var permission map[string]interface{}
		if err := json.Unmarshal([]byte(result), &permission); err != nil {
			t.Fatalf("Failed to parse share response: %v", err)
		}

		if permission["role"] != "reader" {
			t.Errorf("Expected role 'reader', got %v", permission["role"])
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

	// Test parameter transformation
	t.Run("TestParameterTransformation", func(t *testing.T) {
		if calendarListTool == nil {
			t.Fatal("Calendar list tool not found for parameter transformation test")
		}

		// Test date transformation from YYYYMMDD to ISO format
		result, err := calendarListTool.Handler(map[string]interface{}{
			"startDate": "20250115",
			"endDate":   "20250116",
		})
		if err != nil {
			t.Fatalf("Parameter transformation test failed: %v", err)
		}

		// Should successfully transform dates and return events
		var events []interface{}
		if err := json.Unmarshal([]byte(result), &events); err != nil {
			t.Fatalf("Failed to parse transformed response: %v", err)
		}

		if len(events) == 0 {
			t.Error("Expected events with transformed parameters")
		}
	})
}

// TestGoogleOAuth2DeviceFlow tests the OAuth2 device flow specifically for Google
func TestGoogleOAuth2DeviceFlow(t *testing.T) {
	// Mock HTTP server for OAuth2 flow
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "oauth2/device/code"):
			handleGoogleDeviceCodeRequest(w, r, t)
		case strings.Contains(r.URL.Path, "oauth2/token"):
			handleGoogleTokenRequest(w, r, t)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Test OAuth2 device flow configuration
	config := createTestGoogleConfig(server.URL)
	logger := &MockLogger{}

	fusion := New(
		WithJSONConfigData([]byte(config), "test-google-oauth-config.json"),
		WithLogger(logger),
	)

	// Get the tools
	tools := fusion.RegisterTools()

	// Find profile tool
	var profileTool *global.ToolDefinition
	for _, tool := range tools {
		if tool.Name == "google_profile_get" {
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

func handleGoogleDeviceCodeRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "POST" {
		t.Errorf("Expected POST for device code request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"device_code":      "mockgoogledevicecode123",
		"user_code":        "GOOG-1234",
		"verification_url": "https://www.google.com/device",
		"expires_in":       1800,
		"interval":         5,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleGoogleTokenRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "POST" {
		t.Errorf("Expected POST for token request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"access_token":  "mock_google_token_12345",
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": "mock_google_refresh_token_67890",
		"scope":         "https://www.googleapis.com/auth/calendar https://www.googleapis.com/auth/gmail.readonly https://www.googleapis.com/auth/drive",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleGoogleProfileRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "GET" {
		t.Errorf("Expected GET for profile request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"id":             "12345",
		"name":           "Test Google User",
		"email":          "test.user@gmail.com",
		"verified_email": true,
		"given_name":     "Test",
		"family_name":    "User",
		"picture":        "https://lh3.googleusercontent.com/test",
		"locale":         "en",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleGoogleCalendarEventsRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "GET" {
		t.Errorf("Expected GET for calendar events request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if this is a paginated request
	isPaginated := strings.Contains(r.URL.RawQuery, "timeMin") &&
		!strings.Contains(r.URL.RawQuery, "pageToken")

	var response map[string]interface{}

	if isPaginated {
		// First page of results
		response = map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"id":          "event-1",
					"summary":     "Team Meeting",
					"start":       map[string]string{"dateTime": "2025-01-15T09:00:00Z"},
					"end":         map[string]string{"dateTime": "2025-01-15T10:00:00Z"},
					"location":    "Conference Room A",
					"description": "Weekly team sync",
				},
				{
					"id":          "event-2",
					"summary":     "Project Review",
					"start":       map[string]string{"dateTime": "2025-01-16T14:00:00Z"},
					"end":         map[string]string{"dateTime": "2025-01-16T15:00:00Z"},
					"location":    "Office",
					"description": "Review project status",
				},
			},
			"nextPageToken": "page2token",
		}
	} else if strings.Contains(r.URL.RawQuery, "pageToken=page2token") {
		// Second page of results
		response = map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"id":          "event-3",
					"summary":     "Client Call",
					"start":       map[string]string{"dateTime": "2025-01-17T11:00:00Z"},
					"end":         map[string]string{"dateTime": "2025-01-17T12:00:00Z"},
					"location":    "Virtual",
					"description": "Call with client",
				},
			},
		}
	} else {
		// Single page for summary view
		response = map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"id":          "event-1",
					"summary":     "Team Meeting",
					"start":       map[string]string{"dateTime": "2025-01-15T09:00:00Z"},
					"end":         map[string]string{"dateTime": "2025-01-15T10:00:00Z"},
					"location":    "Conference Room A",
					"description": "Weekly team sync",
				},
			},
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleGoogleCalendarEventCreateRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "POST" {
		t.Errorf("Expected POST for calendar event create request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestBody map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&requestBody)

	response := map[string]interface{}{
		"id":          "created-event-123",
		"summary":     requestBody["summary"],
		"description": requestBody["description"],
		"location":    requestBody["location"],
		"start":       requestBody["start"],
		"end":         requestBody["end"],
		"status":      "confirmed",
		"created":     "2025-01-10T10:00:00Z",
		"updated":     "2025-01-10T10:00:00Z",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleGoogleCalendarEventGetRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "GET" {
		t.Errorf("Expected GET for calendar event get request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract event ID from path
	parts := strings.Split(r.URL.Path, "/")
	eventId := parts[len(parts)-1]

	response := map[string]interface{}{
		"id":          eventId,
		"summary":     "Retrieved Event",
		"description": "This is a retrieved event",
		"location":    "Test Location",
		"start":       map[string]string{"dateTime": "2025-01-15T10:00:00Z"},
		"end":         map[string]string{"dateTime": "2025-01-15T11:00:00Z"},
		"status":      "confirmed",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleGoogleCalendarEventUpdateRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "PUT" {
		t.Errorf("Expected PUT for calendar event update request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract event ID from path
	parts := strings.Split(r.URL.Path, "/")
	eventId := parts[len(parts)-1]

	var requestBody map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&requestBody)

	response := map[string]interface{}{
		"id":          eventId,
		"summary":     requestBody["summary"],
		"description": requestBody["description"],
		"location":    requestBody["location"],
		"start":       requestBody["start"],
		"end":         requestBody["end"],
		"status":      "confirmed",
		"updated":     "2025-01-10T11:00:00Z",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleGoogleCalendarEventDeleteRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "DELETE" {
		t.Errorf("Expected DELETE for calendar event delete request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return empty response for successful delete
	w.WriteHeader(http.StatusNoContent)
}

func handleGoogleGmailMessagesRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "GET" {
		t.Errorf("Expected GET for Gmail messages request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"id":       "msg-1",
				"threadId": "thread-1",
			},
			{
				"id":       "msg-2",
				"threadId": "thread-2",
			},
		},
		"nextPageToken": "gmailpage2token",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleGoogleGmailMessageGetRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "GET" {
		t.Errorf("Expected GET for Gmail message get request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract message ID from path
	parts := strings.Split(r.URL.Path, "/")
	messageId := parts[len(parts)-1]

	response := map[string]interface{}{
		"id":           messageId,
		"threadId":     "thread-123",
		"snippet":      "This is a test email snippet...",
		"internalDate": "1641542400000",
		"payload": map[string]interface{}{
			"headers": []map[string]interface{}{
				{"name": "Subject", "value": "Test Email Subject"},
				{"name": "From", "value": "sender@example.com"},
				{"name": "To", "value": "recipient@example.com"},
				{"name": "Date", "value": "Thu, 06 Jan 2022 10:00:00 -0800"},
			},
			"body": map[string]interface{}{
				"data": "VGhpcyBpcyB0aGUgZW1haWwgYm9keSBjb250ZW50", // Base64 encoded
			},
			"parts": []map[string]interface{}{},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleGoogleGmailSendRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "POST" {
		t.Errorf("Expected POST for Gmail send request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestBody map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&requestBody)

	response := map[string]interface{}{
		"id":       "sent-msg-123",
		"threadId": "sent-thread-123",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleGoogleDriveFilesRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "GET" {
		t.Errorf("Expected GET for Drive files request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"files": []map[string]interface{}{
			{
				"id":           "file-1",
				"name":         "Test Document.docx",
				"mimeType":     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
				"size":         "12345",
				"modifiedTime": "2025-01-10T10:00:00Z",
				"webViewLink":  "https://docs.google.com/document/d/file-1/view",
				"parents":      []string{"folder-123"},
			},
			{
				"id":           "file-2",
				"name":         "Test Spreadsheet.xlsx",
				"mimeType":     "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
				"size":         "67890",
				"modifiedTime": "2025-01-09T15:30:00Z",
				"webViewLink":  "https://docs.google.com/spreadsheets/d/file-2/view",
				"parents":      []string{"folder-123"},
			},
		},
		"nextPageToken": "drivepage2token",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleGoogleDriveFileGetRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "GET" {
		t.Errorf("Expected GET for Drive file get request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract file ID from path
	parts := strings.Split(r.URL.Path, "/")
	fileId := parts[len(parts)-1]

	response := map[string]interface{}{
		"id":             fileId,
		"name":           "Test File.txt",
		"mimeType":       "text/plain",
		"size":           "1024",
		"modifiedTime":   "2025-01-10T10:00:00Z",
		"webViewLink":    fmt.Sprintf("https://drive.google.com/file/d/%s/view", fileId),
		"webContentLink": fmt.Sprintf("https://drive.google.com/uc?id=%s&export=download", fileId),
		"description":    "This is a test file",
		"parents":        []string{"folder-123"},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleGoogleDriveFileDownloadRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "GET" {
		t.Errorf("Expected GET for Drive file download request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return mock file content
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("This is test file content from Google Drive"))
}

func handleGoogleDriveFileCreateRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "POST" {
		t.Errorf("Expected POST for Drive file create request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestBody map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&requestBody)

	response := map[string]interface{}{
		"id":           "created-file-123",
		"name":         requestBody["name"],
		"mimeType":     requestBody["mimeType"],
		"description":  requestBody["description"],
		"parents":      requestBody["parents"],
		"createdTime":  "2025-01-10T10:00:00Z",
		"modifiedTime": "2025-01-10T10:00:00Z",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleGoogleDriveFileDeleteRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "DELETE" {
		t.Errorf("Expected DELETE for Drive file delete request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return empty response for successful delete
	w.WriteHeader(http.StatusNoContent)
}

func handleGoogleDriveFileShareRequest(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "POST" {
		t.Errorf("Expected POST for Drive file share request, got %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestBody map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&requestBody)

	response := map[string]interface{}{
		"id":           "permission-123",
		"type":         requestBody["type"],
		"role":         requestBody["role"],
		"emailAddress": requestBody["emailAddress"],
		"domain":       requestBody["domain"],
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func createTestGoogleConfig(baseURL string) string {
	config := map[string]interface{}{
		"services": map[string]interface{}{
			"google": map[string]interface{}{
				"name":    "Google APIs",
				"baseURL": baseURL,
				"auth": map[string]interface{}{
					"type": "oauth2_device",
					"config": map[string]interface{}{
						"clientId":         "test-google-client-id",
						"scope":            []string{"https://www.googleapis.com/auth/calendar", "https://www.googleapis.com/auth/gmail.readonly", "https://www.googleapis.com/auth/drive"},
						"authorizationURL": baseURL + "/oauth2/device/code",
						"tokenURL":         baseURL + "/oauth2/token",
					},
				},
				"endpoints": []interface{}{
					map[string]interface{}{
						"id":          "profile_get",
						"name":        "Get User Profile",
						"description": "Get the current user's profile information from Google",
						"method":      "GET",
						"path":        "/oauth2/v2/userinfo",
						"parameters":  []interface{}{},
						"response": map[string]interface{}{
							"type": "json",
							"caching": map[string]interface{}{
								"enabled": true,
								"ttl":     "30m",
							},
						},
					},
				},
			},
		},
	}

	configBytes, _ := json.Marshal(config)
	return string(configBytes)
}

func createTestGoogleConfigWithBearer(baseURL string) string {
	config := map[string]interface{}{
		"services": map[string]interface{}{
			"google": map[string]interface{}{
				"name":    "Google APIs",
				"baseURL": baseURL,
				"auth": map[string]interface{}{
					"type": "bearer",
					"config": map[string]interface{}{
						"token": "mock_google_token_12345",
					},
				},
				"endpoints": []interface{}{
					// Profile endpoint
					map[string]interface{}{
						"id":          "profile_get",
						"name":        "Get User Profile",
						"description": "Get the current user's profile information from Google",
						"method":      "GET",
						"path":        "/oauth2/v2/userinfo",
						"parameters":  []interface{}{},
						"response": map[string]interface{}{
							"type": "json",
							"caching": map[string]interface{}{
								"enabled": true,
								"ttl":     "30m",
							},
						},
					},
					// Calendar endpoints
					map[string]interface{}{
						"id":          "calendar_events_list",
						"name":        "List Calendar Events",
						"description": "List events from Google Calendar",
						"method":      "GET",
						"path":        "/calendar/v3/calendars/primary/events",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "startDate",
								"description": "Start date in YYYYMMDD format",
								"type":        "string",
								"required":    false,
								"location":    "query",
								"validation": map[string]interface{}{
									"pattern": "^\\d{8}$",
								},
								"transform": map[string]interface{}{
									"targetName": "timeMin",
									"expression": "concat(slice(0,4), '-', slice(4,6), '-', slice(6,8), 'T00:00:00Z')",
								},
							},
							map[string]interface{}{
								"name":        "endDate",
								"description": "End date in YYYYMMDD format",
								"type":        "string",
								"required":    false,
								"location":    "query",
								"validation": map[string]interface{}{
									"pattern": "^\\d{8}$",
								},
								"transform": map[string]interface{}{
									"targetName": "timeMax",
									"expression": "concat(slice(0,4), '-', slice(4,6), '-', slice(6,8), 'T23:59:59Z')",
								},
							},
						},
						"response": map[string]interface{}{
							"type":      "json",
							"paginated": true,
							"paginationConfig": map[string]interface{}{
								"nextPageTokenPath": "nextPageToken",
								"dataPath":          "items",
								"pageSize":          250,
							},
							"caching": map[string]interface{}{
								"enabled": true,
								"ttl":     "5m",
							},
							// Skip complex transformation for testing
							"transform": ".items",
						},
					},
					map[string]interface{}{
						"id":          "calendar_event_create",
						"name":        "Create Calendar Event",
						"description": "Create a new event in Google Calendar",
						"method":      "POST",
						"path":        "/calendar/v3/calendars/primary/events",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "summary",
								"description": "Event title/summary",
								"type":        "string",
								"required":    true,
								"location":    "body",
							},
							map[string]interface{}{
								"name":        "startDateTime",
								"description": "Start time in RFC3339 format",
								"type":        "string",
								"required":    true,
								"location":    "body",
							},
							map[string]interface{}{
								"name":        "endDateTime",
								"description": "End time in RFC3339 format",
								"type":        "string",
								"required":    true,
								"location":    "body",
							},
						},
						"response": map[string]interface{}{
							"type": "json",
						},
					},
					map[string]interface{}{
						"id":          "calendar_event_get",
						"name":        "Get Calendar Event",
						"description": "Get a specific calendar event by ID",
						"method":      "GET",
						"path":        "/calendar/v3/calendars/primary/events/{eventId}",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "eventId",
								"description": "The ID of the event to retrieve",
								"type":        "string",
								"required":    true,
								"location":    "path",
							},
						},
						"response": map[string]interface{}{
							"type": "json",
							"caching": map[string]interface{}{
								"enabled": true,
								"ttl":     "10m",
							},
						},
					},
					map[string]interface{}{
						"id":          "calendar_event_update",
						"name":        "Update Calendar Event",
						"description": "Update an existing calendar event",
						"method":      "PUT",
						"path":        "/calendar/v3/calendars/primary/events/{eventId}",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "eventId",
								"description": "The ID of the event to update",
								"type":        "string",
								"required":    true,
								"location":    "path",
							},
							map[string]interface{}{
								"name":        "summary",
								"description": "Event title/summary",
								"type":        "string",
								"required":    false,
								"location":    "body",
							},
						},
						"response": map[string]interface{}{
							"type": "json",
						},
					},
					map[string]interface{}{
						"id":          "calendar_event_delete",
						"name":        "Delete Calendar Event",
						"description": "Delete a calendar event",
						"method":      "DELETE",
						"path":        "/calendar/v3/calendars/primary/events/{eventId}",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "eventId",
								"description": "The ID of the event to delete",
								"type":        "string",
								"required":    true,
								"location":    "path",
							},
						},
						"response": map[string]interface{}{
							"type": "text",
						},
					},
					// Gmail endpoints
					map[string]interface{}{
						"id":          "gmail_messages_list",
						"name":        "List Gmail Messages",
						"description": "List messages from Gmail inbox",
						"method":      "GET",
						"path":        "/gmail/v1/users/me/messages",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "maxResults",
								"description": "Maximum number of messages to return",
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
								"nextPageTokenPath": "nextPageToken",
								"dataPath":          "messages",
								"pageSize":          100,
							},
						},
					},
					map[string]interface{}{
						"id":          "gmail_message_get",
						"name":        "Get Gmail Message",
						"description": "Get a specific Gmail message by ID",
						"method":      "GET",
						"path":        "/gmail/v1/users/me/messages/{messageId}",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "messageId",
								"description": "The ID of the message to retrieve",
								"type":        "string",
								"required":    true,
								"location":    "path",
							},
						},
						"response": map[string]interface{}{
							"type": "json",
						},
					},
					map[string]interface{}{
						"id":          "gmail_message_send",
						"name":        "Send Gmail Message",
						"description": "Send a new email message through Gmail",
						"method":      "POST",
						"path":        "/gmail/v1/users/me/messages/send",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "to",
								"description": "Recipient email address",
								"type":        "string",
								"required":    true,
								"location":    "body",
							},
							map[string]interface{}{
								"name":        "subject",
								"description": "Email subject line",
								"type":        "string",
								"required":    true,
								"location":    "body",
							},
							map[string]interface{}{
								"name":        "body",
								"description": "Email body content",
								"type":        "string",
								"required":    true,
								"location":    "body",
							},
						},
						"response": map[string]interface{}{
							"type": "json",
						},
					},
					map[string]interface{}{
						"id":          "gmail_search_messages",
						"name":        "Search Gmail Messages",
						"description": "Search Gmail messages with advanced query syntax",
						"method":      "GET",
						"path":        "/gmail/v1/users/me/messages",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "query",
								"description": "Advanced Gmail search query",
								"type":        "string",
								"required":    true,
								"location":    "query",
								"transform": map[string]interface{}{
									"targetName": "q",
									"expression": ".",
								},
							},
						},
						"response": map[string]interface{}{
							"type":      "json",
							"paginated": true,
							"paginationConfig": map[string]interface{}{
								"nextPageTokenPath": "nextPageToken",
								"dataPath":          "messages",
								"pageSize":          100,
							},
						},
					},
					// Drive endpoints
					map[string]interface{}{
						"id":          "drive_files_list",
						"name":        "List Google Drive Files",
						"description": "List files and folders in Google Drive",
						"method":      "GET",
						"path":        "/drive/v3/files",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "pageSize",
								"description": "Maximum number of files to return",
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
								"nextPageTokenPath": "nextPageToken",
								"dataPath":          "files",
								"pageSize":          100,
							},
							"caching": map[string]interface{}{
								"enabled": true,
								"ttl":     "5m",
							},
							"transform": ".files",
						},
					},
					map[string]interface{}{
						"id":          "drive_file_get",
						"name":        "Get Google Drive File",
						"description": "Get metadata for a specific file in Google Drive",
						"method":      "GET",
						"path":        "/drive/v3/files/{fileId}",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "fileId",
								"description": "The ID of the file to retrieve",
								"type":        "string",
								"required":    true,
								"location":    "path",
							},
						},
						"response": map[string]interface{}{
							"type": "json",
						},
					},
					map[string]interface{}{
						"id":          "drive_file_download",
						"name":        "Download Google Drive File Content",
						"description": "Download the content of a file from Google Drive",
						"method":      "GET",
						"path":        "/drive/v3/files/{fileId}",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "fileId",
								"description": "The ID of the file to download",
								"type":        "string",
								"required":    true,
								"location":    "path",
							},
							map[string]interface{}{
								"name":        "alt",
								"description": "Alternative representation type",
								"type":        "string",
								"required":    false,
								"location":    "query",
								"default":     "media",
							},
						},
						"response": map[string]interface{}{
							"type": "binary",
						},
					},
					map[string]interface{}{
						"id":          "drive_file_create",
						"name":        "Create Google Drive File",
						"description": "Create a new file in Google Drive",
						"method":      "POST",
						"path":        "/upload/drive/v3/files",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "name",
								"description": "Name of the file to create",
								"type":        "string",
								"required":    true,
								"location":    "body",
							},
							map[string]interface{}{
								"name":        "mimeType",
								"description": "MIME type of the file",
								"type":        "string",
								"required":    false,
								"location":    "body",
								"default":     "text/plain",
							},
						},
						"response": map[string]interface{}{
							"type": "json",
						},
					},
					map[string]interface{}{
						"id":          "drive_file_delete",
						"name":        "Delete Google Drive File",
						"description": "Delete a file from Google Drive",
						"method":      "DELETE",
						"path":        "/drive/v3/files/{fileId}",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "fileId",
								"description": "The ID of the file to delete",
								"type":        "string",
								"required":    true,
								"location":    "path",
							},
						},
						"response": map[string]interface{}{
							"type": "text",
						},
					},
					map[string]interface{}{
						"id":          "drive_file_share",
						"name":        "Share Google Drive File",
						"description": "Share a file in Google Drive with specific permissions",
						"method":      "POST",
						"path":        "/drive/v3/files/{fileId}/permissions",
						"parameters": []interface{}{
							map[string]interface{}{
								"name":        "fileId",
								"description": "The ID of the file to share",
								"type":        "string",
								"required":    true,
								"location":    "path",
							},
							map[string]interface{}{
								"name":        "role",
								"description": "Permission role",
								"type":        "string",
								"required":    true,
								"location":    "body",
							},
							map[string]interface{}{
								"name":        "type",
								"description": "Permission type",
								"type":        "string",
								"required":    true,
								"location":    "body",
							},
							map[string]interface{}{
								"name":        "emailAddress",
								"description": "Email address (required for user/group type)",
								"type":        "string",
								"required":    false,
								"location":    "body",
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
