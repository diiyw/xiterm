package ecs

//go:generate go run ../internal/ecsgen

import (
	"math/bits"
	"slices"
)

type filterKind uint8

const (
	filterAdded filterKind = iota
	filterChanged
	filterWith
	filterWithout
	filterUnscoped
)

type rowFilter struct {
	component componentID
	kind      filterKind
	lastRun   Tick
	thisRun   Tick
}

type querySpec struct {
	required componentMask
	without  componentMask
}

const inlineQueryFilterCount = 8

type queryFilters struct {
	extra  []rowFilter
	count  int
	inline [inlineQueryFilterCount]rowFilter
}

func (filters *queryFilters) append(filter rowFilter) {
	if filters.count < len(filters.inline) {
		filters.inline[filters.count] = filter
	} else {
		filters.extra = append(filters.extra, filter)
	}
	filters.count++
}

func (filters *queryFilters) at(index int) rowFilter {
	if index < len(filters.inline) {
		return filters.inline[index]
	}
	return filters.extra[index-len(filters.inline)]
}

type queryPlan struct {
	spec    querySpec
	blocks  []queryArchetypeBlock
	scanned int
}

// Store only words containing matching tables. Dense matches use one bit per
// table; sparse matches skip gaps instead of scanning the world's whole mask.
type queryArchetypeBlock struct {
	mask  uint64
	index uint32
}

type simpleQueryKey struct {
	count uint8
	ids   [7]componentID
}

func (p *queryPlan) update(world *World) {
	for _, archetype := range world.archetypes[p.scanned:] {
		if archetype.mask.contains(p.spec.required) && !archetype.mask.intersects(p.spec.without) {
			word, bit := archetype.id/64, uint64(1)<<(archetype.id%64)
			if last := len(p.blocks) - 1; last >= 0 && p.blocks[last].index == word {
				p.blocks[last].mask |= bit
			} else {
				p.blocks = append(p.blocks, queryArchetypeBlock{index: word, mask: bit})
			}
		}
	}
	p.scanned = len(world.archetypes)
}

type QueryOption interface {
	resolve(*World) rowFilter
}

type withOption[T any] struct{}
type withoutOption[T any] struct{}
type addedOption[T any] struct{ context *SystemContext }
type changedOption[T any] struct{ context *SystemContext }
type unscopedOption struct{}

func (unscopedOption) resolve(*World) rowFilter { return rowFilter{kind: filterUnscoped} }

// Unscoped visits matching tables independently of the current execution scope.
// It is intended for world-owned indexes that also include suspended scenes.
func Unscoped() QueryOption { return unscopedOption{} }

func (withOption[T]) resolve(world *World) rowFilter {
	return rowFilter{component: world.registerComponent[T](), kind: filterWith}
}

func (withoutOption[T]) resolve(world *World) rowFilter {
	return rowFilter{component: world.registerComponent[T](), kind: filterWithout}
}

func (option addedOption[T]) resolve(world *World) rowFilter {
	return world.resolveChangeFilter[T](option.context, filterAdded)
}

func (option changedOption[T]) resolve(world *World) rowFilter {
	return world.resolveChangeFilter[T](option.context, filterChanged)
}

func With[T any]() QueryOption {
	return withOption[T]{}
}

func Without[T any]() QueryOption {
	return withoutOption[T]{}
}

func (context *SystemContext) Added[T any]() QueryOption {
	return addedOption[T]{context: context}
}

func (context *SystemContext) Changed[T any]() QueryOption {
	return changedOption[T]{context: context}
}

func (world *World) resolveChangeFilter[T any](context *SystemContext, kind filterKind) rowFilter {
	if context == nil {
		panic("ecs: system context is nil")
	}
	if context.World != world {
		panic("ecs: change filter context belongs to a different world")
	}
	return rowFilter{component: world.registerComponent[T](), kind: kind, lastRun: context.lastRun, thisRun: context.thisRun}
}

type queryView struct {
	world    *World
	plan     *queryPlan
	entities []Entity
	filters  queryFilters
	limited  bool
	unscoped bool
}

func (v queryView) begin(callback bool) bool {
	if !callback || v.world == nil || v.plan == nil {
		return false
	}
	v.plan.update(v.world)
	v.world.activeQueryIterators++
	return true
}

func (v queryView) end() {
	v.world.activeQueryIterators--
}

// filterBinding resolves one change filter against one bound table: its tick
// column, its block bounds and the read window the filter was created with.
// All three are constant for the whole traversal of that table.
type filterBinding struct {
	ticks   []Tick
	ranges  []tickRange
	lastRun Tick
	thisRun Tick
}

// queryCursor shares query lifetime, table binding, tick filters and restricted
// entity traversal. Typed adapters traverse ordinary table rows directly.
type queryCursor struct {
	archetype      *archetype
	extra          *[]filterBinding
	scopeTable     *scopeTable
	scope          *QueryScope
	inline         [inlineQueryFilterCount]filterBinding
	view           queryView
	entityIndex    int
	row            int
	scopeBlock     int
	archetypeBits  uint64
	archetypeIndex int
	archetypeBase  uint32
	tick           Tick
}

func (v queryView) cursor(callback bool) (queryCursor, bool) {
	if !v.begin(callback) {
		return queryCursor{}, false
	}
	cursor := queryCursor{view: v, archetypeIndex: -1, row: -1}
	if !v.limited && !v.unscoped && v.world.queryScope != nil {
		cursor.scope = v.world.queryScope
		cursor.scope.bind(v.world)
	}
	if count := v.filters.count; count > len(cursor.inline) {
		cursor.extra = queryBindingPool.get(count - len(cursor.inline))
	}
	return cursor, true
}

// binding returns the resolved storage of one change filter.
func (cursor *queryCursor) binding(index int) *filterBinding {
	if index < len(cursor.inline) {
		return &cursor.inline[index]
	}
	return &(*cursor.extra)[index-len(cursor.inline)]
}

// nextEntity visits a caller-owned entity list in its supplied order. Validate
// generations on every pass because a saved list may contain retired entities.
func (cursor *queryCursor) nextEntity() bool {
	for cursor.entityIndex < len(cursor.view.entities) {
		entity := cursor.view.entities[cursor.entityIndex]
		cursor.entityIndex++
		_, meta, alive := cursor.view.world.entityMeta(entity)
		if !alive {
			continue
		}
		archetype := cursor.view.world.archetypes[meta.archetypeID]
		if archetype != cursor.archetype {
			if !archetype.mask.contains(cursor.view.plan.spec.required) || archetype.mask.intersects(cursor.view.plan.spec.without) {
				continue
			}
			cursor.bindArchetype(archetype)
		}
		cursor.row = int(meta.row)
		if cursor.view.filters.count != 0 && !cursor.rowMatches() {
			continue
		}
		return true
	}
	return false
}

// nextArchetype binds one matching nonempty table. Typed adapters traverse its contiguous
// rows directly, keeping cursor calls and column binding out of the hot loop.
func (cursor *queryCursor) nextArchetype() bool {
	for {
		for cursor.archetypeBits == 0 {
			cursor.archetypeIndex++
			if cursor.archetypeIndex >= len(cursor.view.plan.blocks) {
				return false
			}
			block := cursor.view.plan.blocks[cursor.archetypeIndex]
			cursor.archetypeBits = block.mask & cursor.view.world.occupiedArchetypes[block.index]
			cursor.archetypeBase = block.index * 64
		}
		id := cursor.archetypeBase + uint32(bits.TrailingZeros64(cursor.archetypeBits))
		cursor.archetypeBits &= cursor.archetypeBits - 1
		archetype := cursor.view.world.archetypes[id]
		cursor.scopeTable = nil
		if cursor.scope != nil {
			table := cursor.scope.table(id)
			if table == nil {
				continue
			}
			if table.count == 0 {
				continue
			}
			if int(table.count) != len(archetype.entities) {
				cursor.scopeTable, cursor.scopeBlock = table, 0
			}
		}
		cursor.bindArchetype(archetype)
		return true
	}
}

func (cursor *queryCursor) bindArchetype(archetype *archetype) {
	cursor.archetype = archetype
	for index := 0; index < cursor.view.filters.count; index++ {
		filter := cursor.view.filters.at(index)
		column := archetype.column(filter.component)
		binding := cursor.binding(index)
		binding.ticks = column.ticks(filter.kind)
		binding.ranges = column.ranges(filter.kind)
		binding.lastRun = filter.lastRun
		binding.thisRun = filter.thisRun
	}
}

func (cursor *queryCursor) rangeMatches(rangeIndex int) bool {
	for index := 0; index < cursor.view.filters.count; index++ {
		binding := cursor.binding(index)
		if !binding.ranges[rangeIndex].matches(rowFilter{lastRun: binding.lastRun, thisRun: binding.thisRun}) {
			return false
		}
	}
	return true
}

func (cursor *queryCursor) groupMatches(group int) bool {
	for index := 0; index < cursor.view.filters.count; index++ {
		binding := cursor.binding(index)
		if !tickGroupRange(binding.ranges, group, len(binding.ticks)).matches(rowFilter{lastRun: binding.lastRun, thisRun: binding.thisRun}) {
			return false
		}
	}
	return true
}

// nextBlock skips unchanged groups before testing individual blocks. The
// generated adapters only run row filters inside matching 64-row blocks.
func (cursor *queryCursor) nextBlock(start, length int) (int, int) {
	for start < length {
		if table := cursor.scopeTable; table != nil && len(table.sparse) != 0 {
			for cursor.scopeBlock < len(table.sparse) && int(table.sparse[cursor.scopeBlock]) < start {
				cursor.scopeBlock++
			}
			if cursor.scopeBlock == len(table.sparse) {
				return length, length
			}
			start = int(table.sparse[cursor.scopeBlock])
		}
		if cursor.view.filters.count != 0 && start%queryTickGroupSize == 0 && !cursor.groupMatches(start/queryTickGroupSize) {
			start = (start/queryTickGroupSize + 1) * queryTickGroupSize
			continue
		}
		block := start / queryTickBlockSize
		end := min((block+1)*queryTickBlockSize, length)
		if cursor.view.filters.count != 0 && !cursor.rangeMatches(block) {
			start = end
			continue
		}
		if table := cursor.scopeTable; table != nil {
			if len(table.dense) != 0 {
				if block >= len(table.dense) {
					return length, length
				}
				word := table.dense[block] & (^uint64(0) << uint(start%64))
				if word == 0 {
					start = end
					continue
				}
				start = block*64 + bits.TrailingZeros64(word)
				end = min(end, start+bits.TrailingZeros64(^(word>>uint(start%64))))
			} else {
				end = start + 1
				for cursor.scopeBlock+1 < len(table.sparse) && int(table.sparse[cursor.scopeBlock+1]) == end && end%64 != 0 {
					cursor.scopeBlock++
					end++
				}
			}
		}
		return start, end
	}
	return length, length
}

func (cursor *queryCursor) mutationTick() Tick {
	if cursor.tick == 0 {
		cursor.tick = cursor.view.world.mutationTick()
	}
	return cursor.tick
}

func (cursor *queryCursor) close() {
	if cursor.extra != nil {
		queryBindingPool.put(cursor.extra)
	}
	cursor.view.end()
}

func buildQuery(world *World, key simpleQueryKey, options []QueryOption) queryView {
	key.sort()
	for index := 1; index < int(key.count); index++ {
		if key.ids[index-1] == key.ids[index] {
			panic("ecs: query contains duplicate component type")
		}
	}
	if len(options) == 0 {
		return queryView{world: world, plan: world.simpleQueryPlan(key)}
	}
	// Reuse the world's scratch masks so a warm query builds no new storage.
	spec := &world.queryMasks
	spec.required = maskForInto(spec.required, key.ids[:key.count])
	spec.without = spec.without[:0]
	view := queryView{world: world}
	for _, option := range options {
		if option == nil {
			continue
		}
		filter := option.resolve(world)
		if filter.kind == filterUnscoped {
			view.unscoped = true
			continue
		}
		if filter.kind == filterWithout {
			spec.without = maskWith(spec.without, filter.component)
			continue
		}
		spec.required = maskWith(spec.required, filter.component)
		if filter.kind != filterWith {
			view.filters.append(filter)
		}
	}
	view.plan = world.queryPlan(*spec)
	return view
}

func (cursor *queryCursor) rowMatches() bool {
	row := cursor.row
	for index := 0; index < cursor.view.filters.count; index++ {
		binding := cursor.binding(index)
		if !tickInRange(binding.ticks[row], binding.lastRun, binding.thisRun) {
			return false
		}
	}
	return true
}

func (w *World) queryPlan(spec querySpec) *queryPlan {
	spec.required = spec.required[:lastMaskWord(spec.required)+1]
	spec.without = spec.without[:lastMaskWord(spec.without)+1]
	hash := spec.required.hash(0x243f6a8885a308d3)
	hash = spec.without.hash(hash ^ 0x13198a2e03707344)
	for _, plan := range w.queryPlans[hash] {
		if plan.spec.required.equal(spec.required) && plan.spec.without.equal(spec.without) {
			return plan
		}
	}
	plan := &queryPlan{spec: querySpec{required: slices.Clone(spec.required), without: slices.Clone(spec.without)}}
	w.queryPlans[hash] = append(w.queryPlans[hash], plan)
	return plan
}

func (w *World) simpleQueryPlan(key simpleQueryKey) *queryPlan {
	if plan := w.simpleQueryPlans[key]; plan != nil {
		return plan
	}
	ids := key.ids[:key.count]
	w.queryMasks.required = maskForInto(w.queryMasks.required, ids)
	plan := w.queryPlan(querySpec{required: w.queryMasks.required})
	w.simpleQueryPlans[key] = plan
	return plan
}

func (key *simpleQueryKey) sort() {
	slices.Sort(key.ids[:key.count])
}

// maskWith sets one bit, growing the mask word-wise. Reused capacity is
// cleared so an enlarged mask never inherits stale bits.
func maskWith(mask componentMask, id componentID) componentMask {
	word := int(id) / 64
	if len(mask) <= word {
		length := len(mask)
		mask = slices.Grow(mask, word+1-length)[:word+1]
		clear(mask[length:])
	}
	mask[word] |= uint64(1) << (uint(id) % 64)
	return mask
}
