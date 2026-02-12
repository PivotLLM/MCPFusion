/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PivotLLM/MCPFusion/global"
	"github.com/itchyny/gojq"
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

		// For static parameters, always use the default value
		if param.Static {
			if param.Default == nil {
				return "", fmt.Errorf("static path parameter %s must have a default value", param.Name)
			}
			value = param.Default
			exists = true
		} else if !exists && param.Required {
			return "", fmt.Errorf("required path parameter %s not provided", param.Name)
		}

		if exists {
			// Apply time token substitution first
			value = m.timeTokenProcessor.ProcessValue(value)

			// Apply quoting if specified
			if param.Quoted {
				valueStr := fmt.Sprintf("%v", value)
				// Escape existing quotes
				valueStr = strings.ReplaceAll(valueStr, `"`, `\"`)
				// Wrap in quotes
				value = `"` + valueStr + `"`
			}

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
			// Use QueryEscape instead of PathEscape to ensure all special characters
			// (including '=' in base64-encoded IDs) are properly encoded for Microsoft Graph
			fullURL = strings.ReplaceAll(fullURL, placeholder, url.QueryEscape(valueStr))
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

		// For static parameters, always use the default value
		if param.Static {
			if param.Default == nil {
				return fmt.Errorf("static query parameter %s must have a default value", param.Name)
			}
			value = param.Default
		} else if !exists {
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

		// Apply quoting if specified
		if param.Quoted {
			valueStr := fmt.Sprintf("%v", value)
			// Escape existing quotes
			valueStr = strings.ReplaceAll(valueStr, `"`, `\"`)
			// Wrap in quotes
			value = `"` + valueStr + `"`
		}

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

		// For static parameters, always use the default value
		if param.Static {
			if param.Default == nil {
				return fmt.Errorf("static header parameter %s must have a default value", param.Name)
			}
			value = param.Default
		} else if !exists {
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

		// Apply quoting if specified
		if param.Quoted {
			valueStr := fmt.Sprintf("%v", value)
			// Escape existing quotes
			valueStr = strings.ReplaceAll(valueStr, `"`, `\"`)
			// Wrap in quotes
			value = `"` + valueStr + `"`
		}

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

// setNestedValue sets a value in a nested map using dot-notation keys.
// e.g., setNestedValue(body, "start.dateTime", "2025-01-15T10:00:00Z")
// produces: {"start": {"dateTime": "2025-01-15T10:00:00Z"}}
func setNestedValue(body map[string]interface{}, key string, value interface{}) {
	parts := strings.Split(key, ".")
	if len(parts) == 1 {
		body[key] = value
		return
	}
	current := body
	for _, part := range parts[:len(parts)-1] {
		if existing, ok := current[part]; ok {
			if m, ok := existing.(map[string]interface{}); ok {
				current = m
			} else {
				m := make(map[string]interface{})
				current[part] = m
				current = m
			}
		} else {
			m := make(map[string]interface{})
			current[part] = m
			current = m
		}
	}
	current[parts[len(parts)-1]] = value
}

// BuildRequestBody builds the request body from parameters.
// When requestBody is non-nil, body parameters without a transform.targetName
// are collected, passed through the named encoder, and placed at wrapperPath.
// Parameters with targetName bypass encoding and go directly into the body.
func (m *Mapper) BuildRequestBody(params []ParameterConfig, args map[string]interface{},
	requestBody *RequestBodyConfig) (map[string]interface{}, error) {

	body := make(map[string]interface{})

	// Track flat params (no targetName) separately when encoding is configured
	var flatParamNames []string

	for _, param := range params {
		if param.Location != "body" {
			continue
		}

		value, exists := args[param.Name]

		// For static parameters, always use the default value
		if param.Static {
			if param.Default == nil {
				return nil, fmt.Errorf("static body parameter %s must have a default value", param.Name)
			}
			value = param.Default
		} else if !exists {
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

		// Apply quoting if specified
		if param.Quoted {
			valueStr := fmt.Sprintf("%v", value)
			// Escape existing quotes
			valueStr = strings.ReplaceAll(valueStr, `"`, `\"`)
			// Wrap in quotes
			value = `"` + valueStr + `"`
		}

		// Determine if this param has a target name (bypasses encoding)
		hasTargetName := param.Transform != nil && param.Transform.TargetName != ""

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
			setNestedValue(body, paramName, transformedValue)
		} else {
			setNestedValue(body, param.Name, value)
		}

		// Track flat params (no targetName) for potential encoding
		if requestBody != nil && !hasTargetName {
			flatParamNames = append(flatParamNames, param.Name)
		}
	}

	// Apply body encoding if configured
	if requestBody != nil && len(flatParamNames) > 0 {
		encoder, ok := GetBodyEncoder(requestBody.Encoding)
		if !ok {
			return nil, fmt.Errorf("unknown body encoding: %s", requestBody.Encoding)
		}

		// Extract flat params from body into a separate map for encoding
		flatParams := make(map[string]interface{}, len(flatParamNames))
		for _, name := range flatParamNames {
			flatParams[name] = body[name]
			delete(body, name)
		}

		encoded, err := encoder.Encode(flatParams)
		if err != nil {
			return nil, fmt.Errorf("body encoding failed: %w", err)
		}

		setNestedValue(body, requestBody.WrapperPath, encoded)

		if m.logger != nil {
			m.logger.Debugf("Encoded %d parameters with %s → %s", len(flatParamNames), requestBody.Encoding, requestBody.WrapperPath)
		}
	}

	if len(body) == 0 {
		return nil, nil
	}

	if m.logger != nil {
		m.logger.Debugf("Built request body with %d top-level keys", len(body))
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
	case ".":
		return value, nil
	default:
		// If we can't handle the transformation, return the original value
		if m.logger != nil {
			m.logger.Warningf("Unsupported transformation expression: %s", expression)
		}
		return value, nil
	}
}

// TransformResponse applies jq transformations to the response using gojq.
func (m *Mapper) TransformResponse(data interface{}, transform string) (interface{}, error) {
	if transform == "" {
		return data, nil
	}

	query, err := gojq.Parse(transform)
	if err != nil {
		return nil, fmt.Errorf("invalid transform expression: %w", err)
	}

	iter := query.Run(data)
	v, ok := iter.Next()
	if !ok {
		return nil, fmt.Errorf("transform produced no output")
	}
	if err, isErr := v.(error); isErr {
		return nil, fmt.Errorf("transform execution failed: %w", err)
	}
	return v, nil
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
	case "integer":
		return "integer"
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
