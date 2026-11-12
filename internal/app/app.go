// Package app implements XiTerm's native Onix window and terminal workspace.
package app

import (
	"fmt"
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/diiyw/xiterm/assets"
	"github.com/diiyw/xiterm/internal/config"
	"github.com/diiyw/xiterm/internal/terminal"
	"github.com/diiyw/xiterm/internal/workspace"
	"github.com/onix-engine/onix/graphics"
	"github.com/onix-engine/onix/input"
	"github.com/onix-engine/onix/render"
	"github.com/onix-engine/onix/text"
	"github.com/onix-engine/onix/window"
	iconpng "github.com/onix-engine/onix/window/codec/png"
	"github.com/onix-engine/onix/window/filedialog"
)

const (
	tabsH     float32 = 42
	paneHeadH float32 = 24
	searchH   float32 = 38
)

type button struct {
	rect   workspace.Rect
	action buttonAction
	value  int
}

type pollPane struct {
	pane *workspace.Pane
	tab  int
}

type App struct {
	textInput         textInputState
	logo              *graphics.Texture
	icons             map[string]*graphics.Texture
	keyModifiers      func(input.KeyCode) (uv.KeyMod, bool)
	nerdFont          *text.Font
	uiFont            *text.Font
	mousePane         *workspace.Pane
	dragSplit         *workspace.Node
	tabDrag           tabDragState
	lastTitle         string
	lastSessionTitle  string
	mouseX, mouseY    float32
	win               *window.Context
	cfg               config.Config
	configPath        string
	tabs              []*workspace.Tab
	active            int
	paint             painter
	font, boldFont    *text.Font
	buttons           []button
	keys              map[input.KeyCode]bool
	search            bool
	query             string
	historyBudget     *terminal.HistoryBudget
	searchIndex       terminal.SearchIndex
	bufferTrimAt      time.Time
	trimRenderBuffers func() error
	matches           []terminal.Match
	matchIndex        int
	searchRevision    uint64
	searchSession     *terminal.Session
	overlay           string
	menu              bool
	menuX, menuY      float32
	menuIndex         int
	notice            string
	noticeUntil       time.Time
	dragPane          *workspace.Pane
	width, height     float32
	dirty             bool
	blink             bool
	blinkAt           time.Time
	paintAt           time.Time
	pollOffset        int
	pollPanes         []pollPane
	smokeDeadline     time.Time
}

type Options struct {
	Config        config.Config
	ConfigPath    string
	Directory     string
	SmokeDuration time.Duration
}

func Run(opts Options) error {
	runtime.LockOSThread()
	filedialog.Register()
	a := &App{cfg: opts.Config, configPath: opts.ConfigPath, keys: map[input.KeyCode]bool{}, dirty: true, matchIndex: -1}
	a.win = window.NewContext(a.cfg.Width, a.cfg.Height, "XiTerm", window.DefaultOptions())
	defer a.win.Destroy()
	a.trimRenderBuffers = render.ActiveDevice().TrimIdleBuffers
	a.historyBudget = terminal.NewHistoryBudget(64 << 20)
	a.paint.solid = graphics.CreateTextureByColor(graphics.White)
	defer a.paint.solid.Dispose()
	a.win.SetIconDecoder(iconpng.Decoder{})
	if err := a.win.SetIcon(assets.IconPNG); err != nil {
		return fmt.Errorf("set application icon: %w", err)
	}
	decoder := iconpng.Decoder{}
	icon, err := decoder.DecodeIcon(assets.IconPNG)
	if err != nil {
		return err
	}
	rgba := image.NewRGBA(icon.Bounds())
	draw.Draw(rgba, rgba.Bounds(), icon, icon.Bounds().Min, draw.Src)
	a.logo = graphics.CreateTextureByImageNoMipmap(rgba)
	defer a.logo.Dispose()
	// Adaptive uses native logical pixels, including when the window shrinks.
	// Expand scales a fixed scene and would leave the PTY grid unchanged.
	a.win.SetViewport(1, 1)
	a.win.SetViewportMode(window.ViewportModeAdaptive)
	if err := a.loadIcons(); err != nil {
		return err
	}
	defer a.disposeIcons()
	a.win.SetVsyncEnabled(true)
	// Pace painting ourselves so native input and PTY parsing can continue
	// between frames without sleeping inside Onix's event loop.
	a.win.SetTargetFPS(0)
	regular, bold := assets.Regular, assets.Bold
	if a.cfg.FontPath != "" {
		b, err := os.ReadFile(a.cfg.FontPath)
		if err != nil {
			return err
		}
		regular = b
		bold = b
	}
	if a.font, err = text.LoadFont(regular); err != nil {
		return err
	}
	defer a.font.Dispose()
	if a.cfg.FontPath != "" {
		a.boldFont = a.font
	} else {
		if a.boldFont, err = text.LoadFont(bold); err != nil {
			return err
		}
		defer a.boldFont.Dispose()
	}
	if a.cfg.FontPath != "" {
		a.nerdFont, err = text.LoadFont(assets.Regular)
		if err != nil {
			return err
		}
		defer a.nerdFont.Dispose()
	}
	if a.uiFont, err = loadSystemUIFont(); err != nil {
		return fmt.Errorf("load system UI font: %w", err)
	}
	defer a.uiFont.Dispose()
	a.setFontSize(a.cfg.FontSize)
	defer a.paint.disposeGlyphCaches()
	profile := a.cfg.Profile()
	if opts.Directory != "" {
		profile.Directory = opts.Directory
	}
	if err = a.newTab(profile); err != nil {
		return err
	}
	defer a.closeAll()
	a.bindInput()
	cleanupInput := a.installNativeInput()
	defer cleanupInput()
	if opts.SmokeDuration > 0 {
		a.smokeDeadline = time.Now().Add(opts.SmokeDuration)
	}
	a.win.SetIdleDeadline(a.idleDeadline)
	return a.win.LoopGame(func() (bool, error) {
		if !a.smokeDeadline.IsZero() && time.Now().After(a.smokeDeadline) {
			a.win.RequestClose()
		}
		a.poll()
		if a.win.TakeFrameRequest() {
			a.dirty = true
		}
		if err := a.trimIdleMemory(time.Now()); err != nil {
			return false, err
		}
		a.advanceTimers(time.Now())
		w, h := a.win.GetViewport()
		if a.width != float32(w) || a.height != float32(h) {
			a.width = float32(w)
			a.height = float32(h)
			a.scheduleMemoryTrim(time.Now())
			a.dirty = true
		}
		if !a.frameReady(time.Now()) {
			return false, nil
		}
		a.dirty = false
		a.layout()
		a.win.SetTextInputConfig(a.textInputConfig())
		a.paint.canvas = a.win.Canvas2D(w, h)
		a.draw()
		a.paint.regular.FlushUploads()
		a.paint.bold.FlushUploads()
		return true, a.paint.canvas.FinishFrame()
	})
}

// Only animate while the cursor can actually be seen. A zero idle deadline
// lets the native loop sleep until input or PTY output arrives.
func (a *App) cursorCanBlink() bool {
	p := a.pane()
	return p != nil && !a.search && !a.menu && a.overlay == "" &&
		p.Scroll == 0 && !p.Session.Exited && !p.Session.Term.CursorHidden()
}

func (a *App) idleDeadline() (time.Time, bool) {
	for _, t := range a.tabs {
		pending := false
		t.Root.Visit(func(p *workspace.Pane) { pending = pending || p.Session.PendingOutput() })
		if pending {
			return time.Time{}, false
		}
	}
	var deadline time.Time
	if a.dirty || a.win.FrameRequested() {
		if !time.Now().Before(a.paintAt) {
			return time.Time{}, false
		}
		deadline = a.paintAt
	}
	if a.cursorCanBlink() {
		if deadline.IsZero() || a.blinkAt.Before(deadline) {
			deadline = a.blinkAt
		}
	}
	for _, next := range []time.Time{a.noticeUntil, a.smokeDeadline, a.bufferTrimAt} {
		if !next.IsZero() && (deadline.IsZero() || next.Before(deadline)) {
			deadline = next
		}
	}
	return deadline, true
}

// Limit painting to 60 FPS while leaving event handling free to run sooner.
func (a *App) frameReady(now time.Time) bool {
	if !a.dirty || now.Before(a.paintAt) {
		return false
	}
	a.paintAt = now.Add(time.Second / 60)
	return true
}

func (a *App) advanceTimers(now time.Time) {
	if a.cursorCanBlink() {
		if !now.Before(a.blinkAt) {
			a.blink = !a.blink
			a.blinkAt = now.Add(550 * time.Millisecond)
			a.dirty = true
		}
	} else {
		a.blink = false
		a.blinkAt = time.Time{}
	}
	if !a.noticeUntil.IsZero() && !now.Before(a.noticeUntil) {
		a.noticeUntil = time.Time{}
		a.dirty = true
	}
}

func (a *App) setFontSize(size int) {
	a.cfg.FontSize = max(10, min(36, size))
	if a.paint.regular != nil && a.paint.size == float32(a.cfg.FontSize) {
		return
	}
	a.paint.disposeGlyphCaches()
	a.scheduleMemoryTrim(time.Now())
	a.paint.size = float32(a.cfg.FontSize)
	// Standalone caches own their atlases, so old sizes are released on zoom
	// instead of accumulating in Font's cache for the process lifetime.
	a.paint.regular = text.NewGlyphCache(a.font, float64(a.cfg.FontSize), 72)
	a.paint.bold = a.paint.regular
	if a.boldFont != a.font {
		a.paint.bold = text.NewGlyphCache(a.boldFont, float64(a.cfg.FontSize), 72)
	}
	if a.nerdFont != nil {
		a.paint.nerd = text.NewGlyphCache(a.nerdFont, float64(a.cfg.FontSize), 72)
	}
	if a.uiFont != nil {
		a.paint.ui = text.NewGlyphCache(a.uiFont, float64(a.cfg.FontSize), 72)
	}
	a.paint.cellW = a.paint.regular.GlyphMetrics('M').Advance
	a.paint.lineH = float32(a.cfg.FontSize) + 7
	a.dirty = true
}
func (a *App) tab() *workspace.Tab {
	if len(a.tabs) == 0 {
		return nil
	}
	return a.tabs[a.active]
}
func (a *App) pane() *workspace.Pane {
	if t := a.tab(); t != nil {
		return t.Active
	}
	return nil
}
func (a *App) newPane(p config.Profile) (*workspace.Pane, error) {
	s, err := terminal.Start(p, 80, 24, a.cfg.Scrollback, a.win.RequestFrame)
	if err != nil {
		return nil, err
	}
	s.SetHistoryBudget(a.historyBudget)
	a.scheduleMemoryTrim(time.Now())
	return &workspace.Pane{Session: s}, nil
}
func (a *App) newTab(p config.Profile) error {
	pane, err := a.newPane(p)
	if err != nil {
		return err
	}
	a.tabs = append(a.tabs, &workspace.Tab{Root: workspace.Leaf(pane), Active: pane})
	a.endTabDrag()
	a.active = len(a.tabs) - 1
	a.resetSearch()
	a.dirty = true
	return nil
}
func (a *App) split(axis workspace.Axis) {
	p := a.pane()
	if p == nil {
		return
	}
	if axis == workspace.Columns && p.Bounds.W < 32*a.paint.cellW || axis == workspace.Rows && p.Bounds.H < 8*a.paint.lineH {
		a.notify("This pane is too small to split")
		return
	}
	profile := p.Session.Profile
	// OSC 7 is untrusted output. Only inherit an existing local directory.
	if dir := localDirectory(p.Session.Directory); dir != "" {
		profile.Directory = dir
	}
	next, err := a.newPane(profile)
	if err != nil {
		a.notify(err.Error())
		return
	}
	a.tab().Root.Split(p, next, axis)
	a.tab().Active = next
	a.resetSearch()
	a.dirty = true
}
func (a *App) closePane() {
	a.closePaneAt(a.active, a.pane())
}
func (a *App) closePaneAt(index int, p *workspace.Pane) {
	if p == nil || index < 0 || index >= len(a.tabs) {
		return
	}
	t := a.tabs[index]
	a.endTabDrag()
	p.Session.Close()
	t.Root = t.Root.Remove(p)
	if t.Root == nil {
		a.tabs = slices.Delete(a.tabs, index, index+1)
		if len(a.tabs) == 0 {
			a.win.RequestClose()
			return
		}
		if a.active > index {
			a.active--
		}
		a.active = min(a.active, len(a.tabs)-1)
	} else if t.Active == p {
		t.Active = t.Root.FirstPane()
	}
	a.dragPane = nil
	a.dragSplit = nil
	a.mousePane = nil
	a.resetSearch()
	a.dirty = true
}
func (a *App) closeTab(index int) {
	if index < 0 || index >= len(a.tabs) {
		return
	}
	a.endTabDrag()
	a.tabs[index].Root.Visit(func(p *workspace.Pane) { p.Session.Close() })
	a.tabs = slices.Delete(a.tabs, index, index+1)
	if len(a.tabs) == 0 {
		a.win.RequestClose()
		return
	}
	if a.active > index {
		a.active--
	}
	a.active = min(a.active, len(a.tabs)-1)
	a.dragPane = nil
	a.dragSplit = nil
	a.mousePane = nil
	a.resetSearch()
	a.dirty = true
}
func (a *App) closeAll() {
	for _, t := range a.tabs {
		t.Root.Visit(func(p *workspace.Pane) { p.Session.Close() })
	}
}
func (a *App) notify(message string) {
	a.notice = message
	a.noticeUntil = time.Now().Add(5 * time.Second)
	a.dirty = true
}
func (a *App) poll() {
	panes := a.pollPanes[:0]
	for i, t := range a.tabs {
		t.Root.Visit(func(p *workspace.Pane) { panes = append(panes, pollPane{pane: p, tab: i}) })
	}
	deadline := time.Now().Add(4 * time.Millisecond)
	for i, start := 0, a.pollOffset; i < len(panes); i++ {
		index := (start + i) % len(panes)
		p := panes[index].pane
		history := p.Session.HistoryLen()
		if p.Session.PollUntil(deadline) {
			if p.Scroll > 0 {
				p.Scroll += max(0, p.Session.HistoryLen()-history)
			}
			a.dirty = true
			a.scheduleMemoryTrim(time.Now())
			p.Scroll = min(p.Scroll, p.Session.HistoryLen())
			if a.dragPane != p {
				p.Selected = false
			}
		}
		a.pollOffset = (index + 1) % len(panes)
		if !time.Now().Before(deadline) {
			break
		}
	}
	// Remove exited sessions after polling so slice mutations cannot skip a tab.
	for _, p := range panes {
		// Another pane's output can evict rows through the shared budget even
		// when this session had no output of its own.
		p.pane.Scroll = min(p.pane.Scroll, p.pane.Session.HistoryLen())
		if !p.pane.Session.Exited {
			a.handleTransfer(p.pane.Session)
		}
	}
	for i := len(panes) - 1; i >= 0; i-- {
		if p := panes[i]; p.pane.Session.Exited {
			a.closePaneAt(p.tab, p.pane)
		}
	}
	// Preserve capacity without retaining sessions removed during this poll.
	clear(panes)
	a.pollPanes = panes[:0]
	if p := a.pane(); p != nil {
		if a.search && (a.searchSession != p.Session || a.searchRevision != p.Session.Revision) {
			a.updateSearch(false)
		}
		if p.Session.Title != a.lastSessionTitle || a.lastTitle == "" {
			title := "XiTerm — " + shorten(p.Session.Title, 80)
			if title != a.lastTitle {
				a.win.SetTitle(title)
				a.lastTitle = title
			}
			a.lastSessionTitle = p.Session.Title
		}
	}
}
func (a *App) layout() {
	top := tabsH
	if a.search {
		top += searchH
	}
	for _, t := range a.tabs {
		t.Root.Layout(workspace.Rect{X: 0, Y: top, W: a.width, H: max(1, a.height-top)})
		// Hidden tabs retain their viewport and resize together to avoid a stale PTY size.
		t.Root.Visit(func(p *workspace.Pane) {
			cols := max(2, int((p.Bounds.W-24)/a.paint.cellW))
			rows := max(1, int((p.Bounds.H-a.paneHeaderHeight(p)-16)/a.paint.lineH))
			if err := p.Session.Resize(cols, rows); err != nil && p.Session.IOErr == nil {
				p.Session.IOErr = err
				a.notify(err.Error())
			}
		})
	}
}
func (a *App) paneHeaderHeight(p *workspace.Pane) float32 {
	for _, t := range a.tabs {
		if t.Root.Pane == nil && t.Root.Contains(p) {
			return paneHeadH
		}
	}
	return 0
}
func (a *App) resetSearch() {
	a.searchSession = nil
	a.matches = nil
	a.searchIndex.Clear()
	a.matchIndex = -1
	a.dirty = true
}
func (a *App) updateSearch(jump bool) {
	p := a.pane()
	if p == nil {
		return
	}
	p.Session.IndexSearch(&a.searchIndex, a.query)
	a.searchRevision = p.Session.Revision
	a.searchSession = p.Session
	if a.searchIndex.Count() == 0 {
		a.matchIndex = -1
	} else if a.matchIndex < 0 || a.matchIndex >= a.searchIndex.Count() {
		a.matchIndex = a.searchIndex.Count() - 1
	}
	if jump {
		a.revealMatch()
	}
	a.dirty = true
}
func (a *App) revealMatch() {
	if a.matchIndex < 0 || a.matchIndex >= a.searchIndex.Count() {
		return
	}
	p := a.pane()
	m, ok := a.searchIndex.Match(p.Session, a.matchIndex)
	if !ok {
		return
	}
	p.Scroll = max(0, min(p.Session.HistoryLen(), p.Session.HistoryLen()-m.Row+p.Session.Term.Height()/2))
	a.dirty = true
}
func (a *App) nextMatch(delta int) {
	if a.searchIndex.Count() == 0 {
		return
	}
	a.matchIndex = (a.matchIndex + delta + a.searchIndex.Count()) % a.searchIndex.Count()
	a.revealMatch()
}
func (a *App) switchTab(index int) {
	if index < 0 || index >= len(a.tabs) {
		return
	}
	a.endTabDrag()
	a.active = index
	a.dragPane = nil
	a.dragSplit = nil
	a.mousePane = nil
	a.resetSearch()
	a.dirty = true
}
func (a *App) toggleSearch() {
	a.search = !a.search
	a.overlay = ""
	if a.search {
		a.updateSearch(true)
	} else {
		a.resetSearch()
	}
	a.dirty = true
}
func (a *App) saveConfig() {
	if err := config.Save(a.configPath, a.cfg); err != nil {
		a.notify(err.Error())
	} else {
		a.notify("Preferences saved to " + a.configPath)
	}
}

func localDirectory(value string) string {
	// Shells may report either a local path or an OSC 7 file URL.
	if strings.HasPrefix(value, "file://") {
		return directoryFromURL(value)
	}
	if !filepath.IsAbs(value) {
		return ""
	}
	if info, err := os.Stat(value); err == nil && info.IsDir() {
		return value
	}
	return ""
}
func (a *App) addButton(label string, r workspace.Rect, active bool, action buttonAction, value int) {
	a.buttons = append(a.buttons, button{rect: r, action: action, value: value})
	if active {
		a.paint.rect(r, surface)
	}
	a.paint.label(label, r.X+10, r.Y+(r.H-a.paint.size)/2-1, foreground, false)
}
func (a *App) runAction(fn func() error) {
	if err := fn(); err != nil {
		a.notify(err.Error())
	}
	a.dirty = true
}

func (a *App) currentProfile() config.Profile {
	if p := a.pane(); p != nil {
		profile := p.Session.Profile
		if dir := localDirectory(p.Session.Directory); dir != "" {
			profile.Directory = dir
		}
		return profile
	}
	return a.cfg.Profile()
}
