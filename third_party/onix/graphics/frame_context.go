package graphics

import (
	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/render"
)

// FrameContext belongs to an ECS world/window. Engine systems obtain their
// canvas, device and upload queue here; immediate-mode globals do not select
// the destination of an ECS frame. BindFrame and uploads run on the render thread.
type FrameContext struct {
	Canvas  *Canvas2D
	Uploads UploadQueue
}

func (frame *FrameContext) Device() *render.Device {
	if frame == nil || frame.Canvas == nil {
		return nil
	}
	return frame.Canvas.Device()
}
func FrameFor(world *ecs.World) *FrameContext {
	if world == nil {
		return nil
	}
	frame, _ := world.GetResource[*FrameContext]()
	return frame
}
func CanvasFor(world *ecs.World) *Canvas2D {
	if frame := FrameFor(world); frame != nil {
		return frame.Canvas
	}
	return nil
}
func DeviceFor(world *ecs.World) *render.Device { return FrameFor(world).Device() }

// BindFrame installs or updates the world's explicit rendering destination.
// Rebinding the same canvas does not allocate or invalidate ECS resources.
func BindFrame(world *ecs.World, canvas *Canvas2D) *FrameContext {
	frame := FrameFor(world)
	if frame == nil {
		frame = &FrameContext{}
		world.InsertResource(frame)
	}
	frame.Canvas = canvas
	return frame
}
func (frame *FrameContext) AdvanceUploads(budget UploadBudget) (bool, error) {
	return frame.Uploads.Advance(frame.Device(), budget)
}
