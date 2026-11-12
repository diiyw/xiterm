package ecs

import (
	"math/bits"
	"slices"

	"github.com/onix-engine/onix/internal/reuse"
)

type componentMask []uint64

// maskFor builds a mask from ids in any order. Words grow on demand, so the
// result never keeps trailing zero words.
func maskFor(ids []componentID) componentMask {
	return maskForInto(nil, ids)
}

// maskForInto rebuilds a caller-owned mask in place, reusing its capacity.
func maskForInto(mask componentMask, ids []componentID) componentMask {
	mask = mask[:0]
	for _, id := range ids {
		mask = maskWith(mask, id)
	}
	return mask
}

func (m componentMask) has(id componentID) bool {
	word := int(id) / 64
	return word < len(m) && m[word]&(uint64(1)<<(uint(id)%64)) != 0
}

// without clears one bit in place, ignoring ids outside the current range.
func (m componentMask) without(id componentID) componentMask {
	if word := int(id) / 64; word < len(m) {
		m[word] &^= uint64(1) << (uint(id) % 64)
	}
	return m
}

func (m componentMask) contains(other componentMask) bool {
	for i, word := range other {
		if i >= len(m) || m[i]&word != word {
			return false
		}
	}
	return true
}

func (m componentMask) intersects(other componentMask) bool {
	limit := min(len(m), len(other))
	for i := 0; i < limit; i++ {
		if m[i]&other[i] != 0 {
			return true
		}
	}
	return false
}

// equal compares masks without their trailing zero words.
func (m componentMask) equal(other componentMask) bool {
	return slices.Equal(m[:lastMaskWord(m)+1], other[:lastMaskWord(other)+1])
}

func lastMaskWord(mask componentMask) int {
	for i := len(mask) - 1; i >= 0; i-- {
		if mask[i] != 0 {
			return i
		}
	}
	return -1
}

func (m componentMask) hash(seed uint64) uint64 {
	hash := seed
	for _, word := range m {
		hash ^= uint64(bits.RotateLeft64(word, 17)) + 0x9e3779b97f4a7c15 + hash<<6 + hash>>2
	}
	return hash
}

func hashComponentIDs(ids []componentID) uint64 {
	hash := uint64(0xcbf29ce484222325)
	for _, id := range ids {
		hash = hashComponentID(hash, id)
	}
	return hash
}

func hashComponentID(hash uint64, id componentID) uint64 {
	return hash ^ (uint64(bits.RotateLeft64(uint64(id), 17)) + 0x9e3779b97f4a7c15 + hash<<6 + hash>>2)
}

type archetype struct {
	edges *archetypeEdges
	// ids is sorted and columns is aligned with it. Single-column lookups use
	// a short linear scan; bulk bindings can merge the sorted IDs directly.
	mask       componentMask
	ids        []componentID
	columns    []column
	entities   []Entity
	retention  reuse.Retention
	rowVersion uint64 // world structure version of the last removal
	id         uint32
	retiring   bool
	hasCleanup bool // conservative: may remain true after a hook is disabled
}

type archetypeEdges []archetypeEdge

type archetypeEdge struct {
	component componentID
	target    uint32
}

func (a *archetype) edge(id componentID) (uint32, bool) {
	if a.edges == nil {
		return 0, false
	}
	edges := *a.edges
	if len(edges) > 8 {
		// Consecutive component IDs can be addressed directly. Sparse IDs use
		// binary search so a large transition set never needs a linear scan.
		if offset := uint64(id - edges[0].component); offset < uint64(len(edges)) && edges[offset].component == id {
			return edges[offset].target, true
		}
		lo, hi := 0, len(edges)
		for lo < hi {
			mid := int(uint(lo+hi) >> 1)
			if edges[mid].component < id {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		if lo < len(edges) && edges[lo].component == id {
			return edges[lo].target, true
		}
		return 0, false
	}
	for _, edge := range edges {
		if edge.component == id {
			return edge.target, true
		}
		if edge.component > id {
			break
		}
	}
	return 0, false
}

func (a *archetype) has(id componentID) bool {
	return a.mask.has(id)
}

// column returns the storage for one component, or nil when this table does
// not own it. Callers rely on nil to detect a missing column.
func (a *archetype) column(id componentID) column {
	index := a.columnIndex(id)
	if index < 0 {
		return nil
	}
	return a.columns[index]
}

// columnIndex reports the position of one component in ids and columns, or -1
// when this table does not own it. Tables hold few components, so a sorted
// linear scan beats a binary search plus a mask test.
func (a *archetype) columnIndex(id componentID) int {
	for index, candidate := range a.ids {
		if candidate == id {
			return index
		}
		if candidate > id {
			break
		}
	}
	return -1
}

// appendEntity adds one row and returns its index.
func (a *archetype) appendEntity(entity Entity) int {
	row := len(a.entities)
	a.entities = append(a.entities, entity)
	if a.retiring {
		a.retention.Record(len(a.entities))
	}
	return row
}

// reserve grows table rows without reserving new world entity slots. Migration
// reuses existing slots; only spawners need to reserve both kinds of storage.
func (a *archetype) reserve(capacity int) {
	if capacity <= 0 {
		return
	}
	if capacity > cap(a.entities) {
		a.entities = slices.Grow(a.entities, capacity-len(a.entities))
	}
	additional := capacity - len(a.entities)
	for _, storage := range a.columns {
		storage.reserve(additional)
	}
}

// sortUniqueIDs sorts component IDs in place and reports the duplicate that
// broke the sort order, if any.
func sortUniqueIDs(ids []componentID) (componentID, bool) {
	slices.Sort(ids)
	for index := 1; index < len(ids); index++ {
		if ids[index-1] == ids[index] {
			return ids[index], false
		}
	}
	return 0, true
}

func addComponentID(ids []componentID, id componentID) []componentID {
	index, found := slices.BinarySearch(ids, id)
	if found {
		return slices.Clone(ids)
	}
	out := make([]componentID, len(ids)+1)
	copy(out, ids[:index])
	out[index] = id
	copy(out[index+1:], ids[index:])
	return out
}

func removeComponentID(ids []componentID, id componentID) []componentID {
	index, found := slices.BinarySearch(ids, id)
	if !found {
		return slices.Clone(ids)
	}
	out := make([]componentID, len(ids)-1)
	copy(out, ids[:index])
	copy(out[index:], ids[index+1:])
	return out
}
