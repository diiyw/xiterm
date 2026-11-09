package app

import (
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	"github.com/diiyw/xiterm/internal/terminal"
	"github.com/diiyw/xiterm/internal/workspace"
	"github.com/onix-engine/onix/input"
)

func TestLocalDirectoryURL(t *testing.T) {
	dir := t.TempDir()
	urlPath := filepath.ToSlash(dir)
	if runtime.GOOS == "windows" {
		urlPath = "/" + urlPath
	}
	if got := directoryFromURL((&url.URL{Scheme: "file", Path: urlPath}).String()); got != dir {
		t.Fatalf("directory=%q", got)
	}
	if got := directoryFromURL("file://remote.invalid" + dir); got != "" {
		t.Fatal("accepted remote host as local directory")
	}
	if localDirectory("relative") != "" {
		t.Fatal("accepted relative directory")
	}
}

func TestNativeModifierSnapshotClearsLostKeyReleases(t *testing.T) {
	a := &App{keys: map[input.KeyCode]bool{
		input.KeyRightSuper: true, input.KeyLeftControl: true, input.KeyRightShift: true,
	}}
	a.syncModifiers(0)
	if a.shortcut() || a.ctrl() || a.super() || a.shift() || a.mods() != 0 {
		t.Fatal("stale modifiers survived an unmodified native key event")
	}
	// These keys must not dispatch application actions after returning to the
	// window. An accidental action would try to create a native tab or split.
	for _, key := range []input.KeyCode{input.KeyC, input.KeyD, input.KeyT} {
		a.keydown(key)
	}
	a.syncModifiers(uv.ModCtrl | uv.ModShift)
	if !a.ctrl() || !a.shift() || a.super() {
		t.Fatal("current shortcut modifiers were not restored")
	}
}

func TestArrowKeysReachShellInBothCursorModes(t *testing.T) {
	for _, application := range []bool{false, true} {
		for _, key := range []input.KeyCode{input.KeyUp, input.KeyDown} {
			t.Run(fmt.Sprintf("application=%v/key=%v", application, key), func(t *testing.T) {
				term := vt.NewEmulator(80, 24)
				defer term.Close()
				if application {
					fmt.Fprint(term, "\x1b[?1h")
				}
				pane := &workspace.Pane{Session: &terminal.Session{Term: term}, Scroll: 4, Selected: true}
				a := &App{keys: map[input.KeyCode]bool{}, tabs: []*workspace.Tab{{Root: workspace.Leaf(pane), Active: pane}}}
				got := make(chan string, 1)
				go func() {
					data := make([]byte, 3)
					_, _ = io.ReadFull(term, data)
					got <- string(data)
				}()
				a.keydown(key)
				want := "\x1b["
				if application {
					want = "\x1bO"
				}
				if key == input.KeyUp {
					want += "A"
				} else {
					want += "B"
				}
				select {
				case output := <-got:
					if output != want {
						t.Fatalf("shell received %q, want %q", output, want)
					}
				case <-time.After(time.Second):
					t.Fatal("arrow key did not reach shell input")
				}
				if pane.Scroll != 0 || pane.Selected {
					t.Fatal("history navigation did not return to the live prompt")
				}
			})
		}
	}
}
func TestSpecialKeys(t *testing.T) {
	for key, want := range map[input.KeyCode]rune{input.KeyUp: uv.KeyUp, input.KeyF5: uv.KeyF5, input.KeyDelete: uv.KeyDelete} {
		got, ok := specialKey(key)
		if !ok || got != want {
			t.Fatalf("key=%v got=%v want=%v", key, got, want)
		}
	}
}

func TestModifiedNavigation(t *testing.T) {
	if got := modifiedSequence(input.KeyLeft, uv.ModCtrl); got != "\x1b[1;5D" {
		t.Fatalf("Ctrl+Left=%q", got)
	}
	if got := modifiedSequence(input.KeyDelete, uv.ModShift|uv.ModCtrl); got != "\x1b[3;6~" {
		t.Fatalf("Ctrl+Shift+Delete=%q", got)
	}
}

func TestMenuEnterWorksWithoutTextInput(t *testing.T) {
	a := &App{menu: true, menuIndex: 7}
	if a.textInputConfig().Active {
		t.Fatal("menu must not activate the IME")
	}
	a.keydown(input.KeyEnter)
	if a.menu || a.overlay != "settings" {
		t.Fatalf("Enter did not activate Settings: menu=%v overlay=%q", a.menu, a.overlay)
	}
}

func TestHistoryNavigation(t *testing.T) {
	for _, height := range []int{1, 5} {
		term := vt.NewEmulator(40, height)
		term.SetScrollbackSize(100)
		defer term.Close()
		for i := range 30 {
			fmt.Fprintf(term, "line %d\r\n", i)
		}
		pane := &workspace.Pane{Session: &terminal.Session{Term: term}, Selected: true}
		a := &App{keys: map[input.KeyCode]bool{}, tabs: []*workspace.Tab{{Root: workspace.Leaf(pane), Active: pane}}}
		page := max(1, height-1)
		a.keydown(input.KeyPageUp)
		if pane.Scroll != page || pane.Selected {
			t.Fatalf("height=%d: PgUp scroll=%d selection=%v", height, pane.Scroll, pane.Selected)
		}
		a.keydown(input.KeyPageDown)
		if pane.Scroll != 0 {
			t.Fatal("PgDn did not return to the live screen")
		}
		a.keys[input.KeyLeftShift] = true
		a.keydown(input.KeyHome)
		if pane.Scroll != pane.Session.HistoryLen() {
			t.Fatal("Shift+Home did not reach the oldest history")
		}
		a.keydown(input.KeyPageUp)
		if pane.Scroll != pane.Session.HistoryLen() {
			t.Fatal("PgUp scrolled beyond history")
		}
		a.keydown(input.KeyEnd)
		a.keydown(input.KeyPageDown)
		if pane.Scroll != 0 {
			t.Fatal("Shift+End/PgDn scrolled below the live screen")
		}
		a.keydown(input.KeyPageUp)
		if pane.Scroll != page {
			t.Fatal("Shift+PgUp did not scroll a page")
		}
		a.keydown(input.KeyPageDown)
		if pane.Scroll != 0 {
			t.Fatal("Shift+PgDn did not scroll back")
		}
	}
}

func TestNavigationReachesApplications(t *testing.T) {
	for _, altScreen := range []bool{false, true} {
		for _, tc := range []struct {
			name string
			key  input.KeyCode
			mod  uv.KeyMod
			want string
		}{
			{"Home", input.KeyHome, 0, "\x1b[H"},
			{"End", input.KeyEnd, 0, "\x1b[F"},
			{"PgUp", input.KeyPageUp, 0, "\x1b[5~"},
			{"PgDn", input.KeyPageDown, 0, "\x1b[6~"},
			{"ShiftHome", input.KeyHome, uv.ModShift, "\x1b[1;2H"},
			{"ShiftEnd", input.KeyEnd, uv.ModShift, "\x1b[1;2F"},
			{"ShiftPgUp", input.KeyPageUp, uv.ModShift, "\x1b[5;2~"},
			{"ShiftPgDn", input.KeyPageDown, uv.ModShift, "\x1b[6;2~"},
			{"CtrlHome", input.KeyHome, uv.ModCtrl, "\x1b[1;5H"},
			{"CtrlEnd", input.KeyEnd, uv.ModCtrl, "\x1b[1;5F"},
			{"CtrlPgUp", input.KeyPageUp, uv.ModCtrl, "\x1b[5;5~"},
			{"AltPgDn", input.KeyPageDown, uv.ModAlt, "\x1b[6;3~"},
		} {
			if !altScreen && (tc.mod == uv.ModShift || tc.mod == 0 && (tc.key == input.KeyPageUp || tc.key == input.KeyPageDown)) {
				continue // These combinations navigate the main screen's scrollback.
			}
			t.Run(fmt.Sprintf("alt=%v/%s", altScreen, tc.name), func(t *testing.T) {
				term := vt.NewEmulator(40, 5)
				defer term.Close()
				if altScreen {
					fmt.Fprint(term, "\x1b[?1049h")
				}
				pane := &workspace.Pane{Session: &terminal.Session{Term: term}, Scroll: 4, Selected: true}
				a := &App{keys: map[input.KeyCode]bool{}, tabs: []*workspace.Tab{{Root: workspace.Leaf(pane), Active: pane}}}
				a.syncModifiers(tc.mod)
				got := make(chan string, 1)
				go func() {
					data := make([]byte, len(tc.want))
					_, _ = io.ReadFull(term, data)
					got <- string(data)
				}()
				a.keydown(tc.key)
				select {
				case output := <-got:
					if output != tc.want {
						t.Fatalf("application received %q, want %q", output, tc.want)
					}
				case <-time.After(time.Second):
					t.Fatal("navigation key did not reach application")
				}
				if pane.Scroll != 0 || pane.Selected {
					t.Fatal("application input did not clear scroll/selection")
				}
			})
		}
	}
}
