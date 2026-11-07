/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

// Package fileperm creates folders and files with exactly the requested
// permissions, whatever the process umask.
package fileperm

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// MkdirAll creates path and any missing parents like os.MkdirAll, and leaves
// each folder it creates with exactly mode. Folders that already exist are
// left as they are.
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
	if err := os.Mkdir(path, mode); err != nil {
		if errors.Is(err, fs.ErrExist) {
			// Created concurrently: not ours, so its mode is left alone.
			if fi, statErr := os.Stat(path); statErr == nil && fi.IsDir() {
				return nil
			}
		}
		return err
	}
	return os.Chmod(path, mode)
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
