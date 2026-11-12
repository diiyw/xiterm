package ecs

import "github.com/onix-engine/onix/internal/reuse"

// Maintain releases caches after sustained underuse. Call once at the end of a
// frame, after all borrowed component accesses and drawing snapshots are done.
// Table and column identities survive; borrowed rows expire if storage moves.
func (w *World) Maintain() {
	w.assertStructuralMutationAllowed()
	w.maintenance++
	for _, value := range w.frameScratch {
		if cache, ok := value.(interface{ MaintainCache() }); ok {
			cache.MaintainCache()
		}
	}
	for link := &w.scopes; *link != nil; {
		s := *link
		if w.maintenance-s.used >= reuse.RetentionFrames {
			*link = s.next
			s.dropCache()
		} else {
			s.maintain()
			link = &s.next
		}
	}
	kept := w.storageCandidates[:0]
	for _, a := range w.storageCandidates {
		length := len(a.entities)
		minimum := 64
		if length == 0 {
			minimum = 0
		}
		target := a.retention.TrimCapacity(cap(a.entities), length, minimum)
		if target != 0 {
			if target < 0 {
				target = 0
			}
			if target == 0 {
				a.entities = nil
			} else {
				values := make([]Entity, length, target)
				copy(values, a.entities)
				a.entities = values
			}
			for _, c := range a.columns {
				c.compact(target)
			}
			w.structureVersion++
		}
		a.retiring = cap(a.entities) > minimum && length <= cap(a.entities)/4
		if a.retiring {
			kept = append(kept, a)
		}
	}
	clear(w.storageCandidates[len(kept):])
	w.storageCandidates = kept
}

func (c *typedColumn[T]) compact(capacity int) {
	if capacity == 0 {
		c.values = nil
		c.added = nil
		c.changed = nil
		c.addedRanges = nil
		c.changedRanges = nil
		return
	}
	values := make([]T, len(c.values), capacity)
	copy(values, c.values)
	c.values = values
	added, changed := make([]Tick, len(c.added), capacity), make([]Tick, len(c.changed), capacity)
	copy(added, c.added)
	copy(changed, c.changed)
	c.added, c.changed = added, changed
	c.addedRanges = make([]tickRange, tickRangeCount(capacity))
	c.changedRanges = make([]tickRange, tickRangeCount(capacity))
	for row := range c.values {
		includeTick(c.addedRanges, row, c.added[row])
		includeTick(c.changedRanges, row, c.changed[row])
	}
}
