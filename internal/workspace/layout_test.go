package workspace

import (
	"slices"
	"testing"
)

func TestPaneQueriesReuseAndDividerBoundaries(t *testing.T) {
	a, b, c, outside := &Pane{}, &Pane{}, &Pane{}, &Pane{}
	root := Leaf(a)
	root.Split(a, b, Columns)
	root.Split(b, c, Rows)
	root.Layout(Rect{W: 801, H: 601})
	if root.PaneCount() != 3 || root.FirstPane() != a || !root.Contains(c) || root.Contains(outside) {
		t.Fatal("pane queries did not follow nested splits")
	}
	for _, p := range []*Pane{a, b, c} {
		if root.PaneAt(p.Bounds.X+10, p.Bounds.Y+10) != p {
			t.Fatal("pane hit test failed")
		}
	}
	if root.PaneAt(400, 100) != nil || root.PaneAt(500, 300) != nil || root.PaneAt(-1, 100) != nil {
		t.Fatal("divider or out-of-window hit resolved to a pane")
	}
	dst := make([]*Pane, 1, 4)
	dst[0] = outside
	if got := root.AppendPanes(dst); !slices.Equal(got, []*Pane{outside, a, b, c}) {
		t.Fatalf("appended panes = %v", got)
	}
	tab := &Tab{Root: root, Active: a}
	tab.FocusNext(-4)
	if tab.Active != c {
		t.Fatal("large backwards focus did not wrap")
	}
	if allocs := testing.AllocsPerRun(100, func() {
		_ = root.AppendPanes(dst[:0])
		_ = root.PaneCount()
		_ = root.Contains(c)
		_ = root.PaneAt(100, 100)
		tab.FocusNext(1)
	}); allocs != 0 {
		t.Fatalf("warmed pane queries allocated %.0f times", allocs)
	}
	var empty *Node
	if empty.PaneCount() != 0 || empty.FirstPane() != nil || empty.Contains(a) || empty.PaneAt(0, 0) != nil {
		t.Fatal("empty tree queries returned a pane")
	}
}

func TestNestedSplitLayoutAndRemoval(t *testing.T) {
	a, b, c := &Pane{}, &Pane{}, &Pane{}
	root := Leaf(a)
	if !root.Split(a, b, Columns) || !root.Split(b, c, Rows) {
		t.Fatal("split failed")
	}
	root.Layout(Rect{X: 10, Y: 20, W: 801, H: 601})
	if a.Bounds.W != 400 || b.Bounds.X != 411 || b.Bounds.H != 300 || c.Bounds.Y != 321 {
		t.Fatalf("bad geometry: %+v %+v %+v", a.Bounds, b.Bounds, c.Bounds)
	}
	root = root.Remove(b)
	root.Layout(Rect{X: 10, Y: 20, W: 801, H: 601})
	if len(root.Panes()) != 2 || c.Bounds.H != 601 {
		t.Fatal("removal must collapse the surviving split")
	}
	root = root.Remove(a)
	if root.Pane != c {
		t.Fatal("root did not collapse")
	}
	if root.Remove(c) != nil {
		t.Fatal("last pane must remove root")
	}
}
func TestFocusWraps(t *testing.T) {
	a, b := &Pane{}, &Pane{}
	root := Leaf(a)
	root.Split(a, b, Columns)
	tab := Tab{Root: root, Active: a}
	tab.FocusNext(-1)
	if tab.Active != b {
		t.Fatal("previous did not wrap")
	}
	tab.FocusNext(1)
	if tab.Active != a {
		t.Fatal("next did not wrap")
	}
}

func TestDividerDragAndHitArea(t *testing.T) {
	a, b := &Pane{}, &Pane{}
	root := Leaf(a)
	root.Split(a, b, Columns)
	root.Layout(Rect{W: 1001, H: 600})
	if root.DividerAt(500, 300) != root || root.DividerAt(100, 300) != nil {
		t.Fatal("incorrect divider hit test")
	}
	root.DragDivider(700, 300)
	root.Layout(root.Bounds)
	if a.Bounds.W != 700 || b.Bounds.X != 701 {
		t.Fatalf("drag failed: %v %v", a.Bounds, b.Bounds)
	}
	root.DragDivider(0, 300)
	if root.Ratio != .15 {
		t.Fatal("divider must keep both panes visible")
	}
}
