package scene

import (
	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/internal/reuse"
)

const (
	hierarchyActive uint8 = 1 << iota
	hierarchyVisible
	hierarchyEnabled = hierarchyActive | hierarchyVisible
)

const (
	hierarchyResolved uint8 = 1 << iota
	hierarchyVisiting
	hierarchyQueued
	hierarchyDepthResolved
	hierarchyDepthVisiting
)

type hierarchyNode struct {
	entity, parent, firstChild, nextSibling, previousSibling ecs.Entity
	depth                                                    uint32
	local, inherited, state                                  uint8
}

// Hierarchy is the world-owned parent/child index shared by scene systems.
// It includes suspended scenes and never retains component storage pointers.
type Hierarchy struct {
	changes                 ecs.SystemContext
	entities, queue, path   []ecs.Entity
	nodes                   []hierarchyNode
	retention               [4]reuse.Retention
	versions, members       [3]uint64
	version                 uint64
	mutationVersion         uint64
	entityVersion           uint64
	invalidated, recomputed int
	lastRun                 ecs.Tick
	initialized             bool
}

// HierarchyFor synchronizes topology and invalidates inherited state only in
// affected subtrees. Borrowed lists remain valid until the next synchronization.
func HierarchyFor(world *ecs.World) *Hierarchy {
	h := world.FrameValue[Hierarchy, Hierarchy]()
	h.sync(world)
	return h
}

// Version changes for live entity membership or an actual parent change.
func (h *Hierarchy) Version() uint64 { return h.version }

// Entities returns the borrowed live entity list, independent of query scopes.
func (h *Hierarchy) Entities() []ecs.Entity { return h.entities }

func (h *Hierarchy) Parent(entity ecs.Entity) ecs.Entity {
	if n := h.node(entity); n != nil {
		return n.parent
	}
	return 0
}

func (h *Hierarchy) FirstChild(entity ecs.Entity) ecs.Entity {
	if n := h.node(entity); n != nil {
		return n.firstChild
	}
	return 0
}

func (h *Hierarchy) NextSibling(entity ecs.Entity) ecs.Entity {
	if n := h.node(entity); n != nil {
		return n.nextSibling
	}
	return 0
}

// Depth returns the number of live ancestors, including ancestors outside the
// current query scope. Missing entities and cycles return -1.
func (h *Hierarchy) Depth(entity ecs.Entity) int {
	h.path = h.path[:0]
	depth, cyclic := -1, false
	for n := h.node(entity); n != nil; n = h.node(n.parent) {
		if n.state&hierarchyDepthResolved != 0 {
			cyclic = n.depth == ^uint32(0)
			depth = int(n.depth)
			break
		}
		if n.state&hierarchyDepthVisiting != 0 {
			cyclic = true
			break
		}
		n.state |= hierarchyDepthVisiting
		h.path = append(h.path, n.entity)
	}
	for i := len(h.path) - 1; i >= 0; i-- {
		n := h.node(h.path[i])
		if cyclic {
			n.depth = ^uint32(0)
		} else {
			depth++
			n.depth = uint32(depth)
		}
		n.state = n.state&^hierarchyDepthVisiting | hierarchyDepthResolved
	}
	if cyclic {
		return -1
	}
	return depth
}

func (h *Hierarchy) node(entity ecs.Entity) *hierarchyNode {
	index, ok := entity.Index()
	if !ok || int(index) >= len(h.nodes) || h.nodes[index].entity != entity {
		return nil
	}
	return &h.nodes[index]
}

func (h *Hierarchy) sync(world *ecs.World) {
	mutation := world.MutationVersion()
	if h.initialized && h.mutationVersion == mutation {
		return
	}
	var versions, members [3]uint64
	versions[0], members[0] = world.ComponentValueRevision[Parent]()
	versions[1], members[1] = world.ComponentValueRevision[Activity]()
	versions[2], members[2] = world.ComponentValueRevision[Visibility]()
	h.invalidated, h.recomputed = 0, 0
	if !h.initialized || h.entityVersion != world.EntityVersion() {
		h.rebuild(world)
	} else {
		since := h.lastRun
		if since == world.ChangeTick() {
			since--
		}
		h.changes = world.ChangesSince(since)
		changes := &h.changes
		if h.versions[0] != versions[0] {
			if h.members[0] != members[0] {
				for _, entity := range h.entities {
					h.refreshParent(world, entity)
				}
			} else {
				for entity := range world.Query[Parent](ecs.Unscoped(), changes.Changed[Parent]()).All {
					h.refreshParent(world, entity)
				}
			}
		}
		h.refreshFlags[Activity](world, changes, versions, members, 1)
		h.refreshFlags[Visibility](world, changes, versions, members, 2)
		for i := 0; i < len(h.queue); i++ {
			n := h.node(h.queue[i])
			n.state &^= hierarchyResolved | hierarchyDepthResolved
			for child := n.firstChild; child != 0; child = h.NextSibling(child) {
				h.mark(child)
			}
		}
		h.invalidated = len(h.queue)
		for _, entity := range h.queue {
			h.node(entity).state &^= hierarchyQueued
		}
		h.queue = h.queue[:0]
	}
	h.versions, h.members = versions, members
	h.entityVersion, h.mutationVersion = world.EntityVersion(), mutation
	h.lastRun, h.initialized = world.ChangeTick(), true
}

func (h *Hierarchy) rebuild(world *ecs.World) {
	for _, entity := range h.entities {
		index, _ := entity.Index()
		h.nodes[index] = hierarchyNode{}
	}

	h.entities = h.entities[:0]
	for entity := range world.Entities() {
		index, _ := entity.Index()
		if int(index) >= len(h.nodes) {
			h.nodes = append(h.nodes, make([]hierarchyNode, int(index)+1-len(h.nodes))...)
		}
		parent, _ := world.Get[Parent](entity)
		h.nodes[index] = hierarchyNode{entity: entity, parent: parent.Entity, local: hierarchyLocalFlags(world, entity)}
		h.entities = append(h.entities, entity)
	}
	for _, entity := range h.entities {
		h.link(h.node(entity))
	}
	h.version++
	h.invalidated = len(h.entities)
}

func (h *Hierarchy) link(n *hierarchyNode) {
	if parent := h.node(n.parent); parent != nil {
		n.nextSibling = parent.firstChild
		if next := h.node(n.nextSibling); next != nil {
			next.previousSibling = n.entity
		}
		parent.firstChild = n.entity
	}
}

func (h *Hierarchy) refreshParent(world *ecs.World, entity ecs.Entity) {
	n := h.node(entity)
	parent, _ := world.Get[Parent](entity)
	if n.parent == parent.Entity {
		return
	}
	if previous := h.node(n.previousSibling); previous != nil {
		previous.nextSibling = n.nextSibling
	} else if old := h.node(n.parent); old != nil {
		old.firstChild = n.nextSibling
	}
	if next := h.node(n.nextSibling); next != nil {
		next.previousSibling = n.previousSibling
	}
	n.parent, n.previousSibling, n.nextSibling = parent.Entity, 0, 0
	h.link(n)
	h.mark(entity)
	h.version++
}

func hierarchyLocalFlags(world *ecs.World, entity ecs.Entity) uint8 {
	flags := uint8(hierarchyEnabled)
	if value := world.GetRef[Activity](entity); value != nil && !value.Active {
		flags &^= hierarchyActive
	}
	if value := world.GetRef[Visibility](entity); value != nil && !value.Visible {
		flags &^= hierarchyVisible
	}
	return flags
}

func (h *Hierarchy) refreshFlags[T any](world *ecs.World, changes *ecs.SystemContext, versions, members [3]uint64, index int) {
	if h.versions[index] == versions[index] {
		return
	}
	refresh := func(entity ecs.Entity) {
		n := h.node(entity)
		local := hierarchyLocalFlags(world, entity)
		if n.local != local {
			n.local = local
			h.mark(entity)
		}
	}
	if h.members[index] != members[index] {
		for _, entity := range h.entities {
			refresh(entity)
		}
	} else {
		for entity := range world.Query[T](ecs.Unscoped(), changes.Changed[T]()).All {
			refresh(entity)
		}
	}
}

func (h *Hierarchy) mark(entity ecs.Entity) {
	if n := h.node(entity); n != nil && n.state&hierarchyQueued == 0 {
		n.state |= hierarchyQueued
		h.queue = append(h.queue, entity)
	}
}

func (h *Hierarchy) flags(entity ecs.Entity) uint8 {
	h.path = h.path[:0]
	flags := uint8(hierarchyEnabled)
	for n := h.node(entity); n != nil; n = h.node(n.parent) {
		if n.state&hierarchyResolved != 0 {
			flags = n.inherited
			break
		}
		if n.state&hierarchyVisiting != 0 {
			flags = 0
			break
		}
		n.state |= hierarchyVisiting
		h.path = append(h.path, n.entity)
	}
	for i := len(h.path) - 1; i >= 0; i-- {
		n := h.node(h.path[i])
		flags &= n.local
		n.inherited = flags
		n.state = n.state&^hierarchyVisiting | hierarchyResolved
		h.recomputed++
	}
	return flags
}

func (h *Hierarchy) MaintainCache() {
	h.entities = h.retention[0].Trim(h.entities, 64)
	h.queue = h.retention[1].Trim(h.queue[:0], 64)
	h.path = h.retention[2].Trim(h.path[:0], 64)
	length := len(h.nodes)
	for length > 0 && h.nodes[length-1].entity == 0 {
		length--
	}
	h.nodes = h.retention[3].Trim(h.nodes[:length], 64)
}
