package ecs

import (
	"iter"
	"math/bits"
	"reflect"
	"slices"

	"github.com/onix-engine/onix/internal/reuse"
)

// QueryScope confines ordinary queries to a borrowed entity set. Direct
// component access remains available for loading and lifetime management.
// Ordinary queries use table order; explicit Query.In lists preserve order.
// Membership is compiled at structural boundaries; warm iteration does not allocate.
// World.Maintain retires caches unused for 30 frames. Borrowed membership survives.
type QueryScope struct {
	next      *QueryScope
	values    map[reflect.Type]any
	world     *World
	entities  []Entity
	tables    []scopeTable
	directory []uint32 // one-based positions of participating tables
	retention [2]reuse.Retention
	used      uint64
	version   uint64
	compiled  uint64
	structure uint64
	pending   bool
}

// Sparse tables store sorted row IDs; dense tables store one bit per row.
// The directory costs four bytes per historical table, with large records
// allocated only for tables that have actually contained scope members.
type scopeTable struct {
	dense     []uint64
	sparse    []uint32
	retention reuse.Retention
	used      uint64
	id        uint32
	count     uint32
}

func (s *QueryScope) table(id uint32) *scopeTable {
	if int(id) >= len(s.directory) || s.directory[id] == 0 {
		return nil
	}
	return &s.tables[s.directory[id]-1]
}

func (s *QueryScope) bind(world *World) {
	s.touch(world)
	if s.compiled == s.version {
		if s.structure == world.structureVersion {
			return
		}
		if !s.pending && s.rowsUnchanged(world) {
			s.structure = world.structureVersion
			return
		}
	}
	for i := range s.tables {
		table := &s.tables[i]
		table.count = 0
		if cap(table.dense) != 0 {
			words := (len(world.archetypes[table.id].entities) + 63) / 64
			table.dense = slices.Grow(table.dense, max(0, words-len(table.dense)))[:words]
			clear(table.dense)
		}
		table.sparse = table.sparse[:0]
	}
	s.pending = false
	// Rebuild in the existing representation in one entity pass. Adapt once
	// per participating table afterwards; no per-entity work array is retained.
	for _, entity := range s.entities {
		_, meta, alive := world.entityMeta(entity)
		if !alive {
			if index, valid := entity.Index(); valid && (int(index) >= len(world.entities) || world.entities[index].generation == entity.generation()) {
				s.pending = true
			}
			continue
		}
		id := meta.archetypeID
		if int(id) >= len(s.directory) {
			s.directory = append(s.directory, make([]uint32, int(id)+1-len(s.directory))...)
		}
		if s.directory[id] == 0 {
			s.tables = append(s.tables, scopeTable{id: id})
			s.directory[id] = uint32(len(s.tables))
		}
		table := s.table(id)
		table.count++
		if len(table.dense) != 0 {
			table.dense[meta.row/64] |= uint64(1) << (meta.row % 64)
		} else {
			table.sparse = append(table.sparse, meta.row)
		}
	}
	for i := range s.tables {
		table := &s.tables[i]
		if table.count == 0 {
			table.dense = table.dense[:0]
			continue
		}
		table.used = world.maintenance
		rows := len(world.archetypes[table.id].entities)
		if int(table.count) > rows/128 {
			if len(table.dense) == 0 {
				table.dense = make([]uint64, (rows+63)/64)
				for _, row := range table.sparse {
					table.dense[row/64] |= uint64(1) << (row % 64)
				}
				table.sparse = nil
				table.retention = reuse.Retention{}
			}
			table.count = 0
			for _, word := range table.dense {
				table.count += uint32(bits.OnesCount64(word))
			}
		} else {
			if len(table.dense) != 0 {
				for index, word := range table.dense {
					for word != 0 {
						table.sparse = append(table.sparse, uint32(index*64+bits.TrailingZeros64(word)))
						word &= word - 1
					}
				}
				table.dense = nil
				table.retention = reuse.Retention{}
			} else {
				slices.Sort(table.sparse)
				table.sparse = slices.Compact(table.sparse)
			}
			table.count = uint32(len(table.sparse))
		}
	}
	s.compiled, s.structure = s.version, world.structureVersion
}

func (s *QueryScope) rowsUnchanged(world *World) bool {
	for i := range s.tables {
		table := &s.tables[i]
		if table.count != 0 && world.archetypes[table.id].rowVersion > s.structure {
			return false
		}
	}
	return true
}

func (s *QueryScope) maintain() {
	for i := 0; i < len(s.tables); {
		table := &s.tables[i]
		if table.count == 0 && s.world.maintenance-table.used >= reuse.RetentionFrames {
			s.directory[table.id] = 0
			last := len(s.tables) - 1
			s.tables[i] = s.tables[last]
			if i != last {
				s.directory[s.tables[i].id] = uint32(i + 1)
			}
			s.tables[last] = scopeTable{}
			s.tables = s.tables[:last]
			continue
		}
		if len(table.dense) != 0 {
			table.dense = table.retention.Trim(table.dense, 4)
		} else {
			table.sparse = table.retention.Trim(table.sparse, 4)
		}
		i++
	}
	length := len(s.directory)
	for length > 0 && s.directory[length-1] == 0 {
		length--
	}
	s.directory = s.retention[0].Trim(s.directory[:length], 64)
	s.tables = s.retention[1].Trim(s.tables, 4)
	for _, value := range s.values {
		if cache, ok := value.(interface{ MaintainCache() }); ok {
			cache.MaintainCache()
		}
	}
}

// ScopeValue keeps long-lived system caches separate for execution and
// rendering scopes. Switching scopes does not rebuild the other scope's cache.
// Fetch values again when using a scope after ReleaseCache, an empty Reset or
// 30 frames without using it; previously borrowed cache pointers have expired.
func (world *World) ScopeValue[Tag, T any]() *T {
	scope := world.queryScope
	if scope == nil {
		return world.FrameValue[Tag, T]()
	}
	scope.touch(world)
	if scope.values == nil {
		scope.values = make(map[reflect.Type]any)
	}
	return cachedValue[Tag, T](scope.values)
}

func (s *QueryScope) Reset(entities []Entity) {
	if s.world != nil {
		s.world.assertStructuralMutationAllowed()
	}
	s.entities = entities
	s.version++
	if len(entities) == 0 {
		s.ReleaseCache()
	}
}

// ReleaseCache retires compiled membership and system values, preserving the
// borrowed entity set. Existing cache pointers must no longer be used.
func (s *QueryScope) ReleaseCache() {
	if s.world != nil {
		s.world.assertStructuralMutationAllowed()
		for link := &s.world.scopes; *link != nil; link = &(*link).next {
			if *link == s {
				*link = s.next
				break
			}
		}
	}
	s.dropCache()
}

func (s *QueryScope) dropCache() {
	s.values = nil
	s.tables = nil
	s.world = nil
	s.next = nil
	s.compiled = 0
	s.structure = 0
	s.directory = nil
	s.pending = false
	s.retention = [2]reuse.Retention{}
	s.used = 0
}

func (s *QueryScope) touch(world *World) {
	if s.world != world {
		s.ReleaseCache()
		s.world = world
		s.next = world.scopes
		world.scopes = s
		s.compiled = s.version - 1
	}
	s.used = world.maintenance
}

func (w *World) QueryScope() *QueryScope { return w.queryScope }

// InQueryScope reports whether a live entity belongs to the current query
// scope. It lets hierarchy traversals honor scopes without scanning them.
func (w *World) InQueryScope(entity Entity) bool {
	_, meta, alive := w.entityMeta(entity)
	if !alive || w.queryScope == nil {
		return alive
	}
	scope := w.queryScope
	scope.bind(w)
	table := scope.table(meta.archetypeID)
	if table == nil || table.count == 0 {
		return false
	}
	if len(table.dense) != 0 {
		word := int(meta.row) / 64
		return word < len(table.dense) && table.dense[word]&(uint64(1)<<(meta.row%64)) != 0
	}
	_, found := slices.BinarySearch(table.sparse, meta.row)
	return found
}

// SetQueryScope returns the previous scope. Nil restores unrestricted queries.
// A scope cannot change while a query iterator is live.
func (w *World) SetQueryScope(scope *QueryScope) *QueryScope {
	if w.activeQueryIterators != 0 {
		panic("ecs: cannot change query scope during iteration")
	}
	previous := w.queryScope
	w.queryScope = scope
	return previous
}

// QueryScopeKey identifies both the selected scope and its structural revision.
func (w *World) QueryScopeKey() (*QueryScope, uint64) {
	if w.queryScope == nil {
		return nil, 0
	}
	return w.queryScope, w.queryScope.version
}

// Entities traverses every live entity independently of the current scope.
// It is intended for rebuilding scopes, not normal system updates.
func (w *World) Entities() iter.Seq[Entity] {
	return func(yield func(Entity) bool) {
		w.activeQueryIterators++
		defer func() { w.activeQueryIterators-- }()
		for block, word := range w.occupiedArchetypes {
			for word != 0 {
				table := w.archetypes[block*64+bits.TrailingZeros64(word)]
				word &= word - 1
				for _, entity := range table.entities {
					if !yield(entity) {
						return
					}
				}
			}
		}
	}
}
