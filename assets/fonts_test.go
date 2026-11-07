package assets

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/onix-engine/onix/font"
)

func TestEmbeddedNerdGlyphs(t *testing.T) {
	for name, data := range map[string][]byte{"regular": Regular, "bold": Bold} {
		face, err := font.Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		defer face.Close()
		for _, r := range []rune{'M', '\ue0b0', '\uf09b', '\uf120', '\U000f02dc'} {
			if face.GlyphIndex(r) == 0 {
				t.Errorf("%s missing glyph U+%04X", name, r)
			}
		}
		m, _ := face.HorizontalMetrics(face.GlyphIndex('M'))
		for _, r := range []rune{'\ue0b0', '\uf09b', '\uf120'} {
			g, _ := face.HorizontalMetrics(face.GlyphIndex(r))
			if g.AdvanceWidth != m.AdvanceWidth {
				t.Errorf("%s U+%04X is not a one-cell glyph", name, r)
			}
		}
	}
}
func TestEmbeddedIcon(t *testing.T) {
	im, err := png.Decode(bytes.NewReader(IconPNG))
	if err != nil {
		t.Fatal(err)
	}
	if im.Bounds().Dx() != 512 || im.Bounds().Dy() != 512 {
		t.Fatal("icon must be 512px")
	}
}
