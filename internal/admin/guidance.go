/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package admin

// GuidanceError is an error with advice for the person running the command.
// The error string stays plain; the CLI reports the guidance after it.
type GuidanceError struct {
	Err      error
	Guidance string
}

func (e *GuidanceError) Error() string { return e.Err.Error() }

func (e *GuidanceError) Unwrap() error { return e.Err }
