/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// BodyEncoder encodes a set of flat parameters into a single encoded string.
// Implementations transform key-value parameter maps into API-specific formats.
type BodyEncoder interface {
	Encode(params map[string]interface{}) (string, error)
}

// bodyEncoders is the registry of available body encoders. Read-only after init.
var bodyEncoders = map[string]BodyEncoder{
	"rfc2822_base64url": &RFC2822Base64URLEncoder{},
}

// GetBodyEncoder returns the encoder registered under the given name.
func GetBodyEncoder(name string) (BodyEncoder, bool) {
	enc, ok := bodyEncoders[name]
	return enc, ok
}

// RFC2822Base64URLEncoder builds an RFC 2822 MIME message from flat email
// parameters (to, cc, bcc, subject, body) and returns it as a base64url-encoded
// string with no padding, as required by the Gmail API.
type RFC2822Base64URLEncoder struct{}

// sanitizeHeader strips CR and LF characters from a header value to prevent
// header injection attacks.
func sanitizeHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	return value
}

// Encode assembles an RFC 2822 message and base64url-encodes it.
func (e *RFC2822Base64URLEncoder) Encode(params map[string]interface{}) (string, error) {
	toString := func(key string) string {
		v, ok := params[key]
		if !ok || v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}

	to := sanitizeHeader(toString("to"))
	subject := sanitizeHeader(toString("subject"))
	body := toString("body")
	cc := sanitizeHeader(toString("cc"))
	bcc := sanitizeHeader(toString("bcc"))

	var msg strings.Builder
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	if to != "" {
		msg.WriteString("To: " + to + "\r\n")
	}
	if cc != "" {
		msg.WriteString("Cc: " + cc + "\r\n")
	}
	if bcc != "" {
		msg.WriteString("Bcc: " + bcc + "\r\n")
	}
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	encoded := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(msg.String()))
	return encoded, nil
}
