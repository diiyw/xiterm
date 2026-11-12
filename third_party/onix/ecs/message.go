package ecs

import (
	"iter"
	"reflect"
)

type messageStore interface {
	advance()
}

type messageBuffer[T any] struct {
	older      []T
	newer      []T
	olderStart uint64
	newerStart uint64
	count      uint64
}

func (b *messageBuffer[T]) write(message T) {
	if len(b.newer) == 0 {
		b.newerStart = b.count
	}
	b.newer = append(b.newer, message)
	b.count++
}

// oldestStart reports the sequence number of the oldest retained message.
func (b *messageBuffer[T]) oldestStart() uint64 {
	if len(b.older) > 0 {
		return b.olderStart
	}
	return b.newerStart
}

func (b *messageBuffer[T]) readInto(cursor *uint64, out []T) []T {
	start := max(*cursor, b.oldestStart())
	if start >= b.count {
		*cursor = b.count
		return out[:0]
	}
	needed := b.count - start
	if uint64(cap(out)) < needed {
		out = make([]T, 0, needed)
	} else {
		out = out[:0]
	}
	if len(b.older) > 0 && start < b.newerStart {
		offset := start - b.olderStart
		out = append(out, b.older[offset:]...)
		start = b.newerStart
	}
	if start < b.count {
		offset := start - b.newerStart
		out = append(out, b.newer[offset:]...)
	}
	*cursor = b.count
	return out
}

func (b *messageBuffer[T]) advance() {
	recycled := b.older
	clear(recycled)
	b.older = b.newer
	b.olderStart = b.newerStart
	b.newer = recycled[:0]
	b.newerStart = b.count
}

// MessageReader tracks consumption of one message type within one world.
// Its zero value starts at the oldest retained message. Consumers use separate
// readers; copying a reader copies its current position.
type MessageReader[T any] struct {
	cursor uint64
}

// Read copies unread messages to a new slice and consumes them. Use All for
// direct iteration or ReadInto when a retained copy needs reusable storage.
func (r *MessageReader[T]) Read(world *World) []T {
	return r.ReadInto(world, nil)
}

// ReadInto writes unread messages into a caller-owned buffer and advances the
// reader. Reusing the returned slice avoids the allocation performed by Read.
func (r *MessageReader[T]) ReadInto(world *World, dst []T) []T {
	if r == nil {
		panic("ecs: message reader is nil")
	}
	requireWorld(world)
	return world.messageBufferFor[T]().readInto(&r.cursor, dst)
}

// All visits unread messages without copying them. The reader consumes each
// message before yielding it, so break, return, and panic retain the remaining
// messages. Pointers are borrowed only for the current iteration body; do not
// advance message buffers during iteration. Use a separate reader per consumer.
func (r *MessageReader[T]) All(world *World) iter.Seq[*T] {
	// Keep iteration here so Go can inline direct consumers into this closure.
	return func(yield func(*T) bool) {
		if r == nil {
			panic("ecs: message reader is nil")
		}
		requireWorld(world)
		b := world.messageBufferFor[T]()
		cursor := &r.cursor
		start := max(*cursor, b.oldestStart())
		if len(b.older) > 0 && start < b.newerStart {
			offset := start - b.olderStart
			for index := int(offset); index < len(b.older); index++ {
				*cursor = b.olderStart + uint64(index) + 1
				if !yield(&b.older[index]) {
					return
				}
			}
			start = b.newerStart
		}
		if start < b.count {
			offset := start - b.newerStart
			for index := int(offset); index < len(b.newer); index++ {
				*cursor = b.newerStart + uint64(index) + 1
				if !yield(&b.newer[index]) {
					return
				}
			}
		}
		*cursor = b.count
	}
}

// Discard skips all currently retained messages for this reader in constant
// time. Other readers and messages written later are unaffected.
func (r *MessageReader[T]) Discard(world *World) {
	if r == nil {
		panic("ecs: message reader is nil")
	}
	requireWorld(world)
	r.cursor = world.messageBufferFor[T]().count
}

func (world *World) WriteMessage[T any](message T) {
	requireWorld(world)
	world.mutationVersion++
	world.messageBufferFor[T]().write(message)
}

func (world *World) AdvanceMessages() {
	requireWorld(world)
	for _, messages := range world.messages {
		messages.advance()
	}
}

type messageScratchKey[Tag, T any] struct{}

type messageScratch[T any] struct {
	older   []T
	scratch Scratch[T]
}

func (s *messageScratch[T]) advance() {
	recycled := s.older
	clear(recycled)
	s.older = s.scratch.Values
	s.scratch.Values = recycled[:0]
}

// MessageScratch provides two-buffer storage for data borrowed by messages.
// Append to Values; AdvanceMessages retains it for the next message frame and
// recycles the older buffer. Tags isolate arenas even when T is identical.
// Do not overwrite borrowed values before two calls to AdvanceMessages.
func (world *World) MessageScratch[Tag, T any]() *Scratch[T] {
	requireWorld(world)
	key := reflect.TypeFor[messageScratchKey[Tag, T]]()
	if value, ok := world.messages[key]; ok {
		return &value.(*messageScratch[T]).scratch
	}
	value := &messageScratch[T]{}
	world.messages[key] = value
	return &value.scratch
}

func (world *World) messageBufferFor[T any]() *messageBuffer[T] {
	typ := reflect.TypeFor[T]()
	if store, ok := world.messages[typ]; ok {
		return store.(*messageBuffer[T])
	}
	store := &messageBuffer[T]{}
	world.messages[typ] = store
	return store
}
