// Skia-derived LCD filter and mask preblend mathematics.
// See resources/SKIA_LICENSE for the upstream BSD license.

package font

import (
	"image"
	"math"
	"slices"
)

// GlyphLCD produces raw RGB coverage using Skia's four-sample LCD filter.
// Atlas users apply foreground preblend and retain the resulting bitmap.
func (f *ScaledFace) GlyphLCD(r rune, phaseX uint8) (image.Rectangle, *image.RGBA, Int26_6, bool) {
	if IsGlyphFormatControl(r) {
		return image.Rectangle{}, nil, 0, true
	}
	if f == nil || f.font == nil {
		return image.Rectangle{}, nil, 0, false
	}
	face, id, scale := f.resolveGlyph(r)
	outline, err := face.Outline(id)
	if err != nil || outline == nil {
		return image.Rectangle{}, nil, 0, false
	}
	metrics, err := face.HorizontalMetrics(id)
	if err != nil {
		return image.Rectangle{}, nil, 0, false
	}
	advance := toFixed(float64(metrics.AdvanceWidth) * scale)
	shift := float64(phaseX&3) / 4
	bounds := image.Rect(int(math.Floor(float64(outline.Bounds.XMin)*scale+shift)), int(math.Floor(float64(outline.Bounds.YMin)*scale)), int(math.Ceil(float64(outline.Bounds.XMax)*scale+shift)), int(math.Ceil(float64(outline.Bounds.YMax)*scale)))
	if len(outline.Contours) == 0 || bounds.Empty() {
		return image.Rectangle{}, nil, advance, true
	}
	scratch := acquireGlyphRasterScratch()
	contours := scratch.scaleOutline(outline.Contours, scale, bounds, shift, 0, 4)
	stride := bounds.Dx() * 4
	length := stride * bounds.Dy()
	scratch.mask = slices.Grow(scratch.mask[:0], length)[:length]
	mask := image.Alpha{Pix: scratch.mask, Stride: stride, Rect: image.Rect(0, 0, stride, bounds.Dy())}
	paintCoverage(&mask, contours)
	bitmap := filterLCD(mask.Pix, stride, bounds.Dy())
	releaseGlyphRasterScratch(scratch)
	bounds.Min.X--
	bounds.Max.X++
	return image.Rect(bounds.Min.X, -bounds.Max.Y, bounds.Max.X, -bounds.Min.Y), bitmap, advance, true
}

// filterLCD evaluates the three phase-shifted FIR kernels used by Skia's
// GenerateImageFromPath. Each row of coefficients sums to 0x110, preserving
// Skia's small ink-spread boost before gamma correction.
func filterLCD(samples []byte, stride, height int) *image.RGBA {
	coefficients := [3][12]int{
		{3, 11, 28, 51, 64, 57, 36, 16, 5, 1, 0, 0},
		{0, 2, 8, 22, 43, 61, 61, 43, 22, 8, 2, 0},
		{0, 0, 1, 5, 16, 36, 57, 64, 51, 28, 11, 3},
	}
	bitmap := image.NewRGBA(image.Rect(0, 0, stride/4+2, height))
	for y := 0; y < height; y++ {
		row := samples[y*stride : (y+1)*stride]
		for x := 0; x < bitmap.Rect.Dx(); x++ {
			sums := [3]int{}
			start := x*4 - 8
			for sample := max(0, start); sample < min(stride, start+12); sample++ {
				value := int(row[sample])
				tap := sample - start
				for c := 0; c < 3; c++ {
					sums[c] += coefficients[c][tap] * value
				}
			}
			out := bitmap.Pix[y*bitmap.Stride+x*4:]
			for c := 0; c < 3; c++ {
				out[c] = uint8(min(255, sums[c]>>8))
			}
			out[3] = 255
		}
	}
	return bitmap
}
