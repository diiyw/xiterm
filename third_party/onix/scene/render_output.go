package scene

import (
	"errors"

	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/graphics"
)

var (
	ErrOutputCapacity = errors.New("scene: render output lease capacity reached")
	ErrOutputReleased = errors.New("scene: render output lease released")
)

// RenderOutputHandle is one lease on immutable logical drawing data. Copying a
// handle borrows that lease; Retain creates an independently releasable lease.
// Pools and handles are used on the engine's single logic/render thread.
type RenderOutputHandle struct {
	pool *RenderOutputPool
	id   uint64
}

type RenderOutput2D struct{ Handle RenderOutputHandle }

var renderOutputRevision2D uint64

type renderOutput struct {
	release               func() error
	text                  Text2D
	refs                  int
	width, height         float32
	clipWidth, clipHeight float32
	hasClip               bool
}

type outputLease struct {
	output *renderOutput
	id     uint64
}

// Fixed storage makes cached-output Retain/Release allocation-free and rejects
// capacity overflow explicitly rather than silently growing the hot path.
type RenderOutputPool struct {
	leases   []outputLease
	free     []int
	sequence uint64
}

func NewRenderOutputPool(capacity int) *RenderOutputPool {
	if capacity < 1 {
		return nil
	}
	p := &RenderOutputPool{leases: make([]outputLease, capacity), free: make([]int, capacity)}
	for i := range p.free {
		p.free[i] = capacity - 1 - i
	}
	return p
}

// Text prepares glyph geometry once. The caller keeps the font alive until
// release runs after the final lease. No font or mutable draw callback is
// exposed through the resulting handle. Atlas textures use driver retirement.
func (p *RenderOutputPool) Text(value Text2D, release func() error) (RenderOutputHandle, error) {
	return p.TextBox(value, Size2D{}, release)
}

// TextBox limits painted output to positive logical bounds. Zero is unbounded.
func (p *RenderOutputPool) TextBox(value Text2D, bounds Size2D, release func() error) (RenderOutputHandle, error) {
	if p == nil || len(p.free) == 0 {
		return RenderOutputHandle{}, ErrOutputCapacity
	}
	// Strip all private layout caches: copying a measured Text2D must not share
	// mutable geometry buffers with the producer's future layout work.
	value = Text2D{Value: value.Value, Font: value.Font, FontSize: value.FontSize, DPI: value.DPI,
		WrapMode: value.WrapMode, WrapWidth: value.WrapWidth, LetterSpacing: value.LetterSpacing,
		LineHeight: value.LineHeight, InkOffsetY: value.InkOffsetY, TabWidth: value.TabWidth,
		TextAlignment: value.TextAlignment, MaxWidth: value.MaxWidth, Color: value.Color, Alpha: value.Alpha,
		ShadowX: value.ShadowX, ShadowY: value.ShadowY, ShadowColor: value.ShadowColor,
		OutlineWidth: value.OutlineWidth, OutlineColor: value.OutlineColor,
		Spans: append([]TextSpan2D(nil), value.Spans...), InlineObjects: append([]TextInlineObject2D(nil), value.InlineObjects...)}
	value.rasterScale = 1
	if value.Font == nil || !value.ensureLayout() {
		return RenderOutputHandle{}, errors.New("scene: invalid text output")
	}
	w, h := value.layoutSize()
	clipWidth, clipHeight := w, h
	if bounds.Width > 0 {
		clipWidth = bounds.Width
		w = min(w, bounds.Width)
	}
	if bounds.Height > 0 {
		clipHeight = bounds.Height
		h = min(h, bounds.Height)
	}
	return p.acquire(&renderOutput{text: value, width: w, height: h, clipWidth: clipWidth, clipHeight: clipHeight, hasClip: bounds.Width > 0 || bounds.Height > 0, release: release}), nil
}

func (p *RenderOutputPool) acquire(o *renderOutput) RenderOutputHandle {
	i := p.free[len(p.free)-1]
	p.free = p.free[:len(p.free)-1]
	p.sequence++
	if p.sequence >= (^uint64(0)-uint64(i))/uint64(len(p.leases)) {
		panic("scene: output identity exhausted")
	}
	id := p.sequence*uint64(len(p.leases)) + uint64(i)
	p.leases[i] = outputLease{id: id, output: o}
	o.refs++
	return RenderOutputHandle{p, id}
}

func (h RenderOutputHandle) output() *renderOutput {
	if h.pool == nil || h.id == 0 {
		return nil
	}
	lease := &h.pool.leases[h.id%uint64(len(h.pool.leases))]
	if lease.id != h.id {
		return nil
	}
	return lease.output
}

func (h RenderOutputHandle) Valid() bool { return h.output() != nil }
func (h RenderOutputHandle) Empty() bool { return h.pool == nil && h.id == 0 }
func (h RenderOutputHandle) Size() (float32, float32, bool) {
	if o := h.output(); o != nil {
		return o.width, o.height, true
	}
	return 0, 0, false
}
func (h RenderOutputHandle) Retain() (RenderOutputHandle, error) {
	o := h.output()
	if o == nil {
		return RenderOutputHandle{}, ErrOutputReleased
	}
	if len(h.pool.free) == 0 {
		return RenderOutputHandle{}, ErrOutputCapacity
	}
	return h.pool.acquire(o), nil
}
func (h RenderOutputHandle) Release() error {
	if h.Empty() {
		return nil
	}
	o := h.output()
	if o == nil {
		return ErrOutputReleased
	}
	i := int(h.id % uint64(len(h.pool.leases)))
	h.pool.leases[i] = outputLease{}
	renderOutputRevision2D++
	h.pool.free = append(h.pool.free, i)
	o.refs--
	if o.refs == 0 {
		if o.text.face != nil {
			_ = o.text.face.Close()
		}
		if o.release != nil {
			return o.release()
		}
	}
	return nil
}
func (p *RenderOutputPool) Leases() int { return len(p.leases) - len(p.free) }

func (snapshots *renderSnapshots2D) appendOutputRenderItems2D[T renderGlobalTransform2D](c *graphics.Canvas2D, world *ecs.World, hierarchy *renderHierarchy2DCache, items []renderItem2D, cull, ui bool) []renderItem2D {
	viewport := renderViewport2DForSpace(c, world, ui)
	for entity, access := range world.Query3[RenderOutput2D, T, Visibility]().All {
		o := access.A.Ref().Handle.output()
		if o == nil || !access.C.Ref().Visible || !o.text.paintsVisibleGlyphs() {
			continue
		}
		matrix, clip := (*access.B.Ref()).renderMatrix2D(), (*access.B.Ref()).renderClip2D()
		anchor := anchorForEntity2D(world, entity)
		base := quadBaseTransform(matrix, anchor, o.width, o.height, ui)
		if o.hasClip {
			clip = intersectClipRect2D(clip, clipRectFromLocal2D(base, ClipRect2D{Width: o.clipWidth, Height: o.clipHeight}))
		}
		if !renderItemVisible2D(viewport, matrix, anchor, o.width, o.height, ui, clip, cull) {
			continue
		}
		text := snapshots.addText(o.text)
		info := renderEntityInfo2D(world, entity, ui, 0, hierarchy)
		start := len(items)
		items = o.text.appendRenderItems(items, entity, base, clip, ui, info.sortIndex, text)
		for i := start; i < len(items); i++ {
			items[i].entity, items[i].layers, items[i].ui, items[i].target, items[i].scene, items[i].group = entity, info.layers, ui, info.target, info.scene, info.group
			items[i].width, items[i].height = o.width, o.height
		}
	}
	return items
}
