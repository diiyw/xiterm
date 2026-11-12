package scene

import (
	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/internal/entitycache"
	"github.com/onix-engine/onix/internal/reuse"
)

type renderHierarchy2DInfo struct {
	layers   RenderLayers
	scene    SceneLayer
	group    uint64
	target   ecs.Entity
	stack    int
	hasStack bool
}

type renderEntity2DInfo struct {
	layers    RenderLayers
	scene     SceneLayer
	group     uint64
	target    ecs.Entity
	sortIndex int
}

type renderHierarchy2DCacheScratch struct{}

type renderHierarchy2DCache struct {
	hierarchy *Hierarchy
	sparse    map[ecs.Entity]renderHierarchy2DInfo
	values    entitycache.Store[renderHierarchy2DInfo]
	retention reuse.MapRetention
	highest   uint32
	versions  [6]uint64
	structure uint64
}

func (c *renderHierarchy2DCache) MaintainCache() {
	if c.sparse != nil {
		c.sparse = c.retention.Trim(c.sparse, 64)
	} else {
		c.values.Trim()
	}
}

// frameRenderHierarchy2DCachePtr retains inherited state until its inputs change.
func frameRenderHierarchy2DCachePtr(world *ecs.World) *renderHierarchy2DCache {
	cache := world.FrameValue[renderHierarchy2DCacheScratch, renderHierarchy2DCache]()
	cache.hierarchy = HierarchyFor(world)
	versions := [6]uint64{
		componentValueVersion[Parent](world), componentValueVersion[SceneLayer](world),
		componentValueVersion[RenderLayers](world), componentValueVersion[UITargetCamera](world),
		componentValueVersion[UIStackIndex](world), componentValueVersion[PainterGroup2D](world),
	}
	if cache.versions != versions || cache.structure != world.EntityVersion() {
		cache.resetValues()
		cache.versions, cache.structure = versions, world.EntityVersion()
	}
	return cache
}

func renderEntityInfo2D(world *ecs.World, entity ecs.Entity, ui bool, fallback int, cache *renderHierarchy2DCache) renderEntity2DInfo {
	resolved := cache.resolve(world, entity)
	if !ui {
		layers, _ := world.Get[RenderLayers](entity)
		if order, ok := world.Get[RenderOrder2D](entity); ok {
			fallback = int(order)
		}
		return renderEntity2DInfo{scene: resolved.scene, group: resolved.group, layers: layers, sortIndex: fallback}
	}
	if resolved.hasStack {
		fallback = resolved.stack
	}
	return renderEntity2DInfo{scene: resolved.scene, group: resolved.group, layers: resolved.layers, target: resolved.target, sortIndex: fallback}
}

func (c *renderHierarchy2DCache) resolve(world *ecs.World, entity ecs.Entity) renderHierarchy2DInfo {
	if entity == 0 {
		return renderHierarchy2DInfo{}
	}
	if c.sparse != nil {
		if info, ok := c.sparse[entity]; ok {
			return info
		}
	} else if info := c.values.Get(entity); info != nil {
		return *info
	}
	// A provisional default also terminates cycles, without a second map.
	c.setValue(entity, renderHierarchy2DInfo{})
	info := renderHierarchy2DInfo{layers: RenderLayers{}}
	if parent := c.hierarchy.Parent(entity); parent != 0 {
		info = c.resolve(world, parent)
	}
	if layers, ok := world.Get[RenderLayers](entity); ok {
		info.layers = layers
	}
	if layer, ok := world.Get[SceneLayer](entity); ok {
		info.scene = layer
	}
	if group, ok := world.Get[PainterGroup2D](entity); ok {
		info.group = uint64(group)
	}
	if target, ok := world.Get[UITargetCamera](entity); ok {
		info.target = target.Entity
	}
	if stack, ok := world.Get[UIStackIndex](entity); ok {
		info.stack, info.hasStack = int(stack), true
	}
	c.setValue(entity, info)
	return info
}

// Keep a compact slot index for dense populations. Sparse high IDs use a map
// so a few rendered entities never reserve an index for the entire World.
func (c *renderHierarchy2DCache) setValue(entity ecs.Entity, value renderHierarchy2DInfo) {
	index, _ := entity.Index()
	c.highest = max(c.highest, index)
	if c.sparse == nil && uint64(c.highest) >= uint64(max(128, (c.values.Len()+1)*4)) {
		c.sparse = make(map[ecs.Entity]renderHierarchy2DInfo, c.values.Len()+1)
		for entity, value := range c.values.All {
			c.sparse[entity] = *value
		}
		c.values = entitycache.Store[renderHierarchy2DInfo]{}
	}
	if c.sparse != nil {
		c.sparse[entity] = value
		if len(c.sparse) > 64 && uint64(c.highest) < uint64(len(c.sparse))*4 {
			for entity, value := range c.sparse {
				*c.values.Ensure(entity) = value
			}
			c.sparse, c.retention = nil, reuse.MapRetention{}
		}
	} else {
		*c.values.Ensure(entity) = value
	}
}

func (c *renderHierarchy2DCache) resetValues() {
	c.highest = 0
	if c.sparse != nil {
		c.retention.Record(len(c.sparse))
		clear(c.sparse)
	} else {
		c.values.Reset()
	}
}

func componentValueVersion[T any](world *ecs.World) uint64 {
	version, _ := world.ComponentValueRevision[T]()
	return version
}
