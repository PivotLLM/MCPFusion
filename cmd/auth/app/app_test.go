/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package app

import (
	"runtime"
	"testing"
)

// Standard library only: this file is copied into every project.

func stamp(t *testing.T, commit, number string) {
	t.Helper()
	og, ob := gitCommit, buildNumber
	t.Cleanup(func() { gitCommit, buildNumber = og, ob })
	gitCommit, buildNumber = commit, number
}

func TestVersion(t *testing.T) {
	tests := []struct {
		name, commit, number string
		want                 string
		wantDate             int
	}{
		{"unstamped", "", "", version, 0},
		{"commit only", "abc12345", "", version + "+abc12345", 0},
		{"number only", "", "20260902155301", version + " [20260902155301]", 20260902},
		{"both", "abc12345", "20260902155301", version + "+abc12345 [20260902155301]", 20260902},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stamp(t, tt.commit, tt.number)
			if got := Version(); got != tt.want {
				t.Errorf("Version()=%q want %q", got, tt.want)
			}
			if got := SemVer(); got != version {
				t.Errorf("SemVer()=%q want %q", got, version)
			}
			if got := Build(); got != tt.number {
				t.Errorf("Build()=%q want %q", got, tt.number)
			}
			if got := BuildDate(); got != tt.wantDate {
				t.Errorf("BuildDate()=%d want %d", got, tt.wantDate)
			}
		})
	}
}

func TestVersion_IsRealRelease(t *testing.T) {
	if version == "" || version == "dev" {
		t.Fatalf("version=%q must be a real release number", version)
	}
}

func TestBuildDate_Invalid(t *testing.T) {
	for _, n := range []string{"202609", "YYYYMMDDHHMMSS"} {
		stamp(t, "", n)
		if got := BuildDate(); got != 0 {
			t.Errorf("BuildDate() with %q = %d want 0", n, got)
		}
	}
}

func TestIdentityAccessors(t *testing.T) {
	for label, got := range map[string]string{
		"Name": Name(), "TagLine": TagLine(), "Copyright": Copyright(),
	} {
		if got == "" {
			t.Errorf("%s() must not be empty", label)
		}
	}
}

func TestBuildInfo(t *testing.T) {
	ob, og := buildTime, goVersion
	t.Cleanup(func() { buildTime, goVersion = ob, og })

	buildTime, goVersion = "2026-02-20T00:00:00Z", "go1.23.0"
	if b, g := BuildInfo(); b != buildTime || g != goVersion {
		t.Errorf("BuildInfo()=%q,%q want %q,%q", b, g, buildTime, goVersion)
	}

	buildTime, goVersion = "", ""
	if b, g := BuildInfo(); b != "" || g != runtime.Version() {
		t.Errorf("BuildInfo()=%q,%q want \"\",%q", b, g, runtime.Version())
	}
}
