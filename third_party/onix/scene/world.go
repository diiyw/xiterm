package scene

import "github.com/onix-engine/onix/ecs"

func requireWorld(world *ecs.World) {
	if world == nil {
		panic("scene: world is nil")
	}
}
