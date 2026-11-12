package font

import (
	"image"
	"math"
)

type Metrics struct {
	Height  Int26_6
	Ascent  Int26_6
	Descent Int26_6
}

type FaceOptions struct {
	Size float64
	DPI  float64
}

type ScaledFace struct {
	font        *Face
	pixelsPerEm float64
	scale       float64
	metrics     Metrics
}

// IsGlyphFormatControl identifies joiners, variation selectors and emoji tags.
// They remain in the source text for selection but have no standalone ink or advance.
func IsGlyphFormatControl(r rune) bool {
	return r == 0x200c || r == 0x200d || r >= 0xfe00 && r <= 0xfe0f || r >= 0xe0020 && r <= 0xe007f || r >= 0xe0100 && r <= 0xe01ef
}

func NewFace(face *Face, opts *FaceOptions) (*ScaledFace, error) {
	if face == nil {
		return nil, nil
	}

	size := 16.0
	dpi := 72.0
	if opts != nil {
		if opts.Size > 0 {
			size = opts.Size
		}
		if opts.DPI > 0 {
			dpi = opts.DPI
		}
	}

	if face.Head.UnitsPerEm == 0 {
		return &ScaledFace{font: face}, nil
	}

	pixelsPerEm := size * dpi / 72.0
	scale := pixelsPerEm / float64(face.Head.UnitsPerEm)

	ascent := float64(face.Hhea.Ascent) * scale
	descent := math.Abs(float64(face.Hhea.Descent) * scale)
	lineGap := float64(face.Hhea.LineGap) * scale

	return &ScaledFace{
		font:        face,
		pixelsPerEm: pixelsPerEm,
		scale:       scale,
		metrics: Metrics{
			Height:  toFixed(ascent + descent + lineGap),
			Ascent:  toFixed(ascent),
			Descent: toFixed(descent),
		},
	}, nil
}

func (f *ScaledFace) Close() error {
	return nil
}

func (f *ScaledFace) Metrics() Metrics {
	if f == nil {
		return Metrics{}
	}
	return f.metrics
}

func (f *ScaledFace) Kern(left, right rune) Int26_6 {
	if f == nil || f.font == nil || IsGlyphFormatControl(left) || IsGlyphFormatControl(right) {
		return 0
	}

	leftFace, leftID, scale := f.resolveGlyph(left)
	rightFace, rightID := f.font.ResolveGlyph(right)
	if leftFace != rightFace {
		return 0
	}
	value := leftFace.GlyphKerning(leftID, rightID)
	return toFixed(float64(value) * scale)
}

func (f *ScaledFace) resolveGlyph(r rune) (*Face, uint16, float64) {
	face, id := f.font.ResolveGlyph(r)
	scale := f.scale
	if face != f.font && face.Head.UnitsPerEm != 0 {
		scale = f.pixelsPerEm / float64(face.Head.UnitsPerEm)
	}
	return face, id, scale
}

func (f *ScaledFace) GlyphAdvance(r rune) (Int26_6, bool) {
	if IsGlyphFormatControl(r) {
		return 0, true
	}
	if f == nil || f.font == nil {
		return 0, false
	}

	face, glyphID, scale := f.resolveGlyph(r)
	metrics, err := face.HorizontalMetrics(glyphID)
	if err != nil {
		return 0, false
	}
	return toFixed(float64(metrics.AdvanceWidth) * scale), true
}

func (f *ScaledFace) Glyph(dot Point26_6, r rune) (image.Rectangle, image.Image, image.Point, Int26_6, bool) {
	if IsGlyphFormatControl(r) {
		return image.Rectangle{}, nil, image.Point{}, 0, true
	}
	if f == nil || f.font == nil {
		return image.Rectangle{}, nil, image.Point{}, 0, false
	}

	face, glyphID, _ := f.resolveGlyph(r)
	bitmap, err := face.RasterizeGlyphAt(glyphID, f.pixelsPerEm, uint8((dot.X&63)>>4), uint8((dot.Y&63)>>4))
	if err != nil || bitmap == nil {
		return image.Rectangle{}, nil, image.Point{}, 0, false
	}

	bounds := bitmap.Bounds
	dr := image.Rect(bounds.Min.X, -bounds.Max.Y, bounds.Max.X, -bounds.Min.Y).Add(image.Pt(int(dot.X>>6), int(dot.Y>>6)))
	if bitmap.Color != nil {
		return dr, bitmap.Color, image.Point{}, toFixed(bitmap.Advance), true
	}
	return dr, bitmap.Image, image.Point{}, toFixed(bitmap.Advance), true
}

// GlyphBounds matches Glyph at the pixel origin without rasterizing a bitmap.
func (f *ScaledFace) GlyphBounds(r rune) (image.Rectangle, Int26_6, bool) {
	if IsGlyphFormatControl(r) {
		return image.Rectangle{}, 0, true
	}
	if f == nil || f.font == nil || f.font.Head.UnitsPerEm == 0 {
		return image.Rectangle{}, 0, false
	}
	face, glyphID, scale := f.resolveGlyph(r)
	metrics, err := face.HorizontalMetrics(glyphID)
	if err != nil {
		return image.Rectangle{}, 0, false
	}
	if bounds, ok := face.colorBounds(glyphID, f.pixelsPerEm, 0, 0); ok {
		return image.Rect(bounds.Min.X, -bounds.Max.Y, bounds.Max.X, -bounds.Min.Y), toFixed(float64(metrics.AdvanceWidth) * scale), true
	}
	bounds, err := face.GlyphBounds(glyphID)
	if err != nil {
		return image.Rectangle{}, 0, false
	}
	dr := image.Rect(
		int(math.Floor(float64(bounds.XMin)*scale)),
		-int(math.Ceil(float64(bounds.YMax)*scale)),
		int(math.Ceil(float64(bounds.XMax)*scale)),
		-int(math.Floor(float64(bounds.YMin)*scale)),
	)
	return dr, toFixed(float64(metrics.AdvanceWidth) * scale), true
}

func (f *ScaledFace) GlyphOutline(r rune) (*Outline, Int26_6, bool) {
	if f == nil || f.font == nil {
		return nil, 0, false
	}

	face, glyphID, scale := f.resolveGlyph(r)
	metrics, err := face.HorizontalMetrics(glyphID)
	if err != nil {
		return nil, 0, false
	}
	outline, err := face.Outline(glyphID)
	if err != nil {
		return nil, 0, false
	}
	if outline == nil {
		return nil, toFixed(float64(metrics.AdvanceWidth) * scale), true
	}

	scaled := &Outline{
		Contours: make([]OutlineContour, 0, len(outline.Contours)),
		Bounds: Bounds{
			XMin: int16(math.Round(float64(outline.Bounds.XMin) * scale)),
			YMin: int16(math.Round(float64(outline.Bounds.YMin) * scale)),
			XMax: int16(math.Round(float64(outline.Bounds.XMax) * scale)),
			YMax: int16(math.Round(float64(outline.Bounds.YMax) * scale)),
		},
	}
	for _, contour := range outline.Contours {
		scaledContour := make(OutlineContour, 0, len(contour))
		for _, point := range contour {
			scaledContour = append(scaledContour, OutlinePoint{
				X: point.X * scale,
				Y: -point.Y * scale,
			})
		}
		scaled.Contours = append(scaled.Contours, scaledContour)
	}

	return scaled, toFixed(float64(metrics.AdvanceWidth) * scale), true
}

func toFixed(v float64) Int26_6 {
	return Int26_6(math.Round(v * 64))
}
