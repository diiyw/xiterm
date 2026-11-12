package text

import (
	"errors"
	"image"
	"image/draw"
	"math"
	"math/bits"
	"sync"
	"weak"

	"github.com/onix-engine/onix/font"
	"github.com/onix-engine/onix/graphics"
	"github.com/onix-engine/onix/render"
)

const (
	defaultGlyphAtlasSize = 512
	glyphBitmapPadding    = 2
	glyphAtlasPadding     = 2
	glyphAtlasByteLimit   = 2 << 20
	glyphCacheEntryLimit  = 4096
)

type Glyph struct {
	page    *glyphAtlas
	Atlas   int32
	X, Y    int32
	Width   int32
	Height  int32
	Rune    rune
	OffsetX float32
	OffsetY float32
	Advance float32
}

// GlyphRef shares an immutable raster descriptor. The descriptor owns its atlas
// page independently of cache slots, so eviction cannot change a live layout.
type GlyphRef struct{ value *Glyph }

func (ref GlyphRef) Glyph() Glyph {
	if ref.value != nil {
		return *ref.value
	}
	return Glyph{}
}

// RGBA reports whether the atlas retains RGB channels (color or LCD glyphs).
func (g Glyph) RGBA() bool { return g.page != nil && g.page.lcd }

func (g Glyph) Visible() bool {
	return g.Width > 0 && g.Height > 0
}

// KeepAlive retains the atlas until pending canvas draws have been submitted.
// Copied glyphs already retain their pages for the lifetime of their layout.
func (g Glyph) KeepAlive(canvas *graphics.Canvas2D) {
	if g.page != nil {
		canvas.RetainUntilFlush(g.page)
	}
}

// Cached records do not pin pages. Returned glyphs do, so evicting a page from
// the LRU never invalidates geometry held by live text or render snapshots.
type glyphMetrics struct {
	Atlas, X, Y, Width, Height int32
	Rune                       rune
	OffsetX, OffsetY, Advance  float32
}

type cachedGlyph struct {
	page      weak.Pointer[glyphAtlas]
	reference weak.Pointer[Glyph]
	metrics   glyphMetrics
}

func cacheGlyph(g Glyph) cachedGlyph {
	return cachedGlyph{page: weak.Make(g.page), metrics: glyphMetrics{
		Atlas: g.Atlas, X: g.X, Y: g.Y, Width: g.Width, Height: g.Height,
		Rune: g.Rune, OffsetX: g.OffsetX, OffsetY: g.OffsetY, Advance: g.Advance,
	}}
}

func (g cachedGlyph) glyph() Glyph {
	return Glyph{Atlas: g.metrics.Atlas, X: g.metrics.X, Y: g.metrics.Y,
		Width: g.metrics.Width, Height: g.metrics.Height, Rune: g.metrics.Rune,
		OffsetX: g.metrics.OffsetX, OffsetY: g.metrics.OffsetY, Advance: g.metrics.Advance}
}

type glyphAtlas struct {
	texture    *graphics.Texture
	prev, next *glyphAtlas
	pending    []graphics.TextureRegionUpdate
	size       int
	cursorX    int
	cursorY    int
	rowH       int
	index      int
	lcd        bool
	cached     bool
	queued     bool
}

type glyphAtlasSlot struct {
	page    weak.Pointer[glyphAtlas]
	texture *graphics.Texture
}

type glyphAtlasPool struct {
	err            error
	oldest, newest *glyphAtlas
	atlases        []glyphAtlasSlot
	dirtyPages     []int
	cachedBytes    int
	cachedPages    int
	pages          int
	byteLimit      int
	sweep          int
	mu             sync.Mutex
}

func newGlyphAtlas(size int) *glyphAtlas {
	return &glyphAtlas{
		texture: graphics.CreateEmptyTexture(size, size, render.TextureFormatR8, graphics.ResourceUsageDefault),
		size:    size,
	}
}

func (a *glyphAtlas) allocate(width, height int) (int, int, bool) {
	width += glyphAtlasPadding
	height += glyphAtlasPadding
	if width > a.size || height > a.size {
		return 0, 0, false
	}
	if a.cursorX+width > a.size {
		a.cursorX = 0
		a.cursorY += a.rowH
		a.rowH = 0
	}
	if a.cursorY+height > a.size {
		return 0, 0, false
	}
	x, y := a.cursorX, a.cursorY
	a.cursorX += width
	a.rowH = max(a.rowH, height)
	return x, y, true
}

func (a *glyphAtlas) upload(x, y int, bitmap image.Image) {
	if a.lcd {
		rgba := bitmap.(*image.RGBA)
		a.pending = append(a.pending, graphics.TextureRegionUpdate{X: x, Y: y, Width: rgba.Rect.Dx(), Height: rgba.Rect.Dy(), Pixels: rgba.Pix})
		return
	}
	if bitmap == nil {
		return
	}
	bounds := bitmap.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return
	}
	// The atlas stores coverage only; RGB from the temporary glyph bitmap is
	// intentionally discarded before the update reaches the texture shadow.
	var pixels []byte
	alphaBitmap, isAlpha := bitmap.(*image.Alpha)
	if isAlpha && alphaBitmap.Rect == bounds && alphaBitmap.Stride == width &&
		len(alphaBitmap.Pix) == width*height && bounds.Min == (image.Point{}) {
		pixels = alphaBitmap.Pix
	}
	gray, isGray := bitmap.(*image.Gray)
	if pixels == nil && isGray && gray.Rect == bounds && gray.Stride == width {
		if len(gray.Pix) == width*height && bounds.Min == (image.Point{}) {
			pixels = gray.Pix
		}
	}
	if pixels == nil {
		pixels = make([]byte, width*height)
		if isAlpha {
			for row := range height {
				offset := alphaBitmap.PixOffset(bounds.Min.X, bounds.Min.Y+row)
				copy(pixels[row*width:(row+1)*width], alphaBitmap.Pix[offset:offset+width])
			}
		} else if isGray {
			for row := range height {
				offset := gray.PixOffset(bounds.Min.X, bounds.Min.Y+row)
				copy(pixels[row*width:(row+1)*width], gray.Pix[offset:offset+width])
			}
		} else {
			for row := range height {
				for column := range width {
					_, _, _, alpha := bitmap.At(bounds.Min.X+column, bounds.Min.Y+row).RGBA()
					pixels[row*width+column] = uint8(alpha >> 8)
				}
			}
		}
	}
	a.pending = append(a.pending, graphics.TextureRegionUpdate{
		X: x, Y: y, Width: width, Height: height, Pixels: pixels,
	})
}

func (a *glyphAtlas) flushUploads() {
	if len(a.pending) == 0 {
		return
	}
	a.texture.UpdateRegions(a.pending)
	clear(a.pending)
	a.pending = a.pending[:0]
}

type GlyphCache struct {
	face           *font.ScaledFace
	pool           *glyphAtlasPool
	glyphs         map[rune]uint16
	phasedGlyphs   map[phasedGlyphKey]uint16
	outlinedGlyphs map[outlinedGlyphKey]uint16
	clock          []glyphRecord
	referenced     [glyphCacheEntryLimit / 64]uint64
	entries        int
	lastUse        uint64 // guarded by the owning Font's mutex
	mu             sync.Mutex
	hand           uint16
	ownsPool       bool
}

// Maps contain compact slot indices. The clock owns each glyph and eviction key
// once; hits set one bit instead of moving or copying linked records.
type glyphRecord struct {
	value cachedGlyph
	key   glyphRecordKey
}
type glyphRecordKey struct {
	phase phasedGlyphKey
	width float32
	kind  uint8 // 0: plain, 1: phased, 2: outline
}

type phasedGlyphKey struct {
	character rune
	x, y      uint8
	lcd       bool
	color     font.LCDColor
	preblend  bool
}

type outlinedGlyphKey struct {
	rune  rune
	width float32
}

func NewGlyphCache(f *Font, size, dpi float64) *GlyphCache {
	return newGlyphCache(f, size, dpi, new(glyphAtlasPool), true)
}

func newGlyphCache(f *Font, size, dpi float64, pool *glyphAtlasPool, ownsPool bool) *GlyphCache {
	var face *font.ScaledFace
	if f != nil {
		face = f.FontFace(size, dpi)
	}
	return &GlyphCache{
		face:           face,
		pool:           pool,
		ownsPool:       ownsPool,
		glyphs:         make(map[rune]uint16),
		outlinedGlyphs: make(map[outlinedGlyphKey]uint16),
	}
}

func (c *GlyphCache) Atlas(index int32) (*graphics.Texture, bool) {
	if c == nil || c.pool == nil {
		return nil, false
	}
	return c.pool.atlas(int(index))
}

func (c *GlyphCache) AtlasCount() int {
	if c == nil || c.pool == nil {
		return 0
	}
	return c.pool.count()
}

func (c *GlyphCache) Metrics() font.Metrics {
	if c.face == nil {
		return font.Metrics{}
	}
	return c.face.Metrics()
}

func (c *GlyphCache) Kern(prev, curr rune) float32 {
	if c.face == nil || prev < 0 {
		return 0
	}
	return fixedToFloat32(c.face.Kern(prev, curr))
}

func (c *GlyphCache) Glyph(r rune) Glyph { return c.GlyphAt(r, 0, 0) }

// GlyphMetrics preserves ink bounds and bitmap padding without populating atlases.
// Measurement results are not inserted into the rasterized glyph cache.
func (c *GlyphCache) GlyphMetrics(r rune) Glyph {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.glyphs[r]; ok {
		c.touchEntry(entry)
		return c.clock[entry].value.glyph()
	}
	g := Glyph{Rune: r}
	if c.face == nil {
		return g
	}
	dr, advance, ok := c.face.GlyphBounds(r)
	g.Advance = fixedToFloat32(advance)
	if ok && !dr.Empty() {
		g.Width, g.Height = int32(dr.Dx()+2*glyphBitmapPadding), int32(dr.Dy()+2*glyphBitmapPadding)
		g.OffsetX, g.OffsetY = float32(dr.Min.X-glyphBitmapPadding), float32(dr.Min.Y-glyphBitmapPadding)
	}
	return g
}

// GlyphAt reuses a bounded set of device-pixel phases, avoiding a second
// bilinear blur when an axis-aligned glyph's origin is fractional.
func (c *GlyphCache) GlyphAt(r rune, phaseX, phaseY uint8) Glyph {
	return c.glyphAt(r, phaseX, phaseY, 0, false)
}

func (c *GlyphCache) GlyphGrayAt(r rune, phaseX, phaseY uint8, color font.LCDColor) Glyph {
	return c.glyphAt(r, phaseX, phaseY, color&511, true)
}

func (c *GlyphCache) glyphAt(r rune, phaseX, phaseY uint8, color font.LCDColor, preblend bool) Glyph {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.glyphAtLocked(r, phaseX, phaseY, color, preblend)
}

func (c *GlyphCache) GlyphWithReference(r rune) (Glyph, GlyphRef) {
	c.mu.Lock()
	defer c.mu.Unlock()
	glyph := c.glyphAtLocked(r, 0, 0, 0, false)
	return glyph, c.reference(c.glyphs[r], glyph)
}

func (c *GlyphCache) reference(slot uint16, glyph Glyph) GlyphRef {
	if !glyph.Visible() {
		return GlyphRef{}
	}
	record := &c.clock[slot]
	value := record.value.reference.Value()
	if value == nil {
		value = new(Glyph)
		*value = glyph
		record.value.reference = weak.Make(value)
	}
	return GlyphRef{value: value}
}

func (c *GlyphCache) glyphAtLocked(r rune, phaseX, phaseY uint8, color font.LCDColor, preblend bool) Glyph {
	phaseX, phaseY = phaseX&3, phaseY&3
	key := phasedGlyphKey{character: r, x: phaseX, y: phaseY, color: color, preblend: preblend}
	if !preblend && phaseX == 0 && phaseY == 0 {
		if entry, ok := c.glyphs[r]; ok {
			if g, live := c.resolve(entry); live {
				return g
			}
		}
	} else if entry, ok := c.phasedGlyphs[key]; ok {
		if g, live := c.resolve(entry); live {
			return g
		}
	}
	if c.face == nil {
		g := Glyph{Rune: r}
		c.storeGlyph(key, g)
		return g
	}

	dr, mask, maskp, advance, ok := c.face.Glyph(font.Point26_6{X: font.Int26_6(phaseX) * 16, Y: font.Int26_6(phaseY) * 16}, r)
	if !ok {
		g := Glyph{Rune: r, Advance: fixedToFloat32(advance)}
		c.storeGlyph(key, g)
		return g
	}

	advancePx := fixedToFloat32(advance)
	if dr.Empty() {
		g := Glyph{Rune: r, Advance: advancePx}
		c.storeGlyph(key, g)
		return g
	}

	var bitmap image.Image
	_, colored := mask.(*image.RGBA)
	if colored {
		rgba := image.NewRGBA(image.Rect(0, 0, dr.Dx()+2*glyphBitmapPadding, dr.Dy()+2*glyphBitmapPadding))
		draw.Draw(rgba, rgba.Rect.Inset(glyphBitmapPadding), mask, maskp, draw.Src)
		bitmap = rgba
	} else {
		alpha := glyphBitmap(mask, maskp, dr)
		if preblend {
			color.PreblendGray(alpha)
		}
		bitmap = alpha
	}
	page, x, y := c.allocateFormat(bitmap, colored)

	g := Glyph{
		Rune:    r,
		Atlas:   int32(page.index),
		X:       int32(x),
		Y:       int32(y),
		Width:   int32(bitmap.Bounds().Dx()),
		Height:  int32(bitmap.Bounds().Dy()),
		OffsetX: float32(dr.Min.X - glyphBitmapPadding),
		OffsetY: float32(dr.Min.Y - glyphBitmapPadding),
		Advance: advancePx,
		page:    page,
	}
	c.storeGlyph(key, g)
	return g
}

func (c *GlyphCache) storeGlyph(key phasedGlyphKey, glyph Glyph) {
	if !key.lcd && !key.preblend && key.x == 0 && key.y == 0 {
		old, exists := c.glyphs[key.character]
		slot := c.entrySlot(glyphRecordKey{phase: key}, old, exists)
		c.clock[slot].value = cacheGlyph(glyph)
		c.glyphs[key.character] = slot
		return
	}
	if c.phasedGlyphs == nil {
		c.phasedGlyphs = make(map[phasedGlyphKey]uint16)
	}
	old, exists := c.phasedGlyphs[key]
	slot := c.entrySlot(glyphRecordKey{phase: key, kind: 1}, old, exists)
	c.clock[slot].value = cacheGlyph(glyph)
	c.phasedGlyphs[key] = slot
}

func (c *GlyphCache) resolve(slot uint16) (Glyph, bool) {
	c.touchEntry(slot)
	entry := c.clock[slot].value
	g := entry.glyph()
	if !g.Visible() {
		return g, true
	}
	g.page = entry.page.Value()
	if g.page == nil {
		return Glyph{}, false
	}
	c.pool.touch(g.page)
	return g, true
}

func (c *GlyphCache) touchEntry(slot uint16) { c.referenced[slot/64] |= uint64(1) << (slot % 64) }

func (c *GlyphCache) entrySlot(key glyphRecordKey, old uint16, exists bool) uint16 {
	if exists {
		c.touchEntry(old)
		return old
	}
	var slot uint16
	if c.entries < glyphCacheEntryLimit {
		slot = uint16(c.entries)
		c.clock = append(c.clock, glyphRecord{key: key})
		c.entries++
	} else {
		for {
			slot = c.hand
			c.hand = (c.hand + 1) % glyphCacheEntryLimit
			bit := uint64(1) << (slot % 64)
			if c.referenced[slot/64]&bit == 0 {
				break
			}
			c.referenced[slot/64] &^= bit
		}
		victim := c.clock[slot].key
		switch victim.kind {
		case 0:
			delete(c.glyphs, victim.phase.character)
		case 1:
			delete(c.phasedGlyphs, victim.phase)
		case 2:
			delete(c.outlinedGlyphs, outlinedGlyphKey{victim.phase.character, victim.width})
		}
		c.clock[slot] = glyphRecord{key: key}
	}
	c.touchEntry(slot)
	return slot
}

func (c *GlyphCache) storeOutline(key outlinedGlyphKey, glyph Glyph) {
	old, exists := c.outlinedGlyphs[key]
	slot := c.entrySlot(glyphRecordKey{phase: phasedGlyphKey{character: key.rune}, width: key.width, kind: 2}, old, exists)
	c.clock[slot].value = cacheGlyph(glyph)
	c.outlinedGlyphs[key] = slot
}

func (c *GlyphCache) GlyphLCDAt(r rune, phaseX uint8) Glyph {
	return c.GlyphLCDColorAt(r, phaseX, 0)
}

func (c *GlyphCache) GlyphLCDColorAt(r rune, phaseX uint8, color font.LCDColor) Glyph {
	color &= 511
	c.mu.Lock()
	defer c.mu.Unlock()
	key := phasedGlyphKey{character: r, x: phaseX & 3, lcd: true, color: color}
	if entry, ok := c.phasedGlyphs[key]; ok {
		if glyph, live := c.resolve(entry); live {
			return glyph
		}
	}
	if c.face == nil {
		return Glyph{Rune: r}
	}
	rect, mask, advance, ok := c.face.GlyphLCD(r, key.x)
	glyph := Glyph{Rune: r, Advance: fixedToFloat32(advance)}
	if ok && mask != nil && !rect.Empty() {
		color.PreblendLCD(mask)
		page, x, y := c.allocateFormat(mask, true)
		glyph.Atlas, glyph.X, glyph.Y = int32(page.index), int32(x), int32(y)
		glyph.page = page
		glyph.Width, glyph.Height = int32(mask.Rect.Dx()), int32(mask.Rect.Dy())
		glyph.OffsetX, glyph.OffsetY = float32(rect.Min.X), float32(rect.Min.Y)
	}
	c.storeGlyph(key, glyph)
	return glyph
}

func (c *GlyphCache) OutlineGlyph(r rune, width float32) Glyph {
	if width <= 0 || math.IsNaN(float64(width)) || math.IsInf(float64(width), 0) {
		return c.Glyph(r)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.outlineGlyphLocked(r, width)
}

func (c *GlyphCache) OutlineWithReference(r rune, width float32) GlyphRef {
	if width <= 0 || math.IsNaN(float64(width)) || math.IsInf(float64(width), 0) {
		_, ref := c.GlyphWithReference(r)
		return ref
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	glyph := c.outlineGlyphLocked(r, width)
	return c.reference(c.outlinedGlyphs[outlinedGlyphKey{r, width}], glyph)
}

func (c *GlyphCache) outlineGlyphLocked(r rune, width float32) Glyph {
	key := outlinedGlyphKey{rune: r, width: width}
	if entry, ok := c.outlinedGlyphs[key]; ok {
		if g, live := c.resolve(entry); live {
			return g
		}
	}
	if c.face == nil {
		g := Glyph{Rune: r}
		c.storeOutline(key, g)
		return g
	}

	dr, mask, maskp, advance, ok := c.face.Glyph(font.Point26_6{}, r)
	if !ok || dr.Empty() {
		g := Glyph{Rune: r, Advance: fixedToFloat32(advance)}
		c.storeOutline(key, g)
		return g
	}

	padding := glyphBitmapPadding + int(math.Ceil(float64(width)))
	bitmap := glyphBitmapWithPadding(mask, maskp, dr, padding)
	dilateGlyphBitmap(bitmap, width)
	page, x, y := c.allocate(bitmap)

	g := Glyph{
		Rune:    r,
		Atlas:   int32(page.index),
		X:       int32(x),
		Y:       int32(y),
		Width:   int32(bitmap.Bounds().Dx()),
		Height:  int32(bitmap.Bounds().Dy()),
		OffsetX: float32(dr.Min.X - padding),
		OffsetY: float32(dr.Min.Y - padding),
		Advance: fixedToFloat32(advance),
		page:    page,
	}
	c.storeOutline(key, g)
	return g
}

func (c *GlyphCache) FlushUploads() {
	if c == nil || c.pool == nil {
		return
	}
	c.pool.flushUploads()
}

func (c *GlyphCache) Dispose() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	pool, ownsPool := c.pool, c.ownsPool
	c.face = nil
	c.pool = nil
	c.ownsPool = false
	clear(c.glyphs)
	clear(c.phasedGlyphs)
	c.glyphs = nil
	clear(c.outlinedGlyphs)
	c.phasedGlyphs, c.outlinedGlyphs = nil, nil
	c.entries = 0
	c.clock = nil
	c.referenced = [glyphCacheEntryLimit / 64]uint64{}
	c.hand = 0
	if ownsPool && pool != nil {
		return pool.dispose()
	}
	return nil
}

func (c *GlyphCache) allocate(bitmap image.Image) (*glyphAtlas, int, int) {
	return c.allocateFormat(bitmap, false)
}

func (c *GlyphCache) allocateFormat(bitmap image.Image, lcd bool) (*glyphAtlas, int, int) {
	if c.pool == nil {
		c.pool = new(glyphAtlasPool)
		c.ownsPool = true
	}
	return c.pool.allocateFormat(bitmap, lcd)
}

func (p *glyphAtlasPool) atlas(index int) (*graphics.Texture, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if index < 0 || index >= len(p.atlases) {
		return nil, false
	}
	page := p.atlases[index].page.Value()
	if page == nil {
		return nil, false
	}
	p.retain(page)
	return page.texture, true
}

func (p *glyphAtlasPool) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pages
}

func (p *glyphAtlasPool) touch(page *glyphAtlas) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.retain(page)
}

func (a *glyphAtlas) bytes() int {
	channels := 1
	if a.lcd {
		channels = 4
	}
	return a.size * a.size * channels
}

func (p *glyphAtlasPool) remove(page *glyphAtlas) {
	if page.prev != nil {
		page.prev.next = page.next
	} else {
		p.oldest = page.next
	}
	if page.next != nil {
		page.next.prev = page.prev
	} else {
		p.newest = page.prev
	}
	page.prev, page.next, page.cached = nil, nil, false
	p.cachedBytes -= page.bytes()
	p.cachedPages--
}

func (p *glyphAtlasPool) retain(page *glyphAtlas) {
	limit := p.byteLimit
	if limit <= 0 {
		limit = glyphAtlasByteLimit
	}
	// An oversized live glyph owns its page directly. Do not evict smaller
	// reusable pages for an entry that cannot fit the cache in the first place.
	if page.bytes() > limit {
		return
	}
	if p.newest == page {
		return
	}
	if page.cached {
		p.remove(page)
	}
	page.prev, page.cached = p.newest, true
	if p.newest != nil {
		p.newest.next = page
	} else {
		p.oldest = page
	}
	p.newest = page
	p.cachedBytes += page.bytes()
	p.cachedPages++
	for p.cachedBytes > limit {
		p.remove(p.oldest)
	}
}

// Only the render/owning thread disposes textures. Weak page references let
// GC identify unused pages without invoking driver methods from a finalizer.
func (p *glyphAtlasPool) releaseSlot(slot *glyphAtlasSlot) {
	if slot.texture == nil {
		return
	}
	if err := slot.texture.Dispose(); err != nil {
		p.err = errors.Join(p.err, err)
		return
	}
	*slot = glyphAtlasSlot{}
	p.pages--
}

func (p *glyphAtlasPool) allocateFormat(bitmap image.Image, lcd bool) (*glyphAtlas, int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	width, height := bitmap.Bounds().Dx(), bitmap.Bounds().Dy()
	free := -1
	for index := range p.atlases {
		slot := &p.atlases[index]
		atlas := slot.page.Value()
		if atlas == nil {
			p.releaseSlot(slot)
			if slot.texture == nil {
				free = index
			}
			continue
		}
		if atlas.lcd != lcd {
			continue
		}
		if x, y, ok := atlas.allocate(width, height); ok {
			atlas.upload(x, y, bitmap)
			p.queueUpload(atlas)
			p.retain(atlas)
			return atlas, x, y
		}
	}
	size := max(defaultGlyphAtlasSize, nextPowerOfTwo(max(width, height)+glyphAtlasPadding))
	var atlas *glyphAtlas
	if lcd {
		atlas = &glyphAtlas{texture: graphics.CreateEmptyTexture(size, size, render.TextureFormatRGBA8, graphics.ResourceUsageDefault), size: size, lcd: true}
	} else {
		atlas = newGlyphAtlas(size)
	}
	if free < 0 {
		free = len(p.atlases)
		p.atlases = append(p.atlases, glyphAtlasSlot{})
	}
	atlas.index = free
	p.atlases[free] = glyphAtlasSlot{page: weak.Make(atlas), texture: atlas.texture}
	p.pages++
	p.retain(atlas)
	x, y, _ := atlas.allocate(width, height)
	atlas.upload(x, y, bitmap)
	p.queueUpload(atlas)
	return atlas, x, y
}

func (p *glyphAtlasPool) queueUpload(page *glyphAtlas) {
	if !page.queued {
		page.queued = true
		p.dirtyPages = append(p.dirtyPages, page.index)
	}
}

func (p *glyphAtlasPool) flushUploads() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, index := range p.dirtyPages {
		slot := &p.atlases[index]
		if page := slot.page.Value(); page != nil {
			page.flushUploads()
			page.queued = false
		} else {
			p.releaseSlot(slot)
		}
	}
	clear(p.dirtyPages)
	p.dirtyPages = p.dirtyPages[:0]
	// Reclamation is bounded independently of the number of pinned pages or text runs.
	if p.pages == p.cachedPages {
		return
	}
	for range min(8, len(p.atlases)) {
		if p.sweep >= len(p.atlases) {
			p.sweep = 0
		}
		slot := &p.atlases[p.sweep]
		p.sweep++
		if slot.page.Value() == nil {
			p.releaseSlot(slot)
		}
	}
}

func (p *glyphAtlasPool) dispose() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for index := range p.atlases {
		slot := &p.atlases[index]
		if atlas := slot.page.Value(); atlas != nil {
			if atlas.cached {
				p.remove(atlas)
			}
			clear(atlas.pending)
			atlas.pending = nil
		}
		p.releaseSlot(slot)
	}
	p.atlases = nil
	p.pages, p.cachedPages, p.cachedBytes = 0, 0, 0
	p.oldest, p.newest = nil, nil
	clear(p.dirtyPages)
	p.dirtyPages = nil
	p.sweep = 0
	err := p.err
	p.err = nil
	return err
}

func nextPowerOfTwo(value int) int {
	if value <= 1 {
		return 1
	}
	return 1 << bits.Len(uint(value-1))
}

func glyphBitmap(mask image.Image, maskp image.Point, dr image.Rectangle) *image.Alpha {
	return glyphBitmapWithPadding(mask, maskp, dr, glyphBitmapPadding)
}

func glyphBitmapWithPadding(mask image.Image, maskp image.Point, dr image.Rectangle, padding int) *image.Alpha {
	width := dr.Dx() + padding*2
	height := dr.Dy() + padding*2
	bitmap := image.NewAlpha(image.Rect(0, 0, width, height))
	for y := 0; y < dr.Dy(); y++ {
		for x := 0; x < dr.Dx(); x++ {
			_, _, _, a := mask.At(maskp.X+x, maskp.Y+y).RGBA()
			alpha := uint8(a >> 8)
			offset := bitmap.PixOffset(x+padding, y+padding)
			bitmap.Pix[offset] = alpha
		}
	}
	return bitmap
}

func dilateGlyphBitmap(bitmap image.Image, width float32) {
	if bitmap == nil || width <= 0 || math.IsNaN(float64(width)) {
		return
	}
	bounds := bitmap.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 {
		return
	}
	var readAlpha func(x, y int) uint8
	var writeAlpha func(x, y int, value uint8)
	switch value := bitmap.(type) {
	case *image.Alpha:
		readAlpha = func(x, y int) uint8 { return value.Pix[value.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)] }
		writeAlpha = func(x, y int, alpha uint8) { value.Pix[value.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)] = alpha }
	case *image.Gray:
		readAlpha = func(x, y int) uint8 { return value.Pix[value.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)] }
		writeAlpha = func(x, y int, alpha uint8) { value.Pix[value.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)] = alpha }
	case *image.RGBA:
		readAlpha = func(x, y int) uint8 { return value.Pix[value.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)+3] }
		writeAlpha = func(x, y int, alpha uint8) { value.Pix[value.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)+3] = alpha }
	default:
		return
	}
	var localSource [64 * 64]uint8
	source := localSource[:0]
	if size := w * h; size <= cap(source) {
		source = localSource[:size]
	} else {
		source = make([]uint8, size)
	}
	for y := range h {
		for x := range w {
			source[y*w+x] = readAlpha(x, y)
		}
	}
	radius := max(w, h)
	if requested := math.Ceil(float64(width + 1)); requested < float64(radius) {
		radius = int(requested)
	}
	var localKernel [9 * 9]glyphDilationSample
	kernel := localKernel[:0]
	if size := (radius*2 + 1) * (radius*2 + 1); size > cap(kernel) {
		kernel = make([]glyphDilationSample, 0, size)
	}
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			coverage := min(max(width+1-float32(math.Hypot(float64(dx), float64(dy))), 0), 1)
			if coverage > 0 {
				kernel = append(kernel, glyphDilationSample{x: dx, y: dy, coverage: coverage})
			}
		}
	}
	for y := range h {
		for x := range w {
			alpha := source[y*w+x]
			if alpha == 0 {
				continue
			}
			for _, sample := range kernel {
				targetY := y + sample.y
				if targetY < 0 || targetY >= h {
					continue
				}
				targetX := x + sample.x
				if targetX < 0 || targetX >= w {
					continue
				}
				value := uint8(float32(alpha) * sample.coverage)
				writeAlpha(targetX, targetY, max(readAlpha(targetX, targetY), value))
			}
		}
	}
}

type glyphDilationSample struct {
	x, y     int
	coverage float32
}

func fixedToFloat32(v font.Int26_6) float32 {
	return float32(v) / 64
}
