package app

import "github.com/diiyw/xiterm/internal/workspace"

type menuItem struct {
	label, icon string
	action      func()
}

func (a *App) menuItems() []menuItem {
	return []menuItem{
		{"New tab", "plus", func() { a.runAction(func() error { return a.newTab(a.currentProfile()) }) }},
		{"Split columns", "columns", func() { a.split(workspace.Columns) }},
		{"Split rows", "rows", func() { a.split(workspace.Rows) }},
		{"Copy", "copy", a.copySelection},
		{"Paste", "paste", a.paste},
		{"Find", "search", a.toggleSearch},
		{"Profiles", "profiles", func() { a.overlay = "profiles" }},
		{"Settings", "settings", func() { a.overlay = "settings" }},
		{"Keyboard shortcuts", "help", func() { a.overlay = "help" }},
		{"Close pane", "close", a.closePane},
	}
}

func (a *App) menuRect() workspace.Rect {
	w := min(float32(280), a.width-8)
	h := float32(len(a.menuItems()))*a.menuRowHeight() + 12
	return workspace.Rect{X: max(4, min(a.menuX, a.width-w-4)), Y: max(4, min(a.menuY, a.height-h-4)), W: w, H: h}
}

func (a *App) menuRowHeight() float32 {
	return max(1, min(32, (a.height-20)/float32(len(a.menuItems()))))
}

func (a *App) menuItemAt(x, y float32) int {
	r := a.menuRect()
	rowH := a.menuRowHeight()
	index := int((y - r.Y - 6) / rowH)
	if !r.Contains(x, y) || y < r.Y+6 || y >= r.Y+6+float32(len(a.menuItems()))*rowH || index >= len(a.menuItems()) {
		return -1
	}
	return index
}

func (a *App) activateMenu(index int) {
	items := a.menuItems()
	a.menu = false
	a.buttons = nil
	if index >= 0 && index < len(items) {
		items[index].action()
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
	for i, item := range a.menuItems() {
		y := r.Y + 6 + float32(i)*rowH
		if i == a.menuIndex {
			a.paint.rect(workspace.Rect{X: r.X + 5, Y: y, W: r.W - 10, H: rowH}, surface)
		}
		a.drawIcon(item.icon, workspace.Rect{X: r.X + 14, Y: y + (rowH-16)/2, W: 16, H: 16}, muted)
		a.paint.label(item.label, r.X+42, y+(rowH-a.paint.size)/2-1, foreground, false)
	}
}
