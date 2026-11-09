package app

import (
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/diiyw/xiterm/assets"
	"github.com/onix-engine/onix/text"
)

func TestSystemUIFontLabelsAndSearchCaret(t *testing.T) {
	terminalFont, err := text.LoadFont(assets.Regular)
	if err != nil {
		t.Fatal(err)
	}
	defer terminalFont.Dispose()
	uiFont, err := loadSystemUIFont()
	if err != nil {
		t.Fatal(err)
	}
	defer uiFont.Dispose()
	a := &App{font: terminalFont, boldFont: terminalFont, uiFont: uiFont, search: true, width: 800}
	a.setFontSize(15)
	defer a.paint.disposeGlyphCaches()
	if a.paint.ui == a.paint.regular || a.paint.labelCache() != a.paint.ui {
		t.Fatal("UI labels must use their own system font cache")
	}
	if got := a.paint.cellW; got != a.paint.regular.GlyphMetrics('M').Advance {
		t.Fatal("UI font changed the terminal grid width")
	}
	a.query = "Wi中文"
	var width float32
	for _, r := range a.query {
		width += a.paint.ui.GlyphMetrics(r).Advance
	}
	if got := a.textInputConfig().Caret.X; got != 70+width {
		t.Fatalf("search caret = %v, want %v for the system UI font", got, 70+width)
	}
	value := "Wi中文 repeated title"
	limit := a.paint.labelWidth("Wi中…")
	short := a.paint.shortenLabel(value, limit)
	if !utf8.ValidString(short) || short == value || a.paint.labelWidth(short) > limit {
		t.Fatalf("title %q does not fit UI width %v", short, limit)
	}
	if got := a.paint.shortenLabel(value, 0); got != "" {
		t.Fatalf("title in zero width = %q", got)
	}
}

func TestSystemChineseFontFallback(t *testing.T) {
	ui, err := loadSystemUIFont()
	if err != nil {
		t.Fatal(err)
	}
	defer ui.Dispose()
	regular, err := text.LoadFont(assets.Regular)
	if err != nil {
		t.Fatal(err)
	}
	defer regular.Dispose()
	bold, err := text.LoadFont(assets.Bold)
	if err != nil {
		t.Fatal(err)
	}
	defer bold.Dispose()
	for name, face := range map[string]*text.Font{"UI": ui, "terminal": regular, "bold terminal": bold} {
		for _, r := range "中文显示腾讯应用宝" {
			owner, id := face.ResolveGlyph(r)
			if id == 0 {
				t.Fatalf("%s has no installed system fallback for %c", name, r)
			}
			bitmap, err := owner.RasterizeGlyph(id, 15)
			if err != nil || bitmap == nil || bitmap.Bounds.Empty() || bitmap.Image == nil || !slices.ContainsFunc(bitmap.Image.Pix, func(alpha uint8) bool { return alpha != 0 }) {
				t.Fatalf("%s cannot render system glyph %c: %v", name, r, err)
			}
		}
	}
}
