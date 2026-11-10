package app

import (
	"image/color"
	"math"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/diiyw/xiterm/assets"
	"github.com/diiyw/xiterm/internal/workspace"
	"github.com/onix-engine/onix/graphics"
	"github.com/onix-engine/onix/text"
)

var (
	background = rgb(0x16191f)
	chrome     = rgb(0x1c2027)
	surface    = rgb(0x252b35)
	border     = rgb(0x303744)
	foreground = rgb(0xd8dee9)
	muted      = rgb(0x8e99ab)
	accent     = rgb(0x98c379)
	selection  = rgb(0x344b65)
)

func rgb(v uint32) graphics.ColorRGBA {
	return graphics.ColorFromRGB(uint8(v>>16), uint8(v>>8), uint8(v))
}
func fromColor(c color.Color, fallback graphics.ColorRGBA) graphics.ColorRGBA {
	if c == nil {
		return fallback
	}
	r, g, b, a := c.RGBA()
	return graphics.ColorFromRGBA(uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8))
}

type painter struct {
	canvas        *graphics.Canvas2D
	solid         *graphics.Texture
	regular, bold *text.GlyphCache
	nerd          *text.GlyphCache
	ui            *text.GlyphCache
	cellW, lineH  float32
	size          float32
}

func (p *painter) disposeGlyphCaches() {
	if p.bold != p.regular {
		_ = p.bold.Dispose()
	}
	_ = p.regular.Dispose()
	_ = p.nerd.Dispose()
	_ = p.ui.Dispose()
	p.regular, p.bold, p.nerd = nil, nil, nil
	p.ui = nil
}

func (p *painter) rect(r workspace.Rect, c graphics.ColorRGBA) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	// Grid fills need hard edges: independently antialiased shapes leave seams
	// where adjacent cells meet, especially at fractional split-pane origins.
	// Snap both edges to physical pixels rather than rounding each width.
	scale := p.canvas.RasterScale()
	snap := func(v float32) float32 { return float32(math.Round(float64(v)*scale) / scale) }
	x, y := snap(r.X), snap(r.Y)
	w, h := snap(r.X+r.W)-x, snap(r.Y+r.H)-y
	if w <= 0 || h <= 0 {
		return
	}
	p.canvas.BindTexture(p.solid)
	p.canvas.DrawTintedAffine([6]float32{w, 0, x, 0, h, y}, [6]float32{1, 0, 0, 0, 1, 0}, c, 1)
}
func (p *painter) label(value string, x, y float32, c graphics.ColorRGBA, bold bool) float32 {
	cache := p.labelCache()
	for _, r := range value {
		g := cache.Glyph(r)
		p.glyph(cache, g, x, y+p.size, c)
		if bold {
			p.glyph(cache, g, x+0.35, y+p.size, c)
		}
		x += g.Advance
	}
	return x
}

func (p *painter) labelCache() *text.GlyphCache {
	if p.ui != nil {
		return p.ui
	}
	return p.regular
}

func (p *painter) labelWidth(value string) float32 {
	var width float32
	for _, r := range value {
		width += p.labelCache().GlyphMetrics(r).Advance
	}
	return width
}

func (p *painter) shortenLabel(value string, width float32) string {
	if p.labelWidth(value) <= width {
		return value
	}
	if width < p.labelWidth("…") {
		return ""
	}
	width -= p.labelWidth("…")
	for i, r := range value {
		width -= p.labelWidth(string(r))
		if width < 0 {
			return value[:i] + "…"
		}
	}
	return value
}

// fittedLabel draws adjacent strings with a shared width limit. Drawing the
// prefix and ellipsis separately avoids allocating a truncated label each frame.
func (p *painter) fittedLabel(width, x, y float32, c graphics.ColorRGBA, values ...string) {
	var total float32
	for _, value := range values {
		total += p.labelWidth(value)
	}
	if total <= width {
		for _, value := range values {
			x = p.label(value, x, y, c, false)
		}
		return
	}
	ellipsis := p.labelWidth("…")
	if width < ellipsis {
		return
	}
	width -= ellipsis
	for _, value := range values {
		var advance float32
		for i, r := range value {
			next := p.labelCache().GlyphMetrics(r).Advance
			if advance+next > width {
				p.label(value[:i], x, y, c, false)
				p.label("…", x+advance, y, c, false)
				return
			}
			advance += next
		}
		p.label(value, x, y, c, false)
		x += advance
		width -= advance
	}
}
func (p *painter) glyph(cache *text.GlyphCache, g text.Glyph, x, baseline float32, c graphics.ColorRGBA) {
	if !g.Visible() {
		return
	}
	atlas, ok := cache.Atlas(g.Atlas)
	if !ok {
		return
	}
	cache.FlushUploads()
	p.canvas.BindTexture(atlas)
	p.canvas.DrawTintedAffine([6]float32{float32(g.Width), 0, x + g.OffsetX, 0, float32(g.Height), baseline + g.OffsetY}, [6]float32{float32(g.Width) / float32(atlas.Width), 0, float32(g.X) / float32(atlas.Width), 0, float32(g.Height) / float32(atlas.Height), float32(g.Y) / float32(atlas.Height)}, c, 1)
	g.KeepAlive(p.canvas)
}
func (p *painter) cell(c *uv.Cell, x, y float32, fg, bg graphics.ColorRGBA) {
	if c == nil || c.Width == 0 {
		return
	}
	if c.Style.Attrs&uv.AttrReverse != 0 {
		fg, bg = bg, fg
	}
	if c.Style.Attrs&uv.AttrFaint != 0 {
		fg.R *= .65
		fg.G *= .65
		fg.B *= .65
	}
	if c.Style.Attrs&uv.AttrConceal == 0 {
		cache := p.regular
		if c.Style.Attrs&uv.AttrBold != 0 {
			cache = p.bold
		}
		// The VT owns cell widths; combining code points share the cell origin.
		for _, r := range c.Content {
			if r == ' ' {
				continue
			}
			actual, g := p.glyphFor(cache, r)
			p.glyph(actual, g, x, y+p.size, fg)
		}
	}
	if c.Style.Underline != uv.UnderlineNone {
		p.rect(workspace.Rect{X: x, Y: y + p.size + 2, W: p.cellW * float32(c.Width), H: 1}, fg)
	}
	if c.Style.Attrs&uv.AttrStrikethrough != 0 {
		p.rect(workspace.Rect{X: x, Y: y + p.lineH*.5, W: p.cellW * float32(c.Width), H: 1}, fg)
	}
}
func shorten(s string, limit int) string {
	count, end := 0, 0
	for i := range s {
		if count == max(0, limit-1) {
			end = i
		}
		count++
		if count > limit {
			return s[:end] + "…"
		}
	}
	return s
}

func withAlpha(c graphics.ColorRGBA, a float32) graphics.ColorRGBA { c.A = a; return c }

func (p *painter) glyphFor(cache *text.GlyphCache, r rune) (*text.GlyphCache, text.Glyph) {
	if p.nerd != nil && assets.IsNerdGlyph(r) {
		cache = p.nerd
	}
	return cache, cache.Glyph(r)
}
