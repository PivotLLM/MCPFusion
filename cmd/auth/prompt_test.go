/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package main

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/PivotLLM/MCPFusion/cmd/auth/mcp"
)

func TestPromptsForCredentials(t *testing.T) {
	tests := []struct {
		name string
		cfg  *mcp.ServiceConfigData
		want bool
	}{
		{"nil config", nil, false},
		{"user_credentials without fields", &mcp.ServiceConfigData{AuthTypeStr: "user_credentials"}, true},
		{"session_jwt with fields", &mcp.ServiceConfigData{AuthTypeStr: "session_jwt",
			Fields: []mcp.CredentialField{{Name: "username"}}}, true},
		{"session_jwt without fields", &mcp.ServiceConfigData{AuthTypeStr: "session_jwt"}, false},
		{"oauth2_external", &mcp.ServiceConfigData{AuthTypeStr: "oauth2_external"}, false},
		{"unknown type but fields present", &mcp.ServiceConfigData{Fields: []mcp.CredentialField{{Name: "k"}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := promptsForCredentials(tt.cfg); got != tt.want {
				t.Errorf("promptsForCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPromptCredentialFields(t *testing.T) {
	fields := []mcp.CredentialField{
		{Name: "username", Label: "UnifyEM username", Description: "Your admin username"},
		{Name: "password", Label: "UnifyEM password", Secret: true},
	}

	t.Run("plain and secret fields", func(t *testing.T) {
		in := bufio.NewReader(strings.NewReader("  alice  \n"))
		var out bytes.Buffer
		secretCalls := 0
		values, err := promptCredentialFields(fields, in, &out, func() (string, error) {
			secretCalls++
			return "s3cret\n", nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if values["username"] != "alice" || values["password"] != "s3cret" {
			t.Errorf("values = %v", values)
		}
		if secretCalls != 1 {
			t.Errorf("secret reader called %d times, want 1", secretCalls)
		}
		text := out.String()
		if !strings.Contains(text, "UnifyEM username: Your admin username") || !strings.Contains(text, "Enter UnifyEM password: ") {
			t.Errorf("prompts missing from output: %q", text)
		}
		if strings.Contains(text, "s3cret") || strings.Contains(text, "alice") {
			t.Errorf("prompt output must not echo values: %q", text)
		}
	})

	t.Run("final line without newline is accepted", func(t *testing.T) {
		in := bufio.NewReader(strings.NewReader("bob"))
		values, err := promptCredentialFields(fields[:1], in, &bytes.Buffer{}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if values["username"] != "bob" {
			t.Errorf("values = %v", values)
		}
	})

	t.Run("empty value is rejected", func(t *testing.T) {
		in := bufio.NewReader(strings.NewReader("\n"))
		_, err := promptCredentialFields(fields[:1], in, &bytes.Buffer{}, nil)
		if err == nil || !strings.Contains(err.Error(), "'username' cannot be empty") {
			t.Errorf("expected empty-value error, got %v", err)
		}
	})

	t.Run("secret reader failure is reported", func(t *testing.T) {
		in := bufio.NewReader(strings.NewReader("alice\n"))
		_, err := promptCredentialFields(fields, in, &bytes.Buffer{}, func() (string, error) {
			return "", errors.New("tty gone")
		})
		if err == nil || !strings.Contains(err.Error(), "'password'") {
			t.Errorf("expected password read error, got %v", err)
		}
	})

	t.Run("label falls back to name", func(t *testing.T) {
		in := bufio.NewReader(strings.NewReader("v\n"))
		var out bytes.Buffer
		if _, err := promptCredentialFields([]mcp.CredentialField{{Name: "api_key"}}, in, &out, nil); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "Enter api_key: ") {
			t.Errorf("output = %q", out.String())
		}
	})

	t.Run("piped secret input falls back to line read", func(t *testing.T) {
		// Not a terminal under go test, so terminalSecretReader must read the line.
		in := bufio.NewReader(strings.NewReader("piped-secret\n"))
		values, err := promptCredentialFields(fields[1:], in, &bytes.Buffer{}, terminalSecretReader(in))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if values["password"] != "piped-secret" {
			t.Errorf("values = %v", values)
		}
	})
}
