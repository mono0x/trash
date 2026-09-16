package trash

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestMoveErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		path string
		want error
	}{
		{"empty", "", fs.ErrInvalid},
		{"NUL", "file\x00suffix", fs.ErrInvalid},
		{"missing", filepath.Join(t.TempDir(), "missing"), fs.ErrNotExist},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := Move(tt.path)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Move(%q) = %v, want %v", tt.path, err, tt.want)
			}
			var pe *os.PathError
			if !errors.As(err, &pe) || pe.Op != "trash" || pe.Path != tt.path {
				t.Fatalf("Move(%q) = %#v, want PathError with original path", tt.path, err)
			}
		})
	}
}

func TestMoveNULDoesNotTruncate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keep")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Move(path + "\x00suffix"); !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("Move = %v, want ErrInvalid", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
		t.Fatalf("original file changed: %q, %v", data, err)
	}
}
