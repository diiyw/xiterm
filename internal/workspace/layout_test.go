package workspace

import "testing"

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
