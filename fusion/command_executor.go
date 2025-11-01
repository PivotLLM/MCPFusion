/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
)

// CommandExecutor handles low-level command execution
type CommandExecutor struct {
	logger global.Logger
}

// NewCommandExecutor creates a new command executor
func NewCommandExecutor(logger global.Logger) *CommandExecutor {
	return &CommandExecutor{
		logger: logger,
	}
}

// ExecutionConfig holds all execution parameters
type ExecutionConfig struct {
	Executable       string
	Args             []string
	Env              []string
	Cwd              string
	Stdin            string
	Timeout          int
	KillGracePeriod  int
	CaptureStdout    bool
	CaptureStderr    bool
	UseShell         bool
	ShellInterpreter string
}

// ExecutionResult holds the command execution result
type ExecutionResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
	TimedOut bool
	Error    error
}

// Execute runs a command with the given configuration
func (e *CommandExecutor) Execute(ctx context.Context, config ExecutionConfig) ExecutionResult {
	startTime := time.Now()
	result := ExecutionResult{}

	// Create independent context with timeout for command execution
	// Use Background() to decouple from HTTP request context timeout
	// This allows commands to run for their full configured timeout even if
	// the HTTP client has a shorter timeout
	var cmdCtx context.Context
	var cancel context.CancelFunc
	if config.Timeout > 0 {
		cmdCtx, cancel = context.WithTimeout(context.Background(), time.Duration(config.Timeout)*time.Second)
		defer cancel()
	} else {
		cmdCtx = context.Background()
	}

	// Monitor parent context cancellation in background (for server shutdown)
	// This ensures we respect graceful shutdown even though command uses independent context
	parentDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			// Parent context cancelled (server shutdown or client disconnect)
			// Cancel command context to stop execution
			if cancel != nil {
				cancel()
			}
		case <-parentDone:
			// Command completed normally
			return
		}
	}()
	defer close(parentDone)

	// Build command
	var cmd *exec.Cmd
	if config.UseShell {
		// Use shell execution
		shellInterpreter := config.ShellInterpreter
		if shellInterpreter == "" {
			shellInterpreter = "/bin/sh"
		}

		// For shell execution, combine executable and args into a single command string
		var fullCommand string
		if len(config.Args) > 0 {
			// If args provided, combine them
			fullCommand = config.Executable + " " + strings.Join(config.Args, " ")
		} else {
			// Otherwise just the executable (which might be a full command string)
			fullCommand = config.Executable
		}

		cmd = exec.CommandContext(cmdCtx, shellInterpreter, "-c", fullCommand)
	} else {
		// Direct execution
		cmd = exec.CommandContext(cmdCtx, config.Executable, config.Args...)
	}

	// Set working directory if specified
	if config.Cwd != "" {
		cmd.Dir = config.Cwd
	}

	// Set environment variables
	if len(config.Env) > 0 {
		cmd.Env = config.Env
	}

	// Set up stdin if provided
	if config.Stdin != "" {
		cmd.Stdin = strings.NewReader(config.Stdin)
	}

	// Set up stdout/stderr capture
	var stdoutBuf, stderrBuf bytes.Buffer
	if config.CaptureStdout {
		cmd.Stdout = &stdoutBuf
	}
	if config.CaptureStderr {
		cmd.Stderr = &stderrBuf
	}

	// Execute command
	err := cmd.Run()
	result.Duration = time.Since(startTime)

	// Capture output
	if config.CaptureStdout {
		result.Stdout = stdoutBuf.String()
	}
	if config.CaptureStderr {
		result.Stderr = stderrBuf.String()
	}

	// Check for timeout
	if cmdCtx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.Error = fmt.Errorf("command timed out after %d seconds", config.Timeout)
		result.ExitCode = -1
		return result
	}

	// Get exit code
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Error = err
			result.ExitCode = -1
		}
	} else {
		result.ExitCode = 0
	}

	return result
}

// FormatResponse formats the execution result as text
func (e *CommandExecutor) FormatResponse(result ExecutionResult) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("Exit Code: %d\n", result.ExitCode))
	sb.WriteString(fmt.Sprintf("Execution Time: %.3fs\n", result.Duration.Seconds()))

	// Status
	if result.TimedOut {
		sb.WriteString("Status: Timed Out\n")
	} else if result.ExitCode == 0 {
		sb.WriteString("Status: Success\n")
	} else {
		sb.WriteString("Status: Failed\n")
	}

	// Error if present
	if result.Error != nil && !result.TimedOut {
		sb.WriteString(fmt.Sprintf("Error: %v\n", result.Error))
	}

	// Always show stdout section
	sb.WriteString("\n--- stdout ---\n")
	if result.Stdout != "" {
		sb.WriteString(result.Stdout)
		if !strings.HasSuffix(result.Stdout, "\n") {
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("(none)\n")
	}

	// Always show stderr section
	sb.WriteString("\n--- stderr ---\n")
	if result.Stderr != "" {
		sb.WriteString(result.Stderr)
		if !strings.HasSuffix(result.Stderr, "\n") {
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("(none)\n")
	}

	return sb.String()
}
