package vt

import "testing"

func TestOSCTitlePreservesUTF8(t *testing.T) {
	for _, tc := range []struct {
		name, start, title, end string
	}{
		{"Chinese with BEL", "\x1b]2;", "列出2025年后最新记录", "\a"},
		{"Unicode with ESC backslash", "\x1b]0;", "Ü✜𠜎🌜", "\x1b\\"},
		{"Unicode with standalone ST", "\x9d2;", "本期", "\x9c"},
		{"ST after invalid UTF-8 prefix", "\x1b]2;", "title\xe0", "\x9c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			term := newTestTerminal(t, 80, 1)
			var titles []string
			term.SetCallbacks(Callbacks{Title: func(s string) { titles = append(titles, s) }})
			input := []byte(tc.start + tc.title + tc.end + "OK")
			// PTY reads can split both a UTF-8 character and its terminator.
			for i := range input {
				if _, err := term.Write(input[i : i+1]); err != nil {
					t.Fatal(err)
				}
			}
			if len(titles) != 1 || titles[0] != tc.title {
				t.Errorf("title updates = %q, want [%q]", titles, tc.title)
			}
			if got := term.String(); got != "OK" {
				t.Errorf("screen = %q, want only the text after the title", got)
			}
		})
	}
}

// OSC 8 wire format per https://gist.github.com/egmontkov/eb6100b9 is
// ESC ] 8 ; <params> ; <uri> ST. handleHyperlink must store the
// <params> segment in Link.Params and the <uri> segment in Link.URL.
// Prior to the fix in this commit the two were swapped: Link.URL
// received parts[1] (params) and Link.Params received parts[2] (uri),
// which produced visible-but-unclickable links once Buffer.Render()
// emitted them back out via ansi.SetHyperlink(URL, Params).
func TestHandleHyperlink_StoresURLAndParamsInCorrectFields(t *testing.T) {
	t.Parallel()

	t.Run("empty params, populated uri (common case)", func(t *testing.T) {
		t.Parallel()
		term := newTestTerminal(t, 80, 24)
		// Feed an OSC 8 open + a single character + OSC 8 close so the
		// open's Link metadata sticks on the cell.
		_, _ = term.Write([]byte("\x1b]8;;https://example.com/\x1b\\X\x1b]8;;\x1b\\"))
		cell := term.CellAt(0, 0)
		if cell == nil {
			t.Fatal("cell at (0,0) is nil")
		}
		if got, want := cell.Link.URL, "https://example.com/"; got != want {
			t.Errorf("Link.URL = %q, want %q", got, want)
		}
		if got, want := cell.Link.Params, ""; got != want {
			t.Errorf("Link.Params = %q, want %q", got, want)
		}
	})

	t.Run("populated params and uri", func(t *testing.T) {
		t.Parallel()
		term := newTestTerminal(t, 80, 24)
		_, _ = term.Write([]byte("\x1b]8;id=42:tag=demo;https://example.com/path?x=1\x1b\\X\x1b]8;;\x1b\\"))
		cell := term.CellAt(0, 0)
		if cell == nil {
			t.Fatal("cell at (0,0) is nil")
		}
		if got, want := cell.Link.URL, "https://example.com/path?x=1"; got != want {
			t.Errorf("Link.URL = %q, want %q", got, want)
		}
		if got, want := cell.Link.Params, "id=42:tag=demo"; got != want {
			t.Errorf("Link.Params = %q, want %q", got, want)
		}
	})
}
