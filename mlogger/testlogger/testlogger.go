/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

// Package testlogger provides a Logger implementation backed by testing.T.
// Import this package only in test files.
package testlogger

import (
	"fmt"
	"testing"

	"github.com/PivotLLM/MCPFusion/mlogger"
)

// Logger routes all log output through testing.T so messages appear only on
// test failure and are attributed to the correct test.
type Logger struct {
	T *testing.T
}

var _ mlogger.Logger = (*Logger)(nil)

// New returns a Logger backed by t.
func New(t *testing.T) *Logger {
	t.Helper()
	return &Logger{T: t}
}

func (l *Logger) Debug(msg string)             { l.T.Log("[DEBUG] " + msg) }
func (l *Logger) Info(msg string)              { l.T.Log("[INFO] " + msg) }
func (l *Logger) Notice(msg string)            { l.T.Log("[NOTICE] " + msg) }
func (l *Logger) Warning(msg string)           { l.T.Log("[WARNING] " + msg) }
func (l *Logger) Error(msg string)             { l.T.Log("[ERROR] " + msg) }
func (l *Logger) Fatal(msg string)             { l.T.Fatal("[FATAL] " + msg) }
func (l *Logger) Debugf(f string, v ...any)    { l.T.Log("[DEBUG] " + fmt.Sprintf(f, v...)) }
func (l *Logger) Infof(f string, v ...any)     { l.T.Log("[INFO] " + fmt.Sprintf(f, v...)) }
func (l *Logger) Noticef(f string, v ...any)   { l.T.Log("[NOTICE] " + fmt.Sprintf(f, v...)) }
func (l *Logger) Warningf(f string, v ...any)  { l.T.Log("[WARNING] " + fmt.Sprintf(f, v...)) }
func (l *Logger) Errorf(f string, v ...any)    { l.T.Log("[ERROR] " + fmt.Sprintf(f, v...)) }
func (l *Logger) Fatalf(f string, v ...any)    { l.T.Fatalf("[FATAL] "+f, v...) }
func (l *Logger) DebugFields(v ...any)         { l.T.Log("[DEBUG] " + mlogger.FormatFields(v...)) }
func (l *Logger) InfoFields(v ...any)          { l.T.Log("[INFO] " + mlogger.FormatFields(v...)) }
func (l *Logger) NoticeFields(v ...any)        { l.T.Log("[NOTICE] " + mlogger.FormatFields(v...)) }
func (l *Logger) WarningFields(v ...any)       { l.T.Log("[WARNING] " + mlogger.FormatFields(v...)) }
func (l *Logger) ErrorFields(v ...any)         { l.T.Log("[ERROR] " + mlogger.FormatFields(v...)) }
func (l *Logger) FatalFields(v ...any)         { l.T.Fatal("[FATAL] " + mlogger.FormatFields(v...)) }
func (l *Logger) FatalExit()                   { l.T.Fatal("[FATAL EXIT]") }
func (l *Logger) Close()                       {}
