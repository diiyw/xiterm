package app

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/charmbracelet/x/vt"
	"github.com/diiyw/xiterm/internal/terminal"
	"github.com/onix-engine/onix/graphics"
)

func TestWideCellBackground(t *testing.T) {
	defaultBG, defaultFG := rgb(0x16191f), rgb(0xd8dee9)
	for _, tc := range []struct {
		name, sgr string
		want      graphics.ColorRGBA
	}{
		{"true color", "\x1b[48;2;94;92;133m", rgb(0x5e5c85)},
		{"palette", "\x1b[44m", rgb(0x5e5c85)},
		{"reverse", "\x1b[38;2;94;92;133;7m", rgb(0x5e5c85)},
		{"reverse default foreground", "\x1b[7m", defaultFG},
		{"default", "", defaultBG},
	} {
		for _, history := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/history=%t", tc.name, history), func(t *testing.T) {
				s := &terminal.Session{Term: vt.NewEmulator(12, 2)}
				defer s.Term.Close()
				s.Term.SetScrollbackSize(10)
				s.Term.SetIndexedColor(4, color.RGBA{R: 94, G: 92, B: 133, A: 255})
				fmt.Fprint(s.Term, tc.sgr+"中文A文 B\x1b[0m.")
				if history {
					fmt.Fprint(s.Term, "\r\nnext\r\nlast")
					if s.HistoryLen() != 1 {
						t.Fatalf("history length = %d, want 1", s.HistoryLen())
					}
				}
				for _, col := range []int{1, 3, 6} {
					if c := s.Cell(col, 0); c == nil || c.Width != 0 {
						t.Fatalf("column %d is not a wide continuation: %+v", col, c)
					}
				}
				for col := 0; col < s.Term.Width(); col++ {
					want := defaultBG
					if col < 9 {
						want = tc.want
					}
					if got := cellBackground(s, col, 0, defaultBG, defaultFG); got != want {
						t.Errorf("column %d background = %+v, want %+v", col, got, want)
					}
				}
			})
		}
	}
}
