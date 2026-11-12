//go:build js && wasm && !wechat

package window

import (
	"errors"
	"github.com/onix-engine/onix/scene"
	"strconv"
	"syscall/js"
)

// NewBrowserWebViewBackend creates a browser iframe backend. Inline HTML receives an
// Onix bridge; cross-origin navigations are still supported, but browsers do
// not permit injecting the bridge into a foreign document.
func NewBrowserWebViewBackend(context *Context) (scene.WebViewBackend, error) {
	if context == nil || !context.document.Truthy() {
		return nil, errors.New("window: browser document is unavailable")
	}
	backend := &browserWebViewBackend{
		context:  context,
		channels: make(map[string]*browserWebViewHandle),
	}
	backend.listener = js.FuncOf(func(_ js.Value, args []js.Value) interface{} {
		if len(args) > 0 {
			backend.receive(args[0])
		}
		return nil
	})
	backend.listenerSet = true
	context.document.Call("addEventListener", "message", backend.listener)
	return backend, nil
}

type browserWebViewBackend struct {
	context     *Context
	channels    map[string]*browserWebViewHandle
	listener    js.Func
	next        uint64
	listenerSet bool
	closed      bool
}

func (b *browserWebViewBackend) Close() error {
	if b == nil || b.closed {
		return nil
	}
	b.closed = true
	if b.context != nil && b.listenerSet {
		b.context.document.Call("removeEventListener", "message", b.listener)
		b.listener.Release()
	}
	return nil
}

func (b *browserWebViewBackend) Create(options scene.WebViewOptions, invoke scene.WebViewInvoke, events func(scene.WebViewEvent)) (scene.WebViewHandle, error) {
	if b == nil || b.context == nil || b.closed {
		return nil, errors.New("window: browser webview backend is unavailable")
	}
	b.next++
	channel := strconv.FormatUint(b.next, 10)
	frame := b.context.document.Call("createElement", "iframe")
	frame.Set("title", "Onix WebView")
	frame.Set("data-onix-webview", channel)
	style := frame.Get("style")
	style.Set("position", "absolute")
	style.Set("border", "0")
	style.Set("margin", "0")
	style.Set("padding", "0")
	style.Set("display", "block")
	if options.Transparent {
		style.Set("background", "transparent")
	}
	handle := &browserWebViewHandle{backend: b, frame: frame, channel: channel, invoke: invoke, events: events}
	handle.load = js.FuncOf(func(_ js.Value, _ []js.Value) interface{} {
		if handle.events != nil {
			if !handle.ready {
				handle.ready = true
				handle.events(scene.WebViewEvent{Kind: scene.WebViewEventReady})
			} else {
				handle.events(scene.WebViewEvent{Kind: scene.WebViewEventNavigation, URL: handle.frame.Get("src").String()})
			}
		}
		return nil
	})
	frame.Call("addEventListener", "load", handle.load)
	b.channels[channel] = handle
	b.setBounds(handle, options.Bounds)
	b.setVisible(handle, options.Visible)
	if options.URL != "" {
		frame.Set("src", options.URL)
	} else {
		frame.Set("srcdoc", browserWebViewHTML(options.HTML, channel))
	}
	b.context.document.Get("body").Call("appendChild", frame)
	return handle, nil
}

func (b *browserWebViewBackend) receive(event js.Value) {
	if b == nil || !event.Truthy() {
		return
	}
	data := event.Get("data")
	if !data.Truthy() || data.Type() != js.TypeObject {
		return
	}
	channel := data.Get("__onixWebView").String()
	if channel == "" || data.Get("type").String() != "invoke" {
		return
	}
	handle := b.channels[channel]
	if handle == nil || handle.invoke == nil {
		return
	}
	name := data.Get("name").String()
	payload := data.Get("payload").String()
	result, err := handle.invoke(name, payload)
	response := map[string]any{
		"__onixWebView": handle.channel,
		"type":          "response",
		"id":            data.Get("id").String(),
		"ok":            err == nil,
		"result":        result,
	}
	if err != nil {
		response["error"] = err.Error()
	}
	source := event.Get("source")
	if !source.Truthy() || !source.Equal(handle.frame.Get("contentWindow")) {
		return
	}
	source.Call("postMessage", js.ValueOf(response), "*")
}

func (b *browserWebViewBackend) setBounds(handle *browserWebViewHandle, bounds scene.WebViewBounds) {
	style := handle.frame.Get("style")
	style.Set("left", strconv.FormatFloat(float64(bounds.X), 'f', -1, 32)+"px")
	style.Set("top", strconv.FormatFloat(float64(bounds.Y), 'f', -1, 32)+"px")
	style.Set("width", strconv.FormatFloat(float64(max(bounds.Width, 0)), 'f', -1, 32)+"px")
	style.Set("height", strconv.FormatFloat(float64(max(bounds.Height, 0)), 'f', -1, 32)+"px")
}

func (b *browserWebViewBackend) setVisible(handle *browserWebViewHandle, visible bool) {
	display := "none"
	if visible {
		display = "block"
	}
	handle.frame.Get("style").Set("display", display)
}

type browserWebViewHandle struct {
	backend *browserWebViewBackend
	frame   js.Value
	channel string
	invoke  scene.WebViewInvoke
	events  func(scene.WebViewEvent)
	load    js.Func
	done    bool
	ready   bool
}

func (h *browserWebViewHandle) Navigate(url string) error {
	if h == nil || h.done {
		return errors.New("window: webview is closed")
	}
	h.frame.Set("src", url)
	return nil
}

func (h *browserWebViewHandle) SetHTML(html string) error {
	if h == nil || h.done {
		return errors.New("window: webview is closed")
	}
	h.frame.Set("srcdoc", browserWebViewHTML(html, h.channel))
	return nil
}

func (h *browserWebViewHandle) Eval(script string) error {
	if h == nil || h.done {
		return errors.New("window: webview is closed")
	}
	content := h.frame.Get("contentWindow")
	if !content.Truthy() {
		return errors.New("window: webview content window is unavailable")
	}
	content.Call("eval", script)
	return nil
}

func (h *browserWebViewHandle) SetBounds(bounds scene.WebViewBounds) error {
	if h == nil || h.done {
		return errors.New("window: webview is closed")
	}
	h.backend.setBounds(h, bounds)
	return nil
}

func (h *browserWebViewHandle) SetVisible(visible bool) error {
	if h == nil || h.done {
		return errors.New("window: webview is closed")
	}
	h.backend.setVisible(h, visible)
	return nil
}

func (h *browserWebViewHandle) Destroy() error {
	if h == nil || h.done {
		return nil
	}
	h.done = true
	delete(h.backend.channels, h.channel)
	h.frame.Call("removeEventListener", "load", h.load)
	h.load.Release()
	h.frame.Call("remove")
	return nil
}

func browserWebViewHTML(html, channel string) string {
	bridge := `<script>(function(){var c=` + strconv.Quote(channel) + `,n=0,p={};window.__onixInvoke=function(name,payload){return new Promise(function(resolve,reject){var id=String(++n);p[id]=[resolve,reject];parent.postMessage({__onixWebView:c,type:"invoke",id:id,name:String(name),payload:String(payload==null?"":payload)},"*")})};window.onix=window.onix||{};window.onix.invoke=window.__onixInvoke;window.addEventListener("message",function(e){var d=e.data;if(!d||d.__onixWebView!==c||d.type!=="response")return;var pair=p[d.id];if(!pair)return;delete p[d.id];d.ok?pair[0](d.result):pair[1](new Error(d.error||"webview invocation failed"))})})();</script>`
	return html + bridge
}
