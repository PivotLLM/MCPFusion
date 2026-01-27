/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"fmt"
	"testing"

	"github.com/PivotLLM/MCPFusion/mlogger"
)

// demonstrateTimeTokens demonstrates how the time token substitution system works
func demonstrateTimeTokens() {
	logger, _ := mlogger.New(mlogger.WithLogStdout(true), mlogger.WithDebug(false))
	processor := NewTimeTokenProcessor(logger)

	// Demonstrate different token types
	examples := []string{
		"#DAYS-0",                            // Today at midnight
		"#DAYS-7",                            // 7 days ago at midnight
		"#HOURS-0",                           // Current time
		"#HOURS-24",                          // 24 hours ago
		"startDate=#DAYS-30&endDate=#DAYS-0", // Mixed usage in query string
		"createdDateTime ge #DAYS-7",         // OData filter example
	}

	fmt.Println("Time Token Substitution Examples:")
	fmt.Println("=================================")

	for _, example := range examples {
		result := processor.ProcessValue(example)
		fmt.Printf("Input:  %s\n", example)
		fmt.Printf("Output: %s\n\n", result)
	}

	// Show supported tokens
	tokens := processor.GetSupportedTokens()
	fmt.Println("Supported Token Patterns:")
	for pattern, description := range tokens {
		fmt.Printf("  %s: %s\n", pattern, description)
	}
}

// This test function allows the example to run during testing
func TestExampleTimeTokens(t *testing.T) {
	demonstrateTimeTokens()
}
