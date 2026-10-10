/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PivotLLM/MCPFusion/cmd/auth/config"
	"github.com/PivotLLM/MCPFusion/cmd/auth/providers"
)

// A failed connection returns guidance separately, so the error string itself
// stays a single plain line.
func TestExecuteOAuthFlowConnectFailureGuidance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	cfg := &config.Config{Service: "google", FusionURL: srv.URL, APIToken: "token"}
	err := executeOAuthFlow(context.Background(), cfg, &cliFlags{}, providers.NewProviderRegistry())
	if err == nil {
		t.Fatal("executeOAuthFlow: want error, got nil")
	}

	var ge *guidanceError
	if !errors.As(err, &ge) {
		t.Fatalf("executeOAuthFlow error %T is not a *guidanceError", err)
	}
	if !strings.Contains(ge.guidance, srv.URL) {
		t.Errorf("guidance %q does not name the server URL %q", ge.guidance, srv.URL)
	}
	if msg := err.Error(); strings.Contains(msg, "\n") || !strings.HasPrefix(msg, "failed to connect to MCPFusion server") {
		t.Errorf("error string %q: want a single line starting with the failure", msg)
	}
}
