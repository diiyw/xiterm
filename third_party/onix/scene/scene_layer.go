package scene

import "github.com/onix-engine/onix/ecs"

// SceneLayer separates camera content and orders complete scenes before local
// world/UI order. Descendants inherit the scene root's layer.
type SceneLayer struct {
	ID, Order uint64
	Overlay   bool
}

func LayerForEntity(world *ecs.World, entity ecs.Entity) SceneLayer {
	for remaining := world.EntityCount() + 1; entity != 0 && remaining > 0; remaining-- {
		if layer, ok := world.Get[SceneLayer](entity); ok {
			return layer
		}
		parent, ok := world.Get[Parent](entity)
		if !ok {
			break
		}
		entity = parent.Entity
	}
	return SceneLayer{}
}

func sceneCameraVisible(world *ecs.World, entity ecs.Entity) bool {
	for remaining := world.EntityCount() + 1; entity != 0 && remaining > 0; remaining-- {
		if v, ok := world.Get[Visibility](entity); ok && !v.Visible {
			return false
		}
		// Scene roots pause simulation while retained backgrounds stay visible.
		// Module-local activity still disables that module's authored cameras.
		if a, ok := world.Get[Activity](entity); ok && !a.Active && !world.Has[SceneLayer](entity) {
			return false
		}
		parent, ok := world.Get[Parent](entity)
		if !ok {
			return true
		}
		entity = parent.Entity
	}
	return entity == 0
}
