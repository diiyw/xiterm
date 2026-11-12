//go:build darwin || ios

package font

import "testing"

func TestAppleLoadsCoreTextSystemFontAndCollectionFace(t *testing.T) {
	ref, ok := platformDefaultFont()
	if !ok || ref.path == "" || ref.face == "" {
		t.Fatalf("CoreText did not resolve the UI font: %+v", ref)
	}
	face, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	defer face.Close()
	if face.GlyphIndex('A') == 0 {
		t.Fatal("system UI font has no Latin glyph")
	}
	t.Logf("CoreText font: %s, face: %s", ref.path, ref.face)
}
