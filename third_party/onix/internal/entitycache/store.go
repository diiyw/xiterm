// Package entitycache stores large system records densely behind a small entity index.
package entitycache

import (
	"slices"

	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/internal/reuse"
)

type entry[T any] struct {
	value  T
	entity ecs.Entity
}

// Store validates full entity generations. Record pointers are borrowed until
// the next Ensure, Delete, Reset or Trim; no pointers into ECS columns are retained.
type Store[T any] struct {
	indices   []uint32 // one-based dense positions
	entries   []entry[T]
	retention [2]reuse.Retention
}

func (s *Store[T]) Get(entity ecs.Entity) *T {
	index, ok := entity.Index()
	if !ok || int(index) >= len(s.indices) {
		return nil
	}
	position := s.indices[index]
	if position == 0 || s.entries[position-1].entity != entity {
		return nil
	}
	return &s.entries[position-1].value
}

func (s *Store[T]) Ensure(entity ecs.Entity) *T {
	index, ok := entity.Index()
	if !ok {
		panic("entitycache: invalid entity")
	}
	if int(index) >= len(s.indices) {
		old := len(s.indices)
		s.indices = slices.Grow(s.indices, int(index)+1-old)[:int(index)+1]
		clear(s.indices[old:])
		s.retention[1].Record(len(s.indices))
	}
	if position := s.indices[index]; position != 0 {
		e := &s.entries[position-1]
		if e.entity != entity {
			*e = entry[T]{entity: entity}
		}
		return &e.value
	}
	s.entries = append(s.entries, entry[T]{entity: entity})
	s.retention[0].Record(len(s.entries))
	s.indices[index] = uint32(len(s.entries))
	return &s.entries[len(s.entries)-1].value
}

func (s *Store[T]) Delete(entity ecs.Entity) {
	index, ok := entity.Index()
	if !ok || int(index) >= len(s.indices) {
		return
	}
	position := s.indices[index]
	if position == 0 || s.entries[position-1].entity != entity {
		return
	}
	last := len(s.entries) - 1
	if int(position)-1 != last {
		s.entries[position-1] = s.entries[last]
		moved, _ := s.entries[position-1].entity.Index()
		s.indices[moved] = position
	}
	s.entries[last] = entry[T]{}
	s.entries = s.entries[:last]
	s.indices[index] = 0
}

func (s *Store[T]) Reset() {
	for i := range s.entries {
		index, _ := s.entries[i].entity.Index()
		s.indices[index] = 0
	}
	clear(s.entries)
	s.entries = s.entries[:0]
}

func (s *Store[T]) Len() int { return len(s.entries) }

// All borrows densely stored values until the next mutation or Trim.
func (s *Store[T]) All(yield func(ecs.Entity, *T) bool) {
	for i := range s.entries {
		if !yield(s.entries[i].entity, &s.entries[i].value) {
			return
		}
	}
}

// Trim drops historical record capacity and trailing unused index slots.
func (s *Store[T]) Trim() {
	s.entries = s.retention[0].Trim(s.entries, 64)
	length := len(s.indices)
	for length > 0 && s.indices[length-1] == 0 {
		length--
	}
	s.indices = s.retention[1].Trim(s.indices[:length], 64)
}

// Prune removes entries rejected by keep. The callback must not mutate this store.
func (s *Store[T]) Prune(keep func(ecs.Entity, *T) bool) {
	for i := 0; i < len(s.entries); {
		e := &s.entries[i]
		if keep(e.entity, &e.value) {
			i++
		} else {
			s.Delete(e.entity)
		}
	}
}
