# trash

A Go library that moves files and directories to the trash using native operating
system APIs. Built with [ebitengine/purego](https://github.com/ebitengine/purego),
it builds and runs with `CGO_ENABLED=0`.

```go
import "github.com/mono0x/trash"

if err := trash.Move("report.txt"); err != nil {
	return err
}
```

Requires Go 1.26 or later. Targets `amd64` and `arm64` on each supported OS.

| OS      | Native API                                                                                                                  | Runtime dependency           |
| ------- | --------------------------------------------------------------------------------------------------------------------------- | ---------------------------- |
| Windows | [IFileOperation](https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/nn-shobjidl_core-ifileoperation)         | Windows Shell / COM          |
| macOS   | [NSFileManager.trashItem](https://developer.apple.com/documentation/foundation/filemanager/trashitem(at:resultingitemurl:)) | Foundation                   |
| Linux   | [g_file_trash](https://docs.gtk.org/gio/method.File.trash.html)                                                             | GLib/GIO (`libgio-2.0.so.0`) |

On Linux, GIO manages the freedesktop.org trash layout and recovery metadata.
The GIO shared library must be installed at runtime; a C compiler and development
headers are not required. `CGO_ENABLED=0` does not eliminate runtime dependencies
on system libraries.

`Move` processes one path synchronously. Relative paths are resolved against the
current working directory. Symbolic links are moved without moving their targets;
nonempty directories are supported. Errors are returned as `*os.PathError` with
the original path. Use `errors.Is(err, fs.ErrNotExist)` for missing paths, and
`errors.Is(err, fs.ErrInvalid)` for empty paths or paths containing NUL bytes.

Filesystems without an available trash and environments missing required shared
libraries return errors. On Windows, the operation uses
[FOFX_RECYCLEONDELETE, FOF_WANTNUKEWARNING, and FOFX_EARLYFAILURE](https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/nf-shobjidl_core-ifileoperation-setoperationflags),
among other flags, and reports aborted operations as errors. The library does not
fall back to deleting files directly. Restoring items, listing the trash, and
emptying the trash are outside the API's scope.
