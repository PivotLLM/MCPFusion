/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package admin

import (
	"testing"
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
			if err := (UserCommand{Link: spec}).Run(nil); err == nil {
				t.Errorf("Run with link %q: want error, got nil", spec)
			}
		})
	}
}

func TestAuthCodeRequiresURL(t *testing.T) {
	if err := (AuthCodeCommand{Service: "google"}).Run(nil, nil); err == nil {
		t.Error("Run without URL: want error, got nil")
	}
}
