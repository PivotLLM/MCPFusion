/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package admin

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/tenebris-tech/mlogger"

	"github.com/PivotLLM/MCPFusion/db"
)

func TestTokenCommandRequested(t *testing.T) {
	tests := []struct {
		name string
		cmd  TokenCommand
		want bool
	}{
		{"none", TokenCommand{}, false},
		{"user only", TokenCommand{User: "u"}, false},
		{"add", TokenCommand{Add: "d"}, true},
		{"list", TokenCommand{List: true}, true},
		{"delete", TokenCommand{Delete: "abc"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cmd.Requested(); got != tt.want {
				t.Errorf("Requested() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserCommandRequested(t *testing.T) {
	tests := []struct {
		name string
		cmd  UserCommand
		want bool
	}{
		{"none", UserCommand{}, false},
		{"token only", UserCommand{Token: "t"}, false},
		{"add", UserCommand{Add: "Alice"}, true},
		{"list", UserCommand{List: true}, true},
		{"delete", UserCommand{Delete: "id"}, true},
		{"link", UserCommand{Link: "id:hash"}, true},
		{"unlink", UserCommand{Unlink: "hash"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cmd.Requested(); got != tt.want {
				t.Errorf("Requested() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthCodeCommandRequested(t *testing.T) {
	if (AuthCodeCommand{URL: "http://x"}).Requested() {
		t.Error("Requested() without a service = true, want false")
	}
	if !(AuthCodeCommand{Service: "google"}).Requested() {
		t.Error("Requested() with a service = false, want true")
	}
}

// Invalid input is rejected before the database is touched, so a nil database
// is safe here.
func TestUserLinkInvalidSpec(t *testing.T) {
	for _, spec := range []string{"nocolon", ":hash", "user:", ":"} {
		t.Run(spec, func(t *testing.T) {
			var err error
			out := captureStdout(t, func() { err = (UserCommand{Link: spec}).Run(nil) })
			assertGuidance(t, err, out, "Use: -user-link user_id:key_hash")
		})
	}
}

func TestAuthCodeNoTokens(t *testing.T) {
	database := newTestDB(t)
	var err error
	out := captureStdout(t, func() {
		err = (AuthCodeCommand{Service: "google", URL: "http://x"}).Run(database, mlogger.NewMemoryLogger())
	})
	assertGuidance(t, err, out, "Create one with: "+os.Args[0]+` -token-add "Description"`)
}

func TestAuthCodeMultipleTokensNeedsToken(t *testing.T) {
	database := newTestDB(t)
	addTokens(t, database, 2)
	var err error
	out := captureStdout(t, func() {
		err = (AuthCodeCommand{Service: "google", URL: "http://x"}).Run(database, mlogger.NewMemoryLogger())
	})
	assertGuidance(t, err, out, "Use -auth-token to specify which token's tenant to use")
}

func TestTokenDeleteAmbiguousPrefix(t *testing.T) {
	database := newTestDB(t)

	// Add tokens until two hashes share a first character, then delete by it.
	seen := map[byte]bool{}
	var prefix string
	for prefix == "" {
		_, hash, err := database.AddAPIToken("test")
		if err != nil {
			t.Fatalf("AddAPIToken: %v", err)
		}
		if seen[hash[0]] {
			prefix = hash[:1]
		}
		seen[hash[0]] = true
	}

	var err error
	out := captureStdout(t, func() { err = (TokenCommand{Delete: prefix}).Run(database) })
	assertGuidance(t, err, out, "Please use a longer prefix")
}

// assertGuidance checks that err carries the guidance and that nothing was
// printed: the CLI reports the guidance with the error.
func assertGuidance(t *testing.T, err error, out, wantGuidance string) {
	t.Helper()
	var ge *GuidanceError
	if !errors.As(err, &ge) {
		t.Fatalf("error = %v, want *GuidanceError", err)
	}
	if ge.Guidance != wantGuidance {
		t.Errorf("guidance = %q, want %q", ge.Guidance, wantGuidance)
	}
	if strings.Contains(err.Error(), wantGuidance) {
		t.Errorf("error string %q contains the guidance", err.Error())
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing printed", out)
	}
}

func newTestDB(t *testing.T) db.Database {
	t.Helper()
	database, err := db.New(db.WithLogger(mlogger.NewMemoryLogger()), db.WithDataDir(t.TempDir()))
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func addTokens(t *testing.T, database db.Database, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if _, _, err := database.AddAPIToken("test"); err != nil {
			t.Fatalf("AddAPIToken: %v", err)
		}
	}
}

// captureStdout runs fn and returns what it wrote to standard output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(out)
}

func TestAuthCodeRequiresURL(t *testing.T) {
	if err := (AuthCodeCommand{Service: "google"}).Run(nil, nil); err == nil {
		t.Error("Run without URL: want error, got nil")
	}
}
