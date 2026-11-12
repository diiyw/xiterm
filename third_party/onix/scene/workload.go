package scene

import "github.com/onix-engine/onix/ecs"

type taskCountCache struct {
	versions [5]uint64
	scope    uint64
	count    int
	valid    bool
}

// ActiveTaskCount counts scoped, active entities satisfying ready. Tag identifies
// the predicate; dependency changes invalidate inputs it reads outside T.
// Unchanged inputs reuse the count without scanning entities or allocating.
func ActiveTaskCount[Tag, T any](world *ecs.World, dependency uint64, ready func(*ecs.World, ecs.Entity, *T) bool) int {
	cache := world.ScopeValue[Tag, taskCountCache]()
	_, scope := world.QueryScopeKey()
	versions := [5]uint64{componentValueVersion[T](world), componentValueVersion[Activity](world), componentValueVersion[Parent](world), world.EntityVersion(), dependency}
	if cache.valid && cache.scope == scope && cache.versions == versions {
		return cache.count
	}
	count := 0
	for entity, access := range world.Query[T]().All {
		if IsActive(world, entity) && ready(world, entity, access.Ref()) {
			count++
		}
	}
	cache.versions, cache.scope, cache.count, cache.valid = versions, scope, count, true
	return count
}

type animationTasks struct{}
type particleTasks struct{}
type trailTasks struct{}
type spineTasks struct{}

func AnimationTaskCount(world *ecs.World) int {
	return ActiveTaskCount[animationTasks, Animator](world, 0, func(_ *ecs.World, _ ecs.Entity, a *Animator) bool {
		return a.Clip != nil && a.Playing && a.Speed > 0 && isFiniteAnimationValue(a.Speed) && a.Clip.Duration > 0
	})
}

func ParticleTaskCount(world *ecs.World) int {
	return ActiveTaskCount[particleTasks, ParticleEmitter2D](world, 0, func(_ *ecs.World, _ ecs.Entity, p *ParticleEmitter2D) bool {
		return p.Active()
	})
}

func SpriteTrailTaskCount(world *ecs.World) int {
	return ActiveTaskCount[trailTasks, SpriteTrail2D](world, componentValueVersion[Sprite2D](world), func(w *ecs.World, e ecs.Entity, t *SpriteTrail2D) bool {
		// Existing samples must finish fading. A trail with zero distance also
		// keeps emitting while stationary; other trails wake on tracked movement.
		return t.SampleCount() > 0 || t.MinDistance <= 0 && w.Has[Sprite2D](e)
	})
}

func SpineTaskCount(world *ecs.World) int {
	return ActiveTaskCount[spineTasks, Spine2D](world, 0, func(_ *ecs.World, _ ecs.Entity, s *Spine2D) bool {
		return s.Player != nil && s.Player.Resource != nil && s.Player.AnimationDuration() > 0 && !s.Player.AnimationComplete()
	})
}
