package app

import (
	"net/url"
	"path/filepath"
	"runtime"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/onix-engine/onix/input"
)

func TestLocalDirectoryURL(t *testing.T) {
	dir := t.TempDir()
	urlPath := filepath.ToSlash(dir)
	if runtime.GOOS == "windows" {
		urlPath = "/" + urlPath
	}
	if got := directoryFromURL((&url.URL{Scheme: "file", Path: urlPath}).String()); got != dir {
		t.Fatalf("directory=%q", got)
	}
	if got := directoryFromURL("file://remote.invalid" + dir); got != "" {
		t.Fatal("accepted remote host as local directory")
	}
	if localDirectory("relative") != "" {
		t.Fatal("accepted relative directory")
	}
}
func TestSpecialKeys(t *testing.T) {
	for key, want := range map[input.KeyCode]rune{input.KeyUp: uv.KeyUp, input.KeyF5: uv.KeyF5, input.KeyDelete: uv.KeyDelete} {
		got, ok := specialKey(key)
		if !ok || got != want {
			t.Fatalf("key=%v got=%v want=%v", key, got, want)
		}
	}
}

func TestModifiedNavigation(t *testing.T) {
	if got := modifiedSequence(input.KeyLeft, uv.ModCtrl); got != "\x1b[1;5D" {
		t.Fatalf("Ctrl+Left=%q", got)
	}
	if got := modifiedSequence(input.KeyDelete, uv.ModShift|uv.ModCtrl); got != "\x1b[3;6~" {
		t.Fatalf("Ctrl+Shift+Delete=%q", got)
	}
}
