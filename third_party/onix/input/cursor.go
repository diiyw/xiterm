package input

// Cursor is the system pointer shape, independent of keyboard focus.
type Cursor uint8

const (
	CursorDefault Cursor = iota
	CursorText
	CursorPointer
	CursorCrosshair
	CursorMove
	CursorEWResize
	CursorNSResize
	CursorNESWResize
	CursorNWSEResize
	CursorGrab
	CursorGrabbing
	CursorNotAllowed
	CursorWait
	CursorProgress
	CursorHelp
	CursorNone
	CursorVerticalText
	CursorAllScroll
	CursorContextMenu
	CursorCell
	CursorCopy
	CursorAlias
	CursorZoomIn
	CursorZoomOut
)

var cursorNames = [...]string{"default", "text", "pointer", "crosshair", "move", "ew-resize", "ns-resize", "nesw-resize", "nwse-resize", "grab", "grabbing", "not-allowed", "wait", "progress", "help", "none", "vertical-text", "all-scroll", "context-menu", "cell", "copy", "alias", "zoom-in", "zoom-out"}

func (c Cursor) String() string {
	if int(c) >= len(cursorNames) {
		return "default"
	}
	return cursorNames[c]
}

// ParseCursor resolves CSS cursor keywords to native pointer shapes.
func ParseCursor(value string) (Cursor, bool) {
	switch value {
	case "auto":
		return CursorDefault, true
	case "col-resize", "e-resize", "w-resize":
		return CursorEWResize, true
	case "row-resize", "n-resize", "s-resize":
		return CursorNSResize, true
	case "ne-resize", "sw-resize":
		return CursorNESWResize, true
	case "nw-resize", "se-resize":
		return CursorNWSEResize, true
	case "no-drop":
		return CursorNotAllowed, true
	}
	for cursor, name := range cursorNames {
		if value == name {
			return Cursor(cursor), true
		}
	}
	return CursorDefault, false
}

// SetCursor selects the pointer shape for the current UI hit.
func (s *State) SetCursor(cursor Cursor) { s.cursor = cursor }

func (s *State) Cursor() Cursor { return s.cursor }
