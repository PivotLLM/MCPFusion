/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package mlogger

// NullLogger discards all log messages. Useful when no logging is needed.
type NullLogger struct{}

var _ Logger = (*NullLogger)(nil)

// NewNullLogger returns a NullLogger that discards all log messages.
func NewNullLogger() *NullLogger { return &NullLogger{} }

func (n *NullLogger) Debug(string)             {}
func (n *NullLogger) Info(string)              {}
func (n *NullLogger) Notice(string)            {}
func (n *NullLogger) Warning(string)           {}
func (n *NullLogger) Error(string)             {}
func (n *NullLogger) Fatal(string)             {}
func (n *NullLogger) Debugf(string, ...any)    {}
func (n *NullLogger) Infof(string, ...any)     {}
func (n *NullLogger) Noticef(string, ...any)   {}
func (n *NullLogger) Warningf(string, ...any)  {}
func (n *NullLogger) Errorf(string, ...any)    {}
func (n *NullLogger) Fatalf(string, ...any)    {}
func (n *NullLogger) DebugFields(...any)       {}
func (n *NullLogger) InfoFields(...any)        {}
func (n *NullLogger) NoticeFields(...any)      {}
func (n *NullLogger) WarningFields(...any)     {}
func (n *NullLogger) ErrorFields(...any)       {}
func (n *NullLogger) FatalFields(...any)       {}
func (n *NullLogger) FatalExit()               {}
func (n *NullLogger) Close()                   {}
