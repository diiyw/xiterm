package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/diiyw/xiterm/assets"
)

func TestLinuxUIFontUsesCurrentDesktopSetting(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "desktop.ttf")
	pattern := filepath.Join(dir, "pattern")
	if err := os.WriteFile(fixture, assets.Regular, 0600); err != nil {
		t.Fatal(err)
	}
	for name, script := range map[string]string{
		"gsettings": "#!/bin/sh\nprintf \"'Desktop UI 11'\\n\"\n",
		"fc-match":  "#!/bin/sh\nprintf '%s' \"$3\" > \"$XITERM_TEST_PATTERN\"\nprintf '%s\\n0' \"$XITERM_TEST_FONT\"\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	t.Setenv("XDG_CURRENT_DESKTOP", "ubuntu:GNOME")
	t.Setenv("XITERM_TEST_FONT", fixture)
	t.Setenv("XITERM_TEST_PATTERN", pattern)
	ui, err := loadSystemUIFont()
	if err != nil {
		t.Fatal(err)
	}
	defer ui.Dispose()
	got, err := os.ReadFile(pattern)
	if err != nil || string(got) != "Desktop UI" {
		t.Fatalf("Fontconfig pattern = %q, %v; want the desktop's selected family", got, err)
	}
	if ui.GlyphIndex('\uf120') == 0 {
		t.Fatal("did not load the font resolved from the desktop setting")
	}
}
