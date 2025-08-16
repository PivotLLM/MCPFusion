/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PivotLLM/MCPFusion/global"
)

// Mapper handles parameter mapping and transformation
type Mapper struct {
	logger             global.Logger
	timeTokenProcessor *TimeTokenProcessor
}

// NewMapper creates a new Mapper
func NewMapper(logger global.Logger) *Mapper {
	return &Mapper{
		logger:             logger,
		timeTokenProcessor: NewTimeTokenProcessor(logger),
	}
}

// BuildURL builds a URL with path parameters replaced
func (m *Mapper) BuildURL(baseURL, path string, params []ParameterConfig, args map[string]interface{}) (string, error) {
	// Start with base URL
	fullURL := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")

	// Replace path parameters
	for _, param := range params {
		if param.Location != "path" {
			continue
		}

		value, exists := args[param.Name]
		if !exists && param.Required {
			return "", fmt.Errorf("required path parameter %s not provided", param.Name)
		}

		if exists {
			// Apply time token substitution first
			value = m.timeTokenProcessor.ProcessValue(value)

			// Apply transformation if specified
			if param.Transform != nil {
				transformedValue, err := m.transformParameter(param, value)
				if err != nil {
					return "", fmt.Errorf("failed to transform path parameter %s: %w", param.Name, err)
				}
				value = transformedValue
			}

			// Replace placeholder in path
			placeholder := "{" + param.Name + "}"
			valueStr := fmt.Sprintf("%v", value)
			fullURL = strings.ReplaceAll(fullURL, placeholder, url.PathEscape(valueStr))
		}
	}

	if m.logger != nil {
		m.logger.Debugf("Built URL: %s", fullURL)
	}

	return fullURL, nil
}

// ApplyQueryParams adds query parameters to a request
func (m *Mapper) ApplyQueryParams(req *http.Request, params []ParameterConfig, args map[string]interface{}) error {
	query := req.URL.Query()

	for _, param := range params {
		if param.Location != "query" {
			continue
		}

		value, exists := args[param.Name]
		if !exists {
			if param.Default != nil {
				value = param.Default
			} else if param.Required {
				return fmt.Errorf("required query parameter %s not provided", param.Name)
			} else {
				continue
			}
		}

		// Apply time token substitution first
		value = m.timeTokenProcessor.ProcessValue(value)

		// Apply transformation if specified
		if param.Transform != nil {
			transformedValue, err := m.transformParameter(param, value)
			if err != nil {
				return fmt.Errorf("failed to transform query parameter %s: %w", param.Name, err)
			}

			// Use the target name if specified
			paramName := param.Transform.TargetName
			if paramName == "" {
				paramName = param.Name
			}

			valueStr := fmt.Sprintf("%v", transformedValue)
			query.Set(paramName, valueStr)
		} else {
			valueStr := fmt.Sprintf("%v", value)
			query.Set(param.Name, valueStr)
		}
	}

	req.URL.RawQuery = query.Encode()

	if m.logger != nil {
		m.logger.Debugf("Applied query parameters: %s", req.URL.RawQuery)
	}

	return nil
}

// ApplyHeaders adds header parameters to a request
func (m *Mapper) ApplyHeaders(req *http.Request, params []ParameterConfig, args map[string]interface{}) error {
	for _, param := range params {
		if param.Location != "header" {
			continue
		}

		value, exists := args[param.Name]
		if !exists {
			if param.Default != nil {
				value = param.Default
			} else if param.Required {
				return fmt.Errorf("required header parameter %s not provided", param.Name)
			} else {
				continue
			}
		}

		// Apply time token substitution first
		value = m.timeTokenProcessor.ProcessValue(value)

		// Apply transformation if specified
		if param.Transform != nil {
			transformedValue, err := m.transformParameter(param, value)
			if err != nil {
				return fmt.Errorf("failed to transform header parameter %s: %w", param.Name, err)
			}
			value = transformedValue
		}

		valueStr := fmt.Sprintf("%v", value)
		req.Header.Set(param.Name, valueStr)
	}

	return nil
}

// BuildRequestBody builds the request body from parameters
func (m *Mapper) BuildRequestBody(params []ParameterConfig, args map[string]interface{}) (map[string]interface{}, error) {
	body := make(map[string]interface{})

	for _, param := range params {
		if param.Location != "body" {
			continue
		}

		value, exists := args[param.Name]
		if !exists {
			if param.Default != nil {
				value = param.Default
			} else if param.Required {
				return nil, fmt.Errorf("required body parameter %s not provided", param.Name)
			} else {
				continue
			}
		}

		// Apply time token substitution first
		value = m.timeTokenProcessor.ProcessValue(value)

		// Apply transformation if specified
		if param.Transform != nil {
			transformedValue, err := m.transformParameter(param, value)
			if err != nil {
				return nil, fmt.Errorf("failed to transform body parameter %s: %w", param.Name, err)
			}

			// Use the target name if specified
			paramName := param.Transform.TargetName
			if paramName == "" {
				paramName = param.Name
			}
			body[paramName] = transformedValue
		} else {
			body[param.Name] = value
		}
	}

	if len(body) == 0 {
		return nil, nil
	}

	if m.logger != nil {
		m.logger.Debugf("Built request body with %d parameters", len(body))
	}

	return body, nil
}

// transformParameter applies transformation to a parameter value
func (m *Mapper) transformParameter(param ParameterConfig, value interface{}) (interface{}, error) {
	if param.Transform == nil {
		return value, nil
	}

	// For now, we'll implement simple transformations
	// In the future, this could use a more sophisticated expression engine

	valueStr := fmt.Sprintf("%v", value)
	expression := param.Transform.Expression

	// Handle date transformation (YYYYMMDD to ISO 8601)
	if strings.Contains(expression, "slice") && strings.Contains(expression, "concat") {
		// This is the pattern for date transformation
		if len(valueStr) == 8 {
			// Transform YYYYMMDD to YYYY-MM-DDTHH:MM:SSZ
			year := valueStr[0:4]
			month := valueStr[4:6]
			day := valueStr[6:8]

			if strings.Contains(expression, "T00:00:00Z") {
				return fmt.Sprintf("%s-%s-%sT00:00:00Z", year, month, day), nil
			} else if strings.Contains(expression, "T23:59:59Z") {
				return fmt.Sprintf("%s-%s-%sT23:59:59Z", year, month, day), nil
			}
		}
	}

	// Handle other simple transformations
	switch expression {
	case "uppercase":
		return strings.ToUpper(valueStr), nil
	case "lowercase":
		return strings.ToLower(valueStr), nil
	case "trim":
		return strings.TrimSpace(valueStr), nil
	default:
		// If we can't handle the transformation, return the original value
		if m.logger != nil {
			m.logger.Warningf("Unsupported transformation expression: %s", expression)
		}
		return value, nil
	}
}

// TransformResponse applies JQ-like transformations to the response
func (m *Mapper) TransformResponse(data interface{}, transform string) (interface{}, error) {
	// For now, we'll implement basic transformations
	// In the future, this could use a proper JQ library

	if transform == "" {
		return data, nil
	}

	// Handle simple field selection like ".value" or "$.value"
	if strings.HasPrefix(transform, ".") || strings.HasPrefix(transform, "$.") {
		// Remove the leading "." or "$."
		fieldPath := transform
		if strings.HasPrefix(fieldPath, "$.") {
			fieldPath = fieldPath[2:]
		} else if strings.HasPrefix(fieldPath, ".") {
			fieldPath = fieldPath[1:]
		}

		parts := strings.Split(fieldPath, ".")
		current := data

		for _, part := range parts {
			// Handle array access like ".value[0]"
			if strings.Contains(part, "[") && strings.Contains(part, "]") {
				// This is simplified - a real implementation would parse properly
				fieldName := part[:strings.Index(part, "[")]

				if obj, ok := current.(map[string]interface{}); ok {
					if field, exists := obj[fieldName]; exists {
						current = field
					} else {
						return nil, fmt.Errorf("field %s not found in response", fieldName)
					}
				} else {
					return nil, fmt.Errorf("cannot access field %s on non-object", fieldName)
				}
			} else {
				// Regular field access
				if obj, ok := current.(map[string]interface{}); ok {
					if field, exists := obj[part]; exists {
						current = field
					} else {
						return nil, fmt.Errorf("field %s not found in response", part)
					}
				} else {
					return nil, fmt.Errorf("cannot access field %s on non-object", part)
				}
			}
		}

		return current, nil
	}

	// Handle the complex transformation from the Microsoft example
	// ".value | map({subject: .subject, start: .start.dateTime, end: .end.dateTime})"
	if strings.Contains(transform, "| map(") {
		// Extract the field to map over
		fieldPart := strings.TrimSpace(strings.Split(transform, "|")[0])

		// Get the array to map over
		var arrayData []interface{}
		if fieldPart == ".value" {
			if obj, ok := data.(map[string]interface{}); ok {
				if arr, ok := obj["value"].([]interface{}); ok {
					arrayData = arr
				} else {
					return nil, fmt.Errorf("expected array at .value")
				}
			}
		}

		// For the Microsoft calendar example specifically
		if strings.Contains(transform, "subject: .subject") {
			result := make([]map[string]interface{}, 0, len(arrayData))

			for _, item := range arrayData {
				if obj, ok := item.(map[string]interface{}); ok {
					mapped := make(map[string]interface{})

					// Extract subject
					if subject, ok := obj["subject"].(string); ok {
						mapped["subject"] = subject
					}

					// Extract start time
					if start, ok := obj["start"].(map[string]interface{}); ok {
						if dateTime, ok := start["dateTime"].(string); ok {
							mapped["start"] = dateTime
						}
					}

					// Extract end time
					if end, ok := obj["end"].(map[string]interface{}); ok {
						if dateTime, ok := end["dateTime"].(string); ok {
							mapped["end"] = dateTime
						}
					}

					result = append(result, mapped)
				}
			}

			return result, nil
		}
	}

	// If we can't handle the transformation, return the original data
	if m.logger != nil {
		m.logger.Warningf("Complex transformation not fully implemented: %s", transform)
	}

	return data, nil
}

// ConvertToMCPParameters converts endpoint parameters to MCP tool parameters
func (m *Mapper) ConvertToMCPParameters(params []ParameterConfig) map[string]interface{} {
	properties := make(map[string]interface{})
	required := make([]string, 0)

	for _, param := range params {
		// Use MCP-compliant name (alias or sanitized)
		mcpName := GetMCPParameterName(&param)

		// Log the mapping if different from original
		if m.logger != nil && mcpName != param.Name {
			if param.Alias != "" {
				m.logger.Infof("Using parameter alias '%s' for '%s'", mcpName, param.Name)
			} else {
				m.logger.Warningf("Auto-sanitized parameter '%s' to '%s' - consider adding explicit alias",
					param.Name, mcpName)
			}
		}

		// Create property definition
		prop := map[string]interface{}{
			"type":        m.getMCPType(param.Type),
			"description": param.Description,
		}

		// Add validation constraints
		if param.Validation != nil {
			if param.Validation.Pattern != "" {
				prop["pattern"] = param.Validation.Pattern
			}
			if param.Validation.MinLength != nil && *param.Validation.MinLength > 0 {
				prop["minLength"] = *param.Validation.MinLength
			}
			if param.Validation.MaxLength != nil && *param.Validation.MaxLength > 0 {
				prop["maxLength"] = *param.Validation.MaxLength
			}
			if len(param.Validation.Enum) > 0 {
				prop["enum"] = param.Validation.Enum
			}
			// Min/Max are not yet supported in ValidationConfig
		}

		// Add default value if specified
		if param.Default != nil {
			prop["default"] = param.Default
		}

		properties[mcpName] = prop // Use MCP-compliant name

		// Track required parameters with MCP-compliant names
		if param.Required {
			required = append(required, mcpName)
		}
	}

	// Build the schema
	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

// getMCPType converts internal types to JSON Schema types
func (m *Mapper) getMCPType(paramType ParameterType) string {
	switch paramType {
	case "number":
		return "number"
	case "boolean":
		return "boolean"
	case "array":
		return "array"
	case "object":
		return "object"
	default:
		return "string"
	}
}
