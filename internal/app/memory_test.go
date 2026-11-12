package app

import (
	"runtime"
	"testing"
	"time"
	"weak"

	"github.com/diiyw/xiterm/assets"
	"github.com/onix-engine/onix/text"
	"github.com/onix-engine/onix/window"
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

func TestIdleTrimIsScheduledDespiteBlinkAndRunsOutsidePainting(t *testing.T) {
	a := &App{win: &window.Context{}}
	calls := 0
	a.trimRenderBuffers = func() error { calls++; return nil }
	now := time.Now()
	a.scheduleMemoryTrim(now)
	if deadline, idle := a.idleDeadline(); !idle || deadline != now.Add(memoryTrimDelay) {
		t.Fatal("idle loop did not schedule memory cleanup")
	}
	if err := a.trimIdleMemory(now.Add(time.Second)); err != nil || calls != 0 {
		t.Fatal("memory was trimmed before the idle deadline")
	}
	if err := a.trimIdleMemory(now.Add(memoryTrimDelay)); err != nil || calls != 1 || !a.bufferTrimAt.IsZero() {
		t.Fatal("idle trim did not run exactly once")
	}
	if err := a.trimIdleMemory(now.Add(time.Minute)); err != nil || calls != 1 {
		t.Fatal("idle cleanup kept running without new activity")
	}
}
