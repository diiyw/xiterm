package app

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/diiyw/xiterm/internal/terminal"
	"github.com/diiyw/xiterm/internal/workspace"
	"github.com/onix-engine/onix/input"
)

func browserTestApp(t *testing.T) *App {
	t.Helper()
	term := vt.NewEmulator(80, 8)
	t.Cleanup(func() { _ = term.Close() })
	fmt.Fprint(term, "工作清单：http://127.0.0.1:8787")
	p := &workspace.Pane{Session: &terminal.Session{Term: term}}
	root := workspace.Leaf(p)
	root.Layout(workspace.Rect{Y: tabsH, W: 664, H: 176})
	a := &App{tabs: []*workspace.Tab{{Root: root, Active: p}}, keys: map[input.KeyCode]bool{}}
	a.paint.cellW, a.paint.lineH = 8, 20
	return a
}

func TestBrowserClickAndHover(t *testing.T) {
	a := browserTestApp(t)
	opened := make(chan string, 1)
	a.browserOpen = func(value string) error { opened <- value; return nil }
	x, y := float32(12+10*8+2), tabsH+10
	a.mouseMove(x, y)
	if a.hoverLink.link.URL != "http://127.0.0.1:8787" {
		t.Fatalf("hover=%+v", a.hoverLink)
	}
	a.mouseDown(x, y, input.MouseLeft)
	if a.browserErrors != nil {
		t.Fatal("browser opened before mouse release")
	}
	a.mouseUp(x, y, input.MouseLeft)
	select {
	case got := <-opened:
		if got != "http://127.0.0.1:8787" {
			t.Fatalf("opened %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("click did not open browser")
	}
	if a.dragPane != nil || a.pane().Selected {
		t.Fatal("click retained a selection or drag")
	}
	// Output changing underneath a stationary pointer invalidates the hover.
	fmt.Fprint(a.pane().Session.Term, "\r\x1b[2Kgone")
	a.pane().Session.Revision++
	a.updateLinkHover()
	if a.hoverLink.link.URL != "" {
		t.Fatal("hover retained an overwritten link")
	}
}

func TestBrowserDragDoesNotOpen(t *testing.T) {
	for _, returnToStart := range []bool{false, true} {
		a := browserTestApp(t)
		x, y := float32(94), tabsH+10
		a.mouseDown(x, y, input.MouseLeft)
		a.mouseMove(x+40, y)
		if !a.pane().Selected {
			t.Fatal("drag did not select text")
		}
		if returnToStart {
			a.mouseMove(x, y)
		}
		a.mouseUp(a.mouseX, y, input.MouseLeft)
		if a.browserErrors != nil {
			t.Fatal("drag opened browser")
		}
	}
}

func TestBrowserHitExcludesChromeAndOverlays(t *testing.T) {
	a := browserTestApp(t)
	for _, point := range [][2]float32{{94, tabsH + 1}, {1, tabsH + 10}, {654, tabsH + 10}, {94, 20}} {
		if hit := a.linkAt(point[0], point[1]); hit.link.URL != "" {
			t.Fatalf("chrome hit=%+v", hit)
		}
	}
	for _, mode := range []string{"menu", "overlay", "search", "mouse"} {
		a.menu, a.search, a.overlay, a.pane().Session.MouseTracking = false, false, "", false
		switch mode {
		case "menu":
			a.menu = true
		case "overlay":
			a.overlay = "settings"
		case "search":
			a.search = true
		case "mouse":
			a.pane().Session.MouseTracking = true
		}
		if hit := a.linkAt(94, tabsH+10); hit.link.URL != "" {
			t.Fatalf("%s allowed link hit", mode)
		}
	}
	a.keys[input.KeyLeftControl] = true
	if hit := a.linkAt(94, tabsH+10); hit.link.URL == "" {
		t.Fatal("Ctrl did not allow link hit in mouse tracking mode")
	}
}

func TestBrowserFailureIsReportedOnUIThread(t *testing.T) {
	a := browserTestApp(t)
	a.browserOpen = func(string) error { return errors.New("browser unavailable") }
	a.openWebLink("https://example.com")
	select {
	case err := <-a.browserErrors:
		a.browserErrors <- err
	case <-time.After(time.Second):
		t.Fatal("browser failure was lost")
	}
	// Keep poll's title update from requiring a native window.
	a.lastTitle, a.lastSessionTitle = "XiTerm", a.pane().Session.Title
	a.poll()
	if a.notice != "Could not open browser: browser unavailable" {
		t.Fatalf("notice=%q", a.notice)
	}
}
