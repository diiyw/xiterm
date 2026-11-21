package app

import (
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/diiyw/xiterm/internal/terminal"
	"github.com/onix-engine/onix/input"
)

type textInputState struct {
	config      input.TextInputConfig
	session     *terminal.Session
	search      bool
	composition input.TextComposition
	cursor      uv.Position
}

const searchTextX float32 = 70

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
		state.composition = input.TextComposition{}
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
		_, x, _ := a.searchTextLayout()
		config.Active = true
		config.Caret = input.TextInputRect{
			X: x, Y: tabsH + 10,
			Width: 2, Height: a.paint.lineH,
		}
		return config
	}
	if pane := a.pane(); pane != nil && !pane.Session.Exited {
		cursor := pane.Session.InputPosition()
		if a.textInput.composition.Active && a.textInput.session == pane.Session && !a.textInput.search {
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

func (a *App) searchTextLayout() (queryX, preeditX, right float32) {
	right = max(searchTextX, a.width-270)
	end := searchTextX + a.paint.labelWidth(a.query)
	queryX, preeditX = searchTextX, min(end, right)
	if a.textInput.search && a.textInput.composition.Text != "" {
		width := a.paint.labelWidth(a.textInput.composition.Text)
		preeditX = min(end, max(searchTextX, right-width))
		queryX += preeditX - end
	}
	return
}

func (a *App) textComposition(event input.TextComposition) {
	state := &a.textInput
	if !event.Active {
		event = input.TextComposition{}
	} else {
		if !state.config.Active || state.search != a.search || a.menu || a.overlay != "" {
			return
		}
		if !state.search {
			pane := a.pane()
			if pane == nil || pane.Session != state.session || pane.Session.Exited {
				return
			}
			if !state.composition.Active {
				state.cursor = pane.Session.InputPosition()
			}
			if event.Text != "" {
				pane.Scroll = 0
			}
		}
	}
	if state.composition != event {
		state.composition = event
		a.dirty = true
	}
}
