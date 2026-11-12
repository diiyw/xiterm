package scene

import (
	"cmp"
	"reflect"

	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/graphics"
	"github.com/onix-engine/onix/internal/reuse"
	"github.com/onix-engine/onix/internal/smallsort"
	"github.com/onix-engine/onix/render"
	"github.com/go-gl/mathgl/mgl32"
)

type CameraPipeline uint8

type sceneCameraScratch struct{}

const (
	CameraPipeline2D CameraPipeline = iota
	CameraPipeline3D
	cameraPipelineCount
)

type ExtractedCamera struct {
	Target       *render.RenderTarget
	RenderLayers RenderLayers
	Camera       Camera
	Scene        SceneLayer
	Entity       ecs.Entity
	Projection   Projection
	Transform    GlobalTransform
	OutputSize   mgl32.Vec2
	DepthClear   float32
	Pipeline     CameraPipeline
	ViewMode     render.RenderViewMode
}

// CameraRenderOptions contains the output settings shared by camera pipelines.
type CameraRenderOptions struct {
	Target     *render.RenderTarget
	ViewMode   render.RenderViewMode
	DepthClear float32
}

// CameraRenderer submits the prepared render data for one extracted camera.
type CameraRenderer func(*graphics.Canvas2D, *ecs.World, ExtractedCamera) error

type cameraSettingsReader struct {
	typ      reflect.Type
	read     func(*ecs.World, ecs.Entity) (CameraRenderOptions, bool)
	revision func(*ecs.World) uint64
}

type RenderWorld struct {
	cameraSettings        [cameraPipelineCount]cameraSettingsReader
	collectors2D          [renderCollector2DCount]renderCollector2D
	drawers2D             [renderItemKind2DCount]renderItemDrawer2D
	batchDrawers2D        [renderItemKind2DCount]renderBatchDrawer2D
	cameraRenderers       [cameraPipelineCount]CameraRenderer
	whiteTexture2D        *graphics.Texture
	camera2DLayer         registeredCameraRenderer
	registeredMaterials2D map[reflect.Type]struct{}
	compositors2D         map[compositeViewKey2D]*compositor2D
	materials2D           []material2DReader
	Cameras               []ExtractedCamera
	Items2D               []renderItem2D
	activeCamera2D        ExtractedCamera
	Composite2D           CompositeStatistics2D
	Preparation2D         RenderPreparation2D
	registry2D            uint64
	compositeFrame2D      uint64
	activeCamera2DSet     bool
	compositeAllowed2D    bool
}

type compositeViewKey2D struct {
	Camera ecs.Entity
	Scene  uint64
}

// Dispose releases retained GPU output while the window's device is alive.
func (state *RenderWorld) Dispose() error {
	for key, view := range state.compositors2D {
		if err := view.dispose(); err != nil {
			return err
		}
		delete(state.compositors2D, key)
	}
	return nil
}

type visibleRenderItems2DScratch struct{}

func NewRenderWorld() *RenderWorld {
	return &RenderWorld{registeredMaterials2D: make(map[reflect.Type]struct{})}
}

type material2DReader struct {
	read      func(*ecs.World, ecs.Entity) (graphics.Material2DData, bool)
	structure func(*ecs.World) uint64
	poll      func(*ecs.World, *retainedRender2D)
}

// RegisterMaterial2D enables extraction of MeshMaterial2D[T] components. It
// mirrors Bevy's Material2dPlugin<T> registration boundary.
func (state *RenderWorld) RegisterMaterial2D[T graphics.Material2D]() {
	typ := reflect.TypeFor[T]()
	if _, ok := state.registeredMaterials2D[typ]; ok {
		return
	}
	state.registeredMaterials2D[typ] = struct{}{}
	state.registry2D++
	state.materials2D = append(state.materials2D, material2DReader{read: func(world *ecs.World, entity ecs.Entity) (graphics.Material2DData, bool) {
		component, ok := world.Get[MeshMaterial2D[T]](entity)
		if !ok || component.Material == nil {
			return graphics.Material2DData{}, false
		}
		return (*component.Material).Material2D(), true
	}, structure: (*ecs.World).ComponentStructureVersion[MeshMaterial2D[T]], poll: func(world *ecs.World, cache *retainedRender2D) {
		// Material pointers may be edited without replacing their ECS component.
		// Inspect only entities using this type, preserving the static scene cache.
		for entity := range world.Query[MeshMaterial2D[T]]().All {
			if node := cache.nodes[entity]; node != nil && !node.queued {
				material := material2DForEntity(world, entity)
				for position := range node.positions {
					if cache.items[position].materialData() != material {
						cache.mark(entity)
						break
					}
				}
			}
		}
	}})
}

func material2DForEntity(world *ecs.World, entity ecs.Entity) graphics.Material2DData {
	var result graphics.Material2DData
	readers := RenderWorldFor(world).materials2D
	for index := len(readers) - 1; index >= 0; index-- {
		if material, ok := readers[index].read(world, entity); ok {
			result = material
			break
		}
	}
	if shader, ok := world.Get[Shader[graphics.Material2DShader]](entity); ok {
		result.Shader, result.Custom = shader.Program, shader.Custom
	}
	return result
}

// RenderWorldFor returns the world's render state, creating it on first use.
func RenderWorldFor(world *ecs.World) *RenderWorld {
	state, ok := world.GetResource[*RenderWorld]()
	if ok && state != nil {
		return state
	}
	state = NewRenderWorld()
	world.InsertResource(state)
	return state
}

func particleFallbackTexture2D(world *ecs.World) *graphics.Texture {
	state := RenderWorldFor(world)
	if state.whiteTexture2D == nil || !state.whiteTexture2D.ValidFor(graphics.DeviceFor(world)) {
		state.whiteTexture2D = graphics.CreateTextureByColor(graphics.White)
	}
	return state.whiteTexture2D
}

func ExtractCamerasSystem(name string) ecs.SystemConfig {
	if name == "" {
		name = "extract_cameras"
	}
	return ecs.SystemConfig{Name: name, System: ExtractCameras}
}

// ExtractCameras refreshes snapshots of active registered cameras.
// Pipelines can call it directly without constructing a SystemConfig.
func ExtractCameras(context *ecs.SystemContext) error {
	world := context.World
	state := RenderWorldFor(world)
	state.Cameras = state.Cameras[:0]
	c := graphics.CanvasFor(world)
	if c == nil {
		return nil
	}
	canvasWidth, canvasHeight := c.Size()
	var fallback ExtractedCamera
	has2D := false
	sceneCameras := world.FrameScratchMap[sceneCameraScratch, uint64, bool]()
	for entity, access := range world.Query[Camera]().All {
		if !sceneCameraVisible(world, entity) {
			continue
		}
		camera := access.Get()

		if !camera.Active {
			continue
		}
		projection, projectionOK := world.Get[Projection](entity)
		transform, transformOK := world.Get[GlobalTransform](entity)
		if !projectionOK || !transformOK {
			continue
		}
		width, height := int(canvasWidth), int(canvasHeight)
		pipeline, options, found := cameraSettingsForEntity(state, world, entity)
		if !found {
			continue
		}
		if options.Target != nil {
			width, height = options.Target.Size()
		}
		camera.UpdateViewport(width, height)
		candidate := ExtractedCamera{
			Scene:        LayerForEntity(world, entity),
			Entity:       entity,
			Camera:       camera,
			Projection:   projection,
			Transform:    transform,
			Pipeline:     pipeline,
			RenderLayers: LayersForEntity(world, entity),
			Target:       options.Target,
			ViewMode:     options.ViewMode,
			DepthClear:   options.DepthClear,
			OutputSize:   mgl32.Vec2{canvasWidth, canvasHeight},
		}
		if camera.Fallback {
			fallback = candidate
			continue
		}
		if candidate.Scene.Overlay && candidate.Target == nil {
			candidate.Camera.Clear = false
		}
		has2D = has2D || pipeline == CameraPipeline2D
		if pipeline == CameraPipeline2D {
			sceneCameras[candidate.Scene.ID] = true
		}
		state.Cameras = append(state.Cameras, candidate)
	}
	if fallback.Entity != 0 {
		for _, access := range world.Query[SceneLayer]().All {
			layer := access.Get()
			if sceneCameras[layer.ID] {
				continue
			}
			candidate := fallback
			candidate.Scene = layer
			if layer.Overlay {
				candidate.Camera.Clear = false
			}
			state.Cameras = append(state.Cameras, candidate)
			sceneCameras[layer.ID] = true
		}
		if !has2D && len(state.Cameras) == 0 {
			state.Cameras = append(state.Cameras, fallback)
		}
	}
	smallsort.InsertionSortFunc(state.Cameras, compareExtractedCamera)
	return nil
}

func cameraSettingsForEntity(state *RenderWorld, world *ecs.World, entity ecs.Entity) (CameraPipeline, CameraRenderOptions, bool) {
	if state != nil {
		for pipeline := len(state.cameraSettings) - 1; pipeline >= 0; pipeline-- {
			if read := state.cameraSettings[pipeline].read; read != nil {
				if options, ok := read(world, entity); ok {
					return CameraPipeline(pipeline), options, true
				}
			}
		}
	}
	return CameraPipeline2D, CameraRenderOptions{ViewMode: render.RenderViewLit}, world.Has[Camera2D](entity)
}

func compareExtractedCamera(a, b ExtractedCamera) int {
	if order := cmp.Compare(a.Scene.Order, b.Scene.Order); order != 0 {
		return order
	}
	if order := cmp.Compare(a.Camera.Order, b.Camera.Order); order != 0 {
		return order
	}
	return cmp.Compare(a.Entity, b.Entity)
}

func activeCamera2DComponents(world *ecs.World) (Camera, Projection, GlobalTransform, bool) {
	if state, ok := world.GetResource[*RenderWorld](); ok && state != nil && state.activeCamera2DSet {
		return state.activeCamera2D.Camera, state.activeCamera2D.Projection, state.activeCamera2D.Transform, true
	}
	return primaryCamera2DComponents(world)
}

// ClipFromWorld includes viewport placement for draws into the full output surface.
func (camera ExtractedCamera) ClipFromWorld() mgl32.Mat4 {
	clipFromWorld := camera.Camera.ClipFromWorld(camera.Transform, camera.Projection)
	var width, height float32
	if camera.Target != nil {
		targetWidth, targetHeight := camera.Target.Size()
		width, height = float32(targetWidth), float32(targetHeight)
	} else {
		width, height = camera.OutputSize[0], camera.OutputSize[1]
		if width <= 0 || height <= 0 {
			return clipFromWorld
		}
	}
	viewport := camera.Camera.ResolvedViewport()
	if viewport.X == 0 && viewport.Y == 0 && viewport.Width == width && viewport.Height == height {
		return clipFromWorld
	}
	return viewport.PlacementMatrix(width, height).Mul4(clipFromWorld)
}

// PlacementMatrix maps local clip coordinates into this viewport's output region.
func (viewport CameraViewport) PlacementMatrix(width, height float32) mgl32.Mat4 {
	sx, sy := viewport.Width/width, viewport.Height/height
	ox := (viewport.X+viewport.Width*0.5)*2/width - 1
	oy := 1 - (viewport.Y+viewport.Height*0.5)*2/height
	return mgl32.Mat4{
		sx, 0, 0, 0,
		0, sy, 0, 0,
		0, 0, 1, 0,
		ox, oy, 0, 1,
	}
}

func visibleRenderItems2D(world *ecs.World, items []renderItem2D, camera ExtractedCamera, defaultUICamera ecs.Entity) []uint32 {
	visible := world.FrameScratch[visibleRenderItems2DScratch, uint32]()
	for i := range items {
		if renderItemForCamera2D(&items[i], camera, defaultUICamera) {
			visible.Values = append(visible.Values, uint32(i))
		}
	}
	visible.ReleaseUnused()
	world.FrameValue[visibleRenderItems2DScratch, reuse.Retention]().Record(len(visible.Values))
	return visible.Values
}
