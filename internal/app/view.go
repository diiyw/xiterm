package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/diiyw/xiterm/internal/terminal"
	"github.com/diiyw/xiterm/internal/workspace"
)

func (a *App) draw() {
	p := &a.paint
	p.canvas.ClearColor(background)
	a.buttons = a.buttons[:0]
	p.rect(workspace.Rect{W: a.width, H: toolbarH}, chrome)
	p.canvas.BindTexture(a.logo)
	p.canvas.DrawAffine([6]float32{28, 0, 12, 0, 28, 9}, [6]float32{1, 0, 0, 0, 1, 0}, 1)
	p.label("XiTerm", 49, 13, accent, true)
	x := float32(132)
	actions := []struct {
		label  string
		action func()
	}{
		{"New tab", func() { a.runAction(func() error { return a.newTab(a.currentProfile()) }) }},
		{"Split |", func() { a.split(workspace.Columns) }},
		{"Split —", func() { a.split(workspace.Rows) }},
		{"Find", a.toggleSearch},
		{"Profiles", func() { a.overlay = "profiles"; a.dirty = true }},
		{"Settings", func() { a.overlay = "settings"; a.dirty = true }},
	}
	for _, item := range actions {
		w := float32(len([]rune(item.label)))*p.cellW + 20
		if x+w > a.width-55 {
			break
		}
		a.addButton(item.label, workspace.Rect{X: x, Y: 7, W: w, H: 32}, false, item.action)
		x += w + 2
	}
	a.addButton("?", workspace.Rect{X: a.width - 42, Y: 7, W: 32, H: 32}, false, func() { a.overlay = "help"; a.dirty = true })
	p.rect(workspace.Rect{Y: toolbarH, W: a.width, H: tabsH}, chrome)
	tabWidth := min(float32(220), max(float32(40), (a.width-48)/float32(max(1, len(a.tabs)))))
	for i, t := range a.tabs {
		r := workspace.Rect{X: float32(i) * tabWidth, Y: toolbarH, W: tabWidth, H: tabsH}
		index := i
		if i == a.active {
			p.rect(r, background)
			p.rect(workspace.Rect{X: r.X + 10, Y: r.Y + r.H - 2, W: r.W - 20, H: 2}, accent)
		}
		title := t.Active.Session.Title
		if len(t.Root.Panes()) > 1 {
			title = fmt.Sprintf("%s · %d", title, len(t.Root.Panes()))
		}
		if t.Active.Session.Exited {
			title = "[exit] " + title
		}
		p.canvas.PushClipRect(r.X, r.Y, r.W-28, r.H)
		p.label(shorten(title, max(1, int((r.W-42)/p.cellW))), r.X+14, r.Y+10, foreground, false)
		p.canvas.PopClipRect()
		a.buttons = append(a.buttons, button{rect: r, action: func() { a.switchTab(index) }})
		a.addButton("×", workspace.Rect{X: r.X + r.W - 29, Y: r.Y + 4, W: 26, H: 29}, false, func() { a.closeTab(index) })
	}
	a.addButton("+", workspace.Rect{X: float32(len(a.tabs))*tabWidth + 4, Y: toolbarH + 4, W: 34, H: 29}, false, func() { a.runAction(func() error { return a.newTab(a.currentProfile()) }) })
	if a.search {
		a.drawSearch()
	}
	if t := a.tab(); t != nil {
		t.Root.Visit(func(pane *workspace.Pane) { a.drawPane(pane, pane == t.Active) })
	}
	if t := a.tab(); t != nil {
		a.drawDividers(t.Root)
	}
	a.drawStatus()
	if a.overlay != "" {
		a.drawOverlay()
	}
}
func (a *App) drawSearch() {
	y := toolbarH + tabsH
	p := &a.paint
	p.rect(workspace.Rect{Y: y, W: a.width, H: searchH}, surface)
	p.label("Find", 15, y+10, muted, false)
	query := a.query
	if query == "" {
		query = "Type to search terminal history…"
	}
	p.canvas.PushClipRect(70, y, a.width-340, searchH)
	p.label(query, 70, y+10, foreground, false)
	p.canvas.PopClipRect()
	count := "No matches"
	if len(a.matches) > 0 {
		count = fmt.Sprintf("%d / %d", a.matchIndex+1, len(a.matches))
	}
	p.label(count, a.width-250, y+10, muted, false)
	a.addButton("Prev", workspace.Rect{X: a.width - 150, Y: y + 3, W: 55, H: 32}, false, func() { a.nextMatch(-1) })
	a.addButton("Next", workspace.Rect{X: a.width - 94, Y: y + 3, W: 55, H: 32}, false, func() { a.nextMatch(1) })
	a.addButton("×", workspace.Rect{X: a.width - 38, Y: y + 3, W: 32, H: 32}, false, a.toggleSearch)
}
func selected(p *workspace.Pane, row, col int) bool {
	if !p.Selected {
		return false
	}
	start, end := p.SelectionStart, p.SelectionEnd
	if start.Row > end.Row || start.Row == end.Row && start.Col > end.Col {
		start, end = end, start
	}
	return (row > start.Row || row == start.Row && col >= start.Col) && (row < end.Row || row == end.Row && col <= end.Col)
}
func (a *App) drawPane(pane *workspace.Pane, active bool) {
	p := &a.paint
	r := pane.Bounds
	s := pane.Session
	p.canvas.PushClipRect(r.X, r.Y, r.W, r.H)
	defer p.canvas.PopClipRect()
	head := chrome
	if active {
		head = surface
	}
	p.rect(workspace.Rect{X: r.X, Y: r.Y, W: r.W, H: paneHeadH}, head)
	title := filepath.Base(s.Profile.Shell) + "  ·  " + s.Profile.Name
	if active {
		title = "› " + title
	} else {
		title = "  " + title
	}
	p.label(shorten(title, max(1, int((r.W-120)/p.cellW))), r.X+12, r.Y+5, muted, false)
	if r.W > 230 {
		p.label(fmt.Sprintf("%d × %d", s.Term.Width(), s.Term.Height()), r.X+r.W-100, r.Y+5, muted, false)
	}
	start := s.HistoryLen() - pane.Scroll
	cols, rows := s.Term.Width(), s.Term.Height()
	visibleMatches := map[int][]terminal.Match{}
	if a.search && active {
		for _, m := range a.matches {
			if m.Row >= start && m.Row < start+rows {
				visibleMatches[m.Row] = append(visibleMatches[m.Row], m)
			}
		}
	}
	x0, y0 := r.X+12, r.Y+paneHeadH+8
	// Paint backgrounds in a separate pass so glyphs can share GPU batches.
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			c := s.Cell(x, start+y)
			bg := fromColor(s.Term.BackgroundColor(), background)
			if c != nil {
				bg = fromColor(s.ResolveColor(c.Style.Bg), bg)
				if c.Style.Attrs&uv.AttrReverse != 0 {
					bg = fromColor(s.ResolveColor(c.Style.Fg), foreground)
				}
			}
			if selected(pane, start+y, x) {
				bg = selection
			}
			if a.search && active {
				for _, m := range visibleMatches[start+y] {
					if m.Row == start+y && x >= m.Col && x < m.Col+m.Width {
						bg = rgb(0x66552e)
						break
					}
				}
			}
			p.rect(workspace.Rect{X: x0 + float32(x)*p.cellW, Y: y0 + float32(y)*p.lineH, W: p.cellW, H: p.lineH}, bg)
		}
	}
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			c := s.Cell(x, start+y)
			if c == nil {
				continue
			}
			fg := fromColor(s.ResolveColor(c.Style.Fg), fromColor(s.Term.ForegroundColor(), foreground))
			bg := fromColor(s.ResolveColor(c.Style.Bg), background)
			p.cell(c, x0+float32(x)*p.cellW, y0+float32(y)*p.lineH, fg, bg)
		}
	}
	if active && !a.search && a.overlay == "" && pane.Scroll == 0 && !s.Term.CursorHidden() && !s.Exited && !a.blink {
		cursor := s.Term.CursorPosition()
		p.rect(workspace.Rect{X: x0 + float32(cursor.X)*p.cellW, Y: y0 + float32(cursor.Y)*p.lineH + 2, W: 2, H: p.lineH - 4}, accent)
	}
	if pane.Scroll > 0 {
		label := fmt.Sprintf("↑ %d lines · type to return", pane.Scroll)
		w := float32(len([]rune(label)))*p.cellW + 20
		p.rect(workspace.Rect{X: r.X + r.W - w - 10, Y: r.Y + r.H - 30, W: w, H: 25}, surface)
		p.label(label, r.X+r.W-w, r.Y+r.H-25, accent, false)
	}
	if s.Exited {
		message := "Session ended · close this pane or open a new tab"
		if s.ExitErr != nil {
			message = "Session ended: " + s.ExitErr.Error()
		}
		p.rect(workspace.Rect{X: r.X, Y: r.Y + r.H - 34, W: r.W, H: 34}, surface)
		p.label(shorten(message, int((r.W-24)/p.cellW)), r.X+12, r.Y+r.H-25, muted, false)
	}
}
func (a *App) drawStatus() {
	p := &a.paint
	y := a.height - statusH
	p.rect(workspace.Rect{Y: y, W: a.width, H: statusH}, chrome)
	message := a.profileLabel()
	if pane := a.pane(); pane != nil {
		dir := pane.Session.Directory
		if home, err := os.UserHomeDir(); err == nil {
			dir = strings.Replace(dir, home, "~", 1)
		}
		message += "   " + dir
	}
	if time.Now().Before(a.noticeUntil) {
		message = a.notice
	}
	p.canvas.PushClipRect(0, y, a.width-240, statusH)
	p.label(message, 15, y+7, muted, false)
	p.canvas.PopClipRect()
	hint := "Ctrl+Shift+T  new tab  ·  F1 help"
	if runtime.GOOS == "darwin" {
		hint = "Cmd+T  new tab  ·  F1 help"
	}
	p.label(hint, a.width-float32(len(hint))*p.cellW-14, y+7, muted, false)
}
func (a *App) drawOverlay() {
	// Overlays replace hit targets so clicks cannot reach a terminal beneath them.
	a.buttons = a.buttons[:0]
	p := &a.paint
	w := min(float32(660), a.width-40)
	h := min(float32(510), a.height-80)
	r := workspace.Rect{X: (a.width - w) / 2, Y: (a.height - h) / 2, W: w, H: h}
	p.rect(workspace.Rect{W: a.width, H: a.height}, withAlpha(background, .8))
	p.rect(r, surface)
	p.canvas.PushClipRect(r.X, r.Y, r.W, r.H)
	defer p.canvas.PopClipRect()
	title := "Keyboard shortcuts"
	if a.overlay == "profiles" {
		title = "Session profiles"
	}
	if a.overlay == "settings" {
		title = "Preferences"
	}
	p.label(title, r.X+24, r.Y+22, foreground, true)
	a.addButton("×", workspace.Rect{X: r.X + r.W - 46, Y: r.Y + 14, W: 32, H: 32}, false, func() { a.overlay = ""; a.dirty = true })
	y := r.Y + 65
	switch a.overlay {
	case "profiles":
		p.label("Open a new tab with a profile", r.X+24, y, muted, false)
		y += 38
		for _, profile := range a.cfg.Profiles {
			profile := profile
			a.addButton(shorten(profile.Name+"  ·  "+filepath.Base(profile.Shell), int((w-70)/p.cellW)), workspace.Rect{X: r.X + 20, Y: y, W: w - 40, H: 40}, true, func() { a.runAction(func() error { return a.newTab(profile) }); a.overlay = "" })
			y += 46
		}
		p.label("Edit profiles in config.json; restart to reload.", r.X+24, r.Y+r.H-40, muted, false)
	case "settings":
		p.label(fmt.Sprintf("Font size    %d px", a.cfg.FontSize), r.X+24, y, foreground, false)
		a.addButton("−", workspace.Rect{X: r.X + w - 120, Y: y - 5, W: 38, H: 32}, false, func() { a.setFontSize(a.cfg.FontSize - 1) })
		a.addButton("+", workspace.Rect{X: r.X + w - 70, Y: y - 5, W: 38, H: 32}, false, func() { a.setFontSize(a.cfg.FontSize + 1) })
		y += 48
		p.label(fmt.Sprintf("Scrollback   %d lines", a.cfg.Scrollback), r.X+24, y, foreground, false)
		y += 40
		p.label("Font         "+fontName(a.cfg.FontPath), r.X+24, y, foreground, false)
		y += 50
		p.label("Configuration file", r.X+24, y, muted, false)
		y += 28
		p.label(shorten(a.configPath, int((w-48)/p.cellW)), r.X+24, y, foreground, false)
		y += 45
		p.label("Profiles, shell, directory and font are configurable.", r.X+24, y, muted, false)
		y += 27
		p.label("Restart after editing config.json.", r.X+24, y, muted, false)
		a.addButton("Save preferences", workspace.Rect{X: r.X + 24, Y: r.Y + r.H - 63, W: 190, H: 38}, true, a.saveConfig)
	default:
		mod := "Ctrl+Shift+"
		if runtime.GOOS == "darwin" {
			mod = "Cmd+"
		}
		rows := [][2]string{{mod + "T", "New tab"}, {mod + "W", "Close pane"}, {mod + "D / E", "Split columns / rows"}, {"Ctrl+Tab / Alt+1…9", "Switch tab"}, {"Alt+Left / Right", "Switch pane"}, {mod + "F", "Find in terminal history"}, {mod + "C / V", "Copy selection / paste"}, {"Shift+PageUp / PageDown", "Scroll history"}, {mod + "+ / −", "Adjust font size"}, {mod + ", / P", "Preferences / profiles"}, {"F11 / Esc", "Fullscreen / dismiss"}}
		for _, row := range rows {
			p.label(row[0], r.X+24, y, accent, false)
			p.label(row[1], r.X+w*.51, y, foreground, false)
			y += 31
		}
	}
}
func fontName(path string) string {
	if path == "" {
		return "JetBrains Mono Nerd Font"
	}
	return filepath.Base(path)
}

func (a *App) drawDividers(n *workspace.Node) {
	if n == nil || n.Pane != nil {
		return
	}
	r := n.Bounds
	if n.Axis == workspace.Columns {
		r.X = n.First.Bounds.X + n.First.Bounds.W
		r.W = 1
	} else {
		r.Y = n.First.Bounds.Y + n.First.Bounds.H
		r.H = 1
	}
	color := border
	if n == a.dragSplit {
		color = accent
	}
	a.paint.rect(r, color)
	a.drawDividers(n.First)
	a.drawDividers(n.Second)
}
