package app

import (
	"github.com/diiyw/xiterm/internal/config"
	"github.com/diiyw/xiterm/internal/workspace"
	"github.com/onix-engine/onix/graphics"
)

var (
	background, chrome, surface, border  graphics.ColorRGBA
	foreground, muted, accent, selection graphics.ColorRGBA
	searchHighlight                      graphics.ColorRGBA
)

type uiTheme struct {
	background, chrome, surface, border  graphics.ColorRGBA
	foreground, muted, accent, selection graphics.ColorRGBA
	searchHighlight                      graphics.ColorRGBA
}

func makeUITheme(theme config.Theme) uiTheme {
	return uiTheme{
		background:      rgb(theme.Background),
		chrome:          rgb(theme.Chrome),
		surface:         rgb(theme.Surface),
		border:          rgb(theme.Border),
		foreground:      rgb(theme.Foreground),
		muted:           rgb(theme.Muted),
		accent:          rgb(theme.Accent),
		selection:       rgb(theme.Selection),
		searchHighlight: rgb(theme.SearchHighlight),
	}
}

func (a *App) setTheme(name string) {
	if name == "" {
		name = config.DefaultTheme
	}
	theme := config.ThemeByName(name)
	a.cfg.Theme = theme.Name
	a.theme = makeUITheme(theme)
	background, chrome, surface, border = a.theme.background, a.theme.chrome, a.theme.surface, a.theme.border
	foreground, muted, accent, selection = a.theme.foreground, a.theme.muted, a.theme.accent, a.theme.selection
	searchHighlight = a.theme.searchHighlight
	for _, tab := range a.tabs {
		tab.Root.Visit(func(pane *workspace.Pane) { pane.Session.ApplyTheme(theme) })
	}
	a.dirty = true
}

func themeLabel(name string) string {
	switch name {
	case "dark":
		return "Dark"
	case "solarized":
		return "Solarized"
	default:
		return "Light"
	}
}
