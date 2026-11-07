/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

// Package fileperm creates folders and files with exactly the requested
// permissions, whatever the process umask.
package fileperm

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// mkdir is os.Mkdir; tests replace it to create a folder concurrently.
var mkdir = os.Mkdir

// MkdirAll creates path and any missing parents like os.MkdirAll, and leaves
// each folder it creates with exactly mode. The mode is set through a
// descriptor opened on the new folder, and refused if a symlink or another
// entry has taken its place. Folders that already exist, or that another
// process creates meanwhile, are left as they are.
func MkdirAll(path string, mode fs.FileMode) error {
	mode = mode.Perm()
	path = filepath.Clean(path)
	fi, err := os.Stat(path)
	if err == nil {
		if fi.IsDir() {
			return nil
		}
		return &os.PathError{Op: "mkdir", Path: path, Err: fs.ErrExist}
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if parent := filepath.Dir(path); parent != path {
		if err := MkdirAll(parent, mode); err != nil {
			return err
		}
	}
	if err := mkdir(path, mode); err != nil {
		if errors.Is(err, fs.ErrExist) {
			// Created concurrently: not ours, so its mode is left alone.
			if fi, statErr := os.Stat(path); statErr == nil && fi.IsDir() {
				return nil
			}
		}
		return err
	}
	return chmodCreatedDir(path, mode)
}

// chmodCreatedDir sets mode on the folder just created at path through a
// descriptor opened without following symlinks, and refuses if what is at
// path is not a folder or not the one that was opened.
func chmodCreatedDir(path string, mode fs.FileMode) error {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("%s: changed while being created: %w", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only descriptor
	opened, err := f.Stat()
	if err != nil {
		return err
	}
	current, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !opened.IsDir() || !current.IsDir() || !os.SameFile(opened, current) {
		return fmt.Errorf("%s: changed while being created", path)
	}
	return f.Chmod(mode)
}

// WriteFile writes data to path like os.WriteFile and leaves the file with
// exactly mode. The mode is set on the open descriptor, never by path.
func WriteFile(path string, data []byte, mode fs.FileMode) error {
	mode = mode.Perm()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if err := f.Chmod(mode); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
