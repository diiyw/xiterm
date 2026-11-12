package ecs

// ComponentBinding resolves a component type once for repeated access in one world.
// It retains no column pointers and survives entity migrations.
type ComponentBinding[T any] struct {
	world *World
	id    componentID
}

func (w *World) BindComponent[T any]() ComponentBinding[T] {
	return ComponentBinding[T]{world: w, id: w.registerComponent[T]()}
}

// EntityRow borrows one entity's location until the next structural mutation.
// Use a fresh Row after spawning, despawning, adding/removing components or
// World.Maintain, which may compact component storage.
type EntityRow struct {
	world     *World
	table     *archetype
	row       uint32
	structure uint64
}

func (w *World) Row(entity Entity) EntityRow {
	requireWorld(w)
	_, meta, ok := w.entityMeta(entity)
	if !ok {
		return EntityRow{}
	}
	return EntityRow{world: w, table: w.archetypes[meta.archetypeID], row: meta.row, structure: w.structureVersion}
}

// Access returns a missing access (Ref is nil) for an invalid row or absent component.
// Reads preserve ticks. Mut and CommitComponent mark actual write access.
func (b ComponentBinding[T]) Access(row EntityRow) ComponentAccess[T] {
	if row.world == nil {
		return ComponentAccess[T]{}
	}
	if b.world != row.world {
		panic("ecs: component binding belongs to a different world")
	}
	if row.structure != b.world.structureVersion {
		panic("ecs: entity row expired after structural mutation")
	}
	column, _ := row.table.column(b.id).(*typedColumn[T])
	if column == nil {
		return ComponentAccess[T]{}
	}
	return ComponentAccess[T]{value: &column.values[row.row], state: &column.columnState, rowTick: uint64(row.row)}
}

func (b ComponentBinding[T]) Get(row EntityRow) (value T, ok bool) {
	if ref := b.Access(row).Ref(); ref != nil {
		return *ref, true
	}
	return
}
