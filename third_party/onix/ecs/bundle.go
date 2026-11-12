package ecs

type bundleEntry interface {
	componentID(*World) componentID
}

type bundleValue interface {
	appendTo(column, Tick)
}

type entityBundleEntry interface {
	bundleEntry
	materialize(Entity) bundleValue
}

type componentValue[T any] struct {
	value T
}

func (v componentValue[T]) componentID(world *World) componentID {
	return world.registerComponent[T]()
}

func (v componentValue[T]) appendTo(target column, tick Tick) {
	target.(*typedColumn[T]).append(v.value, tick)
}

type entityComponentValue[T any] struct {
	build func(Entity) T
}

func (v entityComponentValue[T]) componentID(world *World) componentID {
	return world.registerComponent[T]()
}

func (v entityComponentValue[T]) materialize(entity Entity) bundleValue {
	return componentValue[T]{value: v.build(entity)}
}

type Bundle struct {
	entries []bundleEntry
}

// PreparedBundle caches the world-specific component IDs and sorted entries
// of a Bundle. It is immutable after creation and can be reused for repeated
// spawns without rebuilding the bundle description.
type PreparedBundle struct {
	world   *World
	entries []preparedBundleEntry
}

type preparedBundleEntry struct {
	entry bundleEntry
	id    componentID
}

func NewBundle(capacity int) Bundle {
	if capacity < 0 {
		panic("ecs: bundle capacity is negative")
	}
	return Bundle{entries: make([]bundleEntry, 0, capacity)}
}

// Extend appends a separately authored component group before spawning. Groups
// must have disjoint component types, as with individual Bundle.Put calls.
func (b *Bundle) Extend(group Bundle) { b.entries = append(b.entries, group.entries...) }

func (bundle *Bundle) Put[T any](value T) {
	if bundle == nil {
		panic("ecs: bundle is nil")
	}
	bundle.entries = append(bundle.entries, componentValue[T]{value: value})
}

// Value inspects a statically supplied component before spawning.
func (bundle Bundle) Value[T any]() (T, bool) {
	for _, entry := range bundle.entries {
		if value, ok := entry.(componentValue[T]); ok {
			return value.value, true
		}
	}
	var zero T
	return zero, false
}

func (bundle *Bundle) PutEntity[T any](build func(Entity) T) {
	if bundle == nil {
		panic("ecs: bundle is nil")
	}
	if build == nil {
		panic("ecs: entity component builder is nil")
	}
	bundle.entries = append(bundle.entries, entityComponentValue[T]{build: build})
}
