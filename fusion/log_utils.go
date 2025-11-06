/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"fmt"
	"strings"
	"time"
)

// SanitizeTokenForLogging safely truncates and redacts token values for logging
// It preserves a small prefix for debugging while hiding sensitive parts
func SanitizeTokenForLogging(token string) string {
	if len(token) == 0 {
		return "[EMPTY]"
	}

	// For very short tokens, fully redact
	if len(token) <= 8 {
		return "[REDACTED]"
	}

	// For longer tokens, show first 8 characters + redacted suffix
	return token[:8] + "...[REDACTED]"
}

// SanitizeStringForLogging checks if a string might contain sensitive information
// and redacts it appropriately. It looks for patterns that suggest tokens, keys, etc.
func SanitizeStringForLogging(value string) string {
	if len(value) == 0 {
		return value
	}

	// Convert to lowercase for pattern matching
	lowerValue := strings.ToLower(value)

	// Check for patterns that suggest this might be a token or key
	sensitivePatterns := []string{
		"bearer ",
		"token=",
		"key=",
		"secret=",
		"password=",
		"auth=",
		"authorization=",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerValue, pattern) {
			return SanitizeTokenForLogging(value)
		}
	}

	// Check if it looks like a JWT token (has dots and is long)
	if strings.Count(value, ".") >= 2 && len(value) > 50 {
		return SanitizeTokenForLogging(value)
	}

	// Check if it looks like a long random string (potential token)
	if len(value) > 32 && !strings.Contains(value, " ") {
		// Contains mostly alphanumeric characters - might be a token
		alphanumericCount := 0
		for _, r := range value {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
				(r >= '0' && r <= '9') || r == '-' || r == '_' {
				alphanumericCount++
			}
		}

		// If more than 80% alphanumeric, treat as potential token
		if float64(alphanumericCount)/float64(len(value)) > 0.8 {
			return SanitizeTokenForLogging(value)
		}
	}

	// Return original value if no sensitive patterns detected
	return value
}

// SanitizeHeaderForLogging safely formats header values for logging
// It automatically redacts known sensitive headers and applies pattern detection
func SanitizeHeaderForLogging(headerName, headerValue string) string {
	lowerName := strings.ToLower(headerName)

	// Known sensitive headers - always redact
	sensitiveHeaders := []string{
		"authorization",
		"api-key",
		"x-api-key",
		"x-auth-token",
		"x-access-token",
		"bearer",
		"token",
		"secret",
		"password",
		"cookie",
		"set-cookie",
	}

	for _, sensitive := range sensitiveHeaders {
		if strings.Contains(lowerName, sensitive) {
			if len(headerValue) > 0 {
				return fmt.Sprintf("[REDACTED - length %d]", len(headerValue))
			}
			return "[REDACTED]"
		}
	}

	// Apply general string sanitization for other headers
	return SanitizeStringForLogging(headerValue)
}

// SanitizeCacheKeyForLogging creates a safe representation of cache keys for logging
// It truncates long tenant hashes and service names to prevent log pollution
func SanitizeCacheKeyForLogging(key string) string {
	if len(key) == 0 {
		return key
	}

	// Check if it's a tenant cache key format: tenant:{hash}:token:{service}
	parts := strings.Split(key, ":")
	if len(parts) == 4 && parts[0] == "tenant" && parts[2] == "token" {
		tenantHash := parts[1]
		serviceName := parts[3]

		// Truncate tenant hash to first 12 characters
		truncatedHash := tenantHash
		if len(tenantHash) > 12 {
			truncatedHash = tenantHash[:12] + "..."
		}

		// Truncate service name if too long
		truncatedService := serviceName
		if len(serviceName) > 20 {
			truncatedService = serviceName[:20] + "..."
		}

		return fmt.Sprintf("tenant:%s:token:%s", truncatedHash, truncatedService)
	}

	// For other key formats, truncate if too long
	if len(key) > 50 {
		return key[:50] + "...[TRUNCATED]"
	}

	return key
}

// FormatExpiryForLogging creates a safe, relative time representation for logging
// It avoids exposing exact timestamps while preserving debugging utility
func FormatExpiryForLogging(expiresAt *time.Time) string {
	if expiresAt == nil {
		return "no_expiry"
	}

	timeUntilExpiry := time.Until(*expiresAt)
	if timeUntilExpiry <= 0 {
		return "expired"
	}

	// Round to nearest minute for cleaner logs
	return fmt.Sprintf("expires_in=%v", timeUntilExpiry.Round(time.Minute))
}
