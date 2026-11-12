package scene

import (
	"errors"
	"fmt"

	"github.com/onix-engine/onix/ecs"
	"math"
	"sync"
)

// WebViewHandler handles a named invocation from a web page. Payload is the
// JSON text supplied by the page and the returned string is resolved back to
// the JavaScript Promise. Returning an error rejects that Promise.
type WebViewHandler func(payload string) (result string, err error)

// WebViewMessageHandler handles every bridge invocation, including its name.
type WebViewMessageHandler func(name, payload string) (result string, err error)

// WebView is a native browser surface owned by an ECS entity. URL and HTML are
// mutually exclusive; URL takes precedence when both are set. Bounds are
// expressed in logical window pixels.
//
// The component is deliberately independent of any platform UI type. Install
// a WebViewRuntime resource and WebViewSystem to materialize it.
type WebView struct {
	OnMessage WebViewHandler
	OnInvoke  WebViewMessageHandler
	Bindings  map[string]WebViewHandler
	HTML      string
	URL       string
	// Bounds is an alternate grouped form of X/Y/Width/Height. When it is
	// non-zero it takes precedence over the scalar fields.
	Bounds      WebViewBounds
	Y           float32
	Height      float32
	Width       float32
	X           float32
	Visible     bool
	DevTools    bool
	Transparent bool
}

// NewWebView creates a visible WebView that navigates to url.
func NewWebView(url string) WebView {
	return WebView{URL: url, Visible: true}
}

// NewWebViewHTML creates a visible WebView containing inline HTML.
func NewWebViewHTML(html string) WebView {
	return WebView{HTML: html, Visible: true}
}

// Bind adds or replaces a named JavaScript invocation handler. Bindings are
// resolved from the current component on invocation, so changing the map does
// not recreate the native view.
func (w *WebView) Bind(name string, handler WebViewHandler) bool {
	if w == nil || name == "" || handler == nil {
		return false
	}
	if w.Bindings == nil {
		w.Bindings = make(map[string]WebViewHandler, 1)
	}
	w.Bindings[name] = handler
	return true
}

// Unbind removes a named JavaScript invocation handler.
func (w *WebView) Unbind(name string) bool {
	if w == nil || len(w.Bindings) == 0 {
		return false
	}
	if _, ok := w.Bindings[name]; !ok {
		return false
	}
	delete(w.Bindings, name)
	return true
}

// WebViewBounds is the logical rectangle occupied by a browser surface.
type WebViewBounds struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// WebViewOptions is passed to a platform backend when an instance is created.
type WebViewOptions struct {
	URL         string
	HTML        string
	Bounds      WebViewBounds
	Visible     bool
	DevTools    bool
	Transparent bool
}

// WebViewEventKind identifies a platform event delivered to ECS.
type WebViewEventKind uint8

const (
	WebViewEventReady WebViewEventKind = iota + 1
	WebViewEventMessage
	WebViewEventNavigation
	WebViewEventError
	WebViewEventClosed
)

// WebViewEvent is emitted through the ECS message queue by WebViewRuntime.
// Message contains the raw JSON payload for WebViewEventMessage. Error is a
// stable text copy so events remain safe after a backend callback returns.
type WebViewEvent struct {
	Name    string
	Payload string
	Message string
	URL     string
	Error   string
	Entity  ecs.Entity
	Kind    WebViewEventKind
}

// WebViewHandle is the platform-owned browser surface.
type WebViewHandle interface {
	Navigate(url string) error
	SetHTML(html string) error
	Eval(script string) error
	SetBounds(bounds WebViewBounds) error
	SetVisible(visible bool) error
	Destroy() error
}

// WebViewInvoke dispatches a named JavaScript bridge call to Go. The backend
// must call it on the platform UI thread and use the returned result to settle
// the JavaScript Promise.
type WebViewInvoke func(name, payload string) (result string, err error)

// WebViewBackend creates browser surfaces. The callback may be called by the
// backend at any time while the returned handle is alive.
type WebViewBackend interface {
	Create(options WebViewOptions, invoke WebViewInvoke, events func(WebViewEvent)) (WebViewHandle, error)
}

// ErrWebViewUnavailable indicates that the current target has no browser
// backend. WeChat builds intentionally return this error.
var ErrWebViewUnavailable = errors.New("ecs: webview is unavailable on this platform")

// WebViewRuntime owns native handles for all WebView components in one world.
// It is safe to reuse a runtime across frames; synchronization only invokes
// backend methods when component state has changed.
type WebViewRuntime struct {
	Backend WebViewBackend
	entries map[ecs.Entity]webViewEntry
	invokes map[ecs.Entity]webViewInvocation
	pending []WebViewEvent
	mu      sync.Mutex
	closed  bool
}

type webViewEntry struct {
	handle      WebViewHandle
	url         string
	html        string
	bounds      WebViewBounds
	visible     bool
	devTools    bool
	transparent bool
}

type webViewInvocation struct {
	bindings  map[string]WebViewHandler
	onInvoke  WebViewMessageHandler
	onMessage WebViewHandler
}

// NewWebViewRuntime creates a runtime using backend. A nil backend is valid
// and causes WebViewSystem to emit one unavailable event per entity.
func NewWebViewRuntime(backend WebViewBackend) *WebViewRuntime {
	return &WebViewRuntime{Backend: backend}
}

// Close destroys every materialized browser surface. It is idempotent.
func (r *WebViewRuntime) Close() error {
	if r == nil || r.closed {
		return nil
	}
	r.closed = true
	var first error
	for entity, entry := range r.entries {
		if entry.handle != nil {
			if err := entry.handle.Destroy(); err != nil && first == nil {
				first = fmt.Errorf("destroy webview %d: %w", entity, err)
			}
		}
		delete(r.entries, entity)
	}
	r.mu.Lock()
	clear(r.invokes)
	r.mu.Unlock()
	if closer, ok := r.Backend.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil && first == nil {
			first = err
		}
	}
	r.mu.Lock()
	clear(r.pending)
	r.pending = nil
	r.mu.Unlock()
	return first
}

// Handle returns the native handle for entity after synchronization. The
// handle is owned by the runtime and must not be destroyed by the caller.
func (r *WebViewRuntime) Handle(entity ecs.Entity) (WebViewHandle, bool) {
	if r == nil || r.closed {
		return nil, false
	}
	entry, ok := r.entries[entity]
	return entry.handle, ok && entry.handle != nil
}

// Eval evaluates script in an existing browser surface.
func (r *WebViewRuntime) Eval(entity ecs.Entity, script string) error {
	handle, ok := r.Handle(entity)
	if !ok {
		return fmt.Errorf("ecs: webview %d is not materialized", entity)
	}
	return handle.Eval(script)
}

// Navigate requests navigation without changing the authored component. The
// component's URL is updated as well, so the request remains stable across the
// next synchronization pass.
func (r *WebViewRuntime) Navigate(world *ecs.World, entity ecs.Entity, url string) error {
	if world == nil {
		return errors.New("ecs: webview world is nil")
	}
	value := world.GetMut[WebView](entity)
	if value == nil {
		return fmt.Errorf("ecs: entity %d has no WebView component", entity)
	}
	value.URL, value.HTML = url, ""
	if handle, exists := r.Handle(entity); exists {
		if err := handle.Navigate(url); err != nil {
			return err
		}
		entry := r.entries[entity]
		entry.url, entry.html = url, ""
		r.entries[entity] = entry
	}
	return nil
}

func (r *WebViewRuntime) enqueue(entity ecs.Entity, event WebViewEvent) {
	if r == nil {
		return
	}
	event.Entity = entity
	r.mu.Lock()
	r.pending = append(r.pending, event)
	r.mu.Unlock()
}

func (r *WebViewRuntime) drain(world *ecs.World) {
	r.mu.Lock()
	pending := r.pending
	r.pending = nil
	r.mu.Unlock()
	for _, event := range pending {
		if event.Kind == WebViewEventClosed {
			delete(r.entries, event.Entity)
			r.mu.Lock()
			delete(r.invokes, event.Entity)
			r.mu.Unlock()
		}
		if world.IsAlive(event.Entity) && world.Has[WebView](event.Entity) {
			world.WriteMessage(event)
		}
	}
	clear(pending)
}

// WebViewSystem synchronizes WebView components into the runtime resource.
// It is intended for an update stage after scene/component mutations.
func WebViewSystem(name string) ecs.SystemConfig {
	if name == "" {
		name = "sync_webviews"
	}
	return ecs.SystemConfig{Name: name, System: syncWebViews}
}

// WebViewTaskCount keeps native browser callbacks serviced while a view exists.
// Installing the feature without any browser surfaces does not prevent idle.
func WebViewTaskCount(world *ecs.World) int {
	count := 0
	for range world.Query[WebView]().All {
		count++
	}
	return count
}

// SyncWebViews runs one WebView synchronization pass immediately.
func SyncWebViews(world *ecs.World) error {
	return WebViewSystem("").Run(world)
}

// EvalWebView evaluates script in a materialized WebView.
func EvalWebView(world *ecs.World, entity ecs.Entity, script string) error {
	if world == nil {
		return errors.New("ecs: webview world is nil")
	}
	runtime, ok := world.GetResource[*WebViewRuntime]()
	if !ok || runtime == nil {
		return ErrWebViewUnavailable
	}
	return runtime.Eval(entity, script)
}

// NavigateWebView updates a WebView URL and applies it immediately when its
// native handle already exists. A later synchronization pass retries the
// operation when the component was not materialized yet.
func NavigateWebView(world *ecs.World, entity ecs.Entity, url string) error {
	if world == nil {
		return errors.New("ecs: webview world is nil")
	}
	runtime, ok := world.GetResource[*WebViewRuntime]()
	if !ok || runtime == nil {
		return ErrWebViewUnavailable
	}
	return runtime.Navigate(world, entity, url)
}

func syncWebViews(context *ecs.SystemContext) error {
	if context == nil || context.World == nil {
		return nil
	}
	runtime, ok := context.World.GetResource[*WebViewRuntime]()
	if !ok || runtime == nil {
		return nil
	}
	if runtime.closed {
		return nil
	}
	if runtime.entries == nil {
		runtime.entries = make(map[ecs.Entity]webViewEntry)
	}
	if runtime.invokes == nil {
		runtime.invokes = make(map[ecs.Entity]webViewInvocation)
	}
	runtime.drain(context.World)
	for entity, access := range context.World.Query[WebView](ecs.Unscoped()).All {
		value := access.Get()
		value.Visible = value.Visible && IsInteractive(context.World, entity)

		runtime.syncEntity(context.World, entity, value)
	}
	for entity, entry := range runtime.entries {
		if context.World.IsAlive(entity) && context.World.Has[WebView](entity) {
			continue
		}
		if entry.handle != nil {
			_ = entry.handle.Destroy()
		}
		delete(runtime.entries, entity)
		runtime.mu.Lock()
		delete(runtime.invokes, entity)
		runtime.mu.Unlock()
	}
	return nil
}

func (r *WebViewRuntime) syncEntity(world *ecs.World, entity ecs.Entity, value WebView) {
	r.mu.Lock()
	r.invokes[entity] = webViewInvocation{bindings: value.Bindings, onInvoke: value.OnInvoke, onMessage: value.OnMessage}
	r.mu.Unlock()
	url, html := value.URL, value.HTML
	if url != "" {
		html = ""
	}
	bounds := value.Bounds
	if bounds == (WebViewBounds{}) {
		bounds = WebViewBounds{X: value.X, Y: value.Y, Width: value.Width, Height: value.Height}
	}
	bounds.X, bounds.Y = webViewCoordinate(bounds.X), webViewCoordinate(bounds.Y)
	bounds.Width, bounds.Height = webViewDimension(bounds.Width), webViewDimension(bounds.Height)
	entry, exists := r.entries[entity]
	if !exists {
		if r.Backend == nil {
			world.WriteMessage(WebViewEvent{Entity: entity, Kind: WebViewEventError, Error: ErrWebViewUnavailable.Error()})
			r.entries[entity] = webViewEntry{url: url, html: html, bounds: bounds, visible: value.Visible, devTools: value.DevTools, transparent: value.Transparent}
			return
		}
		invoke := func(name, payload string) (string, error) {
			r.enqueue(entity, WebViewEvent{Kind: WebViewEventMessage, Name: name, Payload: payload, Message: payload})
			return r.invoke(entity, name, payload)
		}
		events := func(event WebViewEvent) { r.enqueue(entity, event) }
		handle, err := r.Backend.Create(WebViewOptions{URL: url, HTML: html, Bounds: bounds, Visible: value.Visible, DevTools: value.DevTools, Transparent: value.Transparent}, invoke, events)
		if err == nil && handle == nil {
			err = errors.New("ecs: webview backend returned a nil handle")
		}
		if err != nil {
			world.WriteMessage(WebViewEvent{Entity: entity, Kind: WebViewEventError, Error: err.Error()})
			r.entries[entity] = webViewEntry{url: url, html: html, bounds: bounds, visible: value.Visible, devTools: value.DevTools, transparent: value.Transparent}
			return
		}
		entry = webViewEntry{handle: handle, url: url, html: html, bounds: bounds, visible: value.Visible, devTools: value.DevTools, transparent: value.Transparent}
		r.entries[entity] = entry
		return
	}
	if entry.handle == nil {
		if entry.url != url || entry.html != html || entry.bounds != bounds || entry.visible != value.Visible || entry.devTools != value.DevTools || entry.transparent != value.Transparent {
			delete(r.entries, entity)
			r.syncEntity(world, entity, value)
		}
		return
	}
	if entry.devTools != value.DevTools || entry.transparent != value.Transparent {
		_ = entry.handle.Destroy()
		delete(r.entries, entity)
		r.syncEntity(world, entity, value)
		return
	}
	if url != "" && entry.url != url {
		if err := entry.handle.Navigate(url); err != nil {
			world.WriteMessage(WebViewEvent{Entity: entity, Kind: WebViewEventError, Error: err.Error()})
		}
		entry.url, entry.html = url, html
	} else if url == "" && (entry.url != "" || entry.html != html) {
		if err := entry.handle.SetHTML(html); err != nil {
			world.WriteMessage(WebViewEvent{Entity: entity, Kind: WebViewEventError, Error: err.Error()})
		}
		entry.html = html
	}
	if entry.bounds != bounds {
		if err := entry.handle.SetBounds(bounds); err != nil {
			world.WriteMessage(WebViewEvent{Entity: entity, Kind: WebViewEventError, Error: err.Error()})
		}
		entry.bounds = bounds
	}
	if entry.visible != value.Visible {
		if err := entry.handle.SetVisible(value.Visible); err != nil {
			world.WriteMessage(WebViewEvent{Entity: entity, Kind: WebViewEventError, Error: err.Error()})
		}
		entry.visible = value.Visible
	}
	entry.devTools, entry.transparent = value.DevTools, value.Transparent
	r.entries[entity] = entry
}

func webViewCoordinate(value float32) float32 {
	if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
		return 0
	}
	return value
}

func webViewDimension(value float32) float32 {
	if value <= 0 || math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
		return 0
	}
	return value
}

func (r *WebViewRuntime) invoke(entity ecs.Entity, name, payload string) (string, error) {
	if r == nil {
		return "", ErrWebViewUnavailable
	}
	r.mu.Lock()
	value, ok := r.invokes[entity]
	r.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("ecs: webview entity %d no longer exists", entity)
	}
	if handler := value.bindings[name]; handler != nil {
		return handler(payload)
	}
	if value.onInvoke != nil {
		return value.onInvoke(name, payload)
	}
	if value.onMessage != nil {
		return value.onMessage(payload)
	}
	return "", fmt.Errorf("ecs: webview binding %q is not registered", name)
}
