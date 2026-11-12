package ecs

import "reflect"

type resourceSlot[T any] struct {
	value   T
	added   Tick
	changed Tick
}

func (world *World) InsertResource[T any](resource T) {
	requireWorld(world)
	world.mutationVersion++
	typ := reflect.TypeFor[T]()
	tick := world.mutationTick()
	if value, ok := world.resources[typ]; ok {
		slot, ok := value.(*resourceSlot[T])
		if !ok {
			panic("ecs: resource storage type mismatch")
		}
		slot.value = resource
		slot.changed = tick
		return
	}
	world.resources[typ] = &resourceSlot[T]{value: resource, added: tick, changed: tick}
}

func (world *World) GetResource[T any]() (T, bool) {
	var zero T
	requireWorld(world)
	slot, ok := world.resourceSlotFor[T]()
	if !ok {
		return zero, false
	}
	return slot.value, true
}

// ResourceChangeTick returns the last write tick, or zero for a missing resource.
func (world *World) ResourceChangeTick[T any]() Tick {
	requireWorld(world)
	if slot, ok := world.resourceSlotFor[T](); ok {
		return slot.changed
	}
	return 0
}

func (world *World) GetResourceMut[T any]() (*T, bool) {
	requireWorld(world)
	slot, ok := world.resourceSlotFor[T]()
	if !ok {
		return nil, false
	}
	slot.changed = world.mutationTick()
	world.mutationVersion++
	return &slot.value, true
}

func (world *World) RemoveResource[T any]() (T, bool) {
	var zero T
	requireWorld(world)
	typ := reflect.TypeFor[T]()
	value, ok := world.resources[typ]
	if !ok {
		return zero, false
	}
	delete(world.resources, typ)
	world.mutationVersion++
	slot, ok := value.(*resourceSlot[T])
	if !ok {
		panic("ecs: resource storage type mismatch")
	}
	return slot.value, true
}

func (context *SystemContext) ResourceAdded[T any]() bool {
	slot, ok := context.contextResourceSlot[T]()
	return ok && tickInRange(slot.added, context.lastRun, context.thisRun)
}

func (context *SystemContext) ResourceChanged[T any]() bool {
	slot, ok := context.contextResourceSlot[T]()
	return ok && tickInRange(slot.changed, context.lastRun, context.thisRun)
}

func (world *World) resourceSlotFor[T any]() (*resourceSlot[T], bool) {
	value, ok := world.resources[reflect.TypeFor[T]()]
	if !ok {
		return nil, false
	}
	slot, ok := value.(*resourceSlot[T])
	if !ok {
		panic("ecs: resource storage type mismatch")
	}
	return slot, true
}

func (context *SystemContext) contextResourceSlot[T any]() (*resourceSlot[T], bool) {
	if context == nil || context.World == nil {
		return nil, false
	}
	return context.World.resourceSlotFor[T]()
}
