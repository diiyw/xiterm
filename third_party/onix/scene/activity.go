package scene

import "github.com/onix-engine/onix/ecs"

// Activity controls simulation independently of visibility. An entity may be
// invisible while running; suspended scene content must do neither.
type Activity struct{ Active bool }

func IsActive(world *ecs.World, entity ecs.Entity) bool {
	return HierarchyFor(world).flags(entity)&hierarchyActive != 0
}

// IsInteractive requires both activity and visibility along the parent chain.
// Hidden UI cannot retain keyboard focus or receive semantic form input.
func IsInteractive(world *ecs.World, entity ecs.Entity) bool {
	return HierarchyFor(world).flags(entity) == hierarchyEnabled
}

// IsVisible allows paused overlay backgrounds to render without enabling
// their simulation or input. Query scopes determine scene participation.
func IsVisible(world *ecs.World, entity ecs.Entity) bool {
	return HierarchyFor(world).flags(entity)&hierarchyVisible != 0
}
