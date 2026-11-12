package scene

import (
	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/spine"
	"github.com/go-gl/mathgl/mgl32"
)

type Spine2D struct {
	Player *spine.Player
}

func SpineModelMatrix2D(transform mgl32.Mat4, ui bool) mgl32.Mat4 {
	if ui {
		return transform.Mul4(mgl32.Scale3D(1, -1, 1))
	}
	return transform
}

func UpdateSpine2DSystem(name string) ecs.SystemConfig {
	if name == "" {
		name = "update_spine_2d"
	}
	return ecs.SystemConfig{Name: name, System: updateSpine2D}
}

func updateSpine2D(context *ecs.SystemContext) error {
	delta := float32(0)
	if clock, ok := context.World.GetResource[*Time](); ok && clock != nil {
		delta = clock.DeltaSeconds()
	}
	if delta <= 0 {
		return nil
	}
	for entity, access := range context.World.Query[Spine2D]().All {
		if !IsActive(context.World, entity) {
			continue
		}
		component := access.Mut()
		if component != nil && component.Player != nil {
			component.Player.Update(delta)
		}
	}
	return nil
}
