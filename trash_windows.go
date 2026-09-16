package trash

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
)

type guid struct {
	data1 uint32
	data2 uint16
	data3 uint16
	data4 [8]byte
}

var (
	classFileOperation = guid{0x3ad05575, 0x8857, 0x4850, [8]byte{0x92, 0x77, 0x11, 0xb8, 0x5b, 0xdb, 0x8e, 0x09}}
	iidFileOperation   = guid{0x947aab5f, 0x0a5c, 0x4c13, [8]byte{0xb4, 0xd6, 0x4b, 0xf7, 0x83, 0x6f, 0xc9, 0xf8}}
	iidShellItem       = guid{0x43826d1e, 0xe718, 0x42ee, [8]byte{0xbc, 0x55, 0xa1, 0xe2, 0x61, 0xc3, 0x7b, 0xfe}}
)

type shellAPI struct {
	initialize     func(uintptr, uint32) int32
	uninitialize   func()
	createInstance func(*guid, uintptr, uint32, *guid, **fileOperation) int32
	createItem     func(*uint16, uintptr, *guid, **shellItem) int32
}

var loadShell = sync.OnceValues(func() (*shellAPI, error) {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetSystemDirectoryW")
	if err := proc.Find(); err != nil {
		return nil, err
	}
	var getSystemDirectory func(*uint16, uint32) uint32
	purego.RegisterFunc(&getSystemDirectory, proc.Addr())
	buffer := make([]uint16, 32768)
	n := getSystemDirectory(&buffer[0], uint32(len(buffer)))
	if n == 0 || n >= uint32(len(buffer)) {
		return nil, fmt.Errorf("GetSystemDirectoryW failed")
	}
	systemDirectory := syscall.UTF16ToString(buffer[:n])
	api := new(shellAPI)
	type symbol struct {
		name   string
		target any
	}
	for _, library := range []struct {
		name    string
		symbols []symbol
	}{
		{"ole32.dll", []symbol{
			{"CoInitializeEx", &api.initialize},
			{"CoUninitialize", &api.uninitialize},
			{"CoCreateInstance", &api.createInstance},
		}},
		{"shell32.dll", []symbol{
			{"SHCreateItemFromParsingName", &api.createItem},
		}},
	} {
		dll := syscall.NewLazyDLL(filepath.Join(systemDirectory, library.name))
		if err := dll.Load(); err != nil {
			return nil, err
		}
		for _, symbol := range library.symbols {
			proc := dll.NewProc(symbol.name)
			if err := proc.Find(); err != nil {
				return nil, err
			}
			purego.RegisterFunc(symbol.target, proc.Addr())
		}
	}
	return api, nil
})

type fileOperation struct{ vtable *fileOperationVTable }
type fileOperationVTable struct {
	_                       [2]uintptr // IUnknown.QueryInterface, AddRef
	release                 uintptr
	_                       [2]uintptr // Advise, Unadvise
	setOperationFlags       uintptr
	_                       [12]uintptr // SetProgressMessage through CopyItems, in COM vtable order.
	deleteItem              uintptr
	_                       [2]uintptr // DeleteItems, NewItem
	performOperations       uintptr
	getAnyOperationsAborted uintptr
}
type shellItem struct{ vtable *shellItemVTable }
type shellItemVTable struct {
	_       [2]uintptr // IUnknown.QueryInterface, AddRef
	release uintptr
}

func move(path string) error {
	api, err := loadShell()
	if err != nil {
		return err
	}
	// A fresh goroutine owns the STA so an existing caller's COM apartment is
	// never changed. Every interface is released before CoUninitialize.
	result := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		result <- moveSTA(api, path)
	}()
	return <-result
}

func moveSTA(api *shellAPI, path string) error {
	const coinitApartmentThreaded = 0x2
	if err := hresult("CoInitializeEx", api.initialize(0, coinitApartmentThreaded)); err != nil {
		return err
	}
	defer api.uninitialize()
	var operation *fileOperation
	const classContextInProcessServer = 0x1
	if err := hresult("CoCreateInstance", api.createInstance(&classFileOperation, 0, classContextInProcessServer, &iidFileOperation, &operation)); err != nil {
		return err
	}
	var releaseOperation func(*fileOperation) uint32
	purego.RegisterFunc(&releaseOperation, operation.vtable.release)
	defer releaseOperation(operation)

	var setFlags func(*fileOperation, uint32) int32
	purego.RegisterFunc(&setFlags, operation.vtable.setOperationFlags)
	const (
		fofSilent              = 0x0004
		fofNoConfirmation      = 0x0010
		fofNoErrorUI           = 0x0400
		fofNoConnectedElements = 0x2000
		fofWantNukeWarning     = 0x4000
		fofxRecycleOnDelete    = 0x00080000
		fofxEarlyFailure       = 0x00100000
	)
	// Require recycling and abort on errors, including a permanent-delete warning.
	flags := uint32(fofSilent | fofNoConfirmation | fofNoErrorUI | fofNoConnectedElements | fofWantNukeWarning | fofxRecycleOnDelete | fofxEarlyFailure)
	if err := hresult("IFileOperation.SetOperationFlags", setFlags(operation, flags)); err != nil {
		return err
	}
	name, err := syscall.UTF16FromString(path)
	if err != nil {
		return err
	}
	var item *shellItem
	if err := hresult("SHCreateItemFromParsingName", api.createItem(&name[0], 0, &iidShellItem, &item)); err != nil {
		return err
	}
	var releaseItem func(*shellItem) uint32
	purego.RegisterFunc(&releaseItem, item.vtable.release)
	defer releaseItem(item)

	var deleteItem func(*fileOperation, *shellItem, unsafe.Pointer) int32
	purego.RegisterFunc(&deleteItem, operation.vtable.deleteItem)
	if err := hresult("IFileOperation.DeleteItem", deleteItem(operation, item, nil)); err != nil {
		return err
	}
	var perform func(*fileOperation) int32
	purego.RegisterFunc(&perform, operation.vtable.performOperations)
	if err := hresult("IFileOperation.PerformOperations", perform(operation)); err != nil {
		return err
	}
	var getAborted func(*fileOperation, *int32) int32
	purego.RegisterFunc(&getAborted, operation.vtable.getAnyOperationsAborted)
	var aborted int32
	if err := hresult("IFileOperation.GetAnyOperationsAborted", getAborted(operation, &aborted)); err != nil {
		return err
	}
	if aborted != 0 {
		return fmt.Errorf("IFileOperation was aborted")
	}
	return nil
}

func hresult(operation string, result int32) error {
	if result < 0 {
		return fmt.Errorf("%s: HRESULT 0x%08X", operation, uint32(result))
	}
	return nil
}
