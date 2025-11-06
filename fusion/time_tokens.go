/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
)

// TimeTokenProcessor handles time token substitution in parameter values
type TimeTokenProcessor struct {
	logger         global.Logger
	daysRegex      *regexp.Regexp
	hoursRegex     *regexp.Regexp
	minsRegex      *regexp.Regexp
	daysPlusRegex  *regexp.Regexp
	hoursPlusRegex *regexp.Regexp
	minsPlusRegex  *regexp.Regexp
}

// NewTimeTokenProcessor creates a new time token processor
func NewTimeTokenProcessor(logger global.Logger) *TimeTokenProcessor {
	return &TimeTokenProcessor{
		logger:         logger,
		daysRegex:      regexp.MustCompile(`#DAYS-(\d+)`),
		hoursRegex:     regexp.MustCompile(`#HOURS-(\d+)`),
		minsRegex:      regexp.MustCompile(`#MINS-(\d+)`),
		daysPlusRegex:  regexp.MustCompile(`#DAYS\+(\d+)`),
		hoursPlusRegex: regexp.MustCompile(`#HOURS\+(\d+)`),
		minsPlusRegex:  regexp.MustCompile(`#MINS\+(\d+)`),
	}
}

// ProcessValue processes a parameter value and replaces any time tokens
func (ttp *TimeTokenProcessor) ProcessValue(value interface{}) interface{} {
	// Only process string values
	if strValue, ok := value.(string); ok {
		processed := ttp.substituteTimeTokens(strValue)
		if processed != strValue && ttp.logger != nil {
			ttp.logger.Debugf("Time token substitution: '%s' -> '%s'", strValue, processed)
		}
		return processed
	}

	// Return non-string values unchanged
	return value
}

// substituteTimeTokens replaces time tokens in a string value
func (ttp *TimeTokenProcessor) substituteTimeTokens(value string) string {
	result := value

	// Process #DAYS-N tokens
	result = ttp.daysRegex.ReplaceAllStringFunc(result, func(match string) string {
		// Extract the number from the match
		matches := ttp.daysRegex.FindStringSubmatch(match)
		if len(matches) != 2 {
			if ttp.logger != nil {
				ttp.logger.Errorf("Invalid #DAYS token format: %s", match)
			}
			return match // Return original if parsing fails
		}

		days, err := strconv.Atoi(matches[1])
		if err != nil {
			if ttp.logger != nil {
				ttp.logger.Errorf("Invalid number in #DAYS token: %s", matches[1])
			}
			return match // Return original if parsing fails
		}

		// Calculate N days ago at 00:00:00 UTC
		timestamp := time.Now().UTC().AddDate(0, 0, -days).Truncate(24 * time.Hour)

		// Format as ISO 8601
		return timestamp.Format(time.RFC3339)
	})

	// Process #HOURS-N tokens
	result = ttp.hoursRegex.ReplaceAllStringFunc(result, func(match string) string {
		// Extract the number from the match
		matches := ttp.hoursRegex.FindStringSubmatch(match)
		if len(matches) != 2 {
			if ttp.logger != nil {
				ttp.logger.Errorf("Invalid #HOURS token format: %s", match)
			}
			return match // Return original if parsing fails
		}

		hours, err := strconv.Atoi(matches[1])
		if err != nil {
			if ttp.logger != nil {
				ttp.logger.Errorf("Invalid number in #HOURS token: %s", matches[1])
			}
			return match // Return original if parsing fails
		}

		// Calculate N hours ago from current time
		timestamp := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)

		// Format as ISO 8601
		return timestamp.Format(time.RFC3339)
	})

	// Process #MINS-N tokens
	result = ttp.minsRegex.ReplaceAllStringFunc(result, func(match string) string {
		// Extract the number from the match
		matches := ttp.minsRegex.FindStringSubmatch(match)
		if len(matches) != 2 {
			if ttp.logger != nil {
				ttp.logger.Errorf("Invalid #MINS token format: %s", match)
			}
			return match // Return original if parsing fails
		}

		mins, err := strconv.Atoi(matches[1])
		if err != nil {
			if ttp.logger != nil {
				ttp.logger.Errorf("Invalid number in #MINS token: %s", matches[1])
			}
			return match // Return original if parsing fails
		}

		// Calculate N minutes ago from current time
		timestamp := time.Now().UTC().Add(-time.Duration(mins) * time.Minute)

		// Format as ISO 8601
		return timestamp.Format(time.RFC3339)
	})

	// Process #DAYS+N tokens
	result = ttp.daysPlusRegex.ReplaceAllStringFunc(result, func(match string) string {
		// Extract the number from the match
		matches := ttp.daysPlusRegex.FindStringSubmatch(match)
		if len(matches) != 2 {
			if ttp.logger != nil {
				ttp.logger.Errorf("Invalid #DAYS+ token format: %s", match)
			}
			return match // Return original if parsing fails
		}

		days, err := strconv.Atoi(matches[1])
		if err != nil {
			if ttp.logger != nil {
				ttp.logger.Errorf("Invalid number in #DAYS+ token: %s", matches[1])
			}
			return match // Return original if parsing fails
		}

		// Calculate N days in the future at 00:00:00 UTC
		timestamp := time.Now().UTC().AddDate(0, 0, days).Truncate(24 * time.Hour)

		// Format as ISO 8601
		return timestamp.Format(time.RFC3339)
	})

	// Process #HOURS+N tokens
	result = ttp.hoursPlusRegex.ReplaceAllStringFunc(result, func(match string) string {
		// Extract the number from the match
		matches := ttp.hoursPlusRegex.FindStringSubmatch(match)
		if len(matches) != 2 {
			if ttp.logger != nil {
				ttp.logger.Errorf("Invalid #HOURS+ token format: %s", match)
			}
			return match // Return original if parsing fails
		}

		hours, err := strconv.Atoi(matches[1])
		if err != nil {
			if ttp.logger != nil {
				ttp.logger.Errorf("Invalid number in #HOURS+ token: %s", matches[1])
			}
			return match // Return original if parsing fails
		}

		// Calculate N hours in the future from current time
		timestamp := time.Now().UTC().Add(time.Duration(hours) * time.Hour)

		// Format as ISO 8601
		return timestamp.Format(time.RFC3339)
	})

	// Process #MINS+N tokens
	result = ttp.minsPlusRegex.ReplaceAllStringFunc(result, func(match string) string {
		// Extract the number from the match
		matches := ttp.minsPlusRegex.FindStringSubmatch(match)
		if len(matches) != 2 {
			if ttp.logger != nil {
				ttp.logger.Errorf("Invalid #MINS+ token format: %s", match)
			}
			return match // Return original if parsing fails
		}

		mins, err := strconv.Atoi(matches[1])
		if err != nil {
			if ttp.logger != nil {
				ttp.logger.Errorf("Invalid number in #MINS+ token: %s", matches[1])
			}
			return match // Return original if parsing fails
		}

		// Calculate N minutes in the future from current time
		timestamp := time.Now().UTC().Add(time.Duration(mins) * time.Minute)

		// Format as ISO 8601
		return timestamp.Format(time.RFC3339)
	})

	return result
}

// HasTimeTokens checks if a string contains any time tokens
func (ttp *TimeTokenProcessor) HasTimeTokens(value string) bool {
	return ttp.daysRegex.MatchString(value) || ttp.hoursRegex.MatchString(value) ||
		ttp.minsRegex.MatchString(value) || ttp.daysPlusRegex.MatchString(value) ||
		ttp.hoursPlusRegex.MatchString(value) || ttp.minsPlusRegex.MatchString(value)
}

// ValidateTimeTokens validates that time tokens in a string are well-formed
func (ttp *TimeTokenProcessor) ValidateTimeTokens(value string) error {
	// Find all potential time tokens
	daysMatches := ttp.daysRegex.FindAllStringSubmatch(value, -1)
	hoursMatches := ttp.hoursRegex.FindAllStringSubmatch(value, -1)
	minsMatches := ttp.minsRegex.FindAllStringSubmatch(value, -1)
	daysPlusMatches := ttp.daysPlusRegex.FindAllStringSubmatch(value, -1)
	hoursPlusMatches := ttp.hoursPlusRegex.FindAllStringSubmatch(value, -1)
	minsPlusMatches := ttp.minsPlusRegex.FindAllStringSubmatch(value, -1)

	// Validate DAYS tokens
	for _, match := range daysMatches {
		if len(match) != 2 {
			return fmt.Errorf("invalid #DAYS token format: %s", match[0])
		}

		days, err := strconv.Atoi(match[1])
		if err != nil {
			return fmt.Errorf("invalid number in #DAYS token: %s", match[1])
		}

		// Validate reasonable range (0-365 days)
		if days < 0 || days > 365 {
			return fmt.Errorf("days value out of range (0-365): %d", days)
		}
	}

	// Validate HOURS tokens
	for _, match := range hoursMatches {
		if len(match) != 2 {
			return fmt.Errorf("invalid #HOURS token format: %s", match[0])
		}

		hours, err := strconv.Atoi(match[1])
		if err != nil {
			return fmt.Errorf("invalid number in #HOURS token: %s", match[1])
		}

		// Validate reasonable range (0-8760 hours = 1 year)
		if hours < 0 || hours > 8760 {
			return fmt.Errorf("hours value out of range (0-8760): %d", hours)
		}
	}

	// Validate DAYS+ tokens
	for _, match := range daysPlusMatches {
		if len(match) != 2 {
			return fmt.Errorf("invalid #DAYS+ token format: %s", match[0])
		}

		days, err := strconv.Atoi(match[1])
		if err != nil {
			return fmt.Errorf("invalid number in #DAYS+ token: %s", match[1])
		}

		// Validate reasonable range (0-365 days)
		if days < 0 || days > 365 {
			return fmt.Errorf("days value out of range (0-365): %d", days)
		}
	}

	// Validate HOURS+ tokens
	for _, match := range hoursPlusMatches {
		if len(match) != 2 {
			return fmt.Errorf("invalid #HOURS+ token format: %s", match[0])
		}

		hours, err := strconv.Atoi(match[1])
		if err != nil {
			return fmt.Errorf("invalid number in #HOURS+ token: %s", match[1])
		}

		// Validate reasonable range (0-8760 hours = 1 year)
		if hours < 0 || hours > 8760 {
			return fmt.Errorf("hours value out of range (0-8760): %d", hours)
		}
	}

	// Validate MINS tokens
	for _, match := range minsMatches {
		if len(match) != 2 {
			return fmt.Errorf("invalid #MINS token format: %s", match[0])
		}

		mins, err := strconv.Atoi(match[1])
		if err != nil {
			return fmt.Errorf("invalid number in #MINS token: %s", match[1])
		}

		// Validate reasonable range (0-525600 minutes = 1 year)
		if mins < 0 || mins > 525600 {
			return fmt.Errorf("minutes value out of range (0-525600): %d", mins)
		}
	}

	// Validate MINS+ tokens
	for _, match := range minsPlusMatches {
		if len(match) != 2 {
			return fmt.Errorf("invalid #MINS+ token format: %s", match[0])
		}

		mins, err := strconv.Atoi(match[1])
		if err != nil {
			return fmt.Errorf("invalid number in #MINS+ token: %s", match[1])
		}

		// Validate reasonable range (0-525600 minutes = 1 year)
		if mins < 0 || mins > 525600 {
			return fmt.Errorf("minutes value out of range (0-525600): %d", mins)
		}
	}

	return nil
}

// GetSupportedTokens returns information about supported time token patterns
func (ttp *TimeTokenProcessor) GetSupportedTokens() map[string]string {
	return map[string]string{
		"#DAYS-N":  "N days ago at 00:00:00 UTC (e.g., #DAYS-0 = today at midnight, #DAYS-3 = 3 days ago at midnight)",
		"#HOURS-N": "N hours ago from current time (e.g., #HOURS-6 = 6 hours ago, #HOURS-24 = 24 hours ago)",
		"#MINS-N":  "N minutes ago from current time (e.g., #MINS-5 = 5 minutes ago, #MINS-30 = 30 minutes ago)",
		"#DAYS+N":  "N days in the future at 00:00:00 UTC (e.g., #DAYS+0 = today at midnight, #DAYS+7 = 7 days from now at midnight)",
		"#HOURS+N": "N hours in the future from current time (e.g., #HOURS+6 = 6 hours from now, #HOURS+24 = 24 hours from now)",
		"#MINS+N":  "N minutes in the future from current time (e.g., #MINS+5 = 5 minutes from now, #MINS+30 = 30 minutes from now)",
	}
}

// ProcessParameterArgs processes all parameter arguments and substitutes time tokens
func (ttp *TimeTokenProcessor) ProcessParameterArgs(args map[string]interface{}) map[string]interface{} {
	if args == nil {
		return nil
	}

	processed := make(map[string]interface{})
	hasTokens := false

	for key, value := range args {
		processedValue := ttp.ProcessValue(value)
		processed[key] = processedValue

		// Check if any substitution occurred
		if fmt.Sprintf("%v", processedValue) != fmt.Sprintf("%v", value) {
			hasTokens = true
		}
	}

	if hasTokens && ttp.logger != nil {
		ttp.logger.Infof("Processed time tokens in parameter arguments")
	}

	return processed
}

// SubstituteTimeTokensInParameterValue is a convenience function for processing individual parameter values
// This can be used when processing parameters in different locations (path, query, body, header)
func SubstituteTimeTokensInParameterValue(value interface{}, logger global.Logger) interface{} {
	processor := NewTimeTokenProcessor(logger)
	return processor.ProcessValue(value)
}

// SubstituteTimeTokensInString is a convenience function for processing string values
func SubstituteTimeTokensInString(value string, logger global.Logger) string {
	processor := NewTimeTokenProcessor(logger)
	result := processor.ProcessValue(value)
	if strResult, ok := result.(string); ok {
		return strResult
	}
	return value // Fallback to original if something goes wrong
}
