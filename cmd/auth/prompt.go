/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/PivotLLM/MCPFusion/cmd/auth/mcp"
)

// promptsForCredentials reports whether the server-side config for a service
// means fusion-auth should prompt for field values instead of running an
// OAuth flow: user_credentials services always, and any service (such as
// session_jwt with a credentials block) that advertises fields.
func promptsForCredentials(cfg *mcp.ServiceConfigData) bool {
	if cfg == nil {
		return false
	}
	if authType, ok := cfg.AuthType(); ok && authType == "user_credentials" {
		return true
	}
	return len(cfg.Fields) > 0
}

// secretReader reads one line of input without echoing it.
type secretReader func() (string, error)

// terminalSecretReader masks input when stdin is a terminal and falls back to
// a plain line read otherwise (pipes, tests), so scripted use keeps working.
func terminalSecretReader(in *bufio.Reader) secretReader {
	return func() (string, error) {
		fd := int(os.Stdin.Fd())
		if term.IsTerminal(fd) {
			raw, err := term.ReadPassword(fd)
			fmt.Println() // ReadPassword swallows the newline the user typed
			if err != nil {
				return "", err
			}
			return string(raw), nil
		}
		line, err := in.ReadString('\n')
		if err != nil && line == "" {
			return "", err
		}
		return line, nil
	}
}

// promptCredentialFields asks the user for each field in order and returns the
// collected values keyed by field name. Fields marked secret are read through
// readSecret so they are not echoed. Every value is required.
func promptCredentialFields(fields []mcp.CredentialField, in *bufio.Reader, out io.Writer, readSecret secretReader) (map[string]string, error) {
	values := make(map[string]string, len(fields))
	for _, field := range fields {
		label := field.Label
		if label == "" {
			label = field.Name
		}
		if field.Description != "" {
			fmt.Fprintf(out, "%s: %s\n", label, field.Description)
		}
		fmt.Fprintf(out, "Enter %s: ", label)

		var (
			value string
			err   error
		)
		if field.Secret {
			value, err = readSecret()
		} else {
			value, err = in.ReadString('\n')
			if err != nil && value != "" {
				err = nil // accept a final line without a trailing newline
			}
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read input for '%s': %w", field.Name, err)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("value for '%s' cannot be empty", field.Name)
		}
		values[field.Name] = value
	}
	return values, nil
}
