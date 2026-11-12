//go:build cgo && (android || linux || darwin || ios || (windows && onix_opengl))

package render

/*
#include <stdlib.h>
*/
import "C"

import "unsafe"

// nativeTextureUpload stores a validated batch in native memory. Go fills it
// once, then the synchronous graphics API consumes each region in place.
type nativeTextureUpload struct {
	pointer   unsafe.Pointer
	capacity  int
	used      int
	retention bufferRetention
}

func (upload *nativeTextureUpload) reserve(length int) ([]byte, bool) {
	if length > upload.capacity {
		capacity := nextTextureUploadCapacity(length)
		pointer := C.realloc(upload.pointer, C.size_t(capacity))
		if pointer == nil {
			return nil, false
		}
		upload.pointer, upload.capacity = pointer, capacity
	}
	upload.used = max(upload.used, length)
	return unsafe.Slice((*byte)(upload.pointer), length), true
}

func nextTextureUploadCapacity(length int) int {
	capacity := max(64<<10, length)
	if length > 64<<10 && length/2 <= int(^uint(0)>>1)-length {
		capacity = length + length/2
	}
	return capacity
}

func (upload *nativeTextureUpload) trim() {
	used := upload.used
	upload.used = 0
	if target := upload.retention.trimCapacity(upload.capacity, used, 64<<10); target != 0 {
		if used == 0 {
			upload.release()
			return
		}
		if pointer := C.realloc(upload.pointer, C.size_t(target)); pointer != nil {
			upload.pointer, upload.capacity = pointer, target
		}
	}
}

func (upload *nativeTextureUpload) release() {
	C.free(upload.pointer)
	*upload = nativeTextureUpload{}
}
