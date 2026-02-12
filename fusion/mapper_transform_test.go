/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapper_TransformResponse(t *testing.T) {
	mapper := NewMapper(nil)

	tests := []struct {
		name        string
		data        interface{}
		transform   string
		expected    interface{}
		expectError bool
	}{
		// 1. Empty transform returns data unchanged
		{
			name: "empty transform returns data unchanged",
			data: map[string]interface{}{
				"id":   "123",
				"name": "test",
			},
			transform:   "",
			expected:    map[string]interface{}{"id": "123", "name": "test"},
			expectError: false,
		},

		// 2. Simple field access
		{
			name: "simple field access extracts a field",
			data: map[string]interface{}{
				"id":   "msg_001",
				"name": "Alice",
				"age":  float64(30),
			},
			transform:   ".id",
			expected:    "msg_001",
			expectError: false,
		},

		// 3. Nested field access
		{
			name: "nested field access extracts nested field",
			data: map[string]interface{}{
				"payload": map[string]interface{}{
					"mimeType": "text/html",
					"body": map[string]interface{}{
						"size": float64(1024),
					},
				},
			},
			transform:   ".payload.mimeType",
			expected:    "text/html",
			expectError: false,
		},

		// 4. Object construction
		{
			name: "object construction builds new object",
			data: map[string]interface{}{
				"id":      "item_42",
				"name":    "Widget",
				"status":  "active",
				"created": "2025-01-15",
			},
			transform: "{id: .id, name: .name}",
			expected: map[string]interface{}{
				"id":   "item_42",
				"name": "Widget",
			},
			expectError: false,
		},

		// 5. Array mapping
		{
			name: "array mapping maps array elements",
			data: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{"id": "1", "name": "Alpha", "extra": "discard"},
					map[string]interface{}{"id": "2", "name": "Beta", "extra": "discard"},
					map[string]interface{}{"id": "3", "name": "Gamma", "extra": "discard"},
				},
			},
			transform: ".items | map({id: .id, name: .name})",
			expected: []interface{}{
				map[string]interface{}{"id": "1", "name": "Alpha"},
				map[string]interface{}{"id": "2", "name": "Beta"},
				map[string]interface{}{"id": "3", "name": "Gamma"},
			},
			expectError: false,
		},

		// 6. Select with test (regex filtering)
		{
			name: "select with test filters array by regex",
			data: map[string]interface{}{
				"payload": map[string]interface{}{
					"headers": []interface{}{
						map[string]interface{}{"name": "Subject", "value": "Hello World"},
						map[string]interface{}{"name": "From", "value": "alice@example.com"},
						map[string]interface{}{"name": "X-Custom", "value": "ignore-me"},
						map[string]interface{}{"name": "Date", "value": "2025-06-01"},
						map[string]interface{}{"name": "Content-Type", "value": "text/plain"},
					},
				},
			},
			transform: `[.payload.headers[] | select(.name | test("^(Subject|From)$"; "i"))]`,
			expected: []interface{}{
				map[string]interface{}{"name": "Subject", "value": "Hello World"},
				map[string]interface{}{"name": "From", "value": "alice@example.com"},
			},
			expectError: false,
		},

		// 7. Alternative operator (fallback)
		{
			name: "alternative operator falls back to second value",
			data: map[string]interface{}{
				"start": map[string]interface{}{
					"date": "2025-07-04",
				},
			},
			transform:   ".start.dateTime // .start.date",
			expected:    "2025-07-04",
			expectError: false,
		},
		{
			name: "alternative operator uses first value when present",
			data: map[string]interface{}{
				"start": map[string]interface{}{
					"dateTime": "2025-07-04T10:00:00-04:00",
					"date":     "2025-07-04",
				},
			},
			transform:   ".start.dateTime // .start.date",
			expected:    "2025-07-04T10:00:00-04:00",
			expectError: false,
		},

		// 11. Error: invalid jq expression
		{
			name:        "error on invalid jq expression",
			data:        map[string]interface{}{"id": "1"},
			transform:   ".foo | invalid[syntax",
			expected:    nil,
			expectError: true,
		},

		// 12. Missing field in object construction returns null
		{
			name: "missing field in object construction returns null",
			data: map[string]interface{}{
				"id": "item_1",
			},
			transform: "{id: .id, name: .name}",
			expected: map[string]interface{}{
				"id":   "item_1",
				"name": nil,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mapper.TransformResponse(tt.data, tt.transform)

			if tt.expectError {
				require.Error(t, err, "Expected an error but got none")
				return
			}

			require.NoError(t, err, "Unexpected error")
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMapper_TransformResponse_GmailMessage tests the full Gmail message
// transform expression used in the Google Workspace configuration.
func TestMapper_TransformResponse_GmailMessage(t *testing.T) {
	mapper := NewMapper(nil)

	transform := `{id: .id, threadId: .threadId, labelIds: .labelIds, snippet: .snippet, internalDate: .internalDate, sizeEstimate: .sizeEstimate, headers: [.payload.headers[] | select(.name | test("^(Subject|From|To|Date|Cc)$"; "i"))]}`

	data := map[string]interface{}{
		"id":             "18f1a2b3c4d5e6f7",
		"threadId":       "18f1a2b3c4d5e6f7",
		"labelIds":       []interface{}{"INBOX", "UNREAD"},
		"snippet":        "Hey, just wanted to follow up on our meeting notes from yesterday...",
		"internalDate":   "1719835200000",
		"sizeEstimate":   float64(4521),
		"historyId":      "987654",
		"resultSizeEstimate": float64(1),
		"payload": map[string]interface{}{
			"mimeType": "multipart/alternative",
			"headers": []interface{}{
				map[string]interface{}{"name": "Delivered-To", "value": "user@example.com"},
				map[string]interface{}{"name": "Date", "value": "Mon, 01 Jul 2025 12:00:00 -0400"},
				map[string]interface{}{"name": "From", "value": "Alice Smith <alice@example.com>"},
				map[string]interface{}{"name": "To", "value": "Bob Jones <bob@example.com>"},
				map[string]interface{}{"name": "Cc", "value": "Carol White <carol@example.com>"},
				map[string]interface{}{"name": "Subject", "value": "Re: Meeting Notes"},
				map[string]interface{}{"name": "MIME-Version", "value": "1.0"},
				map[string]interface{}{"name": "Content-Type", "value": "multipart/alternative; boundary=\"000\""},
				map[string]interface{}{"name": "X-Mailer", "value": "Some Client"},
				map[string]interface{}{"name": "Message-ID", "value": "<abc123@mail.example.com>"},
			},
			"body": map[string]interface{}{
				"size": float64(0),
			},
			"parts": []interface{}{
				map[string]interface{}{
					"mimeType": "text/plain",
					"body": map[string]interface{}{
						"size": float64(256),
						"data": "SGVsbG8gV29ybGQ=",
					},
				},
			},
		},
	}

	result, err := mapper.TransformResponse(data, transform)
	require.NoError(t, err, "Gmail transform should not error")

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok, "Result should be a map")

	// Verify top-level fields
	assert.Equal(t, "18f1a2b3c4d5e6f7", resultMap["id"])
	assert.Equal(t, "18f1a2b3c4d5e6f7", resultMap["threadId"])
	assert.Equal(t, []interface{}{"INBOX", "UNREAD"}, resultMap["labelIds"])
	assert.Equal(t, "Hey, just wanted to follow up on our meeting notes from yesterday...", resultMap["snippet"])
	assert.Equal(t, "1719835200000", resultMap["internalDate"])
	assert.Equal(t, float64(4521), resultMap["sizeEstimate"])

	// Verify filtered headers contain only the expected ones
	headers, ok := resultMap["headers"].([]interface{})
	require.True(t, ok, "Headers should be an array")
	assert.Len(t, headers, 5, "Should have exactly 5 filtered headers (Date, From, To, Cc, Subject)")

	expectedHeaderNames := map[string]bool{
		"Date": false, "From": false, "To": false, "Cc": false, "Subject": false,
	}
	for _, h := range headers {
		hMap, ok := h.(map[string]interface{})
		require.True(t, ok, "Each header should be a map")
		name, _ := hMap["name"].(string)
		_, expected := expectedHeaderNames[name]
		assert.True(t, expected, "Header %q should be one of the expected headers", name)
		expectedHeaderNames[name] = true
	}
	for name, found := range expectedHeaderNames {
		assert.True(t, found, "Expected header %q was not in the result", name)
	}

	// Verify unwanted fields are not present
	assert.Nil(t, resultMap["historyId"], "historyId should not be in output")
	assert.Nil(t, resultMap["payload"], "payload should not be in output")
}

// TestMapper_TransformResponse_CalendarEvents tests the full Google Calendar
// events transform expression used in the Google Workspace configuration.
func TestMapper_TransformResponse_CalendarEvents(t *testing.T) {
	mapper := NewMapper(nil)

	transform := `.items | map({id: .id, summary: .summary, start: .start.dateTime // .start.date, end: .end.dateTime // .end.date, location: .location, description: .description})`

	data := map[string]interface{}{
		"kind":    "calendar#events",
		"summary": "Primary Calendar",
		"items": []interface{}{
			// Timed event with dateTime
			map[string]interface{}{
				"id":          "evt_001",
				"summary":     "Team Standup",
				"description": "Daily sync meeting",
				"location":    "Conference Room A",
				"start": map[string]interface{}{
					"dateTime": "2025-07-01T09:00:00-04:00",
					"timeZone": "America/New_York",
				},
				"end": map[string]interface{}{
					"dateTime": "2025-07-01T09:30:00-04:00",
					"timeZone": "America/New_York",
				},
				"status":    "confirmed",
				"htmlLink":  "https://calendar.google.com/event?id=evt_001",
				"organizer": map[string]interface{}{"email": "organizer@example.com"},
			},
			// All-day event with date only
			map[string]interface{}{
				"id":      "evt_002",
				"summary": "Company Holiday",
				"start": map[string]interface{}{
					"date": "2025-07-04",
				},
				"end": map[string]interface{}{
					"date": "2025-07-05",
				},
				"status":  "confirmed",
				"htmlLink": "https://calendar.google.com/event?id=evt_002",
			},
			// Event with no location or description
			map[string]interface{}{
				"id":      "evt_003",
				"summary": "Lunch Break",
				"start": map[string]interface{}{
					"dateTime": "2025-07-01T12:00:00-04:00",
				},
				"end": map[string]interface{}{
					"dateTime": "2025-07-01T13:00:00-04:00",
				},
			},
		},
	}

	result, err := mapper.TransformResponse(data, transform)
	require.NoError(t, err, "Calendar transform should not error")

	items, ok := result.([]interface{})
	require.True(t, ok, "Result should be an array")
	require.Len(t, items, 3, "Should have 3 events")

	// Verify first event (timed event with dateTime)
	evt1, ok := items[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "evt_001", evt1["id"])
	assert.Equal(t, "Team Standup", evt1["summary"])
	assert.Equal(t, "2025-07-01T09:00:00-04:00", evt1["start"])
	assert.Equal(t, "2025-07-01T09:30:00-04:00", evt1["end"])
	assert.Equal(t, "Conference Room A", evt1["location"])
	assert.Equal(t, "Daily sync meeting", evt1["description"])

	// Verify second event (all-day event with date fallback)
	evt2, ok := items[1].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "evt_002", evt2["id"])
	assert.Equal(t, "Company Holiday", evt2["summary"])
	assert.Equal(t, "2025-07-04", evt2["start"], "Should fall back to .start.date")
	assert.Equal(t, "2025-07-05", evt2["end"], "Should fall back to .end.date")
	assert.Nil(t, evt2["location"], "Missing location should be null")
	assert.Nil(t, evt2["description"], "Missing description should be null")

	// Verify third event (no location/description)
	evt3, ok := items[2].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "evt_003", evt3["id"])
	assert.Equal(t, "Lunch Break", evt3["summary"])
	assert.Equal(t, "2025-07-01T12:00:00-04:00", evt3["start"])
	assert.Equal(t, "2025-07-01T13:00:00-04:00", evt3["end"])
	assert.Nil(t, evt3["location"])
	assert.Nil(t, evt3["description"])

	// Verify unwanted fields are stripped
	assert.Nil(t, evt1["status"], "status should not be in output")
	assert.Nil(t, evt1["htmlLink"], "htmlLink should not be in output")
	assert.Nil(t, evt1["organizer"], "organizer should not be in output")
}

// TestMapper_TransformResponse_DriveFiles tests the full Google Drive files
// transform expression used in the Google Workspace configuration.
func TestMapper_TransformResponse_DriveFiles(t *testing.T) {
	mapper := NewMapper(nil)

	transform := `.files | map({id: .id, name: .name, type: .mimeType, size: .size, modified: .modifiedTime, link: .webViewLink, parents: .parents})`

	data := map[string]interface{}{
		"kind":             "drive#fileList",
		"incompleteSearch": false,
		"files": []interface{}{
			map[string]interface{}{
				"id":           "file_abc123",
				"name":         "Q3 Report.docx",
				"mimeType":     "application/vnd.google-apps.document",
				"size":         "245760",
				"modifiedTime": "2025-06-30T14:22:00.000Z",
				"webViewLink":  "https://docs.google.com/document/d/file_abc123/edit",
				"parents":      []interface{}{"folder_root"},
				"createdTime":  "2025-06-01T08:00:00.000Z",
				"starred":      false,
				"trashed":      false,
				"owners": []interface{}{
					map[string]interface{}{"displayName": "Alice Smith"},
				},
			},
			map[string]interface{}{
				"id":           "file_def456",
				"name":         "Budget 2025.xlsx",
				"mimeType":     "application/vnd.google-apps.spreadsheet",
				"modifiedTime": "2025-06-28T09:15:00.000Z",
				"webViewLink":  "https://docs.google.com/spreadsheets/d/file_def456/edit",
				"parents":      []interface{}{"folder_finance"},
				"createdTime":  "2025-05-10T10:00:00.000Z",
			},
			map[string]interface{}{
				"id":           "folder_shared",
				"name":         "Shared Resources",
				"mimeType":     "application/vnd.google-apps.folder",
				"modifiedTime": "2025-06-25T16:45:00.000Z",
				"webViewLink":  "https://drive.google.com/drive/folders/folder_shared",
			},
		},
	}

	result, err := mapper.TransformResponse(data, transform)
	require.NoError(t, err, "Drive transform should not error")

	files, ok := result.([]interface{})
	require.True(t, ok, "Result should be an array")
	require.Len(t, files, 3, "Should have 3 files")

	// Verify first file (document with all fields)
	f1, ok := files[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "file_abc123", f1["id"])
	assert.Equal(t, "Q3 Report.docx", f1["name"])
	assert.Equal(t, "application/vnd.google-apps.document", f1["type"])
	assert.Equal(t, "245760", f1["size"])
	assert.Equal(t, "2025-06-30T14:22:00.000Z", f1["modified"])
	assert.Equal(t, "https://docs.google.com/document/d/file_abc123/edit", f1["link"])
	assert.Equal(t, []interface{}{"folder_root"}, f1["parents"])

	// Verify second file (spreadsheet without size)
	f2, ok := files[1].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "file_def456", f2["id"])
	assert.Equal(t, "Budget 2025.xlsx", f2["name"])
	assert.Equal(t, "application/vnd.google-apps.spreadsheet", f2["type"])
	assert.Nil(t, f2["size"], "Missing size should be null")
	assert.Equal(t, "2025-06-28T09:15:00.000Z", f2["modified"])
	assert.Equal(t, []interface{}{"folder_finance"}, f2["parents"])

	// Verify third file (folder without size or parents)
	f3, ok := files[2].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "folder_shared", f3["id"])
	assert.Equal(t, "Shared Resources", f3["name"])
	assert.Equal(t, "application/vnd.google-apps.folder", f3["type"])
	assert.Nil(t, f3["size"], "Missing size should be null")
	assert.Nil(t, f3["parents"], "Missing parents should be null")

	// Verify unwanted fields are stripped
	assert.Nil(t, f1["createdTime"], "createdTime should not be in output")
	assert.Nil(t, f1["starred"], "starred should not be in output")
	assert.Nil(t, f1["owners"], "owners should not be in output")
}
