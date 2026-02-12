/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_setNestedValue(t *testing.T) {
	tests := []struct {
		name     string
		initial  map[string]interface{}
		key      string
		value    interface{}
		expected map[string]interface{}
	}{
		{
			name:    "single key flat assignment",
			initial: map[string]interface{}{},
			key:     "subject",
			value:   "Meeting",
			expected: map[string]interface{}{
				"subject": "Meeting",
			},
		},
		{
			name:    "two-level nesting",
			initial: map[string]interface{}{},
			key:     "start.dateTime",
			value:   "2025-01-15T10:00:00Z",
			expected: map[string]interface{}{
				"start": map[string]interface{}{
					"dateTime": "2025-01-15T10:00:00Z",
				},
			},
		},
		{
			name:    "three-level nesting",
			initial: map[string]interface{}{},
			key:     "a.b.c",
			value:   42,
			expected: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": 42,
					},
				},
			},
		},
		{
			name: "shared prefix merges into one object",
			initial: func() map[string]interface{} {
				m := map[string]interface{}{}
				setNestedValue(m, "start.dateTime", "2025-01-15T10:00:00Z")
				return m
			}(),
			key:   "start.timeZone",
			value: "America/New_York",
			expected: map[string]interface{}{
				"start": map[string]interface{}{
					"dateTime": "2025-01-15T10:00:00Z",
					"timeZone": "America/New_York",
				},
			},
		},
		{
			name: "overwrite non-map existing key",
			initial: map[string]interface{}{
				"start": "scalar-value",
			},
			key:   "start.dateTime",
			value: "2025-01-15T10:00:00Z",
			expected: map[string]interface{}{
				"start": map[string]interface{}{
					"dateTime": "2025-01-15T10:00:00Z",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setNestedValue(tt.initial, tt.key, tt.value)
			assert.Equal(t, tt.expected, tt.initial)
		})
	}
}

func TestMapper_BuildRequestBody_DotNotation(t *testing.T) {
	mapper := NewMapper(nil)

	params := []ParameterConfig{
		{
			Name:     "startDateTime",
			Type:     ParameterTypeString,
			Required: true,
			Location: ParameterLocationBody,
			Transform: &TransformConfig{
				TargetName: "start.dateTime",
				Expression: ".",
			},
		},
		{
			Name:     "startTimeZone",
			Type:     ParameterTypeString,
			Required: false,
			Location: ParameterLocationBody,
			Transform: &TransformConfig{
				TargetName: "start.timeZone",
				Expression: ".",
			},
		},
		{
			Name:     "subject",
			Type:     ParameterTypeString,
			Required: true,
			Location: ParameterLocationBody,
		},
	}

	args := map[string]interface{}{
		"startDateTime": "2025-07-01T10:00:00Z",
		"startTimeZone": "America/New_York",
		"subject":       "Team Meeting",
	}

	body, err := mapper.BuildRequestBody(params, args, nil)
	require.NoError(t, err)
	require.NotNil(t, body)

	// Verify the flat key
	assert.Equal(t, "Team Meeting", body["subject"])

	// Verify the nested structure
	startObj, ok := body["start"].(map[string]interface{})
	require.True(t, ok, "start should be a nested map")
	assert.Equal(t, "2025-07-01T10:00:00Z", startObj["dateTime"])
	assert.Equal(t, "America/New_York", startObj["timeZone"])
}

func TestMapper_BuildRequestBody_IdentityPassthrough(t *testing.T) {
	mapper := NewMapper(nil)

	params := []ParameterConfig{
		{
			Name:     "description",
			Type:     ParameterTypeString,
			Required: true,
			Location: ParameterLocationBody,
			Transform: &TransformConfig{
				TargetName: "description",
				Expression: ".",
			},
		},
	}

	args := map[string]interface{}{
		"description": "A simple test value",
	}

	body, err := mapper.BuildRequestBody(params, args, nil)
	require.NoError(t, err)
	require.NotNil(t, body)

	// Identity expression "." should pass the value through unchanged
	assert.Equal(t, "A simple test value", body["description"])
}

func TestMapper_BuildRequestBody_WithEncoding(t *testing.T) {
	mapper := NewMapper(nil)

	params := []ParameterConfig{
		{Name: "to", Type: ParameterTypeString, Required: true, Location: ParameterLocationBody},
		{Name: "subject", Type: ParameterTypeString, Required: true, Location: ParameterLocationBody},
		{Name: "body", Type: ParameterTypeString, Required: true, Location: ParameterLocationBody},
	}
	args := map[string]interface{}{
		"to":      "alice@example.com",
		"subject": "Test",
		"body":    "Hello",
	}
	rbConfig := &RequestBodyConfig{
		Encoding:    "rfc2822_base64url",
		WrapperPath: "message.raw",
	}

	result, err := mapper.BuildRequestBody(params, args, rbConfig)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Flat params should be gone
	assert.Nil(t, result["to"])
	assert.Nil(t, result["subject"])
	assert.Nil(t, result["body"])

	// Encoded value should be at message.raw
	msgObj, ok := result["message"].(map[string]interface{})
	require.True(t, ok, "message should be a nested map")
	rawStr, ok := msgObj["raw"].(string)
	require.True(t, ok, "raw should be a string")

	decoded, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(rawStr)
	require.NoError(t, err)
	assert.Contains(t, string(decoded), "To: alice@example.com")
	assert.Contains(t, string(decoded), "Subject: Test")
	assert.True(t, strings.HasSuffix(string(decoded), "\r\n\r\nHello"))
}

func TestMapper_BuildRequestBody_WithEncoding_MixedParams(t *testing.T) {
	mapper := NewMapper(nil)

	params := []ParameterConfig{
		{
			Name:     "messageId",
			Type:     ParameterTypeString,
			Required: true,
			Location: ParameterLocationBody,
			Transform: &TransformConfig{
				TargetName: "message.threadId",
				Expression: ".",
			},
		},
		{Name: "to", Type: ParameterTypeString, Required: true, Location: ParameterLocationBody},
		{Name: "subject", Type: ParameterTypeString, Required: true, Location: ParameterLocationBody},
		{Name: "body", Type: ParameterTypeString, Required: true, Location: ParameterLocationBody},
	}
	args := map[string]interface{}{
		"messageId": "thread123",
		"to":        "bob@example.com",
		"subject":   "Reply",
		"body":      "Thanks!",
	}
	rbConfig := &RequestBodyConfig{
		Encoding:    "rfc2822_base64url",
		WrapperPath: "message.raw",
	}

	result, err := mapper.BuildRequestBody(params, args, rbConfig)
	require.NoError(t, err)
	require.NotNil(t, result)

	// messageId with targetName should bypass encoding → message.threadId
	msgObj, ok := result["message"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "thread123", msgObj["threadId"])

	// Flat params should be encoded → message.raw
	rawStr, ok := msgObj["raw"].(string)
	require.True(t, ok, "raw should be a string")
	decoded, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(rawStr)
	require.NoError(t, err)
	assert.Contains(t, string(decoded), "To: bob@example.com")
	assert.Contains(t, string(decoded), "Subject: Reply")

	// Flat params should not appear at top level
	assert.Nil(t, result["to"])
	assert.Nil(t, result["subject"])
	assert.Nil(t, result["body"])
}

func TestMapper_BuildRequestBody_NoEncoding_FlatParams(t *testing.T) {
	mapper := NewMapper(nil)

	params := []ParameterConfig{
		{Name: "to", Type: ParameterTypeString, Required: true, Location: ParameterLocationBody},
		{Name: "subject", Type: ParameterTypeString, Required: true, Location: ParameterLocationBody},
	}
	args := map[string]interface{}{
		"to":      "alice@example.com",
		"subject": "Test",
	}

	// nil requestBody = backwards compatible behavior
	result, err := mapper.BuildRequestBody(params, args, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "alice@example.com", result["to"])
	assert.Equal(t, "Test", result["subject"])
}
