//go:build windows && cgo

package window

import "github.com/onix-engine/onix/input"

const wmSetCursor = 0x0020

var (
	setCursorProc = user32DLL.NewProc("SetCursor")
	systemCursors [input.CursorZoomOut + 1]hcursor
)

func (c *Context) applyPointerCursor(input.Cursor) {
	if c.window == 0 || c.mouseCaptured {
		return
	}
	c.setPointerCursor()
}

func (c *Context) setPointerCursor() {
	cursor := c.Cursor()
	if cursor == input.CursorNone {
		setCursorProc.Call(0)
		return
	}
	// Windows shares native shapes for grab/move and directional resize aliases.
	ids := [...]uintptr{32512, 32513, 32649, 32515, 32646, 32644, 32645, 32643, 32642, 32646, 32646, 32648, 32514, 32650, 32651, 0, 32513, 32646, 32512, 32515, 32649, 32649, 32515, 32515}
	if int(cursor) >= len(ids) {
		cursor = input.CursorDefault
	}
	if systemCursors[cursor] == 0 {
		systemCursors[cursor] = loadCursorW(0, ids[cursor])
	}
	setCursorProc.Call(uintptr(systemCursors[cursor]))
}
