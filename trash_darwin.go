package trash

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var loadFoundation = sync.OnceValue(func() error {
	// Foundation must stay loaded while its Objective-C classes are registered.
	_, err := purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	return err
})

var (
	newSelector            = objc.RegisterName("new")
	drainSelector          = objc.RegisterName("drain")
	fileURLSelector        = objc.RegisterName("fileURLWithFileSystemRepresentation:isDirectory:relativeToURL:")
	defaultManagerSelector = objc.RegisterName("defaultManager")
	trashSelector          = objc.RegisterName("trashItemAtURL:resultingItemURL:error:")
	descriptionSelector    = objc.RegisterName("localizedDescription")
	utf8Selector           = objc.RegisterName("UTF8String")
)

func move(path string) error {
	if err := loadFoundation(); err != nil {
		return fmt.Errorf("load Foundation: %w", err)
	}
	// Autorelease pools belong to the current native thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(newSelector)
	defer pool.Send(drainSelector)

	url := objc.ID(objc.GetClass("NSURL")).Send(fileURLSelector, path, false, objc.ID(0))
	if url == 0 {
		return fmt.Errorf("create file URL")
	}
	manager := objc.ID(objc.GetClass("NSFileManager")).Send(defaultManagerSelector)
	var nativeError objc.ID
	if objc.Send[bool](manager, trashSelector, url, uintptr(0), &nativeError) {
		return nil
	}
	if nativeError == 0 {
		return fmt.Errorf("NSFileManager could not move item to trash")
	}
	description := nativeError.Send(descriptionSelector)
	return fmt.Errorf("NSFileManager: %s", objc.Send[string](description, utf8Selector))
}
