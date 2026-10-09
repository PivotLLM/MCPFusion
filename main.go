/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/tenebris-tech/mlogger"

	"github.com/PivotLLM/MCPFusion/app"
	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/PivotLLM/MCPFusion/internal/admin"
	"github.com/PivotLLM/MCPFusion/internal/env"
)

func main() {
	opts := parseFlags()

	// Show help and exit if requested
	if opts.help {
		flag.Usage()
		return
	}

	// Show version and exit if requested
	if opts.version {
		printVersion()
		return
	}

	// Load the environment file first: it can set the log file location.
	loadedEnvFile := env.Load()

	// Create the logger
	logger, err := mlogger.New(
		mlogger.WithPrefix(app.Name()),
		mlogger.WithDateFormat("2006-01-02 15:04:05"),
		mlogger.WithLogFile(env.LogFile()),
		mlogger.WithLogStdout(true),
		mlogger.WithDebug(opts.debug),
	)
	if err != nil {
		fmt.Printf("Unable to create logger: %v", err)
		os.Exit(1)
	}

	// Fatalf closes the log and exits with status 1.
	if err := run(opts, logger, loadedEnvFile); err != nil {
		var ge *admin.GuidanceError
		if errors.As(err, &ge) {
			logger.Fatalf("%v. %s", err, ge.Guidance)
		}
		logger.Fatalf("%v", err)
	}
}

// run executes the requested administration command, or runs the server until
// it is signalled to stop. The database is closed before it returns.
func run(opts options, logger global.Logger, loadedEnvFile string) error {
	knowledgeEnabled := env.KnowledgeEnabled()
	perfEnabled := env.PerfEnabled(opts.perf)

	// Log startup banner
	logger.Infof("%s %s", app.Name(), app.Version())
	logger.Info(app.Copyright())

	// Log knowledge and perf activation state
	if knowledgeEnabled {
		logger.Info("Knowledge provider: enabled")
	} else {
		logger.Info("Knowledge provider: disabled (MCP_FUSION_KNOWLEDGE=false)")
	}
	if perfEnabled {
		logger.Warning("Perf provider: enabled (MCP_FUSION_PERF — DO NOT USE IN PRODUCTION)")
	} else {
		logger.Info("Perf provider: disabled")
	}

	// Log warning if no-auth mode is enabled
	if opts.noAuth {
		logger.Warning("**************************************************************")
		logger.Warning("* SECURITY WARNING: Authentication is DISABLED              *")
		logger.Warning("* This mode is INSECURE and should ONLY be used for testing *")
		logger.Warning("* All requests will use the 'NOAUTH' tenant context         *")
		logger.Warning("**************************************************************")
	}

	// Log environment file loading status
	if loadedEnvFile != "" {
		logger.Infof("Loaded environment from: %s", loadedEnvFile)
	} else {
		logger.Debug("No environment file loaded (searched: /opt/mcpfusion/env, ~/.mcpfusion)")
	}

	// Now that env files are loaded, check for fusion configs
	configFiles := env.ConfigFiles(opts.config, logger)

	// Determine listen address from environment or flag
	listen, fromEnv := env.Listen(opts.port)
	if fromEnv {
		logger.Infof("Using listen address from MCP_FUSION_LISTEN: %s", listen)
	}

	// Initialize database
	logger.Info("Initializing database")

	// Database configuration
	dbOpts := []db.Option{
		db.WithLogger(logger),
	}
	if dbDataDir := os.Getenv("MCP_FUSION_DB_DIR"); dbDataDir != "" {
		dbOpts = append(dbOpts, db.WithDataDir(dbDataDir))
	}

	// Initialize database (required)
	database, err := db.New(dbOpts...)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	// Close is idempotent: the server closes the database and reports it during
	// shutdown, so this only acts on the command and error paths.
	defer func() {
		if err := database.Close(); err != nil {
			logger.Errorf("Error closing database: %v", err)
		}
	}()
	logger.Info("Database initialized successfully")

	// Administration commands run instead of the server.
	if opts.token.Requested() {
		if err := opts.token.Run(database); err != nil {
			return fmt.Errorf("token management failed: %w", err)
		}
		return nil
	}

	if opts.user.Requested() {
		if err := opts.user.Run(database); err != nil {
			return fmt.Errorf("user management failed: %w", err)
		}
		return nil
	}

	if opts.authCode.Requested() {
		if err := opts.authCode.Run(database, logger); err != nil {
			return fmt.Errorf("auth code generation failed: %w", err)
		}
		return nil
	}

	srv := server{
		logger:           logger,
		database:         database,
		listen:           listen,
		configFiles:      configFiles,
		debug:            opts.debug,
		noAuth:           opts.noAuth,
		knowledgeEnabled: knowledgeEnabled,
		perfEnabled:      perfEnabled,
	}

	// The server runs until SIGINT or SIGTERM. The signal context is created here,
	// after the administration commands, so that those can still be interrupted.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return srv.run(ctx)
}
