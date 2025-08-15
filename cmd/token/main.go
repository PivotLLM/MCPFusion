/*=============================================================================
= Copyright (c) 2025 Tenebris Technologies Inc.                              =
= All rights reserved.                                                       =
=============================================================================*/

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/mlogger"
)

const (
	ExitSuccess = 0
	ExitError   = 1
)

// CLI colors for better UX
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
	ColorGray   = "\033[37m"
)

type Config struct {
	DataDir string
	Debug   bool
	NoColor bool
}

func main() {
	config := parseFlags()
	
	// Initialize logger
	logger, err := createLogger(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to initialize logger: %v\n", err)
		os.Exit(ExitError)
	}
	defer logger.Close()

	// Initialize database
	database, err := initializeDatabase(config, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to initialize database: %v\n", err)
		os.Exit(ExitError)
	}
	defer database.Close()

	// Parse command and execute
	if err := executeCommand(config, database, logger); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(ExitError)
	}
}

func parseFlags() *Config {
	config := &Config{}
	
	flag.StringVar(&config.DataDir, "data-dir", "", "Data directory path (default: /opt/mcpfusion or ~/.mcpfusion)")
	flag.BoolVar(&config.Debug, "debug", false, "Enable debug logging")
	flag.BoolVar(&config.NoColor, "no-color", false, "Disable colored output")
	
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "MCPFusion Token Management CLI\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s [flags] <command> [arguments]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  add [description]        Generate and add new API token\n")
		fmt.Fprintf(os.Stderr, "  list                     List all API tokens\n")
		fmt.Fprintf(os.Stderr, "  delete <hash|prefix>     Delete API token by hash or prefix\n")
		fmt.Fprintf(os.Stderr, "  help                     Show this help message\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s add \"Development token\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s list\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s delete abc12345\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s delete a1b2c3d4e5f6...\n", os.Args[0])
	}
	
	flag.Parse()
	return config
}

func createLogger(config *Config) (*mlogger.MLogger, error) {
	loggerOpts := []mlogger.Option{
		mlogger.WithPrefix("token-cli"),
		mlogger.WithLogStdout(false), // CLI output should not be mixed with logs
		mlogger.WithDebug(config.Debug),
	}
	
	// Only log to file if in debug mode
	if config.Debug {
		logFile := filepath.Join(os.TempDir(), "mcpfusion-token-cli.log")
		loggerOpts = append(loggerOpts, 
			mlogger.WithLogFile(logFile),
			mlogger.WithLogStdout(true))
	}
	
	logger, err := mlogger.New(loggerOpts...)
	if err != nil {
		return nil, err
	}
	
	return logger.(*mlogger.MLogger), nil
}

func initializeDatabase(config *Config, logger *mlogger.MLogger) (db.Database, error) {
	dbOpts := []db.Option{
		db.WithLogger(logger),
	}
	
	if config.DataDir != "" {
		dbOpts = append(dbOpts, db.WithDataDir(config.DataDir))
	}
	
	return db.New(dbOpts...)
}

func executeCommand(config *Config, database db.Database, logger *mlogger.MLogger) error {
	args := flag.Args()
	if len(args) == 0 {
		return fmt.Errorf("no command specified. Use 'help' for usage information")
	}
	
	command := args[0]
	
	switch command {
	case "add":
		return handleAddCommand(config, database, args[1:])
	case "list":
		return handleListCommand(config, database, args[1:])
	case "delete":
		return handleDeleteCommand(config, database, args[1:])
	case "help", "-h", "--help":
		flag.Usage()
		return nil
	default:
		return fmt.Errorf("unknown command '%s'. Use 'help' for usage information", command)
	}
}

func handleAddCommand(config *Config, database db.Database, args []string) error {
	var description string
	
	if len(args) > 0 {
		description = strings.Join(args, " ")
	} else {
		description = "API Token"
	}
	
	// Validate description length
	if len(description) > 255 {
		return fmt.Errorf("description too long (max 255 characters)")
	}
	
	fmt.Printf("Generating new API token...\n")
	
	token, hash, err := database.AddAPIToken(description)
	if err != nil {
		return fmt.Errorf("failed to create API token: %w", err)
	}
	
	// Show the token only once with security warning
	fmt.Printf("\n")
	fmt.Print(colorize(config, ColorGreen, "✓ API Token created successfully\n"))
	fmt.Printf("\n")
	fmt.Print(colorize(config, ColorYellow, "⚠ SECURITY WARNING: This token will only be displayed once!\n"))
	fmt.Print(colorize(config, ColorYellow, "   Copy it now and store it securely.\n"))
	fmt.Printf("\n")
	fmt.Printf("Token:       %s\n", colorize(config, ColorCyan, token))
	fmt.Printf("Hash:        %s\n", hash[:12]+"...")
	fmt.Printf("Description: %s\n", description)
	fmt.Printf("\n")
	fmt.Printf("Use this token in the Authorization header:\n")
	fmt.Printf("  Authorization: Bearer %s\n", token)
	fmt.Printf("\n")
	
	return nil
}

func handleListCommand(config *Config, database db.Database, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("list command does not accept arguments")
	}
	
	tokens, err := database.ListAPITokens()
	if err != nil {
		return fmt.Errorf("failed to list API tokens: %w", err)
	}
	
	if len(tokens) == 0 {
		fmt.Printf("No API tokens found.\n")
		fmt.Printf("Use '%s add \"description\"' to create one.\n", os.Args[0])
		return nil
	}
	
	// Sort tokens by creation date (newest first)
	sort.Slice(tokens, func(i, j int) bool {
		return tokens[i].CreatedAt.After(tokens[j].CreatedAt)
	})
	
	fmt.Printf("API Tokens (%d total):\n\n", len(tokens))
	
	// Create tabwriter for formatted output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	
	// Header
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
		colorize(config, ColorBlue, "PREFIX"),
		colorize(config, ColorBlue, "HASH"),
		colorize(config, ColorBlue, "CREATED"),
		colorize(config, ColorBlue, "LAST USED"),
		colorize(config, ColorBlue, "DESCRIPTION"))
	
	// Separator
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
		strings.Repeat("-", 8),
		strings.Repeat("-", 12),
		strings.Repeat("-", 19),
		strings.Repeat("-", 19),
		strings.Repeat("-", 20))
	
	// Data rows
	for _, token := range tokens {
		createdAt := token.CreatedAt.Format("2006-01-02 15:04:05")
		lastUsed := token.LastUsed.Format("2006-01-02 15:04:05")
		hash := token.Hash[:12] + "..."
		
		// Highlight unused tokens
		prefix := token.Prefix
		if token.CreatedAt.Equal(token.LastUsed) {
			prefix = colorize(config, ColorYellow, token.Prefix)
		}
		
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			prefix,
			hash,
			createdAt,
			lastUsed,
			token.Description)
	}
	
	w.Flush()
	
	fmt.Printf("\n")
	fmt.Printf("Use '%s delete <prefix|hash>' to delete a token.\n", os.Args[0])
	
	return nil
}

func handleDeleteCommand(config *Config, database db.Database, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("delete command requires exactly one argument (hash or prefix)")
	}
	
	identifier := args[0]
	
	// Resolve the identifier to a full hash
	hash, err := database.ResolveAPIToken(identifier)
	if err != nil {
		if strings.Contains(err.Error(), "ambiguous identifier") {
			return fmt.Errorf("ambiguous identifier '%s'. Please provide more characters to uniquely identify the token", identifier)
		}
		if strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("no API token found matching '%s'", identifier)
		}
		return fmt.Errorf("failed to resolve token identifier: %w", err)
	}
	
	// Get token metadata for confirmation
	metadata, err := database.GetAPITokenMetadata(hash)
	if err != nil {
		return fmt.Errorf("failed to get token metadata: %w", err)
	}
	
	// Show confirmation details
	fmt.Printf("Token to delete:\n")
	fmt.Printf("  Prefix:      %s\n", metadata.Prefix)
	fmt.Printf("  Hash:        %s\n", hash[:12]+"...")
	fmt.Printf("  Description: %s\n", metadata.Description)
	fmt.Printf("  Created:     %s\n", metadata.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("\n")
	
	// Ask for confirmation
	fmt.Print(colorize(config, ColorYellow, "Are you sure you want to delete this token? [y/N]: "))
	var response string
	fmt.Scanln(&response)
	
	response = strings.ToLower(strings.TrimSpace(response))
	if response != "y" && response != "yes" {
		fmt.Printf("Operation cancelled.\n")
		return nil
	}
	
	// Delete the token
	if err := database.DeleteAPIToken(hash); err != nil {
		return fmt.Errorf("failed to delete API token: %w", err)
	}
	
	fmt.Print(colorize(config, ColorGreen, "✓ API token deleted successfully\n"))
	
	return nil
}

func colorize(config *Config, color, text string) string {
	if config.NoColor {
		return text
	}
	return color + text + ColorReset
}