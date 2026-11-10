package app

import "github.com/diiyw/xiterm/internal/workspace"

type menuItem struct {
	label, icon string
	action      buttonAction
}

type buttonAction uint8

const (
	actionNewTab buttonAction = iota
	actionSplitColumns
	actionSplitRows
	actionCopy
	actionPaste
	actionSearch
	actionProfiles
	actionSettings
	actionHelp
	actionClosePane
	actionSwitchTab
	actionCloseTab
	actionNextMatch
	actionCloseOverlay
	actionOpenProfile
	actionFontDelta
	actionSaveConfig
)

var menuItems = [...]menuItem{
	{"New tab", "plus", actionNewTab},
	{"Split columns", "columns", actionSplitColumns},
	{"Split rows", "rows", actionSplitRows},
	{"Copy", "copy", actionCopy},
	{"Paste", "paste", actionPaste},
	{"Find", "search", actionSearch},
	{"Profiles", "profiles", actionProfiles},
	{"Settings", "settings", actionSettings},
	{"Keyboard shortcuts", "help", actionHelp},
	{"Close pane", "close", actionClosePane},
}

func (a *App) activateButton(action buttonAction, value int) {
	switch action {
	case actionNewTab:
		a.runAction(func() error { return a.newTab(a.currentProfile()) })
	case actionSplitColumns:
		a.split(workspace.Columns)
	case actionSplitRows:
		a.split(workspace.Rows)
	case actionCopy:
		a.copySelection()
	case actionPaste:
		a.paste()
	case actionSearch:
		a.toggleSearch()
	case actionProfiles:
		a.overlay = "profiles"
	case actionSettings:
		a.overlay = "settings"
	case actionHelp:
		a.overlay = "help"
	case actionClosePane:
		a.closePane()
	case actionSwitchTab:
		a.switchTab(value)
	case actionCloseTab:
		a.closeTab(value)
	case actionNextMatch:
		a.nextMatch(value)
	case actionCloseOverlay:
		a.overlay = ""
	case actionOpenProfile:
		if value >= 0 && value < len(a.cfg.Profiles) {
			a.runAction(func() error { return a.newTab(a.cfg.Profiles[value]) })
			a.overlay = ""
		}
	case actionFontDelta:
		a.setFontSize(a.cfg.FontSize + value)
	case actionSaveConfig:
		a.saveConfig()
	}
	a.dirty = true
}

func (a *App) menuRect() workspace.Rect {
	w := min(float32(280), a.width-8)
	h := float32(len(menuItems))*a.menuRowHeight() + 12
	return workspace.Rect{X: max(4, min(a.menuX, a.width-w-4)), Y: max(4, min(a.menuY, a.height-h-4)), W: w, H: h}
}

func (a *App) menuRowHeight() float32 {
	return max(1, min(32, (a.height-20)/float32(len(menuItems))))
}

func (a *App) menuItemAt(x, y float32) int {
	r := a.menuRect()
	rowH := a.menuRowHeight()
	index := int((y - r.Y - 6) / rowH)
	if !r.Contains(x, y) || y < r.Y+6 || y >= r.Y+6+float32(len(menuItems))*rowH || index >= len(menuItems) {
		return -1
	}
	return index
}

func (a *App) activateMenu(index int) {
	a.menu = false
	a.buttons = a.buttons[:0]
	if index >= 0 && index < len(menuItems) {
		a.activateButton(menuItems[index].action, 0)
	}
	a.dirty = true
}

func (a *App) drawContextMenu() {
	// The menu owns hit testing until it is dismissed.
	a.buttons = a.buttons[:0]
	r := a.menuRect()
	a.paint.rect(r, border)
	a.paint.rect(workspace.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}, chrome)
	rowH := a.menuRowHeight()
	for i, item := range menuItems {
		y := r.Y + 6 + float32(i)*rowH
		if i == a.menuIndex {
			a.paint.rect(workspace.Rect{X: r.X + 5, Y: y, W: r.W - 10, H: rowH}, surface)
		}
		a.drawIcon(item.icon, workspace.Rect{X: r.X + 14, Y: y + (rowH-16)/2, W: 16, H: 16}, muted)
		a.paint.label(item.label, r.X+42, y+(rowH-a.paint.size)/2-1, foreground, false)
	}
}
