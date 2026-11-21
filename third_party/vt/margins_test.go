package vt

import "testing"

func TestOutOfBoundsMarginsAreIgnored(t *testing.T) {
	for name, sequence := range map[string]string{
		"bottom": "\x1b[1;6r",
		"top":    "\x1b[6;7r",
		"right":  "\x1b[1;11s",
		"left":   "\x1b[11;12s",
	} {
		for _, screen := range []string{"main", "alt"} {
			t.Run(name+"/"+screen, func(t *testing.T) {
				term := NewEmulator(10, 5)
				defer term.Close()
				if screen == "alt" {
					term.WriteString("\x1b[?1049h")
				}
				term.WriteString("\x1b[?69h\x1b[2;9s\x1b[2;4r\x1b[3;4H")
				region, cursor := term.scr.ScrollRegion(), term.CursorPosition()
				// PTY reads may split a control sequence at any byte.
				for i := range len(sequence) {
					term.WriteString(sequence[i : i+1])
				}
				if got := term.scr.ScrollRegion(); got != region {
					t.Fatalf("invalid margins changed region: got %v, want %v", got, region)
				}
				if got := term.CursorPosition(); got != cursor {
					t.Fatalf("invalid margins moved cursor: got %v, want %v", got, cursor)
				}
			})
		}
	}
}

func TestOutputAfterOutOfBoundsMargins(t *testing.T) {
	for name, operation := range map[string]string{
		"delete line": "\x1b[M", "insert line": "\x1b[L",
		"delete cell": "\x1b[P", "insert cell": "\x1b[@",
		"scroll up": "\x1b[S", "scroll down": "\x1b[T",
		"line feed": "\r\n", "reverse index": "\x1bM",
	} {
		t.Run(name, func(t *testing.T) {
			term := NewEmulator(80, 24)
			defer term.Close()
			term.WriteString("\x1b[?69h\x1b[1;999s\x1b[1;999r" + operation)
			term.WriteString("\x1b[HOK")
			if c := term.CellAt(0, 0); c == nil || c.Content != "O" {
				t.Fatal("terminal no longer accepts output")
			}
		})
	}
}
