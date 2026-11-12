package ecs

import (
	"reflect"
	"slices"
)

type Tick uint32

const maxChangeAge = ^uint32(0) - (2*518_400_000 - 1)

func tickInRange(value, lastRun, thisRun Tick) bool {
	if value == 0 {
		return false
	}
	ticksSinceChange := min(uint32(thisRun-value), maxChangeAge)
	ticksSinceSystem := min(uint32(thisRun-lastRun), maxChangeAge)
	return ticksSinceSystem > ticksSinceChange
}

type componentID uint32

type componentInfo struct {
	typ       reflect.Type
	newColumn func() column
	onRemove  func(*World, Entity, column, int)
	version   uint64
	structure uint64
	values    uint64
	members   uint64
}

type column interface {
	reserve(int)
	compact(int)
	moveTo(column, int)
	swapRemove(int)
	removeRow(int)
	ticks(filterKind) []Tick
	ranges(filterKind) []tickRange
	markChanged(int, Tick)
}

type typedColumn[T any] struct {
	values []T
	columnState
}

// columnState keeps change tracking independent of the component value type.
// Query access can hold this metadata and an already resolved value pointer.
type columnState struct {
	world         *World
	added         []Tick
	changed       []Tick
	addedRanges   []tickRange
	changedRanges []tickRange
	id            componentID
}

const queryTickBlockSize = 64

const queryTickGroupBlocks = 64
const queryTickGroupSize = queryTickBlockSize * queryTickGroupBlocks

// Blocks grow from the front and group caches from the back of the same slice.
// This preserves direct block indexing on writes without extra slice headers.
func tickRangeCount(rows int) int {
	blocks := (rows + queryTickBlockSize - 1) / queryTickBlockSize
	return blocks + (blocks+queryTickGroupBlocks-1)/queryTickGroupBlocks
}

func growTickRanges(ranges []tickRange, rows, capacity int) []tickRange {
	needed := tickRangeCount(capacity)
	if needed <= len(ranges) {
		return ranges
	}
	ranges = slices.Grow(ranges, needed-len(ranges))
	ranges = ranges[:cap(ranges)]
	// Moving the group cache to the new tail invalidates it. Only leaf block
	// bounds are persistent; the gap and all group slots start empty.
	clear(ranges[(rows+queryTickBlockSize-1)/queryTickBlockSize:])
	return ranges
}

func includeTick(ranges []tickRange, row int, tick Tick) {
	// Component insertion and mutation clocks never produce the zero sentinel.
	ranges[row/queryTickBlockSize].includeNonzero(tick)
	// Zero invalidates the group cache. Rebuild only when a query needs it,
	// instead of updating a second min/max pair on every component write.
	ranges[len(ranges)-1-row/queryTickGroupSize].max = 0
}

func tickGroupRange(ranges []tickRange, group, rows int) tickRange {
	index := len(ranges) - 1 - group
	if bounds := ranges[index]; bounds.max != 0 {
		return bounds
	}
	var bounds tickRange
	start := group * queryTickGroupBlocks
	end := min(start+queryTickGroupBlocks, (rows+queryTickBlockSize-1)/queryTickBlockSize)
	for _, block := range ranges[start:end] {
		bounds.invertedMin = max(bounds.invertedMin, block.invertedMin)
		bounds.max = max(bounds.max, block.max)
	}
	ranges[index] = bounds
	return bounds
}

// Inverting the lower bound lets both bounds accumulate with max, starting at
// zero without a special case for the first tick. Removed ticks are retained
// conservatively and may cause extra scans.
type tickRange struct{ invertedMin, max Tick }

func (bounds *tickRange) include(tick Tick) {
	if tick == 0 {
		return
	}
	bounds.includeNonzero(tick)
}

func (bounds *tickRange) includeNonzero(tick Tick) {
	bounds.invertedMin = max(bounds.invertedMin, ^tick)
	bounds.max = max(bounds.max, tick)
}

func (bounds tickRange) matches(filter rowFilter) bool {
	window := min(uint32(filter.thisRun-filter.lastRun), maxChangeAge)
	if window == 0 || bounds.max == 0 {
		return false
	}
	first := filter.thisRun - Tick(window-1)
	if first <= filter.thisRun {
		return bounds.max >= first && ^bounds.invertedMin <= filter.thisRun
	}
	return bounds.max >= first || ^bounds.invertedMin <= filter.thisRun
}

func (c *columnState) touch() {
	c.world.componentInfos[c.id].version++
	c.world.mutationVersion++
}

func (c *typedColumn[T]) reserve(additional int) {
	if additional <= 0 {
		return
	}
	c.values = slices.Grow(c.values, additional)
	c.added = slices.Grow(c.added, additional)
	c.changed = slices.Grow(c.changed, additional)
	c.addedRanges = growTickRanges(c.addedRanges, len(c.values), len(c.values)+additional)
	c.changedRanges = growTickRanges(c.changedRanges, len(c.values), len(c.values)+additional)
}

func (c *typedColumn[T]) append(value T, tick Tick) {
	c.appendWithTicks(value, tick, tick)
	c.world.componentInfos[c.id].values++
	c.world.componentInfos[c.id].members++
}

func (c *typedColumn[T]) appendWithTicks(value T, added, changed Tick) {
	c.values = append(c.values, value)
	c.added = append(c.added, added)
	c.changed = append(c.changed, changed)
	row := len(c.values) - 1
	if tickRangeCount(len(c.values)) > len(c.addedRanges) {
		c.addedRanges = growTickRanges(c.addedRanges, row, len(c.values))
		c.changedRanges = growTickRanges(c.changedRanges, row, len(c.values))
	}
	if row%queryTickBlockSize == 0 {
		c.addedRanges[row/queryTickBlockSize] = tickRange{}
		c.changedRanges[row/queryTickBlockSize] = tickRange{}
	}
	includeTick(c.addedRanges, row, added)
	includeTick(c.changedRanges, row, changed)
	c.touch()
	c.world.componentInfos[c.id].structure++
}

func (c *typedColumn[T]) replace(row int, value T, tick Tick) {
	c.values[row] = value
	c.markChanged(row, tick)
}

// moveTo transfers one row into target and drops it from the source. Change
// ticks are preserved; both tables record the migration.
func (c *typedColumn[T]) moveTo(target column, row int) {
	dst := target.(*typedColumn[T])
	dst.appendWithTicks(c.values[row], c.added[row], c.changed[row])
	c.removeRow(row)
}

func (c *typedColumn[T]) swapRemove(row int) {
	c.removeRow(row)
	c.world.componentInfos[c.id].values++
	c.world.componentInfos[c.id].members++
}

func (c *typedColumn[T]) removeRow(row int) {
	last := len(c.values) - 1
	if row != last {
		c.values[row] = c.values[last]
		c.added[row] = c.added[last]
		c.changed[row] = c.changed[last]
		includeTick(c.addedRanges, row, c.added[row])
		includeTick(c.changedRanges, row, c.changed[row])
	}
	var zero T
	c.values[last] = zero
	c.values = c.values[:last]
	c.added = c.added[:last]
	c.changed = c.changed[:last]
	c.touch()
	c.world.componentInfos[c.id].structure++
}

func (c *columnState) ticks(kind filterKind) []Tick {
	if kind == filterAdded {
		return c.added
	}
	return c.changed
}

func (c *columnState) markChanged(row int, tick Tick) {
	c.changed[row] = tick
	includeTick(c.changedRanges, row, tick)
	c.touch()
	c.world.componentInfos[c.id].values++
}

func (c *columnState) ranges(kind filterKind) []tickRange {
	if kind == filterAdded {
		return c.addedRanges
	}
	return c.changedRanges
}

func (world *World) registerComponent[T any]() componentID {
	if world == nil {
		panic("ecs: world is nil")
	}
	typ := reflect.TypeFor[T]()
	if id, ok := world.componentTypes[typ]; ok {
		return id
	}
	id := componentID(len(world.componentInfos))
	world.componentTypes[typ] = id
	world.componentInfos = append(world.componentInfos, componentInfo{
		typ: typ,
		newColumn: func() column {
			return &typedColumn[T]{columnState: columnState{world: world, id: id}}
		},
	})
	return id
}

// ComponentRevision reads the content and structural versions in one lookup.
// Missing types return zero without registering storage or advancing ticks.
func (world *World) ComponentRevision[T any]() (version, structure uint64) {
	requireWorld(world)
	if id, ok := world.componentTypes[reflect.TypeFor[T]()]; ok {
		info := &world.componentInfos[id]
		return info.version, info.structure
	}
	return 0, 0
}

// ComponentValueRevision reports writes and membership changes, excluding
// migrations that only relocate unchanged values between archetypes.
// The second result changes only when values are inserted or removed.
func (world *World) ComponentValueRevision[T any]() (version, membership uint64) {
	requireWorld(world)
	if id, ok := world.componentTypes[reflect.TypeFor[T]()]; ok {
		info := &world.componentInfos[id]
		return info.values, info.members
	}
	return 0, 0
}

// ComponentVersion changes for writes, insertion, removal and row migration.
func (world *World) ComponentVersion[T any]() uint64 {
	version, _ := world.ComponentRevision[T]()
	return version
}

// ComponentStructureVersion changes when rows are inserted or removed.
func (world *World) ComponentStructureVersion[T any]() uint64 {
	_, structure := world.ComponentRevision[T]()
	return structure
}

func (archetype *archetype) componentColumn[T any](id componentID) *typedColumn[T] {
	return archetype.column(id).(*typedColumn[T])
}

// OnComponentRemove installs a cleanup hook for removal and despawn. Hooks
// must not change entity structure; archetype migration does not invoke them.
func (world *World) OnComponentRemove[T any](cleanup func(*World, Entity, *T)) {
	id := world.registerComponent[T]()
	if cleanup == nil {
		world.componentInfos[id].onRemove = nil
		return
	}
	world.componentInfos[id].onRemove = func(world *World, entity Entity, storage column, row int) {
		cleanup(world, entity, &storage.(*typedColumn[T]).values[row])
	}
	for _, archetype := range world.archetypes {
		if archetype.has(id) {
			archetype.hasCleanup = true
		}
	}
}

func (w *World) cleanupComponent(id componentID, entity Entity, storage column, row int) {
	cleanup := w.componentInfos[id].onRemove
	if cleanup == nil {
		return
	}
	w.activeComponentCleanups++
	defer func() { w.activeComponentCleanups-- }()
	cleanup(w, entity, storage, row)
}
