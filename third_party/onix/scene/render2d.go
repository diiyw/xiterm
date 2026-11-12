package scene

import (
	"math"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/font"
	"github.com/onix-engine/onix/graphics"
	"github.com/onix-engine/onix/render"
	onixtext "github.com/onix-engine/onix/text"
	"github.com/onix-engine/onix/text/piecetree"
)

type RenderSettings2D struct {
	ClearColor graphics.ColorRGBA
}

type DrawViews2D struct {
	World mgl32.Mat4
	UI    mgl32.Mat4
}

type RenderOrder2D int
type UIStackIndex int

const DefaultParticleRenderOrder RenderOrder2D = 500

type Sprite2D struct {
	Texture *graphics.Texture
	Slice   *NineSlice2D
	Shape   Shape2D
	Tint    graphics.ColorRGBA
	Width   float32
	Height  float32
	Alpha   float32
}

type MeshMaterial2D[T graphics.Material2D] struct {
	Material *T
}

type NineSlice2D struct {
	Left   float32
	Right  float32
	Top    float32
	Bottom float32
}

type Shape2D struct {
	Kind        render.ShapeKind
	Fill        graphics.ColorRGBA
	Border      graphics.ColorRGBA
	Radius      float32
	BorderWidth float32
}

type TextWrapMode uint8

const (
	TextWrapWords TextWrapMode = iota
	TextWrapNewlines
	TextWrapNone
	// TextWrapCharacters wraps every line at WrapWidth, including long words.
	TextWrapCharacters
	// TextWrapTokens wraps at word/identifier and punctuation boundaries without
	// discarding whitespace. A token wider than WrapWidth remains on one line.
	TextWrapTokens
)

type TextAlignment uint8

const (
	TextAlignLeft TextAlignment = iota
	TextAlignCenter
	TextAlignRight
)

// TextIndex resolves rune offsets against a matching immutable snapshot.
type TextIndex interface {
	MatchesSnapshot(piecetree.Snapshot) bool
	ByteOffset(int) int
}

type Text2D struct {
	lastValue   piecetree.Snapshot
	layoutValue piecetree.Snapshot
	// Source supplies immutable document text; Value is used when Source is empty.
	Source piecetree.Snapshot
	// Index optionally shares a document's indexed text buffer for caret lookup.
	Index          TextIndex
	lastLineLayout *TextLineLayout2D
	face           *font.ScaledFace
	Font           *onixtext.Font
	// LineLayout spaces visual lines without inserting characters into Value.
	LineLayout *TextLineLayout2D
	lastFont   *onixtext.Font

	cache  *onixtext.GlyphCache
	Value  string
	glyphs []textGlyph2D
	// Spans are sorted, non-overlapping UTF-8 byte ranges.
	Spans               []TextSpan2D
	lastInlineObjects   []TextInlineObject2D
	outlineGlyphs       []onixtext.GlyphRef
	inlineObjectLayouts []textInlineObjectLayout2D
	lines               []textLineLayout2D
	wrapRangeCache      []textWrapRange2D
	InlineObjects       []TextInlineObject2D
	// Prelayout uses positions already computed by a layout engine for one line.
	// Its producer increments Revision whenever those positions or Value change.
	Prelayout              TextPrelayout2D
	lastDPI                float64
	DPI                    float64
	lastPrelayoutRevision  uint64
	lastLineLayoutRevision uint64
	FontSize               float64
	lineCount              int
	lastFontSize           float64
	lastRasterScale        float64
	rasterScale            float64
	OutlineColor           graphics.ColorRGBA
	Color                  graphics.ColorRGBA
	ShadowColor            graphics.ColorRGBA
	glyphBand              textGlyphBand
	lastGlyphBand          textGlyphBand
	ShadowX                float32
	lastTabWidth           float32
	WrapWidth              float32
	OutlineWidth           float32
	ShadowY                float32
	glyphLineHeight        float32
	LetterSpacing          float32
	Alpha                  float32
	MaxWidth               float32
	measuredLineHeight     float32
	// TabWidth is the distance between tab stops in logical pixels.
	TabWidth          float32
	measuredWidth     float32
	lastWrapWidth     float32
	lastLetterSpacing float32
	lastLineHeight    float32
	layoutHeight      float32
	layoutWidth       float32
	lastMaxWidth      float32
	lastOutlineWidth  float32
	// InkOffsetY shifts painted glyphs without changing line or caret geometry.
	InkOffsetY        float32
	LineHeight        float32
	lastTextAlignment TextAlignment
	glyphlessLayout   bool
	lastWrapMode      TextWrapMode
	TextAlignment     TextAlignment
	measurementValid  bool
	// LCD enables RGB-stripe antialiasing on an opaque UI surface.
	LCD             bool
	changed         bool
	WrapMode        TextWrapMode
	prelayoutActive bool
}

type TextPrelayout2D struct {
	Glyphs           []font.PositionedGlyph
	OriginX, OriginY float64
	Width, Height    float32
	Revision         uint64
}

// TextLineLayout2D adds vertical gaps and a minimum scrollable extent. Owners
// increment Revision whenever offsets or dimensions change; unchanged layouts
// require no slice comparison or allocation during measurement.
type TextLineLayout2D struct {
	Offsets       []float32
	Width, Height float32
	Revision      uint64
}

type TextSpan2D struct {
	Start int
	End   int
	Color graphics.ColorRGBA
}

type TextInlineObject2D struct {
	Start    int
	End      int
	Width    float32
	Height   float32
	Baseline float32
}

type TextInlineObjectRect2D struct {
	Start int
	End   int
	Rect  TextRangeRect2D
}

type TextRangeRect2D struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

type ClipRect2D struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// RoundedClip2D adds a uniform corner radius to a local clip rectangle. It is
// consumed by the built-in sprite pipeline in addition to ClipRect2D; keeping
// the rectangle separate preserves the existing scissor/culling fast path.
type RoundedClip2D struct {
	X         float32
	Y         float32
	Width     float32
	Height    float32
	Radius    float32
	MaskColor graphics.ColorRGBA
}

type Size2D struct {
	Width  float32
	Height float32
}

type ClipExempt2D struct {
	// ParentOnly skips the immediate parent's clip while retaining ancestor clips.
	ParentOnly bool
}

// textGlyph2D keeps only raster-space bitmap metrics: logical placement divides
// them by the raster scale, so storing both forms doubled every laid-out glyph.
type textGlyph2D struct {
	raster        onixtext.GlyphRef
	logicalWidth  float32
	logicalHeight float32
	x             float32
	y             float32
	byteIndex     int32
	line          int32
}

type textInlineObjectLayout2D struct {
	object TextInlineObject2D
	rect   TextRangeRect2D
	line   int
}

type textLineLayout2D struct {
	baseline float32
	x        float32
	y        float32
	width    float32
	height   float32
	advance  float32
	start    int
	end      int
}

func NewSprite2D() Sprite2D {
	return Sprite2D{Tint: graphics.White, Alpha: 1}
}

func Sprite2DFromTexture(texture *graphics.Texture) Sprite2D {
	sprite := NewSprite2D()
	sprite.Texture = texture
	if texture != nil {
		sprite.Width = float32(texture.LogicalWidth())
		sprite.Height = float32(texture.LogicalHeight())
	}
	return sprite
}

func NewText2D(value string) Text2D {
	return Text2D{
		Font:         onixtext.DefaultFont(),
		Value:        value,
		FontSize:     18,
		DPI:          72,
		Color:        graphics.White,
		OutlineColor: graphics.Black,
		Alpha:        1,
		changed:      true,
	}
}

func NewColorSprite2D(clr graphics.ColorRGBA, width, height int) Sprite2D {
	sprite := NewSprite2D()
	sprite.Texture = graphics.CreateTextureByColor(graphics.White)
	sprite.Tint = clr
	sprite.Width = float32(width)
	sprite.Height = float32(height)
	return sprite
}

func RoundedRectShape2D(fill graphics.ColorRGBA, radius float32) Shape2D {
	return Shape2D{Kind: render.ShapeRoundedRect, Fill: fill, Radius: radius}
}

func OutlinedRoundedRectShape2D(fill, border graphics.ColorRGBA, radius, borderWidth float32) Shape2D {
	return Shape2D{
		Kind:        render.ShapeRoundedRect,
		Fill:        fill,
		Border:      border,
		Radius:      radius,
		BorderWidth: borderWidth,
	}
}

func CheckmarkShape2D(fill graphics.ColorRGBA) Shape2D {
	return Shape2D{Kind: render.ShapeCheckmark, Fill: fill}
}

func TriangleShape2D(fill graphics.ColorRGBA, direction uint8) Shape2D {
	return Shape2D{Kind: render.ShapeTriangle, Fill: fill, BorderWidth: float32(direction)}
}

func drawSpriteRenderItem2D(c *graphics.Canvas2D, item *renderItem2D, viewport renderViewport2D) {
	if !viewport.visible(item.quadTransform(), item.width, item.height) {
		return
	}
	c.SetMaterial2DParameters(item.materialData().Custom)
	item.drawSprite(c)
}

func drawShapeRenderItem2D(c *graphics.Canvas2D, item *renderItem2D, viewport renderViewport2D) {
	if !viewport.visible(item.quadTransform(), item.width, item.height) {
		return
	}
	shape := item.snapshot[Sprite2D]().Shape
	// A zero-width border is invisible even when its computed color is opaque.
	// HTML controls retain that color with border:0 and still need the mask.
	if item.clip.hasRounded && shape.Kind == render.ShapeRoundedRect && shape.Radius <= 0 && shape.BorderWidth <= 0 && shape.Fill.A > 0 {
		texture := graphics.CreateTextureByColor(graphics.White)
		if texture != nil && texture.ValidFor(c.Device()) {
			model, textureModel, ok := clippedQuad2D(item.quadTransform(), mgl32.Ident3(), item.width, item.height, item.clip)
			if !ok {
				return
			}
			c.BindTexture(texture)
			// Match the former RGBA8 texel's quantization without retaining one
			// global texture (and device residency) for each animated color.
			fill := shape.Fill.GoColor()
			tint := graphics.ColorFromRGBA(fill.R, fill.G, fill.B, fill.A)
			c.DrawTintedMatricesCustom(model, textureModel, tint, item.snapshot[Sprite2D]().Alpha, roundedClipMaterial2D([4][4]float32{}, item.clip.rounded))
			return
		}
	}
	model, local, ok := clippedQuad2D(item.quadTransform(), mgl32.Ident3(), item.width, item.height, item.clip)
	if !ok {
		return
	}
	c.DrawShapeMatrices(
		model,
		local,
		item.width,
		item.height,
		shape.Kind,
		shape.Fill,
		shape.Border,
		shape.Radius,
		shape.BorderWidth,
		item.snapshot[Sprite2D]().Alpha,
	)
}

func drawParticleRenderItem2D(c *graphics.Canvas2D, item *renderItem2D, viewport renderViewport2D) {
	for _, particle := range item.snapshot[ParticleEmitter2D]().particles {
		alpha := item.snapshot[ParticleEmitter2D]().alphaFor(particle)
		if alpha <= 0 || particle.clr.A <= 0 {
			continue
		}
		model := particleModelMatrix(item.transform, particle, item.ui)
		if !viewport.visible(model, 1, 1) {
			continue
		}
		texture := particle.texture
		if texture == nil {
			texture = item.texture
		}
		model, clippedTexture, ok := clippedQuad2D(model, textureMatrix(texture), 1, 1, item.clip)
		if ok {
			c.BindTexture(texture)
			custom := item.materialData().Custom
			if item.clip.hasRounded {
				custom = roundedClipMaterial2D(custom, item.clip.rounded)
			}
			c.DrawTintedMatricesCustom(model, clippedTexture, particle.clr, alpha, custom)
		}
	}
	// The render phase may reuse this material for the following sprite.
	c.BindTexture(item.texture)
}

func drawTilemapRenderItem2D(c *graphics.Canvas2D, item *renderItem2D, viewport renderViewport2D) {
	item.drawTilemap(c, viewport)
}

func drawTextRenderItem2D(c *graphics.Canvas2D, item *renderItem2D, viewport renderViewport2D) {
	if !textRenderItemVisible2D(viewport, item) {
		return
	}
	item.drawText(c)
}

func (item renderItem2D) drawSprite(c *graphics.Canvas2D) {
	base := item.quadTransform()
	if item.snapshot[Sprite2D]().Slice == nil {
		width, height := item.width, item.height
		if item.texture.Trimmed() && item.texture.SourceWidth > 0 && item.texture.SourceHeight > 0 {
			scaleX := item.width / float32(item.texture.SourceWidth)
			scaleY := item.height / float32(item.texture.SourceHeight)
			base = base.Mul4(mgl32.Translate3D(float32(item.texture.OffsetX)*scaleX, float32(item.texture.OffsetY)*scaleY, 0))
			width = float32(item.texture.Width) * scaleX
			height = float32(item.texture.Height) * scaleY
		}
		model, texture, ok := clippedQuad2D(base, textureMatrix(item.texture), width, height, item.clip)
		if ok {
			item.drawSpriteMatrices(c, model, texture)
		}
		return
	}

	rasterScaleX, rasterScaleY := item.texture.RasterScale()
	xSpans, xCount := nineSliceAxisSpans(
		float32(item.texture.LogicalWidth()), float32(item.texture.OffsetX)/rasterScaleX, float32(item.texture.Width)/rasterScaleX,
		item.width, item.snapshot[Sprite2D]().Slice.Left, item.snapshot[Sprite2D]().Slice.Right,
	)
	ySpans, yCount := nineSliceAxisSpans(
		float32(item.texture.LogicalHeight()), float32(item.texture.OffsetY)/rasterScaleY, float32(item.texture.Height)/rasterScaleY,
		item.height, item.snapshot[Sprite2D]().Slice.Top, item.snapshot[Sprite2D]().Slice.Bottom,
	)
	baseTexture := textureMatrix(item.texture)
	for y := range yCount {
		for x := range xCount {
			xSpan, ySpan := xSpans[x], ySpans[y]
			model := base.Mul4(mgl32.Translate3D(xSpan.targetOffset, ySpan.targetOffset, 0))
			texture := baseTexture.
				Mul3(mgl32.Translate2D(xSpan.textureOffset, ySpan.textureOffset)).
				Mul3(mgl32.Scale2D(xSpan.textureSize, ySpan.textureSize))
			model, texture, ok := clippedQuad2D(model, texture, xSpan.targetSize, ySpan.targetSize, item.clip)
			if ok {
				item.drawSpriteMatrices(c, model, texture)
			}
		}
	}
}

func (item renderItem2D) drawSpriteMatrices(c *graphics.Canvas2D, model mgl32.Mat4, texture mgl32.Mat3) {
	custom := item.materialData().Custom
	if item.clip.hasRounded {
		custom = roundedClipMaterial2D(custom, item.clip.rounded)
	}
	c.DrawTintedMatricesCustom(model, texture, item.snapshot[Sprite2D]().Tint, item.snapshot[Sprite2D]().Alpha, custom)
}

func nineSliceAxis(sourceSize, targetSize, leading, trailing float32) ([4]float32, [4]float32) {
	source := [4]float32{0, leading, sourceSize - trailing, sourceSize}
	target := [4]float32{0, leading, targetSize - trailing, targetSize}
	fixed := leading + trailing
	if fixed > 0 && targetSize < fixed {
		scale := targetSize / fixed
		target[1] = leading * scale
		target[2] = target[1]
	}
	return source, target
}

type nineSliceAxisSpan struct {
	textureOffset float32
	textureSize   float32
	targetOffset  float32
	targetSize    float32
}

func nineSliceAxisSpans(logicalSize, packedOffset, packedSize, targetSize, leading, trailing float32) ([3]nineSliceAxisSpan, int) {
	var spans [3]nineSliceAxisSpan
	if logicalSize <= 0 || packedSize <= 0 || targetSize <= 0 {
		return spans, 0
	}
	source, target := nineSliceAxis(logicalSize, targetSize, leading, trailing)
	packedStart := max(float32(0), packedOffset)
	packedEnd := min(logicalSize, packedOffset+packedSize)
	count := 0
	for index := range 3 {
		sourceSize := source[index+1] - source[index]
		targetCellSize := target[index+1] - target[index]
		start := max(source[index], packedStart)
		end := min(source[index+1], packedEnd)
		if sourceSize <= 0 || targetCellSize <= 0 || end <= start {
			continue
		}
		targetStart := target[index] + (start-source[index])/sourceSize*targetCellSize
		targetEnd := target[index] + (end-source[index])/sourceSize*targetCellSize
		spans[count] = nineSliceAxisSpan{
			textureOffset: (start - packedOffset) / packedSize,
			textureSize:   (end - start) / packedSize,
			targetOffset:  targetStart,
			targetSize:    targetEnd - targetStart,
		}
		count++
	}
	return spans, count
}

type renderGlobalTransform2D interface {
	renderMatrix2D() mgl32.Mat4
	renderClip2D() clipRect2D
}

func collectSpriteTrails2D(c *graphics.Canvas2D, world *ecs.World, snapshots *renderSnapshots2D, hierarchy *renderHierarchy2DCache, items []renderItem2D, cull bool) []renderItem2D {
	items = snapshots.appendSpriteTrailRenderItems2D[GlobalTransform](world, hierarchy, items, renderViewport2DForSpace(c, world, false), cull, false)
	return snapshots.appendSpriteTrailRenderItems2D[UIGlobalTransform](world, hierarchy, items, renderViewport2DForSpace(c, world, true), cull, true)
}

func collectSprites2D(c *graphics.Canvas2D, world *ecs.World, snapshots *renderSnapshots2D, hierarchy *renderHierarchy2DCache, items []renderItem2D, cull bool) []renderItem2D {
	items = snapshots.appendSpriteRenderItems2D[GlobalTransform](world, hierarchy, items, renderViewport2DForSpace(c, world, false), cull, false)
	return snapshots.appendSpriteRenderItems2D[UIGlobalTransform](world, hierarchy, items, renderViewport2DForSpace(c, world, true), cull, true)
}

func collectSpriteLayers2D(c *graphics.Canvas2D, world *ecs.World, snapshots *renderSnapshots2D, hierarchy *renderHierarchy2DCache, items []renderItem2D, cull bool) []renderItem2D {
	items = snapshots.appendSpriteLayerRenderItems2D[GlobalTransform](world, hierarchy, items, renderViewport2DForSpace(c, world, false), cull, false)
	return snapshots.appendSpriteLayerRenderItems2D[UIGlobalTransform](world, hierarchy, items, renderViewport2DForSpace(c, world, true), cull, true)
}

func collectParticles2D(c *graphics.Canvas2D, world *ecs.World, snapshots *renderSnapshots2D, hierarchy *renderHierarchy2DCache, items []renderItem2D, cull bool) []renderItem2D {
	items = snapshots.appendParticleRenderItems2D[GlobalTransform](world, hierarchy, items, renderViewport2DForSpace(c, world, false), cull, false)
	return snapshots.appendParticleRenderItems2D[UIGlobalTransform](world, hierarchy, items, renderViewport2DForSpace(c, world, true), cull, true)
}

func collectTilemaps2D(c *graphics.Canvas2D, world *ecs.World, snapshots *renderSnapshots2D, hierarchy *renderHierarchy2DCache, items []renderItem2D, cull bool) []renderItem2D {
	items = snapshots.appendTilemapRenderItems2D[GlobalTransform](world, hierarchy, items, renderViewport2DForSpace(c, world, false), cull, false)
	return snapshots.appendTilemapRenderItems2D[UIGlobalTransform](world, hierarchy, items, renderViewport2DForSpace(c, world, true), cull, true)
}

func collectSpines2D(_ *graphics.Canvas2D, world *ecs.World, snapshots *renderSnapshots2D, hierarchy *renderHierarchy2DCache, items []renderItem2D, _ bool) []renderItem2D {
	items = snapshots.appendSpineRenderItems2D[GlobalTransform](world, hierarchy, items, false)
	return snapshots.appendSpineRenderItems2D[UIGlobalTransform](world, hierarchy, items, true)
}

func collectText2D(c *graphics.Canvas2D, world *ecs.World, snapshots *renderSnapshots2D, hierarchy *renderHierarchy2DCache, items []renderItem2D, cull bool) []renderItem2D {
	items = snapshots.appendTextRenderItems2D[GlobalTransform](c, world, hierarchy, items, renderViewport2DForSpace(c, world, false), cull, false)
	items = snapshots.appendTextRenderItems2D[UIGlobalTransform](c, world, hierarchy, items, renderViewport2DForSpace(c, world, true), cull, true)
	items = snapshots.appendOutputRenderItems2D[GlobalTransform](c, world, hierarchy, items, cull, false)
	return snapshots.appendOutputRenderItems2D[UIGlobalTransform](c, world, hierarchy, items, cull, true)
}

func (snapshots *renderSnapshots2D) appendSpriteTrailRenderItems2D[T renderGlobalTransform2D](world *ecs.World, hierarchy *renderHierarchy2DCache, items []renderItem2D, viewport renderViewport2D, cull, ui bool) []renderItem2D {
	for entity, access := range world.Query3[SpriteTrail2D, T, Visibility]().All {
		trail := access.A.Ref()
		visibility := access.C.Ref()

		if !visibility.Visible {
			continue
		}
		info := renderEntityInfo2D(world, entity, ui, 0, hierarchy)
		material := material2DForEntity(world, entity)
		for sampleIndex := range trail.samples {
			sample := &trail.samples[sampleIndex]
			if !renderItemVisible2D(viewport, sample.transform, sample.anchor, sample.width, sample.height, ui, sample.clip, cull) {
				continue
			}
			sprite := sample.sprite
			sprite.Alpha *= trail.alphaAt(sample.age)
			if !spriteDrawable2D(&sprite, graphics.DeviceFor(world)) {
				continue
			}
			item := newSpriteRenderItem2D(uint32(len(items)), entity, snapshots.addSprite(sprite), snapshots.addMaterial(material), sample.anchor, sample.transform, sample.clip, sample.width, sample.height, ui, info.sortIndex)
			item.entity, item.layers, item.target, item.scene, item.group = entity, info.layers, info.target, info.scene, info.group
			items = append(items, item)
		}
	}
	return items
}

func (snapshots *renderSnapshots2D) appendSpriteRenderItems2D[T renderGlobalTransform2D](world *ecs.World, hierarchy *renderHierarchy2DCache, items []renderItem2D, viewport renderViewport2D, cull, ui bool) []renderItem2D {
	for entity, access := range world.Query3[Sprite2D, T, Visibility]().All {
		sprite := access.A.Ref()
		transform := access.B.Ref()
		visibility := access.C.Ref()

		if sprite.Texture != nil {
		}
		if !visibility.Visible || !spriteDrawable2D(sprite, graphics.DeviceFor(world)) {
			continue
		}
		width, height := sprite.Size()
		width, height = renderSize2D(world, entity, width, height)
		if width <= 0 || height <= 0 {
			continue
		}
		matrix, clip := (*transform).renderMatrix2D(), (*transform).renderClip2D()
		anchor := anchorForEntity2D(world, entity)
		if !renderItemVisible2D(viewport, matrix, anchor, width, height, ui, clip, cull) {
			continue
		}
		info := renderEntityInfo2D(world, entity, ui, 0, hierarchy)
		item := newSpriteRenderItem2D(uint32(len(items)), entity, snapshots.capture(sprite, snapshots.addSprite), snapshots.addMaterial(material2DForEntity(world, entity)), anchor, matrix, clip, width, height, ui, info.sortIndex)
		item.entity, item.layers, item.target, item.scene, item.group = entity, info.layers, info.target, info.scene, info.group
		items = append(items, item)
	}
	return items
}

func (snapshots *renderSnapshots2D) appendSpriteLayerRenderItems2D[T renderGlobalTransform2D](world *ecs.World, hierarchy *renderHierarchy2DCache, items []renderItem2D, viewport renderViewport2D, cull, ui bool) []renderItem2D {
	for entity, access := range world.Query3[SpriteLayers2D, T, Visibility]().All {
		layers := access.A.Ref()
		transform := access.B.Ref()
		visibility := access.C.Ref()

		if layers == nil || transform == nil || visibility == nil || !visibility.Visible {
			continue
		}
		rootAlpha := float32(1)
		if root, ok := world.Get[Sprite2D](entity); ok {
			rootAlpha = root.Alpha
		}
		if rootAlpha <= 0 {
			continue
		}
		rootMatrix, clip := (*transform).renderMatrix2D(), (*transform).renderClip2D()
		info := renderEntityInfo2D(world, entity, ui, 0, hierarchy)
		material := material2DForEntity(world, entity)
		for index := range layers.Layers {
			layer := &layers.Layers[index]
			width, height := layer.Sprite.Size()
			if !layer.Visible || !spriteDrawable2D(&layer.Sprite, graphics.DeviceFor(world)) || width <= 0 || height <= 0 {
				continue
			}
			matrix := rootMatrix.Mul4(layers.LayerMatrix(index))
			if !renderItemVisible2D(viewport, matrix, layer.Anchor, width, height, ui, clip, cull) {
				continue
			}
			sprite := layer.Sprite
			sprite.Alpha *= rootAlpha
			if sprite.Alpha <= 0 {
				continue
			}
			item := newSpriteRenderItem2D(uint32(len(items)), entity, snapshots.addSprite(sprite), snapshots.addMaterial(material), layer.Anchor, matrix, clip, width, height, ui, info.sortIndex+layer.Order)
			item.entity, item.layers, item.target, item.scene, item.group = entity, info.layers, info.target, info.scene, info.group
			items = append(items, item)
		}
	}
	return items
}

func (snapshots *renderSnapshots2D) appendParticleRenderItems2D[T renderGlobalTransform2D](world *ecs.World, hierarchy *renderHierarchy2DCache, items []renderItem2D, _ renderViewport2D, _ bool, ui bool) []renderItem2D {
	var fallbackTexture *graphics.Texture
	for entity, access := range world.Query3[ParticleEmitter2D, T, Visibility]().All {
		emitter := access.A.Ref()
		transform := access.B.Ref()
		visibility := access.C.Ref()

		if !visibility.Visible {
			continue
		}
		if len(emitter.particles) == 0 {
			continue
		}
		matrix, clip := (*transform).renderMatrix2D(), (*transform).renderClip2D()
		if clip.empty() {
			continue
		}
		info := renderEntityInfo2D(world, entity, ui, int(DefaultParticleRenderOrder), hierarchy)
		if emitter.Texture == nil && fallbackTexture == nil {
			fallbackTexture = particleFallbackTexture2D(world)
		}
		item := newParticleRenderItem2D(uint32(len(items)), entity, snapshots.capture(emitter, snapshots.addEmitter), matrix, clip, ui, info.sortIndex, fallbackTexture)
		item.entity, item.layers, item.target, item.scene, item.group = entity, info.layers, info.target, info.scene, info.group
		items = append(items, item)
	}
	return items
}

func (snapshots *renderSnapshots2D) appendTilemapRenderItems2D[T renderGlobalTransform2D](world *ecs.World, hierarchy *renderHierarchy2DCache, items []renderItem2D, viewport renderViewport2D, cull, ui bool) []renderItem2D {
	for entity, access := range world.Query3[Tilemap2D, T, Visibility]().All {
		tilemap := access.A.Ref()
		transform := access.B.Ref()
		visibility := access.C.Ref()

		matrix, clip := (*transform).renderMatrix2D(), (*transform).renderClip2D()
		if !visibility.Visible || tilemap.Alpha <= 0 || !tilemap.ValidFor(graphics.DeviceFor(world)) || clip.empty() {
			continue
		}
		anchor := anchorForEntity2D(world, entity)
		if !renderItemVisible2D(viewport, matrix, anchor, tilemap.PixelWidth(), tilemap.PixelHeight(), ui, clip, cull) {
			continue
		}
		info := renderEntityInfo2D(world, entity, ui, 0, hierarchy)
		item := newTilemapRenderItem2D(uint32(len(items)), entity, snapshots.capture(tilemap, snapshots.addTilemap), anchor, matrix, clip, ui, info.sortIndex)
		item.entity, item.layers, item.target, item.scene, item.group = entity, info.layers, info.target, info.scene, info.group
		items = append(items, item)
	}
	return items
}

func (snapshots *renderSnapshots2D) appendSpineRenderItems2D[T renderGlobalTransform2D](world *ecs.World, hierarchy *renderHierarchy2DCache, items []renderItem2D, ui bool) []renderItem2D {
	for entity, access := range world.Query3[Spine2D, T, Visibility]().All {
		component := access.A.Ref()
		transform := access.B.Ref()
		visibility := access.C.Ref()

		if !visibility.Visible || component.Player == nil || component.Player.Alpha <= 0 || component.Player.Resource == nil {
			continue
		}
		matrix, clip := (*transform).renderMatrix2D(), (*transform).renderClip2D()
		info := renderEntityInfo2D(world, entity, ui, 0, hierarchy)
		item := renderItem2D{
			entity: entity, layers: info.layers, scene: info.scene, group: info.group, order: renderOrder2D(entity), sequence: uint32(len(items)),
			depth: matrix[14], ui: ui, sortIndex: info.sortIndex, material: renderMaterialKey{pipeline: render.PipelineMesh, textureID: component.Player.Resource.Texture.BatchKey()},
			kind: renderItemSpine2D, content: snapshots.capture(component, snapshots.addSpine), texture: component.Player.Resource.Texture, transform: matrix, clip: clip, target: info.target,
		}
		items = append(items, item)
	}
	return items
}

func (snapshots *renderSnapshots2D) appendTextRenderItems2D[T renderGlobalTransform2D](c *graphics.Canvas2D, world *ecs.World, hierarchy *renderHierarchy2DCache, items []renderItem2D, viewport renderViewport2D, cull, ui bool) []renderItem2D {
	_, canvasHeight := c.Size()
	for entity, access := range world.Query3[Text2D, T, Visibility]().All {
		textAccess := access.A
		transformAccess := access.B
		visibilityAccess := access.C

		text := textAccess.Mut()
		transform := transformAccess.Ref()
		visibility := visibilityAccess.Ref()
		if text == nil || transform == nil || visibility == nil {
			continue
		}
		matrix, clip := (*transform).renderMatrix2D(), (*transform).renderClip2D()
		anchor := anchorForEntity2D(world, entity)
		// Window before the paint gate as well: an invisible input layer still has
		// its caret and selection measured, and must not shape the whole document.
		text.setGlyphBand(text.visibleGlyphWindow(matrix, anchor, ui, clip, canvasHeight))
		if !visibility.Visible || text.Text().TrimSpaceEmpty() || text.Font == nil || !text.paintsVisibleGlyphs() {
			continue
		}
		text.rasterScale = textRasterScale(c)
		// A first layout has no size to window another anchor against, so measure
		// first and re-window from the size that pass establishes.
		if !text.ensureMeasuredLayout() {
			continue
		}
		text.setGlyphBand(text.visibleGlyphWindow(matrix, anchor, ui, clip, canvasHeight))
		if !text.ensureLayout() || text.cache == nil || text.cache.AtlasCount() == 0 {
			continue
		}
		width, height := text.layoutSize()
		if !renderItemVisible2D(viewport, matrix, anchor, width, height, ui, clip, cull) {
			continue
		}
		base := quadBaseTransform(matrix, anchor, width, height, ui)
		snapshot := snapshots.addText(*text)
		info := renderEntityInfo2D(world, entity, ui, 0, hierarchy)
		start := len(items)
		items = text.appendRenderItems(items, entity, base, clip, ui, info.sortIndex, snapshot)
		for index := start; index < len(items); index++ {
			items[index].entity = entity
			items[index].layers, items[index].scene, items[index].group = info.layers, info.scene, info.group
			items[index].ui = ui
			items[index].target = info.target
			items[index].width = width
			items[index].height = height
		}
	}
	return items
}

func DrawScreenText2D(c *graphics.Canvas2D, text *Text2D, transform mgl32.Mat4, anchor Anchor) bool {
	if c == nil || text == nil || text.Text().TrimSpaceEmpty() || text.Font == nil {
		return false
	}
	text.rasterScale = textRasterScale(c)
	if !text.ensureLayout() || text.cache == nil || text.cache.AtlasCount() == 0 {
		return false
	}

	width, height := text.layoutSize()
	base := quadBaseTransform(transform, anchor, width, height, true)
	scratch := c.ScratchValue[standaloneTextScratchTag, standaloneTextScratch]()
	scratch.text = text.paintSnapshot()
	scratch.layers = text.appendRenderItems(scratch.layers[:0], 0, base, clipRect2D{}, true, 0, &scratch.text)
	drawStandaloneTextLayers(c, scratch.layers)
	clear(scratch.layers)
	scratch.layers = scratch.layers[:0]
	scratch.text = textPaint2D{}
	return true
}

func MeasureText2D(text *Text2D) (float32, float32, bool) {
	if text == nil || text.Text().Len() == 0 && text.LineLayout == nil || text.Font == nil {
		return 0, 0, false
	}
	if !text.ensureMeasuredLayout() || text.face == nil {
		return 0, 0, false
	}
	width, height := text.layoutSize()
	return width, height, true
}

func MeasureTextMinimumWidth2D(text *Text2D) (float32, bool) {
	if text == nil || text.Text().TrimSpaceEmpty() || !text.ensureFace() {
		return 0, false
	}
	objects := text.validInlineObjects()
	minimum := float32(0)
	if text.WrapMode != TextWrapWords {
		for start := 0; start <= text.Text().Len(); {
			end := text.Text().Len()
			if offset := text.Text().IndexByte(start, '\n'); offset >= 0 {
				end = offset
			}
			minimum = max(minimum, textWidthWithInlineObjects(text.face, strings.TrimSuffix(text.Text().Slice(start, end), "\r"), start, objects, text.LetterSpacing, text.TabWidth))
			if end == text.Text().Len() {
				break
			}
			start = end + 1
		}
		return minimum, true
	}
	for start := 0; start < text.Text().Len(); {
		for start < text.Text().Len() && isTextWrapDelimiter(text.Text().ByteAt(start)) {
			start++
		}
		end := start
		for end < text.Text().Len() && !isTextWrapDelimiter(text.Text().ByteAt(end)) {
			_, size := text.Text().DecodeRune(end)
			end += size
		}
		if end > start {
			minimum = max(minimum, textWidthWithInlineObjects(text.face, text.Text().Slice(start, end), start, objects, text.LetterSpacing, text.TabWidth))
		}
		start = end
	}
	return minimum, true
}

func MeasureTextValue2D[V piecetree.Value](text *Text2D, inputValue V) (float32, float32, bool) {
	value := piecetree.Text(inputValue)
	if text == nil || !text.ensureMeasuredLayout() || text.face == nil {
		return 0, 0, false
	}
	if value.Equal(text.Text()) && text.measurementValid {
		return text.measuredWidth, text.measuredLineHeight, true
	}
	measuredWidth := float32(0)
	for start := 0; start <= value.Len(); {
		end := value.Len()
		if offset := value.IndexByte(start, '\n'); offset >= 0 {
			end = offset
		}
		measuredWidth = max(measuredWidth, textWidthWithInlineObjects(text.face, value.Slice(start, end), start, nil, text.LetterSpacing, text.TabWidth))
		if end == value.Len() {
			break
		}
		start = end + 1
	}
	measuredLineHeight := fixed26_6ToFloat32(text.face.Metrics().Height)
	if text.LineHeight > 0 {
		measuredLineHeight = text.LineHeight
	}
	if value.Equal(text.Text()) {
		text.measuredWidth = measuredWidth
		text.measuredLineHeight = measuredLineHeight
		text.measurementValid = true
	}
	return measuredWidth, measuredLineHeight, true
}

// MeasureTextLineOffsets2D writes prefix widths for sorted UTF-8 byte offsets
// in a single pass. Callers supply storage so repeated measurements allocate
// nothing, including lines with many highlighted ranges.
func MeasureTextLineOffsets2D(text *Text2D, value string, offsets []int, widths []float32) bool {
	if text == nil || len(widths) < len(offsets) || !text.ensureMeasuredLayout() || text.face == nil {
		return false
	}
	advance := textAdvance2D{previous: -1}
	index := 0
	for slot, offset := range offsets {
		if offset < index || offset > len(value) {
			return false
		}
		for index < offset {
			character, size := utf8.DecodeRuneInString(value[index:])
			if index+size > offset || character == '\n' {
				return false
			}
			if character != '\r' {
				advance.add(text.face, character, nil, text.LetterSpacing, text.TabWidth)
			}
			index += size
		}
		widths[slot] = advance.width
	}
	return true
}

func MeasureTextRangeRects2D(text *Text2D, start, end int) ([]TextRangeRect2D, bool) {
	if text == nil || start < 0 || start >= end || end > text.Text().Len() || !text.ensureLayout() || text.face == nil {
		return nil, false
	}
	if len(text.lines) == 0 {
		return nil, false
	}
	type lineBounds struct {
		left  float32
		right float32
		set   bool
	}
	bounds := make([]lineBounds, len(text.lines))
	for _, glyph := range text.glyphs {
		byteIndex := int(glyph.byteIndex)
		if byteIndex < start || byteIndex >= end {
			continue
		}
		line := int(glyph.line)
		left := glyph.x
		right := glyph.x + glyph.logicalWidth
		if !bounds[line].set || left < bounds[line].left {
			bounds[line].left = left
		}
		if !bounds[line].set || right > bounds[line].right {
			bounds[line].right = right
		}
		bounds[line].set = true
	}
	for _, object := range text.inlineObjectLayouts {
		if object.object.End <= start || object.object.Start >= end {
			continue
		}
		line := object.line
		left := object.rect.X
		right := object.rect.X + object.rect.Width
		if !bounds[line].set || left < bounds[line].left {
			bounds[line].left = left
		}
		if !bounds[line].set || right > bounds[line].right {
			bounds[line].right = right
		}
		bounds[line].set = true
	}
	result := make([]TextRangeRect2D, 0, len(bounds))
	for line, bound := range bounds {
		if !bound.set || bound.right <= bound.left {
			continue
		}
		result = append(result, TextRangeRect2D{X: bound.left, Y: text.lines[line].y, Width: bound.right - bound.left, Height: text.lines[line].height})
	}
	return result, len(result) > 0
}

func TextInlineObjectRects2D(text *Text2D) ([]TextInlineObjectRect2D, bool) {
	if text == nil || !text.ensureMeasuredLayout() || len(text.inlineObjectLayouts) == 0 {
		return nil, false
	}
	result := make([]TextInlineObjectRect2D, len(text.inlineObjectLayouts))
	for index, object := range text.inlineObjectLayouts {
		result[index] = TextInlineObjectRect2D{Start: object.object.Start, End: object.object.End, Rect: object.rect}
	}
	return result, true
}

// TextLineCount2D returns the number of visual lines without allocating.
func TextLineCount2D(text *Text2D) int {
	if text == nil || !text.ensureMeasuredLayout() {
		return 0
	}
	return len(text.lines)
}

// TextLineRange2D exposes byte offsets and geometry of one cached visual line.
func TextLineRange2D(text *Text2D, index int) (start, end int, rect TextRangeRect2D, ok bool) {
	if text == nil || !text.ensureMeasuredLayout() || index < 0 || index >= len(text.lines) {
		return 0, 0, TextRangeRect2D{}, false
	}
	line := text.lines[index]
	return line.start, line.end, TextRangeRect2D{X: line.x, Y: line.y, Width: line.width, Height: line.height}, true
}

// AppendTextUnderlineRects2D appends baseline-aligned underline segments for a
// byte range on one visual line, leaving gaps around ink crossing the underline.
// The returned bounds include those gaps. Reusing dst avoids allocations.
func AppendTextUnderlineRects2D(dst []TextRangeRect2D, text *Text2D, lineIndex, start, end int, thickness float32) ([]TextRangeRect2D, TextRangeRect2D, bool) {
	if text == nil || thickness <= 0 || !text.ensureMeasuredLayout() || text.face == nil || lineIndex < 0 || lineIndex >= len(text.lines) {
		return dst, TextRangeRect2D{}, false
	}
	line := text.lines[lineIndex]
	start, end = max(start, line.start), min(end, line.end)
	if start >= end {
		return dst, TextRangeRect2D{}, false
	}
	y := line.baseline + text.InkOffsetY + thickness
	pen, left, cursor := line.x, line.x, line.x
	previous, hasItem := rune(-1), false
	for index, character := range text.Text().Slice(line.start, end) {
		index += line.start
		if character == '\r' {
			continue
		}
		if hasItem {
			pen += text.LetterSpacing
		}
		if previous >= 0 && character != '\t' {
			pen += fixed26_6ToFloat32(text.face.Kern(previous, character))
		}
		if index == start {
			left, cursor = pen, pen
		}
		if character == '\t' {
			pen += textTabAdvance(pen-line.x, text.TabWidth)
			previous, hasItem = -1, true
			continue
		}
		ink, advance, ok := text.face.GlyphBounds(character)
		if index >= start && ok && !ink.Empty() && float32(ink.Min.Y) <= thickness*2 && float32(ink.Max.Y) > thickness {
			gapLeft, gapRight := pen+float32(ink.Min.X)-thickness, pen+float32(ink.Max.X)+thickness
			if gapLeft > cursor {
				dst = append(dst, TextRangeRect2D{X: cursor, Y: y, Width: gapLeft - cursor, Height: thickness})
			}
			cursor = max(cursor, gapRight)
		}
		pen += fixed26_6ToFloat32(advance)
		previous, hasItem = character, true
	}
	if pen > cursor {
		dst = append(dst, TextRangeRect2D{X: cursor, Y: y, Width: pen - cursor, Height: thickness})
	}
	return dst, TextRangeRect2D{X: left, Y: y, Width: pen - left, Height: thickness}, true
}

func TextLineRects2D(text *Text2D) ([]TextRangeRect2D, bool) {
	if text == nil || !text.ensureMeasuredLayout() || len(text.lines) == 0 {
		return nil, false
	}
	result := make([]TextRangeRect2D, len(text.lines))
	for index, line := range text.lines {
		result[index] = TextRangeRect2D{X: line.x, Y: line.y, Width: line.width, Height: line.height}
	}
	return result, true
}

// TextCaretRect2D returns the local caret rectangle for a rune index. It uses
// the text layout's actual line ranges, so wrapped lines and explicit newlines
// share the same geometry as rendering and hit testing.
func TextCaretRect2D[V piecetree.Value](text *Text2D, inputValue V, caret int) (x, y, width, height float32, ok bool) {
	value := piecetree.Text(inputValue)
	if text == nil || text.Font == nil || !text.ensureMeasuredLayout() || text.face == nil {
		return 0, 0, 0, 0, false
	}
	caret = max(0, caret)
	if value.Equal(text.Text()) && len(text.lines) > 0 {
		byteIndex := text.caretByteIndex(value, caret)
		lineIndex := min(sort.Search(len(text.lines), func(index int) bool { return byteIndex <= text.lines[index].end }), len(text.lines)-1)
		line := text.lines[lineIndex]
		prefixEnd := min(max(byteIndex, line.start), line.end)
		if len(text.inlineObjectLayouts) > 0 {
			advance := textAdvance2D{previous: -1}
			objectIndex := sort.Search(len(text.inlineObjectLayouts), func(i int) bool { return text.inlineObjectLayouts[i].object.End > line.start })
			cursor := value.Cursor(line.start, prefixEnd)
			for cursor.Offset() < prefixEnd {
				position := cursor.Offset()
				character, size := cursor.DecodeRune()
				var object *TextInlineObject2D
				if objectIndex < len(text.inlineObjectLayouts) && text.inlineObjectLayouts[objectIndex].object.Start == position {
					object = &text.inlineObjectLayouts[objectIndex].object
					size = min(object.End, prefixEnd) - position
					objectIndex++
				}
				advance.add(text.face, character, object, text.LetterSpacing, text.TabWidth)
				cursor.Advance(size)
			}
			return line.x + advance.width, line.y, 1, line.height, true
		}
		prefixWidth, _, measured := MeasureTextValue2D(text, value.Slice(line.start, prefixEnd))
		if !measured {
			return 0, 0, 0, 0, false
		}
		return line.x + prefixWidth, line.y, 1, line.height, true
	}

	byteIndex := value.ByteOffset(caret)
	line, _ := value.Position(byteIndex)
	lineStart := value.LineStart(line)
	prefixWidth, lineHeight, measured := MeasureTextValue2D(text, value.Slice(lineStart, byteIndex))
	if !measured {
		return 0, 0, 0, 0, false
	}
	return prefixWidth, float32(line) * lineHeight, 1, lineHeight, true
}

func (text *Text2D) caretByteIndex(value piecetree.Snapshot, caret int) int {
	if text.Index != nil && text.Index.MatchesSnapshot(value) {
		return text.Index.ByteOffset(caret)
	}
	return value.ByteOffset(caret)
}

func TextCaretIndex2D[V piecetree.Value](text *Text2D, inputValue V, x, y float32) int {
	value := piecetree.Text(inputValue)
	if text == nil || text.Font == nil || !text.ensureMeasuredLayout() || text.face == nil {
		return 0
	}
	lineHeight := int(text.face.Metrics().Height >> 6)
	if lineHeight <= 0 {
		lineHeight = 1
	}
	targetLine := max(0, int(y/float32(lineHeight)))
	layoutMatches := value.Equal(text.Text())
	if layoutMatches && len(text.lines) > 0 {
		targetLine = min(sort.Search(len(text.lines), func(index int) bool { return y < text.lines[index].y+text.lines[index].height }), len(text.lines)-1)
	}
	lineStartByte, lineEndByte := 0, value.Len()
	if layoutMatches && targetLine < len(text.lines) {
		lineStartByte = min(text.lines[targetLine].start, value.Len())
		lineEndByte = min(text.lines[targetLine].end, value.Len())
		x -= text.lines[targetLine].x
	} else {
		lineStartByte = value.LineStart(targetLine)
		if end := value.IndexByte(lineStartByte, '\n'); end >= 0 {
			lineEndByte = end
		}
	}

	advance := float32(0)
	previous := rune(-1)
	index := value.RuneOffset(lineStartByte)
	hasItem := false
	objects := []TextInlineObject2D(nil)
	if layoutMatches {
		objects = text.validInlineObjects()
	}
	objectIndex, skipUntil := 0, 0
	for objectIndex < len(objects) && objects[objectIndex].End <= lineStartByte {
		objectIndex++
	}
	cursor := value.Cursor(lineStartByte, lineEndByte)
	for cursor.Offset() < lineEndByte {
		globalIndex := cursor.Offset()
		r, size := cursor.DecodeRune()
		cursor.Advance(size)
		if globalIndex < skipUntil {
			continue
		}
		if r == '\r' {
			index++
			continue
		}
		previousX := advance
		if objectIndex < len(objects) && objects[objectIndex].Start == globalIndex {
			object := objects[objectIndex]
			if hasItem {
				advance += text.LetterSpacing
			}
			advance += object.Width
			if x < (previousX+advance)*0.5 {
				return index
			}
			index += value.RuneOffset(object.End) - value.RuneOffset(object.Start)
			skipUntil = object.End
			objectIndex++
			previous = -1
			hasItem = true
			continue
		}
		if hasItem {
			advance += text.LetterSpacing
		}
		if r == '\t' {
			advance += textTabAdvance(advance, text.TabWidth)
			previous = -1
		} else {
			if previous >= 0 {
				advance += fixed26_6ToFloat32(text.face.Kern(previous, r))
			}
			glyphAdvance, _ := text.face.GlyphAdvance(r)
			advance += fixed26_6ToFloat32(glyphAdvance)
			previous = r
		}
		nextX := advance
		if x < (previousX+nextX)*0.5 {
			return index
		}
		hasItem = true
		index++
	}
	return index
}

func SpriteBounds2D(sprite *Sprite2D, anchor Anchor, transform mgl32.Mat4, ui bool) (float32, float32, float32, float32, bool) {
	if sprite == nil {
		return 0, 0, 0, 0, false
	}
	width, height := sprite.Size()
	return boundsForSpace2D(transform, anchor, width, height, ui)
}

func TextBounds2D(text *Text2D, anchor Anchor, transform mgl32.Mat4, ui bool) (float32, float32, float32, float32, bool) {
	width, height, ok := MeasureText2D(text)
	if !ok {
		return 0, 0, 0, 0, false
	}
	return boundsForSpace2D(transform, anchor, width, height, ui)
}

func Bounds2D(transform mgl32.Mat4, anchor Anchor, width, height float32) (float32, float32, float32, float32, bool) {
	return boundsFromBase2D(quadBaseTransform(transform, anchor, width, height, false), width, height)
}

func BoundsInSpace2D(transform mgl32.Mat4, anchor Anchor, width, height float32, ui bool) (float32, float32, float32, float32, bool) {
	return boundsForSpace2D(transform, anchor, width, height, ui)
}

func boundsForSpace2D(transform mgl32.Mat4, anchor Anchor, width, height float32, ui bool) (float32, float32, float32, float32, bool) {
	return boundsFromBase2D(quadBaseTransform(transform, anchor, width, height, ui), width, height)
}

func boundsFromBase2D(transform mgl32.Mat4, width, height float32) (float32, float32, float32, float32, bool) {
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0, false
	}
	topLeft := transform.Mul4x1(mgl32.Vec4{0, 0, 0, 1})
	topRight := transform.Mul4x1(mgl32.Vec4{width, 0, 0, 1})
	bottomLeft := transform.Mul4x1(mgl32.Vec4{0, height, 0, 1})
	bottomRight := transform.Mul4x1(mgl32.Vec4{width, height, 0, 1})

	return min(topLeft.X(), topRight.X(), bottomLeft.X(), bottomRight.X()),
		min(topLeft.Y(), topRight.Y(), bottomLeft.Y(), bottomRight.Y()),
		max(topLeft.X(), topRight.X(), bottomLeft.X(), bottomRight.X()),
		max(topLeft.Y(), topRight.Y(), bottomLeft.Y(), bottomRight.Y()),
		true
}

type renderViewport2D struct {
	clipFromLayer mgl32.Mat4
	active        bool
}

func renderViewport2DForSpace(c *graphics.Canvas2D, world *ecs.World, ui bool) renderViewport2D {
	width, height := c.Size()
	if ui {
		return renderViewport2D{
			clipFromLayer: mgl32.Ortho2D(0, width, height, 0),
			active:        width > 0 && height > 0,
		}
	}
	camera, projection, transform, ok := activeCamera2DComponents(world)
	if !ok {
		return renderViewport2D{}
	}
	return renderViewport2D{clipFromLayer: camera.ClipFromWorld(transform, projection), active: true}
}

func (v renderViewport2D) visible(transform mgl32.Mat4, width, height float32) bool {
	if !v.active || width <= 0 || height <= 0 {
		return false
	}
	return v.visibleBase(transform, width, height)
}

func (v renderViewport2D) visibleBase(transform mgl32.Mat4, width, height float32) bool {
	if !v.active || width <= 0 || height <= 0 {
		return false
	}
	minX, minY, maxX, maxY, ok := boundsFromBase2D(v.clipFromLayer.Mul4(transform), width, height)
	if !ok {
		return false
	}
	return maxX >= -1 && maxY >= -1 && minX <= 1 && minY <= 1
}

type renderItemKind2D uint8

const (
	renderItemSprite2D renderItemKind2D = iota
	renderItemShape2D
	renderItemParticle2D
	renderItemTilemap2D
	renderItemTextLayer2D
	renderItemSpine2D
	renderItemKind2DCount
)

type renderItem2D struct {
	content    any // a typed pointer to exactly one owned or borrowed snapshot
	paint      *paintItem2D
	texture    *graphics.Texture
	material2D *graphics.Material2DData
	layers     RenderLayers
	scene      SceneLayer
	material   renderMaterialKey
	glyphStart int
	order      uint64
	sortIndex  int
	group      uint64
	entity     ecs.Entity
	target     ecs.Entity
	glyphEnd   int
	transform  mgl32.Mat4
	clip       clipRect2D
	color      graphics.ColorRGBA
	anchor     Anchor
	width      float32
	height     float32
	sequence   uint32
	depth      float32
	outlined   bool
	lcdText    bool
	kind       renderItemKind2D
	ui         bool
}

// snapshot reads the active payload without allocating or retaining ECS storage.
func (item renderItem2D) snapshot[T any]() *T { value, _ := item.content.(*T); return value }

func (item *renderItem2D) materialData() graphics.Material2DData {
	if item.material2D != nil {
		return *item.material2D
	}
	return graphics.Material2DData{}
}

func spriteDrawable2D(sprite *Sprite2D, device *render.Device) bool {
	if sprite == nil || sprite.Alpha <= 0 {
		return false
	}
	if sprite.Shape.Kind != 0 {
		return sprite.Shape.Fill.A > 0 || sprite.Shape.Border.A > 0
	}
	return sprite.Tint.A > 0 && sprite.Texture != nil && sprite.Texture.ValidFor(device)
}

// Size resolves logical dimensions, using texture dimensions for omitted axes.
func (s *Sprite2D) Size() (width, height float32) {
	if s == nil {
		return 0, 0
	}
	if s.Width > 0 {
		width = s.Width
	} else if s.Texture != nil {
		width = float32(s.Texture.LogicalWidth())
	}
	if s.Height > 0 {
		height = s.Height
	} else if s.Texture != nil {
		height = float32(s.Texture.LogicalHeight())
	}
	return width, height
}

func (t *Text2D) shadowEnabled() bool {
	return t != nil && t.ShadowColor.A > 0
}

func (t *Text2D) outlineEnabled() bool {
	return t != nil && validOutlineWidth(t.OutlineWidth) && t.OutlineColor.A > 0
}

func validOutlineWidth(width float32) bool {
	return width > 0 && !math.IsNaN(float64(width)) && !math.IsInf(float64(width), 0)
}

// paintsVisibleGlyphs reports whether any run of the text would reach the painter.
func (t *Text2D) paintsVisibleGlyphs() bool {
	// A fully transparent color with no shadow or outline, such as the invisible
	// input layer behind a highlighted code view, paints nothing.
	if t.Alpha <= 0 {
		return false
	}
	if t.Color.A > 0 || t.shadowEnabled() || t.outlineEnabled() {
		return true
	}
	for _, span := range t.Spans {
		if span.Color.A > 0 {
			return true
		}
	}
	return false
}

// resetLayoutBuffer gives a rebuild a buffer sized to the records it expects.
// It keeps retained capacity only while that fits, so a narrowed band releases
// a document-sized buffer and a widened one refits once instead of cascading.
func resetLayoutBuffer[T any](buffer []T, incoming int) []T {
	if cap(buffer) > incoming*2+4096 || cap(buffer) < incoming {
		return make([]T, 0, incoming)
	}
	return buffer[:0]
}

// trimLayoutBuffer releases a previous window's peak after the current
// traversal has established how many records the replacement actually needs.
func trimLayoutBuffer[T any](buffer []T) []T {
	if cap(buffer) > len(buffer)*2+4096 {
		return slices.Clone(buffer)
	}
	return buffer
}

// layoutLineSpan bounds the lines and wrap ranges the next layout produces.
// Wrapped text can split any line, so its previous pass is the tighter bound.
func (t *Text2D) layoutLineSpan(value piecetree.Snapshot) int {
	span := value.Lines()
	if t.WrapMode != TextWrapNone {
		span = max(span, t.lineCount)
	}
	return span
}

// textGlyphBand is the vertical window, in the text's own units, whose lines get
// glyph records. An unset band shapes the whole value.
type textGlyphBand struct {
	top    float32
	bottom float32
	valid  bool
}

// glyphBandOverscan is extra window heights shaped around the visible range, so
// scrolling only re-shapes once the view leaves the region already shaped.
const glyphBandOverscan = 2

// visibleGlyphWindow maps the layer-space window a UI text paints into onto its
// local vertical range. Rotated text needs both axes, so it stays unwindowed.
func (t *Text2D) visibleGlyphWindow(matrix mgl32.Mat4, anchor Anchor, ui bool, clip clipRect2D, canvasHeight float32) textGlyphBand {
	const epsilon = 1e-5
	width, height := t.layoutSize()
	// Top-left anchored text does not shift with its own size, so it windows
	// before the first layout; another anchor needs the laid-out height.
	if !ui || anchor != AnchorTopLeft && height <= 0 || canvasHeight <= 0 || absFloat32(matrix[1]) > epsilon || absFloat32(matrix[5]) <= epsilon {
		return textGlyphBand{}
	}
	base := quadBaseTransform(matrix, anchor, width, height, ui)
	top, bottom := float32(0), canvasHeight
	if clip.valid {
		top, bottom = max(top, clip.minY), min(bottom, clip.maxY)
	}
	scale, origin := base[5], base[13]
	band := textGlyphBand{top: (top - origin) / scale, bottom: (bottom - origin) / scale, valid: true}
	if band.top > band.bottom {
		band.top, band.bottom = band.bottom, band.top
	}
	return band
}

// setGlyphBand windows glyph generation to a visible range, keeping the current
// band until the window escapes it so scrolling does not re-shape every frame.
func (t *Text2D) setGlyphBand(window textGlyphBand) {
	if !window.valid || t.glyphBand.valid && window.top >= t.glyphBand.top && window.bottom <= t.glyphBand.bottom {
		return
	}
	margin := (window.bottom - window.top) * glyphBandOverscan
	t.glyphBand = textGlyphBand{top: window.top - margin, bottom: window.bottom + margin, valid: true}
}

// Natural font metrics bound the paint window even when explicit line height
// is smaller. Full layout and window rebuild use the same bound.
func (t *Text2D) inGlyphBand(lineTop float32) bool {
	if !t.glyphBand.valid {
		return true
	}
	return lineTop+t.glyphLineHeight >= t.glyphBand.top && lineTop <= t.glyphBand.bottom
}

func (t *Text2D) glyphLineRange() (int, int) {
	if !t.glyphBand.valid {
		return 0, len(t.lines)
	}
	start := sort.Search(len(t.lines), func(index int) bool {
		line := t.lines[index]
		return line.y+t.glyphLineHeight >= t.glyphBand.top
	})
	end := start
	for end < len(t.lines) && t.lines[end].y <= t.glyphBand.bottom {
		end++
	}
	return start, end
}

// Reuse line breaks, alignment and baselines when only the paint window changes.
func (t *Text2D) rebuildGlyphWindow() {
	value := t.layoutValue
	rasterScale := normalizeTextRasterScale(t.rasterScale)
	logicalScale := float32(rasterScale)
	start, end := t.glyphLineRange()
	records := 0
	if start < end {
		// UTF-8 bytes bound the glyph count without a separate rune scan.
		records = t.lines[end-1].end - t.lines[start].start
	}
	t.glyphs = resetLayoutBuffer(t.glyphs, records)
	if validOutlineWidth(t.OutlineWidth) {
		t.outlineGlyphs = resetLayoutBuffer(t.outlineGlyphs, records)
	} else {
		t.outlineGlyphs = nil
	}
	objectIndex := 0
	if start < end {
		objectIndex = sort.Search(len(t.inlineObjectLayouts), func(index int) bool {
			return t.inlineObjectLayouts[index].object.Start >= t.lines[start].start
		})
	}
	for lineIndex := start; lineIndex < end; lineIndex++ {
		line := t.lines[lineIndex]
		pen, previous := float32(0), rune(-1)
		hasItem := false
		skipUntil := 0
		cursor := value.Cursor(line.start, line.end)
		for cursor.Offset() < line.end {
			byteIndex := cursor.Offset()
			character, size := cursor.DecodeRune()
			cursor.Advance(size)
			if byteIndex < skipUntil || character == '\r' {
				continue
			}
			for objectIndex < len(t.inlineObjectLayouts) && t.inlineObjectLayouts[objectIndex].object.Start < byteIndex {
				objectIndex++
			}
			if hasItem {
				pen += t.LetterSpacing
			}
			if objectIndex < len(t.inlineObjectLayouts) && t.inlineObjectLayouts[objectIndex].object.Start == byteIndex {
				object := t.inlineObjectLayouts[objectIndex].object
				pen += object.Width
				skipUntil, previous, hasItem = object.End, -1, true
				objectIndex++
				continue
			}
			if character == '\t' {
				pen += textTabAdvance(pen, t.TabWidth)
				previous, hasItem = -1, true
				continue
			}
			pen += t.cache.Kern(previous, character) / logicalScale
			raster, reference := t.cache.GlyphWithReference(character)
			logical := logicalTextGlyph(raster, rasterScale)
			if logical.Visible() {
				if validOutlineWidth(t.OutlineWidth) {
					t.outlineGlyphs = append(t.outlineGlyphs, t.cache.OutlineWithReference(character, t.OutlineWidth*float32(rasterScale)))
				}
				t.glyphs = append(t.glyphs, textGlyph2D{
					raster: reference, logicalWidth: float32(raster.Width) / logicalScale, logicalHeight: float32(raster.Height) / logicalScale,
					x: pen + logical.OffsetX + line.x, y: logical.OffsetY + line.baseline,
					byteIndex: int32(byteIndex), line: int32(lineIndex),
				})
			}
			pen += logical.Advance
			previous, hasItem = character, true
		}
	}
	t.glyphlessLayout = false
	t.lastGlyphBand = t.glyphBand
	t.cache.FlushUploads()
}

func (t *Text2D) invalidateFace() {
	t.changed = true
	t.cache = nil
	t.glyphs = nil
	t.outlineGlyphs = nil
	t.measurementValid = false
	if t.face != nil {
		_ = t.face.Close()
		t.face = nil
	}
}

func normalizeTextRasterScale(scale float64) float64 {
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return 1
	}
	return scale
}

func textRasterScale(c *graphics.Canvas2D) float64 {
	if c == nil {
		return 1
	}
	return normalizeTextRasterScale(c.RasterScale())
}

func logicalTextGlyph(glyph onixtext.Glyph, rasterScale float64) onixtext.Glyph {
	if rasterScale == 1 {
		return glyph
	}
	width := int32(math.Round(float64(glyph.Width) / rasterScale))
	height := int32(math.Round(float64(glyph.Height) / rasterScale))
	if glyph.Width > 0 {
		width = max(width, 1)
	}
	if glyph.Height > 0 {
		height = max(height, 1)
	}
	glyph.Width, glyph.Height = width, height
	glyph.OffsetX /= float32(rasterScale)
	glyph.OffsetY /= float32(rasterScale)
	glyph.Advance /= float32(rasterScale)
	return glyph
}

func (t *Text2D) ensureFace() bool {
	if t.Font == nil {
		return false
	}
	if t.FontSize <= 0 {
		t.FontSize = 18
	}
	if t.DPI <= 0 {
		t.DPI = 72
	}
	if t.face == nil {
		t.face = t.Font.FontFace(t.FontSize, t.DPI)
	}
	return true
}

// ensureLayout shapes text into the glyph records the painter consumes.
func (t *Text2D) ensureLayout() bool { return t.buildLayout(true) }

// ensureMeasuredLayout computes line geometry and sizes, shaping glyphs only for
// a text whose visible window is already established.
func (t *Text2D) ensureMeasuredLayout() bool {
	return t.buildLayout(t.glyphBand.valid && t.paintsVisibleGlyphs())
}

func (t *Text2D) buildLayout(withGlyphs bool) bool {
	if t == nil {
		return false
	}
	if t.canUsePrelayout() {
		return t.buildPrelayout(withGlyphs)
	}
	if t.prelayoutActive {
		t.changed = true
		t.prelayoutActive = false
	}
	lineRevision := uint64(0)
	if t.LineLayout != nil {
		lineRevision = t.LineLayout.Revision
	}
	if t.LineLayout != t.lastLineLayout || lineRevision != t.lastLineLayoutRevision {
		t.changed = true
	}
	rasterScale := normalizeTextRasterScale(t.rasterScale)
	if t.Font != t.lastFont || t.FontSize != t.lastFontSize || t.DPI != t.lastDPI || rasterScale != t.lastRasterScale {
		t.invalidateFace()
	}
	if t.Text() != t.lastValue || t.WrapMode != t.lastWrapMode || t.WrapWidth != t.lastWrapWidth ||
		t.LetterSpacing != t.lastLetterSpacing || t.LineHeight != t.lastLineHeight || t.TabWidth != t.lastTabWidth || t.TextAlignment != t.lastTextAlignment ||
		t.MaxWidth != t.lastMaxWidth || t.OutlineWidth != t.lastOutlineWidth ||
		!slices.Equal(t.InlineObjects, t.lastInlineObjects) {
		t.changed = true
	}
	if t.Text() != t.lastValue || t.LetterSpacing != t.lastLetterSpacing || t.LineHeight != t.lastLineHeight || t.TabWidth != t.lastTabWidth {
		t.measurementValid = false
	}
	if !t.changed && t.cache != nil {
		if withGlyphs && (t.glyphlessLayout || t.glyphBand != t.lastGlyphBand) {
			t.rebuildGlyphWindow()
		}
		return true
	}
	if !t.ensureFace() {
		return false
	}
	t.cache = t.Font.GlyphCache(t.FontSize*rasterScale, t.DPI)
	objects := t.validInlineObjects()
	layoutValue := t.ellipsizedText()
	// A measure-only pass writes no glyph records, so it must not size a fresh
	// buffer for them; clearing keeps the shaped window's capacity for the paint.
	records := cap(t.glyphs)
	if withGlyphs && !t.glyphBand.valid {
		records = layoutValue.Runes()
	}
	lineSpan := t.layoutLineSpan(layoutValue)
	previousLines := t.lines
	reuseWidths := t.WrapMode == t.lastWrapMode && t.WrapWidth == t.lastWrapWidth &&
		t.MaxWidth <= 0 && t.lastMaxWidth <= 0 && len(objects) == 0 && len(t.lastInlineObjects) == 0 &&
		t.Font == t.lastFont && t.FontSize == t.lastFontSize && t.DPI == t.lastDPI && rasterScale == t.lastRasterScale &&
		t.LetterSpacing == t.lastLetterSpacing && t.TabWidth == t.lastTabWidth
	t.glyphs = resetLayoutBuffer(t.glyphs, records)
	if validOutlineWidth(t.OutlineWidth) {
		t.outlineGlyphs = resetLayoutBuffer(t.outlineGlyphs, records)
	} else {
		t.outlineGlyphs = nil
	}
	t.inlineObjectLayouts = resetLayoutBuffer(t.inlineObjectLayouts, len(objects))
	t.lines = resetLayoutBuffer(t.lines, lineSpan)
	t.glyphlessLayout = !withGlyphs

	metrics := t.face.Metrics()
	lineHeight := fixed26_6ToFloat32(metrics.Height)
	fontAscent := fixed26_6ToFloat32(metrics.Ascent)
	fontMetricsDescent := fixed26_6ToFloat32(metrics.Descent)
	fontBaseline := fontAscent + (lineHeight-fontAscent-fontMetricsDescent)*.5
	fontDescent := lineHeight - fontBaseline
	t.glyphLineHeight = max(fontBaseline+fontDescent, t.LineHeight)
	pen := float32(0)
	prev := rune(-1)
	line := 0
	lineTop := float32(0)
	lineAscent, lineDescent := fontBaseline, fontDescent
	lineGlyphStart, lineObjectStart := 0, 0
	lineVisualWidth := float32(0)
	lineHasItem := false
	lineStart, lineEnd := 0, 0
	t.layoutWidth, t.layoutHeight = 0, 0
	finishLine := func() {
		naturalHeight := lineAscent + lineDescent
		height := naturalHeight
		if t.LineHeight > 0 {
			height = t.LineHeight
		}
		verticalOffset := (height - naturalHeight) * .5
		for index := lineGlyphStart; index < len(t.glyphs); index++ {
			t.glyphs[index].y += lineTop + lineAscent + verticalOffset
		}
		for index := lineObjectStart; index < len(t.inlineObjectLayouts); index++ {
			t.inlineObjectLayouts[index].rect.Y += lineTop + lineAscent + verticalOffset
		}
		t.lines = append(t.lines, textLineLayout2D{baseline: lineTop + lineAscent + verticalOffset, y: lineTop, width: lineVisualWidth, height: height, advance: pen, start: lineStart, end: lineEnd})
		t.layoutWidth = max(t.layoutWidth, lineVisualWidth)
		lineTop += height
		line++
		pen = 0
		prev = -1
		lineAscent, lineDescent = fontBaseline, fontDescent
		lineGlyphStart, lineObjectStart = len(t.glyphs), len(t.inlineObjectLayouts)
		lineVisualWidth = 0
		lineHasItem = false
	}
	objectIndex, skipUntil := 0, 0
	layoutRange := func(textRange textWrapRange2D) {
		if t.LineLayout != nil && line < len(t.LineLayout.Offsets) {
			lineTop = max(lineTop, t.LineLayout.Offsets[line])
		}
		shapeGlyphs := withGlyphs && t.inGlyphBand(lineTop)
		if shapeGlyphs && t.glyphBand.valid {
			// Current line bytes bound its glyph count. Old document offsets
			// cannot predict a replacement line's density.
			count := textRange.end - textRange.start
			t.glyphs = slices.Grow(t.glyphs, count)
			if validOutlineWidth(t.OutlineWidth) {
				t.outlineGlyphs = slices.Grow(t.outlineGlyphs, count)
			}
		}
		lineStart, lineEnd = textRange.start, textRange.end
		// Keep offscreen, unchanged line metrics across edits. Read this slot
		// before finishLine overwrites it when both slices share their storage.
		if reuseWidths && !shapeGlyphs && line < len(previousLines) {
			previous := previousLines[line]
			if layoutValue.EqualRange(lineStart, lineEnd, t.lastValue, previous.start, previous.end) {
				lineVisualWidth, pen = previous.width, previous.advance
				return
			}
		}
		cursor := layoutValue.Cursor(textRange.start, textRange.end)
		for cursor.Offset() < textRange.end {
			byteIndex := cursor.Offset()
			r, size := cursor.DecodeRune()
			cursor.Advance(size)
			if byteIndex < skipUntil {
				continue
			}
			if r == '\r' {
				continue
			}
			for objectIndex < len(objects) && objects[objectIndex].Start < byteIndex {
				objectIndex++
			}
			if objectIndex < len(objects) && objects[objectIndex].Start == byteIndex && objects[objectIndex].End <= layoutValue.Len() && layoutValue.EqualRange(objects[objectIndex].Start, objects[objectIndex].End, t.Text(), objects[objectIndex].Start, objects[objectIndex].End) {
				object := objects[objectIndex]
				if lineHasItem {
					pen += t.LetterSpacing
				}
				t.inlineObjectLayouts = append(t.inlineObjectLayouts, textInlineObjectLayout2D{
					object: object,
					rect:   TextRangeRect2D{X: pen, Y: -object.Baseline, Width: object.Width, Height: object.Height},
					line:   line,
				})
				pen += object.Width
				lineVisualWidth = max(lineVisualWidth, pen)
				lineAscent = max(lineAscent, object.Baseline)
				lineDescent = max(lineDescent, object.Height-object.Baseline)
				prev = -1
				skipUntil = object.End
				objectIndex++
				lineHasItem = true
				continue
			}
			if lineHasItem {
				pen += t.LetterSpacing
			}
			if r == '\t' {
				pen += textTabAdvance(pen, t.TabWidth)
				lineVisualWidth = max(lineVisualWidth, pen)
				prev = -1
				lineHasItem = true
				continue
			}
			pen += t.cache.Kern(prev, r) / float32(rasterScale)
			var rasterGlyph onixtext.Glyph
			var reference onixtext.GlyphRef
			if shapeGlyphs {
				rasterGlyph, reference = t.cache.GlyphWithReference(r)
			} else {
				rasterGlyph = t.cache.GlyphMetrics(r)
			}
			g := logicalTextGlyph(rasterGlyph, rasterScale)
			if g.Visible() {
				logicalWidth := float32(rasterGlyph.Width) / float32(rasterScale)
				if shapeGlyphs {
					if validOutlineWidth(t.OutlineWidth) {
						t.outlineGlyphs = append(t.outlineGlyphs, t.cache.OutlineWithReference(r, t.OutlineWidth*float32(rasterScale)))
					}
					t.glyphs = append(t.glyphs, textGlyph2D{
						raster:        reference,
						logicalWidth:  logicalWidth,
						logicalHeight: float32(rasterGlyph.Height) / float32(rasterScale),
						x:             pen + g.OffsetX,
						y:             g.OffsetY,
						byteIndex:     int32(byteIndex),
						line:          int32(line),
					})
				}
			}
			pen += g.Advance
			// Layout uses advances; bitmap padding and ink overhangs belong only
			// to painting and must not change alignment at another raster scale.
			lineVisualWidth = max(lineVisualWidth, pen)
			prev = r
			lineHasItem = true
		}
	}
	layoutRanges := func(ranges []textWrapRange2D) {
		step := lineHeight
		if t.LineHeight > 0 {
			step = t.LineHeight
		}
		if withGlyphs && t.glyphBand.valid && len(objects) == 0 && step > 0 {
			// Plain text has constant line height. Its current wrap ranges give
			// the whole paint window's byte bound without another text scan.
			start := max(0, min(len(ranges), int((t.glyphBand.top-t.glyphLineHeight)/step)))
			start = max(0, start-1)
			end := min(len(ranges), max(0, min(len(ranges), int(t.glyphBand.bottom/step)))+2)
			count := 0
			if start < end {
				count = ranges[end-1].end - ranges[start].start
			}
			t.glyphs = resetLayoutBuffer(t.glyphs, count)
			if validOutlineWidth(t.OutlineWidth) {
				t.outlineGlyphs = resetLayoutBuffer(t.outlineGlyphs, count)
			}
		}
		for index, textRange := range ranges {
			if index > 0 {
				finishLine()
			}
			layoutRange(textRange)
		}
	}
	wrapWidth := t.WrapWidth
	if t.WrapMode == TextWrapWords && wrapWidth > 0 {
		layoutRanges(t.wrapRanges(layoutValue, objects))
	} else if (t.WrapMode == TextWrapCharacters || t.WrapMode == TextWrapTokens) && wrapWidth > 0 {
		layoutRanges(t.wrapPreservedRanges(layoutValue, objects, wrapWidth, reuseWidths))
	} else {
		lineStart := 0
		for lineStart <= layoutValue.Len() {
			lineEnd := layoutValue.Len()
			if offset := layoutValue.IndexByte(lineStart, '\n'); offset >= 0 {
				lineEnd = offset
			}
			if lineStart > 0 {
				finishLine()
			}
			layoutRange(textWrapRange2D{start: lineStart, end: lineEnd})
			if lineEnd == layoutValue.Len() {
				break
			}
			lineStart = lineEnd + 1
		}
	}
	finishLine()
	if withGlyphs && t.glyphBand.valid {
		t.glyphs = trimLayoutBuffer(t.glyphs)
		t.outlineGlyphs = trimLayoutBuffer(t.outlineGlyphs)
	}
	t.lineCount = len(t.lines)
	alignmentWidth := t.layoutWidth
	if t.WrapWidth > 0 {
		alignmentWidth = max(alignmentWidth, t.WrapWidth)
	}
	if t.TextAlignment != TextAlignLeft {
		for lineIndex := range t.lines {
			offset := alignmentWidth - t.lines[lineIndex].width
			if t.TextAlignment == TextAlignCenter {
				offset *= .5
			}
			t.lines[lineIndex].x = offset
		}
		for glyphIndex := range t.glyphs {
			t.glyphs[glyphIndex].x += t.lines[t.glyphs[glyphIndex].line].x
		}
		for objectIndex := range t.inlineObjectLayouts {
			object := &t.inlineObjectLayouts[objectIndex]
			object.rect.X += t.lines[object.line].x
		}
		t.layoutWidth = alignmentWidth
	}
	t.layoutHeight = lineTop
	if t.LineLayout != nil {
		t.layoutWidth = max(t.layoutWidth, t.LineLayout.Width)
		t.layoutHeight = max(t.layoutHeight, t.LineLayout.Height)
	}
	t.cache.FlushUploads()

	t.changed = false
	t.layoutValue = layoutValue
	t.lastValue = t.Text()
	t.lastFont = t.Font
	t.lastFontSize = t.FontSize
	t.lastDPI = t.DPI
	t.lastRasterScale = rasterScale
	t.lastWrapMode = t.WrapMode
	t.lastWrapWidth = t.WrapWidth
	t.lastLetterSpacing = t.LetterSpacing
	t.lastLineHeight = t.LineHeight
	t.lastTabWidth = t.TabWidth
	t.lastTextAlignment = t.TextAlignment
	t.lastMaxWidth = t.MaxWidth
	t.lastOutlineWidth = t.OutlineWidth
	t.lastLineLayout, t.lastLineLayoutRevision = t.LineLayout, lineRevision
	t.lastGlyphBand = t.glyphBand
	t.lastInlineObjects = append(resetLayoutBuffer(t.lastInlineObjects, len(t.InlineObjects)), t.InlineObjects...)
	return true
}

func (t *Text2D) canUsePrelayout() bool {
	if len(t.Prelayout.Glyphs) == 0 || t.Source != (piecetree.Snapshot{}) || t.WrapMode != TextWrapNone || t.WrapWidth != 0 || t.MaxWidth != 0 || t.LetterSpacing != 0 || t.TextAlignment != TextAlignLeft || len(t.InlineObjects) != 0 || t.LineLayout != nil {
		return false
	}
	return !t.prelayoutActive || t.lastValue == t.Text() || t.lastPrelayoutRevision != t.Prelayout.Revision
}

func (t *Text2D) buildPrelayout(withGlyphs bool) bool {
	rasterScale := normalizeTextRasterScale(t.rasterScale)
	if t.Font != t.lastFont || t.FontSize != t.lastFontSize || t.DPI != t.lastDPI || rasterScale != t.lastRasterScale {
		t.invalidateFace()
	}
	if t.prelayoutActive && !t.changed && t.cache != nil && t.lastPrelayoutRevision == t.Prelayout.Revision && t.lastValue == t.Text() && t.lastOutlineWidth == t.OutlineWidth && (!withGlyphs || !t.glyphlessLayout) {
		return true
	}
	if !t.ensureFace() {
		return false
	}
	t.cache = t.Font.GlyphCache(t.FontSize*rasterScale, t.DPI)
	t.layoutWidth, t.measuredWidth = t.Prelayout.Width, t.Prelayout.Width
	t.layoutHeight, t.measuredLineHeight = t.Prelayout.Height, t.Prelayout.Height
	t.measurementValid = true
	t.lineCount = 1
	baseline := float32(t.Prelayout.Glyphs[0].Y - t.Prelayout.OriginY)
	t.lines = append(resetLayoutBuffer(t.lines, 1), textLineLayout2D{
		baseline: baseline, width: t.layoutWidth, height: t.layoutHeight, advance: t.layoutWidth, end: t.Text().Len(),
	})
	t.glyphLineHeight = t.layoutHeight
	t.glyphs = resetLayoutBuffer(t.glyphs, len(t.Prelayout.Glyphs))
	if validOutlineWidth(t.OutlineWidth) {
		t.outlineGlyphs = resetLayoutBuffer(t.outlineGlyphs, len(t.Prelayout.Glyphs))
	} else {
		t.outlineGlyphs = nil
	}
	if withGlyphs {
		logicalScale := float32(rasterScale)
		byteIndex := 0
		for _, positioned := range t.Prelayout.Glyphs {
			currentByte := byteIndex
			if byteIndex < len(t.Value) {
				_, size := utf8.DecodeRuneInString(t.Value[byteIndex:])
				byteIndex += size
			}
			raster, reference := t.cache.GlyphWithReference(positioned.Rune)
			logical := logicalTextGlyph(raster, rasterScale)
			if !logical.Visible() {
				continue
			}
			if validOutlineWidth(t.OutlineWidth) {
				t.outlineGlyphs = append(t.outlineGlyphs, t.cache.OutlineWithReference(positioned.Rune, t.OutlineWidth*logicalScale))
			}
			t.glyphs = append(t.glyphs, textGlyph2D{
				raster:        reference,
				logicalWidth:  float32(raster.Width) / logicalScale,
				logicalHeight: float32(raster.Height) / logicalScale,
				x:             positioned.PenX - float32(t.Prelayout.OriginX) + logical.OffsetX,
				y:             float32(positioned.Y-t.Prelayout.OriginY) + logical.OffsetY,
				byteIndex:     int32(currentByte),
			})
		}
		t.cache.FlushUploads()
	}
	t.glyphlessLayout = !withGlyphs
	t.prelayoutActive = true
	t.changed = false
	t.layoutValue, t.lastValue = t.Text(), t.Text()
	t.lastFont, t.lastFontSize, t.lastDPI, t.lastRasterScale = t.Font, t.FontSize, t.DPI, rasterScale
	t.lastPrelayoutRevision = t.Prelayout.Revision
	t.lastOutlineWidth = t.OutlineWidth
	return true
}

type textWrapRange2D struct {
	start int
	end   int
}

func (t *Text2D) wrapRanges(value piecetree.Snapshot, objects []TextInlineObject2D) []textWrapRange2D {
	if value.Len() == 0 {
		t.wrapRangeCache = append(resetLayoutBuffer(t.wrapRangeCache, 0), textWrapRange2D{})
		return t.wrapRangeCache
	}
	width := t.WrapWidth
	words := t.WrapMode == TextWrapWords && width > 0
	result := resetLayoutBuffer(t.wrapRangeCache, t.layoutLineSpan(value))
	lineStart := 0
	for lineStart <= value.Len() {
		lineEnd := value.Len()
		if offset := value.IndexByte(lineStart, '\n'); offset >= 0 {
			lineEnd = offset
		}
		if !words {
			result = append(result, textWrapRange2D{start: lineStart, end: lineEnd})
		} else {
			result = t.appendWordWrapRanges(result, value, lineStart, lineEnd, width, objects)
		}
		if lineEnd == value.Len() {
			break
		}
		lineStart = lineEnd + 1
	}
	t.wrapRangeCache = result
	return t.wrapRangeCache
}

// Measure each character once, retaining whitespace and atomic inline objects.
func (t *Text2D) wrapPreservedRanges(value piecetree.Snapshot, objects []TextInlineObject2D, width float32, reuseMetrics bool) []textWrapRange2D {
	previousRanges := t.wrapRangeCache
	result := resetLayoutBuffer(t.wrapRangeCache, t.layoutLineSpan(value))
	objectIndex := 0
	for start := 0; start <= value.Len(); {
		end := value.Len()
		if offset := value.IndexByte(start, '\n'); offset >= 0 {
			end = offset
		}
		if reuseMetrics && len(result) < len(previousRanges) {
			previous := previousRanges[len(result)]
			// A complete hard line that fitted before still fits at this width.
			if (previous.start == 0 || t.lastValue.ByteAt(previous.start-1) == '\n') &&
				(previous.end == t.lastValue.Len() || t.lastValue.ByteAt(previous.end) == '\n') &&
				value.EqualRange(start, end, t.lastValue, previous.start, previous.end) {
				result = append(result, textWrapRange2D{start: start, end: end})
				if end == value.Len() {
					break
				}
				start = end + 1
				continue
			}
		}
		rangeStart := len(result)
		lineStart := start
		advance := textAdvance2D{previous: -1}
		cursor := value.Cursor(start, end)
		for cursor.Offset() < end {
			position := cursor.Offset()
			character, size := cursor.DecodeRune()
			next := position + size
			for objectIndex < len(objects) && objects[objectIndex].Start < position {
				objectIndex++
			}
			var object *TextInlineObject2D
			if objectIndex < len(objects) && objects[objectIndex].Start == position && objects[objectIndex].End <= end {
				object = &objects[objectIndex]
				next = object.End
			}
			cursor.Advance(next - position)
			candidate := advance
			candidate.add(t.face, character, object, t.LetterSpacing, t.TabWidth)
			token := t.WrapMode == TextWrapTokens && object == nil && isTextWrapTokenRune(character)
			if token {
				for next < end && (objectIndex >= len(objects) || next < objects[objectIndex].Start) {
					following, size := cursor.DecodeRune()
					if !isTextWrapTokenRune(following) {
						break
					}
					candidate.add(t.face, following, nil, t.LetterSpacing, t.TabWidth)
					next += size
					cursor.Advance(size)
				}
			}
			canBreak := t.WrapMode != TextWrapTokens || character != ' ' && character != '\t' && character != '\r' && character != '\n'
			if canBreak && position > lineStart && candidate.width > width {
				result = append(result, textWrapRange2D{start: lineStart, end: position})
				lineStart = position
				if token {
					// Remove only the prefix and inter-token spacing/kerning; the
					// measured token can move intact without another traversal.
					candidate.width -= advance.width
					if advance.hasItem {
						candidate.width -= t.LetterSpacing
					}
					if advance.previous >= 0 {
						candidate.width -= fixed26_6ToFloat32(t.face.Kern(advance.previous, character))
					}
				} else {
					candidate = textAdvance2D{previous: -1}
					candidate.add(t.face, character, object, t.LetterSpacing, t.TabWidth)
				}
			}
			advance = candidate
		}
		result = append(result, textWrapRange2D{start: lineStart, end: end})
		// Multiple new ranges may overwrite unread old slots in shared storage.
		if len(result)-rangeStart > 1 {
			reuseMetrics = false
		}
		if end == value.Len() {
			break
		}
		start = end + 1
	}
	t.wrapRangeCache = result
	return result
}

func isTextWrapTokenRune(value rune) bool {
	if value < utf8.RuneSelf {
		return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
	}
	return unicode.IsLetter(value) || unicode.IsDigit(value) || unicode.IsMark(value)
}

type textAdvance2D struct {
	width    float32
	previous rune
	hasItem  bool
}

func (advance *textAdvance2D) add(face *font.ScaledFace, character rune, object *TextInlineObject2D, spacing, tabWidth float32) {
	if character == '\r' && object == nil {
		return
	}
	if advance.hasItem {
		advance.width += spacing
	}
	switch {
	case object != nil:
		advance.width += object.Width
		advance.previous = -1
	case character == '\t':
		advance.width += textTabAdvance(advance.width, tabWidth)
		advance.previous = -1
	default:
		if advance.previous >= 0 {
			advance.width += fixed26_6ToFloat32(face.Kern(advance.previous, character))
		}
		width, _ := face.GlyphAdvance(character)
		advance.width += fixed26_6ToFloat32(width)
		advance.previous = character
	}
	advance.hasItem = true
}

func (t *Text2D) appendWordWrapRanges(result []textWrapRange2D, value piecetree.Snapshot, start, end int, width float32, objects []TextInlineObject2D) []textWrapRange2D {
	if start == end {
		return append(result, textWrapRange2D{start: start, end: end})
	}
	initialLength := len(result)
	lineStart := start
	advance := textAdvance2D{previous: -1}
	cursor := value.Cursor(start, end)
	objectIndex := 0
	for objectIndex < len(objects) && objects[objectIndex].End <= start {
		objectIndex++
	}
	for cursor.Offset() < end {
		for cursor.Offset() < end && isTextWrapSpace(cursor.Byte()) {
			advance.add(t.face, rune(cursor.Byte()), nil, t.LetterSpacing, t.TabWidth)
			cursor.Advance(1)
		}
		wordStart := cursor.Offset()
		if wordStart == end {
			break
		}
		// Measure the word once in its current line. Moving it to the next
		// line only removes the prefix and the first item's boundary spacing.
		prefix := advance
		boundary := float32(0)
		wordHasItem := false
		for cursor.Offset() < end && !isTextWrapSpace(cursor.Byte()) {
			character, size := cursor.DecodeRune()
			position := cursor.Offset()
			for objectIndex < len(objects) && objects[objectIndex].Start < position {
				objectIndex++
			}
			var object *TextInlineObject2D
			if objectIndex < len(objects) && objects[objectIndex].Start == position && objects[objectIndex].End <= end {
				object = &objects[objectIndex]
				size = object.End - position
			}
			if !wordHasItem && (character != '\r' || object != nil) {
				if prefix.hasItem {
					boundary = t.LetterSpacing
				}
				if prefix.previous >= 0 && object == nil {
					boundary += fixed26_6ToFloat32(t.face.Kern(prefix.previous, character))
				}
				wordHasItem = true
			}
			advance.add(t.face, character, object, t.LetterSpacing, t.TabWidth)
			cursor.Advance(size)
		}
		wordEnd := cursor.Offset()
		word := advance
		word.width -= prefix.width + boundary
		if !wordHasItem {
			word = textAdvance2D{previous: -1}
		}
		// The first word may exceed the width and remains atomic. Its trailing
		// whitespace does not participate in that first-word overflow check.
		if lineStart == wordStart && word.width > width {
			result = append(result, textWrapRange2D{start: wordStart, end: wordEnd})
			for cursor.Offset() < end && isTextWrapSpace(cursor.Byte()) {
				cursor.Advance(1)
			}
			lineStart = cursor.Offset()
			advance = textAdvance2D{previous: -1}
			continue
		}
		spaces := cursor
		for cursor.Offset() < end && isTextWrapSpace(cursor.Byte()) {
			advance.add(t.face, rune(cursor.Byte()), nil, t.LetterSpacing, t.TabWidth)
			cursor.Advance(1)
		}
		if lineStart != wordStart && advance.width > width {
			result = append(result, textWrapRange2D{start: lineStart, end: trimTextWrapSpace(value, wordStart)})
			lineStart = wordStart
			advance = word
			// Tabs depend on the new line origin; only replay the trailing
			// whitespace after moving a word, never the preceding line.
			for spaces.Offset() < cursor.Offset() {
				advance.add(t.face, rune(spaces.Byte()), nil, t.LetterSpacing, t.TabWidth)
				spaces.Advance(1)
			}
		}
	}
	if lineStart < end {
		result = append(result, textWrapRange2D{start: lineStart, end: trimTextWrapSpace(value, end)})
	} else if len(result) == initialLength {
		result = append(result, textWrapRange2D{start: start, end: end})
	}
	return result
}

func isTextWrapSpace(value byte) bool {
	return value == ' ' || value == '\t'
}

func isTextWrapDelimiter(value byte) bool {
	return isTextWrapSpace(value) || value == '\r' || value == '\n'
}

func trimTextWrapSpace(value piecetree.Snapshot, end int) int {
	for end > 0 && isTextWrapSpace(value.ByteAt(end-1)) {
		end--
	}
	return end
}

func (t *Text2D) validInlineObjects() []TextInlineObject2D {
	if t == nil || len(t.InlineObjects) == 0 {
		return nil
	}
	result := make([]TextInlineObject2D, 0, len(t.InlineObjects))
	previousEnd := 0
	for _, object := range t.InlineObjects {
		if object.Start < previousEnd || object.Start < 0 || object.Start >= object.End || object.End > t.Text().Len() || object.Width <= 0 || object.Height <= 0 || object.Baseline < 0 || object.Baseline > object.Height || !utf8.RuneStart(t.Text().ByteAt(object.Start)) || !(object.End == t.Text().Len() || utf8.RuneStart(t.Text().ByteAt(object.End))) || strings.ContainsAny(t.Text().Slice(object.Start, object.End), "\r\n") {
			continue
		}
		result = append(result, object)
		previousEnd = object.End
	}
	return result
}

func (t *Text2D) ellipsizedText() piecetree.Snapshot {
	if t == nil {
		return piecetree.Snapshot{}
	}
	if t.MaxWidth <= 0 || t.Font == nil || t.face == nil {
		return t.Text()
	}
	objects := t.validInlineObjects()
	needsEllipsis := false
	for start := 0; start <= t.Text().Len(); {
		end := t.Text().Len()
		if offset := t.Text().IndexByte(start, '\n'); offset >= 0 {
			end = offset
		}
		line := t.Text().Slice(start, end)
		if strings.HasSuffix(line, "\r") || textWidthWithInlineObjects(t.face, line, start, objects, t.LetterSpacing, t.TabWidth) > t.MaxWidth {
			needsEllipsis = true
			break
		}
		if end == t.Text().Len() {
			break
		}
		start = end + 1
	}
	if !needsEllipsis {
		return t.Text()
	}
	ellipsis := "..."
	for len(ellipsis) > 0 && textWidthWithInlineObjects(t.face, ellipsis, 0, nil, t.LetterSpacing, t.TabWidth) > t.MaxWidth {
		ellipsis = ellipsis[:len(ellipsis)-1]
	}

	var result strings.Builder
	result.Grow(t.Text().Len())
	for start := 0; start <= t.Text().Len(); {
		end := t.Text().Len()
		hasNewline := false
		if offset := t.Text().IndexByte(start, '\n'); offset >= 0 {
			end = offset
			hasNewline = true
		}
		line := t.Text().Slice(start, end)
		line = strings.TrimSuffix(line, "\r")
		if textWidthWithInlineObjects(t.face, line, start, objects, t.LetterSpacing, t.TabWidth) > t.MaxWidth {
			prefixEnd := fittingTextPrefixWithInlineObjects(t.face, line, start, objects, ellipsis, t.MaxWidth, t.LetterSpacing, t.TabWidth)
			result.WriteString(line[:prefixEnd])
			result.WriteString(ellipsis)
		} else {
			result.WriteString(line)
		}
		if !hasNewline {
			break
		}
		result.WriteByte('\n')
		start = end + 1
	}
	return piecetree.FromString(result.String())
}

func textTabAdvance(position, interval float32) float32 {
	if interval <= 0 {
		return 0
	}
	return (float32(math.Floor(float64(max(position, 0)/interval)))+1)*interval - position
}

func textWidthWithInlineObjects(face *font.ScaledFace, value string, valueStart int, objects []TextInlineObject2D, letterSpacing, tabWidth float32) float32 {
	advance := textAdvance2D{previous: -1}
	objectIndex, skipUntil := 0, 0
	for objectIndex < len(objects) && objects[objectIndex].End <= valueStart {
		objectIndex++
	}
	for byteIndex, character := range value {
		globalIndex := valueStart + byteIndex
		if globalIndex < skipUntil {
			continue
		}
		if character == '\r' {
			continue
		}
		if objectIndex < len(objects) && objects[objectIndex].Start == globalIndex {
			advance.add(face, character, &objects[objectIndex], letterSpacing, tabWidth)
			skipUntil = objects[objectIndex].End
			objectIndex++
			continue
		}
		advance.add(face, character, nil, letterSpacing, tabWidth)
	}
	return advance.width
}

func fittingTextPrefixWithInlineObjects(face *font.ScaledFace, value string, valueStart int, objects []TextInlineObject2D, suffix string, maxWidth, letterSpacing, tabWidth float32) int {
	suffixWidth := textWidthWithInlineObjects(face, suffix, 0, nil, letterSpacing, tabWidth)
	if suffixWidth > maxWidth {
		return 0
	}
	advance := float32(0)
	previous := rune(-1)
	hasItem := false
	best := 0
	objectIndex, skipUntil := 0, 0
	for objectIndex < len(objects) && objects[objectIndex].End <= valueStart {
		objectIndex++
	}
	firstSuffix, _ := utf8.DecodeRuneInString(suffix)
	for byteIndex, character := range value {
		globalIndex := valueStart + byteIndex
		if globalIndex < skipUntil {
			continue
		}
		candidateEnd := byteIndex
		if objectIndex < len(objects) && objects[objectIndex].Start == globalIndex {
			if hasItem {
				advance += letterSpacing
			}
			advance += objects[objectIndex].Width
			candidateEnd = objects[objectIndex].End - valueStart
			skipUntil = objects[objectIndex].End
			objectIndex++
			previous = -1
			hasItem = true
		} else {
			if hasItem {
				advance += letterSpacing
			}
			if character == '\t' {
				advance += textTabAdvance(advance, tabWidth)
			} else {
				if previous >= 0 {
					advance += fixed26_6ToFloat32(face.Kern(previous, character))
				}
				glyphAdvance, _ := face.GlyphAdvance(character)
				advance += fixed26_6ToFloat32(glyphAdvance)
			}
			_, size := utf8.DecodeRuneInString(value[byteIndex:])
			candidateEnd = byteIndex + size
			previous = character
			if character == '\t' {
				previous = -1
			}
			hasItem = true
		}
		candidate := advance + suffixWidth
		if hasItem && suffix != "" {
			candidate += letterSpacing
			if previous >= 0 {
				candidate += fixed26_6ToFloat32(face.Kern(previous, firstSuffix))
			}
		}
		if candidate <= maxWidth {
			best = candidateEnd
		}
	}
	return best
}

func (t *textPaint2D) drawGlyph(c *graphics.Canvas2D, glyph onixtext.Glyph, base mgl32.Mat4, x, y float32, texture *graphics.Texture, clr graphics.ColorRGBA, clip clipRect2D) {
	model := base.Mul4(mgl32.Translate3D(x, y, 0))
	textureWidth, textureHeight := float32(texture.Width), float32(texture.Height)
	rasterScale := float32(normalizeTextRasterScale(t.rasterScale))
	width := float32(glyph.Width) / rasterScale
	height := float32(glyph.Height) / rasterScale
	// Glyph bitmaps include transparent texel padding on every side. Keep the
	// full bitmap range so pixel-center interpolation samples each coverage
	// texel once; the padding prevents linear filtering from reaching the next
	// atlas entry without shrinking the visible glyph at its own edges.
	textureM := mgl32.Translate2D(float32(glyph.X)/textureWidth, float32(glyph.Y)/textureHeight).
		Mul3(mgl32.Scale2D(float32(glyph.Width)/textureWidth, float32(glyph.Height)/textureHeight))
	model, textureM, ok := clippedQuad2D(model, textureM, width, height, clip)
	if ok {
		custom := [4][4]float32{}
		if clip.hasRounded {
			custom = roundedClipMaterial2D(custom, clip.rounded)
		}
		c.DrawTintedMatricesCustom(model, textureM, clr, t.Alpha, custom)
		glyph.KeepAlive(c)
	}
}

func roundedClipMaterial2D(custom [4][4]float32, clip roundedClip2D) [4][4]float32 {
	custom[2] = [4]float32{clip.minX, clip.minY, clip.maxX, clip.maxY}
	custom[3][0], custom[3][1] = clip.radius, 1
	return custom
}

func newSpriteRenderItem2D(sequence uint32, entity ecs.Entity, sprite *Sprite2D, material2D *graphics.Material2DData, anchor Anchor, transform mgl32.Mat4, clip clipRect2D, width, height float32, ui bool, sortIndex int) renderItem2D {
	kind := renderItemSprite2D
	var shader *graphics.Material2DShader
	if material2D != nil {
		shader = material2D.Shader
	}
	material := textureMaterialKey(sprite.Texture, shader)
	texture := sprite.Texture
	if sprite.Shape.Kind != 0 {
		kind = renderItemShape2D
		material = shapeMaterialKey()
		texture = nil
	}
	return renderItem2D{
		order:      renderOrder2D(entity),
		sequence:   sequence,
		depth:      transform[14],
		ui:         ui,
		sortIndex:  sortIndex,
		material:   material,
		material2D: material2D,
		kind:       kind,
		content:    sprite,
		texture:    texture,
		transform:  transform,
		anchor:     anchor,
		clip:       clip,
		width:      width,
		height:     height,
	}
}

func newParticleRenderItem2D(sequence uint32, entity ecs.Entity, emitter *ParticleEmitter2D, transform mgl32.Mat4, clip clipRect2D, ui bool, sortIndex int, fallbackTexture *graphics.Texture) renderItem2D {
	texture := emitter.Texture
	if texture == nil {
		texture = fallbackTexture
	}
	return renderItem2D{
		order:     renderOrder2D(entity),
		sequence:  sequence,
		depth:     transform[14],
		ui:        ui,
		sortIndex: sortIndex,
		material:  textureMaterialKey(texture, nil),
		kind:      renderItemParticle2D,
		content:   emitter,
		texture:   texture,
		transform: transform,
		clip:      clip,
	}
}

func newTilemapRenderItem2D(sequence uint32, entity ecs.Entity, tilemap *Tilemap2D, anchor Anchor, transform mgl32.Mat4, clip clipRect2D, ui bool, sortIndex int) renderItem2D {
	return renderItem2D{
		order: renderOrder2D(entity), sequence: sequence, depth: transform[14], ui: ui, sortIndex: sortIndex,
		material: textureMaterialKey(tilemap.Texture, nil), kind: renderItemTilemap2D, content: tilemap,
		texture: tilemap.Texture, transform: transform, anchor: anchor, clip: clip,
	}
}

func (item renderItem2D) drawTilemap(c *graphics.Canvas2D, viewport renderViewport2D) {
	tilemap := item.snapshot[Tilemap2D]()
	base := quadBaseTransform(item.transform, item.anchor, tilemap.PixelWidth(), tilemap.PixelHeight(), item.ui)
	rows := tilemap.Texture.Height / tilemap.TileHeight
	if rows <= 0 {
		return
	}
	visible := tilemapVisibleRange2D(viewport, base, tilemap)
	textureScaleX := float32(tilemap.TileWidth) / float32(tilemap.Texture.Width)
	textureScaleY := float32(tilemap.TileHeight) / float32(tilemap.Texture.Height)
	for cellRow := visible.minY; cellRow < visible.maxY; cellRow++ {
		for cellColumn := visible.minX; cellColumn < visible.maxX; cellColumn++ {
			value := tilemap.Tiles[cellRow*tilemap.Width+cellColumn]
			if value <= 0 {
				continue
			}
			tile := value - 1
			column, row := tile%tilemap.Columns, tile/tilemap.Columns
			if row >= rows {
				continue
			}
			model := base
			model.SetCol(3, base.Col(0).Mul(float32(cellColumn*tilemap.TileWidth)).Add(base.Col(1).Mul(float32(cellRow*tilemap.TileHeight))).Add(base.Col(3)))
			if !viewport.visible(model, float32(tilemap.TileWidth), float32(tilemap.TileHeight)) {
				continue
			}
			texture := mgl32.Mat3{0: textureScaleX, 4: textureScaleY, 6: float32(column) * textureScaleX, 7: float32(row) * textureScaleY, 8: 1}
			model, texture, ok := clippedQuad2D(model, texture, float32(tilemap.TileWidth), float32(tilemap.TileHeight), item.clip)
			if ok {
				custom := item.materialData().Custom
				if item.clip.hasRounded {
					custom = roundedClipMaterial2D(custom, item.clip.rounded)
				}
				c.DrawTintedMatricesCustom(model, texture, graphics.White, tilemap.Alpha, custom)
			}
		}
	}
}

func textureMatrix(texture *graphics.Texture) mgl32.Mat3 {
	if texture == nil {
		return mgl32.Ident3()
	}
	return mgl32.Translate2D(texture.U, texture.V).
		Mul3(mgl32.Scale2D(texture.MaxU, texture.MaxV)).
		Mul3(mgl32.HomogRotate2D(mgl32.DegToRad(texture.Angle)))
}

type clipRect2D struct {
	rounded    roundedClip2D
	minX       float32
	minY       float32
	maxX       float32
	maxY       float32
	valid      bool
	hasRounded bool
}

type roundedClip2D struct {
	minX      float32
	minY      float32
	maxX      float32
	maxY      float32
	radius    float32
	maskColor graphics.ColorRGBA
}

type renderItems2DScratch struct{}

func (c clipRect2D) empty() bool {
	return c.valid && (c.maxX <= c.minX || c.maxY <= c.minY)
}

func renderItemVisible2D(viewport renderViewport2D, transform mgl32.Mat4, anchor Anchor, width, height float32, ui bool, clip clipRect2D, cull bool) bool {
	if width <= 0 || height <= 0 || clip.empty() {
		return false
	}
	base := quadBaseTransform(transform, anchor, width, height, ui)
	if clip.valid && !clipIntersectsBounds2D(base, width, height, clip) {
		return false
	}
	return !cull || viewport.visible(base, width, height)
}

func textRenderItemVisible2D(viewport renderViewport2D, item *renderItem2D) bool {
	if item == nil || item.width <= 0 || item.height <= 0 {
		return false
	}
	padding := float32(0)
	if item.snapshot[textPaint2D]() != nil {
		padding = max(validOutlinePadding2D(item.snapshot[textPaint2D]().OutlineWidth), absFloat32(item.snapshot[textPaint2D]().ShadowX), absFloat32(item.snapshot[textPaint2D]().ShadowY))
	}
	if padding > 0 {
		return viewport.visibleBase(
			item.transform.Mul4(mgl32.Translate3D(-padding, -padding, 0)),
			item.width+padding*2,
			item.height+padding*2,
		)
	}
	return viewport.visibleBase(item.transform, item.width, item.height)
}

func spineRenderItemVisible2D(viewport renderViewport2D, transform mgl32.Mat4, clip clipRect2D, minX, minY, maxX, maxY float32, valid bool) bool {
	width, height := maxX-minX, maxY-minY
	if !valid || width <= 0 || height <= 0 || clip.empty() {
		return false
	}
	base := transform.Mul4(mgl32.Translate3D(minX, minY, 0))
	if clip.valid && !clipIntersectsBounds2D(base, width, height, clip) {
		return false
	}
	return viewport.visible(base, width, height)
}

func validOutlinePadding2D(width float32) float32 {
	if !validOutlineWidth(width) {
		return 0
	}
	return width
}

func clipRectFromLocal2D(transform mgl32.Mat4, clip ClipRect2D) clipRect2D {
	if clip.Width <= 0 || clip.Height <= 0 {
		return clipRect2D{valid: true}
	}
	minX, minY, maxX, maxY, ok := boundsFromBase2D(
		transform.Mul4(mgl32.Translate3D(clip.X, clip.Y, 0)),
		clip.Width,
		clip.Height,
	)
	if !ok {
		return clipRect2D{}
	}
	return clipRect2D{minX: minX, minY: minY, maxX: maxX, maxY: maxY, valid: true}
}

func roundedClipFromLocal2D(transform mgl32.Mat4, clip RoundedClip2D) roundedClip2D {
	if clip.Width <= 0 || clip.Height <= 0 {
		return roundedClip2D{}
	}
	minX, minY, maxX, maxY, ok := boundsFromBase2D(
		transform.Mul4(mgl32.Translate3D(clip.X, clip.Y, 0)),
		clip.Width,
		clip.Height,
	)
	if !ok {
		return roundedClip2D{}
	}
	scaleX := float32(math.Hypot(float64(transform[0]), float64(transform[1])))
	scaleY := float32(math.Hypot(float64(transform[4]), float64(transform[5])))
	scale := min(max(scaleX, 0), max(scaleY, 0))
	if scale <= 0 {
		scale = 1
	}
	radius := max(min(clip.Radius*scale, (maxX-minX)*0.5), 0)
	radius = min(radius, (maxY-minY)*0.5)
	return roundedClip2D{minX: minX, minY: minY, maxX: maxX, maxY: maxY, radius: radius, maskColor: clip.MaskColor}
}

func addRoundedClip2D(clip clipRect2D, rounded roundedClip2D) clipRect2D {
	if rounded.maxX <= rounded.minX || rounded.maxY <= rounded.minY || rounded.radius <= 0 {
		return clip
	}
	clip.hasRounded = true
	clip.rounded = rounded
	return clip
}

func intersectClipRect2D(a, b clipRect2D) clipRect2D {
	if !a.valid {
		return b
	}
	if !b.valid {
		return a
	}
	clip := clipRect2D{
		minX:  max(a.minX, b.minX),
		minY:  max(a.minY, b.minY),
		maxX:  min(a.maxX, b.maxX),
		maxY:  min(a.maxY, b.maxY),
		valid: true,
	}
	if a.hasRounded || b.hasRounded {
		if a.hasRounded && b.hasRounded {
			// Nested CSS clips are represented by the tighter rounded region;
			// the rectangle intersection still retains the exact conservative
			// bounds for culling and texture clipping.
			chosen := a.rounded
			if (b.rounded.maxX-b.rounded.minX)*(b.rounded.maxY-b.rounded.minY) < (chosen.maxX-chosen.minX)*(chosen.maxY-chosen.minY) {
				chosen = b.rounded
			}
			chosen.minX, chosen.minY = clip.minX, clip.minY
			chosen.maxX, chosen.maxY = clip.maxX, clip.maxY
			chosen.radius = min(chosen.radius, min((clip.maxX-clip.minX)*0.5, (clip.maxY-clip.minY)*0.5))
			clip.rounded = chosen
		} else if a.hasRounded {
			clip.rounded = a.rounded
		} else {
			clip.rounded = b.rounded
		}
		clip.hasRounded = clip.rounded.maxX > clip.rounded.minX && clip.rounded.maxY > clip.rounded.minY && clip.rounded.radius > 0
	}
	return clip
}

func clippedQuad2D(transform mgl32.Mat4, texture mgl32.Mat3, width, height float32, clip clipRect2D) (mgl32.Mat4, mgl32.Mat3, bool) {
	if width <= 0 || height <= 0 {
		return mgl32.Ident4(), mgl32.Ident3(), false
	}
	if !clip.valid {
		return transform.Mul4(mgl32.Scale3D(width, height, 1)), texture, true
	}
	if clip.maxX <= clip.minX || clip.maxY <= clip.minY {
		return mgl32.Ident4(), mgl32.Ident3(), false
	}

	origin := transform.Mul4x1(mgl32.Vec4{0, 0, 0, 1})
	xAxis := transform.Mul4x1(mgl32.Vec4{1, 0, 0, 1}).Sub(origin)
	yAxis := transform.Mul4x1(mgl32.Vec4{0, 1, 0, 1}).Sub(origin)
	const epsilon = 1e-5
	if absFloat32(xAxis.Y()) > epsilon || absFloat32(yAxis.X()) > epsilon {
		if !clipIntersectsBounds2D(transform, width, height, clip) {
			return mgl32.Ident4(), mgl32.Ident3(), false
		}
		return transform.Mul4(mgl32.Scale3D(width, height, 1)), texture, true
	}

	localX0, localX1, ok := clippedAxisRange2D(origin.X(), xAxis.X(), width, clip.minX, clip.maxX)
	if !ok {
		return mgl32.Ident4(), mgl32.Ident3(), false
	}
	localY0, localY1, ok := clippedAxisRange2D(origin.Y(), yAxis.Y(), height, clip.minY, clip.maxY)
	if !ok {
		return mgl32.Ident4(), mgl32.Ident3(), false
	}

	localW := localX1 - localX0
	localH := localY1 - localY0
	model := transform.
		Mul4(mgl32.Translate3D(localX0, localY0, 0)).
		Mul4(mgl32.Scale3D(localW, localH, 1))
	texture = texture.
		Mul3(mgl32.Translate2D(localX0/width, localY0/height)).
		Mul3(mgl32.Scale2D(localW/width, localH/height))
	return model, texture, true
}

func clippedAxisRange2D(origin, scale, size, clipMin, clipMax float32) (float32, float32, bool) {
	const epsilon = 1e-5
	if absFloat32(scale) <= epsilon {
		return 0, size, origin >= clipMin && origin <= clipMax
	}
	a := (clipMin - origin) / scale
	b := (clipMax - origin) / scale
	if a > b {
		a, b = b, a
	}
	start := max(float32(0), a)
	end := min(size, b)
	return start, end, end > start
}

func clipIntersectsBounds2D(transform mgl32.Mat4, width, height float32, clip clipRect2D) bool {
	minX, minY, maxX, maxY, ok := boundsFromBase2D(transform, width, height)
	if !ok {
		return false
	}
	return maxX >= clip.minX && maxY >= clip.minY && minX <= clip.maxX && minY <= clip.maxY
}

type renderMaterialKey struct {
	pipeline  render.Pipeline
	textureID uint64
	shaderID  uint64
}

func textureMaterialKey(texture *graphics.Texture, shader *graphics.Material2DShader) renderMaterialKey {
	if texture == nil {
		return renderMaterialKey{}
	}
	shaderID := uint64(0)
	if shader != nil {
		shaderID = shader.BatchKey()
	}
	return renderMaterialKey{pipeline: render.PipelineSprite, textureID: texture.BatchKey(), shaderID: shaderID}
}

func shapeMaterialKey() renderMaterialKey {
	return renderMaterialKey{pipeline: render.PipelineShape}
}

func (item renderItem2D) renderSortKey() renderSortKey2D {
	return renderSortKey2D{
		scene:     item.scene.Order,
		group:     item.group,
		ui:        item.ui,
		sortIndex: item.sortIndex,
		depth:     item.depth,
		order:     item.order,
		sequence:  item.sequence,
	}
}

func (item *renderItem2D) renderPhaseKeys() (renderSortKey2D, renderMaterialKey) {
	return renderSortKey2D{scene: item.scene.Order, group: item.group, ui: item.ui, sortIndex: item.sortIndex, depth: item.depth, order: item.order, sequence: item.sequence}, item.material
}

func (item renderItem2D) renderMaterial() renderMaterialKey {
	return item.material
}

func (item renderItem2D) quadTransform() mgl32.Mat4 {
	return quadBaseTransform(item.transform, item.anchor, item.width, item.height, item.ui)
}

func renderSize2D(world *ecs.World, entity ecs.Entity, width, height float32) (float32, float32) {
	return width, height
}

func cloneSprite2D(sprite Sprite2D) Sprite2D {
	if sprite.Slice != nil {
		slice := *sprite.Slice
		sprite.Slice = &slice
	}
	return sprite
}

func anchorForEntity2D(world *ecs.World, entity ecs.Entity) Anchor {
	anchor, _ := world.Get[Anchor](entity)
	return anchor
}

func quadBaseTransform(transform mgl32.Mat4, anchor Anchor, width, height float32, screenSpace bool) mgl32.Mat4 {
	x, y := anchor.TopLeft(width, height, screenSpace)
	base := transform.Mul4(mgl32.Translate3D(x, y, 0))
	if !screenSpace {
		base = base.Mul4(mgl32.Scale3D(1, -1, 1))
	}
	return base
}

func renderOrder2D(entity ecs.Entity) uint64 {
	return uint64(entity)
}

func (t *Text2D) lineWidth() float32 {
	if t == nil {
		return 0
	}
	return t.layoutWidth
}

func (t *Text2D) layoutSize() (float32, float32) {
	if t == nil || t.face == nil {
		return 0, 0
	}
	return t.lineWidth(), t.layoutHeight
}

func absFloat32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func fixed26_6ToFloat32(v font.Int26_6) float32 {
	return float32(v) / 64
}

// Text returns the authoritative immutable text used for geometry and paint.
func (t Text2D) Text() piecetree.Snapshot {
	if t.Source != (piecetree.Snapshot{}) {
		return t.Source
	}
	return piecetree.FromString(t.Value)
}
