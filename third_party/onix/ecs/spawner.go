package ecs

import "fmt"

type spawnPlan struct {
	world     *World
	archetype *archetype
}

func (p *spawnPlan) reserve(capacity int) {
	if p == nil || p.world == nil || p.archetype == nil || capacity <= 0 {
		return
	}
	p.world.assertStructuralMutationAllowed()
	p.world.ReserveEntities(capacity)
	p.archetype.reserve(capacity)
}

func newSpawnPlan(world *World, ids []componentID) spawnPlan {
	if world == nil {
		panic("ecs: world is nil")
	}
	world.assertStructuralMutationAllowed()
	if duplicate, ok := sortUniqueIDs(ids); !ok {
		panic(fmt.Sprintf("ecs: duplicate component %s in spawner", world.componentInfos[duplicate].typ))
	}
	return spawnPlan{world: world, archetype: world.archetypeFor(ids)}
}

func (p *spawnPlan) begin() (Entity, int, Tick) {
	if p == nil || p.world == nil || p.archetype == nil {
		panic("ecs: spawner is not initialized")
	}
	p.world.assertStructuralMutationAllowed()
	entity := p.world.reserveEntity()
	return entity, p.archetype.appendEntity(entity), p.world.mutationTick()
}

func (p *spawnPlan) finish(entity Entity, row int) {
	index, _ := entity.Index()
	p.world.commitSpawn(index, p.archetype, row)
}
