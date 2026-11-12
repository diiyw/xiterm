package font

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"runtime"
)

// Borrow table storage from the face. Glyph lookup and measurement do not
// decode PNGs or allocate; rasterized color glyphs use the regular bounded cache.
type colorGlyphTables struct{ sbix, cblc, cbdt, colr, cpal []byte }

func (c *colorGlyphTables) load(s *sfnt) {
	c.sbix, _ = s.table("sbix")
	c.cblc, _ = s.table("CBLC")
	c.cbdt, _ = s.table("CBDT")
	c.colr, _ = s.table("COLR")
	c.cpal, _ = s.table("CPAL")
}

func colorU16(data []byte, offset int) int {
	if offset < 0 || offset > len(data)-2 {
		return 0
	}
	return int(binary.BigEndian.Uint16(data[offset:]))
}

func colorU32(data []byte, offset int) int {
	if offset < 0 || offset > len(data)-4 {
		return 0
	}
	return int(binary.BigEndian.Uint32(data[offset:]))
}

func colorSlice(data []byte, start, end int) []byte {
	if start < 0 || end < start || end > len(data) {
		return nil
	}
	return data[start:end]
}

type embeddedColorGlyph struct {
	png                       []byte
	x, y, width, height, ppem int // y is the bottom edge, positive upwards
}

func strikeDistance(ppem int, size float64) float64 {
	if ppem <= 0 {
		return math.Inf(1)
	}
	// Prefer the closest larger strike, avoiding upscaling when possible.
	if float64(ppem) >= size {
		return float64(ppem) - size
	}
	return 65536 + size - float64(ppem)
}

func (c *colorGlyphTables) embedded(id uint16, size float64) (best embeddedColorGlyph) {
	distance := math.Inf(1)
	for i, count := 0, min(colorU32(c.sbix, 4), max(0, len(c.sbix)-8)/4); i < count; i++ {
		strike := colorSlice(c.sbix, colorU32(c.sbix, 8+i*4), len(c.sbix))
		ppem := colorU16(strike, 0)
		if d := strikeDistance(ppem, size); d >= distance {
			continue
		}
		glyph := int(id)
		var data []byte
		for depth := 0; depth < 8; depth++ {
			start, end := colorU32(strike, 4+glyph*4), colorU32(strike, 8+glyph*4)
			data = colorSlice(strike, start, end)
			if len(data) < 8 || string(data[4:8]) != "dupe" {
				break
			}
			if len(data) < 10 {
				data = nil
				break
			}
			glyph = colorU16(data, 8)
		}
		if len(data) < 32 || string(data[4:8]) != "png " {
			continue
		}
		pngData := data[8:]
		if string(pngData[:8]) != "\x89PNG\r\n\x1a\n" {
			continue
		}
		best = embeddedColorGlyph{png: pngData, ppem: ppem, x: int(int16(colorU16(data, 0))), y: int(int16(colorU16(data, 2))), width: colorU32(pngData, 16), height: colorU32(pngData, 20)}
		distance = strikeDistance(ppem, size)
	}
	for i, count := 0, min(colorU32(c.cblc, 4), max(0, len(c.cblc)-8)/48); i < count; i++ {
		strike := c.cblc[8+i*48 : 8+(i+1)*48]
		ppem := int(strike[45])
		if strikeDistance(ppem, size) >= distance || int(id) < colorU16(strike, 40) || int(id) > colorU16(strike, 42) {
			continue
		}
		base, count := colorU32(strike, 0), colorU32(strike, 8)
		count = min(count, max(0, len(c.cblc)-base)/8)
		for j := 0; j < count; j++ {
			record := base + j*8
			first, last := colorU16(c.cblc, record), colorU16(c.cblc, record+2)
			if int(id) < first || int(id) > last {
				continue
			}
			sub := colorSlice(c.cblc, base+colorU32(c.cblc, record+4), len(c.cblc))
			format, imageFormat, offset := colorU16(sub, 0), colorU16(sub, 2), colorU32(sub, 4)
			index := int(id) - first
			start, end := 0, 0
			switch format {
			case 1:
				start, end = colorU32(sub, 8+index*4), colorU32(sub, 12+index*4)
			case 3:
				start, end = colorU16(sub, 8+index*2), colorU16(sub, 10+index*2)
			default:
				continue
			}
			data := colorSlice(c.cbdt, offset+start, offset+end)
			metricsSize := 0
			switch imageFormat {
			case 17:
				metricsSize = 5
			case 18:
				metricsSize = 8
			default:
				continue
			}
			if len(data) < metricsSize+4 {
				continue
			}
			pngData := colorSlice(data, metricsSize+4, metricsSize+4+colorU32(data, metricsSize))
			if len(pngData) == 0 {
				continue
			}
			best = embeddedColorGlyph{png: pngData, ppem: ppem, width: int(data[1]), height: int(data[0]), x: int(int8(data[2])), y: int(int8(data[3])) - int(data[0])}
			distance = strikeDistance(ppem, size)
			break
		}
	}
	return best
}

func (c *colorGlyphTables) layers(id uint16) []byte {
	count, base := colorU16(c.colr, 2), colorU32(c.colr, 4)
	if base < 14 || base > len(c.colr) || count > (len(c.colr)-base)/6 {
		return nil
	}
	lo, hi := 0, count
	for lo < hi {
		mid := (lo + hi) / 2
		if colorU16(c.colr, base+mid*6) < int(id) {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == count || colorU16(c.colr, base+lo*6) != int(id) {
		return nil
	}
	record := base + lo*6
	first, n := colorU16(c.colr, record+2), colorU16(c.colr, record+4)
	if first+n > colorU16(c.colr, 12) {
		return nil
	}
	start := colorU32(c.colr, 8) + first*4
	return colorSlice(c.colr, start, start+n*4)
}

func colorPixelBounds(x, y, width, height, ppem int, size float64, phaseX, phaseY uint8) image.Rectangle {
	scale := size / float64(ppem)
	sx, sy := float64(phaseX&3)/4, -float64(phaseY&3)/4
	return image.Rect(int(math.Floor(float64(x)*scale+sx)), int(math.Floor(float64(y)*scale+sy)), int(math.Ceil(float64(x+width)*scale+sx)), int(math.Ceil(float64(y+height)*scale+sy)))
}

func (f *Face) colorBounds(id uint16, size float64, phaseX, phaseY uint8) (image.Rectangle, bool) {
	defer runtime.KeepAlive(f)
	if g := f.color.embedded(id, size); len(g.png) != 0 {
		if g.width <= 0 || g.height <= 0 || g.width > 4096 || g.height > 4096 {
			return image.Rectangle{}, false
		}
		return colorPixelBounds(g.x, g.y, g.width, g.height, g.ppem, size, phaseX, phaseY), true
	}
	layers := f.color.layers(id)
	var bounds image.Rectangle
	for i := 0; i < len(layers); i += 4 {
		b, err := f.GlyphBounds(uint16(colorU16(layers, i)))
		if err != nil {
			return image.Rectangle{}, false
		}
		bounds = bounds.Union(colorPixelBounds(int(b.XMin), int(b.YMin), int(b.XMax)-int(b.XMin), int(b.YMax)-int(b.YMin), int(f.Head.UnitsPerEm), size, phaseX, phaseY))
	}
	return bounds, len(layers) != 0
}

func (f *Face) rasterizeColorGlyph(id uint16, size float64, phaseX, phaseY uint8) (*GlyphBitmap, error) {
	bounds, ok := f.colorBounds(id, size, phaseX, phaseY)
	if !ok {
		return nil, nil
	}
	if bounds.Dx() > 4096 || bounds.Dy() > 4096 {
		return nil, fmt.Errorf("color glyph too large")
	}
	metrics, err := f.HorizontalMetrics(id)
	if err != nil {
		return nil, err
	}
	scale := size / float64(f.Head.UnitsPerEm)
	bitmap := &GlyphBitmap{Color: image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy())), Bounds: bounds, Advance: float64(metrics.AdvanceWidth) * scale, Scale: scale}
	if g := f.color.embedded(id, size); len(g.png) != 0 {
		config, err := png.DecodeConfig(bytes.NewReader(g.png))
		if err != nil {
			return nil, err
		}
		if config.Width != g.width || config.Height != g.height {
			return nil, fmt.Errorf("color glyph PNG dimensions differ from metrics")
		}
		source, err := png.Decode(bytes.NewReader(g.png))
		if err != nil {
			return nil, err
		}
		resizeColorGlyph(bitmap.Color, source, g, bounds, size, phaseX, phaseY)
		return bitmap, nil
	}
	layers := f.color.layers(id)
	for i := 0; i < len(layers); i += 4 {
		layer, err := f.rasterizeOutlineGlyph(uint16(colorU16(layers, i)), size, phaseX, phaseY)
		if err != nil {
			return nil, err
		}
		if layer == nil || layer.Image == nil {
			continue
		}
		index := colorU16(layers, i+2)
		if len(f.color.cpal) < 14 || colorU16(f.color.cpal, 4) == 0 || index >= colorU16(f.color.cpal, 2) {
			return nil, fmt.Errorf("invalid color glyph palette")
		}
		palette := colorU32(f.color.cpal, 8) + (colorU16(f.color.cpal, 12)+index)*4
		entry := colorSlice(f.color.cpal, palette, palette+4)
		if len(entry) != 4 {
			return nil, fmt.Errorf("invalid color glyph palette")
		}

		a := uint16(entry[3])
		fill := image.NewUniform(color.RGBA{uint8(uint16(entry[2]) * a / 255), uint8(uint16(entry[1]) * a / 255), uint8(uint16(entry[0]) * a / 255), entry[3]})
		x, y := layer.Bounds.Min.X-bounds.Min.X, bounds.Max.Y-layer.Bounds.Max.Y
		draw.DrawMask(bitmap.Color, layer.Image.Rect.Add(image.Pt(x, y)), fill, image.Point{}, layer.Image, image.Point{}, draw.Over)
	}
	return bitmap, nil
}

// Area sampling retains small details when reducing system emoji strikes to
// UI sizes. It runs only on a cold glyph/size/phase cache miss.
func resizeColorGlyph(dst *image.RGBA, src image.Image, g embeddedColorGlyph, bounds image.Rectangle, size float64, phaseX, phaseY uint8) {
	scale := size / float64(g.ppem)
	left := float64(g.x)*scale + float64(phaseX&3)/4
	top := -float64(g.y+g.height)*scale + float64(phaseY&3)/4
	for y := 0; y < dst.Rect.Dy(); y++ {
		y0 := max(0, (float64(y-bounds.Max.Y)-top)/scale)
		y1 := min(float64(g.height), (float64(y+1-bounds.Max.Y)-top)/scale)
		for x := 0; x < dst.Rect.Dx(); x++ {
			x0 := max(0, (float64(x+bounds.Min.X)-left)/scale)
			x1 := min(float64(g.width), (float64(x+1+bounds.Min.X)-left)/scale)
			var sums [4]float64
			for sy := int(math.Floor(y0)); sy < int(math.Ceil(y1)); sy++ {
				for sx := int(math.Floor(x0)); sx < int(math.Ceil(x1)); sx++ {
					weight := (min(x1, float64(sx+1)) - max(x0, float64(sx))) * (min(y1, float64(sy+1)) - max(y0, float64(sy)))
					r, g, b, a := src.At(sx, sy).RGBA()
					for i, value := range [4]uint32{r, g, b, a} {
						sums[i] += float64(value) * weight
					}
				}
			}
			offset := dst.PixOffset(x, y)
			for i, sum := range sums {
				dst.Pix[offset+i] = uint8(min(255, math.Round(sum*scale*scale/257)))
			}
		}
	}
}
