//go:build linux && cgo && !android && !ios && !js && !wasm && !onix_drm

package window

/*
#cgo pkg-config: x11 xext xrender xi wayland-client wayland-cursor xkbcommon
#cgo noescape onix_window_poll_events_batch
#cgo nocallback onix_window_poll_events_batch
#include <stdlib.h>
#include "linux_native.h"

static int onix_window_poll_events_batch(OnixNativeWindow *window, OnixEvent *events, int capacity, int *should_close) {
	onix_window_poll_events(window);
	int count = 0;
	while (count < capacity && onix_window_pop_event(window, &events[count]) != 0) {
		count++;
	}
	*should_close = onix_window_should_close(window);
	return count;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/onix-engine/onix/input"
	"github.com/onix-engine/onix/render"
)

type Context struct {
	textInputSupport
	iconDecoderSupport
	renderErr error
	renderContext
	closeRequestSupport
	onMouseMove    func(x, y float64)
	onTouchEnd     func(x, y float64)
	wakePipe       *wakePipe
	onFilesDropped func(x, y float64, files []string)
	onTouchMove    func(x, y float64)
	nativeWindow   *C.OnixNativeWindow
	onTouchStart   func(x, y float64)
	onKeydown      func(input.KeyCode)
	onKeyup        func(input.KeyCode)
	onMouseDown    func(x, y float64, button input.MouseButton)
	onMouseUp      func(x, y float64, button input.MouseButton)
	onMouseWheel   func(xoff, yoff float64)
	framePacer     framePacer
	loopControl
	decorations DecorationMode
	inputRegion []InputRegionRect
	cursorSupport
	mouseCaptureSupport
	viewport       viewportState
	events         [256]C.OnixEvent
	Height         int
	windowHeight   int
	Width          int
	windowWidth    int
	cornerRadius   int
	roundedCorners bool
	shadow         bool
	resizable      bool
	border         bool
	transparent    bool
	shouldClose    bool
}

// NativeHandle returns the native surface identifier when the host needs to
// embed a platform control. On Wayland this is the wl_surface pointer.
func (c *Context) NativeHandle() uintptr {
	if c == nil || c.nativeWindow == nil {
		return 0
	}
	return uintptr(C.onix_window_native_surface(c.nativeWindow))
}

// NativeX11Window returns the X11 parent window used for embedded native
// controls. It returns zero when the context uses another display backend.
func (c *Context) NativeX11Window() uintptr {
	if c == nil || c.nativeWindow == nil || C.onix_window_backend(c.nativeWindow) != C.int(C.ONIX_BACKEND_X11) {
		return 0
	}
	return uintptr(C.onix_window_native_surface(c.nativeWindow))
}

func NewContext(width, height int, title string, options Options) *Context {
	runtime.LockOSThread()
	options = options.normalized()

	ctitle := C.CString(title)
	defer C.free(unsafe.Pointer(ctitle))

	nativeWindow := C.onix_window_create(
		C.int(width),
		C.int(height),
		ctitle,
		C.int(boolInt(options.Decorations == DecorationNone)),
		C.int(boolInt(webViewBackendRequiresX11 || render.CompiledBackend == render.BackendOpenGL)),
	)
	if nativeWindow == nil {
		panic(errors.New(C.GoString(C.onix_last_error())))
	}

	ctx := &Context{
		Width:        int(C.onix_window_framebuffer_width(nativeWindow)),
		Height:       int(C.onix_window_framebuffer_height(nativeWindow)),
		windowWidth:  int(C.onix_window_window_width(nativeWindow)),
		windowHeight: int(C.onix_window_window_height(nativeWindow)),
		nativeWindow: nativeWindow,
		viewport:     newViewportState(width, height),
		framePacer:   newFramePacer(),
		cornerRadius: options.CornerRadius,
		transparent:  options.Transparent,
		decorations:  options.Decorations,
		border:       options.Border,
		shadow:       options.Shadow,
		resizable:    options.Resizable,
	}
	ctx.closeRequestSupport.close = func() { ctx.shouldClose = true }
	surfaceKind := render.NativeSurfaceXlib
	if C.onix_window_backend(nativeWindow) == C.int(C.ONIX_BACKEND_WAYLAND) {
		surfaceKind = render.NativeSurfaceWayland
	}
	driver, err := newDesktopRenderDriver(render.NativeSurface{
		Kind:        surfaceKind,
		Display:     C.onix_window_native_display(nativeWindow),
		Window:      uintptr(C.onix_window_native_surface(nativeWindow)),
		Width:       ctx.Width,
		Height:      ctx.Height,
		Transparent: options.Transparent,
	})
	if err != nil {
		C.onix_window_destroy(nativeWindow)
		C.free(unsafe.Pointer(nativeWindow))
		panic(err)
	}
	if err := ctx.setRenderDriver(driver); err != nil {
		driver.Destroy()
		C.onix_window_destroy(nativeWindow)
		C.free(unsafe.Pointer(nativeWindow))
		panic(err)
	}
	ctx.applyCursor = func(cursor input.Cursor) { C.onix_window_set_cursor(ctx.nativeWindow, C.int(cursor)) }
	ctx.textInputSupport.applyConfig = ctx.applyTextInputConfig
	ctx.wakePipe, err = ctx.openWakePipe()
	if err != nil {
		ctx.Destroy()
		panic(err)
	}
	ctx.applyMouseCapture = func(captured bool) bool {
		return ctx.nativeWindow != nil && C.onix_window_set_mouse_captured(ctx.nativeWindow, C.int(boolInt(captured))) != 0
	}
	ctx.SetDecorations(options.Decorations)
	ctx.SetBorderVisible(options.Border)
	ctx.SetShadowVisible(options.Shadow)
	ctx.SetResizable(options.Resizable)
	ctx.adjustViewport()
	return ctx
}

func (c *Context) SetDecorations(mode DecorationMode) {
	c.decorations = mode
	C.onix_window_set_decorations(c.nativeWindow, C.int(boolInt(mode == DecorationNone)))
	C.onix_window_set_border_visible(c.nativeWindow, C.int(boolInt(c.border && mode == DecorationNone)))
	c.SetRoundedCorners(mode == DecorationNone && !c.transparent)
}

func (c *Context) SetTitleBarHeight(float64) {}

func (c *Context) SetShadowVisible(visible bool) {
	c.shadow = visible
	C.onix_window_set_shadow_visible(c.nativeWindow, C.int(boolInt(visible)))
}

func (c *Context) SetBorderVisible(visible bool) {
	if c == nil || c.nativeWindow == nil {
		return
	}
	c.border = visible
	C.onix_window_set_border_visible(c.nativeWindow, C.int(boolInt(visible && c.decorations == DecorationNone)))
}

// UsesClientWindowBorder reports whether custom decorations should render the
// border in the surface content for consistent X11 and Wayland appearance.
func (c *Context) UsesClientWindowBorder() bool {
	return c != nil && c.decorations == DecorationNone && c.nativeWindow != nil
}

func (c *Context) SetResizable(enabled bool) {
	c.resizable = enabled
	C.onix_window_set_resizable(c.nativeWindow, C.int(boolInt(enabled)))
}

func (c *Context) SetRoundedCorners(enabled bool) {
	c.roundedCorners = enabled
	c.applyRoundedCorners()
}

func (c *Context) applyRoundedCorners() {
	radius := 0
	if c.roundedCorners && C.onix_window_is_maximized(c.nativeWindow) == 0 && C.onix_window_is_fullscreen(c.nativeWindow) == 0 {
		radius = c.cornerRadius
	}
	c.Device.SetSurfaceCornerRadius(radius)
}

func (c *Context) SetTaskbarVisible(visible bool) {
	flag := C.int(0)
	if visible {
		flag = 1
	}
	C.onix_window_set_taskbar_visible(c.nativeWindow, flag)
}

func (c *Context) SetAlwaysOnTop(enabled bool) {
	flag := C.int(0)
	if enabled {
		flag = 1
	}
	C.onix_window_set_always_on_top(c.nativeWindow, flag)
}

func (c *Context) SetInputRegion(rects []InputRegionRect) {
	if c == nil || inputRegionsEqual(c.inputRegion, rects) {
		return
	}
	c.inputRegion = cloneInputRegion(rects)
	c.applyInputRegion()
}

func (c *Context) applyInputRegion() {
	if c == nil || c.nativeWindow == nil {
		return
	}
	rects := c.viewport.nativeInputRegion(c.inputRegion)
	enabled := C.int(0)
	if c.inputRegion != nil {
		enabled = 1
	}
	var pointer *C.OnixInputRect
	if len(rects) > 0 {
		native := make([]C.OnixInputRect, len(rects))
		for index, rect := range rects {
			native[index] = C.OnixInputRect{
				x:      C.int(rect.X),
				y:      C.int(rect.Y),
				width:  C.int(rect.Width),
				height: C.int(rect.Height),
			}
		}
		pointer = &native[0]
		C.onix_window_set_input_region(c.nativeWindow, enabled, pointer, C.int(len(native)))
		return
	}
	C.onix_window_set_input_region(c.nativeWindow, enabled, pointer, 0)
}

func (c *Context) Minimize()         { C.onix_window_minimize(c.nativeWindow) }
func (c *Context) ToggleMaximize()   { C.onix_window_toggle_maximize(c.nativeWindow) }
func (c *Context) ToggleFullscreen() { C.onix_window_toggle_fullscreen(c.nativeWindow) }
func (c *Context) StartWindowDrag()  { C.onix_window_start_drag(c.nativeWindow) }

func (c *Context) adjustViewport() {
	if !c.viewport.apply(c.Device.Driver(),
		float64(c.Width),
		float64(c.Height),
		float64(c.windowWidth),
		float64(c.windowHeight),
	) {
		return
	}
	if c.inputRegion != nil {
		c.applyInputRegion()
	}
}

// OnFilesDropped registers a callback for files dropped onto the window.
// Coordinates are in logical viewport units. The files slice is only valid
// for the duration of the callback and must be copied when retained.
func (c *Context) OnFilesDropped(f func(x, y float64, files []string)) {
	c.onFilesDropped = chainCallback3(c.onFilesDropped, f)
}

func (c *Context) SetTitle(title string) {
	if c.nativeWindow == nil {
		return
	}
	ctitle := C.CString(title)
	defer C.free(unsafe.Pointer(ctitle))
	C.onix_window_set_title(c.nativeWindow, ctitle)
}

func (c *Context) SetIcon(data []byte) error {
	if c.nativeWindow == nil || len(data) == 0 {
		return nil
	}
	if C.onix_window_backend(c.nativeWindow) != C.int(C.ONIX_BACKEND_X11) {
		return nil
	}

	icon, err := loadIconNRGBA(data, c.decoder)
	if err != nil {
		return err
	}
	pixels := nrgbaToARGB32(icon)
	if len(pixels) == 0 {
		return fmt.Errorf("decode window icon: empty image")
	}

	C.onix_window_set_icon(
		c.nativeWindow,
		C.int(icon.Bounds().Dx()),
		C.int(icon.Bounds().Dy()),
		(*C.uint32_t)(unsafe.Pointer(&pixels[0])),
	)
	return nil
}

func (c *Context) SetVsyncEnabled(enable bool) {
	c.Device.SetVSync(enable)
	if c.nativeWindow == nil {
		return
	}
	flag := 0
	if enable {
		flag = 1
	}
	C.onix_window_set_vsync(c.nativeWindow, C.int(flag))
}

func (c *Context) SetTargetFPS(fps int) {
	c.framePacer.setTargetFPS(fps)
}

func (c *Context) LoopGame(f func() (bool, error)) error {
	for !c.shouldClose {
		c.ActivateRender()
		if wait, idle := c.idleWait(); idle {
			C.onix_window_wait_events(c.nativeWindow, C.int(c.wakePipe.read), C.int(idleTimeoutMillis(wait)))
			c.wakePipe.drain()
			c.framePacer.reset()
		}
		var nativeShouldClose C.int
		count := int(C.onix_window_poll_events_batch(c.nativeWindow, &c.events[0], C.int(len(c.events)), &nativeShouldClose))
		if count != 0 {
			c.RequestFrame()
		}
		for index := 0; index < count; index++ {
			c.handleEvent(&c.events[index])
		}
		if c.renderErr != nil {
			return c.renderErr
		}
		if c.shouldClose || nativeShouldClose != 0 {
			c.shouldClose = true
			break
		}
		c.framePacer.wait()
		rendered, err := f()
		if err != nil {
			return err
		}
		if rendered {
			if err := c.Device.Present(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Context) handleEvent(event *C.OnixEvent) {
	switch int(event.kind) {
	case int(C.ONIX_EVENT_CLOSE):
		if c.acceptCloseRequest() {
			c.shouldClose = true
		} else {
			C.onix_window_cancel_close(c.nativeWindow)
		}
	case int(C.ONIX_EVENT_WINDOW_SIZE):
		c.windowWidth = int(event.arg0)
		c.windowHeight = int(event.arg1)
		c.adjustViewport()
	case int(C.ONIX_EVENT_WINDOW_STATE):
		c.applyRoundedCorners()
	case int(C.ONIX_EVENT_FRAMEBUFFER_SIZE):
		c.Width = int(event.arg0)
		c.Height = int(event.arg1)
		c.renderErr = c.Device.ResizeSurface(c.Width, c.Height)
		c.adjustViewport()
	case int(C.ONIX_EVENT_KEY_DOWN):
		if c.onKeydown != nil && event.arg0 != 0 {
			c.onKeydown(input.KeyCode(event.arg0))
		}
	case int(C.ONIX_EVENT_KEY_UP):
		if c.onKeyup != nil && event.arg0 != 0 {
			c.onKeyup(input.KeyCode(event.arg0))
		}
	case int(C.ONIX_EVENT_TEXT_INPUT):
		c.emitTextInput(C.GoString(&event.text[0]))
	case int(C.ONIX_EVENT_TEXT_COMPOSITION):
		c.emitTextComposition(input.TextComposition{Text: C.GoString(&event.text[0]), Active: event.arg0 != 0})
	case int(C.ONIX_EVENT_TEXT_COMMAND):
		command := input.TextCommand(event.arg0)
		count := int(event.arg1)
		if count <= 0 {
			count = 1
		}
		for range count {
			c.emitTextCommand(command)
		}
	case int(C.ONIX_EVENT_MOUSE_CAPTURE):
		c.updateMouseCapture(event.arg0 != 0)
	case int(C.ONIX_EVENT_MOUSE_DELTA):
		c.addMouseDelta(float64(event.x), float64(event.y))
	case int(C.ONIX_EVENT_MOUSE_MOVE):
		if c.onMouseMove != nil {
			x, y := c.GetLogicalXY(float64(event.x), float64(event.y))
			c.onMouseMove(x, y)
		}
	case int(C.ONIX_EVENT_MOUSE_DOWN):
		if c.onMouseDown != nil && event.arg0 >= 0 {
			x, y := c.GetLogicalXY(float64(event.x), float64(event.y))
			c.onMouseDown(x, y, input.MouseButton(event.arg0))
		}
	case int(C.ONIX_EVENT_MOUSE_UP):
		if c.onMouseUp != nil && event.arg0 >= 0 {
			x, y := c.GetLogicalXY(float64(event.x), float64(event.y))
			c.onMouseUp(x, y, input.MouseButton(event.arg0))
		}
	case int(C.ONIX_EVENT_MOUSE_WHEEL):
		if c.onMouseWheel != nil {
			c.onMouseWheel(float64(event.x), float64(event.y))
		}
	case int(C.ONIX_EVENT_FILES_DROPPED):
		if c.onFilesDropped != nil {
			x, y := c.GetLogicalXY(float64(event.x), float64(event.y))
			if path := normalizeDroppedPath(C.GoString(&event.text[0])); path != "" {
				c.onFilesDropped(x, y, []string{path})
			}
		}
	}
}

func (c *Context) applyTextInputConfig(previous, current input.TextInputConfig) {
	if previous.Active && current.Active && previous.Revision != current.Revision {
		C.onix_window_set_text_input_config(c.nativeWindow, 0, 0, 0)
	}
	active := C.int(0)
	if current.Active {
		active = 1
	}
	multiline := C.int(0)
	if current.Multiline {
		multiline = 1
	}
	C.onix_window_set_text_input_config(c.nativeWindow, active, multiline, C.int(current.Kind))
}

func (c *Context) Destroy() {
	c.updateMouseCapture(false)
	c.closeWakePipe(c.wakePipe)
	c.destroyRenderer()
	if c.nativeWindow != nil {
		C.onix_window_destroy(c.nativeWindow)
		C.free(unsafe.Pointer(c.nativeWindow))
		c.nativeWindow = nil
	}
}
