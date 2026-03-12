/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package mlogger

import (
	"fmt"
	"sync"
)

// MemoryLogger captures log messages in memory. Thread-safe.
// Useful for tests that need to assert on log output.
type MemoryLogger struct {
	mu   sync.Mutex
	logs []string
}

var _ Logger = (*MemoryLogger)(nil)

// NewMemoryLogger returns a MemoryLogger that captures all log messages.
func NewMemoryLogger() *MemoryLogger {
	return &MemoryLogger{logs: make([]string, 0)}
}

// Logs returns a copy of all captured log messages.
func (m *MemoryLogger) Logs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cpy := make([]string, len(m.logs))
	copy(cpy, m.logs)
	return cpy
}

// Reset clears all captured log messages.
func (m *MemoryLogger) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = m.logs[:0]
}

func (m *MemoryLogger) add(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, msg)
}

func (m *MemoryLogger) Debug(msg string)             { m.add("DEBUG: " + msg) }
func (m *MemoryLogger) Info(msg string)              { m.add("INFO: " + msg) }
func (m *MemoryLogger) Notice(msg string)            { m.add("NOTICE: " + msg) }
func (m *MemoryLogger) Warning(msg string)           { m.add("WARNING: " + msg) }
func (m *MemoryLogger) Error(msg string)             { m.add("ERROR: " + msg) }
func (m *MemoryLogger) Fatal(msg string)             { m.add("FATAL: " + msg) }
func (m *MemoryLogger) Debugf(f string, v ...any)    { m.add(fmt.Sprintf("DEBUG: "+f, v...)) }
func (m *MemoryLogger) Infof(f string, v ...any)     { m.add(fmt.Sprintf("INFO: "+f, v...)) }
func (m *MemoryLogger) Noticef(f string, v ...any)   { m.add(fmt.Sprintf("NOTICE: "+f, v...)) }
func (m *MemoryLogger) Warningf(f string, v ...any)  { m.add(fmt.Sprintf("WARNING: "+f, v...)) }
func (m *MemoryLogger) Errorf(f string, v ...any)    { m.add(fmt.Sprintf("ERROR: "+f, v...)) }
func (m *MemoryLogger) Fatalf(f string, v ...any)    { m.add(fmt.Sprintf("FATAL: "+f, v...)) }
func (m *MemoryLogger) DebugFields(v ...any)         { m.add("DEBUG: " + FormatFields(v...)) }
func (m *MemoryLogger) InfoFields(v ...any)          { m.add("INFO: " + FormatFields(v...)) }
func (m *MemoryLogger) NoticeFields(v ...any)        { m.add("NOTICE: " + FormatFields(v...)) }
func (m *MemoryLogger) WarningFields(v ...any)       { m.add("WARNING: " + FormatFields(v...)) }
func (m *MemoryLogger) ErrorFields(v ...any)         { m.add("ERROR: " + FormatFields(v...)) }
func (m *MemoryLogger) FatalFields(v ...any)         { m.add("FATAL: " + FormatFields(v...)) }
func (m *MemoryLogger) FatalExit()                   { m.add("FATAL: FatalExit called") }
func (m *MemoryLogger) Close()                       {}
