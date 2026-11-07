package app

import (
	"image/color"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/onix-engine/onix/graphics"
	"github.com/onix-engine/onix/render"
	"github.com/onix-engine/onix/text"
	"github.com/diiyw/xiterm/assets"
	"github.com/diiyw/xiterm/internal/workspace"
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
	regular, bold *text.GlyphCache
	nerd          *text.GlyphCache
	cellW, lineH  float32
	size          float32
}

func (p *painter) rect(r workspace.Rect, c graphics.ColorRGBA) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	p.canvas.DrawShapeAffine([6]float32{r.W, 0, r.X, 0, r.H, r.Y}, [6]float32{1, 0, 0, 0, 1, 0}, r.W, r.H, render.ShapeRoundedRect, c, graphics.ColorRGBA{}, 0, 0, 1)
}
func (p *painter) label(value string, x, y float32, c graphics.ColorRGBA, bold bool) {
	cache := p.regular
	if bold {
		cache = p.bold
	}
	for _, r := range value {
		actual, g := p.glyphFor(cache, r)
		p.glyph(actual, g, x, y+p.size, c)
		x += g.Advance
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
	r := []rune(s)
	if len(r) > limit {
		return string(r[:max(0, limit-1)]) + "…"
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
