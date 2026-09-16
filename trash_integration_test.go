package trash

import (
	"errors"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMoveNative(t *testing.T) {
	if os.Getenv("TRASH_INTEGRATION") != "1" {
		t.Skip("set TRASH_INTEGRATION=1 to exercise the native trash")
	}
	if runtime.GOOS == "linux" {
		// Set this before GIO's first call, since GLib caches its data directory.
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}
	t.Run("native error", func(t *testing.T) {
		if err := move(filepath.Join(t.TempDir(), "missing")); err == nil {
			t.Fatal("native API accepted a missing path")
		}
	})
	for _, kind := range []string{"file", "relative file", "directory", "symlink", "broken symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "trash-test-"+filepath.Base(filepath.Dir(root))+"-日本語 😀-"+kind)
			target := filepath.Join(root, "target")
			const content = "recoverable contents"
			switch kind {
			case "file", "relative file":
				if err := os.WriteFile(path, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "child"), []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink", "broken symlink":
				if kind == "symlink" {
					if err := os.WriteFile(target, []byte(content), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(target, path); err != nil {
					if runtime.GOOS == "windows" {
						t.Skipf("symlinks unavailable: %v", err)
					}
					t.Fatal(err)
				}
			}
			input := path
			if kind == "relative file" {
				t.Chdir(root)
				input = filepath.Base(path)
			}
			if err := Move(input); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("source still exists: %v", err)
			}
			restoreTestItem(t, path)
			switch kind {
			case "file", "relative file", "directory":
				check := path
				if kind == "directory" {
					check = filepath.Join(path, "child")
				}
				if data, err := os.ReadFile(check); err != nil || string(data) != content {
					t.Fatalf("restored contents = %q, %v", data, err)
				}
			case "symlink", "broken symlink":
				if actual, err := os.Readlink(path); err != nil || actual != target {
					t.Fatalf("restored link = %q, %v", actual, err)
				}
				if kind == "symlink" {
					if data, err := os.ReadFile(target); err != nil || string(data) != content {
						t.Fatalf("link target changed: %q, %v", data, err)
					}
				}
			}
		})
	}
}

func restoreTestItem(t *testing.T, path string) {
	t.Helper()
	name := filepath.Base(path)
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(filepath.Join(home, ".Trash", name), path); err != nil {
			t.Fatal(err)
		}
	case "linux":
		trash := filepath.Join(os.Getenv("XDG_DATA_HOME"), "Trash")
		infoPath := filepath.Join(trash, "info", name+".trashinfo")
		info, err := os.ReadFile(infoPath)
		if err != nil || !strings.Contains(string(info), "[Trash Info]\n") || !strings.Contains(string(info), "DeletionDate=") {
			t.Fatalf("invalid recovery metadata: %q, %v", info, err)
		}
		var original string
		for line := range strings.SplitSeq(string(info), "\n") {
			if value, ok := strings.CutPrefix(line, "Path="); ok {
				original, err = url.PathUnescape(value)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		if original != path {
			t.Fatalf("recovery path = %q, want %q", original, path)
		}
		if err := os.Rename(filepath.Join(trash, "files", name), path); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(infoPath); err != nil {
			t.Fatal(err)
		}
	case "windows":
		// PowerShell is only used by the test to restore the unique item through
		// the Shell, which also removes its Recycle Bin metadata.
		command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `
$ErrorActionPreference = 'Stop'
$shell = New-Object -ComObject Shell.Application
$items = @($shell.Namespace(10).Items() | Where-Object {
    $_.Name -eq $env:TRASH_TEST_NAME -and $_.ExtendedProperty('System.Recycle.DeletedFrom') -eq $env:TRASH_TEST_PARENT
})
if ($items.Count -ne 1) { throw 'Expected exactly one recycled test item' }
$shell.Namespace($env:TRASH_TEST_PARENT).MoveHere($items[0], 0x14)
`)
		command.Env = append(os.Environ(), "TRASH_TEST_NAME="+name, "TRASH_TEST_PARENT="+filepath.Dir(path))
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("restore: %v\n%s", err, output)
		}
		deadline := time.Now().Add(10 * time.Second)
		for {
			if _, err := os.Lstat(path); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("timed out restoring test item")
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}
