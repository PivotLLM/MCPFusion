/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/PivotLLM/MCPFusion/mlogger"
)

func TestTimeTokenIntegration_MapperProcessing(t *testing.T) {
	logger, _ := mlogger.New(mlogger.WithDebug(false))
	mapper := NewMapper(logger)

	tests := []struct {
		name        string
		parameters  []ParameterConfig
		args        map[string]interface{}
		location    string
		expectRegex string
	}{
		{
			name: "Query parameter with DAYS token",
			parameters: []ParameterConfig{
				{
					Name:        "startDate",
					Type:        "string",
					Location:    "query",
					Required:    true,
					Description: "Start date for query",
				},
			},
			args: map[string]interface{}{
				"startDate": "#DAYS-7",
			},
			location:    "query",
			expectRegex: `startDate=\d{4}-\d{2}-\d{2}T00%3A00%3A00Z`,
		},
		{
			name: "Body parameter with HOURS token",
			parameters: []ParameterConfig{
				{
					Name:        "timestamp",
					Type:        "string",
					Location:    "body",
					Required:    true,
					Description: "Timestamp for request",
				},
			},
			args: map[string]interface{}{
				"timestamp": "#HOURS-1",
			},
			location:    "body",
			expectRegex: `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`,
		},
		{
			name: "Header parameter with mixed tokens",
			parameters: []ParameterConfig{
				{
					Name:        "X-Date-Range",
					Type:        "string",
					Location:    "header",
					Required:    true,
					Description: "Date range header",
				},
			},
			args: map[string]interface{}{
				"X-Date-Range": "from=#DAYS-30&to=#DAYS-0",
			},
			location:    "header",
			expectRegex: `from=\d{4}-\d{2}-\d{2}T00:00:00Z&to=\d{4}-\d{2}-\d{2}T00:00:00Z`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.location {
			case "query":
				// Test query parameter processing
				// Create a mock HTTP request to test query parameter application
				req, _ := http.NewRequest("GET", "http://example.com", nil)

				err := mapper.ApplyQueryParams(req, tt.parameters, tt.args)
				if err != nil {
					t.Fatalf("ApplyQueryParams failed: %v", err)
				}

				queryString := req.URL.RawQuery
				matched, err := regexp.MatchString(tt.expectRegex, queryString)
				if err != nil {
					t.Fatalf("Regex error: %v", err)
				}
				if !matched {
					t.Errorf("Query string '%s' doesn't match pattern '%s'", queryString, tt.expectRegex)
				}

			case "body":
				// Test body parameter processing
				body, err := mapper.BuildRequestBody(tt.parameters, tt.args)
				if err != nil {
					t.Fatalf("BuildRequestBody failed: %v", err)
				}

				if timestamp, ok := body["timestamp"].(string); ok {
					matched, err := regexp.MatchString(tt.expectRegex, timestamp)
					if err != nil {
						t.Fatalf("Regex error: %v", err)
					}
					if !matched {
						t.Errorf("Body timestamp '%s' doesn't match pattern '%s'", timestamp, tt.expectRegex)
					}
				} else {
					t.Error("Expected timestamp in body")
				}

			case "header":
				// Test header parameter processing
				req, _ := http.NewRequest("GET", "http://example.com", nil)

				err := mapper.ApplyHeaders(req, tt.parameters, tt.args)
				if err != nil {
					t.Fatalf("ApplyHeaders failed: %v", err)
				}

				headerValue := req.Header.Get("X-Date-Range")
				matched, err := regexp.MatchString(tt.expectRegex, headerValue)
				if err != nil {
					t.Fatalf("Regex error: %v", err)
				}
				if !matched {
					t.Errorf("Header value '%s' doesn't match pattern '%s'", headerValue, tt.expectRegex)
				}
			}
		})
	}
}

func TestTimeTokenIntegration_ParameterNameMapper(t *testing.T) {
	logger, _ := mlogger.New(mlogger.WithDebug(false))

	// Test that time token processing works after parameter name mapping
	params := []ParameterConfig{
		{
			Name:        "$filter",
			Alias:       "filter",
			Type:        "string",
			Location:    "query",
			Required:    false,
			Description: "OData filter parameter",
		},
	}

	// Build parameter mapper
	mapper, err := BuildParameterMappings(params, logger)
	if err != nil {
		t.Fatalf("Failed to build parameter mappings: %v", err)
	}

	// Simulate MCP args (using sanitized/alias names)
	mcpArgs := map[string]interface{}{
		"filter": "createdDateTime ge #DAYS-7",
	}

	// Map to original API names
	apiArgs := mapper.MapArgsToOriginal(mcpArgs)

	// Process time tokens
	timeTokenProcessor := NewTimeTokenProcessor(logger)
	processedArgs := timeTokenProcessor.ProcessParameterArgs(apiArgs)

	// Verify the filter parameter was processed
	if filterValue, exists := processedArgs["$filter"]; exists {
		if filterStr, ok := filterValue.(string); ok {
			// Should contain a properly formatted timestamp
			matched, err := regexp.MatchString(`createdDateTime ge \d{4}-\d{2}-\d{2}T00:00:00Z`, filterStr)
			if err != nil {
				t.Fatalf("Regex error: %v", err)
			}
			if !matched {
				t.Errorf("Filter value '%s' doesn't contain expected timestamp pattern", filterStr)
			}
		} else {
			t.Error("Filter value should be a string")
		}
	} else {
		t.Error("Expected $filter parameter in processed args")
	}
}

func TestTimeTokenIntegration_PathParameters(t *testing.T) {
	logger, _ := mlogger.New(mlogger.WithDebug(false))
	mapper := NewMapper(logger)

	// Test path parameters with time tokens
	params := []ParameterConfig{
		{
			Name:        "date",
			Type:        "string",
			Location:    "path",
			Required:    true,
			Description: "Date path parameter",
		},
	}

	args := map[string]interface{}{
		"date": "#DAYS-1",
	}

	baseURL := "https://api.example.com"
	path := "/events/{date}"

	resultURL, err := mapper.BuildURL(baseURL, path, params, args)
	if err != nil {
		t.Fatalf("BuildURL failed: %v", err)
	}

	// Check that the URL contains a properly formatted date
	// Note: URL path encoding doesn't encode colons in the time portion
	expectedPattern := `https://api\.example\.com/events/\d{4}-\d{2}-\d{2}T00:00:00Z`
	matched, err := regexp.MatchString(expectedPattern, resultURL)
	if err != nil {
		t.Fatalf("Regex error: %v", err)
	}
	if !matched {
		t.Errorf("URL '%s' doesn't match expected pattern '%s'", resultURL, expectedPattern)
	}
}

func TestTimeTokenIntegration_ComplexScenario(t *testing.T) {
	logger, _ := mlogger.New(mlogger.WithDebug(false))
	mapper := NewMapper(logger)

	// Test complex scenario with multiple parameter types and time tokens
	params := []ParameterConfig{
		{
			Name:        "startDate",
			Type:        "string",
			Location:    "query",
			Required:    true,
			Description: "Start date",
		},
		{
			Name:        "endDate",
			Type:        "string",
			Location:    "query",
			Required:    true,
			Description: "End date",
		},
		{
			Name:        "X-Request-Time",
			Type:        "string",
			Location:    "header",
			Required:    false,
			Description: "Request timestamp",
		},
		{
			Name:        "lastModified",
			Type:        "string",
			Location:    "body",
			Required:    false,
			Description: "Last modified time",
		},
	}

	args := map[string]interface{}{
		"startDate":      "#DAYS-30",
		"endDate":        "#DAYS-0",
		"X-Request-Time": "#HOURS-0",
		"lastModified":   "#HOURS-24",
	}

	// Test query parameters
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	err := mapper.ApplyQueryParams(req, params, args)
	if err != nil {
		t.Fatalf("ApplyQueryParams failed: %v", err)
	}

	queryString := req.URL.RawQuery
	// Should contain both startDate and endDate with proper timestamps
	startDatePattern := `startDate=\d{4}-\d{2}-\d{2}T00%3A00%3A00Z`
	endDatePattern := `endDate=\d{4}-\d{2}-\d{2}T00%3A00%3A00Z`

	if matched, _ := regexp.MatchString(startDatePattern, queryString); !matched {
		t.Errorf("Query string missing proper startDate: %s", queryString)
	}
	if matched, _ := regexp.MatchString(endDatePattern, queryString); !matched {
		t.Errorf("Query string missing proper endDate: %s", queryString)
	}

	// Test headers
	err = mapper.ApplyHeaders(req, params, args)
	if err != nil {
		t.Fatalf("ApplyHeaders failed: %v", err)
	}

	requestTime := req.Header.Get("X-Request-Time")
	requestTimePattern := `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`
	if matched, _ := regexp.MatchString(requestTimePattern, requestTime); !matched {
		t.Errorf("Header X-Request-Time doesn't match pattern: %s", requestTime)
	}

	// Test body
	body, err := mapper.BuildRequestBody(params, args)
	if err != nil {
		t.Fatalf("BuildRequestBody failed: %v", err)
	}

	if lastModified, ok := body["lastModified"].(string); ok {
		lastModifiedPattern := `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`
		if matched, _ := regexp.MatchString(lastModifiedPattern, lastModified); !matched {
			t.Errorf("Body lastModified doesn't match pattern: %s", lastModified)
		}
	} else {
		t.Error("Expected lastModified in body")
	}
}
