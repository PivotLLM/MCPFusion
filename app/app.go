/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

// Package app holds the application's identity: name, tagline, copyright and
// version. It is the only place any of them is defined.
//
// Values are unexported and read through accessors, so there is exactly one way
// to read each. The version is a const so the linker cannot override it; only
// build metadata is injected at link time.
package app

import (
	"runtime"
	"strconv"
)

const (
	name      = "MCPFusion"
	tagLine   = "Configuration-driven MCP server for APIs and command-line tools"
	copyright = "Copyright (c) 2025-2026 Tenebris Technologies Inc."

	// version is bare semver. Build tooling reads this line; keep it a single
	// one-line assignment.
	version = "0.4.3"
)

// Build metadata, injected via ldflags:
//
//	-X github.com/PivotLLM/MCPFusion/app.gitCommit=<sha8>
//	-X github.com/PivotLLM/MCPFusion/app.buildTime=<rfc3339>
//	-X github.com/PivotLLM/MCPFusion/app.goVersion=<go version>
//	-X github.com/PivotLLM/MCPFusion/app.buildNumber=<utc yyyymmddhhmmss>
//
// Keep these as plain package-level vars: -X cannot set struct fields and
// fails silently if asked to.
var (
	gitCommit   string
	buildTime   string
	goVersion   string
	buildNumber string
)

// Name returns the product name.
func Name() string { return name }

// TagLine returns the one-line product description.
func TagLine() string { return tagLine }

// Copyright returns the copyright notice.
func Copyright() string { return copyright }

// Version returns the display form, "0.1.0+1a2b3c4d [20260902155301]", omitting
// whichever parts are not stamped.
//
// The commit is joined with "+" (SemVer build metadata, ignored in comparisons)
// so release and commit stay one token when the string is copied.
func Version() string {
	v := version
	if gitCommit != "" {
		v += "+" + gitCommit
	}
	if buildNumber != "" {
		v += " [" + buildNumber + "]"
	}
	return v
}

// SemVer returns the bare release number, for protocol handshakes and version
// comparisons. Use Version everywhere else.
func SemVer() string { return version }

// Build returns the build number (UTC link time as yyyymmddhhmmss), or "" if
// unstamped. Unlike the commit hash, it orders builds.
func Build() string { return buildNumber }

// BuildDate returns the UTC link day as YYYYMMDD, or 0 if unstamped. Use it only
// where an int is required; the full stamp does not fit in 32 bits.
func BuildDate() int {
	if len(buildNumber) < 8 {
		return 0
	}
	n, err := strconv.Atoi(buildNumber[:8])
	if err != nil {
		return 0
	}
	return n
}

// BuildInfo returns the build timestamp and Go toolchain version. The toolchain
// falls back to runtime.Version(); the timestamp stays empty if unstamped.
func BuildInfo() (string, string) {
	if goVersion == "" {
		return buildTime, runtime.Version()
	}
	return buildTime, goVersion
}
