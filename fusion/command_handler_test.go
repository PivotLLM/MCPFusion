/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"strings"
	"testing"
)

func TestCommandExecutor_SimpleCommand(t *testing.T) {
	executor := NewCommandExecutor(nil)

	config := ExecutionConfig{
		Executable:    "/bin/echo",
		Args:          []string{"hello", "world"},
		CaptureStdout: true,
		CaptureStderr: true,
		Timeout:       10,
	}

	result := executor.Execute(context.Background(), config)

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	if !strings.Contains(result.Stdout, "hello world") {
		t.Errorf("Expected stdout to contain 'hello world', got: %s", result.Stdout)
	}
}

func TestCommandExecutor_WithPrefix(t *testing.T) {
	// Test that prefix handling works correctly
	handler := &CommandHandler{}

	config := &ExecutionConfig{}
	param := &ParameterConfig{
		Name:     "ports",
		Prefix:   "-p",
		Type:     ParameterTypeString,
		Location: ParameterLocationArgument,
	}

	handler.handleArgumentParameter(config, param, "80,443")

	if len(config.Args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(config.Args))
	}

	if config.Args[0] != "-p" || config.Args[1] != "80,443" {
		t.Errorf("Expected ['-p', '80,443'], got %v", config.Args)
	}
}

func TestCommandExecutor_BooleanFlag(t *testing.T) {
	handler := &CommandHandler{}

	config := &ExecutionConfig{}
	param := &ParameterConfig{
		Name:     "--verbose",
		Type:     ParameterTypeBoolean,
		Location: ParameterLocationArgument,
	}

	// Test true
	handler.handleArgumentParameter(config, param, true)
	if len(config.Args) != 1 || config.Args[0] != "--verbose" {
		t.Errorf("Expected ['--verbose'], got %v", config.Args)
	}

	// Test false
	config.Args = []string{}
	handler.handleArgumentParameter(config, param, false)
	if len(config.Args) != 0 {
		t.Errorf("Expected no args for false boolean, got %v", config.Args)
	}
}

func TestCommandExecutor_Environment(t *testing.T) {
	executor := NewCommandExecutor(nil)

	config := ExecutionConfig{
		Executable:    "/bin/sh",
		Args:          []string{"-c", "echo $TEST_VAR"},
		Env:           []string{"TEST_VAR=hello"},
		CaptureStdout: true,
		Timeout:       10,
	}

	result := executor.Execute(context.Background(), config)

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("Expected stdout to contain 'hello', got: %s", result.Stdout)
	}
}

func TestCommandExecutor_Timeout(t *testing.T) {
	executor := NewCommandExecutor(nil)

	config := ExecutionConfig{
		Executable:    "/bin/sleep",
		Args:          []string{"10"},
		Timeout:       1, // 1 second timeout
		CaptureStdout: true,
		CaptureStderr: true,
	}

	result := executor.Execute(context.Background(), config)

	if !result.TimedOut {
		t.Error("Expected command to timeout")
	}

	if result.ExitCode != -1 {
		t.Errorf("Expected exit code -1 for timeout, got %d", result.ExitCode)
	}
}
