package app

import (
	"slices"
	"testing"

	"github.com/charmbracelet/x/vt"
	"github.com/diiyw/xiterm/internal/terminal"
	"github.com/diiyw/xiterm/internal/workspace"
	"github.com/onix-engine/onix/input"
)

func tabDragApp(t *testing.T, count int, width float32) *App {
	t.Helper()
	a := &App{width: width, keys: map[input.KeyCode]bool{}}
	for range count {
		term := vt.NewEmulator(80, 24)
		t.Cleanup(func() { _ = term.Close() })
		pane := &workspace.Pane{Session: &terminal.Session{Term: term}}
		a.tabs = append(a.tabs, &workspace.Tab{Root: workspace.Leaf(pane), Active: pane})
	}
	for i := range a.tabs {
		r := a.tabRect(i)
		a.buttons = append(a.buttons, button{rect: r, action: actionSwitchTab, value: i})
		r.X, r.W = r.X+r.W-32, 28
		a.buttons = append(a.buttons, button{rect: r, action: actionCloseTab, value: i})
	}
	return a
}

func TestTabDragReordersAndKeepsActiveSession(t *testing.T) {
	for _, width := range []float32{1000, 380} {
		for _, from := range []int{0, 1, 3} {
			for _, to := range []int{0, 1, 3} {
				a := tabDragApp(t, 4, width)
				original := slices.Clone(a.tabs)
				r := a.tabRect(from)
				start := r.X + 12
				a.mouseDown(start, 20, input.MouseLeft)
				a.mouseMove(start+float32(to-from)*r.W, 120)
				a.mouseUp(a.mouseX, 120, input.MouseLeft)
				want := slices.Delete(slices.Clone(original), from, from+1)
				want = slices.Insert(want, to, original[from])
				if !slices.Equal(a.tabs, want) || a.active != to || a.pane() != original[from].Active {
					t.Fatalf("width=%v, %d→%d: wrong order or active session", width, from, to)
				}
				if a.tabDrag.tab != nil || a.dragPane != nil || a.mousePane != nil {
					t.Fatal("tab release retained a drag or started terminal selection")
				}
				// Number shortcuts must use the new visual order.
				a.keys[input.KeyLeftAlt] = true
				a.keydown(input.KeyNum1)
				if a.tab() != want[0] {
					t.Fatal("number shortcut did not use reordered tabs")
				}
			}
		}
	}
}

func TestTabDragThresholdOffsetAndBounds(t *testing.T) {
	a := tabDragApp(t, 3, 1000)
	original := slices.Clone(a.tabs)
	r := a.tabRect(1)
	start := r.X + r.W - 40 // Grab near the right edge, outside the close button.
	a.mouseDown(start, 20, input.MouseLeft)
	a.mouseMove(start+4, 20)
	if a.tabDrag.moving || !slices.Equal(a.tabs, original) {
		t.Fatal("small pointer jitter started a drag")
	}
	a.mouseMove(start+10, 20)
	if !a.tabDrag.moving || a.draggedTabRect().X != r.X+10 || !slices.Equal(a.tabs, original) {
		t.Fatal("tab jumped away from the grabbed point")
	}
	a.mouseMove(-1000, 20)
	if a.active != 0 || a.draggedTabRect().X != tabStart {
		t.Fatal("left edge did not clamp to first slot")
	}
	a.mouseMove(10000, 20)
	if a.active != 2 || a.draggedTabRect().X != a.tabRect(2).X || a.tab() != original[1] {
		t.Fatal("right edge did not clamp to last slot")
	}
	a.mouseUp(10000, 20, input.MouseLeft)
	order := slices.Clone(a.tabs)
	a.mouseMove(start, 20)
	if !slices.Equal(a.tabs, order) {
		t.Fatal("pointer movement after release reordered tabs")
	}
}

func TestTabDragReleaseUsesFinalPosition(t *testing.T) {
	a := tabDragApp(t, 3, 1000)
	first := a.tabs[0]
	a.mouseDown(tabStart+12, 20, input.MouseLeft)
	a.mouseUp(a.tabRect(2).X+12, 20, input.MouseLeft)
	if a.tabs[2] != first || a.tabDrag.tab != nil {
		t.Fatal("release did not apply the final pointer position")
	}
}

func TestTabDragCancellation(t *testing.T) {
	for _, cancel := range []string{"escape", "switch", "context menu"} {
		t.Run(cancel, func(t *testing.T) {
			a := tabDragApp(t, 3, 1000)
			a.mouseDown(tabStart+12, 20, input.MouseLeft)
			a.mouseMove(a.tabRect(1).X+12, 20)
			order := slices.Clone(a.tabs)
			switch cancel {
			case "escape":
				a.keydown(input.KeyEscape)
			case "switch":
				a.switchTab(2)
			case "context menu":
				a.mouseDown(300, 20, input.MouseRight)
			}
			if a.tabDrag.tab != nil {
				t.Fatal("drag was not cancelled")
			}
			a.mouseMove(10000, 20)
			if !slices.Equal(a.tabs, order) {
				t.Fatal("cancelled drag changed the order")
			}
		})
	}
}

func TestSingleTabDrag(t *testing.T) {
	a := tabDragApp(t, 1, 1000)
	first := a.tab()
	a.mouseDown(tabStart+12, 20, input.MouseLeft)
	a.mouseMove(10000, 20)
	a.mouseUp(-1000, 20, input.MouseLeft)
	if a.tab() != first || a.active != 0 || a.tabDrag.tab != nil {
		t.Fatal("dragging a single tab corrupted its state")
	}
}
