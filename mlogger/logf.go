/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package mlogger

import "fmt"

// Debugf logs a formatted debug message.
func (m *MLogger) Debugf(format string, v ...any) {
	if m.debug {
		m.writeLog("DEBUG", fmt.Sprintf(format, v...))
	}
}

// Infof logs a formatted informational message.
func (m *MLogger) Infof(format string, v ...any) {
	m.writeLog("INFO", fmt.Sprintf(format, v...))
}

// Noticef logs a formatted notice message.
func (m *MLogger) Noticef(format string, v ...any) {
	m.writeLog("NOTICE", fmt.Sprintf(format, v...))
}

// Warningf logs a formatted warning message.
func (m *MLogger) Warningf(format string, v ...any) {
	m.writeLog("WARNING", fmt.Sprintf(format, v...))
}

// Errorf logs a formatted error message.
func (m *MLogger) Errorf(format string, v ...any) {
	m.writeLog("ERROR", fmt.Sprintf(format, v...))
}

// Fatalf logs a formatted fatal message.
func (m *MLogger) Fatalf(format string, v ...any) {
	m.writeLog("FATAL", fmt.Sprintf(format, v...))
	m.FatalExit()
}
