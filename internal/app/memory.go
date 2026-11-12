package app

import (
	"time"

	"github.com/diiyw/xiterm/internal/workspace"
	"github.com/onix-engine/onix/font"
	"github.com/onix-engine/onix/text"
)

const memoryTrimDelay = 2 * time.Second

func (a *App) scheduleMemoryTrim(now time.Time) {
	a.bufferTrimAt = now.Add(memoryTrimDelay)
}

// Run on the rendering thread outside a frame. Cursor blinks do not reset the
// deadline, so a quiet terminal can release burst capacity while still blinking.
func (a *App) trimIdleMemory(now time.Time) error {
	if a.bufferTrimAt.IsZero() || now.Before(a.bufferTrimAt) {
		return nil
	}
	for _, tab := range a.tabs {
		pending := false
		tab.Root.Visit(func(p *workspace.Pane) { pending = pending || p.Session.PendingOutput() })
		if pending {
			a.scheduleMemoryTrim(now)
			return nil
		}
	}
	a.bufferTrimAt = time.Time{}
	for _, tab := range a.tabs {
		tab.Root.Visit(func(p *workspace.Pane) { p.Session.TrimIdleBuffers() })
	}
	for _, f := range []*font.Face{fontFace(a.font), fontFace(a.boldFont), fontFace(a.nerdFont), fontFace(a.uiFont)} {
		if f != nil {
			f.TrimCaches()
		}
	}
	font.TrimSystemFallbackCaches()
	if a.trimRenderBuffers != nil {
		return a.trimRenderBuffers()
	}
	return nil
}

func fontFace(f *text.Font) *font.Face {
	if f == nil {
		return nil
	}
	return f.Face
}
