package terminal

import (
	"fmt"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

func TestInputPositionDuringSplitRepaint(t *testing.T) {
	s := &Session{Term: vt.NewEmulator(180, 40)}
	defer s.Term.Close()
	s.bindCallbacks()
	// A visible prompt, followed by a partial repaint ending at a status label.
	fmt.Fprint(s.Term, "\x1b[38;18H\x1b[?25l\x1b[8;150Hstatus")
	if got := s.InputPosition(); got != uv.Pos(17, 37) {
		t.Fatalf("hidden repaint moved IME to %+v", got)
	}
	// The next PTY chunk completes the repaint and restores the insertion point.
	fmt.Fprint(s.Term, "\x1b[39;20H\x1b[?25h")
	if got := s.InputPosition(); got != uv.Pos(19, 38) {
		t.Fatalf("restored prompt = %+v", got)
	}
	fmt.Fprint(s.Term, "\x1b[?25l\x1b[2;2H")
	s.Term.Resize(80, 24)
	if got := s.InputPosition(); got != uv.Pos(19, 23) {
		t.Fatalf("resized hidden prompt = %+v", got)
	}
	// Alternate screen coordinates must never leak back into the shell.
	fmt.Fprint(s.Term, "\x1b[?1049h\x1b[4;5H\x1b[?25h")
	if got := s.InputPosition(); got != uv.Pos(4, 3) {
		t.Fatalf("alternate screen prompt = %+v", got)
	}
	fmt.Fprint(s.Term, "\x1b[?1049l\x1b[?25h\x1b[5;6H")
	if got := s.InputPosition(); got != uv.Pos(5, 4) {
		t.Fatalf("shell prompt = %+v", got)
	}
}
