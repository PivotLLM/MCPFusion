/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package admin

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/global"
)

// AuthCodeCommand holds the auth code generation flags.
type AuthCodeCommand struct {
	Service string // -auth-code: service to generate the code for
	URL     string // -auth-url: external URL of this server
	Token   string // -auth-token: API token prefix or hash identifying the tenant
}

// Requested reports whether auth code generation was asked for.
func (c AuthCodeCommand) Requested() bool {
	return c.Service != ""
}

// Run generates an auth code for use with fusion-auth and prints it.
func (c AuthCodeCommand) Run(database db.Database, logger global.Logger) error {
	if c.URL == "" {
		return fmt.Errorf("-auth-url is required with -auth-code")
	}

	// Resolve the tenant hash from API tokens
	tokens, err := database.ListAPITokens()
	if err != nil {
		return fmt.Errorf("failed to list API tokens: %w", err)
	}

	if len(tokens) == 0 {
		return &GuidanceError{
			Err:      fmt.Errorf("no API tokens found"),
			Guidance: fmt.Sprintf("Create one with: %s -token-add \"Description\"", os.Args[0]),
		}
	}

	var tenantHash string
	if len(tokens) == 1 {
		tenantHash = tokens[0].Hash
	} else {
		// Multiple tokens — require -auth-token to disambiguate
		if c.Token == "" {
			return &GuidanceError{
				Err:      fmt.Errorf("multiple API tokens found"),
				Guidance: "Use -auth-token to specify which token's tenant to use",
			}
		}
		resolvedHash, err := database.ResolveAPIToken(c.Token)
		if err != nil {
			return fmt.Errorf("failed to resolve API token '%s': %w", c.Token, err)
		}
		tenantHash = resolvedHash
	}

	// Create the auth code with 15-minute TTL
	code, err := database.CreateAuthCode(tenantHash, c.Service, 15*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to create auth code: %w", err)
	}

	// Build the blob
	blob := fusion.AuthCodeBlob{
		URL:     c.URL,
		Code:    code,
		Service: c.Service,
	}

	blobJSON, err := json.Marshal(blob)
	if err != nil {
		return fmt.Errorf("failed to marshal auth code blob: %w", err)
	}

	encoded := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(blobJSON)

	fmt.Printf("\nAuth code generated successfully\n")
	fmt.Printf("\n")
	fmt.Printf("Service:  %s\n", c.Service)
	fmt.Printf("Server:   %s\n", c.URL)
	fmt.Printf("Expires:  15 minutes\n")
	fmt.Printf("\n")
	fmt.Printf("Run fusion-auth with:\n")
	fmt.Printf("  ./fusion-auth %s\n", encoded)
	fmt.Printf("\n")

	logger.Infof("Generated auth code for service %s (tenant %s)", c.Service, (&fusion.TenantContext{TenantHash: tenantHash}).ShortHash())
	return nil
}
