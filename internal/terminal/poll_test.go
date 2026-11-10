package terminal

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
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

func TestPollYieldsWithinLargeReadAndPreservesOutput(t *testing.T) {
	s := &Session{Term: vt.NewEmulator(120, 12), output: make(chan []byte, 2), finished: make(chan error, 1)}
	defer s.Term.Close()
	s.initScrollback(1000)
	reference := vt.NewEmulator(120, 12)
	defer reference.Close()
	reference.SetScrollbackSize(1000)
	// One read alone exceeds the per-poll byte bound; exercise styles, history,
	// Unicode and escapes split across both queue entries and internal writes.
	payload := strings.Repeat("\x1b[32m中文 e\u0301 👩‍💻\x1b[0m terminal output\r\n", 700)
	s.output <- []byte(payload + "\x1b[3")
	s.output <- []byte("1mFINAL\x1b[0m")
	close(s.output)
	s.finished <- nil
	_, _ = reference.Write([]byte(payload + "\x1b[31mFINAL\x1b[0m"))
	if !s.Poll() || len(s.pendingOutput) == 0 || !s.PendingOutput() {
		t.Fatal("large read did not yield with bytes still pending")
	}
	if s.Exited {
		t.Fatal("exit discarded pending output")
	}
	for polls := 0; s.PendingOutput(); polls++ {
		if polls > len(payload) {
			t.Fatal("poll made no progress")
		}
		s.Poll()
	}
	if !s.Exited {
		t.Fatal("exit was not delivered after draining output")
	}
	compareHistory(t, s, reference)
}

func TestPollRespectsSharedDeadline(t *testing.T) {
	s := &Session{Term: vt.NewEmulator(80, 24), output: make(chan []byte, 1)}
	defer s.Term.Close()
	s.output <- []byte("queued text")
	if s.PollUntil(time.Now().Add(-time.Second)) || !s.PendingOutput() {
		t.Fatal("expired shared budget consumed output")
	}
	if !s.Poll() || s.PendingOutput() {
		t.Fatal("output was not resumed after yielding")
	}
}

func TestPollYieldsAfterExpensiveWrite(t *testing.T) {
	s := &Session{Term: vt.NewEmulator(80, 24), output: make(chan []byte, 1)}
	defer s.Term.Close()
	s.Term.RegisterCsiHandler('m', func(_ ansi.Params) bool {
		time.Sleep(pollBudget)
		return false
	})
	s.output <- []byte("\x1b[31m" + strings.Repeat("x", pollWriteSize*4))
	if !s.Poll() || s.Revision != 1 || !s.PendingOutput() {
		t.Fatal("expensive write did not return control before parsing the next batch")
	}
}

func TestOutputPrefixPreservesGraphemes(t *testing.T) {
	for _, cluster := range []string{"中", "e\u0301", "👩‍💻", "🇨🇳"} {
		for offset := 1; offset < len(cluster); offset++ {
			prefix := strings.Repeat("x", pollWriteSize-offset)
			b := []byte(prefix + cluster + "tail")
			if len(b) <= pollWriteSize {
				continue
			}
			if got := outputPrefix(b); got != len(prefix) {
				t.Fatalf("split %q at offset %d: prefix=%d, want %d", cluster, offset, got, len(prefix))
			}
		}
	}
}
