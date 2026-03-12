/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package mlogger

import (
	"fmt"
	"strings"
)

// FormatFields converts alternating key-value pairs into a logfmt string.
// Keys should be strings. Values are formatted with %v.
// Values containing spaces, '=', or '"' are quoted.
// An odd number of args results in the last key being assigned the value "MISSING".
func FormatFields(args ...any) string {
	if len(args) == 0 {
		return ""
	}

	var sb strings.Builder
	for i := 0; i < len(args); i += 2 {
		if i > 0 {
			sb.WriteByte(' ')
		}

		// Key
		key := fmt.Sprintf("%v", args[i])
		sb.WriteString(key)
		sb.WriteByte('=')

		// Value
		if i+1 >= len(args) {
			sb.WriteString("MISSING")
			break
		}
		val := fmt.Sprintf("%v", args[i+1])
		if strings.ContainsAny(val, " \t=\"") {
			sb.WriteByte('"')
			sb.WriteString(strings.ReplaceAll(val, `"`, `\"`))
			sb.WriteByte('"')
		} else {
			sb.WriteString(val)
		}
	}
	return sb.String()
}

// DebugFields logs a structured debug message as logfmt key=value pairs.
func (m *MLogger) DebugFields(args ...any) {
	if m.debug {
		m.writeLog("DEBUG", FormatFields(args...))
	}
}

// InfoFields logs a structured informational message as logfmt key=value pairs.
func (m *MLogger) InfoFields(args ...any) {
	m.writeLog("INFO", FormatFields(args...))
}

// NoticeFields logs a structured notice message as logfmt key=value pairs.
func (m *MLogger) NoticeFields(args ...any) {
	m.writeLog("NOTICE", FormatFields(args...))
}

// WarningFields logs a structured warning message as logfmt key=value pairs.
func (m *MLogger) WarningFields(args ...any) {
	m.writeLog("WARNING", FormatFields(args...))
}

// ErrorFields logs a structured error message as logfmt key=value pairs.
func (m *MLogger) ErrorFields(args ...any) {
	m.writeLog("ERROR", FormatFields(args...))
}

// FatalFields logs a structured fatal message as logfmt key=value pairs, then exits.
func (m *MLogger) FatalFields(args ...any) {
	m.writeLog("FATAL", FormatFields(args...))
	m.FatalExit()
}
