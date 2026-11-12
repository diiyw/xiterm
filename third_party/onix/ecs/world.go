package ecs

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"

	"github.com/onix-engine/onix/internal/smallsort"
)

type entityMeta struct {
	generation  uint32
	archetypeID uint32
	row         uint32
	state       entityState
}

type entityState uint8

const (
	entityFree entityState = iota
	entityReserved
	entityAlive
)

type World struct {
	queryPlans              map[uint64][]*queryPlan
	scopes                  *QueryScope
	queryScope              *QueryScope
	simpleQueryPlans        map[simpleQueryKey]*queryPlan
	directSystemContexts    map[string]*SystemContext
	componentTypes          map[reflect.Type]componentID
	frameScratch            map[reflect.Type]any
	messages                map[reflect.Type]messageStore
	resources               map[reflect.Type]any
	archetypesByHash        map[uint64][]uint32
	commands                Commands
	queryMasks              querySpec
	free                    []uint32
	storageCandidates       []*archetype
	occupiedArchetypes      []uint64
	entities                []entityMeta
	archetypes              []*archetype
	componentInfos          []componentInfo
	entityCount             int
	maintenance             uint64
	activeQueryIterators    int
	activeComponentCleanups int
	structureVersion        uint64
	entityVersion           uint64
	mutationVersion         uint64
	changeTick              Tick
	activeChangeTick        Tick
}

func (w *World) MutationVersion() uint64 {
	requireWorld(w)
	return w.mutationVersion
}

// StructureVersion changes when entities are spawned, despawned or moved.
func (w *World) StructureVersion() uint64 {
	requireWorld(w)
	return w.structureVersion
}

// EntityVersion changes only when a live entity is spawned or despawned.
// Adding or removing components does not change entity membership.
func (w *World) EntityVersion() uint64 {
	requireWorld(w)
	return w.entityVersion
}

// ChangeTick returns the current change-tracking clock without advancing it.
func (w *World) ChangeTick() Tick {
	requireWorld(w)
	return w.changeTick
}

// EntitySlots reports the metadata slots used by live, free and reserved
// entities. Indexed external caches can grow to this size and check handles.
func (w *World) EntitySlots() int {
	requireWorld(w)
	return len(w.entities)
}

func NewWorld() *World {
	world := &World{
		componentTypes:   map[reflect.Type]componentID{},
		archetypesByHash: map[uint64][]uint32{},
		queryPlans:       map[uint64][]*queryPlan{},
		simpleQueryPlans: map[simpleQueryKey]*queryPlan{},
		resources:        map[reflect.Type]any{},
		messages:         map[reflect.Type]messageStore{},
		frameScratch:     map[reflect.Type]any{},
	}
	world.archetypeFor(nil)
	world.commands.world = world
	return world
}

func requireWorld(world *World) {
	if world == nil {
		panic("ecs: world is nil")
	}
}

func (w *World) Spawn(bundle Bundle) Entity {
	if w == nil {
		panic("ecs: world is nil")
	}
	w.assertStructuralMutationAllowed()

	var inline [8]preparedBundleEntry
	if len(bundle.entries) > len(inline) {
		return w.spawnLargeBundle(bundle)
	}
	entries := w.prepareBundle(bundle, inline[:])
	entity := w.reserveEntity()
	w.spawnReserved(entity, entries, w.mutationTick())
	return entity
}

func (w *World) spawnLargeBundle(bundle Bundle) Entity {
	storage := spawnEntryPool.get(len(bundle.entries))
	defer spawnEntryPool.put(storage)
	entries := w.prepareBundle(bundle, *storage)
	entity := w.reserveEntity()
	w.spawnReserved(entity, entries, w.mutationTick())
	return entity
}

// PrepareBundle resolves and sorts a bundle for this world. Reuse the result
// with SpawnPrepared when the same component shape is spawned repeatedly.
func (w *World) PrepareBundle(bundle Bundle) PreparedBundle {
	requireWorld(w)
	entries := make([]preparedBundleEntry, len(bundle.entries))
	entries = w.prepareBundle(bundle, entries)
	return PreparedBundle{world: w, entries: entries}
}

// SpawnPrepared spawns a previously prepared bundle without rebuilding its
// component entry list. The prepared bundle must belong to this world.
func (w *World) SpawnPrepared(bundle PreparedBundle) Entity {
	requireWorld(w)
	w.assertStructuralMutationAllowed()
	if bundle.world != w {
		panic("ecs: prepared bundle belongs to a different world")
	}
	entity := w.reserveEntity()
	w.spawnReserved(entity, bundle.entries, w.mutationTick())
	return entity
}

func (w *World) prepareBundle(bundle Bundle, storage []preparedBundleEntry) []preparedBundleEntry {
	entries := storage
	if len(entries) < len(bundle.entries) {
		entries = make([]preparedBundleEntry, len(bundle.entries))
	} else {
		entries = entries[:len(bundle.entries)]
	}
	for index, entry := range bundle.entries {
		entries[index] = preparedBundleEntry{id: entry.componentID(w), entry: entry}
	}
	// Keep short bundles on the inline insertion path.
	if len(entries) <= 32 {
		smallsort.InsertionSortFunc(entries, comparePreparedBundleEntry)
	} else {
		smallsort.SortFunc(entries, comparePreparedBundleEntry)
	}
	for index := 1; index < len(entries); index++ {
		if entries[index-1].id == entries[index].id {
			panic(fmt.Sprintf("ecs: duplicate component %s in bundle", w.componentInfos[entries[index].id].typ))
		}
	}
	return entries
}

func comparePreparedBundleEntry(a, b preparedBundleEntry) int {
	return cmp.Compare(a.id, b.id)
}

func (w *World) spawnReserved(entity Entity, entries []preparedBundleEntry, tick Tick) {
	index, ok := entity.Index()
	if !ok || int(index) >= len(w.entities) {
		panic("ecs: reserved entity is invalid")
	}
	meta := &w.entities[index]
	if meta.generation != entity.generation() || meta.state != entityReserved {
		panic(fmt.Sprintf("ecs: entity %d is not reserved", entity))
	}
	var inline [8]bundleValue
	var values []bundleValue
	var pooledValues *[]bundleValue
	committed := false
	defer func() {
		if pooledValues != nil {
			spawnValuePool.put(pooledValues)
		}
		if !committed {
			w.cancelReserved(entity)
		}
	}()
	for index, prepared := range entries {
		builder, ok := prepared.entry.(entityBundleEntry)
		if !ok {
			continue
		}
		if values == nil {
			if len(entries) <= len(inline) {
				values = inline[:len(entries)]
			} else {
				pooledValues = spawnValuePool.get(len(entries))
				values = *pooledValues
			}
		}
		values[index] = builder.materialize(entity)
	}
	meta = &w.entities[index]
	if meta.generation != entity.generation() || meta.state != entityReserved {
		panic(fmt.Sprintf("ecs: entity %d reservation changed while materializing bundle", entity))
	}

	archetype := w.archetypeForBundle(entries)
	row := archetype.appendEntity(entity)
	// The table was selected by these sorted entries, so ids and columns line
	// up with them index for index.
	for index, prepared := range entries {
		var value bundleValue
		if values != nil {
			value = values[index]
		}
		if value == nil {
			value = prepared.entry.(bundleValue)
		}
		value.appendTo(archetype.columns[index], tick)
	}
	w.commitSpawn(index, archetype, row)
	committed = true
}

// commitSpawn publishes a reserved entity as alive in its new table row.
func (w *World) commitSpawn(index uint32, archetype *archetype, row int) {
	meta := &w.entities[index]
	meta.archetypeID = archetype.id
	meta.row = uint32(row)
	meta.state = entityAlive
	if row == 0 {
		w.occupiedArchetypes[archetype.id/64] |= uint64(1) << (archetype.id % 64)
	}
	w.entityCount++
	w.entityVersion++
	w.structureVersion++
	w.mutationVersion++
}

func (w *World) Despawn(entity Entity) bool {
	requireWorld(w)
	w.assertStructuralMutationAllowed()
	index, meta, ok := w.entityMeta(entity)
	if !ok {
		return false
	}
	archetype := w.archetypes[meta.archetypeID]
	w.removeArchetypeRow(archetype, int(meta.row))
	w.releaseEntity(index)
	w.entityCount--
	w.entityVersion++
	w.structureVersion++
	w.mutationVersion++
	return true
}

func (w *World) IsAlive(entity Entity) bool {
	requireWorld(w)
	_, _, ok := w.entityMeta(entity)
	return ok
}

func (w *World) EntityCount() int {
	requireWorld(w)
	return w.entityCount
}

// ReserveEntities grows entity metadata storage without changing the live
// entity count. Use it before a known spawn burst to keep the burst allocation
// free; archetype columns can be reserved through SpawnerN.Reserve.
func (w *World) ReserveEntities(capacity int) {
	requireWorld(w)
	if capacity <= cap(w.entities) {
		return
	}
	w.entities = slices.Grow(w.entities, capacity-len(w.entities))
}

// EntityByPickingID resolves the current live entity occupying a GPU picking
// slot. A stale or background ID returns false.
func (w *World) EntityByPickingID(id uint32) (Entity, bool) {
	requireWorld(w)
	if id == 0 {
		return 0, false
	}
	index := id - 1
	if int(index) >= len(w.entities) {
		return 0, false
	}
	meta := &w.entities[index]
	if meta.state != entityAlive {
		return 0, false
	}
	return makeEntity(index, meta.generation), true
}

// Slot creates a cached lookup for an alive entity in this world. The slot
// does not keep the entity alive; EntityBySlot reports false after despawn or
// slot reuse. An invalid entity produces the zero slot.
func (w *World) Slot(entity Entity) EntitySlot {
	requireWorld(w)
	if _, _, ok := w.entityMeta(entity); !ok {
		return EntitySlot{}
	}
	return EntitySlot{world: w, entity: entity}
}

// EntityBySlot resolves a cached entity lookup without string or map
// allocation. Slots from another world and stale generations are rejected.
func (w *World) EntityBySlot(slot EntitySlot) (Entity, bool) {
	requireWorld(w)
	if slot.world != w {
		return 0, false
	}
	_, _, ok := w.entityMeta(slot.entity)
	if !ok {
		return 0, false
	}
	return slot.entity, true
}

func (world *World) Insert[T any](entity Entity, value T) error {
	requireWorld(world)
	world.assertStructuralMutationAllowed()
	index, meta, ok := world.entityMeta(entity)
	if !ok {
		return fmt.Errorf("ecs: entity %d does not exist", entity)
	}
	id := world.registerComponent[T]()
	oldArchetype := world.archetypes[meta.archetypeID]
	tick := world.mutationTick()
	if oldArchetype.has(id) {
		oldArchetype.componentColumn[T](id).replace(int(meta.row), value, tick)
		return nil
	}

	newArchetype := world.archetypeWithAdded(oldArchetype, id)
	newRow := newArchetype.appendEntity(entity)
	oldRow := int(meta.row)
	addedIndex := newArchetype.columnIndex(id)
	newArchetype.columns[addedIndex].(*typedColumn[T]).append(value, tick)
	moveSharedRow(oldArchetype, newArchetype, oldRow, addedIndex)
	world.removeArchetypeEntity(oldArchetype, oldRow)
	world.commitMigration(index, newArchetype, newRow)
	return nil
}

// commitMigration binds one metadata slot to its destination row and records
// the structural change.
func (w *World) commitMigration(index uint32, target *archetype, row int) {
	w.entities[index].archetypeID = target.id
	w.entities[index].row = uint32(row)
	if row == 0 {
		w.occupiedArchetypes[target.id/64] |= uint64(1) << (target.id % 64)
	}
	w.structureVersion++
}

// moveSharedRow transfers one row of every column both tables own. skip is the
// target column that has no source counterpart (-1 for none). Both tables keep
// ids sorted, so a single merge walk pairs the columns without any lookup.
func moveSharedRow(source, target *archetype, row, skip int) {
	sourceIndex := 0
	for index, component := range target.ids {
		if index == skip {
			continue
		}
		for source.ids[sourceIndex] < component {
			sourceIndex++
		}
		source.columns[sourceIndex].moveTo(target.columns[index], row)
		sourceIndex++
	}
}

func (world *World) Remove[T any](entity Entity) (T, bool) {
	var zero T
	requireWorld(world)
	world.assertStructuralMutationAllowed()
	index, meta, ok := world.entityMeta(entity)
	if !ok {
		return zero, false
	}
	id, registered := world.componentTypes[reflect.TypeFor[T]()]
	oldArchetype := world.archetypes[meta.archetypeID]
	if !registered || !oldArchetype.has(id) {
		return zero, false
	}
	oldRow := int(meta.row)
	removedIndex := oldArchetype.columnIndex(id)
	removed := oldArchetype.columns[removedIndex].(*typedColumn[T]).values[oldRow]
	if world.componentInfos[id].onRemove != nil {
		world.cleanupComponent(id, entity, oldArchetype.columns[removedIndex], oldRow)
	}
	newArchetype := world.archetypeWithRemoved(oldArchetype, id)
	newRow := newArchetype.appendEntity(entity)
	moveSharedRow(oldArchetype, newArchetype, oldRow, -1)
	oldArchetype.columns[removedIndex].swapRemove(oldRow)
	world.removeArchetypeEntity(oldArchetype, oldRow)
	world.commitMigration(index, newArchetype, newRow)
	return removed, true
}

func (world *World) Has[T any](entity Entity) bool {
	requireWorld(world)
	_, meta, ok := world.entityMeta(entity)
	if !ok {
		return false
	}
	id, registered := world.componentTypes[reflect.TypeFor[T]()]
	return registered && world.archetypes[meta.archetypeID].has(id)
}

func (world *World) Get[T any](entity Entity) (value T, ok bool) {
	column, row := world.componentRow[T](entity)
	if column == nil {
		return
	}
	return column.values[row], true
}

// GetRef returns a component reference without marking it changed.
// Callers must not mutate it or retain it across structural mutations.
func (world *World) GetRef[T any](entity Entity) *T {
	column, row := world.componentRow[T](entity)
	if column == nil {
		return nil
	}
	return &column.values[row]
}

// componentRow locates one component row without marking it changed. A nil
// column reports that the entity is stale or its table lacks the component.
func (world *World) componentRow[T any](entity Entity) (*typedColumn[T], int) {
	requireWorld(world)
	_, meta, ok := world.entityMeta(entity)
	if !ok {
		return nil, 0
	}
	id, registered := world.componentTypes[reflect.TypeFor[T]()]
	if !registered {
		return nil, 0
	}
	// A nil column means this table does not own the component, so one type
	// assertion replaces a separate mask test plus column lookup.
	column, _ := world.archetypes[meta.archetypeID].column(id).(*typedColumn[T])
	if column == nil {
		return nil, 0
	}
	return column, int(meta.row)
}

func (world *World) GetMut[T any](entity Entity) *T {
	column, row := world.componentRow[T](entity)
	if column == nil {
		return nil
	}
	column.markChanged(row, world.mutationTick())
	return &column.values[row]
}

// SetIfChanged updates an existing component only when its value differs.
// It returns false for equal values, missing components and stale entities.
// It does not insert components and may be used during query iteration.
func (world *World) SetIfChanged[T comparable](entity Entity, value T) bool {
	column, row := world.componentRow[T](entity)
	if column == nil || column.values[row] == value {
		return false
	}
	column.replace(row, value, world.mutationTick())
	return true
}

// MustMut returns an existing component with write access. A missing component
// or stale entity is a structural error. Do not retain the pointer across
// structural mutations.
func (world *World) MustMut[T any](entity Entity) *T {
	value := world.GetMut[T](entity)
	if value == nil {
		panic(fmt.Sprintf("ecs: entity %d has no component %s", entity, reflect.TypeFor[T]()))
	}
	return value
}

func (w *World) reserveEntity() Entity {
	if len(w.free) > 0 {
		last := len(w.free) - 1
		index := w.free[last]
		w.free = w.free[:last]
		w.entities[index].state = entityReserved
		return makeEntity(index, w.entities[index].generation)
	}
	if uint64(len(w.entities)) >= uint64(invalidEntityIndex) {
		panic("ecs: entity capacity exhausted")
	}
	index := uint32(len(w.entities))
	w.entities = append(w.entities, entityMeta{generation: 1, archetypeID: invalidEntityIndex, state: entityReserved})
	return makeEntity(index, 1)
}

func (w *World) cancelReserved(entity Entity) bool {
	index, ok := entity.Index()
	if !ok || int(index) >= len(w.entities) {
		return false
	}
	meta := &w.entities[index]
	if meta.generation != entity.generation() || meta.state != entityReserved {
		return false
	}
	w.releaseEntity(index)
	return true
}

func (w *World) releaseEntity(index uint32) {
	meta := &w.entities[index]
	meta.archetypeID = invalidEntityIndex
	meta.row = 0
	meta.state = entityFree
	meta.generation++
	if meta.generation == 0 {
		meta.generation = 1
	}
	w.free = append(w.free, index)
}

func (w *World) entityMeta(entity Entity) (uint32, *entityMeta, bool) {
	index, ok := entity.Index()
	if !ok || int(index) >= len(w.entities) {
		return 0, nil, false
	}
	meta := &w.entities[index]
	if meta.generation != entity.generation() || meta.state != entityAlive {
		return 0, nil, false
	}
	return index, meta, true
}

func (w *World) archetypeFor(ids []componentID) *archetype {
	return w.archetypeForIDs(ids, false)
}

func (w *World) archetypeForOwned(ids []componentID) *archetype {
	return w.archetypeForIDs(ids, true)
}

func (w *World) archetypeWithAdded(source *archetype, id componentID) *archetype {
	if targetID, ok := source.edge(id); ok {
		return w.archetypes[targetID]
	}
	target := w.archetypeForOwned(addComponentID(source.ids, id))
	source.edges = setArchetypeEdge(source.edges, id, target.id)
	target.edges = setArchetypeEdge(target.edges, id, source.id)
	return target
}

func (w *World) archetypeWithRemoved(source *archetype, id componentID) *archetype {
	if targetID, ok := source.edge(id); ok {
		return w.archetypes[targetID]
	}
	target := w.archetypeForOwned(removeComponentID(source.ids, id))
	source.edges = setArchetypeEdge(source.edges, id, target.id)
	target.edges = setArchetypeEdge(target.edges, id, source.id)
	return target
}

func setArchetypeEdge(edges *archetypeEdges, id componentID, target uint32) *archetypeEdges {
	if edges == nil {
		edges = new(archetypeEdges)
		*edges = make(archetypeEdges, 0, 4)
	}
	values := *edges
	index := 0
	for index < len(values) && values[index].component < id {
		index++
	}
	if index < len(values) && values[index].component == id {
		values[index].target = target
		return edges
	}
	values = append(values, archetypeEdge{})
	copy(values[index+1:], values[index:])
	values[index] = archetypeEdge{component: id, target: target}
	*edges = values
	return edges
}

func (w *World) archetypeForIDs(ids []componentID, takeOwnership bool) *archetype {
	hash := hashComponentIDs(ids)
	for _, id := range w.archetypesByHash[hash] {
		candidate := w.archetypes[id]
		if slices.Equal(candidate.ids, ids) {
			return candidate
		}
	}
	if !takeOwnership {
		ids = slices.Clone(ids)
	}
	return w.createArchetype(hash, ids)
}

func (w *World) archetypeForBundle(entries []preparedBundleEntry) *archetype {
	hash := uint64(0xcbf29ce484222325)
	for _, entry := range entries {
		hash = hashComponentID(hash, entry.id)
	}
	for _, id := range w.archetypesByHash[hash] {
		candidate := w.archetypes[id]
		if len(candidate.ids) != len(entries) {
			continue
		}
		matches := true
		for index, entry := range entries {
			if candidate.ids[index] != entry.id {
				matches = false
				break
			}
		}
		if matches {
			return candidate
		}
	}
	ids := make([]componentID, len(entries))
	for index, entry := range entries {
		ids[index] = entry.id
	}
	return w.createArchetype(hash, ids)
}

func (w *World) createArchetype(hash uint64, ids []componentID) *archetype {
	archetype := &archetype{id: uint32(len(w.archetypes)), ids: ids}
	if len(ids) > 0 {
		archetype.mask = make(componentMask, int(ids[len(ids)-1])/64+1)
	}
	archetype.columns = make([]column, len(ids))
	for index, id := range ids {
		info := &w.componentInfos[id]
		archetype.mask[int(id)/64] |= uint64(1) << (uint(id) % 64)
		archetype.columns[index] = info.newColumn()
		archetype.hasCleanup = archetype.hasCleanup || info.onRemove != nil
	}
	w.archetypes = append(w.archetypes, archetype)
	if archetype.id%64 == 0 {
		w.occupiedArchetypes = append(w.occupiedArchetypes, 0)
	}
	w.archetypesByHash[hash] = append(w.archetypesByHash[hash], archetype.id)
	return archetype
}

func (w *World) removeArchetypeRow(archetype *archetype, row int) {
	entity := archetype.entities[row]
	if archetype.hasCleanup {
		for index, id := range archetype.ids {
			if w.componentInfos[id].onRemove != nil {
				w.cleanupComponent(id, entity, archetype.columns[index], row)
			}
		}
	}
	for _, storage := range archetype.columns {
		storage.swapRemove(row)
	}
	w.removeArchetypeEntity(archetype, row)
}

func (w *World) removeArchetypeEntity(archetype *archetype, row int) {
	last := len(archetype.entities) - 1
	if !archetype.retiring && (last == 0 || cap(archetype.entities) > 64 && last <= cap(archetype.entities)/4) {
		archetype.retiring = true
		w.storageCandidates = append(w.storageCandidates, archetype)
	}
	moved := archetype.entities[last]
	archetype.entities[row] = moved
	archetype.entities = archetype.entities[:last]
	archetype.rowVersion = w.structureVersion + 1
	if last == 0 {
		w.occupiedArchetypes[archetype.id/64] &^= uint64(1) << (archetype.id % 64)
	}
	if row != last {
		index, _ := moved.Index()
		w.entities[index].row = uint32(row)
	}
}

func (w *World) assertStructuralMutationAllowed() {
	if w.activeComponentCleanups != 0 {
		panic("ecs: structural mutation during component cleanup; use Commands")
	}
	if w.activeQueryIterators != 0 {
		panic("ecs: structural mutation during query iteration; use Commands")
	}
}

func (w *World) nextChangeTick() Tick {
	w.changeTick++
	if w.changeTick == 0 {
		w.changeTick = 1
	}
	return w.changeTick
}

func (w *World) mutationTick() Tick {
	if w.activeChangeTick != 0 {
		return w.activeChangeTick
	}
	return w.nextChangeTick()
}
