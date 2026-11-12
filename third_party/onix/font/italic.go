package font

import "math"

// Italic returns a shared synthetic oblique face, or the original face if it
// already contains italic glyphs. The source must remain open while it is used.
// Font tables and metrics are shared; raster and outline caches are independent.
func (f *Face) Italic() *Face {
	if f == nil || f.italic {
		return f
	}
	f.cacheMu.RLock()
	variant := f.italicVariant
	f.cacheMu.RUnlock()
	if variant != nil {
		return variant
	}
	key := f.CacheKey() ^ 0x6974616c69632d31
	if key == 0 {
		key = 1
	}
	f.cacheMu.Lock()
	defer f.cacheMu.Unlock()
	if f.italicVariant == nil {
		f.italicVariant = &Face{
			italicSource: f, italic: true, cacheKey: key,
			Cmap: f.Cmap, Kern: f.Kern, SFNT: f.SFNT, fallback: f.fallback, glyphFallback: f.glyphFallback,
			CFF: f.CFF, color: f.color, Glyf: f.Glyf, Data: f.Data,
			Loca: f.Loca, pairLookups: f.pairLookups, Metrics: f.Metrics,
			weight: f.weight, asciiGlyphs: f.asciiGlyphs, Hhea: f.Hhea,
			Head: f.Head, Maxp: f.Maxp, xHeight: f.xHeight, capHeight: f.capHeight,
		}
	}
	return f.italicVariant
}

// CSS's default oblique angle is 14 degrees. Font coordinates point upwards.
var italicShear = math.Tan(14 * math.Pi / 180)

func italicBounds(b Bounds) Bounds {
	b.XMin = int16(max(-32768, math.Floor(float64(b.XMin)+italicShear*float64(b.YMin))))
	b.XMax = int16(min(32767, math.Ceil(float64(b.XMax)+italicShear*float64(b.YMax))))
	return b
}

func (f *Face) italicOutline(id uint16) (*Outline, error) {
	source, err := f.italicSource.Outline(id)
	if source == nil || err != nil {
		return source, err
	}
	result := &Outline{Bounds: italicBounds(source.Bounds), Contours: make([]OutlineContour, len(source.Contours))}
	for i, contour := range source.Contours {
		result.Contours[i] = make(OutlineContour, len(contour))
		for j, p := range contour {
			result.Contours[i][j] = OutlinePoint{X: p.X + italicShear*p.Y, Y: p.Y}
		}
	}
	return result, nil
}
