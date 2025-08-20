/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"testing"

	"github.com/PivotLLM/MCPFusion/mlogger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidator_AutoConvertISOToYYYYMMDD(t *testing.T) {
	logger, err := mlogger.New(mlogger.WithDebug(true))
	require.NoError(t, err)
	
	validator := NewValidator(logger)

	// Test parameter configuration for YYYYMMDD format
	params := []ParameterConfig{
		{
			Name:        "startDate",
			Description: "Start date in YYYYMMDD format",
			Type:        "string",
			Required:    true,
			Location:    "query",
			Validation: &ValidationConfig{
				Pattern: "^\\d{8}$", // YYYYMMDD pattern
			},
		},
	}

	tests := []struct {
		name          string
		inputValue    string
		expectedValue string
		shouldPass    bool
	}{
		{
			name:          "Convert ISO with timezone",
			inputValue:    "2025-08-19T00:00:00Z",
			expectedValue: "20250819",
			shouldPass:    true,
		},
		{
			name:          "Convert ISO with milliseconds",
			inputValue:    "2025-12-25T00:00:00.000Z",
			expectedValue: "20251225",
			shouldPass:    true,
		},
		{
			name:          "Convert date only format",
			inputValue:    "2025-01-15",
			expectedValue: "20250115",
			shouldPass:    true,
		},
		{
			name:          "Pass through valid YYYYMMDD",
			inputValue:    "20250819",
			expectedValue: "20250819",
			shouldPass:    true,
		},
		{
			name:          "Reject invalid format",
			inputValue:    "invalid-date",
			expectedValue: "",
			shouldPass:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create args map with test value
			args := map[string]interface{}{
				"startDate": tt.inputValue,
			}

			// Validate parameters
			err := validator.ValidateParameters(params, args)

			if tt.shouldPass {
				assert.NoError(t, err, "Validation should pass for %s", tt.name)
				
				// Check that the value was converted correctly
				actualValue, exists := args["startDate"]
				assert.True(t, exists, "Parameter should exist after validation")
				assert.Equal(t, tt.expectedValue, actualValue, "Value should be converted to YYYYMMDD format")
			} else {
				assert.Error(t, err, "Validation should fail for %s", tt.name)
			}
		})
	}
}

func TestValidator_tryConvertISOToYYYYMMDD(t *testing.T) {
	logger, err := mlogger.New(mlogger.WithDebug(false))
	require.NoError(t, err)
	
	validator := NewValidator(logger)

	tests := []struct {
		name        string
		input       string
		expected    string
		description string
	}{
		{
			name:        "RFC3339 with Z",
			input:       "2025-08-19T00:00:00Z",
			expected:    "20250819",
			description: "Standard ISO format with Z timezone",
		},
		{
			name:        "RFC3339 with milliseconds",
			input:       "2025-12-25T15:30:45.123Z",
			expected:    "20251225",
			description: "ISO format with milliseconds and Z timezone",
		},
		{
			name:        "RFC3339 with timezone offset",
			input:       "2025-01-01T12:00:00-08:00",
			expected:    "20250101",
			description: "ISO format with timezone offset",
		},
		{
			name:        "Date only format",
			input:       "2025-06-15",
			expected:    "20250615",
			description: "Simple YYYY-MM-DD format",
		},
		{
			name:        "Invalid format",
			input:       "not-a-date",
			expected:    "",
			description: "Should return empty string for invalid format",
		},
		{
			name:        "Already YYYYMMDD",
			input:       "20250819",
			expected:    "",
			description: "Should return empty string as it's not an ISO format to convert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.tryConvertISOToYYYYMMDD(tt.input)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

func TestValidator_NoConversionForNonDatePatterns(t *testing.T) {
	logger, err := mlogger.New(mlogger.WithDebug(false))
	require.NoError(t, err)
	
	validator := NewValidator(logger)

	// Test parameter configuration for non-YYYYMMDD pattern
	params := []ParameterConfig{
		{
			Name:        "emailField",
			Description: "Email address",
			Type:        "string",
			Required:    true,
			Location:    "query",
			Validation: &ValidationConfig{
				Pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$", // Email pattern
			},
		},
	}

	// Create args map with ISO date (should not be converted for non-date fields)
	args := map[string]interface{}{
		"emailField": "2025-08-19T00:00:00Z",
	}

	// Validate parameters - should fail because ISO date doesn't match email pattern
	err = validator.ValidateParameters(params, args)
	assert.Error(t, err, "Validation should fail as ISO date doesn't match email pattern")
	
	// Verify the value was not modified
	actualValue, exists := args["emailField"]
	assert.True(t, exists, "Parameter should still exist")
	assert.Equal(t, "2025-08-19T00:00:00Z", actualValue, "Value should not be modified for non-date patterns")
}