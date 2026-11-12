package font

import "sync"

type fontBacking struct {
	closeErr error
	release  func() error
	data     []byte

	closeOnce sync.Once
}

func closeFontBacking(backing *fontBacking) {
	_ = backing.close()
}

func (backing *fontBacking) close() error {
	if backing == nil {
		return nil
	}
	backing.closeOnce.Do(func() {
		if backing.release != nil {
			backing.closeErr = backing.release()
		}
		backing.data = nil
	})
	return backing.closeErr
}

func maxFontFileSize() int64 {
	return int64(^uint(0) >> 1)
}
