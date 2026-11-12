package window

import (
	"github.com/onix-engine/onix/scene"
)

// ErrWebViewUnavailable is returned when no platform browser implementation
// is installed for the current target.
var ErrWebViewUnavailable = scene.ErrWebViewUnavailable

// WebViewBackendFactory allows a host application to provide a native browser
// backend without changing the ECS protocol.
// Registration should happen before creating the window context.
type WebViewBackendFactory func(*Context) (scene.WebViewBackend, error)

var webViewBackendFactory WebViewBackendFactory
var webViewBackendRequiresX11 bool

// RegisterWebViewBackend installs a host-provided platform backend factory.
// Passing nil clears the current registration.
func RegisterWebViewBackend(factory WebViewBackendFactory) {
	webViewBackendFactory = factory
	webViewBackendRequiresX11 = false
}

// RegisterWebViewBackendIfUnset installs factory only when no backend factory
// has been registered. Optional built-in adapters use this form so an
// application-provided factory keeps precedence.
func RegisterWebViewBackendIfUnset(factory WebViewBackendFactory) bool {
	if webViewBackendFactory != nil {
		return false
	}
	webViewBackendFactory = factory
	return true
}

// RegisterX11WebViewBackendIfUnset installs an embedded Linux backend that
// requires an X11 parent window. Linux window creation will prefer X11 or
// XWayland when this registration succeeds.
func RegisterX11WebViewBackendIfUnset(factory WebViewBackendFactory) bool {
	if !RegisterWebViewBackendIfUnset(factory) {
		return false
	}
	webViewBackendRequiresX11 = true
	return true
}

// NewWebViewBackend selects a host-provided backend. The built-in adapters live
// in the optional window/webview package so applications that do not enable
// WebView do not link platform browser dependencies.
func NewWebViewBackend(context *Context) (scene.WebViewBackend, error) {
	if webViewBackendFactory != nil {
		return webViewBackendFactory(context)
	}
	return nil, scene.ErrWebViewUnavailable
}
