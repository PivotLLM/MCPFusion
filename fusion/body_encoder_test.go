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

func TestGetBodyEncoder_Known(t *testing.T) {
	enc, ok := GetBodyEncoder("rfc2822_base64url")
	assert.True(t, ok)
	assert.NotNil(t, enc)
}

func TestGetBodyEncoder_Unknown(t *testing.T) {
	enc, ok := GetBodyEncoder("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, enc)
}

func TestRFC2822Base64URLEncoder_BasicMessage(t *testing.T) {
	enc := &RFC2822Base64URLEncoder{}
	params := map[string]interface{}{
		"to":      "alice@example.com",
		"subject": "Hello",
		"body":    "Hi Alice!",
	}

	encoded, err := enc.Encode(params)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	// Decode and verify
	decoded, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(encoded)
	require.NoError(t, err)

	msg := string(decoded)
	assert.Contains(t, msg, "To: alice@example.com\r\n")
	assert.Contains(t, msg, "Subject: Hello\r\n")
	assert.Contains(t, msg, "MIME-Version: 1.0\r\n")
	assert.Contains(t, msg, "Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	assert.True(t, strings.HasSuffix(msg, "\r\n\r\nHi Alice!"))
	// No Cc or Bcc headers when not provided
	assert.NotContains(t, msg, "Cc:")
	assert.NotContains(t, msg, "Bcc:")
}

func TestRFC2822Base64URLEncoder_WithCcBcc(t *testing.T) {
	enc := &RFC2822Base64URLEncoder{}
	params := map[string]interface{}{
		"to":      "alice@example.com",
		"cc":      "bob@example.com",
		"bcc":     "charlie@example.com",
		"subject": "Team Update",
		"body":    "Please review.",
	}

	encoded, err := enc.Encode(params)
	require.NoError(t, err)

	decoded, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(encoded)
	require.NoError(t, err)

	msg := string(decoded)
	assert.Contains(t, msg, "To: alice@example.com\r\n")
	assert.Contains(t, msg, "Cc: bob@example.com\r\n")
	assert.Contains(t, msg, "Bcc: charlie@example.com\r\n")
	assert.Contains(t, msg, "Subject: Team Update\r\n")
}

func TestRFC2822Base64URLEncoder_EmptyOptionals(t *testing.T) {
	enc := &RFC2822Base64URLEncoder{}
	params := map[string]interface{}{
		"to":      "alice@example.com",
		"subject": "Test",
		"body":    "Content",
		"cc":      "",
		"bcc":     "",
	}

	encoded, err := enc.Encode(params)
	require.NoError(t, err)

	decoded, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(encoded)
	require.NoError(t, err)

	msg := string(decoded)
	assert.NotContains(t, msg, "Cc:")
	assert.NotContains(t, msg, "Bcc:")
}

func TestRFC2822Base64URLEncoder_NoPadding(t *testing.T) {
	enc := &RFC2822Base64URLEncoder{}
	params := map[string]interface{}{
		"to":      "x@y.com",
		"subject": "A",
		"body":    "B",
	}

	encoded, err := enc.Encode(params)
	require.NoError(t, err)

	// Must not contain padding characters
	assert.NotContains(t, encoded, "=")
}

func TestRFC2822Base64URLEncoder_URLSafeChars(t *testing.T) {
	enc := &RFC2822Base64URLEncoder{}
	params := map[string]interface{}{
		"to":      "test@example.com",
		"subject": "Special chars: +/=",
		"body":    "Body with special characters that might produce + and / in standard base64",
	}

	encoded, err := enc.Encode(params)
	require.NoError(t, err)

	// Must not contain standard base64 characters that are not URL-safe
	assert.NotContains(t, encoded, "+")
	assert.NotContains(t, encoded, "/")
}

func TestRFC2822Base64URLEncoder_RoundTrip(t *testing.T) {
	enc := &RFC2822Base64URLEncoder{}
	params := map[string]interface{}{
		"to":      "recipient@example.com",
		"cc":      "cc1@example.com, cc2@example.com",
		"subject": "Important: Q4 Results",
		"body":    "Please find the Q4 results attached.\n\nBest regards,\nSender",
	}

	encoded, err := enc.Encode(params)
	require.NoError(t, err)

	// Decode
	decoded, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(encoded)
	require.NoError(t, err)

	msg := string(decoded)

	// Verify all headers and body are present and correct
	assert.Contains(t, msg, "To: recipient@example.com\r\n")
	assert.Contains(t, msg, "Cc: cc1@example.com, cc2@example.com\r\n")
	assert.Contains(t, msg, "Subject: Important: Q4 Results\r\n")
	assert.Contains(t, msg, "Please find the Q4 results attached.\n\nBest regards,\nSender")
}

func TestRFC2822Base64URLEncoder_HeaderInjection(t *testing.T) {
	enc := &RFC2822Base64URLEncoder{}
	params := map[string]interface{}{
		"to":      "alice@example.com\r\nBcc: evil@attacker.com",
		"subject": "Normal Subject\r\nX-Injected: true",
		"body":    "Body content",
	}

	encoded, err := enc.Encode(params)
	require.NoError(t, err)

	decoded, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(encoded)
	require.NoError(t, err)

	msg := string(decoded)

	// CR/LF stripped from headers — injection attempt is flattened into the same
	// header line, so no separate "Bcc:" or "X-Injected:" header is created.
	// Count occurrences of "Bcc:" — should not appear as its own header
	lines := strings.Split(msg, "\r\n")
	bccHeaderCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "Bcc:") {
			bccHeaderCount++
		}
	}
	assert.Equal(t, 0, bccHeaderCount, "injected Bcc header must not exist as separate header line")

	// The To header should contain the flattened (sanitized) value
	assert.Contains(t, msg, "To: alice@example.comBcc: evil@attacker.com\r\n")
	// The Subject header should contain the flattened value
	assert.Contains(t, msg, "Subject: Normal SubjectX-Injected: true\r\n")
}
