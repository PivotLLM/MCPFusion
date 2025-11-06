/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/PivotLLM/MCPFusion/mlogger"
)

func TestTimeTokenProcessor_ProcessValue(t *testing.T) {
	logger, _ := mlogger.New(mlogger.WithDebug(false))
	processor := NewTimeTokenProcessor(logger)

	tests := []struct {
		name         string
		input        interface{}
		expectRegex  string // Use regex to match because exact time will vary
		expectChange bool
	}{
		{
			name:         "DAYS-0 token (today)",
			input:        "#DAYS-0",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T00:00:00Z$`,
			expectChange: true,
		},
		{
			name:         "DAYS-1 token (yesterday)",
			input:        "#DAYS-1",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T00:00:00Z$`,
			expectChange: true,
		},
		{
			name:         "DAYS-7 token (week ago)",
			input:        "#DAYS-7",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T00:00:00Z$`,
			expectChange: true,
		},
		{
			name:         "HOURS-0 token (now)",
			input:        "#HOURS-0",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,
			expectChange: true,
		},
		{
			name:         "HOURS-1 token (1 hour ago)",
			input:        "#HOURS-1",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,
			expectChange: true,
		},
		{
			name:         "HOURS-24 token (24 hours ago)",
			input:        "#HOURS-24",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,
			expectChange: true,
		},
		{
			name:         "MINS-0 token (now)",
			input:        "#MINS-0",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,
			expectChange: true,
		},
		{
			name:         "MINS-5 token (5 minutes ago)",
			input:        "#MINS-5",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,
			expectChange: true,
		},
		{
			name:         "MINS-30 token (30 minutes ago)",
			input:        "#MINS-30",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,
			expectChange: true,
		},
		{
			name:         "MINS-60 token (60 minutes ago)",
			input:        "#MINS-60",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,
			expectChange: true,
		},
		{
			name:         "Mixed tokens in string",
			input:        "from=#DAYS-7&to=#DAYS-0",
			expectRegex:  `^from=\d{4}-\d{2}-\d{2}T00:00:00Z&to=\d{4}-\d{2}-\d{2}T00:00:00Z$`,
			expectChange: true,
		},
		{
			name:         "Complex string with multiple token types",
			input:        "start=#DAYS-30&end=#HOURS-6&current=#HOURS-0",
			expectRegex:  `^start=\d{4}-\d{2}-\d{2}T00:00:00Z&end=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z&current=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,
			expectChange: true,
		},
		{
			name:         "DAYS+0 token (today in future)",
			input:        "#DAYS+0",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T00:00:00Z$`,
			expectChange: true,
		},
		{
			name:         "DAYS+1 token (tomorrow)",
			input:        "#DAYS+1",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T00:00:00Z$`,
			expectChange: true,
		},
		{
			name:         "DAYS+7 token (week from now)",
			input:        "#DAYS+7",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T00:00:00Z$`,
			expectChange: true,
		},
		{
			name:         "HOURS+0 token (now in future)",
			input:        "#HOURS+0",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,
			expectChange: true,
		},
		{
			name:         "HOURS+1 token (1 hour from now)",
			input:        "#HOURS+1",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,
			expectChange: true,
		},
		{
			name:         "HOURS+24 token (24 hours from now)",
			input:        "#HOURS+24",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,
			expectChange: true,
		},
		{
			name:         "MINS+0 token (now in future)",
			input:        "#MINS+0",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,
			expectChange: true,
		},
		{
			name:         "MINS+5 token (5 minutes from now)",
			input:        "#MINS+5",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,
			expectChange: true,
		},
		{
			name:         "MINS+30 token (30 minutes from now)",
			input:        "#MINS+30",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,
			expectChange: true,
		},
		{
			name:         "MINS+60 token (60 minutes from now)",
			input:        "#MINS+60",
			expectRegex:  `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,
			expectChange: true,
		},
		{
			name:         "Mixed past and future tokens",
			input:        "start=#DAYS-7&end=#DAYS+7&now=#HOURS+0",
			expectRegex:  `^start=\d{4}-\d{2}-\d{2}T00:00:00Z&end=\d{4}-\d{2}-\d{2}T00:00:00Z&now=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,
			expectChange: true,
		},
		{
			name:         "Mixed with minutes tokens",
			input:        "past=#MINS-15&future=#MINS+45&hour=#HOURS-1",
			expectRegex:  `^past=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z&future=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z&hour=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,
			expectChange: true,
		},
		{
			name:         "No tokens - unchanged",
			input:        "regular string value",
			expectRegex:  `^regular string value$`,
			expectChange: false,
		},
		{
			name:         "Non-string value - unchanged",
			input:        42,
			expectRegex:  "",
			expectChange: false,
		},
		{
			name:         "Empty string - unchanged",
			input:        "",
			expectRegex:  `^$`,
			expectChange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.ProcessValue(tt.input)

			// For non-string inputs, should return unchanged
			if _, isString := tt.input.(string); !isString {
				if result != tt.input {
					t.Errorf("Expected non-string value to be unchanged, got %v", result)
				}
				return
			}

			resultStr, ok := result.(string)
			if !ok {
				t.Errorf("Expected result to be a string, got %T", result)
				return
			}

			// Check if result matches expected pattern
			if tt.expectRegex != "" {
				matched, err := regexp.MatchString(tt.expectRegex, resultStr)
				if err != nil {
					t.Errorf("Regex error: %v", err)
					return
				}
				if !matched {
					t.Errorf("Result '%s' doesn't match pattern '%s'", resultStr, tt.expectRegex)
				}
			}

			// Check if change occurred as expected
			changed := resultStr != tt.input.(string)
			if changed != tt.expectChange {
				t.Errorf("Expected change=%v, got change=%v. Input: '%s', Result: '%s'",
					tt.expectChange, changed, tt.input, resultStr)
			}
		})
	}
}

func TestTimeTokenProcessor_HasTimeTokens(t *testing.T) {
	logger, _ := mlogger.New(mlogger.WithDebug(false))
	processor := NewTimeTokenProcessor(logger)

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"DAYS token", "#DAYS-7", true},
		{"HOURS token", "#HOURS-24", true},
		{"DAYS+ token", "#DAYS+7", true},
		{"HOURS+ token", "#HOURS+24", true},
		{"Both past tokens", "#DAYS-7 and #HOURS-12", true},
		{"Both future tokens", "#DAYS+7 and #HOURS+12", true},
		{"Mixed past and future", "#DAYS-7 and #DAYS+14", true},
		{"No tokens", "regular string", false},
		{"Invalid token format", "#DAYS-X", false},
		{"Partial match", "DAYS-7", false},
		{"Empty string", "", false},
		{"Multiple valid tokens", "#DAYS-0 #DAYS+1 #HOURS-6 #HOURS+12", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.HasTimeTokens(tt.input)
			if result != tt.expected {
				t.Errorf("HasTimeTokens(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTimeTokenProcessor_ValidateTimeTokens(t *testing.T) {
	logger, _ := mlogger.New(mlogger.WithDebug(false))
	processor := NewTimeTokenProcessor(logger)

	tests := []struct {
		name      string
		input     string
		expectErr bool
		errMsg    string
	}{
		{"Valid DAYS token", "#DAYS-7", false, ""},
		{"Valid HOURS token", "#HOURS-24", false, ""},
		{"Valid DAYS+ token", "#DAYS+7", false, ""},
		{"Valid HOURS+ token", "#HOURS+24", false, ""},
		{"Valid multiple tokens", "#DAYS-0 #HOURS-1", false, ""},
		{"Valid mixed past and future", "#DAYS-7 #DAYS+14 #HOURS-6 #HOURS+12", false, ""},
		{"Invalid DAYS - too large", "#DAYS-500", true, "days value out of range"},
		{"Invalid HOURS - too large", "#HOURS-10000", true, "hours value out of range"},
		{"Invalid DAYS+ - too large", "#DAYS+500", true, "days value out of range"},
		{"Invalid HOURS+ - too large", "#HOURS+10000", true, "hours value out of range"},
		{"Invalid DAYS - negative", "#DAYS--1", false, ""}, // Regex won't match negative
		{"Invalid number format", "#DAYS-abc", false, ""},  // Regex won't match non-digits
		{"Edge case - boundary values", "#DAYS-365 #HOURS-8760 #DAYS+365 #HOURS+8760", false, ""},
		{"No tokens", "regular string", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.ValidateTimeTokens(tt.input)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestTimeTokenProcessor_ProcessParameterArgs(t *testing.T) {
	logger, _ := mlogger.New(mlogger.WithDebug(false))
	processor := NewTimeTokenProcessor(logger)

	tests := []struct {
		name     string
		input    map[string]interface{}
		validate func(t *testing.T, result map[string]interface{})
	}{
		{
			name: "Mix of token and non-token parameters",
			input: map[string]interface{}{
				"startDate": "#DAYS-7",
				"endDate":   "#DAYS-0",
				"count":     10,
				"name":      "test",
				"timestamp": "#HOURS-1",
			},
			validate: func(t *testing.T, result map[string]interface{}) {
				// Check that startDate was processed
				startDate, ok := result["startDate"].(string)
				if !ok {
					t.Error("startDate should be a string")
					return
				}
				if !isValidISODate(startDate) {
					t.Errorf("startDate should be valid ISO date, got: %s", startDate)
				}

				// Check that non-token values remain unchanged
				if result["count"] != 10 {
					t.Error("count should remain unchanged")
				}
				if result["name"] != "test" {
					t.Error("name should remain unchanged")
				}
			},
		},
		{
			name:  "Nil input",
			input: nil,
			validate: func(t *testing.T, result map[string]interface{}) {
				if result != nil {
					t.Error("Result should be nil for nil input")
				}
			},
		},
		{
			name:  "Empty map",
			input: map[string]interface{}{},
			validate: func(t *testing.T, result map[string]interface{}) {
				if len(result) != 0 {
					t.Error("Result should be empty for empty input")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.ProcessParameterArgs(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestTimeTokenSubstitution_ActualTimeCalculation(t *testing.T) {
	logger, _ := mlogger.New(mlogger.WithDebug(false))
	processor := NewTimeTokenProcessor(logger)

	// Test that DAYS tokens produce times at midnight
	result := processor.ProcessValue("#DAYS-0")
	if resultStr, ok := result.(string); ok {
		// Parse the time to verify it's at midnight
		parsedTime, err := time.Parse(time.RFC3339, resultStr)
		if err != nil {
			t.Errorf("Failed to parse result time: %v", err)
			return
		}

		// Check that it's at midnight (00:00:00)
		if parsedTime.Hour() != 0 || parsedTime.Minute() != 0 || parsedTime.Second() != 0 {
			t.Errorf("DAYS token should produce midnight time, got %s", resultStr)
		}

		// Check that it's today's date
		today := time.Now().UTC().Truncate(24 * time.Hour)
		if !parsedTime.Equal(today) {
			t.Errorf("DAYS-0 should be today at midnight, expected %s, got %s",
				today.Format(time.RFC3339), resultStr)
		}
	} else {
		t.Error("Result should be a string")
	}

	// Test that HOURS tokens preserve the hour precision
	result = processor.ProcessValue("#HOURS-1")
	if resultStr, ok := result.(string); ok {
		parsedTime, err := time.Parse(time.RFC3339, resultStr)
		if err != nil {
			t.Errorf("Failed to parse result time: %v", err)
			return
		}

		// Should be approximately 1 hour ago (within 1 minute tolerance)
		expectedTime := time.Now().UTC().Add(-1 * time.Hour)
		diff := expectedTime.Sub(parsedTime)
		if diff < -time.Minute || diff > time.Minute {
			t.Errorf("HOURS-1 should be approximately 1 hour ago, got %s", resultStr)
		}
	} else {
		t.Error("Result should be a string")
	}

	// Test that MINS tokens preserve the minute precision
	result = processor.ProcessValue("#MINS-5")
	if resultStr, ok := result.(string); ok {
		parsedTime, err := time.Parse(time.RFC3339, resultStr)
		if err != nil {
			t.Errorf("Failed to parse result time: %v", err)
			return
		}

		// Should be approximately 5 minutes ago (within 1 second tolerance)
		expectedTime := time.Now().UTC().Add(-5 * time.Minute)
		diff := expectedTime.Sub(parsedTime)
		if diff < -time.Second || diff > time.Second {
			t.Errorf("MINS-5 should be approximately 5 minutes ago, got %s", resultStr)
		}
	} else {
		t.Error("Result should be a string")
	}
}

func TestTimeTokenSubstitution_FutureTimeCalculation(t *testing.T) {
	logger, _ := mlogger.New(mlogger.WithDebug(false))
	processor := NewTimeTokenProcessor(logger)

	// Test that DAYS+ tokens produce times at midnight in the future
	result := processor.ProcessValue("#DAYS+0")
	if resultStr, ok := result.(string); ok {
		// Parse the time to verify it's at midnight
		parsedTime, err := time.Parse(time.RFC3339, resultStr)
		if err != nil {
			t.Errorf("Failed to parse result time: %v", err)
			return
		}

		// Check that it's at midnight (00:00:00)
		if parsedTime.Hour() != 0 || parsedTime.Minute() != 0 || parsedTime.Second() != 0 {
			t.Errorf("DAYS+ token should produce midnight time, got %s", resultStr)
		}

		// Check that it's today's date (since DAYS+0 should be today at midnight)
		today := time.Now().UTC().Truncate(24 * time.Hour)
		if !parsedTime.Equal(today) {
			t.Errorf("DAYS+0 should be today at midnight, expected %s, got %s",
				today.Format(time.RFC3339), resultStr)
		}
	} else {
		t.Error("Result should be a string")
	}

	// Test that DAYS+1 is tomorrow
	result = processor.ProcessValue("#DAYS+1")
	if resultStr, ok := result.(string); ok {
		parsedTime, err := time.Parse(time.RFC3339, resultStr)
		if err != nil {
			t.Errorf("Failed to parse result time: %v", err)
			return
		}

		// Check that it's tomorrow at midnight
		tomorrow := time.Now().UTC().AddDate(0, 0, 1).Truncate(24 * time.Hour)
		if !parsedTime.Equal(tomorrow) {
			t.Errorf("DAYS+1 should be tomorrow at midnight, expected %s, got %s",
				tomorrow.Format(time.RFC3339), resultStr)
		}
	} else {
		t.Error("Result should be a string")
	}

	// Test that HOURS+ tokens preserve the hour precision
	result = processor.ProcessValue("#HOURS+1")
	if resultStr, ok := result.(string); ok {
		parsedTime, err := time.Parse(time.RFC3339, resultStr)
		if err != nil {
			t.Errorf("Failed to parse result time: %v", err)
			return
		}

		// Should be approximately 1 hour from now (within 1 minute tolerance)
		expectedTime := time.Now().UTC().Add(1 * time.Hour)
		diff := parsedTime.Sub(expectedTime)
		if diff < -time.Minute || diff > time.Minute {
			t.Errorf("HOURS+1 should be approximately 1 hour from now, got %s", resultStr)
		}
	} else {
		t.Error("Result should be a string")
	}

	// Test that MINS+ tokens preserve the minute precision
	result = processor.ProcessValue("#MINS+30")
	if resultStr, ok := result.(string); ok {
		parsedTime, err := time.Parse(time.RFC3339, resultStr)
		if err != nil {
			t.Errorf("Failed to parse result time: %v", err)
			return
		}

		// Should be approximately 30 minutes from now (within 1 second tolerance)
		expectedTime := time.Now().UTC().Add(30 * time.Minute)
		diff := parsedTime.Sub(expectedTime)
		if diff < -time.Second || diff > time.Second {
			t.Errorf("MINS+30 should be approximately 30 minutes from now, got %s", resultStr)
		}
	} else {
		t.Error("Result should be a string")
	}
}

func TestTimeTokenProcessor_GetSupportedTokens(t *testing.T) {
	logger, _ := mlogger.New(mlogger.WithDebug(false))
	processor := NewTimeTokenProcessor(logger)

	tokens := processor.GetSupportedTokens()

	// Check that we have all expected token types
	expectedTokens := []string{"#DAYS-N", "#HOURS-N", "#MINS-N", "#DAYS+N", "#HOURS+N", "#MINS+N"}
	for _, expectedToken := range expectedTokens {
		if _, exists := tokens[expectedToken]; !exists {
			t.Errorf("Should include %s token", expectedToken)
		}
	}

	// Check that descriptions are not empty
	for tokenType, description := range tokens {
		if description == "" {
			t.Errorf("Description for %s should not be empty", tokenType)
		}
	}
}

func TestConvenienceFunctions(t *testing.T) {
	logger, _ := mlogger.New(mlogger.WithDebug(false))

	// Test SubstituteTimeTokensInParameterValue
	result := SubstituteTimeTokensInParameterValue("#DAYS-0", logger)
	if resultStr, ok := result.(string); ok {
		if !isValidISODate(resultStr) {
			t.Errorf("SubstituteTimeTokensInParameterValue should return valid ISO date, got: %s", resultStr)
		}
	} else {
		t.Error("Result should be a string")
	}

	// Test with non-string value
	result = SubstituteTimeTokensInParameterValue(42, logger)
	if result != 42 {
		t.Error("Non-string values should be returned unchanged")
	}

	// Test SubstituteTimeTokensInString
	result2 := SubstituteTimeTokensInString("#HOURS-6", logger)
	if !isValidISODate(result2) {
		t.Errorf("SubstituteTimeTokensInString should return valid ISO date, got: %s", result2)
	}
}

// Helper function to validate ISO 8601 date format
func isValidISODate(dateStr string) bool {
	_, err := time.Parse(time.RFC3339, dateStr)
	return err == nil
}
