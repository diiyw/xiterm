package render

import "fmt"

type generationSlot[V any] struct {
	value      V
	generation uint32
	// A live slot links to its own one-based ID; a free slot links to the
	// next free slot (zero terminates). A free-list node can never link to
	// itself. This uses the former occupancy flag's padded space.
	next uint32
}

type GenerationPool[R, V any] struct {
	slots     []generationSlot[V]
	free      uint32
	freeCount uint32
}

func (p *GenerationPool[R, V]) Insert(value V) Handle[R] {
	if p.free != 0 {
		id := p.free
		slot := &p.slots[id-1]
		p.free = slot.next
		p.freeCount--
		slot.value = value
		slot.next = id
		return NewHandle[R](id, slot.generation)
	}
	if uint64(len(p.slots)) == uint64(^uint32(0)) {
		panic("render: resource handle capacity exceeded")
	}
	index := uint32(len(p.slots))
	p.slots = append(p.slots, generationSlot[V]{value: value, generation: 1, next: index + 1})
	return NewHandle[R](index+1, 1)
}

func (p *GenerationPool[R, V]) Get(handle Handle[R]) (V, bool) {
	var zero V
	// Keep validation local so both value and borrowed lookups can be inlined.
	if !handle.Valid() || handle.ID() > uint32(len(p.slots)) {
		return zero, false
	}
	slot := &p.slots[handle.ID()-1]
	if slot.next != handle.ID() || slot.generation != handle.Generation() {
		return zero, false
	}
	return slot.value, true
}

// getRef borrows a live value until the next Insert, Remove or Clear on this
// pool. Native drivers use it within one operation to avoid copying metadata.
func (p *GenerationPool[R, V]) getRef(handle Handle[R]) *V {
	if !handle.Valid() || handle.ID() > uint32(len(p.slots)) {
		return nil
	}
	slot := &p.slots[handle.ID()-1]
	if slot.next != handle.ID() || slot.generation != handle.Generation() {
		return nil
	}
	return &slot.value
}

func (p *GenerationPool[R, V]) Remove(handle Handle[R]) (V, bool) {
	value := p.getRef(handle)
	if value == nil {
		var zero V
		return zero, false
	}
	index := handle.ID() - 1
	slot := &p.slots[index]
	removed := *value
	var zero V
	slot.value = zero
	slot.next = p.free
	slot.generation++
	if slot.generation == 0 {
		slot.generation = 1
	}
	p.free = index + 1
	p.freeCount++
	return removed, true
}

func (p *GenerationPool[R, V]) Clear() {
	p.slots = nil
	p.free = 0
	p.freeCount = 0
}

func (p *GenerationPool[R, V]) Len() int {
	return len(p.slots) - int(p.freeCount)
}

func (p *GenerationPool[R, V]) Range(yield func(V) bool) {
	for index := range p.slots {
		if p.slots[index].next == uint32(index)+1 && !yield(p.slots[index].value) {
			return
		}
	}
}

func (pool *GenerationPool[R, V]) destroyNativeResource(handle Handle[R], label string, destroy func(V) error) error {
	native, ok := pool.Get(handle)
	if !ok {
		return fmt.Errorf("render: invalid %s handle %s", label, handle)
	}
	if err := destroy(native); err != nil {
		return err
	}
	pool.Remove(handle)
	return nil
}
