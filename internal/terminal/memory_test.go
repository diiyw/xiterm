package terminal

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/vt"
)

func TestClearedHistoryReleasesCells(t *testing.T) {
	for _, escape := range []bool{false, true} {
		t.Run(fmt.Sprint("escape=", escape), func(t *testing.T) {
			s := &Session{Term: vt.NewEmulator(80, 3), output: make(chan []byte, 1)}
			defer s.Term.Close()
			fmt.Fprint(s.Term, strings.Repeat("history payload\r\n", 100))
			lines := s.Term.Scrollback().Lines()
			if len(lines) == 0 {
				t.Fatal("missing history fixture")
			}
			if escape {
				// ED3 can be followed by fresh output in the same PTY read.
				s.output <- []byte("\x1b[3Jnew\r\n")
				s.Poll()
			} else {
				s.ClearHistory()
			}
			remaining := s.Term.ScrollbackLen()
			for i, line := range lines[remaining:] {
				if line != nil {
					t.Fatalf("cleared history line %d still retains cells", i+remaining)
				}
			}
		})
	}
}

func TestFindReusesScratchAcrossUnicodeRows(t *testing.T) {
	s := &Session{Term: vt.NewEmulator(30, 4)}
	defer s.Term.Close()
	fmt.Fprint(s.Term, "İ界ABC\r\nx\r\nİ界abc\r\nlast")
	got := s.Find("界a")
	if len(got) != 2 || got[0] != (Match{Row: 0, Col: 1, Width: 3}) || got[1] != (Match{Row: 2, Col: 1, Width: 3}) {
		t.Fatalf("Unicode column mapping = %+v", got)
	}
}

func BenchmarkFindHistory(b *testing.B) {
	s := &Session{Term: vt.NewEmulator(120, 24)}
	defer s.Term.Close()
	fmt.Fprint(s.Term, strings.Repeat("terminal output with 中文 text and a searchable marker\r\n", 10000))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = s.Find("marker")
	}
}
