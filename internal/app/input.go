package app

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/diiyw/xiterm/internal/terminal"
	"github.com/diiyw/xiterm/internal/workspace"
	"github.com/onix-engine/onix/input"
)

func directoryFromURL(value string) string {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "file" {
		return ""
	}
	host, _ := os.Hostname()
	if u.Host != "" && u.Host != "localhost" && u.Host != host {
		return ""
	}
	path := u.Path
	if runtime.GOOS == "windows" && len(path) > 2 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return localDirectory(filepath.FromSlash(path))
}
func (a *App) ctrl() bool  { return a.keys[input.KeyLeftControl] || a.keys[input.KeyRightControl] }
func (a *App) shift() bool { return a.keys[input.KeyLeftShift] || a.keys[input.KeyRightShift] }
func (a *App) alt() bool   { return a.keys[input.KeyLeftAlt] || a.keys[input.KeyRightAlt] }
func (a *App) super() bool { return a.keys[input.KeyLeftSuper] || a.keys[input.KeyRightSuper] }

// Reconcile both sides from a native event snapshot; key-up can be lost when
// the window loses focus or macOS consumes a Command shortcut.
func (a *App) syncModifiers(mods uv.KeyMod) {
	for _, pair := range []struct {
		left, right input.KeyCode
		mod         uv.KeyMod
	}{
		{input.KeyLeftShift, input.KeyRightShift, uv.ModShift},
		{input.KeyLeftControl, input.KeyRightControl, uv.ModCtrl},
		{input.KeyLeftAlt, input.KeyRightAlt, uv.ModAlt},
		{input.KeyLeftSuper, input.KeyRightSuper, uv.ModSuper},
	} {
		delete(a.keys, pair.right)
		a.keys[pair.left] = mods&pair.mod != 0
	}
}
func (a *App) shortcut() bool {
	if runtime.GOOS == "darwin" {
		return a.super()
	}
	return a.ctrl() && a.shift()
}
func (a *App) mods() uv.KeyMod {
	var m uv.KeyMod
	if a.ctrl() {
		m |= uv.ModCtrl
	}
	if a.shift() {
		m |= uv.ModShift
	}
	if a.alt() {
		m |= uv.ModAlt
	}
	return m
}

func (a *App) bindInput() {
	a.win.SetTextInputConfig(input.TextInputConfig{Active: true, Multiline: true, Revision: 1})
	a.win.OnKeydown(func(key input.KeyCode) {
		if a.keyModifiers != nil && key != input.KeyCapsLock && (key < input.KeyLeftShift || key > input.KeyRightSuper) {
			if mods, ok := a.keyModifiers(key); ok {
				a.syncModifiers(mods)
			}
		}
		a.keys[key] = true
		a.keydown(key)
		a.dirty = true
		a.blink = false
	})
	a.win.OnKeyup(func(key input.KeyCode) { delete(a.keys, key) })
	a.win.OnTextInput(func(value string) {
		if a.ctrl() || a.super() || a.overlay != "" {
			return
		}
		a.menu = false
		if a.search {
			a.query += value
			a.matchIndex = -1
			a.updateSearch(true)
		} else if p := a.pane(); p != nil {
			if a.alt() {
				value = "\x1b" + value
			}
			p.Session.SendText(value)
			p.Scroll = 0
			p.Selected = false
		}
		a.dirty = true
		a.blink = false
	})
	a.win.OnTextCommand(func(command input.TextCommand) {
		if a.menu {
			if command == input.TextCommandEnter {
				a.activateMenu(a.menuIndex)
			}
			return
		}
		if a.overlay != "" {
			return
		}
		if a.search {
			if command == input.TextCommandBackspace && len(a.query) > 0 {
				_, size := utf8.DecodeLastRuneInString(a.query)
				a.query = a.query[:len(a.query)-size]
				a.matchIndex = -1
				a.updateSearch(true)
			}
			if command == input.TextCommandEnter {
				delta := 1
				if a.shift() {
					delta = -1
				}
				a.nextMatch(delta)
			}
		} else if p := a.pane(); p != nil {
			var code rune
			switch command {
			case input.TextCommandEnter:
				code = uv.KeyEnter
			case input.TextCommandBackspace:
				code = uv.KeyBackspace
			default:
				return
			}
			if code == uv.KeyBackspace && a.ctrl() {
				p.Session.SendText("\x17")
			} else if code == uv.KeyBackspace && a.alt() {
				p.Session.SendText("\x1b\x7f")
			} else {
				p.Session.SendKey(uv.KeyPressEvent{Code: code})
			}
			p.Scroll = 0
			p.Selected = false
		}
		a.dirty = true
	})
	a.win.OnMouseDown(func(x, y float64, b input.MouseButton) { a.mouseDown(float32(x), float32(y), b) })
	a.win.OnMouseMove(func(x, y float64) {
		a.mouseX, a.mouseY = float32(x), float32(y)
		a.dirty = true
		if a.menu {
			a.menuIndex = a.menuItemAt(float32(x), float32(y))
			return
		}
		if a.dragSplit != nil {
			a.dragSplit.DragDivider(float32(x), float32(y))
			a.dirty = true
			return
		}
		if a.mousePane != nil {
			event := a.mouseEvent(a.mousePane, float32(x), float32(y), uv.MouseLeft)
			a.mousePane.Session.SendMouse(uv.MouseMotionEvent(event))
			return
		}
		if pane := a.pane(); pane != nil && a.overlay == "" && !a.search && pane.Session.MouseTracking && !a.shift() && pane.Bounds.Contains(float32(x), float32(y)) {
			pane.Session.SendMouse(uv.MouseMotionEvent(a.mouseEvent(pane, float32(x), float32(y), uv.MouseNone)))
			return
		}
		if a.dragPane != nil {
			a.dragPane.SelectionEnd = a.position(a.dragPane, float32(x), float32(y))
			a.dragPane.Selected = a.dragPane.SelectionStart != a.dragPane.SelectionEnd
			a.dirty = true
		}
	})
	a.win.OnMouseUp(func(x, y float64, b input.MouseButton) {
		if a.mousePane != nil {
			a.mousePane.Session.SendMouse(uv.MouseReleaseEvent(a.mouseEvent(a.mousePane, float32(x), float32(y), uv.MouseLeft)))
			a.mousePane = nil
		}
		if b == input.MouseLeft {
			a.dragPane = nil
			a.dragSplit = nil
		}
	})
	a.win.OnMouseWheel(func(_, dy float64) {
		if a.overlay != "" || a.search || a.menu {
			return
		}
		if p := a.pane(); p != nil {
			if p.Session.MouseTracking && !a.shift() {
				button := uv.MouseWheelUp
				if dy < 0 {
					button = uv.MouseWheelDown
				}
				for range max(1, int(abs(float32(dy))*3)) {
					p.Session.SendMouse(uv.MouseWheelEvent(a.mouseEvent(p, a.mouseX, a.mouseY, button)))
				}
			} else if p.Session.Term.IsAltScreen() {
				code := uv.KeyUp
				if dy < 0 {
					code = uv.KeyDown
				}
				for range max(1, int(abs(float32(dy))*3)) {
					p.Session.SendKey(uv.KeyPressEvent{Code: code})
				}
			} else {
				p.Scroll = max(0, min(p.Session.HistoryLen(), p.Scroll+int(dy*3)))
			}
			a.dirty = true
		}
	})
	a.win.OnFilesDropped(func(_, _ float64, files []string) {
		if a.search || a.overlay != "" {
			return
		}
		if p := a.pane(); p != nil {
			var paths []string
			for _, path := range files {
				paths = append(paths, "'"+strings.ReplaceAll(path, "'", "'\\''")+"'")
			}
			p.Session.Paste(strings.Join(paths, " ") + " ")
		}
	})
}

func (a *App) keydown(key input.KeyCode) {
	if key == input.KeyEscape {
		dismissing := a.search || a.overlay != "" || a.menu
		a.menu = false
		a.search = false
		a.overlay = ""
		a.dragPane = nil
		a.dragSplit = nil
		a.mousePane = nil
		if p := a.pane(); p != nil {
			p.Selected = false
			if !dismissing {
				p.Session.SendKey(uv.KeyPressEvent{Code: uv.KeyEscape})
			}
		}
		return
	}
	if a.menu {
		switch key {
		case input.KeyUp:
			if a.menuIndex < 0 {
				a.menuIndex = len(a.menuItems()) - 1
			} else {
				a.menuIndex = (a.menuIndex - 1 + len(a.menuItems())) % len(a.menuItems())
			}
		case input.KeyDown:
			a.menuIndex = (a.menuIndex + 1) % len(a.menuItems())
		}
		return
	}
	if key == input.KeyF11 {
		a.win.ToggleFullscreen()
		return
	}
	if a.shortcut() {
		switch key {
		case input.KeyT:
			a.runAction(func() error { return a.newTab(a.currentProfile()) })
			return
		case input.KeyW:
			if a.shift() && runtime.GOOS == "darwin" {
				a.closeTab(a.active)
			} else {
				a.closePane()
			}
			return
		case input.KeyD:
			axis := workspace.Columns
			if runtime.GOOS == "darwin" && a.shift() {
				axis = workspace.Rows
			}
			a.split(axis)
			return
		case input.KeyE:
			a.split(workspace.Rows)
			return
		case input.KeyF:
			a.toggleSearch()
			return
		case input.KeyC:
			a.copySelection()
			return
		case input.KeyV:
			a.paste()
			return
		case input.KeyP:
			a.overlay = "profiles"
			return
		case input.KeyComma:
			a.overlay = "settings"
			return
		case input.KeyEqual:
			a.setFontSize(a.cfg.FontSize + 1)
			return
		case input.KeyMinus:
			a.setFontSize(a.cfg.FontSize - 1)
			return
		case input.KeyNum0:
			a.setFontSize(15)
			return
		case input.KeyS:
			a.saveConfig()
			return
		case input.KeyK:
			if p := a.pane(); p != nil {
				p.Session.Term.ClearScrollback()
				p.Scroll = 0
				p.Selected = false
				p.Session.SendText("\x0c")
			}
			return
		}
	}
	if a.ctrl() && key == input.KeyTab {
		delta := 1
		if a.shift() {
			delta = -1
		}
		a.switchTab((a.active + delta + len(a.tabs)) % len(a.tabs))
		return
	}
	if a.alt() && key >= input.KeyNum1 && key <= input.KeyNum9 {
		a.switchTab(int(key - input.KeyNum1))
		return
	}
	if a.alt() && (key == input.KeyLeft || key == input.KeyRight) {
		delta := 1
		if key == input.KeyLeft {
			delta = -1
		}
		a.tab().FocusNext(delta)
		a.resetSearch()
		return
	}
	if key == input.KeyF1 {
		if a.overlay == "help" {
			a.overlay = ""
		} else {
			a.overlay = "help"
		}
		return
	}
	if a.search && a.overlay == "" {
		if key == input.KeyUp {
			a.nextMatch(-1)
		} else if key == input.KeyDown {
			a.nextMatch(1)
		}
		return
	}
	if a.overlay != "" {
		return
	}
	p := a.pane()
	if p == nil {
		return
	}
	if a.shift() && (key == input.KeyPageUp || key == input.KeyPageDown) {
		step := p.Session.Term.Height() - 1
		if key == input.KeyPageDown {
			step = -step
		}
		p.Scroll = max(0, min(p.Session.HistoryLen(), p.Scroll+step))
		return
	}
	if key == input.KeyEnter || key == input.KeyBackspace {
		return
	} // text commands include IME commits
	if a.ctrl() && key >= input.KeyA && key <= input.KeyZ {
		p.Session.SendKey(uv.KeyPressEvent{Code: rune('a' + key - input.KeyA), Mod: uv.ModCtrl})
		p.Scroll = 0
		return
	}
	if a.ctrl() && key == input.KeySpace {
		p.Session.SendText("\x00")
		return
	}
	code, ok := specialKey(key)
	if ok {
		mods := a.mods()
		if sequence := modifiedSequence(key, mods); sequence != "" {
			p.Session.SendText(sequence)
		} else if key == input.KeyTab && a.shift() {
			p.Session.SendText("\x1b[Z")
		} else {
			p.Session.SendKey(uv.KeyPressEvent{Code: code, Mod: mods})
		}
		p.Scroll = 0
		p.Selected = false
	}
}

func specialKey(key input.KeyCode) (rune, bool) {
	switch key {
	case input.KeyTab:
		return uv.KeyTab, true
	case input.KeyEscape:
		return uv.KeyEscape, true
	case input.KeyLeft:
		return uv.KeyLeft, true
	case input.KeyRight:
		return uv.KeyRight, true
	case input.KeyUp:
		return uv.KeyUp, true
	case input.KeyDown:
		return uv.KeyDown, true
	case input.KeyHome:
		return uv.KeyHome, true
	case input.KeyEnd:
		return uv.KeyEnd, true
	case input.KeyInsert:
		return uv.KeyInsert, true
	case input.KeyDelete:
		return uv.KeyDelete, true
	case input.KeyPageUp:
		return uv.KeyPgUp, true
	case input.KeyPageDown:
		return uv.KeyPgDown, true
	}
	if key >= input.KeyF2 && key <= input.KeyF10 {
		return uv.KeyF2 + rune(key-input.KeyF2), true
	}
	if key == input.KeyF12 {
		return uv.KeyF12, true
	}
	return 0, false
}
func (a *App) paste() {
	value, ok := a.win.ClipboardText()
	if !ok {
		a.notify("Clipboard is unavailable")
		return
	}
	if a.search {
		a.query += value
		a.updateSearch(true)
		return
	}
	if a.overlay != "" {
		return
	}
	if p := a.pane(); p != nil {
		p.Session.Paste(value)
		p.Scroll = 0
		p.Selected = false
	}
}
func (a *App) copySelection() {
	p := a.pane()
	if p == nil || !p.Selected {
		a.notify("Drag to select terminal text first")
		return
	}
	if !a.win.SetClipboardText(p.Session.Selection(p.SelectionStart, p.SelectionEnd)) {
		a.notify("Could not copy to clipboard")
	} else {
		a.notify("Selection copied")
	}
}
func (a *App) position(p *workspace.Pane, x, y float32) terminal.Position {
	col := max(0, min(p.Session.Term.Width()-1, int((x-p.Bounds.X-12)/a.paint.cellW)))
	row := max(0, min(p.Session.Term.Height()-1, int((y-p.Bounds.Y-a.paneHeaderHeight(p)-8)/a.paint.lineH)))
	return terminal.Position{Row: p.Session.HistoryLen() - p.Scroll + row, Col: col}
}
func (a *App) mouseDown(x, y float32, b input.MouseButton) {
	a.dirty = true
	if b == input.MouseRight {
		if a.overlay != "" {
			return
		}
		if t := a.tab(); t != nil {
			for _, p := range t.Root.Panes() {
				if p.Bounds.Contains(x, y) {
					if t.Active != p {
						t.Active = p
						a.resetSearch()
					}
					break
				}
			}
		}
		a.dragPane, a.dragSplit, a.mousePane = nil, nil, nil
		a.menu, a.menuX, a.menuY, a.menuIndex = true, x, y, -1
		return
	}
	if a.menu {
		if b == input.MouseLeft {
			a.activateMenu(a.menuItemAt(x, y))
		}
		return
	}
	if b == input.MouseMiddle {
		a.paste()
		return
	}
	if b != input.MouseLeft {
		return
	}
	for i := len(a.buttons) - 1; i >= 0; i-- {
		if a.buttons[i].rect.Contains(x, y) {
			a.buttons[i].action()
			return
		}
	}
	if a.overlay != "" {
		a.overlay = ""
		return
	}
	if t := a.tab(); t != nil {
		if divider := t.Root.DividerAt(x, y); divider != nil {
			a.dragSplit = divider
			return
		}
		for _, p := range t.Root.Panes() {
			if p.Bounds.Contains(x, y) {
				t.Active = p
				a.resetSearch()
				p.Selected = false
				if p.Session.MouseTracking && !a.shift() && y >= p.Bounds.Y+a.paneHeaderHeight(p) {
					a.mousePane = p
					p.Session.SendMouse(uv.MouseClickEvent(a.mouseEvent(p, x, y, uv.MouseLeft)))
					return
				}
				p.SelectionStart = a.position(p, x, y)
				p.SelectionEnd = p.SelectionStart
				a.dragPane = p
				return
			}
		}
	}
}
func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func (a *App) mouseEvent(p *workspace.Pane, x, y float32, button uv.MouseButton) uv.Mouse {
	pos := a.position(p, x, y)
	return uv.Mouse{X: pos.Col, Y: pos.Row - p.Session.HistoryLen() + p.Scroll, Button: button, Mod: a.mods()}
}

// Modified navigation uses xterm's CSI modifier parameter. Unmodified keys
// still pass through the VT emulator to honor application cursor/keypad modes.
func modifiedSequence(key input.KeyCode, mods uv.KeyMod) string {
	if mods == 0 {
		return ""
	}
	parameter := 1
	if mods&uv.ModShift != 0 {
		parameter++
	}
	if mods&uv.ModAlt != 0 {
		parameter += 2
	}
	if mods&uv.ModCtrl != 0 {
		parameter += 4
	}
	final := byte(0)
	switch key {
	case input.KeyUp:
		final = 'A'
	case input.KeyDown:
		final = 'B'
	case input.KeyRight:
		final = 'C'
	case input.KeyLeft:
		final = 'D'
	case input.KeyHome:
		final = 'H'
	case input.KeyEnd:
		final = 'F'
	}
	if final != 0 {
		return fmt.Sprintf("\x1b[1;%d%c", parameter, final)
	}
	code := 0
	switch key {
	case input.KeyInsert:
		code = 2
	case input.KeyDelete:
		code = 3
	case input.KeyPageUp:
		code = 5
	case input.KeyPageDown:
		code = 6
	}
	if code != 0 {
		return fmt.Sprintf("\x1b[%d;%d~", code, parameter)
	}
	return ""
}
