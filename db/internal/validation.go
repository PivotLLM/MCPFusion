/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package internal

import (
	"encoding/hex"
	"regexp"
	"strings"
)

// Validation constants
//
//goland:noinspection GoCommentStart
const (
	// Token constraints
	MinTokenLength = 32  // Minimum token length in characters
	MaxTokenLength = 128 // Maximum token length in characters
	HashLength     = 64  // SHA-256 hash length in hex
	PrefixLength   = 8   // Token prefix length

	// Description constraints
	MaxDescriptionLength = 256

	// Service name constraints
	MaxServiceNameLength = 64
	MinServiceNameLength = 1
)

// Regular expressions for validation
var (
	// Service name can contain alphanumeric, underscore, hyphen, dot, and space
	serviceNameRegex = regexp.MustCompile(`^[a-zA-Z0-9._\- ]+$`)

	// Hash must be valid hex string
	hashRegex = regexp.MustCompile(`^[a-fA-F0-9]+$`)

	// Token prefix validation (alphanumeric)
	prefixRegex = regexp.MustCompile(`^[a-zA-Z0-9]+$`)
)

// ValidateToken validates an API token format
func ValidateToken(token string) error {
	if len(token) < MinTokenLength {
		return &ValidationError{
			Field:   "token",
			Value:   len(token),
			Message: "token too short",
		}
	}

	if len(token) > MaxTokenLength {
		return &ValidationError{
			Field:   "token",
			Value:   len(token),
			Message: "token too long",
		}
	}

	// Check if token is valid hex
	if _, err := hex.DecodeString(token); err != nil {
		return &ValidationError{
			Field:   "token",
			Value:   token,
			Message: "token must be valid hexadecimal",
		}
	}

	return nil
}

// ValidateHash validates a hash format
func ValidateHash(hash string) error {
	if len(hash) != HashLength {
		return &ValidationError{
			Field:   "hash",
			Value:   len(hash),
			Message: "hash must be 64 characters (SHA-256)",
		}
	}

	if !hashRegex.MatchString(hash) {
		return &ValidationError{
			Field:   "hash",
			Value:   hash,
			Message: "hash must be valid hexadecimal",
		}
	}

	return nil
}

// ValidateServiceName validates a service name
func ValidateServiceName(serviceName string) error {
	if len(serviceName) < MinServiceNameLength {
		return &ValidationError{
			Field:   "service_name",
			Value:   len(serviceName),
			Message: "service name too short",
		}
	}

	if len(serviceName) > MaxServiceNameLength {
		return &ValidationError{
			Field:   "service_name",
			Value:   len(serviceName),
			Message: "service name too long",
		}
	}

	if !serviceNameRegex.MatchString(serviceName) {
		return &ValidationError{
			Field:   "service_name",
			Value:   serviceName,
			Message: "service name contains invalid characters",
		}
	}

	return nil
}

// ValidateDescription validates a description string
func ValidateDescription(description string) error {
	if len(description) > MaxDescriptionLength {
		return &ValidationError{
			Field:   "description",
			Value:   len(description),
			Message: "description too long",
		}
	}

	// Trim whitespace for validation
	trimmed := strings.TrimSpace(description)
	if len(trimmed) == 0 && len(description) > 0 {
		return &ValidationError{
			Field:   "description",
			Value:   description,
			Message: "description cannot be only whitespace",
		}
	}

	return nil
}

// ValidatePrefix validates a token prefix
//
//goland:noinspection GoUnusedExportedFunction
func ValidatePrefix(prefix string) error {
	if len(prefix) != PrefixLength {
		return &ValidationError{
			Field:   "prefix",
			Value:   len(prefix),
			Message: "prefix must be exactly 8 characters",
		}
	}

	if !prefixRegex.MatchString(prefix) {
		return &ValidationError{
			Field:   "prefix",
			Value:   prefix,
			Message: "prefix must be alphanumeric",
		}
	}

	return nil
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
