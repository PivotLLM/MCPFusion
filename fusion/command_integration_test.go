/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"strings"
	"testing"

	"github.com/PivotLLM/MCPFusion/global"
	"github.com/tenebris-tech/mlogger/testlogger"
)

// newTestLogger returns a Logger backed by t for use across fusion package tests.
func newTestLogger(t *testing.T) *testlogger.Logger {
	t.Helper()
	return testlogger.New(t)
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
	logger := newTestLogger(t)
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
