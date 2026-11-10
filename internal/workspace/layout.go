// Package workspace models terminal tabs and recursive splits independently of rendering.
package workspace

import "github.com/diiyw/xiterm/internal/terminal"

type Rect struct{ X, Y, W, H float32 }

func (r Rect) Contains(x, y float32) bool { return x >= r.X && y >= r.Y && x < r.X+r.W && y < r.Y+r.H }

type Axis uint8

const (
	Columns Axis = iota
	Rows
)

type Pane struct {
	Session        *terminal.Session
	Bounds         Rect
	Scroll         int
	SelectionStart terminal.Position
	SelectionEnd   terminal.Position
	Selected       bool
}

type Node struct {
	Bounds        Rect
	Pane          *Pane
	First, Second *Node
	Ratio         float32
	Axis          Axis
}

func Leaf(p *Pane) *Node { return &Node{Pane: p} }
func (n *Node) Panes() []*Pane {
	return n.AppendPanes(nil)
}

// AppendPanes appends leaves in layout order, reusing dst's capacity.
func (n *Node) AppendPanes(dst []*Pane) []*Pane {
	if n == nil {
		return dst
	}
	if n.Pane != nil {
		return append(dst, n.Pane)
	}
	return n.Second.AppendPanes(n.First.AppendPanes(dst))
}

func (n *Node) PaneCount() int {
	if n == nil {
		return 0
	}
	if n.Pane != nil {
		return 1
	}
	return n.First.PaneCount() + n.Second.PaneCount()
}

func (n *Node) Contains(p *Pane) bool {
	if n == nil {
		return false
	}
	if n.Pane != nil {
		return n.Pane == p
	}
	return n.First.Contains(p) || n.Second.Contains(p)
}

func (n *Node) FirstPane() *Pane {
	if n == nil {
		return nil
	}
	if n.Pane != nil {
		return n.Pane
	}
	return n.First.FirstPane()
}

func (n *Node) PaneAt(x, y float32) *Pane {
	if n == nil || !n.Bounds.Contains(x, y) {
		return nil
	}
	if n.Pane != nil {
		return n.Pane
	}
	if p := n.First.PaneAt(x, y); p != nil {
		return p
	}
	return n.Second.PaneAt(x, y)
}
func (n *Node) Visit(fn func(*Pane)) {
	if n == nil {
		return
	}
	if n.Pane != nil {
		fn(n.Pane)
		return
	}
	n.First.Visit(fn)
	n.Second.Visit(fn)
}
func (n *Node) Split(target, newPane *Pane, axis Axis) bool {
	if n.Pane == target {
		n.First = Leaf(target)
		n.Second = Leaf(newPane)
		n.Pane = nil
		n.Axis = axis
		n.Ratio = .5
		return true
	}
	if n.Pane != nil {
		return false
	}
	return n.First.Split(target, newPane, axis) || n.Second.Split(target, newPane, axis)
}
func (n *Node) Remove(target *Pane) *Node {
	if n == nil {
		return nil
	}
	if n.Pane != nil {
		if n.Pane == target {
			return nil
		}
		return n
	}
	n.First = n.First.Remove(target)
	n.Second = n.Second.Remove(target)
	if n.First == nil {
		return n.Second
	}
	if n.Second == nil {
		return n.First
	}
	return n
}
func (n *Node) Layout(r Rect) {
	if n == nil {
		return
	}
	n.Bounds = r
	if n.Pane != nil {
		n.Pane.Bounds = r
		return
	}
	ratio := n.Ratio
	if ratio <= 0 || ratio >= 1 {
		ratio = .5
	}
	a, b := r, r
	if n.Axis == Columns {
		a.W = (r.W - 1) * ratio
		b.X = r.X + a.W + 1
		b.W = r.W - a.W - 1
	} else {
		a.H = (r.H - 1) * ratio
		b.Y = r.Y + a.H + 1
		b.H = r.H - a.H - 1
	}
	n.First.Layout(a)
	n.Second.Layout(b)
}

type Tab struct {
	Root   *Node
	Active *Pane
}

func (t *Tab) FocusNext(delta int) {
	count := t.Root.PaneCount()
	if count == 0 {
		return
	}
	index, active := 0, -1
	t.Root.Visit(func(p *Pane) {
		if p == t.Active {
			active = index
		}
		index++
	})
	if active < 0 {
		return
	}
	target := (active + delta%count + count) % count
	index = 0
	t.Root.Visit(func(p *Pane) {
		if index == target {
			t.Active = p
		}
		index++
	})
}

// DividerAt finds the deepest split handle beneath a pointer.
func (n *Node) DividerAt(x, y float32) *Node {
	if n == nil || n.Pane != nil || !n.Bounds.Contains(x, y) {
		return nil
	}
	if hit := n.First.DividerAt(x, y); hit != nil {
		return hit
	}
	if hit := n.Second.DividerAt(x, y); hit != nil {
		return hit
	}
	edge := n.First.Bounds.X + n.First.Bounds.W
	value := x
	if n.Axis == Rows {
		edge = n.First.Bounds.Y + n.First.Bounds.H
		value = y
	}
	if value >= edge-4 && value <= edge+5 {
		return n
	}
	return nil
}
func (n *Node) DragDivider(x, y float32) {
	extent, value := n.Bounds.W, x-n.Bounds.X
	if n.Axis == Rows {
		extent = n.Bounds.H
		value = y - n.Bounds.Y
	}
	if extent > 1 {
		n.Ratio = max(.15, min(.85, value/(extent-1)))
	}
}
