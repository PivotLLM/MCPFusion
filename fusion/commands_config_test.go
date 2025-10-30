/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"strings"
	"testing"

	"github.com/PivotLLM/MCPFusion/global"
)

func TestCommandsConfig_ShellExec(t *testing.T) {
	// Create a test logger
	logger := &testLogger{t: t}

	// Load commands.json config
	fusion := New(
		WithLogger(logger),
		WithJSONConfig("../configs/commands.json"),
	)

	if fusion.config == nil {
		t.Fatal("Failed to load commands.json config")
	}

	// Get registered tools
	tools := fusion.RegisterTools()

	// Find command_exec tool
	var execTool *global.ToolDefinition
	for i := range tools {
		if tools[i].Name == "command_exec" {
			execTool = &tools[i]
			break
		}
	}

	if execTool == nil {
		t.Fatal("command_exec tool not found")
	}

	// Test basic execution
	result, err := execTool.Handler(map[string]interface{}{
		"command": "echo 'test123'",
	})

	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if !strings.Contains(result, "Exit Code: 0") {
		t.Errorf("Expected exit code 0 in result: %s", result)
	}

	if !strings.Contains(result, "test123") {
		t.Errorf("Expected 'test123' in result: %s", result)
	}
}

func TestCommandsConfig_DirectExec(t *testing.T) {
	logger := &testLogger{t: t}

	fusion := New(
		WithLogger(logger),
		WithJSONConfig("../configs/commands.json"),
	)

	tools := fusion.RegisterTools()

	// Find command_exec_direct tool
	var execTool *global.ToolDefinition
	for i := range tools {
		if tools[i].Name == "command_exec_direct" {
			execTool = &tools[i]
			break
		}
	}

	if execTool == nil {
		t.Fatal("command_exec_direct tool not found")
	}

	// Test execution with arguments
	result, err := execTool.Handler(map[string]interface{}{
		"executable": "/bin/echo",
		"arguments":  []interface{}{"hello", "world"},
	})

	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if !strings.Contains(result, "Exit Code: 0") {
		t.Errorf("Expected exit code 0 in result: %s", result)
	}

	if !strings.Contains(result, "hello world") {
		t.Errorf("Expected 'hello world' in result: %s", result)
	}
}

func TestCommandsConfig_WithEnvironment(t *testing.T) {
	logger := &testLogger{t: t}

	fusion := New(
		WithLogger(logger),
		WithJSONConfig("../configs/commands.json"),
	)

	tools := fusion.RegisterTools()

	var execTool *global.ToolDefinition
	for i := range tools {
		if tools[i].Name == "command_exec" {
			execTool = &tools[i]
			break
		}
	}

	if execTool == nil {
		t.Fatal("command_exec tool not found")
	}

	// Test with environment variables
	result, err := execTool.Handler(map[string]interface{}{
		"command": "echo $TEST_VAR",
		"environment": map[string]interface{}{
			"TEST_VAR": "hello_env",
		},
	})

	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if !strings.Contains(result, "Exit Code: 0") {
		t.Errorf("Expected exit code 0 in result: %s", result)
	}

	if !strings.Contains(result, "hello_env") {
		t.Errorf("Expected 'hello_env' in result: %s", result)
	}
}

func TestCommandsConfig_DirectExecWithStdin(t *testing.T) {
	logger := &testLogger{t: t}

	fusion := New(
		WithLogger(logger),
		WithJSONConfig("../configs/commands.json"),
	)

	tools := fusion.RegisterTools()

	var execTool *global.ToolDefinition
	for i := range tools {
		if tools[i].Name == "command_exec_direct" {
			execTool = &tools[i]
			break
		}
	}

	if execTool == nil {
		t.Fatal("command_exec_direct tool not found")
	}

	// Test with stdin
	result, err := execTool.Handler(map[string]interface{}{
		"executable": "/bin/cat",
		"stdin":      "stdin test data",
	})

	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if !strings.Contains(result, "Exit Code: 0") {
		t.Errorf("Expected exit code 0 in result: %s", result)
	}

	if !strings.Contains(result, "stdin test data") {
		t.Errorf("Expected 'stdin test data' in result: %s", result)
	}
}
