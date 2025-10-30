/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CommandHandler handles command execution for a specific command configuration
type CommandHandler struct {
	commandGroup *CommandGroupConfig
	command      *CommandConfig
	fusion       *Fusion
	executor     *CommandExecutor
}

// NewCommandHandler creates a new command handler
func NewCommandHandler(fusion *Fusion, commandGroup *CommandGroupConfig, command *CommandConfig) *CommandHandler {
	return &CommandHandler{
		commandGroup: commandGroup,
		command:      command,
		fusion:       fusion,
		executor:     NewCommandExecutor(fusion.logger),
	}
}

// Handle executes the command with provided arguments
func (h *CommandHandler) Handle(ctx context.Context, args map[string]interface{}) (string, error) {
	// Extract control parameters
	execConfig := h.buildExecutionConfig(args)

	// Log execution
	if h.fusion.logger != nil {
		h.fusion.logger.Infof("Executing command %s_%s: %s %v",
			h.commandGroup.Name, h.command.ID, execConfig.Executable, execConfig.Args)
	}

	// Execute command
	result := h.executor.Execute(ctx, execConfig)

	// Format and return response
	return h.executor.FormatResponse(result), nil
}

// buildExecutionConfig constructs execution configuration from parameters
func (h *CommandHandler) buildExecutionConfig(args map[string]interface{}) ExecutionConfig {
	config := ExecutionConfig{
		Timeout:          180, // Default timeout
		KillGracePeriod:  5,
		CaptureStdout:    true,
		CaptureStderr:    true,
		UseShell:         false,
		ShellInterpreter: "/bin/sh",
	}

	// Process parameters in order
	for _, param := range h.command.Parameters {
		// Get value (use default if not provided and not required)
		value, exists := args[param.Name]
		if !exists {
			if param.Default != nil {
				value = param.Default
			} else if param.Static && param.Default != nil {
				value = param.Default
			} else {
				continue
			}
		}

		// Handle based on location
		switch param.Location {
		case ParameterLocationControl:
			h.handleControlParameter(&config, param.Name, value)

		case ParameterLocationArgument:
			h.handleArgumentParameter(&config, &param, value)

		case ParameterLocationArglist:
			h.handleArglistParameter(&config, value)

		case ParameterLocationEnvironment:
			h.handleEnvironmentParameter(&config, param.Name, value)

		case ParameterLocationStdin:
			h.handleStdinParameter(&config, value)
		}
	}

	// If no environment set, use current process environment
	if len(config.Env) == 0 {
		config.Env = os.Environ()
	}

	return config
}

// handleControlParameter handles execution control parameters
func (h *CommandHandler) handleControlParameter(config *ExecutionConfig, name string, value interface{}) {
	switch name {
	case "executable":
		if str, ok := value.(string); ok {
			config.Executable = str
		}
	case "timeout":
		if num, ok := value.(float64); ok {
			config.Timeout = int(num)
		} else if num, ok := value.(int); ok {
			config.Timeout = num
		}
	case "kill_grace_period":
		if num, ok := value.(float64); ok {
			config.KillGracePeriod = int(num)
		} else if num, ok := value.(int); ok {
			config.KillGracePeriod = num
		}
	case "cwd":
		if str, ok := value.(string); ok {
			config.Cwd = str
		}
	case "use_shell":
		if b, ok := value.(bool); ok {
			config.UseShell = b
		}
	case "shell_interpreter":
		if str, ok := value.(string); ok {
			config.ShellInterpreter = str
		}
	case "capture_stdout":
		if b, ok := value.(bool); ok {
			config.CaptureStdout = b
		}
	case "capture_stderr":
		if b, ok := value.(bool); ok {
			config.CaptureStderr = b
		}
	}
}

// handleArgumentParameter handles command-line argument parameters
func (h *CommandHandler) handleArgumentParameter(config *ExecutionConfig, param *ParameterConfig, value interface{}) {
	// Skip if value is nil/empty and parameter is optional
	if value == nil {
		return
	}

	// Handle boolean flags
	if param.Type == ParameterTypeBoolean {
		if b, ok := value.(bool); ok && b {
			// Add flag (parameter name IS the flag)
			config.Args = append(config.Args, param.Name)
		}
		return
	}

	// Convert value to string
	strValue := h.valueToString(value)
	if strValue == "" {
		return
	}

	// Handle prefix
	if param.Prefix != "" {
		// Add prefix and value as separate arguments
		config.Args = append(config.Args, param.Prefix, strValue)
	} else {
		// Add value only
		config.Args = append(config.Args, strValue)
	}
}

// handleArglistParameter handles array of arguments
func (h *CommandHandler) handleArglistParameter(config *ExecutionConfig, value interface{}) {
	if arr, ok := value.([]interface{}); ok {
		for _, item := range arr {
			if str := h.valueToString(item); str != "" {
				config.Args = append(config.Args, str)
			}
		}
	}
}

// handleEnvironmentParameter handles environment variables
func (h *CommandHandler) handleEnvironmentParameter(config *ExecutionConfig, name string, value interface{}) {
	strValue := h.valueToString(value)
	if strValue == "" {
		return
	}

	// Initialize env if needed
	if len(config.Env) == 0 {
		config.Env = os.Environ()
	}

	// Add or update environment variable
	envVar := fmt.Sprintf("%s=%s", name, strValue)

	// Check if variable already exists and update it
	found := false
	for i, env := range config.Env {
		if strings.HasPrefix(env, name+"=") {
			config.Env[i] = envVar
			found = true
			break
		}
	}

	if !found {
		config.Env = append(config.Env, envVar)
	}
}

// handleStdinParameter handles stdin data
func (h *CommandHandler) handleStdinParameter(config *ExecutionConfig, value interface{}) {
	config.Stdin = h.valueToString(value)
}

// valueToString converts a value to string
func (h *CommandHandler) valueToString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case float64:
		// Check if it's an integer
		if v == float64(int(v)) {
			return strconv.Itoa(int(v))
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
