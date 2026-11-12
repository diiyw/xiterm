package ecs

import (
	"fmt"
	"math"
	"reflect"
)

type Entity uint64

// PickingID returns the non-zero entity slot encoded into GPU picking targets.
// The value remains stable for the lifetime of the entity and can be resolved
// without maintaining a per-frame map.
func (e Entity) PickingID() uint32 {
	return uint32(e)
}

// EntitySlot is a cached entity lookup bound to one World.
//
// A slot keeps the entity generation, so resolving it after the entity has
// been despawned or its index has been reused fails instead of returning a
// different entity. Create slots during setup and reuse them in hot paths.
type EntitySlot struct {
	world  *World
	entity Entity
}

// Entity returns the raw entity handle stored in the slot. Use EntityBySlot
// when the slot may have outlived a scene mutation and needs validation.
func (s EntitySlot) Entity() Entity {
	return s.entity
}

// Valid reports whether the slot contains a world-bound entity handle. It
// does not prove that the entity is still alive; resolve it through World.
func (s EntitySlot) Valid() bool {
	return s.world != nil && s.entity != 0
}

// Has reports whether the bound entity has the component. Empty and stale
// slots return false.
func (s EntitySlot) Has[T any]() bool {
	return s.world != nil && s.world.Has[T](s.entity)
}

// Get reads a component from the bound world. Empty and stale slots return false.
func (s EntitySlot) Get[T any]() (T, bool) {
	if s.world == nil {
		var zero T
		return zero, false
	}
	return s.world.Get[T](s.entity)
}

// GetRef returns a component reference without marking it changed, or nil for
// an empty or stale slot. The pointer must not outlive structural mutations.
func (s EntitySlot) GetRef[T any]() *T {
	if s.world == nil {
		return nil
	}
	return s.world.GetRef[T](s.entity)
}

// GetMut returns a mutable component reference, or nil for an empty or stale
// slot. The pointer must not outlive structural mutations.
func (s EntitySlot) GetMut[T any]() *T {
	if s.world == nil {
		return nil
	}
	return s.world.GetMut[T](s.entity)
}

// MustMut requires a component on the bound entity. The pointer must not
// outlive structural mutations.
func (s EntitySlot) MustMut[T any]() *T {
	value := s.GetMut[T]()
	if value == nil {
		panic(fmt.Sprintf("ecs: slot %d has no component %s in its world", s.entity, reflect.TypeFor[T]()))
	}
	return value
}

const invalidEntityIndex = math.MaxUint32

func makeEntity(index, generation uint32) Entity {
	return Entity(uint64(generation)<<32 | uint64(index+1))
}

// Index decodes the zero-based metadata slot. It does not validate liveness.
func (e Entity) Index() (uint32, bool) {
	low := uint32(e)
	if low == 0 {
		return 0, false
	}
	return low - 1, true
}

func (e Entity) generation() uint32 {
	return uint32(uint64(e) >> 32)
}
