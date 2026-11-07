package app

import "github.com/onix-engine/onix/input"

// textInputConfig uses the same logical coordinates as the painted text. Onix
// converts the caret to native window coordinates for the IME candidate panel.
func (a *App) textInputConfig() input.TextInputConfig {
	config := input.TextInputConfig{Multiline: true, Revision: 1}
	if a.overlay != "" || a.menu {
		return config
	}
	if a.search {
		x := float32(70)
		for _, r := range a.query {
			x += a.paint.regular.GlyphMetrics(r).Advance
		}
		config.Active = true
		config.Caret = input.TextInputRect{
			X: min(x, max(70, a.width-270)), Y: tabsH + 10,
			Width: 2, Height: a.paint.lineH,
		}
		return config
	}
	if pane := a.pane(); pane != nil && !pane.Session.Exited {
		cursor := pane.Session.Term.CursorPosition()
		config.Active = true
		config.Caret = input.TextInputRect{
			X:     pane.Bounds.X + 12 + float32(cursor.X)*a.paint.cellW,
			Y:     pane.Bounds.Y + a.paneHeaderHeight(pane) + 8 + float32(cursor.Y)*a.paint.lineH,
			Width: 2, Height: a.paint.lineH,
		}
	}
	// Keep the revision stable when only the caret moves, so repositioning the
	// candidate panel does not discard an in-progress composition.
	return config
}
