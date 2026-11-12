package app

import (
	"slices"

	"github.com/diiyw/xiterm/internal/workspace"
)

const tabStart float32 = 48
const tabDragThreshold float32 = 5

type tabDragState struct {
	tab             *workspace.Tab
	startX, offsetX float32
	moving          bool
}

func (a *App) tabWidth() float32 {
	return min(float32(240), max(float32(40), (a.width-tabStart-44)/float32(max(1, len(a.tabs)))))
}

func (a *App) tabRect(index int) workspace.Rect {
	w := a.tabWidth()
	return workspace.Rect{X: tabStart + float32(index)*w, W: w, H: tabsH}
}

func (a *App) draggedTabRect() workspace.Rect {
	r := a.tabRect(0)
	r.X = max(tabStart, min(tabStart+float32(len(a.tabs)-1)*r.W, a.mouseX-a.tabDrag.offsetX))
	return r
}

func (a *App) beginTabDrag(index int, x float32) {
	if index < 0 || index >= len(a.tabs) {
		return
	}
	a.mouseX = x
	a.tabDrag = tabDragState{tab: a.tabs[index], startX: x, offsetX: x - a.tabRect(index).X}
}

// Keep the grabbed point under the pointer, and move the tab once its center
// crosses into another slot. Tracking the tab itself avoids stale slice indices.
func (a *App) moveTabDrag(x float32) {
	if a.tabDrag.tab == nil {
		return
	}
	a.mouseX = x
	if !a.tabDrag.moving && abs(x-a.tabDrag.startX) < tabDragThreshold {
		return
	}
	a.tabDrag.moving = true
	from := slices.Index(a.tabs, a.tabDrag.tab)
	if from < 0 {
		a.endTabDrag()
		return
	}
	r := a.draggedTabRect()
	to := min(len(a.tabs)-1, int((r.X+r.W/2-tabStart)/r.W))
	if from != to {
		active := a.tab()
		if from < to {
			copy(a.tabs[from:to], a.tabs[from+1:to+1])
		} else {
			copy(a.tabs[to+1:from+1], a.tabs[to:from])
		}
		a.tabs[to] = a.tabDrag.tab
		a.active = slices.Index(a.tabs, active)
	}
	a.dirty = true
}

func (a *App) endTabDrag() {
	if a.tabDrag.tab != nil {
		a.tabDrag = tabDragState{}
		a.dirty = true
	}
}
