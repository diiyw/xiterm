//go:build (linux || darwin) && !android && !ios

package font

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

type mappedFontFile struct {
	fontBacking
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
	data, err := unix.Mmap(int(file.Fd()), 0, int(info.Size()), unix.PROT_READ, unix.MAP_SHARED)
	closeErr := file.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		_ = unix.Munmap(data)
		return nil, closeErr
	}
	mapped := &mappedFontFile{}
	mapped.data = data
	mapped.fontBacking.release = mapped.close
	return &mapped.fontBacking, nil
}

func (mapped *mappedFontFile) close() error {
	if mapped == nil || mapped.data == nil {
		return nil
	}
	data := mapped.data
	mapped.data = nil
	return unix.Munmap(data)
}
