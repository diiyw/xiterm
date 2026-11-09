//go:build darwin || windows

package app

import (
	"bytes"
	"testing"

	"github.com/onix-engine/onix/font"
)

func TestUIFontMatchesNativeSystemFace(t *testing.T) {
	want, err := font.LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	defer want.Close()
	got, err := loadSystemUIFont()
	if err != nil {
		t.Fatal(err)
	}
	defer got.Dispose()
	if !bytes.Equal(got.Data, want.Data) {
		t.Fatal("UI font differs from the current native system font")
	}
	// A collection can contain several faces with identical backing bytes.
	for _, r := range "AWim中" {
		if got.GlyphIndex(r) != want.GlyphIndex(r) {
			t.Fatalf("UI selected the wrong native collection face for %c", r)
		}
	}
}
