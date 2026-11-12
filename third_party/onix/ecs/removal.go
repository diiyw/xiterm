package ecs

// ComponentType identifies a component without constructing or boxing a value.
// Resolve descriptors once with PrepareRemoval or NewPatchN, outside the execution loop.
type ComponentType struct {
	register func(*World) componentID
}

func Type[T any]() ComponentType {
	return ComponentType{register: (*World).registerComponent[T]}
}

// PreparedRemoval removes a component set in one archetype migration.
// Copies share the same world-bound transition cache. Missing types are ignored.
type PreparedRemoval struct {
	plan *removalPlan
}

type removalPlan struct {
	world       *World
	transitions map[uint32]*removalTransition
	mask        componentMask
}

type removalColumn struct {
	source      column
	target      column // nil for a removed component
	id          componentID
	overwritten bool
}

// moveColumns transfers one source row into the prepared targets, dropping the
// storage of columns whose target is nil.
func moveColumns(columns []removalColumn, row int) {
	for _, entry := range columns {
		if entry.overwritten {
			entry.source.removeRow(row)
		} else if entry.target == nil {
			entry.source.swapRemove(row)
		} else {
			entry.source.moveTo(entry.target, row)
		}
	}
}

type removalTransition struct {
	source  *archetype
	target  *archetype
	columns []removalColumn
	removed []int // indices into columns, without duplicating column references
}

// PrepareRemoval registers the supplied types and binds a reusable removal set.
// Duplicate descriptors are harmless; an empty set never changes an entity.
func (world *World) PrepareRemoval(types ...ComponentType) PreparedRemoval {
	requireWorld(world)
	world.assertStructuralMutationAllowed()
	return PreparedRemoval{plan: &removalPlan{
		world: world, mask: componentTypeMask(world, types), transitions: make(map[uint32]*removalTransition),
	}}
}

func componentTypeMask(world *World, types []ComponentType) componentMask {
	var mask componentMask
	for _, typ := range types {
		if typ.register == nil {
			panic("ecs: uninitialized component type")
		}
		mask = maskWith(mask, typ.register(world))
	}
	return mask
}

// Reserve prepares populated or preallocated source archetypes and reserves
// each destination for capacity entities. Unused shapes still need warming.
func (r PreparedRemoval) Reserve(capacity int) {
	p := r.requirePlan()
	p.world.assertStructuralMutationAllowed()
	if capacity <= 0 {
		return
	}
	for _, source := range p.world.archetypes {
		if cap(source.entities) == 0 {
			continue
		}
		if target := p.transition(source).target; target != source {
			target.reserve(capacity)
		}
	}
}

// Apply returns whether any components were removed. It is allocation-free
// once the source transition and destination capacity have been prepared.
// Cleanup hooks see the complete source row and run before storage is moved.
// Hook panics do not move the row; external side effects are not rolled back.
func (r PreparedRemoval) Apply(entity Entity) bool {
	p := r.requirePlan()
	w := p.world
	w.assertStructuralMutationAllowed()
	index, meta, ok := w.entityMeta(entity)
	if !ok {
		return false
	}
	transition := p.transition(w.archetypes[meta.archetypeID])
	if transition.source == transition.target {
		return false
	}
	row := int(meta.row)
	if transition.source.hasCleanup {
		for _, index := range transition.removed {
			removed := transition.columns[index]
			if w.componentInfos[removed.id].onRemove != nil {
				w.cleanupComponent(removed.id, entity, removed.source, row)
			}
		}
	}
	targetRow := transition.target.appendEntity(entity)
	moveColumns(transition.columns, row)
	w.removeArchetypeEntity(transition.source, row)
	w.commitMigration(index, transition.target, targetRow)
	return true
}

func (r PreparedRemoval) requirePlan() *removalPlan {
	if r.plan == nil {
		panic("ecs: removal is not initialized")
	}
	return r.plan
}

func (p *removalPlan) transition(source *archetype) *removalTransition {
	if transition := p.transitions[source.id]; transition != nil {
		return transition
	}
	transition := &removalTransition{source: source, target: source}
	if source.mask.intersects(p.mask) {
		ids := make([]componentID, 0, len(source.ids))
		transition.columns = make([]removalColumn, len(source.ids))
		for i, id := range source.ids {
			entry := removalColumn{id: id, source: source.columns[i]}
			if p.mask.has(id) {
				transition.removed = append(transition.removed, i)
			} else {
				ids = append(ids, id)
			}
			transition.columns[i] = entry
		}
		transition.target = p.world.archetypeForOwned(ids)
		// Removed columns keep a nil target; Apply drops their storage instead
		// of moving it into the destination table.
		targetIndex := 0
		for i := range transition.columns {
			entry := &transition.columns[i]
			if targetIndex < len(ids) && ids[targetIndex] == entry.id {
				entry.target = transition.target.columns[targetIndex]
				targetIndex++
			}
		}
	}
	p.transitions[source.id] = transition
	return transition
}
