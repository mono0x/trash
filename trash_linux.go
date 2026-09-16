package trash

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

type gioAPI struct {
	newFile   func(string) uintptr
	trash     func(uintptr, uintptr, **glibError) int32
	unref     func(uintptr)
	freeError func(*glibError)
}

type glibError struct {
	domain  uint32
	code    int32
	message *byte
}

var loadGIO = sync.OnceValues(func() (*gioAPI, error) {
	handle, err := purego.Dlopen("libgio-2.0.so.0", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, fmt.Errorf("load libgio-2.0.so.0: %w", err)
	}
	api := new(gioAPI)
	for _, symbol := range []struct {
		name   string
		target any
	}{
		{"g_file_new_for_path", &api.newFile},
		{"g_file_trash", &api.trash},
		{"g_object_unref", &api.unref},
		{"g_error_free", &api.freeError},
	} {
		if err := bind(handle, symbol.name, symbol.target); err != nil {
			return nil, errors.Join(fmt.Errorf("load GIO symbol %s: %w", symbol.name, err), purego.Dlclose(handle))
		}
	}
	// GIO registers process-wide types; keep the library loaded.
	return api, nil
})

func bind(handle uintptr, name string, target any) error {
	address, err := purego.Dlsym(handle, name)
	if err != nil {
		return err
	}
	purego.RegisterFunc(target, address)
	return nil
}

func move(path string) error {
	api, err := loadGIO()
	if err != nil {
		return err
	}
	file := api.newFile(path)
	if file == 0 {
		return fmt.Errorf("GIO could not create GFile")
	}
	defer api.unref(file)
	var nativeError *glibError
	ok := api.trash(file, 0, &nativeError)
	if nativeError != nil {
		defer api.freeError(nativeError)
	}
	if ok != 0 {
		return nil
	}
	if nativeError == nil {
		return fmt.Errorf("GIO could not move item to trash")
	}
	return fmt.Errorf("GIO (%d:%d): %s", nativeError.domain, nativeError.code, cString(nativeError.message))
}

func cString(p *byte) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}
