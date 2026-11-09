package app

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/x/vt"
	"github.com/diiyw/xiterm/assets"
	"github.com/diiyw/xiterm/internal/terminal"
	"github.com/diiyw/xiterm/internal/workspace"
	"github.com/onix-engine/onix/input"
	"github.com/onix-engine/onix/text"
)

func TestTextInputCaretFollowsTerminal(t *testing.T) {
	term := vt.NewEmulator(80, 40)
	defer term.Close()
	pane := &workspace.Pane{Session: &terminal.Session{Term: term}, Bounds: workspace.Rect{Y: tabsH}}
	a := &App{paint: painter{cellW: 9, lineH: 22}, tabs: []*workspace.Tab{{Root: workspace.Leaf(pane), Active: pane}}}
	initial := a.textInputConfig()
	if !initial.Active || initial.Caret != (input.TextInputRect{X: 12, Y: 50, Width: 2, Height: 22}) {
		t.Fatalf("initial config = %+v", initial)
	}
	// A TUI can put its prompt near the bottom of the screen.
	fmt.Fprint(term, "\x1b[38;4H")
	a.blink = true
	moved := a.textInputConfig()
	if moved.Caret != (input.TextInputRect{X: 39, Y: 864, Width: 2, Height: 22}) {
		t.Fatalf("bottom prompt caret = %+v", moved.Caret)
	}
	if moved.Revision != initial.Revision {
		t.Fatal("moving the caret must preserve the composition session")
	}
	// Pane placement and font metrics can change after a resize or zoom.
	pane.Bounds = workspace.Rect{X: 400, Y: 100}
	a.paint.cellW, a.paint.lineH = 12, 30
	if got := a.textInputConfig().Caret; got != (input.TextInputRect{X: 448, Y: 1218, Width: 2, Height: 30}) {
		t.Fatalf("resized caret = %+v", got)
	}
}

func TestTextInputCaretFollowsFocus(t *testing.T) {
	term := vt.NewEmulator(80, 24)
	defer term.Close()
	left := &workspace.Pane{Session: &terminal.Session{Term: term}, Bounds: workspace.Rect{Y: tabsH}}
	right := &workspace.Pane{Session: &terminal.Session{Term: term}, Bounds: workspace.Rect{X: 500, Y: tabsH}}
	root := workspace.Leaf(left)
	root.Split(left, right, workspace.Columns)
	tab := &workspace.Tab{Root: root, Active: left}
	a := &App{paint: painter{cellW: 9, lineH: 22}, tabs: []*workspace.Tab{tab}}
	if got := a.textInputConfig().Caret; got.X != 12 || got.Y != 74 {
		t.Fatalf("left pane caret = %+v", got)
	}
	tab.Active = right
	if got := a.textInputConfig().Caret; got.X != 512 || got.Y != 74 {
		t.Fatalf("right pane caret = %+v", got)
	}
	other := &workspace.Pane{Session: &terminal.Session{Term: term}, Bounds: workspace.Rect{Y: tabsH}}
	a.tabs = append(a.tabs, &workspace.Tab{Root: workspace.Leaf(other), Active: other})
	a.active = 1
	if got := a.textInputConfig().Caret; got.X != 12 || got.Y != 50 {
		t.Fatalf("switched tab caret = %+v", got)
	}
	for _, state := range []string{"menu", "overlay", "exited", "no panes"} {
		t.Run(state, func(t *testing.T) {
			copy := *a
			switch state {
			case "menu":
				copy.menu = true
			case "overlay":
				copy.overlay = "settings"
			case "exited":
				other.Session.Exited = true
				defer func() { other.Session.Exited = false }()
			case "no panes":
				copy.tabs = nil
			}
			if copy.textInputConfig().Active {
				t.Fatal("IME remained active without a text input target")
			}
		})
	}
}

func TestTextInputCaretFollowsSearch(t *testing.T) {
	font, err := text.LoadFont(assets.Regular)
	if err != nil {
		t.Fatal(err)
	}
	defer font.Dispose()
	a := &App{search: true, width: 800, paint: painter{regular: font.GlyphCache(15, 72), lineH: 22}}
	config := a.textInputConfig()
	if !config.Active || config.Caret != (input.TextInputRect{X: 70, Y: 52, Width: 2, Height: 22}) {
		t.Fatalf("search caret = %+v", config)
	}
	a.query = "find"
	if got := a.textInputConfig().Caret.X; got <= 70 || got >= 530 {
		t.Fatalf("caret did not follow query text: %v", got)
	}
	// A long visible query is clipped at the search field's right edge.
	a.query = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if got := a.textInputConfig().Caret.X; got != 530 {
		t.Fatalf("long query caret = %v, want 530", got)
	}
}

func TestCompositionAnchorSurvivesOutputAndResize(t *testing.T) {
	term := vt.NewEmulator(80, 40)
	defer term.Close()
	pane := &workspace.Pane{Session: &terminal.Session{Term: term}, Bounds: workspace.Rect{Y: tabsH}}
	a := &App{paint: painter{cellW: 9, lineH: 22}, tabs: []*workspace.Tab{{Root: workspace.Leaf(pane), Active: pane}}}
	fmt.Fprint(term, "\x1b[38;18H")
	before := a.textInputConfig()
	a.textComposition(input.TextComposition{Text: "shi", Active: true})
	fmt.Fprint(term, "\x1b[8;70Hbackground output")
	if got := a.textInputConfig(); got.Caret != before.Caret || got.Revision != before.Revision {
		t.Fatalf("background output moved composition: %+v", got)
	}
	// Geometry still follows zoom/pane movement while the cell stays anchored.
	pane.Bounds.X = 300
	a.paint.cellW = 12
	if got := a.textInputConfig(); got.Caret.X != 516 || got.Revision != before.Revision {
		t.Fatalf("composition did not follow geometry: %+v", got)
	}
	a.textComposition(input.TextComposition{})
	if got := a.textInputConfig(); got.Caret == before.Caret || got.Revision != before.Revision {
		t.Fatalf("composition did not release anchor: %+v", got)
	}
}

func TestChangingInputTargetRestartsComposition(t *testing.T) {
	term := vt.NewEmulator(80, 24)
	defer term.Close()
	left := &workspace.Pane{Session: &terminal.Session{Term: term}}
	right := &workspace.Pane{Session: &terminal.Session{Term: term}, Bounds: workspace.Rect{X: 500}}
	root := workspace.Leaf(left)
	root.Split(left, right, workspace.Columns)
	tab := &workspace.Tab{Root: root, Active: left}
	a := &App{paint: painter{cellW: 9, lineH: 22}, tabs: []*workspace.Tab{tab}}
	first := a.textInputConfig()
	a.textComposition(input.TextComposition{Active: true})
	tab.Active = right
	next := a.textInputConfig()
	if next.Revision == first.Revision || a.textInput.composing || next.Caret.X != 512 {
		t.Fatalf("pane switch kept old composition: %+v", next)
	}
	a.menu = true
	disabled := a.textInputConfig()
	if disabled.Active || disabled.Revision == next.Revision {
		t.Fatalf("menu did not end text input: %+v", disabled)
	}
	a.menu = false
	if resumed := a.textInputConfig(); !resumed.Active || resumed.Revision == disabled.Revision {
		t.Fatalf("dismissed menu did not restart text input: %+v", resumed)
	}
}
