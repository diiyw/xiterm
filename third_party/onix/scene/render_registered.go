package scene

import (
	"reflect"

	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/graphics"
	"github.com/onix-engine/onix/internal/reuse"
	"github.com/onix-engine/onix/internal/smallsort"
)

type renderCollector2D func(*graphics.Canvas2D, *ecs.World, *renderSnapshots2D, *renderHierarchy2DCache, []renderItem2D, bool) []renderItem2D
type renderItemDrawer2D func(*graphics.Canvas2D, *renderItem2D, renderViewport2D)
type renderBatchDrawer2D func(*graphics.Canvas2D, *ecs.World, renderItemList2D, renderViewport2D)
type registeredCameraRenderer func(*graphics.Canvas2D, *ecs.World, *RenderWorld, ExtractedCamera, ecs.Entity) error

type renderCollector2DKind uint8

const (
	renderCollectorSpriteTrail2D renderCollector2DKind = iota
	renderCollectorSprite2D
	renderCollectorSpriteLayers2D
	renderCollectorParticle2D
	renderCollectorTilemap2D
	renderCollectorSpine2D
	renderCollectorText2D
	renderCollectorUIVisual2D
	renderCollector2DCount
)

func RegisterSpriteRenderer2D(world *ecs.World) {
	state := RenderWorldFor(world)
	state.registry2D++
	state.collectors2D[renderCollectorSprite2D] = collectSprites2D
	state.drawers2D[renderItemSprite2D] = drawSpriteRenderItem2D
	state.drawers2D[renderItemShape2D] = drawShapeRenderItem2D
}

func RegisterTextRenderer2D(world *ecs.World) {
	state := RenderWorldFor(world)
	state.registry2D++
	state.collectors2D[renderCollectorText2D] = collectText2D
	state.drawers2D[renderItemTextLayer2D] = drawTextRenderItem2D
}

func RegisterUIRenderer2D(world *ecs.World) {
	RenderWorldFor(world).registry2D++
	RenderWorldFor(world).collectors2D[renderCollectorUIVisual2D] = appendUIVisualRenderItems2D
}

func RegisterSpriteTrailRenderer2D(world *ecs.World) {
	RenderWorldFor(world).registry2D++
	RenderWorldFor(world).collectors2D[renderCollectorSpriteTrail2D] = collectSpriteTrails2D
}

func RegisterSpriteLayersRenderer2D(world *ecs.World) {
	RenderWorldFor(world).registry2D++
	RenderWorldFor(world).collectors2D[renderCollectorSpriteLayers2D] = collectSpriteLayers2D
}

func RegisterParticleRenderer2D(world *ecs.World) {
	state := RenderWorldFor(world)
	state.registry2D++
	state.collectors2D[renderCollectorParticle2D] = collectParticles2D
	state.drawers2D[renderItemParticle2D] = drawParticleRenderItem2D
}

func RegisterTilemapRenderer2D(world *ecs.World) {
	state := RenderWorldFor(world)
	state.registry2D++
	state.collectors2D[renderCollectorTilemap2D] = collectTilemaps2D
	state.drawers2D[renderItemTilemap2D] = drawTilemapRenderItem2D
}

func RegisterSpineRenderer2D(world *ecs.World) {
	state := RenderWorldFor(world)
	state.registry2D++
	state.collectors2D[renderCollectorSpine2D] = collectSpines2D
	state.batchDrawers2D[renderItemSpine2D] = drawSpineRenderBatch2D
}

func RegisterCameraRenderer2D(world *ecs.World) {
	RenderWorldFor(world).camera2DLayer = render2DLayer
}

// RegisterCameraRenderer binds a typed camera component to a render pipeline.
// Registration is idempotent for the same type and pipeline. Extraction borrows
// the component and stores only output settings; stable frames do not allocate.
func (state *RenderWorld) RegisterCameraRenderer[T any](pipeline CameraPipeline, describe func(*T) CameraRenderOptions, draw CameraRenderer) {
	if pipeline >= cameraPipelineCount {
		panic("scene: camera pipeline is out of range")
	}
	typ := reflect.TypeFor[T]()
	if state.cameraSettings[pipeline].typ == typ {
		return
	}
	state.cameraSettings[pipeline] = cameraSettingsReader{
		typ: typ,
		read: func(world *ecs.World, entity ecs.Entity) (CameraRenderOptions, bool) {
			component := world.GetRef[T](entity)
			if component == nil {
				return CameraRenderOptions{}, false
			}
			if describe == nil {
				return CameraRenderOptions{}, true
			}
			return describe(component), true
		},
		revision: componentValueVersion[T],
	}
	state.registry2D++
	state.cameraRenderers[pipeline] = draw
}

// ExtractRender2DSystem extracts items for the renderers registered in world.
func ExtractRender2DSystem(name string) ecs.SystemConfig {
	if name == "" {
		name = "extract_render_2d"
	}
	return ecs.SystemConfig{Name: name, System: extractRender2D}
}

func extractRender2D(context *ecs.SystemContext) error {
	c := graphics.CanvasFor(context.World)
	if c == nil || context.World == nil {
		return nil
	}
	cache := context.World.ScopeValue[retainedRender2D, retainedRender2D]()
	state := RenderWorldFor(context.World)
	state.Items2D = cache.extract(c, context.World, state)
	state.Preparation2D = cache.stats
	return nil
}

func collectRenderItems2D(c *graphics.Canvas2D, world *ecs.World, cull bool) []renderItem2D {
	return collectRenderSnapshots2D(c, world, cull, false)
}

func collectRenderSnapshots2D(c *graphics.Canvas2D, world *ecs.World, cull, borrow bool) []renderItem2D {
	scratch := world.FrameScratch[renderItems2DScratch, renderItem2D]()
	scratch.Values = collectRenderSnapshotsInto2D(c, world, scratch.Values, cull, borrow)
	scratch.ReleaseUnused()
	world.FrameValue[renderItemsRetention2DScratch, reuse.Retention]().Record(len(scratch.Values))
	return scratch.Values
}

func collectRenderSnapshotsInto2D(c *graphics.Canvas2D, world *ecs.World, items []renderItem2D, cull, borrow bool) []renderItem2D {
	state := RenderWorldFor(world)
	snapshots := world.FrameValue[renderSnapshots2DScratch, renderSnapshots2D]()
	snapshots.reset()
	snapshots.borrow = borrow
	hierarchy := frameRenderHierarchy2DCachePtr(world)
	for _, collect := range state.collectors2D {
		if collect != nil {
			items = collect(c, world, snapshots, hierarchy, items, cull)
		}
	}
	smallsort.SortFunc(items, compareRenderPhaseItem2D[renderItem2D])
	snapshots.releaseUnused()
	world.FrameValue[renderSnapshots2DScratch, reuse.Retention]().Record(snapshots.length())
	return items
}

// RenderCamerasSystem submits every registered camera pipeline in camera order.
func RenderCamerasSystem(name string) ecs.SystemConfig {
	if name == "" {
		name = "render_cameras"
	}
	return ecs.SystemConfig{Name: name, System: renderCameras}
}

func renderCameras(context *ecs.SystemContext) error {
	world := context.World
	c := graphics.CanvasFor(context.World)
	if world == nil || c == nil {
		return nil
	}
	state := RenderWorldFor(world)
	state.compositeFrame2D++
	state.Composite2D = CompositeStatistics2D{}
	defer func() {
		for key, view := range state.compositors2D {
			if view.seen != state.compositeFrame2D {
				_ = view.dispose()
				delete(state.compositors2D, key)
			}
		}
	}()
	defaultUICamera := ecs.Entity(0)
	if state.camera2DLayer != nil {
		for _, camera := range state.Cameras {
			if camera.Pipeline == CameraPipeline2D {
				defaultUICamera = camera.Entity
				break
			}
		}
	}
	swapchainStarted := false
	for _, camera := range state.Cameras {
		renderCamera := state.cameraRenderers[camera.Pipeline]
		if renderCamera == nil && state.camera2DLayer == nil {
			continue
		}
		state.compositeAllowed2D = !swapchainStarted
		if camera.Target == nil && !swapchainStarted && camera.Camera.Clear {
			c.ClearColor(camera.Camera.ClearColor)
		}
		if camera.Target == nil {
			swapchainStarted = true
		}
		if renderCamera != nil {
			if err := renderCamera(c, world, camera); err != nil {
				return err
			}
		}
		if state.camera2DLayer != nil {
			if err := state.camera2DLayer(c, world, state, camera, defaultUICamera); err != nil {
				return err
			}
		}
	}
	return nil
}

func render2DLayer(c *graphics.Canvas2D, world *ecs.World, state *RenderWorld, camera ExtractedCamera, defaultUICamera ecs.Entity) error {
	if camera.Scene.ID != 0 {
		for _, candidate := range state.Cameras {
			if candidate.Scene.ID == camera.Scene.ID && candidate.Pipeline == CameraPipeline2D {
				defaultUICamera = candidate.Entity
				break
			}
		}
	}
	if camera.Pipeline == CameraPipeline2D {
		state.activeCamera2D = camera
		state.activeCamera2DSet = true
	}
	cache := world.ScopeValue[retainedRender2D, retainedRender2D]()
	if state.compositeAllowed2D && cache.initialized && !cache.direct && state.cameraRenderers[camera.Pipeline] == nil {
		budget := int64(defaultCompositeBudget2D)
		if settings, ok := world.GetResource[CompositeSettings2D](); ok && settings.MaxCacheBytes != 0 {
			budget = settings.MaxCacheBytes
		}
		key := compositeViewKey2D{camera.Entity, camera.Scene.ID}
		if state.compositors2D == nil {
			state.compositors2D = make(map[compositeViewKey2D]*compositor2D)
		}
		view := state.compositors2D[key]
		if view == nil {
			view = &compositor2D{}
			state.compositors2D[key] = view
		}
		view.seen = state.compositeFrame2D
		for otherKey, other := range state.compositors2D {
			if otherKey != key && other.target != nil {
				budget -= int64(other.width) * int64(other.height) * 4
			}
		}
		drawn, err := view.draw(c, world, state, camera, defaultUICamera, cache, budget)
		state.Composite2D.Tiles += view.stats.Tiles
		state.Composite2D.DirtyTiles += view.stats.DirtyTiles
		state.Composite2D.ReusedTiles += view.stats.ReusedTiles
		state.Composite2D.RasterPixels += view.stats.RasterPixels
		state.Composite2D.Submitted += view.stats.Submitted
		state.Composite2D.Occluded += view.stats.Occluded
		state.Composite2D.PreparedItems += view.stats.PreparedItems
		state.Composite2D.PreparedTiles += view.stats.PreparedTiles
		state.Composite2D.CacheBytes += view.stats.CacheBytes
		state.Composite2D.Direct = state.Composite2D.Direct || view.stats.Direct
		if drawn || err != nil {
			state.activeCamera2DSet = false
			return err
		}
	}
	state.Composite2D.Direct = true
	items := visibleRenderItems2D(world, state.Items2D, camera, defaultUICamera)
	if camera.Pipeline == CameraPipeline2D || len(items) > 0 {
		drawRenderItemList2D(c, world, renderItemList2D{items: state.Items2D, indices: items, count: len(items)}, DrawViews2D{}, true)
	}
	state.activeCamera2DSet = false
	return nil
}

// Draw2D draws every registered 2D renderer using explicit world and UI views.
func Draw2D(c *graphics.Canvas2D, world *ecs.World, views DrawViews2D) {
	if c == nil || world == nil {
		return
	}
	drawRenderItems2D(c, world, collectRenderItems2D(c, world, false), views, false)
}

// Draw2DSubtree draws registered 2D render items owned by root or one of its
// descendants. Filtering reuses the frame render-item buffer.
func Draw2DSubtree(c *graphics.Canvas2D, world *ecs.World, root ecs.Entity, views DrawViews2D) {
	if c == nil || world == nil || root == 0 {
		return
	}
	items := collectRenderItems2D(c, world, false)
	filtered := items[:0]
	for _, item := range items {
		if entityInSubtree2D(world, item.entity, root) {
			filtered = append(filtered, item)
		}
	}
	clear(items[len(filtered):])
	drawRenderItems2D(c, world, filtered, views, false)
}

func entityInSubtree2D(world *ecs.World, entity, root ecs.Entity) bool {
	if world == nil || entity == 0 || root == 0 {
		return false
	}
	for remaining := world.EntityCount() + 1; entity != 0 && remaining > 0; remaining-- {
		if entity == root {
			return true
		}
		parent, ok := world.Get[Parent](entity)
		if !ok || parent.Entity == entity {
			return false
		}
		entity = parent.Entity
	}
	return false
}

func drawRenderItems2D(c *graphics.Canvas2D, world *ecs.World, items []renderItem2D, views DrawViews2D, cull bool) {
	drawRenderItemList2D(c, world, renderItemList2D{items: items, count: len(items)}, views, cull)
}

func drawRenderItemList2D(c *graphics.Canvas2D, world *ecs.World, items renderItemList2D, views DrawViews2D, cull bool) {
	state := RenderWorldFor(world)
	drawRenderPhaseList2D[renderItem2D, *renderItem2D](c, world, items, views, cull, func(item *renderItem2D) bool {
		return state.batchDrawers2D[item.kind] != nil
	}, func(batch renderBatch2D, viewport renderViewport2D, begin bool) {
		first := items.at(batch.start)
		if draw := state.batchDrawers2D[first.kind]; draw != nil {
			draw(c, world, items.slice(batch.start, batch.end), viewport)
			return
		}
		if state.drawers2D[first.kind] == nil {
			return
		}
		if begin && first.kind != renderItemShape2D {
			c.BindMaterial2D(first.materialData().Shader)
			c.BindTexture(first.texture)
		}
		state.drawers2D[first.kind](c, first, viewport)
	})
	// Scene materials must not carry into viewport compositing or editor overlays.
	c.BindMaterial2D(nil)
	c.SetMaterial2DParameters([4][4]float32{})
}
