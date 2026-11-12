package scene

import (
	"hash/maphash"
	"image"
	"math"

	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/graphics"
	"github.com/onix-engine/onix/internal/reuse"
	"github.com/onix-engine/onix/render"
	"github.com/go-gl/mathgl/mgl32"
)

const compositeTileSize2D = 128
const defaultCompositeBudget2D = 64 << 20

var paintSeed2D = maphash.MakeSeed()

type paintItem2D struct {
	contentHash uint64
	rawBounds   [4]float32
	isOpaque    bool
	cacheable   bool
	fringe      int
}

func paintMix2D(hash, value uint64) uint64 {
	return (hash ^ (value + 0x9e3779b97f4a7c15 + hash<<6 + hash>>2)) * 0x100000001b3
}

func preparePaintItem2D(item *renderItem2D, device *render.Device) paintItem2D {
	if !item.ui || item.materialData().Shader != nil || item.kind != renderItemSprite2D && item.kind != renderItemShape2D && item.kind != renderItemTextLayer2D {
		return paintItem2D{}
	}
	if item.texture.DynamicContent() {
		return paintItem2D{}
	}
	// Hash values rather than snapshot addresses. Retained text and sprite
	// snapshots may move during capacity growth without changing any pixels.
	hash := maphash.Comparable(paintSeed2D, struct {
		Color         graphics.ColorRGBA
		Anchor        Anchor
		Width, Height float32
		Kind          renderItemKind2D
	}{item.color, item.anchor, item.width, item.height, item.kind})
	linear := item.transform
	linear[12], linear[13], linear[14] = 0, 0, 0
	hash = paintMix2D(hash, maphash.Comparable(paintSeed2D, linear))
	if texture := item.texture; texture != nil {
		hash = paintMix2D(hash, texture.BatchKey())
		hash = paintMix2D(hash, texture.ContentRevision())
		hash = paintMix2D(hash, maphash.Comparable(paintSeed2D, textureMatrix(texture)))
		sx, sy := texture.RasterScale()
		hash = paintMix2D(hash, maphash.Comparable(paintSeed2D, struct {
			Width, Height, X, Y int
			ScaleX, ScaleY      float32
		}{texture.SourceWidth, texture.SourceHeight, texture.OffsetX, texture.OffsetY, sx, sy}))
		if !texture.ValidFor(device) {
			return paintItem2D{cacheable: true}
		}
	}
	var minX, minY, maxX, maxY float32
	fringe := 1
	var opaque image.Rectangle
	if item.kind == renderItemTextLayer2D {
		text := item.snapshot[textPaint2D]()
		if text == nil {
			return paintItem2D{}
		}
		scale := float32(normalizeTextRasterScale(text.rasterScale))
		step := float64(compositeTileSize2D) * normalizeTextRasterScale(text.rasterScale)
		if step != math.Trunc(step) {
			// Slot translations must preserve the glyph's raster grid phase.
			return paintItem2D{}
		}
		fringe = int(math.Ceil(float64(max(1, 1/scale))))
		hash = paintMix2D(hash, maphash.Comparable(paintSeed2D, struct {
			Alpha, Scale          float32
			LCD, Outlined, LCDRun bool
		}{text.Alpha, scale, text.LCD, item.outlined, item.lcdText}))
		minX, minY = float32(math.Inf(1)), float32(math.Inf(1))
		maxX, maxY = -minX, -minY
		for i := item.glyphStart; i < item.glyphEnd; i++ {
			glyph := text.glyphs[i]
			base := glyph.raster.Glyph()
			source := base
			if item.outlined {
				source = text.outlineGlyphs[i].Glyph()
			}
			hash = paintMix2D(hash, maphash.Comparable(paintSeed2D, [4]float32{glyph.x, glyph.y, glyph.logicalWidth, glyph.logicalHeight}))
			hash = paintMix2D(hash, maphash.Comparable(paintSeed2D, source))
			if !source.Visible() {
				continue
			}
			x := glyph.x + (source.OffsetX-base.OffsetX)/scale
			y := glyph.y + (source.OffsetY-base.OffsetY)/scale
			a, b, d, e, _ := boundsFromBase2D(item.transform.Mul4(mgl32.Translate3D(x, y, 0)), float32(source.Width)/scale, float32(source.Height)/scale)
			minX, minY, maxX, maxY = min(minX, a), min(minY, b), max(maxX, d), max(maxY, e)
		}
		if minX > maxX {
			return paintItem2D{cacheable: true}
		}
	} else {
		sprite := item.snapshot[Sprite2D]()
		if sprite == nil {
			return paintItem2D{}
		}
		hash = paintMix2D(hash, maphash.Comparable(paintSeed2D, struct {
			Shape Shape2D
			Tint  graphics.ColorRGBA
			Alpha float32
		}{sprite.Shape, sprite.Tint, sprite.Alpha}))
		if sprite.Slice != nil {
			hash = paintMix2D(hash, maphash.Comparable(paintSeed2D, *sprite.Slice))
		}
		matrix := item.quadTransform()
		minX, minY, maxX, maxY, _ = boundsFromBase2D(matrix, item.width, item.height)
		if item.kind == renderItemShape2D && sprite.Shape.Kind == render.ShapeRoundedRect && sprite.Shape.Radius == 0 && sprite.Shape.BorderWidth == 0 && sprite.Shape.Fill.A == 1 && sprite.Alpha == 1 && !item.clip.hasRounded && matrix[1] == 0 && matrix[4] == 0 && matrix[3] == 0 && matrix[7] == 0 && matrix[15] == 1 {
			opaque = image.Rect(int(math.Ceil(float64(minX))), int(math.Ceil(float64(minY))), int(math.Floor(float64(maxX))), int(math.Floor(float64(maxY))))
		}
	}
	// Quarter-pixel text rasterization can extend one pixel past base glyph
	// metrics. Keep a conservative fringe around every primitive.
	if !finitePaintBounds2D(minX, minY, maxX, maxY) {
		return paintItem2D{}
	}
	paint := paintItem2D{contentHash: hash, rawBounds: [4]float32{minX, minY, maxX, maxY}, isOpaque: !opaque.Empty(), cacheable: true, fringe: fringe}
	return paint
}

// Geometry-only updates reuse the content fingerprint and glyph geometry.
func (paint paintItem2D) rectangles(item *renderItem2D) (bounds, opaque image.Rectangle) {
	return paint.boundsRect(item), paint.opaqueRect(item)
}

func (paint paintItem2D) boundsRect(item *renderItem2D) image.Rectangle {
	b := paint.rawBounds
	bounds := image.Rect(int(math.Floor(float64(b[0])))-paint.fringe, int(math.Floor(float64(b[1])))-paint.fringe, int(math.Ceil(float64(b[2])))+paint.fringe, int(math.Ceil(float64(b[3])))+paint.fringe)
	if item.clip.valid {
		clip := image.Rect(int(math.Floor(float64(item.clip.minX))), int(math.Floor(float64(item.clip.minY))), int(math.Ceil(float64(item.clip.maxX))), int(math.Ceil(float64(item.clip.maxY))))
		bounds = bounds.Intersect(clip)
	}
	return bounds
}

func (paint paintItem2D) opaqueRect(item *renderItem2D) image.Rectangle {
	if !paint.isOpaque || item.clip.hasRounded {
		return image.Rectangle{}
	}
	b := paint.rawBounds
	opaque := image.Rect(int(math.Ceil(float64(b[0]))), int(math.Ceil(float64(b[1]))), int(math.Floor(float64(b[2]))), int(math.Floor(float64(b[3]))))
	if item.clip.valid {
		inner := image.Rect(int(math.Ceil(float64(item.clip.minX))), int(math.Ceil(float64(item.clip.minY))), int(math.Floor(float64(item.clip.maxX))), int(math.Floor(float64(item.clip.maxY))))
		opaque = opaque.Intersect(inner)
	}
	return opaque
}

func (paint *paintItem2D) tileHash(item *renderItem2D, rect image.Rectangle) uint64 {
	hash := paintMix2D(paint.contentHash, uint64(math.Float32bits(item.transform[12]-float32(rect.Min.X))))
	hash = paintMix2D(hash, uint64(math.Float32bits(item.transform[13]-float32(rect.Min.Y))))
	if item.clip.valid {
		clip := item.clip
		// Only the clip that intersects this tile affects its pixels.
		clip.minX, clip.minY = max(clip.minX, float32(rect.Min.X))-float32(rect.Min.X), max(clip.minY, float32(rect.Min.Y))-float32(rect.Min.Y)
		clip.maxX, clip.maxY = min(clip.maxX, float32(rect.Max.X))-float32(rect.Min.X), min(clip.maxY, float32(rect.Max.Y))-float32(rect.Min.Y)
		if clip.hasRounded {
			clip.rounded.minX -= float32(rect.Min.X)
			clip.rounded.maxX -= float32(rect.Min.X)
			clip.rounded.minY -= float32(rect.Min.Y)
			clip.rounded.maxY -= float32(rect.Min.Y)
		}
		hash = paintMix2D(hash, maphash.Comparable(paintSeed2D, clip))
	}
	return hash
}

func finitePaintBounds2D(values ...float32) bool {
	for _, v := range values {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || v < -1<<29 || v > 1<<29 {
			return false
		}
	}
	return true
}

type compositeTile2D struct {
	candidates     []uint32 // front to back, including occluded content
	hash, previous uint64
	slot           int
	visible        uint32 // visible candidates form a prefix
	covered        bool
	dirty          bool
	changed        bool
}

func (tile *compositeTile2D) visibleItems() []uint32 { return tile.candidates[:tile.visible] }

type compositeCoverage2D struct {
	visible uint32
	present bool
}

type compositeReuse2D struct {
	hash uint64
	slot int
	used bool
}

// CompositeSettings2D bounds the persistent viewport cache. A negative budget
// disables it; zero selects 64 MiB. Dense animation uses direct drawing.
type CompositeSettings2D struct{ MaxCacheBytes int64 }

// CompositeStatistics2D distinguishes cached pixels from draw submission work.
// RasterPixels is the area redrawn, not a GPU time or fragment-count estimate.
type CompositeStatistics2D struct {
	Tiles, DirtyTiles            int
	ReusedTiles                  int
	RasterPixels                 int
	Submitted, Occluded          int
	PreparedItems, PreparedTiles int
	CacheBytes                   int64
	Direct                       bool
}

type compositor2D struct {
	texture           *graphics.Texture
	retained          *retainedRender2D
	target            *render.RenderTarget
	device            *render.Device
	tiles             []compositeTile2D
	damageTiles       []int
	coverage          []compositeCoverage2D
	usedSlots         []bool
	reuse             []compositeReuse2D
	replay            renderItem2D
	indicesRetention  reuse.Retention
	coverageRetention reuse.Retention
	stats             CompositeStatistics2D
	columns           int
	seen              uint64
	cameraKey         uint64
	occluded          int
	generation        uint64
	height            int
	width             int
	clear             graphics.ColorRGBA
	prepared          bool
	valid             bool
	spatial           bool
}

// compositeUI2D consumes a full UI output. Other camera pipelines, transparent
// outputs and custom materials keep their normal rendering semantics.
func (view *compositor2D) draw(c *graphics.Canvas2D, world *ecs.World, state *RenderWorld, camera ExtractedCamera, defaultUICamera ecs.Entity, cache *retainedRender2D, budget int64) (bool, error) {
	generation := cache.generation
	widthValue, heightValue := c.Size()
	width, height := int(widthValue), int(heightValue)
	bytes := int64(width) * int64(height) * 4
	view.stats = CompositeStatistics2D{}
	if camera.Target != nil || camera.Pipeline != CameraPipeline2D || !camera.Camera.Clear || camera.Camera.ClearColor.A != 1 || c.BlendState() != render.PremultipliedAlphaBlend || width <= 0 || height <= 0 || bytes > budget {
		view.stats.Direct = true
		return false, view.dispose()
	}
	if _, ok := c.Device().Driver().(render.RenderTargetDriver); !ok {
		view.stats.Direct = true
		return false, nil
	}
	resized := view.width != width || view.height != height || view.device != c.Device()
	if resized {
		if err := view.dispose(); err != nil {
			return true, err
		}
		view.width, view.height, view.device = width, height, c.Device()
		view.columns = (width + compositeTileSize2D - 1) / compositeTileSize2D
		view.tiles = make([]compositeTile2D, view.columns*((height+compositeTileSize2D-1)/compositeTileSize2D))
		capacity := 1
		for capacity < len(view.tiles)*4 {
			capacity *= 2
		}
		view.reuse = make([]compositeReuse2D, capacity)
		view.usedSlots = make([]bool, len(view.tiles))
	}
	clearChanged := view.clear != camera.Camera.ClearColor
	view.clear = camera.Camera.ClearColor
	cameraKey := maphash.Comparable(paintSeed2D, struct {
		Entity, Default ecs.Entity
		Scene           SceneLayer
		Empty           bool
	}{camera.Entity, defaultUICamera, camera.Scene, camera.RenderLayers.empty})
	for _, block := range camera.RenderLayers.blocks {
		cameraKey = paintMix2D(cameraKey, block)
	}
	if !view.prepared || view.generation != generation || clearChanged || view.cameraKey != cameraKey {
		clear(view.reuse)
		if view.valid && !clearChanged {
			for i := range view.tiles {
				tile := &view.tiles[i]
				entry := view.reuseEntry(tile.previous)
				*entry = compositeReuse2D{hash: tile.previous, slot: tile.slot, used: true}
			}
		}
		if view.prepared && view.spatial && view.retained == cache && view.generation+1 == generation && cache.damageValid && !clearChanged && view.cameraKey == cameraKey && len(view.coverage) == len(state.Items2D) {
			if !view.updateDamage(state, camera, defaultUICamera, cache.damage) {
				view.stats.Direct = true
				return false, view.releaseTarget()
			}
		} else {
			view.stats.PreparedTiles = len(view.tiles)
			view.occluded = 0
			// Dense scrolling uses sequential preparation without maintaining a
			// spatial index that will be replaced again on the next frame.
			spatial := cache.stats.PropertiesUpdated <= max(16, len(state.Items2D)/8)
			if spatial {
				view.coverage = reuse.Resize(view.coverage, len(state.Items2D))
				clear(view.coverage)
			}
			for i := range view.tiles {
				tile := &view.tiles[i]
				tile.candidates = tile.candidates[:0]
				tile.changed = false
				tile.visible = 0
				r := view.tileRect(i)
				tile.hash, tile.covered = paintMix2D(uint64(r.Dx()), uint64(r.Dy())), false
			}
			screen := image.Rect(0, 0, width, height)
			// One reverse traversal builds spatial bins, computes damage and rejects
			// occluded content. No separate scene pass is needed for each concern.
			for i := len(state.Items2D) - 1; i >= 0; i-- {
				view.stats.PreparedItems++
				item := &state.Items2D[i]
				if !renderItemForCamera2D(item, camera, defaultUICamera) {
					continue
				}
				paint := item.paint
				if paint == nil || !paint.cacheable {
					view.stats.Direct = true
					return false, view.releaseTarget()
				}
				raw, opaque := paint.rectangles(item)
				bounds := raw.Intersect(screen)
				if bounds.Empty() {
					continue
				}
				if spatial {
					view.coverage[i].present = true
				}
				x0, y0 := bounds.Min.X/compositeTileSize2D, bounds.Min.Y/compositeTileSize2D
				x1, y1 := (bounds.Max.X-1)/compositeTileSize2D, (bounds.Max.Y-1)/compositeTileSize2D
				visible := false
				for y := y0; y <= y1; y++ {
					for x := x0; x <= x1; x++ {
						index := y*view.columns + x
						tile := &view.tiles[index]
						if spatial || !tile.covered {
							tile.candidates = append(tile.candidates, uint32(i))
						}
						if tile.covered {
							continue
						}
						visible = true
						if spatial {
							view.coverage[i].visible++
						}
						tile.visible++
						tile.hash = paintMix2D(tile.hash, paint.tileHash(item, view.tileRect(index)))
						if view.tileRect(index).In(opaque) {
							tile.covered = true
						}
					}
				}
				if !visible {
					view.occluded++
				}
			}
			view.spatial = spatial
		}
		view.generation, view.prepared = generation, true
		view.retained = cache
		view.cameraKey = cameraKey
	}
	view.stats.Tiles, view.stats.CacheBytes, view.stats.Occluded = len(view.tiles), bytes, view.occluded
	if !view.spatial {
		view.coverage = view.coverage[:0]
	}
	view.coverage = view.coverageRetention.Trim(view.coverage, 64)
	clear(view.usedSlots)
	changed := 0
	indexCapacity, indexCount := 0, 0
	for i := range view.tiles {
		tile := &view.tiles[i]
		indexCapacity += cap(tile.candidates)
		indexCount += len(tile.candidates)
		tile.dirty = !view.valid || clearChanged || tile.hash != tile.previous
		if tile.hash != tile.previous || clearChanged {
			changed++
		}
		if !tile.dirty {
			view.usedSlots[tile.slot] = true
		}
	}
	if view.indicesRetention.TrimCapacity(indexCapacity, indexCount, len(view.tiles)*16) != 0 {
		for i := range view.tiles {
			tile := &view.tiles[i]
			target := max(16, len(tile.candidates)*2)
			if cap(tile.candidates) > target {
				indices := make([]uint32, len(tile.candidates), target)
				copy(indices, tile.candidates)
				tile.candidates = indices
			}
		}
	}
	dirty := 0
	for i := range view.tiles {
		tile := &view.tiles[i]
		if !tile.dirty {
			continue
		}
		if view.valid && !clearChanged {
			entry := view.reuseEntry(tile.hash)
			if entry.used {
				tile.slot, tile.dirty = entry.slot, false
				view.usedSlots[tile.slot] = true
				view.stats.ReusedTiles++
				continue
			}
		}
		dirty++
	}
	// Full-screen animation avoids the extra render target pass and bandwidth.
	if view.target != nil && changed*2 > len(view.tiles) && dirty*2 > len(view.tiles) {
		view.valid, view.stats.Direct = false, true
		view.commitTiles()
		return false, nil
	}
	for i := range view.tiles {
		tile := &view.tiles[i]
		if !tile.dirty {
			continue
		}
		// Keep identity placement when possible. Edge slots must have the same
		// dimensions; reused slots cannot be overwritten during this frame.
		slot := i
		if view.usedSlots[slot] {
			r := view.tileRect(i)
			for slot = 0; slot < len(view.tiles); slot++ {
				other := view.tileRect(slot)
				if !view.usedSlots[slot] && r.Size() == other.Size() {
					break
				}
			}
		}
		tile.slot = slot
		view.usedSlots[slot] = true
	}
	view.stats.DirtyTiles = dirty
	if dirty != 0 {
		if view.target == nil {
			var err error
			view.target, err = render.NewRenderTarget(render.RenderTargetDescriptor{Width: width, Height: height})
			if err != nil {
				return true, err
			}
			view.texture, err = graphics.CreateTextureFromRenderTarget(view.target)
			if err != nil {
				return true, err
			}
		}
		full := !view.valid || dirty == len(view.tiles)
		err := c.DrawTo(view.target, render.RenderTargetClear{ClearColor: full, Color: render.Color(view.clear)}, func() error {
			c.ResetProjectionMatrix()
			c.ResetViewMatrix()
			c.BindMaterial2D(nil)
			for i := range view.tiles {
				tile := &view.tiles[i]
				if !tile.dirty {
					continue
				}
				r := view.tileRect(tile.slot)
				logical := view.tileRect(i)
				dx, dy := float32(r.Min.X-logical.Min.X), float32(r.Min.Y-logical.Min.Y)
				view.stats.RasterPixels += r.Dx() * r.Dy()
				c.PushClipRect(float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()))
				if !full {
					// Opaque output lets an ordinary opaque quad reset precisely the
					// dirty pixels, including pixels uncovered by removals or moves.
					model := mgl32.Translate3D(float32(r.Min.X), float32(r.Min.Y), 0).Mul4(mgl32.Scale3D(float32(r.Dx()), float32(r.Dy()), 1))
					c.DrawShapeMatrices(model, mgl32.Ident3(), float32(r.Dx()), float32(r.Dy()), render.ShapeRoundedRect, view.clear, graphics.ColorRGBA{}, 0, 0, 1)
				}
				for j := int(tile.visible) - 1; j >= 0; j-- {
					view.replay = state.Items2D[tile.candidates[j]]
					item := &view.replay
					item.transform[12] += dx
					item.transform[13] += dy
					if item.clip.valid {
						item.clip.minX, item.clip.maxX = item.clip.minX+dx, item.clip.maxX+dx
						item.clip.minY, item.clip.maxY = item.clip.minY+dy, item.clip.maxY+dy
						item.clip.rounded.minX += dx
						item.clip.rounded.maxX += dx
						item.clip.rounded.minY += dy
						item.clip.rounded.maxY += dy
					}
					if item.texture != nil {
						c.BindTexture(item.texture)
					}
					state.drawers2D[item.kind](c, item, renderViewport2D{clipFromLayer: mgl32.Ortho2D(0, widthValue, heightValue, 0), active: true})
					view.stats.Submitted++
				}
				c.PopClipRect()
			}
			return nil
		})
		if err != nil {
			view.valid = false
			return true, err
		}
		view.valid = true
	}
	view.commitTiles()
	c.ResetProjectionMatrix()
	c.ResetViewMatrix()
	c.BindMaterial2D(nil)
	c.BindTexture(view.texture)
	textureMatrix := mgl32.Ident3()
	if c.Device().Driver().RendererInfo().RenderTargetBottomLeft {
		textureMatrix = mgl32.Translate2D(0, 1).Mul3(mgl32.Scale2D(1, -1))
	}
	identity := true
	for i := range view.tiles {
		identity = identity && view.tiles[i].slot == i
	}
	if identity {
		c.DrawTintedMatricesCustom(mgl32.Scale3D(widthValue, heightValue, 1), textureMatrix, graphics.White, 1, [4][4]float32{})
	} else {
		for i := range view.tiles {
			r, source := view.tileRect(i), view.tileRect(view.tiles[i].slot)
			uv := textureMatrix.Mul3(mgl32.Translate2D(float32(source.Min.X)/widthValue, float32(source.Min.Y)/heightValue)).Mul3(mgl32.Scale2D(float32(source.Dx())/widthValue, float32(source.Dy())/heightValue))
			model := mgl32.Translate3D(float32(r.Min.X), float32(r.Min.Y), 0).Mul4(mgl32.Scale3D(float32(r.Dx()), float32(r.Dy()), 1))
			c.DrawTintedMatricesCustom(model, uv, graphics.White, 1, [4][4]float32{})
		}
	}
	return true, nil
}

// A reusable open-addressed table avoids map bucket allocations during scroll.
func (view *compositor2D) reuseEntry(hash uint64) *compositeReuse2D {
	mask := uint64(len(view.reuse) - 1)
	for index := hash & mask; ; index = (index + 1) & mask {
		entry := &view.reuse[index]
		if !entry.used || entry.hash == hash {
			return entry
		}
	}
}

func (view *compositor2D) tileRect(index int) image.Rectangle {
	x, y := index%view.columns*compositeTileSize2D, index/view.columns*compositeTileSize2D
	return image.Rect(x, y, min(x+compositeTileSize2D, view.width), min(y+compositeTileSize2D, view.height))
}

func (view *compositor2D) commitTiles() {
	for i := range view.tiles {
		view.tiles[i].previous = view.tiles[i].hash
		view.tiles[i].dirty = false
	}
}

func (view *compositor2D) dispose() error {
	if err := view.releaseTarget(); err != nil {
		return err
	}
	// A disabled or retired compositor owns no reusable scene storage. Preserve
	// only the caller's frame accounting; resizing rebuilds the workspace.
	*view = compositor2D{stats: view.stats, seen: view.seen}
	return nil
}

func (view *compositor2D) releaseTarget() error {
	if view.target != nil {
		if err := view.target.Dispose(); err != nil {
			return err
		}
	}
	view.target, view.texture, view.valid, view.prepared = nil, nil, false, false
	return nil
}

func renderItemForCamera2D(item *renderItem2D, camera ExtractedCamera, defaultUICamera ecs.Entity) bool {
	if camera.Scene.ID != 0 && item.scene.ID != 0 && camera.Scene.ID != item.scene.ID || !camera.RenderLayers.Intersects(item.layers) {
		return false
	}
	if !item.ui {
		return camera.Pipeline == CameraPipeline2D
	}
	target := item.target
	if target == 0 {
		target = defaultUICamera
	}
	return target == camera.Entity
}
