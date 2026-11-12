//go:build darwin && cgo && !ios

package window

/*
#cgo LDFLAGS: -framework AppKit -framework Foundation
#include <stdlib.h>
#include "macos_native.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func openNativeFileDialog(context *Context, options FileDialogOptions) ([]string, error) {
	title := C.CString(options.Title)
	defer C.free(unsafe.Pointer(title))

	defaultPath := C.CString(options.DefaultPath)
	defer C.free(unsafe.Pointer(defaultPath))

	var extensionData []byte
	for _, filter := range options.Filters {
		for _, extension := range filter.Extensions {
			extension = normalizedFileDialogExtension(extension)
			extensionData = append(extensionData, extension...)
			extensionData = append(extensionData, 0)
		}
	}
	var extensionPointer *C.char
	if len(extensionData) > 0 {
		extensionPointer = (*C.char)(unsafe.Pointer(&extensionData[0]))
	}

	var nativeWindow *C.OnixMacWindow
	if context != nil {
		nativeWindow = context.nativeWindow
	}
	var paths **C.char
	count := C.onix_macos_file_dialog(
		nativeWindow,
		C.int(options.Mode),
		title,
		defaultPath,
		extensionPointer,
		C.int(len(extensionData)),
		&paths,
	)
	if count < 0 {
		return nil, fmt.Errorf("%w: %s", ErrFileDialogUnavailable, C.GoString(C.onix_macos_last_error()))
	}
	if count == 0 {
		return nil, nil
	}
	defer C.onix_macos_file_dialog_free(paths, count)
	nativePaths := unsafe.Slice(paths, int(count))
	result := make([]string, 0, int(count))
	for _, path := range nativePaths {
		if path != nil {
			result = append(result, C.GoString(path))
		}
	}
	return result, nil
}
