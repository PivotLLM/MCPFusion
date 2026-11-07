//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fileperm

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// narrowUmask sets a umask that would narrow 0770/0660 to 0700/0600 and
// restores the previous one when the test ends. The umask is process-wide,
// so tests using it must not run in parallel.
func narrowUmask(t *testing.T) {
	t.Helper()
	old := syscall.Umask(0o077)
	t.Cleanup(func() { syscall.Umask(old) })
}

func perm(t *testing.T, path string) fs.FileMode {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return fi.Mode().Perm()
}

func TestMkdirAll_ExactUnderUmask(t *testing.T) {
	narrowUmask(t)
	root := t.TempDir()
	existing := filepath.Join(root, "existing")
	if err := os.Mkdir(existing, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(existing, "a", "b", "c")

	if err := MkdirAll(target, 0o770); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for _, dir := range []string{
		filepath.Join(existing, "a"),
		filepath.Join(existing, "a", "b"),
		target,
	} {
		if got := perm(t, dir); got != 0o770 {
			t.Errorf("%s mode = %o, want 770", dir, got)
		}
	}
	if got := perm(t, existing); got != 0o700 {
		t.Errorf("existing folder mode = %o, want 700 (left alone)", got)
	}

	// A second call on an existing tree changes nothing.
	if err := os.Chmod(target, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := MkdirAll(target, 0o770); err != nil {
		t.Fatalf("MkdirAll existing: %v", err)
	}
	if got := perm(t, target); got != 0o750 {
		t.Errorf("existing target mode = %o, want 750 (left alone)", got)
	}
}

func TestMkdirAll_PathIsFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MkdirAll(file, 0o770); err == nil {
		t.Error("MkdirAll on a file: want error, got nil")
	}
	if err := MkdirAll(filepath.Join(file, "sub"), 0o770); err == nil {
		t.Error("MkdirAll under a file: want error, got nil")
	}
}

func TestWriteFile_ExactUnderUmask(t *testing.T) {
	narrowUmask(t)
	dir := t.TempDir()

	tests := []struct {
		name     string
		existing bool
	}{
		{"new file", false},
		{"existing file", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.name)
			if tt.existing {
				if err := os.WriteFile(path, []byte("old contents"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := WriteFile(path, []byte("data"), 0o660); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			if got := perm(t, path); got != 0o660 {
				t.Errorf("mode = %o, want 660", got)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != "data" {
				t.Errorf("contents = %q, want %q", got, "data")
			}
		})
	}
}

func TestWriteFile_MissingDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "f")
	if err := WriteFile(path, []byte("x"), 0o660); err == nil {
		t.Error("WriteFile into a missing folder: want error, got nil")
	}
}

func TestChmodCreatedDir_SymlinkSwapRefused(t *testing.T) {
	narrowUmask(t)
	root := t.TempDir()
	victim := filepath.Join(root, "victim")
	if err := os.Mkdir(victim, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "new")
	if err := os.Symlink(victim, path); err != nil {
		t.Fatal(err)
	}

	if err := chmodCreatedDir(path, 0o777); err == nil {
		t.Fatal("chmodCreatedDir on a symlink: want error, got nil")
	}
	if got := perm(t, victim); got != 0o700 {
		t.Errorf("symlink target mode = %o, want 700 (unchanged)", got)
	}
}

func TestChmodCreatedDir_FileRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := chmodCreatedDir(path, 0o777); err == nil {
		t.Fatal("chmodCreatedDir on a file: want error, got nil")
	}
	if got := perm(t, path); got != 0o600 {
		t.Errorf("file mode = %o, want 600 (unchanged)", got)
	}
}

func TestMkdirAll_ConcurrentCreateLeftAlone(t *testing.T) {
	narrowUmask(t)
	target := filepath.Join(t.TempDir(), "racy")
	orig := mkdir
	t.Cleanup(func() { mkdir = orig })
	mkdir = func(name string, mode fs.FileMode) error {
		if name == target {
			// Another process wins the race and creates the folder first.
			if err := os.Mkdir(name, 0o700); err != nil {
				return err
			}
			if err := os.Chmod(name, 0o711); err != nil {
				return err
			}
		}
		return orig(name, mode)
	}

	if err := MkdirAll(target, 0o770); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if got := perm(t, target); got != 0o711 {
		t.Errorf("concurrently created folder mode = %o, want 711 (left alone)", got)
	}
}
