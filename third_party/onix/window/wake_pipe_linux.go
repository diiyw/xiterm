//go:build linux && cgo && !android

package window

import "syscall"

type wakePipe struct {
	read, write int
	signal      [1]byte
	signals     [64]byte
}

func (c *loopControl) openWakePipe() (*wakePipe, error) {
	var descriptors [2]int
	if err := syscall.Pipe2(descriptors[:], syscall.O_NONBLOCK|syscall.O_CLOEXEC); err != nil {
		return nil, err
	}
	pipe := &wakePipe{read: descriptors[0], write: descriptors[1], signal: [1]byte{1}}
	c.wakeMutex.Lock()
	c.wake = func() { _, _ = syscall.Write(pipe.write, pipe.signal[:]) }
	c.wakeMutex.Unlock()
	return pipe, nil
}

func (pipe *wakePipe) drain() {
	for {
		count, err := syscall.Read(pipe.read, pipe.signals[:])
		if err != nil || count == 0 {
			return
		}
	}
}

func (c *loopControl) closeWakePipe(pipe *wakePipe) {
	c.wakeMutex.Lock()
	c.wake = nil
	if pipe != nil && pipe.read != pipe.write {
		_ = syscall.Close(pipe.read)
		_ = syscall.Close(pipe.write)
		pipe.read, pipe.write = 0, 0
	}
	c.wakeMutex.Unlock()
}
