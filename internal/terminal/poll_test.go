package terminal

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/vt"
)

func TestPendingOutputAcrossBoundedPolls(t *testing.T) {
	s := &Session{Term: vt.NewEmulator(80, 24), output: make(chan []byte, 17), finished: make(chan error, 1)}
	defer s.Term.Close()
	for range cap(s.output) {
		s.output <- []byte("x")
	}
	close(s.output)
	for range 2 {
		if !s.PendingOutput() || !s.Poll() {
			t.Fatal("pending PTY bytes were not applied")
		}
	}
	if !s.PendingOutput() {
		t.Fatal("last chunk could be stranded after coalesced wake notifications")
	}
	s.Poll()
	if s.PendingOutput() || !strings.Contains(s.Term.String(), strings.Repeat("x", 17)) {
		t.Fatal("PTY output was not fully drained")
	}
	s.finished <- nil
	if !s.PendingOutput() {
		t.Fatal("process exit did not prevent indefinite idle")
	}
	s.Poll()
	if !s.Exited || s.PendingOutput() || s.Poll() {
		t.Fatal("closed output queue kept the event loop busy")
	}
}
