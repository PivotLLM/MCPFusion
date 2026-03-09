/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

// Package mlogger provides a simple file-based logger with optional debug message
// suppression and logging to stdout.
package mlogger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type MLogger struct {
	fileHandle *os.File
	logfile    string
	logStdout  bool
	debug      bool
	logLevel   bool
	prefix     string
	dateFormat string
}

// This package implements Logger
var _ Logger = (*MLogger)(nil)

// Option is a function that configures a MLogger
type Option func(*MLogger) error

// New creates a new instance of MLogger with the provided options
func New(options ...Option) (Logger, error) {
	m := &MLogger{
		logLevel:   true,
		dateFormat: "2006-01-02 15:04:05",
	}

	for _, option := range options {
		if err := option(m); err != nil {
			return nil, err
		}
	}

	// Call the OS-specific constructor
	return m.open()
}

// WithPrefix sets a process name or similar short identifier
//
//goland:noinspection GoUnusedExportedFunction
func WithPrefix(prefix string) Option {
	return func(u *MLogger) error {
		if prefix == "" {
			u.prefix = ""
		} else {
			u.prefix = " " + strings.TrimSpace(prefix)
		}
		return nil
	}
}

// WithDateFormat sets the date format for the MLogger
//
//goland:noinspection GoUnusedExportedFunction
func WithDateFormat(dateFormat string) Option {
	return func(u *MLogger) error {
		u.dateFormat = dateFormat
		return nil
	}
}

// WithLogFile sets the log file for the MLogger
//
//goland:noinspection GoUnusedExportedFunction
func WithLogFile(logfile string) Option {
	return func(u *MLogger) error {
		u.logfile = logfile
		return nil
	}
}

// WithLogStdout enables or disables logging to stdout
//
//goland:noinspection GoUnusedExportedFunction
func WithLogStdout(logStdout bool) Option {
	return func(u *MLogger) error {
		u.logStdout = logStdout
		return nil
	}
}

// WithLevel enables or disables logging the level
//
//goland:noinspection GoUnusedExportedFunction
func WithLevel(logLevel bool) Option {
	return func(u *MLogger) error {
		u.logLevel = logLevel
		return nil
	}
}

// WithDebug enables or disables debug logging
//
//goland:noinspection GoUnusedExportedFunction
func WithDebug(debug bool) Option {
	return func(u *MLogger) error {
		u.debug = debug
		return nil
	}
}

// open sets up the logger. This function is not exported, it is called by New
func (m *MLogger) open() (*MLogger, error) {
	var err error
	var fh *os.File

	if m.logfile != "" {

		// Sanitize the file path
		m.logfile = filepath.Clean(m.logfile)

		// Create the directory if it doesn't exist
		dir := filepath.Dir(m.logfile)
		if err = os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		// Open the log file
		fh, err = os.OpenFile(m.logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		m.fileHandle = fh
	} else {
		// If no log file is specified, force stdout logging
		m.logStdout = true
	}
	return m, nil
}

// Close closes the logger
func (m *MLogger) Close() {
	if m.fileHandle != nil {
		_ = m.fileHandle.Sync()
		_ = m.fileHandle.Close()
	}
}

// formatMessage formats the log message with a timestamp.
func (m *MLogger) formatMessage(level string, message string) string {
	var levelStr string
	if m.logLevel {
		levelStr = " [" + level + "]"
	} else {
		levelStr = ""
	}
	return fmt.Sprintf("%s%s%s %s",
		time.Now().Format(m.dateFormat),
		m.prefix, levelStr, message)
}

// writeLog writes a log message
func (m *MLogger) writeLog(level string, message string) {

	tmp := m.formatMessage(level, message) + "\n"

	//  Write and flush
	if m.fileHandle != nil {
		_, _ = m.fileHandle.WriteString(tmp)
		_ = m.fileHandle.Sync()
	}

	if m.logStdout {
		_, _ = os.Stdout.Write([]byte(tmp))
	}
}

// FatalExit closes the log and exits with a status code of 1
func (m *MLogger) FatalExit() {
	m.Close()
	os.Exit(1)
}
