//go:build windows

package font

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

type mappedFontFile struct {
	fontBacking
	address uintptr
	mapping windows.Handle
}

func loadFontFile(path string) (*fontBacking, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if info.Size() > maxFontFileSize() {
		_ = file.Close()
		return nil, fmt.Errorf("font %q is too large to map", path)
	}
	mapping, err := windows.CreateFileMapping(windows.Handle(file.Fd()), nil, windows.PAGE_READONLY, 0, 0, nil)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	address, err := windows.MapViewOfFile(mapping, windows.FILE_MAP_READ, 0, 0, uintptr(info.Size()))
	closeErr := file.Close()
	if err != nil {
		_ = windows.CloseHandle(mapping)
		return nil, err
	}
	if closeErr != nil {
		_ = windows.UnmapViewOfFile(address)
		_ = windows.CloseHandle(mapping)
		return nil, closeErr
	}
	mapped := &mappedFontFile{address: address, mapping: mapping}
	mapped.data = unsafe.Slice((*byte)(unsafe.Pointer(address)), int(info.Size()))
	mapped.fontBacking.release = mapped.close
	return &mapped.fontBacking, nil
}

func (mapped *mappedFontFile) close() error {
	if mapped == nil || mapped.address == 0 {
		return nil
	}
	address, mapping := mapped.address, mapped.mapping
	mapped.address, mapped.mapping, mapped.data = 0, 0, nil
	unmapErr := windows.UnmapViewOfFile(address)
	closeErr := windows.CloseHandle(mapping)
	if unmapErr != nil {
		return unmapErr
	}
	return closeErr
}
