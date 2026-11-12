package window

import "github.com/onix-engine/onix/input"

type cursorSupport struct {
	applyCursor func(input.Cursor)
	cursor      input.Cursor
}

// SetCursor changes the system pointer on the window thread. Repeated values
// do not call the platform, so idle frames and text editing do no native work.
func (s *cursorSupport) SetCursor(cursor input.Cursor) {
	if s.cursor == cursor {
		return
	}
	s.cursor = cursor
	if s.applyCursor != nil {
		s.applyCursor(cursor)
	}
}

func (s *cursorSupport) Cursor() input.Cursor { return s.cursor }
