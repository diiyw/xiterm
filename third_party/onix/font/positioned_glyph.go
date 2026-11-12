package font

// PositionedGlyph carries a glyph's authored position in logical pixels.
// PenX is the pen origin; X includes the glyph's left side bearing.
type PositionedGlyph struct {
	Rune    rune
	PenX    float32
	X       float64
	Y       float64
	Advance float64
}
