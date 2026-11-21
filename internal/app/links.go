package app

import (
	"github.com/diiyw/xiterm/internal/terminal"
	"github.com/diiyw/xiterm/internal/workspace"
	"github.com/onix-engine/onix/input"
)

type linkHit struct {
	pane *workspace.Pane
	link terminal.WebLink
}

type linkHoverKey struct {
	pane     *workspace.Pane
	position terminal.Position
	revision uint64
	width    int
}

type linkClick struct {
	hit  linkHit
	x, y float32
}

func (a *App) linkPosition(x, y float32) (*workspace.Pane, terminal.Position) {
	if a.menu || a.overlay != "" || a.search || a.paint.cellW <= 0 || a.paint.lineH <= 0 {
		return nil, terminal.Position{}
	}
	t := a.tab()
	if t == nil {
		return nil, terminal.Position{}
	}
	p := t.Root.PaneAt(x, y)
	if p == nil || p.Session.MouseTracking && !a.shift() && !a.browserModifier() {
		return nil, terminal.Position{}
	}
	x0, y0 := p.Bounds.X+12, p.Bounds.Y+a.paneHeaderHeight(p)+8
	if x < x0 || y < y0 || x >= x0+float32(p.Session.Term.Width())*a.paint.cellW || y >= y0+float32(p.Session.Term.Height())*a.paint.lineH {
		return nil, terminal.Position{}
	}
	return p, a.position(p, x, y)
}

func (a *App) browserModifier() bool {
	return a.ctrl() || a.super()
}

func (a *App) linkAt(x, y float32) linkHit {
	p, pos := a.linkPosition(x, y)
	if p == nil {
		return linkHit{}
	}
	return linkHit{pane: p, link: p.Session.WebLinkAt(pos)}
}

func (a *App) updateLinkHover() {
	key := linkHoverKey{}
	if a.dragPane == nil && a.dragSplit == nil && a.mousePane == nil && a.tabDrag.tab == nil {
		if p, pos := a.linkPosition(a.mouseX, a.mouseY); p != nil {
			key = linkHoverKey{pane: p, position: pos, revision: p.Session.Revision, width: p.Session.Term.Width()}
		}
	}
	if key != a.linkHoverKey {
		a.linkHoverKey = key
		a.hoverLink = linkHit{}
		if key.pane != nil {
			a.hoverLink = linkHit{pane: key.pane, link: key.pane.Session.WebLinkAt(key.position)}
		}
	}
	if a.win != nil {
		cursor := input.CursorDefault
		if a.hoverLink.link.URL != "" {
			cursor = input.CursorPointer
		}
		a.win.SetCursor(cursor)
	}
}

func (a *App) openWebLink(value string) {
	if !terminal.IsWebURL(value) {
		return
	}
	if a.browserErrors == nil {
		a.browserErrors = make(chan error, 1)
	}
	results := a.browserErrors
	open := a.browserOpen
	if open == nil {
		open = openBrowser
	}
	go func() {
		if err := open(value); err != nil {
			select {
			case results <- err:
			default:
			}
			if a.win != nil {
				a.win.RequestFrame()
			}
		}
	}()
}
