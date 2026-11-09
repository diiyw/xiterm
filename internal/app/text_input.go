package app

import (
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/diiyw/xiterm/internal/terminal"
	"github.com/onix-engine/onix/input"
)

type textInputState struct {
	config    input.TextInputConfig
	session   *terminal.Session
	search    bool
	composing bool
	cursor    uv.Position
}

// textInputConfig uses the same logical coordinates as the painted text. Onix
// converts the caret to native window coordinates for the IME candidate panel.
func (a *App) textInputConfig() input.TextInputConfig {
	config := a.caretConfig()
	var session *terminal.Session
	if config.Active && !a.search {
		session = a.pane().Session
	}
	state := &a.textInput
	config.Revision = max(1, state.config.Revision)
	if state.config.Revision != 0 && (config.Active != state.config.Active || session != state.session || a.search != state.search) {
		config.Revision++
		state.composing = false
		// Recompute after dropping a composition belonging to the old target.
		caret := a.caretConfig()
		config.Caret = caret.Caret
	}
	state.config, state.session, state.search = config, session, a.search
	return config
}

func (a *App) caretConfig() input.TextInputConfig {
	config := input.TextInputConfig{Multiline: true}
	if a.overlay != "" || a.menu {
		return config
	}
	if a.search {
		x := float32(70)
		x += a.paint.labelWidth(a.query)
		config.Active = true
		config.Caret = input.TextInputRect{
			X: min(x, max(70, a.width-270)), Y: tabsH + 10,
			Width: 2, Height: a.paint.lineH,
		}
		return config
	}
	if pane := a.pane(); pane != nil && !pane.Session.Exited {
		cursor := pane.Session.InputPosition()
		if a.textInput.composing && a.textInput.session == pane.Session && !a.textInput.search {
			cursor = a.textInput.cursor
		}
		cursor.X = max(0, min(cursor.X, pane.Session.Term.Width()-1))
		cursor.Y = max(0, min(cursor.Y, pane.Session.Term.Height()-1))
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

func (a *App) textComposition(event input.TextComposition) {
	if event.Active && !a.textInput.composing {
		if pane := a.pane(); pane != nil {
			a.textInput.cursor = pane.Session.InputPosition()
		}
	}
	a.textInput.composing = event.Active
	a.dirty = true
}
