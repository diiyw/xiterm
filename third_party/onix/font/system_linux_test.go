//go:build linux && !android

package font

import (
	"os"
	"testing"
)

func TestLinuxDefaultFontUsesFontconfigSansSerif(t *testing.T) {
	reference, ok := platformDefaultFont()
	if !ok {
		t.Skip("Fontconfig has no usable sans-serif UI font")
	}
	if reference.face == "" {
		t.Fatal("Fontconfig returned an empty PostScript name")
	}
	if _, err := os.Stat(reference.path); err != nil {
		t.Fatalf("Fontconfig UI font path %q: %v", reference.path, err)
	}
}
