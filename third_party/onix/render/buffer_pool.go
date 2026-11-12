package render

import "sync"

// bufferPool holds pointer-free upload scratch, never GPU resources or data
// retained by a caller. Pool entries may be discarded by the garbage collector.
type bufferPool[T ~byte | ~uint32 | ~float32] struct {
	pool        sync.Pool
	maxCapacity int
}

type pooledBuffer[T ~byte | ~uint32 | ~float32] struct {
	data []T
}

func (p *bufferPool[T]) get() *pooledBuffer[T] {
	if buffer := p.pool.Get(); buffer != nil {
		return buffer.(*pooledBuffer[T])
	}
	return &pooledBuffer[T]{}
}

func (p *bufferPool[T]) put(buffer *pooledBuffer[T]) {
	if cap(buffer.data) > p.maxCapacity {
		buffer.data = nil
	} else {
		buffer.data = buffer.data[:0]
	}
	// Store the wrapper pointer so returning a slice does not allocate a box.
	p.pool.Put(buffer)
}
