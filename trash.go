// Package trash moves files and directories to the operating system's trash.
package trash

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Move moves path to the trash using the operating system's native API.
// Relative paths are resolved against the current working directory. A symbolic
// link is moved itself, not its target. Directories include their contents.
// Errors are returned as *os.PathError. Missing paths match fs.ErrNotExist.
// Move requires an available trash on the source filesystem.
func Move(path string) error {
	err := movePath(path)
	if err == nil {
		return nil
	}
	return &os.PathError{Op: "trash", Path: path, Err: err}
}

func movePath(path string) error {
	if path == "" || strings.IndexByte(path, 0) >= 0 {
		return fs.ErrInvalid
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(abs); err != nil {
		if pe, ok := err.(*os.PathError); ok {
			return pe.Err
		}
		return err
	}
	return move(abs)
}
