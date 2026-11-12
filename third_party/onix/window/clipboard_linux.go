//go:build linux && cgo && !android && !ios && !js && !wasm && !onix_drm

package window

/*
#include <stdlib.h>
#include "linux_native.h"
*/
import "C"

import "unsafe"

func (c *Context) SetClipboardText(text string) bool {
	if c == nil || c.nativeWindow == nil {
		return false
	}
	value := C.CString(text)
	defer C.free(unsafe.Pointer(value))
	return C.onix_window_set_clipboard_text(c.nativeWindow, value) != 0
}

func (c *Context) ClipboardText() (string, bool) {
	if c == nil || c.nativeWindow == nil {
		return "", false
	}
	length := int(C.onix_window_get_clipboard_text(c.nativeWindow, nil, 0))
	if length < 0 {
		return "", false
	}
	buffer := make([]byte, length+1)
	C.onix_window_get_clipboard_text(c.nativeWindow, (*C.char)(unsafe.Pointer(&buffer[0])), C.int(len(buffer)))
	return string(buffer[:length]), true
}
