package scene

import (
	"cmp"
	"slices"

	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/graphics"
	"github.com/go-gl/mathgl/mgl32"
)

type Camera struct {
	Viewport   CameraViewport
	Order      int
	Active     bool
	Clear      bool
	ClearColor graphics.ColorRGBA
	// Fallback is reserved for the engine camera used without an authored 2D camera.
	Fallback bool

	viewportWidth  float32
	viewportHeight float32
}

type CameraViewport struct {
	X      float32 `json:"x"`
	Y      float32 `json:"y"`
	Width  float32 `json:"width"`
	Height float32 `json:"height"`
}

type Camera2D struct{}

type UITargetCamera struct {
	Entity ecs.Entity
}

func NewCamera() Camera {
	return Camera{
		Active:     true,
		Clear:      true,
		ClearColor: graphics.ColorRGBA{R: 0.025, G: 0.03, B: 0.04, A: 1},
	}
}

func (c *Camera) UpdateViewport(width, height int) {
	if c == nil {
		return
	}
	c.viewportWidth = float32(width)
	c.viewportHeight = float32(height)
}

func (c Camera) ViewportSize() (float32, float32) {
	viewport := c.ResolvedViewport()
	return viewport.Width, viewport.Height
}

func (c Camera) ResolvedViewport() CameraViewport {
	viewport := c.Viewport
	if viewport.Width <= 0 || viewport.Height <= 0 {
		viewport = CameraViewport{Width: c.viewportWidth, Height: c.viewportHeight}
	}
	viewport.X = min(max(viewport.X, 0), max(0, c.viewportWidth-1))
	viewport.Y = min(max(viewport.Y, 0), max(0, c.viewportHeight-1))
	viewport.Width = min(max(viewport.Width, 1), c.viewportWidth-viewport.X)
	viewport.Height = min(max(viewport.Height, 1), c.viewportHeight-viewport.Y)
	return viewport
}

func (c Camera) ClipFromWorld(transform GlobalTransform, projection Projection) mgl32.Mat4 {
	viewport := c.ResolvedViewport()
	if viewport.Width <= 0 || viewport.Height <= 0 {
		return mgl32.Ident4()
	}
	return projection.Matrix(viewport.Width, viewport.Height).Mul4(transform.Matrix.Inv())
}

func (c Camera) ViewportToWorld2D(transform GlobalTransform, projection Projection, viewportPosition mgl32.Vec2) (mgl32.Vec2, bool) {
	viewport := c.ResolvedViewport()
	if viewport.Width <= 0 || viewport.Height <= 0 || projection.Kind != ProjectionOrthographic {
		return mgl32.Vec2{}, false
	}
	orthographic := projection.Orthographic
	orthographic.Update(viewport.Width, viewport.Height)
	area := orthographic.Area
	relative := viewportPosition.Sub(mgl32.Vec2{viewport.X, viewport.Y})
	view := mgl32.Vec4{
		area.MinX + relative.X()/viewport.Width*(area.MaxX-area.MinX),
		area.MaxY - relative.Y()/viewport.Height*(area.MaxY-area.MinY),
		0,
		1,
	}
	world := transform.Matrix.Mul4x1(view)
	return mgl32.Vec2{world.X(), world.Y()}, true
}

func (c Camera) WorldToViewport2D(transform GlobalTransform, projection Projection, worldPosition mgl32.Vec2) (mgl32.Vec2, bool) {
	viewport := c.ResolvedViewport()
	if viewport.Width <= 0 || viewport.Height <= 0 || projection.Kind != ProjectionOrthographic {
		return mgl32.Vec2{}, false
	}
	orthographic := projection.Orthographic
	orthographic.Update(viewport.Width, viewport.Height)
	view := transform.Matrix.Inv().Mul4x1(mgl32.Vec4{worldPosition.X(), worldPosition.Y(), 0, 1})
	area := orthographic.Area
	width, height := area.MaxX-area.MinX, area.MaxY-area.MinY
	if width == 0 || height == 0 {
		return mgl32.Vec2{}, false
	}
	return mgl32.Vec2{
		(view.X()-area.MinX)/width*viewport.Width + viewport.X,
		(area.MaxY-view.Y())/height*viewport.Height + viewport.Y,
	}, true
}

type RenderLayers struct {
	blocks []uint64
	empty  bool
}

func RenderLayer(layer uint) RenderLayers {
	blocks := make([]uint64, layer/64+1)
	blocks[layer/64] = uint64(1) << (layer % 64)
	return RenderLayers{blocks: blocks}
}

func RenderLayersNone() RenderLayers {
	return RenderLayers{empty: true}
}

func (layers RenderLayers) With(layer uint) RenderLayers {
	blocks := layers.normalizedBlocks()
	needed := int(layer/64 + 1)
	if len(blocks) < needed {
		blocks = append(blocks, make([]uint64, needed-len(blocks))...)
	}
	blocks[layer/64] |= uint64(1) << (layer % 64)
	return RenderLayers{blocks: blocks}
}

func (layers RenderLayers) Without(layer uint) RenderLayers {
	blocks := layers.normalizedBlocks()
	index := int(layer / 64)
	if index < len(blocks) {
		blocks[index] &^= uint64(1) << (layer % 64)
	}
	for len(blocks) > 0 && blocks[len(blocks)-1] == 0 {
		blocks = blocks[:len(blocks)-1]
	}
	if len(blocks) == 0 {
		return RenderLayersNone()
	}
	return RenderLayers{blocks: blocks}
}

func (layers RenderLayers) Contains(layer uint) bool {
	if layers.empty {
		return false
	}
	if len(layers.blocks) == 0 {
		return layer == 0
	}
	index := int(layer / 64)
	return index < len(layers.blocks) && layers.blocks[index]&(uint64(1)<<(layer%64)) != 0
}

func (layers RenderLayers) Intersects(other RenderLayers) bool {
	if layers.empty || other.empty {
		return false
	}
	if len(layers.blocks) == 0 {
		return other.Contains(0)
	}
	if len(other.blocks) == 0 {
		return layers.Contains(0)
	}
	for index := 0; index < min(len(layers.blocks), len(other.blocks)); index++ {
		if layers.blocks[index]&other.blocks[index] != 0 {
			return true
		}
	}
	return false
}

func (layers RenderLayers) normalizedBlocks() []uint64 {
	if layers.empty {
		return nil
	}
	if len(layers.blocks) == 0 {
		return []uint64{1}
	}
	return slices.Clone(layers.blocks)
}

// LayersForEntity returns authored render layers or the default layer zero.
func LayersForEntity(world *ecs.World, entity ecs.Entity) RenderLayers {
	if layers, ok := world.Get[RenderLayers](entity); ok {
		return layers
	}
	return RenderLayers{}
}

func SortedCameraEntities(world *ecs.World) []ecs.Entity {
	requireWorld(world)
	entities := make([]ecs.Entity, 0, 4)
	var fallback ecs.Entity
	has2D := false
	for entity, access := range world.Query[Camera]().All {
		if !IsActive(world, entity) {
			continue
		}
		camera := access.Get()

		if camera.Active {
			if camera.Fallback {
				fallback = entity
				continue
			}
			has2D = has2D || world.Has[Camera2D](entity)
			entities = append(entities, entity)
		}
	}
	if !has2D && fallback != 0 {
		entities = append(entities, fallback)
	}
	sortCameraEntities(world, entities)
	return entities
}

func sortCameraEntities(world *ecs.World, entities []ecs.Entity) {
	for index := 1; index < len(entities); index++ {
		entity := entities[index]
		position := index
		for position > 0 && compareCameraEntities(world, entity, entities[position-1]) < 0 {
			entities[position] = entities[position-1]
			position--
		}
		entities[position] = entity
	}
}

func compareCameraEntities(world *ecs.World, a, b ecs.Entity) int {
	cameraA, _ := world.Get[Camera](a)
	cameraB, _ := world.Get[Camera](b)
	if order := cmp.Compare(cameraA.Order, cameraB.Order); order != 0 {
		return order
	}
	return cmp.Compare(a, b)
}

func PrimaryCamera2D(world *ecs.World) (ecs.Entity, bool) {
	requireWorld(world)
	var primary ecs.Entity
	var fallback ecs.Entity
	primaryOrder := 0
	for entity, access := range world.Query[Camera]().All {
		if !IsActive(world, entity) {
			continue
		}
		camera := access.Get()

		if !camera.Active || !world.Has[Camera2D](entity) {
			continue
		}
		if camera.Fallback {
			fallback = entity
			continue
		}
		if primary == 0 || camera.Order < primaryOrder || camera.Order == primaryOrder && entity < primary {
			primary, primaryOrder = entity, camera.Order
		}
	}
	if primary == 0 {
		primary = fallback
	}
	return primary, primary != 0
}

func primaryCamera2DComponents(world *ecs.World) (Camera, Projection, GlobalTransform, bool) {
	entity, ok := PrimaryCamera2D(world)
	if !ok {
		return Camera{}, Projection{}, GlobalTransform{}, false
	}
	camera, cameraOK := world.Get[Camera](entity)
	projection, projectionOK := world.Get[Projection](entity)
	transform, transformOK := world.Get[GlobalTransform](entity)
	return camera, projection, transform, cameraOK && projectionOK && transformOK
}

func PrimaryCamera2DView(world *ecs.World) (Camera, Projection, GlobalTransform, bool) {
	return primaryCamera2DComponents(world)
}

func ViewportToWorld2D(world *ecs.World, viewportPosition mgl32.Vec2) (mgl32.Vec2, bool) {
	camera, projection, transform, ok := primaryCamera2DComponents(world)
	if !ok {
		return mgl32.Vec2{}, false
	}
	return camera.ViewportToWorld2D(transform, projection, viewportPosition)
}

func WorldToViewport2D(world *ecs.World, worldPosition mgl32.Vec2) (mgl32.Vec2, bool) {
	camera, projection, transform, ok := primaryCamera2DComponents(world)
	if !ok {
		return mgl32.Vec2{}, false
	}
	return camera.WorldToViewport2D(transform, projection, worldPosition)
}

func SpawnCamera2D(world *ecs.World) ecs.Entity {
	spawner := world.NewSpawner5[Camera, Camera2D, Projection, Transform, GlobalTransform]()
	return spawner.Spawn(NewCamera(), Camera2D{}, OrthographicProjection2D(), NewTransform(), GlobalTransform{})
}
