/*=============================================================================
= Copyright (c) 2025 Tenebris Technologies Inc.                              =
= All rights reserved.                                                       =
=============================================================================*/

package fusion

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PivotLLM/MCPFusion/global"
)

// Validator handles parameter validation
type Validator struct {
	logger global.Logger
}

// NewValidator creates a new Validator
func NewValidator(logger global.Logger) *Validator {
	return &Validator{
		logger: logger,
	}
}

// ValidateParameters validates input parameters against their definitions
func (v *Validator) ValidateParameters(params []ParameterConfig, args map[string]interface{}) error {
	if v.logger != nil {
		v.logger.Debugf("Validating %d parameters", len(params))
	}

	for _, param := range params {
		value, exists := args[param.Name]

		// Check required parameters
		if param.Required && !exists {
			if v.logger != nil {
				v.logger.Errorf("Required parameter missing: %s", param.Name)
			}
			return NewValidationError(param.Name, nil, "required", "parameter is required")
		}

		// Skip validation if parameter not provided and not required
		if !exists {
			// Apply default value if specified
			if param.Default != nil {
				args[param.Name] = param.Default
				if v.logger != nil {
					v.logger.Debugf("Applied default value for parameter %s: %v", param.Name, param.Default)
				}
			}
			continue
		}

		// Validate parameter type
		if err := v.validateType(param, value); err != nil {
			if v.logger != nil {
				v.logger.Errorf("Type validation failed for parameter %s: %v", param.Name, err)
			}
			return err
		}

		// Apply additional validation rules
		if param.Validation != nil {
			if err := v.applyValidationRules(param, value); err != nil {
				if v.logger != nil {
					v.logger.Errorf("Validation rule failed for parameter %s: %v", param.Name, err)
				}
				return err
			}
		}
	}

	if v.logger != nil {
		v.logger.Debug("Parameter validation successful")
	}

	return nil
}

// validateType validates that a value matches the expected type
func (v *Validator) validateType(param ParameterConfig, value interface{}) error {
	switch param.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return NewValidationError(param.Name, value, "type",
				"expected string type")
		}

	case "number":
		switch value.(type) {
		case int, int32, int64, float32, float64:
			// Valid number types
		case string:
			// Try to parse string as number
			str := value.(string)
			if _, err := strconv.ParseFloat(str, 64); err != nil {
				return NewValidationError(param.Name, str, "type",
					"expected number type")
			}
		default:
			return NewValidationError(param.Name, value, "type",
				"expected number type")
		}

	case "boolean":
		switch value.(type) {
		case bool:
			// Valid boolean
		case string:
			// Try to parse string as boolean
			str := strings.ToLower(value.(string))
			if str != "true" && str != "false" {
				return NewValidationError(param.Name, value, "type",
					"expected boolean type")
			}
		default:
			return NewValidationError(param.Name, value, "type",
				"expected boolean type")
		}

	case "array":
		switch value.(type) {
		case []interface{}, []string, []int, []float64:
			// Valid array types
		default:
			return NewValidationError(param.Name, value, "type",
				"expected array type")
		}

	case "object":
		switch value.(type) {
		case map[string]interface{}:
			// Valid object type
		default:
			return NewValidationError(param.Name, value, "type",
				"expected object type")
		}

	default:
		if v.logger != nil {
			v.logger.Warningf("Unknown parameter type: %s", param.Type)
		}
	}

	return nil
}

// applyValidationRules applies additional validation rules to a parameter
func (v *Validator) applyValidationRules(param ParameterConfig, value interface{}) error {
	validation := param.Validation

	// Pattern validation for strings
	if validation.Pattern != "" {
		str, ok := value.(string)
		if !ok {
			return nil // Pattern only applies to strings
		}

		pattern, err := regexp.Compile(validation.Pattern)
		if err != nil {
			return NewValidationError(param.Name, validation.Pattern, "pattern",
				"invalid validation pattern")
		}

		if !pattern.MatchString(str) {
			return NewValidationError(param.Name, str, "pattern",
				fmt.Sprintf("value does not match pattern: %s", validation.Pattern))
		}
	}

	// Length validation for strings
	if str, ok := value.(string); ok {
		if validation.MinLength > 0 && len(str) < validation.MinLength {
			return NewValidationError(param.Name, str, "minLength",
				fmt.Sprintf("value length %d is less than minimum %d", len(str), validation.MinLength))
		}

		if validation.MaxLength > 0 && len(str) > validation.MaxLength {
			return NewValidationError(param.Name, str, "maxLength",
				fmt.Sprintf("value length %d exceeds maximum %d", len(str), validation.MaxLength))
		}
	}

	// Enum validation
	if len(validation.Enum) > 0 {
		found := false
		strValue := fmt.Sprintf("%v", value)

		enumStrings := make([]string, len(validation.Enum))
		for i, allowed := range validation.Enum {
			enumStrings[i] = fmt.Sprintf("%v", allowed)
			if strValue == enumStrings[i] {
				found = true
			}
		}

		if !found {
			return NewValidationError(param.Name, strValue, "enum",
				fmt.Sprintf("value must be one of: %s", strings.Join(enumStrings, ", ")))
		}
	}

	// Numeric range validation not supported yet in ValidationConfig

	return nil
}

// ValidateEndpoint validates an endpoint configuration
func (v *Validator) ValidateEndpoint(endpoint EndpointConfig) error {
	if endpoint.ID == "" {
		return NewConfigurationError("endpoint.id", "", "endpoint ID is required", nil)
	}

	if endpoint.Name == "" {
		return NewConfigurationError("endpoint.name", "", "endpoint name is required", nil)
	}

	if endpoint.Method == "" {
		return NewConfigurationError("endpoint.method", "", "endpoint method is required", nil)
	}

	// Validate HTTP method
	validMethods := map[string]bool{
		"GET":     true,
		"POST":    true,
		"PUT":     true,
		"DELETE":  true,
		"PATCH":   true,
		"HEAD":    true,
		"OPTIONS": true,
	}

	if !validMethods[endpoint.Method] {
		return NewConfigurationError("endpoint.method", endpoint.Method,
			"invalid HTTP method", nil)
	}

	if endpoint.Path == "" {
		return NewConfigurationError("endpoint.path", "", "endpoint path is required", nil)
	}

	// Validate parameters
	for i, param := range endpoint.Parameters {
		if err := v.validateParameterConfig(param); err != nil {
			return NewConfigurationError(fmt.Sprintf("endpoint.parameters[%d]", i), "",
				err.Error(), nil)
		}
	}

	// Validate response configuration
	if endpoint.Response.Type != "" {
		validTypes := map[string]bool{
			"json":   true,
			"text":   true,
			"binary": true,
		}

		if !validTypes[string(endpoint.Response.Type)] {
			return NewConfigurationError("endpoint.response.type", string(endpoint.Response.Type),
				"invalid response type", nil)
		}
	}

	return nil
}

// validateParameterConfig validates a parameter configuration
func (v *Validator) validateParameterConfig(param ParameterConfig) error {
	if param.Name == "" {
		return fmt.Errorf("parameter name is required")
	}

	if param.Type == "" {
		return fmt.Errorf("parameter type is required for %s", param.Name)
	}

	validTypes := map[string]bool{
		"string":  true,
		"number":  true,
		"boolean": true,
		"array":   true,
		"object":  true,
	}

	if !validTypes[string(param.Type)] {
		return fmt.Errorf("invalid parameter type for %s: %s", param.Name, param.Type)
	}

	if param.Location == "" {
		return fmt.Errorf("parameter location is required for %s", param.Name)
	}

	validLocations := map[string]bool{
		"path":   true,
		"query":  true,
		"body":   true,
		"header": true,
	}

	if !validLocations[string(param.Location)] {
		return fmt.Errorf("invalid parameter location for %s: %s", param.Name, param.Location)
	}

	// Validate transformation if present
	if param.Transform != nil {
		if param.Transform.TargetName == "" {
			return fmt.Errorf("transform target name is required for %s", param.Name)
		}
		if param.Transform.Expression == "" {
			return fmt.Errorf("transform expression is required for %s", param.Name)
		}
	}

	return nil
}
