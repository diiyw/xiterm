package terminal

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/vt"
)

func TestWebLinkAt(t *testing.T) {
	for _, tc := range []struct {
		name, text, label, target string
	}{
		{"local", "工作清单：http://127.0.0.1:8787", "http://127.0.0.1:8787", "http://127.0.0.1:8787"},
		{"styled", "\x1b[31mhttps://example.com\x1b[0m/path?q=a&b=2#part", "https://example.com/path?q=a&b=2#part", "https://example.com/path?q=a&b=2#part"},
		{"punctuation", "(https://example.com/a_(b)).", "https://example.com/a_(b)", "https://example.com/a_(b)"},
		{"ipv6", "[http://[::1]:8787/]", "http://[::1]:8787/", "http://[::1]:8787/"},
		{"chinese", "地址：https://example.com/页面。", "https://example.com/页面", "https://example.com/页面"},
		{"multiple", "https://one.test https://two.test/path", "https://two.test/path", "https://two.test/path"},
		{"osc8", "中文\x1b]8;;https://example.com/target\x1b\\点这里\x1b]8;;\x1b\\", "点这里", "https://example.com/target"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			term := vt.NewEmulator(120, 4)
			defer term.Close()
			s := &Session{Term: term}
			fmt.Fprint(term, tc.text)
			// Find the visible label using the terminal's own column widths.
			start := -1
			for col := 0; col < term.Width(); col++ {
				if c := s.Cell(col, 0); c == nil || c.Width == 0 {
					continue
				}
				if strings.HasPrefix(s.Selection(Position{0, col}, Position{0, term.Width() - 1}), tc.label) {
					start = col
					break
				}
			}
			if start < 0 {
				t.Fatal("label missing from terminal")
			}
			hit := s.WebLinkAt(Position{0, start})
			if hit.URL != tc.target || hit.Start != start || s.Selection(Position{0, hit.Start}, Position{0, hit.End}) != tc.label {
				t.Fatalf("wrong link: %+v", hit)
			}
			for col := hit.Start; col <= hit.End; col++ {
				if got := s.WebLinkAt(Position{0, col}); got != hit {
					t.Fatalf("column %d: got %+v, want %+v", col, got, hit)
				}
			}
			if s.WebLinkAt(Position{0, hit.End + 1}).URL != "" {
				t.Fatal("link extends into punctuation or padding")
			}
		})
	}
}

func TestWebLinkInPackedHistory(t *testing.T) {
	term := vt.NewEmulator(60, 2)
	defer term.Close()
	s := &Session{Term: term}
	s.initScrollback(10)
	fmt.Fprint(term, "工作清单：http://127.0.0.1:8787\r\nnext\r\nlast")
	if s.HistoryLen() == 0 {
		t.Fatal("expected history")
	}
	if got := s.WebLinkAt(Position{0, 10}); got.URL != "http://127.0.0.1:8787" || got.Start != 10 {
		t.Fatalf("history link=%+v", got)
	}
	for _, pos := range []Position{{-1, 0}, {99, 0}, {0, -1}, {0, 60}} {
		if got := s.WebLinkAt(pos); got.URL != "" {
			t.Fatalf("out-of-bounds link: %+v", got)
		}
	}
}

func TestWebURLValidation(t *testing.T) {
	for _, value := range []string{"http://localhost:8787", "https://example.com/a?b=c&d=e", "HTTP://127.0.0.1:8787", "http://[::1]:8080/"} {
		if !IsWebURL(value) {
			t.Errorf("rejected %q", value)
		}
	}
	for _, value := range []string{"https://", "http:///path", "javascript:alert(1)", "file:///tmp/test", "https://example.com/\nrun", "https://example.com/ a", "https://example.com:\x00"} {
		if IsWebURL(value) {
			t.Errorf("accepted %q", value)
		}
	}
	term := vt.NewEmulator(80, 2)
	defer term.Close()
	fmt.Fprint(term, "\x1b]8;;javascript:alert(1)\x1b\\https://example.com\x1b]8;;\x1b\\")
	if got := (&Session{Term: term}).WebLinkAt(Position{0, 0}); got.URL != "" {
		t.Fatalf("unsupported OSC 8 target became clickable: %+v", got)
	}
}
