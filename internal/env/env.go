/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

// Package env loads the MCPFusion environment file and reads the settings the
// server takes from environment variables.
package env

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"

	"github.com/PivotLLM/MCPFusion/global"
)

// defaultListen is used when neither MCP_FUSION_LISTEN nor a valid port is given.
const defaultListen = "localhost:8888"

// Load loads the first environment file that exists and parses, in priority
// order: /opt/mcpfusion/env, then ~/.mcpfusion. It returns the path of the file
// loaded, or "" if none was. It runs before the logger exists, so it does not log.
func Load() string {
	envFiles := []string{
		"/opt/mcpfusion/env",
	}

	// Add user-specific config files if home directory is available
	homeDir, err := os.UserHomeDir()
	if err == nil {
		envFiles = append(envFiles, homeDir+string(os.PathSeparator)+".mcpfusion")
	}

	for _, envFile := range envFiles {
		if _, err := os.Stat(envFile); err != nil {
			continue
		}
		if err := godotenv.Load(envFile); err == nil {
			// Stop after loading the first successful file.
			return envFile
		}
	}
	return ""
}

// KnowledgeEnabled reports whether the knowledge provider is enabled. It is
// enabled unless MCP_FUSION_KNOWLEDGE is false, 0 or no. Call it after Load so
// values from the environment file are visible.
func KnowledgeEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("MCP_FUSION_KNOWLEDGE")))
	return v != "false" && v != "0" && v != "no"
}

// PerfEnabled reports whether the perf provider is enabled: either the -perf
// flag is set or MCP_FUSION_PERF is true, 1 or yes. Call it after Load so values
// from the environment file are visible.
func PerfEnabled(flag bool) bool {
	if flag {
		return true
	}
	v := strings.ToLower(strings.TrimSpace(os.Getenv("MCP_FUSION_PERF")))
	return v == "true" || v == "1" || v == "yes"
}

// LogFile returns the log file path: MCP_FUSION_LOGFILE if set (even to ""),
// otherwise mcpfusion.log in the current directory.
func LogFile() string {
	if value, exists := os.LookupEnv("MCP_FUSION_LOGFILE"); exists {
		return value
	}
	return "mcpfusion.log"
}

// Listen returns the listen address and whether it came from MCP_FUSION_LISTEN.
// Without that variable it listens on localhost at port, or on the default
// address if port is out of range.
func Listen(port int) (string, bool) {
	if envListen := os.Getenv("MCP_FUSION_LISTEN"); envListen != "" {
		return envListen, true
	}
	if port > 0 && port < 65536 {
		return fmt.Sprintf("localhost:%d", port), false
	}
	return defaultListen, false
}

// ConfigFiles returns the configuration files to load, from configFlag if set,
// otherwise from MCP_FUSION_CONFIGS, otherwise from MCP_FUSION_CONFIG. Each is a
// comma-separated list; entries are trimmed and empty ones dropped.
func ConfigFiles(configFlag string, logger global.Logger) []string {
	configPaths := configFlag

	// If not provided via command line, check environment variables
	if configPaths == "" {
		configPaths = os.Getenv("MCP_FUSION_CONFIGS")
		if configPaths != "" && logger != nil {
			logger.Infof("Using config files from MCP_FUSION_CONFIGS: %s", configPaths)
		}
	}

	// Fall back to the single config environment variable
	if configPaths == "" {
		configPaths = os.Getenv("MCP_FUSION_CONFIG")
		if configPaths != "" && logger != nil {
			logger.Infof("Using config file from MCP_FUSION_CONFIG: %s", configPaths)
		}
	}

	if configPaths == "" {
		return []string{}
	}

	files := strings.Split(configPaths, ",")
	cleanFiles := make([]string, 0, len(files))
	for _, file := range files {
		if trimmed := strings.TrimSpace(file); trimmed != "" {
			cleanFiles = append(cleanFiles, trimmed)
		}
	}

	if logger != nil && len(cleanFiles) > 0 {
		logger.Infof("Found %d configuration file(s) to load", len(cleanFiles))
	}

	return cleanFiles
}
