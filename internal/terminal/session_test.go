//go:build linux || darwin

package terminal

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	"github.com/diiyw/xiterm/internal/config"
)

func waitFor(t *testing.T, s *Session, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.Poll()
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out; screen=%q exit=%v", s.Term.String(), s.ExitErr)
}
func startShell(t *testing.T, script string) *Session {
	t.Helper()
	s, err := Start(config.Profile{Name: "Test", Shell: "/bin/sh", Args: []string{"-c", script}, Directory: t.TempDir()}, 80, 12, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestRealPTYInputColorsResizeAndExit(t *testing.T) {
	s := startShell(t, `stty -echo; printf '\033[32mREADY\033[0m\n'; read value; printf 'INPUT=%s\n' "$value"; stty size; exit 7`)
	waitFor(t, s, func() bool { return strings.Contains(s.Term.String(), "READY") })
	green := s.Term.CellAt(0, 0)
	if green == nil || green.Style.Fg == nil {
		t.Fatal("ANSI color was not parsed")
	}
	if err := s.Resize(97, 31); err != nil {
		t.Fatal(err)
	}
	s.SendText("hello 世界")
	s.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	waitFor(t, s, func() bool { return s.Exited && strings.Contains(s.Term.String(), "31 97") })
	if !strings.Contains(s.Term.String(), "INPUT=hello 世界") {
		t.Fatalf("PTY input lost: %s", s.Term.String())
	}
	if s.ExitErr == nil {
		t.Fatal("expected nonzero exit status")
	}
}
func TestDeviceStatusReplyDoesNotDeadlock(t *testing.T) {
	s := startShell(t, `stty raw -echo; printf '\033[6n'; result=$(dd bs=1 count=6 2>/dev/null); stty sane; printf 'REPLY:%s\n' "$result"`)
	waitFor(t, s, func() bool { return s.Exited })
	if !strings.Contains(s.Term.String(), "REPLY:") {
		t.Fatal("terminal failed to answer cursor position query")
	}
}
func TestCloseKillsShellProcessGroup(t *testing.T) {
	s := startShell(t, `sleep 120 & wait`)
	pid := s.process.(*unixProcess).cmd.Process.Pid
	s.Close()
	s.Close()
	waitFor(t, s, func() bool { return s.Exited })
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatalf("shell still alive: %v", err)
	}
}
func TestSelectionSearchWideCellsAndAlternateScreen(t *testing.T) {
	s := &Session{Term: vt.NewEmulator(20, 3)}
	defer s.Term.Close()
	s.Term.SetScrollbackSize(100)
	fmt.Fprint(s.Term, "old\r\nhello 世界\r\nHELLO again\r\nend")
	if s.HistoryLen() != 1 {
		t.Fatalf("history=%d", s.HistoryLen())
	}
	matches := s.Find("hello")
	if len(matches) != 2 || matches[0].Row != 1 {
		t.Fatalf("matches=%+v", matches)
	}
	wide := s.Find("世界")
	if len(wide) != 1 || wide[0].Col != 6 || wide[0].Width != 4 {
		t.Fatalf("wide match=%+v", wide)
	}
	if got := s.Selection(Position{Row: 1, Col: 0}, Position{Row: 1, Col: 9}); got != "hello 世界" {
		t.Fatalf("selection=%q", got)
	}
	fmt.Fprint(s.Term, "\x1b[?1049hALT")
	if s.HistoryLen() != 0 || !strings.Contains(s.Line(0), "ALT") {
		t.Fatal("alternate screen mixed with primary history")
	}
	fmt.Fprint(s.Term, "\x1b[?1049l")
	if s.HistoryLen() != 1 {
		t.Fatal("primary history not restored")
	}
}
func TestEnvironmentOverrides(t *testing.T) {
	result := environment([]string{"PATH=/bin", "TERM=dumb", "FOO=old"}, []string{"FOO=new", "TERM=bad", "COLORTERM=no"})
	joined := strings.Join(result, "\n")
	if strings.Contains(joined, "old") || strings.Contains(joined, "dumb") || !strings.Contains(joined, "FOO=new") || !strings.Contains(joined, "TERM=xterm-256color") {
		t.Fatalf("env=%v", result)
	}
}
func TestStartFailureIsReported(t *testing.T) {
	_, err := Start(config.Profile{Name: "Bad", Shell: "/not/a/shell", Directory: os.TempDir()}, 80, 24, 100, nil)
	if err == nil {
		t.Fatal("invalid shell accepted")
	}
}

func TestPaletteMouseModesAndBracketedPaste(t *testing.T) {
	s := startShell(t, `stty raw -echo; printf '\033[32mREADY\033[?1000h\033[?1006h\033[?2004h'; dd bs=1 count=15 2>/dev/null | od -An -tx1; printf '\033[?1000l'`)
	waitFor(t, s, func() bool { return s.MouseTracking })
	r, g, b, _ := s.ResolveColor(s.Term.CellAt(0, 0).Style.Fg).RGBA()
	if r>>8 != 0x98 || g>>8 != 0xc3 || b>>8 != 0x79 {
		t.Fatalf("custom palette ignored: %x %x %x", r, g, b)
	}
	s.Paste("abc")
	waitFor(t, s, func() bool { return s.Exited })
	if s.MouseTracking {
		t.Fatal("mouse mode did not reset")
	}
	if !strings.Contains(s.Term.String(), "1b 5b 32 30 30 7e 61 62 63 1b 5b 32 30 31 7e") {
		t.Fatalf("bracketed paste encoding: %q", s.Term.String())
	}
}
