package app

import (
	"runtime"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/diiyw/xiterm/internal/config"
	"github.com/diiyw/xiterm/internal/terminal"
	"github.com/diiyw/xiterm/internal/workspace"
	"github.com/onix-engine/onix/window"
)

func TestIdleScheduling(t *testing.T) {
	term := vt.NewEmulator(80, 24)
	defer term.Close()
	pane := &workspace.Pane{Session: &terminal.Session{Term: term}}
	a := &App{win: &window.Context{}, tabs: []*workspace.Tab{{Root: workspace.Leaf(pane), Active: pane}}}
	now := time.Now()
	a.advanceTimers(now)
	if !a.dirty || a.blinkAt != now.Add(550*time.Millisecond) {
		t.Fatal("visible cursor did not schedule its next blink")
	}
	a.dirty = false
	if deadline, idle := a.idleDeadline(); !idle || deadline != a.blinkAt {
		t.Fatal("idle loop did not wait for the cursor blink")
	}
	for _, state := range []string{"hidden cursor", "search", "menu", "overlay", "scrollback", "exited"} {
		t.Run(state, func(t *testing.T) {
			switch state {
			case "hidden cursor":
				_, _ = term.Write([]byte("\x1b[?25l"))
				defer term.Write([]byte("\x1b[?25h"))
			case "search":
				a.search = true
				defer func() { a.search = false }()
			case "menu":
				a.menu = true
				defer func() { a.menu = false }()
			case "overlay":
				a.overlay = "settings"
				defer func() { a.overlay = "" }()
			case "scrollback":
				pane.Scroll = 1
				defer func() { pane.Scroll = 0 }()
			case "exited":
				pane.Session.Exited = true
				defer func() { pane.Session.Exited = false }()
			}
			a.dirty = false
			a.advanceTimers(now.Add(time.Second))
			if a.dirty {
				t.Fatal("invisible cursor triggered a redraw")
			}
			if deadline, idle := a.idleDeadline(); !idle || !deadline.IsZero() {
				t.Fatal("invisible cursor did not allow indefinite idle")
			}
		})
	}
	a.overlay = "settings"
	a.noticeUntil = now.Add(time.Second)
	a.smokeDeadline = now.Add(2 * time.Second)
	if deadline, idle := a.idleDeadline(); !idle || deadline != a.noticeUntil {
		t.Fatal("notice expiry was not scheduled while the cursor was invisible")
	}
	a.advanceTimers(now.Add(time.Second))
	if !a.noticeUntil.IsZero() || !a.dirty {
		t.Fatal("notice expiry did not request a redraw")
	}
	a.dirty = false
	if deadline, idle := a.idleDeadline(); !idle || deadline != a.smokeDeadline {
		t.Fatal("smoke deadline was not scheduled while the cursor was invisible")
	}
	a.win.RequestFrame()
	if _, idle := a.idleDeadline(); idle {
		t.Fatal("explicit frame request was put to sleep")
	}
}

func TestPendingOutputPreventsIdle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires /bin/sh")
	}
	s, err := terminal.Start(config.Profile{Name: "Idle", Shell: "/bin/sh", Args: []string{"-c", "printf ready; read value"}, Directory: t.TempDir()}, 80, 24, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	pane := &workspace.Pane{Session: s}
	a := &App{win: &window.Context{}, overlay: "settings", tabs: []*workspace.Tab{{Root: workspace.Leaf(pane), Active: pane}}}
	deadline := time.Now().Add(3 * time.Second)
	for !s.PendingOutput() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !s.PendingOutput() {
		t.Fatal("shell produced no output")
	}
	if _, idle := a.idleDeadline(); idle {
		t.Fatal("queued output was put to sleep without a wake notification")
	}
}
