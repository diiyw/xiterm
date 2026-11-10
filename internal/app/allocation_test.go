package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/diiyw/xiterm/assets"
	"github.com/diiyw/xiterm/internal/config"
	"github.com/diiyw/xiterm/internal/terminal"
	"github.com/diiyw/xiterm/internal/workspace"
	"github.com/onix-engine/onix/graphics"
	"github.com/onix-engine/onix/input"
	"github.com/onix-engine/onix/text"
	"github.com/onix-engine/onix/window"
)

func idleApp(tb testing.TB) *App {
	tb.Helper()
	a := &App{win: &window.Context{}, width: 1000, height: 700, lastTitle: "XiTerm — stable"}
	for range 3 {
		term := vt.NewEmulator(80, 24)
		tb.Cleanup(func() { _ = term.Close() })
		pane := &workspace.Pane{Session: &terminal.Session{Term: term, Title: "stable"}}
		root := workspace.Leaf(pane)
		root.Split(pane, &workspace.Pane{Session: pane.Session}, workspace.Columns)
		a.tabs = append(a.tabs, &workspace.Tab{Root: root, Active: pane})
	}
	return a
}

func paintApp(tb testing.TB) *App {
	tb.Helper()
	a := idleApp(tb)
	font, err := text.LoadFont(assets.Regular)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = font.Dispose() })
	uiFont, err := loadSystemUIFont()
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = uiFont.Dispose() })
	a.font, a.boldFont, a.uiFont = font, font, uiFont
	a.setFontSize(15)
	tb.Cleanup(a.paint.disposeGlyphCaches)
	a.paint.canvas = graphics.NewCanvas2D(nil, int(a.width), int(a.height))
	a.paint.solid = graphics.CreateTextureByColor(graphics.White)
	tb.Cleanup(func() { _ = a.paint.solid.Dispose() })
	a.logo = a.paint.solid
	a.cfg.Profiles = []config.Profile{{Name: "Default 中文", Shell: "/bin/sh"}}
	for _, tab := range a.tabs {
		tab.Root.Layout(workspace.Rect{Y: tabsH, W: a.width, H: a.height - tabsH})
		s := tab.Active.Session
		s.Title = "long terminal title 中文 to exercise truncation on every frame"
		s.Profile = a.cfg.Profiles[0]
		fmt.Fprint(s.Term, "\x1b[31m中文 marker\x1b[0m\r\nmarker 中文\r\n")
	}
	return a
}

func TestWarmDrawDoesNotAllocate(t *testing.T) {
	a := paintApp(t)
	for _, mode := range []string{"terminal", "search", "menu", "settings", "profiles", "help", "notice"} {
		t.Run(mode, func(t *testing.T) {
			a.search, a.menu, a.overlay = false, false, ""
			switch mode {
			case "search":
				a.search, a.query = true, "中文"
				a.matches = a.pane().Session.FindInto(a.matches, a.query)
			case "menu":
				a.menu = true
			case "notice":
				a.notice, a.noticeUntil = "Notice 中文", time.Now().Add(time.Hour)
			case "settings", "profiles", "help":
				a.overlay = mode
			}
			draw := func() {
				a.draw()
				if err := a.paint.canvas.FinishFrame(); err != nil {
					t.Fatal(err)
				}
			}
			for range 3 {
				draw()
			}
			if allocs := testing.AllocsPerRun(20, draw); allocs != 0 {
				t.Fatalf("warmed %s drawing allocated %.0f times", mode, allocs)
			}
		})
	}
}

func BenchmarkWarmDraw(b *testing.B) {
	a := paintApp(b)
	a.search, a.query = true, "中文"
	a.matches = a.pane().Session.FindInto(nil, a.query)
	for range 3 {
		a.draw()
		_ = a.paint.canvas.FinishFrame()
	}
	b.ReportAllocs()
	for b.Loop() {
		a.draw()
		_ = a.paint.canvas.FinishFrame()
	}
}

func TestIdlePollAndMenuDoNotAllocate(t *testing.T) {
	a := idleApp(t)
	if allocs := testing.AllocsPerRun(100, a.poll); allocs != 0 {
		t.Fatalf("idle polling allocated %.0f times", allocs)
	}
	if allocs := testing.AllocsPerRun(100, func() {
		_ = a.menuItemAt(150, 150)
		_ = a.paneHeaderHeight(a.pane())
		_ = a.textInputConfig()
		_ = modifiedSequence(input.KeyLeft, 5)
	}); allocs != 0 {
		t.Fatalf("UI hit testing/caret/navigation allocated %.0f times", allocs)
	}
}

func TestButtonDispatchAndMenuHitTesting(t *testing.T) {
	a := idleApp(t)
	a.buttons = []button{{rect: workspace.Rect{X: 10, Y: 10, W: 30, H: 30}, action: actionSwitchTab, value: 2}}
	a.mouseDown(20, 20, input.MouseLeft)
	if a.active != 2 {
		t.Fatal("tab button lost its target index")
	}
	a.buttons[0].action = actionSettings
	a.mouseDown(20, 20, input.MouseLeft)
	if a.overlay != "settings" {
		t.Fatal("settings button did not open overlay")
	}
	a.buttons[0].action = actionCloseOverlay
	a.mouseDown(20, 20, input.MouseLeft)
	if a.overlay != "" {
		t.Fatal("close button did not dismiss overlay")
	}
	a.menuX, a.menuY = 50, 50
	r := a.menuRect()
	for i := range menuItems {
		if got := a.menuItemAt(r.X+20, r.Y+6+(float32(i)+.5)*a.menuRowHeight()); got != i {
			t.Fatalf("menu row %d hit row %d", i, got)
		}
	}
	a.menu = true
	a.activateMenu(8)
	if a.menu || a.overlay != "help" || len(a.buttons) != 0 || cap(a.buttons) == 0 {
		t.Fatal("menu activation did not dispatch and preserve button capacity")
	}
}

func BenchmarkIdlePoll(b *testing.B) {
	a := idleApp(b)
	b.ReportAllocs()
	for b.Loop() {
		a.poll()
	}
}

func BenchmarkMenuHitTest(b *testing.B) {
	a := &App{width: 1000, height: 700, menuX: 100, menuY: 100}
	b.ReportAllocs()
	for b.Loop() {
		_ = a.menuItemAt(150, 150)
	}
}
