/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/PivotLLM/MCPFusion/app"
	"github.com/PivotLLM/MCPFusion/internal/admin"
)

// options holds the parsed command line flags.
type options struct {
	debug   bool
	port    int
	noAuth  bool
	config  string
	help    bool
	version bool
	perf    bool

	token    admin.TokenCommand
	user     admin.UserCommand
	authCode admin.AuthCodeCommand
}

// parseFlags defines and parses the command line flags.
func parseFlags() options {
	var o options

	// Define command line flags
	flag.BoolVar(&o.debug, "debug", true, "Enable debug mode")
	flag.IntVar(&o.port, "port", 8888, "Port to listen on")
	flag.BoolVar(&o.noAuth, "no-auth", false, "Disable authentication (INSECURE - testing only)")
	flag.StringVar(&o.config, "config", "", "Comma-separated list of configuration files (optional)")
	flag.BoolVar(&o.help, "help", false, "Show help information")
	flag.BoolVar(&o.version, "version", false, "Show version information")

	// Token management subcommands
	flag.StringVar(&o.token.Add, "token-add", "", "Add new API token with description")
	flag.BoolVar(&o.token.List, "token-list", false, "List all API tokens")
	flag.StringVar(&o.token.Delete, "token-del", "", "Delete API token by prefix or hash")
	flag.StringVar(&o.token.User, "token-user", "", "User ID to link token to (use with -token-add)")

	// User management subcommands
	flag.StringVar(&o.user.Add, "user-add", "", "Add new user with description")
	flag.StringVar(&o.user.Token, "user-token", "", "Also create an API token with this description (use with -user-add)")
	flag.BoolVar(&o.user.List, "user-list", false, "List all users")
	flag.StringVar(&o.user.Delete, "user-delete", "", "Delete user by ID")
	flag.StringVar(&o.user.Link, "user-link", "", "Link API key to user (format: user_id:key_hash)")
	flag.StringVar(&o.user.Unlink, "user-unlink", "", "Unlink API key from user by key hash")

	// Auth code generation
	flag.StringVar(&o.authCode.Service, "auth-code", "", "Generate auth code for a service (e.g., google)")
	flag.StringVar(&o.authCode.URL, "auth-url", "", "External URL of this server (required with -auth-code)")
	flag.StringVar(&o.authCode.Token, "auth-token", "", "API token prefix/hash to identify tenant (for multi-token setups)")

	// Perf provider flag (never use in production)
	flag.BoolVar(&o.perf, "perf", false, "Enable perf/stress testing tools (never use in production)")

	flag.Usage = usage
	flag.Parse()
	return o
}

// printVersion prints the application identity and build details.
func printVersion() {
	fmt.Printf("%s %s\n%s\n%s\n", app.Name(), app.Version(), app.TagLine(), app.Copyright())
	buildTime, goVersion := app.BuildInfo()
	if buildTime != "" {
		fmt.Printf("Built: %s\n", buildTime)
	}
	fmt.Printf("Go:    %s\n", goVersion)
}

// usage prints the help text.
func usage() {
	fmt.Printf("%s - Multi-Tenant Model Context Protocol Server\n\n", app.Name())
	fmt.Printf("Usage:\n")
	fmt.Printf("  %s [options]\n\n", os.Args[0])
	fmt.Printf("Server Options:\n")
	fmt.Printf("  -config string\n")
	fmt.Printf("        Comma-separated list of configuration files (optional)\n")
	fmt.Printf("        Can also use MCP_FUSION_CONFIGS environment variable\n")
	fmt.Printf("  -debug\n")
	fmt.Printf("        Enable debug mode (default true)\n")
	fmt.Printf("  -help\n")
	fmt.Printf("        Show help information\n")
	fmt.Printf("  -no-auth\n")
	fmt.Printf("        Disable authentication (INSECURE - testing only)\n")
	fmt.Printf("  -port int\n")
	fmt.Printf("        Port to listen on (default 8888)\n")
	fmt.Printf("  -version\n")
	fmt.Printf("        Show version information\n\n")
	fmt.Printf("Token Management Commands:\n")
	fmt.Printf("  -token-add string\n")
	fmt.Printf("        Add new API token with description\n")
	fmt.Printf("  -token-user string\n")
	fmt.Printf("        User ID to link token to (use with -token-add)\n")
	fmt.Printf("  -token-list\n")
	fmt.Printf("        List all API tokens\n")
	fmt.Printf("  -token-del string\n")
	fmt.Printf("        Delete API token by prefix or hash\n\n")
	fmt.Printf("User Management Commands:\n")
	fmt.Printf("  -user-add string\n")
	fmt.Printf("        Add new user with description\n")
	fmt.Printf("  -user-token string\n")
	fmt.Printf("        Also create an API token with this description (use with -user-add)\n")
	fmt.Printf("  -user-list\n")
	fmt.Printf("        List all users and their linked API keys\n")
	fmt.Printf("  -user-delete string\n")
	fmt.Printf("        Delete user by ID\n")
	fmt.Printf("  -user-link string\n")
	fmt.Printf("        Link API key to user (format: user_id:key_hash)\n")
	fmt.Printf("  -user-unlink string\n")
	fmt.Printf("        Unlink API key from user by key hash\n\n")
	fmt.Printf("Auth Code Commands:\n")
	fmt.Printf("  -auth-code string\n")
	fmt.Printf("        Generate auth code for a service (e.g., google)\n")
	fmt.Printf("  -auth-url string\n")
	fmt.Printf("        External URL of this server (required with -auth-code)\n")
	fmt.Printf("  -auth-token string\n")
	fmt.Printf("        API token prefix/hash to identify tenant (for multi-token setups)\n\n")
	fmt.Printf("Environment Variables:\n")
	fmt.Printf("  MCP_FUSION_DB_DIR   Custom database directory (default: /opt/mcpfusion or ~/.mcpfusion)\n")
	fmt.Printf("  MCP_FUSION_DL_DIR   Directory for saving binary downloads (e.g. generated reports)\n\n")
	fmt.Printf("Examples:\n")
	fmt.Printf("  # Start server with configuration\n")
	fmt.Printf("  %s -config configs/microsoft365.json -port 8888\n\n", os.Args[0])
	fmt.Printf("  # Token management examples\n")
	fmt.Printf("  %s -token-add \"Production token\"\n", os.Args[0])
	fmt.Printf("  %s -token-add \"Production token\" -token-user <user-uuid>\n", os.Args[0])
	fmt.Printf("  %s -token-list\n", os.Args[0])
	fmt.Printf("  %s -token-del abc12345\n\n", os.Args[0])
	fmt.Printf("  # Create user with API token in one step\n")
	fmt.Printf("  %s -user-add \"Alice\" -user-token \"Alice laptop\"\n\n", os.Args[0])
	fmt.Printf("  # Generate auth code for fusion-auth\n")
	fmt.Printf("  %s -auth-code google -auth-url http://10.0.0.1:8888\n\n", os.Args[0])
}
