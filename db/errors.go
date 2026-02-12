/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package db

import (
	"errors"
	"fmt"
)

// Base error types
//
//goland:noinspection GoUnusedGlobalVariable
var (
	ErrTenantNotFound   = errors.New("tenant not found")
	ErrTokenNotFound    = errors.New("token not found")
	ErrServiceNotFound  = errors.New("service not found")
	ErrInvalidToken     = errors.New("invalid token")
	ErrInvalidHash      = errors.New("invalid hash")
	ErrDuplicateToken   = errors.New("duplicate token")
	ErrDatabaseClosed   = errors.New("database is closed")
	ErrInvalidBucket    = errors.New("invalid bucket structure")
	ErrCorruptedData    = errors.New("corrupted data")
	ErrPermissionDenied = errors.New("permission denied")
	ErrUserNotFound     = errors.New("user not found")
	ErrUserExists       = errors.New("user already exists")
	ErrKeyAlreadyLinked = errors.New("API key already linked to a user")
	ErrKnowledgeNotFound = errors.New("knowledge entry not found")
)

// DatabaseError represents a database-specific error with context
type DatabaseError struct {
	Op       string // Operation that failed
	Err      error  // Underlying error
	TenantID string // Tenant context (if applicable)
	Service  string // Service context (if applicable)
}

func (e *DatabaseError) Error() string {
	if e.TenantID != "" && e.Service != "" {
		return fmt.Sprintf("db %s (tenant: %s, service: %s): %v", e.Op, e.TenantID[:8], e.Service, e.Err)
	}
	if e.TenantID != "" {
		return fmt.Sprintf("db %s (tenant: %s): %v", e.Op, e.TenantID[:8], e.Err)
	}
	return fmt.Sprintf("db %s: %v", e.Op, e.Err)
}

func (e *DatabaseError) Unwrap() error {
	return e.Err
}

// ValidationError represents a data validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s' (value: %v): %s", e.Field, e.Value, e.Message)
}

// TokenError represents a token-related error
type TokenError struct {
	Type    string // "api", "oauth", "credential"
	TokenID string // Token identifier
	Err     error  // Underlying error
}

func (e *TokenError) Error() string {
	return fmt.Sprintf("%s token error (id: %s): %v", e.Type, e.TokenID, e.Err)
}

func (e *TokenError) Unwrap() error {
	return e.Err
}

// Helper functions for creating specific errors

// NewDatabaseError creates a new DatabaseError
func NewDatabaseError(op string, err error) *DatabaseError {
	return &DatabaseError{
		Op:  op,
		Err: err,
	}
}

// NewDatabaseErrorWithContext creates a new DatabaseError with tenant and service context
func NewDatabaseErrorWithContext(op string, err error, tenantID, service string) *DatabaseError {
	return &DatabaseError{
		Op:       op,
		Err:      err,
		TenantID: tenantID,
		Service:  service,
	}
}

// NewValidationError creates a new ValidationError
func NewValidationError(field string, value interface{}, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// NewTokenError creates a new TokenError
func NewTokenError(tokenType, tokenID string, err error) *TokenError {
	return &TokenError{
		Type:    tokenType,
		TokenID: tokenID,
		Err:     err,
	}
}

// IsNotFound checks if an error is a "not found" type error
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, ErrTenantNotFound) ||
		errors.Is(err, ErrTokenNotFound) ||
		errors.Is(err, ErrServiceNotFound) ||
		errors.Is(err, ErrUserNotFound) ||
		errors.Is(err, ErrKnowledgeNotFound)
}

// IsDatabaseError checks if an error is a DatabaseError
//
//goland:noinspection GoUnusedExportedFunction
func IsDatabaseError(err error) bool {
	var dbErr *DatabaseError
	return errors.As(err, &dbErr)
}

// IsValidationError checks if an error is a ValidationError
func IsValidationError(err error) bool {
	var valErr *ValidationError
	return errors.As(err, &valErr)
}

// IsTokenError checks if an error is a TokenError
//
//goland:noinspection GoUnusedExportedFunction
func IsTokenError(err error) bool {
	var tokErr *TokenError
	return errors.As(err, &tokErr)
}
