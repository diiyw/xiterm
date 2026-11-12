//go:build darwin && cgo && !ios

package window

/*
#cgo LDFLAGS: -framework AppKit -framework Foundation -framework QuartzCore -framework CoreGraphics -framework UniformTypeIdentifiers
#cgo onix_opengl CFLAGS: -DONIX_MAC_OPENGL
#include <stdlib.h>
#include "macos_native.h"

static int onix_macos_poll_events_batch(OnixMacWindow *window, OnixMacEvent *events, int capacity, int *should_close) {
	onix_macos_poll(window);
	int count = 0;
	while (count < capacity && onix_macos_pop_event(window, &events[count]) != 0) {
		count++;
	}
	*should_close = onix_macos_should_close(window);
	return count;
}
*/
import "C"

import (
	"errors"
	"runtime"
	"unsafe"

	"github.com/onix-engine/onix/input"
)

func init() {
	runtime.LockOSThread()
}

// cgo checks the entire allocation behind an event pointer. Keep the batch
// separate from Context, whose callbacks and resources contain Go pointers.
type macEventBuffer [256]C.OnixMacEvent

type Context struct {
	renderContext
	closeRequestSupport
	iconDecoderSupport
	renderErr      error
	nativeWindow   *C.OnixMacWindow
	mouseDownEvent *C.OnixMacEvent
	events         *macEventBuffer
	onKeydown      func(input.KeyCode)
	onKeyup        func(input.KeyCode)
	onMouseDown    func(x, y float64, button input.MouseButton)
	onMouseUp      func(x, y float64, button input.MouseButton)
	onMouseMove    func(x, y float64)
	onMouseWheel   func(xoff, yoff float64)
	onTouchStart   func(x, y float64)
	onTouchMove    func(x, y float64)
	onTouchEnd     func(x, y float64)
	onFilesDropped func(x, y float64, files []string)
	decorations    DecorationMode
	loopControl
	inputRegion []InputRegionRect
	framePacer  framePacer
	cursorSupport
	mouseCaptureSupport
	textInputSupport
	viewport          viewportState
	Width             int
	Height            int
	devicePixelRatio  float64
	nativeShouldClose C.int
	shouldClose       bool
	shadow            bool
	resizable         bool
}

// NativeHandle returns the host NSWindow for embedding platform controls.
func (c *Context) NativeHandle() uintptr {
	if c == nil || c.nativeWindow == nil {
		return 0
	}
	return uintptr(C.onix_macos_native_window(c.nativeWindow))
}

func NewContext(width, height int, title string, options Options) *Context {
	runtime.LockOSThread()
	options = options.normalized()
	ctitle := C.CString(title)
	defer C.free(unsafe.Pointer(ctitle))
	nativeWindow := C.onix_macos_create(
		C.int(width),
		C.int(height),
		ctitle,
		C.int(boolInt(options.Transparent)),
		C.int(boolInt(options.Decorations == DecorationNone)),
		C.double(options.TitleBarHeight),
		C.int(boolInt(options.Shadow)),
		C.int(boolInt(options.Resizable)),
	)
	if nativeWindow == nil {
		return panicContext(errors.New(C.GoString(C.onix_macos_last_error())))
	}
	context := &Context{
		Width:            int(C.onix_macos_width(nativeWindow)),
		Height:           int(C.onix_macos_height(nativeWindow)),
		devicePixelRatio: float64(C.onix_macos_scale(nativeWindow)),
		nativeWindow:     nativeWindow,
		viewport:         newViewportState(width, height),
		framePacer:       newFramePacer(),
		events:           new(macEventBuffer),
		decorations:      options.Decorations,
		shadow:           options.Shadow,
		resizable:        options.Resizable,
	}
	context.applyCursor = func(cursor input.Cursor) { C.onix_macos_set_cursor(context.nativeWindow, C.int(cursor)) }
	context.textInputSupport.applyConfig = context.applyTextInputConfig
	context.applyMouseCapture = func(captured bool) bool {
		return context.nativeWindow != nil && C.onix_macos_set_mouse_captured(context.nativeWindow, C.int(boolInt(captured))) != 0
	}
	context.closeRequestSupport.close = func() { context.shouldClose = true }
	driver, err := newMacRenderDriver(uintptr(C.onix_macos_layer(nativeWindow)), uintptr(C.onix_macos_view(nativeWindow)), context.Width, context.Height)
	if err != nil {
		C.onix_macos_destroy(nativeWindow)
		return panicContext(err)
	}
	if err := context.setRenderDriver(driver); err != nil {
		driver.Destroy()
		C.onix_macos_destroy(nativeWindow)
		return panicContext(err)
	}
	context.adjustViewport()
	context.wake = func() { C.onix_macos_wake() }
	return context
}

func panicContext(err error) *Context {
	panic(err)
}

func (c *Context) SetDecorations(mode DecorationMode) {
	c.decorations = mode
	C.onix_macos_set_decorations(c.nativeWindow, C.int(boolInt(mode == DecorationNone)))
}

func (c *Context) SetTitleBarHeight(height float64) {
	if c == nil || c.nativeWindow == nil {
		return
	}
	C.onix_macos_set_title_bar_height(c.nativeWindow, C.double(max(height, 0)))
}

func (c *Context) SetShadowVisible(visible bool) {
	c.shadow = visible
	C.onix_macos_set_shadow_visible(c.nativeWindow, C.int(boolInt(visible)))
}

func (c *Context) SetBorderVisible(bool) {}

func (c *Context) UsesClientWindowBorder() bool {
	return c != nil && c.decorations == DecorationNone
}

func (c *Context) SetResizable(enabled bool) {
	c.resizable = enabled
	C.onix_macos_set_resizable(c.nativeWindow, C.int(boolInt(enabled)))
}

func (c *Context) SetTaskbarVisible(visible bool) {
	flag := C.int(0)
	if visible {
		flag = 1
	}
	C.onix_macos_set_taskbar_visible(c.nativeWindow, flag)
}

func (c *Context) SetAlwaysOnTop(enabled bool) {
	flag := C.int(0)
	if enabled {
		flag = 1
	}
	C.onix_macos_set_always_on_top(c.nativeWindow, flag)
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
	if len(rects) == 0 {
		C.onix_macos_set_input_region(c.nativeWindow, enabled, nil, 0)
		return
	}
	native := make([]C.OnixMacInputRect, len(rects))
	for index, rect := range rects {
		native[index] = C.OnixMacInputRect{
			x:      C.int(rect.X),
			y:      C.int(rect.Y),
			width:  C.int(rect.Width),
			height: C.int(rect.Height),
		}
	}
	C.onix_macos_set_input_region(c.nativeWindow, enabled, &native[0], C.int(len(native)))
}

func (c *Context) Minimize()         { C.onix_macos_minimize(c.nativeWindow) }
func (c *Context) ToggleMaximize()   { C.onix_macos_toggle_maximize(c.nativeWindow) }
func (c *Context) ToggleFullscreen() { C.onix_macos_toggle_fullscreen(c.nativeWindow) }
func (c *Context) StartWindowDrag()  { C.onix_macos_start_drag(c.nativeWindow, c.mouseDownEvent) }

func (c *Context) adjustViewport() {
	if !c.viewport.apply(c.Device.Driver(),
		float64(c.Width),
		float64(c.Height),
		float64(c.Width)/normalizeDevicePixelRatio(c.devicePixelRatio),
		float64(c.Height)/normalizeDevicePixelRatio(c.devicePixelRatio),
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
func (c *Context) OnFilesDropped(callback func(float64, float64, []string)) {
	c.onFilesDropped = chainCallback3(c.onFilesDropped, callback)
}

func (c *Context) SetTitle(title string) {
	ctitle := C.CString(title)
	defer C.free(unsafe.Pointer(ctitle))
	C.onix_macos_set_title(c.nativeWindow, ctitle)
}

func (c *Context) SetClipboardText(text string) bool {
	value := C.CString(text)
	defer C.free(unsafe.Pointer(value))
	return C.onix_macos_set_clipboard_text(value) != 0
}

func (c *Context) ClipboardText() (string, bool) {
	length := int(C.onix_macos_get_clipboard_text(nil, 0))
	if length < 0 {
		return "", false
	}
	buffer := make([]byte, length+1)
	C.onix_macos_get_clipboard_text((*C.char)(unsafe.Pointer(&buffer[0])), C.int(len(buffer)))
	return string(buffer[:length]), true
}

func (c *Context) SetIcon(data []byte) error    { return nil }
func (c *Context) SetVsyncEnabled(enabled bool) { c.Device.SetVSync(enabled) }
func (c *Context) SetTargetFPS(fps int)         { c.framePacer.setTargetFPS(fps) }
func (c *Context) LoopGame(frame func() (bool, error)) error {
	for !c.shouldClose {
		c.ActivateRender()
		if wait, idle := c.idleWait(); idle {
			C.onix_macos_wait(c.nativeWindow, C.int(idleTimeoutMillis(wait)))
			c.framePacer.reset()
		}
		// AppKit can dispatch WebKit callbacks while polling, so keep callback
		// support and reuse the output field instead of passing a stack local.
		count := int(C.onix_macos_poll_events_batch(c.nativeWindow, &c.events[0], C.int(len(c.events)), &c.nativeShouldClose))
		if count != 0 {
			// Includes redraw notifications from focus, exposure and Dock reopen.
			c.RequestFrame()
		}
		for index := 0; index < count; index++ {
			c.handleEvent(&c.events[index])
		}
		if c.renderErr != nil {
			return c.renderErr
		}
		if c.shouldClose || c.nativeShouldClose != 0 {
			break
		}
		c.framePacer.wait()
		rendered, err := frame()
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

func (c *Context) handleEvent(event *C.OnixMacEvent) {
	switch int(event.kind) {
	case int(C.ONIX_MAC_EVENT_REDRAW):
		// Restoring a window should not wait for the next paced frame.
		c.framePacer.reset()
	case int(C.ONIX_MAC_EVENT_CLOSE):
		if c.acceptCloseRequest() {
			c.shouldClose = true
		} else {
			C.onix_macos_cancel_close(c.nativeWindow)
			// poll_events_batch captured this flag before dispatching events.
			// Clear that snapshot too, or LoopGame exits despite the veto.
			c.nativeShouldClose = 0
			c.RequestFrame()
		}
	case int(C.ONIX_MAC_EVENT_RESIZE):
		c.Width, c.Height = int(event.x), int(event.y)
		c.devicePixelRatio = float64(C.onix_macos_scale(c.nativeWindow))
		c.renderErr = c.Device.ResizeSurface(c.Width, c.Height)
		c.adjustViewport()
	case int(C.ONIX_MAC_EVENT_KEY_DOWN):
		if c.onKeydown != nil {
			c.onKeydown(input.KeyCode(event.arg0))
		}
	case int(C.ONIX_MAC_EVENT_KEY_UP):
		if c.onKeyup != nil {
			c.onKeyup(input.KeyCode(event.arg0))
		}
	case int(C.ONIX_MAC_EVENT_TEXT):
		c.emitTextInput(C.GoString(&event.text[0]))
	case int(C.ONIX_MAC_EVENT_TEXT_COMPOSITION):
		c.emitTextComposition(input.TextComposition{Text: C.GoString(&event.text[0]), Active: event.arg0 != 0})
	case int(C.ONIX_MAC_EVENT_TEXT_COMMAND):
		c.emitTextCommand(input.TextCommand(event.arg0))
	case int(C.ONIX_MAC_EVENT_MOUSE_DOWN):
		if c.onMouseDown != nil {
			x, y := c.GetLogicalXY(float64(event.x), float64(event.y))
			c.mouseDownEvent = event
			c.onMouseDown(x, y, input.MouseButton(event.arg0))
			c.mouseDownEvent = nil
		}
		C.onix_macos_release_event(event)
	case int(C.ONIX_MAC_EVENT_MOUSE_UP):
		if c.onMouseUp != nil {
			x, y := c.GetLogicalXY(float64(event.x), float64(event.y))
			c.onMouseUp(x, y, input.MouseButton(event.arg0))
		}
	case int(C.ONIX_MAC_EVENT_MOUSE_CAPTURE):
		c.updateMouseCapture(event.arg0 != 0)
	case int(C.ONIX_MAC_EVENT_MOUSE_DELTA):
		c.addMouseDelta(float64(event.x), float64(event.y))
	case int(C.ONIX_MAC_EVENT_MOUSE_MOVE):
		if c.onMouseMove != nil {
			x, y := c.GetLogicalXY(float64(event.x), float64(event.y))
			c.onMouseMove(x, y)
		}
	case int(C.ONIX_MAC_EVENT_MOUSE_WHEEL):
		if c.onMouseWheel != nil {
			c.onMouseWheel(float64(event.x), float64(event.y))
		}
	case int(C.ONIX_MAC_EVENT_FILES_DROPPED):
		if c.onFilesDropped != nil {
			x, y := c.GetLogicalXY(float64(event.x), float64(event.y))
			if path := normalizeDroppedPath(C.GoString(&event.text[0])); path != "" {
				c.onFilesDropped(x, y, []string{path})
			}
		}
	}
}

func (c *Context) applyTextInputConfig(_, current input.TextInputConfig) {
	active := C.int(0)
	if current.Active {
		active = 1
	}
	caret, _ := c.viewport.InputRect(
		float64(current.Caret.X),
		float64(current.Caret.Y),
		float64(max(current.Caret.Width, 1)),
		float64(current.Caret.Height),
	)
	C.onix_macos_set_text_input_config(
		c.nativeWindow,
		C.uint64_t(current.Value()),
		active,
		C.int(current.SelectionStart),
		C.int(current.SelectionEnd),
		C.double(caret.X),
		C.double(caret.Y),
		C.double(caret.Width),
		C.double(caret.Height),
	)
}

func (c *Context) Destroy() {
	c.wakeMutex.Lock()
	c.wake = nil
	c.wakeMutex.Unlock()
	c.updateMouseCapture(false)
	c.destroyRenderer()
	if c.nativeWindow != nil {
		C.onix_macos_destroy(c.nativeWindow)
		c.nativeWindow = nil
	}
}
