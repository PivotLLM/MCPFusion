/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package mlogger

// Debug logs a debug message.
func (m *MLogger) Debug(message string) {
	if m.debug {
		m.writeLog("DEBUG", message)
	}
}

// Info logs an informational message.
func (m *MLogger) Info(message string) {
	m.writeLog("INFO", message)
}

// Notice logs a notice message.
func (m *MLogger) Notice(message string) {
	m.writeLog("NOTICE", message)
}

// Warning logs a warning message.
func (m *MLogger) Warning(message string) {
	m.writeLog("WARNING", message)
}

// Error logs an error message.
func (m *MLogger) Error(message string) {
	m.writeLog("ERROR", message)
}

// Fatal logs a fatal error message.
func (m *MLogger) Fatal(message string) {
	m.writeLog("FATAL", message)
	m.FatalExit()
}
