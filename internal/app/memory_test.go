package app

import (
	"runtime"
	"testing"
	"weak"

	"github.com/diiyw/xiterm/assets"
	"github.com/onix-engine/onix/text"
)

func TestZoomReleasesOldGlyphCaches(t *testing.T) {
	font, err := text.LoadFont(assets.Regular)
	if err != nil {
		t.Fatal(err)
	}
	defer font.Dispose()
	uiFont, err := loadSystemUIFont()
	if err != nil {
		t.Fatal(err)
	}
	defer uiFont.Dispose()
	a := &App{font: font, boldFont: font, uiFont: uiFont}
	a.setFontSize(15)
	defer a.paint.disposeGlyphCaches()
	previous := weak.Make(a.paint.regular)
	previousUI := weak.Make(a.paint.ui)
	for size := 16; size <= 36; size++ {
		a.setFontSize(size)
	}
	runtime.GC()
	if previous.Value() != nil {
		t.Fatal("font retained an obsolete zoom cache")
	}
	if previousUI.Value() != nil {
		t.Fatal("UI font retained an obsolete zoom cache")
	}
	if a.paint.regular != a.paint.bold {
		t.Fatal("custom font allocated duplicate regular/bold caches")
	}
	current := a.paint.regular
	a.setFontSize(36)
	if a.paint.regular != current {
		t.Fatal("unchanged font size recreated the cache")
	}
	runtime.KeepAlive(a)
}
