package ecs

// ComponentAccess provides explicit per-component change tracking. Query accesses
// expire after iteration; binding accesses expire after structural mutation.
type ComponentAccess[T any] struct {
	value *T
	state *columnState
	// Pack the two uint32 values into one register when passing query rows.
	rowTick uint64
}

func (a ComponentAccess[T]) Get() T {
	return *a.value
}

// Ref returns a read-only component reference without marking it changed.
func (a ComponentAccess[T]) Ref() *T {
	return a.value
}

func (a ComponentAccess[T]) Mut() *T {
	a.markChanged()
	return a.value
}

func (a ComponentAccess[T]) markChanged() {
	tick := Tick(a.rowTick >> 32)
	if tick == 0 {
		tick = a.state.world.mutationTick()
	}
	a.state.markChanged(int(uint32(a.rowTick)), tick)
}

// CommitComponent writes only when the value differs. Query accesses use their
// query tick; bindings obtain a mutation tick only when a value actually changes.
func CommitComponent[T comparable](access ComponentAccess[T], value T) bool {
	if access.value == nil || *access.value == value {
		return false
	}
	*access.value = value
	access.markChanged()
	return true
}
