package app

import (
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/diiyw/xiterm/internal/terminal"
	"github.com/diiyw/xiterm/internal/workspace"
	"github.com/onix-engine/onix/graphics"
)

var shortcutRows = func() [13][2]string {
	mod := "Ctrl+Shift+"
	if runtime.GOOS == "darwin" {
		mod = "Cmd+"
	}
	return [13][2]string{{mod + "T", "New tab"}, {mod + "W", "Close pane"}, {mod + "D / E", "Split columns / rows"}, {"Ctrl+Tab / Alt+1…9", "Switch tab"}, {"Alt+Left / Right", "Switch pane"}, {mod + "F", "Find in terminal history"}, {mod + "C / V", "Copy selection / paste"}, {"PageUp / PageDown", "Scroll history"}, {"Shift+Home / End", "Oldest history / live screen"}, {"Home / End", "Line start / end"}, {mod + "+ / −", "Adjust font size"}, {mod + ", / P", "Preferences / profiles"}, {"F11 / Esc", "Fullscreen / dismiss"}}
}()

func (a *App) draw() {
	p := &a.paint
	p.canvas.ClearColor(background)
	a.buttons = a.buttons[:0]
	p.rect(workspace.Rect{W: a.width, H: tabsH}, chrome)
	p.canvas.BindTexture(a.logo)
	p.canvas.DrawAffine([6]float32{24, 0, 12, 0, 24, 9}, [6]float32{1, 0, 0, 0, 1, 0}, 1)
	const tabStart float32 = 48
	tabWidth := min(float32(240), max(float32(40), (a.width-tabStart-44)/float32(max(1, len(a.tabs)))))
	for i, t := range a.tabs {
		if i != a.active {
			t.Root.Visit(func(pane *workspace.Pane) { pane.Session.SetViewport(0, 0) })
		}
		r := workspace.Rect{X: tabStart + float32(i)*tabWidth, W: tabWidth, H: tabsH}
		if i == a.active {
			p.rect(r, background)
			p.rect(workspace.Rect{X: r.X + 10, Y: r.H - 2, W: r.W - 20, H: 2}, accent)
		}
		title := t.Active.Session.Title
		var number [32]byte
		var suffix string
		if count := t.Root.PaneCount(); count > 1 {
			buf := append(number[:0], " · "...)
			suffix = string(strconv.AppendInt(buf, int64(count), 10))
		}
		p.canvas.PushClipRect(r.X, r.Y, r.W-32, r.H)
		p.fittedLabel(max(0, r.W-48), r.X+14, 12, foreground, title, suffix)
		p.canvas.PopClipRect()
		a.buttons = append(a.buttons, button{rect: r, action: actionSwitchTab, value: i})
		a.addIconButton("close", workspace.Rect{X: r.X + r.W - 32, Y: 5, W: 28, H: 32}, actionCloseTab, i)
	}
	a.addIconButton("plus", workspace.Rect{X: tabStart + float32(len(a.tabs))*tabWidth + 4, Y: 5, W: 32, H: 32}, actionNewTab, 0)
	if a.search {
		a.drawSearch()
	}
	if t := a.tab(); t != nil {
		t.Root.Visit(func(pane *workspace.Pane) { a.drawPane(pane, pane == t.Active) })
	}
	if t := a.tab(); t != nil {
		a.drawDividers(t.Root)
	}
	a.drawNotice()
	if a.overlay != "" {
		a.drawOverlay()
	}
	if a.menu {
		a.drawContextMenu()
	}
}
func (a *App) drawSearch() {
	y := tabsH
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
	var number [64]byte
	if len(a.matches) > 0 {
		buf := strconv.AppendInt(number[:0], int64(a.matchIndex+1), 10)
		buf = append(buf, " / "...)
		count = string(strconv.AppendInt(buf, int64(len(a.matches)), 10))
	}
	p.label(count, a.width-250, y+10, muted, false)
	a.addIconButton("up", workspace.Rect{X: a.width - 150, Y: y + 3, W: 55, H: 32}, actionNextMatch, -1)
	a.addIconButton("down", workspace.Rect{X: a.width - 94, Y: y + 3, W: 55, H: 32}, actionNextMatch, 1)
	a.addIconButton("close", workspace.Rect{X: a.width - 38, Y: y + 3, W: 32, H: 32}, actionSearch, 0)
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

func cellBackground(s *terminal.Session, col, row int, defaultBG, defaultFG graphics.ColorRGBA) graphics.ColorRGBA {
	c := s.Cell(col, row)
	if c != nil && c.Width == 0 {
		// Wide characters have unstyled continuation cells. Resolve the owning
		// cell before applying per-column selection and search highlights.
		for left := col - 1; left >= 0; left-- {
			previous := s.Cell(left, row)
			if previous == nil {
				break
			}
			if previous.Width > 0 {
				if previous.Width > col-left {
					c = previous
				}
				break
			}
		}
	}
	if c == nil {
		return defaultBG
	}
	if c.Style.Attrs&uv.AttrReverse != 0 {
		return fromColor(s.ResolveColor(c.Style.Fg), defaultFG)
	}
	return fromColor(s.ResolveColor(c.Style.Bg), defaultBG)
}

func (a *App) drawPane(pane *workspace.Pane, active bool) {
	p := &a.paint
	r := pane.Bounds
	s := pane.Session
	p.canvas.PushClipRect(r.X, r.Y, r.W, r.H)
	defer p.canvas.PopClipRect()
	defer func() {
		if status := s.TransferStatus(); status != "" {
			p.rect(workspace.Rect{X: r.X, Y: r.Y + r.H - 30, W: r.W, H: 30}, surface)
			p.fittedLabel(r.W-24, r.X+12, r.Y+r.H-24, accent, status)
		}
	}()
	headerH := a.paneHeaderHeight(pane)
	var number [64]byte
	if headerH > 0 {
		head := chrome
		if active {
			head = surface
		}
		p.rect(workspace.Rect{X: r.X, Y: r.Y, W: r.W, H: headerH}, head)
		p.fittedLabel(max(0, r.W-120), r.X+12, r.Y+4, muted, filepath.Base(s.Profile.Shell), "  ·  ", s.Profile.Name)
		if r.W > 230 {
			buf := strconv.AppendInt(number[:0], int64(s.Term.Width()), 10)
			buf = append(buf, " × "...)
			buf = strconv.AppendInt(buf, int64(s.Term.Height()), 10)
			p.label(string(buf), r.X+r.W-100, r.Y+4, muted, false)
		}
	}
	start := s.HistoryLen() - pane.Scroll
	cols, rows := s.Term.Width(), s.Term.Height()
	s.SetViewport(start, rows)
	var visibleMatches []terminal.Match
	if a.search && active {
		first, _ := slices.BinarySearchFunc(a.matches, start, matchRow)
		last, _ := slices.BinarySearchFunc(a.matches, start+rows, matchRow)
		visibleMatches = a.matches[first:last]
	}
	x0, y0 := r.X+12, r.Y+headerH+8
	defaultBG := fromColor(s.Term.BackgroundColor(), background)
	defaultFG := fromColor(s.Term.ForegroundColor(), foreground)
	// Adjacent cells usually share a background. Submit one rectangle per run
	// rather than thousands of identical cell rectangles on every cursor blink.
	for y := 0; y < rows; y++ {
		rowEnd := 0
		for rowEnd < len(visibleMatches) && visibleMatches[rowEnd].Row == start+y {
			rowEnd++
		}
		rowMatches := visibleMatches[:rowEnd]
		visibleMatches = visibleMatches[rowEnd:]
		matchIndex := 0
		runStart := 0
		runColor := defaultBG
		flush := func(end int) {
			p.rect(workspace.Rect{X: x0 + float32(runStart)*p.cellW, Y: y0 + float32(y)*p.lineH, W: float32(end-runStart) * p.cellW, H: p.lineH}, runColor)
		}
		for x := 0; x < cols; x++ {
			bg := cellBackground(s, x, start+y, defaultBG, defaultFG)
			if selected(pane, start+y, x) {
				bg = selection
			}
			for matchIndex < len(rowMatches) && x >= rowMatches[matchIndex].Col+rowMatches[matchIndex].Width {
				matchIndex++
			}
			if matchIndex < len(rowMatches) && x >= rowMatches[matchIndex].Col {
				bg = rgb(0x66552e)
			}
			if x > 0 && bg != runColor {
				flush(x)
				runStart = x
			}
			runColor = bg
		}
		flush(cols)
	}
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			c := s.Cell(x, start+y)
			if c == nil {
				continue
			}
			fg := fromColor(s.ResolveColor(c.Style.Fg), defaultFG)
			bg := fromColor(s.ResolveColor(c.Style.Bg), background)
			p.cell(c, x0+float32(x)*p.cellW, y0+float32(y)*p.lineH, fg, bg)
		}
	}
	if active && a.cursorCanBlink() && !a.blink {
		cursor := s.Term.CursorPosition()
		p.rect(workspace.Rect{X: x0 + float32(cursor.X)*p.cellW, Y: y0 + float32(cursor.Y)*p.lineH + 2, W: 2, H: p.lineH - 4}, accent)
	}
	if pane.Scroll > 0 {
		buf := strconv.AppendInt(number[:0], int64(pane.Scroll), 10)
		label := string(append(buf, " lines · type to return"...))
		w := p.labelWidth(label) + 20
		p.rect(workspace.Rect{X: r.X + r.W - w - 10, Y: r.Y + r.H - 30, W: w, H: 25}, surface)
		p.label(label, r.X+r.W-w, r.Y+r.H-25, accent, false)
	}
}

func matchRow(m terminal.Match, row int) int {
	return m.Row - row
}
func (a *App) drawNotice() {
	if !time.Now().Before(a.noticeUntil) {
		return
	}
	w := min(a.width-24, float32(len([]rune(a.notice)))*a.paint.cellW+24)
	r := workspace.Rect{X: 12, Y: a.height - 40, W: w, H: 28}
	a.paint.rect(r, surface)
	a.paint.canvas.PushClipRect(r.X, r.Y, r.W, r.H)
	a.paint.label(a.notice, r.X+12, r.Y+5, foreground, false)
	a.paint.canvas.PopClipRect()
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
	a.addIconButton("close", workspace.Rect{X: r.X + r.W - 46, Y: r.Y + 14, W: 32, H: 32}, actionCloseOverlay, 0)
	y := r.Y + 65
	switch a.overlay {
	case "profiles":
		p.label("Open a new tab with a profile", r.X+24, y, muted, false)
		y += 38
		for index, profile := range a.cfg.Profiles {
			r := workspace.Rect{X: r.X + 20, Y: y, W: w - 40, H: 40}
			a.buttons = append(a.buttons, button{rect: r, action: actionOpenProfile, value: index})
			p.rect(r, surface)
			p.fittedLabel(w-70, r.X+10, r.Y+(r.H-p.size)/2-1, foreground, profile.Name, "  ·  ", filepath.Base(profile.Shell))
			y += 46
		}
		p.label("Edit profiles in config.json; restart to reload.", r.X+24, r.Y+r.H-40, muted, false)
	case "settings":
		var number [64]byte
		buf := append(number[:0], "Font size    "...)
		buf = strconv.AppendInt(buf, int64(a.cfg.FontSize), 10)
		p.label(string(append(buf, " px"...)), r.X+24, y, foreground, false)
		a.addIconButton("minus", workspace.Rect{X: r.X + w - 120, Y: y - 5, W: 38, H: 32}, actionFontDelta, -1)
		a.addIconButton("plus", workspace.Rect{X: r.X + w - 70, Y: y - 5, W: 38, H: 32}, actionFontDelta, 1)
		y += 48
		buf = append(number[:0], "Scrollback   "...)
		buf = strconv.AppendInt(buf, int64(a.cfg.Scrollback), 10)
		p.label(string(append(buf, " lines"...)), r.X+24, y, foreground, false)
		y += 40
		p.fittedLabel(w-48, r.X+24, y, foreground, "Font         ", fontName(a.cfg.FontPath))
		y += 50
		p.label("Configuration file", r.X+24, y, muted, false)
		y += 28
		p.fittedLabel(w-48, r.X+24, y, foreground, a.configPath)
		y += 45
		p.label("Profiles, shell, directory and font are configurable.", r.X+24, y, muted, false)
		y += 27
		p.label("Restart after editing config.json.", r.X+24, y, muted, false)
		a.addButton("Save preferences", workspace.Rect{X: r.X + 24, Y: r.Y + r.H - 63, W: 190, H: 38}, true, actionSaveConfig, 0)
	default:
		for _, row := range shortcutRows {
			p.label(row[0], r.X+24, y, accent, false)
			p.label(row[1], r.X+w*.51, y, foreground, false)
			y += 29
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
