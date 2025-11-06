/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"strings"
	"testing"

	"github.com/PivotLLM/MCPFusion/global"
)

// testLogger is a simple logger for tests
type testLogger struct {
	t *testing.T
}

func (l *testLogger) Debug(msg string)                         { l.t.Log("DEBUG:", msg) }
func (l *testLogger) Debugf(format string, args ...interface{})  { l.t.Logf("DEBUG: "+format, args...) }
func (l *testLogger) Info(msg string)                          { l.t.Log("INFO:", msg) }
func (l *testLogger) Infof(format string, args ...interface{})   { l.t.Logf("INFO: "+format, args...) }
func (l *testLogger) Notice(msg string)                        { l.t.Log("NOTICE:", msg) }
func (l *testLogger) Noticef(format string, args ...interface{}) { l.t.Logf("NOTICE: "+format, args...) }
func (l *testLogger) Warning(msg string)                       { l.t.Log("WARN:", msg) }
func (l *testLogger) Warningf(format string, args ...interface{}) { l.t.Logf("WARN: "+format, args...) }
func (l *testLogger) Error(msg string)                         { l.t.Log("ERROR:", msg) }
func (l *testLogger) Errorf(format string, args ...interface{})   { l.t.Logf("ERROR: "+format, args...) }
func (l *testLogger) Fatal(msg string)                         { l.t.Fatal("FATAL:", msg) }
func (l *testLogger) Fatalf(format string, args ...interface{})   { l.t.Fatalf("FATAL: "+format, args...) }
func (l *testLogger) Close()                                   {}

func TestKaliConfig_CommandExec(t *testing.T) {
	// Create a test logger
	logger := &testLogger{t: t}

	// Load kali.json config
	fusion := New(
		WithLogger(logger),
		WithJSONConfig("../configs/kali.json"),
	)

	if fusion.config == nil {
		t.Fatal("Failed to load kali.json config")
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

	// Test execution
	result, err := execTool.Handler(map[string]interface{}{
		"command": "echo 'test output'",
	})

	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if !strings.Contains(result, "Exit Code: 0") {
		t.Errorf("Expected exit code 0 in result: %s", result)
	}

	if !strings.Contains(result, "test output") {
		t.Errorf("Expected 'test output' in result: %s", result)
	}
}

func TestKaliConfig_Nmap(t *testing.T) {
	// Skip if nmap not installed
	executor := NewCommandExecutor(nil)
	checkResult := executor.Execute(context.Background(), ExecutionConfig{
		Executable:    "/usr/bin/which",
		Args:          []string{"nmap"},
		CaptureStdout: true,
		Timeout:       5,
	})

	if checkResult.ExitCode != 0 {
		t.Skip("nmap not installed, skipping test")
	}

	// Load config and test
	logger := &testLogger{t: t}
	fusion := New(
		WithLogger(logger),
		WithJSONConfig("../configs/kali.json"),
	)

	tools := fusion.RegisterTools()

	var nmapTool *global.ToolDefinition
	for i := range tools {
		if tools[i].Name == "command_nmap" {
			nmapTool = &tools[i]
			break
		}
	}

	if nmapTool == nil {
		t.Fatal("command_nmap tool not found")
	}

	// Test with basic scan
	result, err := nmapTool.Handler(map[string]interface{}{
		"target": "127.0.0.1",
		"ports":  "22",
	})

	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	t.Logf("Nmap result: %s", result)

	if !strings.Contains(result, "Exit Code:") {
		t.Errorf("Expected exit code in result: %s", result)
	}
}
