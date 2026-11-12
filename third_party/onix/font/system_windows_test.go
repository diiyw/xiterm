//go:build windows

package font

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsFontNameMatch(t *testing.T) {
	tests := []struct {
		name string
		face string
		want int
	}{
		{"Microsoft YaHei & Microsoft YaHei UI (TrueType)", "Microsoft YaHei UI", 2},
		{"Segoe UI (TrueType)", "Segoe UI", 2},
		{"Example Sans Regular (TrueType)", "Example Sans", 1},
		{"Example Sans Bold (TrueType)", "Example Sans", 0},
	}
	for _, test := range tests {
		if got := windowsFontNameMatch(test.name, test.face); got != test.want {
			t.Errorf("windowsFontNameMatch(%q, %q) = %d, want %d", test.name, test.face, got, test.want)
		}
	}
}

func TestWindowsFontSearchDirsIgnoreEmptyLocalAppData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "")
	for _, directory := range fontSearchDirs() {
		if !filepath.IsAbs(directory) {
			t.Fatalf("font search directory is relative: %q", directory)
		}
	}
}

func TestWindowsDefaultFontUsesActiveUIFont(t *testing.T) {
	name := windowsSystemUIFont()
	if name == "" {
		t.Fatal("active Windows UI font name is empty")
	}
	reference, ok := platformDefaultFont()
	if !ok {
		t.Fatalf("active Windows UI font %q was not found in the font registry", name)
	}
	if reference.face != name {
		t.Fatalf("default font face = %q, want %q", reference.face, name)
	}
	if _, err := os.Stat(reference.path); err != nil {
		t.Fatalf("default font path %q: %v", reference.path, err)
	}
	face, err := LoadDefault()
	if err != nil {
		t.Fatalf("load active Windows UI font: %v", err)
	}
	if face.GlyphIndex('A') == 0 {
		t.Fatalf("active Windows UI font %q has no Latin glyphs", name)
	}
	if strings.Contains(strings.ToLower(name), "yahei") && face.GlyphIndex('中') == 0 {
		t.Fatalf("active Windows UI font %q has no Chinese glyphs", name)
	}
}
