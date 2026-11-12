package ecs

import "sync"

// temporarySlicePool owns only storage borrowed for one call or iterator.
// Views, components and other caller-owned values must never be returned here.
type temporarySlicePool[T any] struct {
	pool sync.Pool
}

const maxTemporarySliceCapacity = 1024

func (p *temporarySlicePool[T]) get(length int) *[]T {
	storage, _ := p.pool.Get().(*[]T)
	if storage == nil {
		storage = new([]T)
	}
	if cap(*storage) < length {
		*storage = make([]T, length)
	} else {
		*storage = (*storage)[:length]
	}
	return storage
}

func (p *temporarySlicePool[T]) put(storage *[]T) {
	if cap(*storage) > maxTemporarySliceCapacity {
		*storage = nil
	} else {
		clear(*storage)
		*storage = (*storage)[:0]
	}
	p.pool.Put(storage)
}

var (
	spawnEntryPool   temporarySlicePool[preparedBundleEntry]
	spawnValuePool   temporarySlicePool[bundleValue]
	queryBindingPool temporarySlicePool[filterBinding]
)
