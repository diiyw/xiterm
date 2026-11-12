//go:build android || ios

package window

import "C"

import (
	"runtime"
	"time"

	"github.com/onix-engine/onix/input"
	"github.com/onix-engine/onix/render"
)

var context *Context

//export update
func update() {
	Update()
}

//export setViewport
func setViewport(width, height int) {
	context.SetViewport(width, height)
}

//export setSurfaceSize
func setSurfaceSize(width, height int) {
	SetSurfaceSize(width, height)
}

func init() {
	runtime.LockOSThread()
	context = &Context{devicePixelRatio: 1}
}

type Context struct {
	renderContext
	loopControl
	cursorSupport
	mouseCaptureSupport
	textInputSupport
	closeRequestSupport
	iconDecoderSupport
	Width, Height    int
	devicePixelRatio float64
	nativeSurface    uintptr
	viewport         viewportState
	update           func() (bool, error)
	onKeyDown        func(input.KeyCode)
	onKeyUp          func(input.KeyCode)
	onMouseDown      func(x, y float64, button input.MouseButton)
	onMouseUp        func(x, y float64, button input.MouseButton)
	onMouseMove      func(x, y float64)
	onMouseWheel     func(xoff float64, yoff float64)
	onTouchStart     func(x, y float64)
	onTouchMove      func(x, y float64)
	onTouchEnd       func(x, y float64)
	framePacer       framePacer
}

// NativeHandle returns the Android native surface when one is active. iOS
// hosts expose their own view hierarchy and return zero.
func (c *Context) NativeHandle() uintptr {
	if c == nil {
		return 0
	}
	return c.nativeSurface
}

func NewContext(width, height int, _ string, _ Options) *Context {
	context.Width = width
	context.Height = height
	context.devicePixelRatio = normalizeDevicePixelRatio(context.devicePixelRatio)
	context.viewport = newViewportState(width, height)
	context.framePacer = newFramePacer()
	context.adjustViewport()
	return context
}

func (c *Context) Init() {
	c.adjustViewport()
}

func (c *Context) SetSurfaceSize(width, height int) {
	c.Width, c.Height = width, height
	if err := c.Device.ResizeSurface(width, height); err != nil {
		panic(err)
	}
	c.adjustViewport()
}

func (c *Context) SetDevicePixelRatio(ratio float64) {
	c.devicePixelRatio = normalizeDevicePixelRatio(ratio)
	c.adjustViewport()
}

func (c *Context) adjustViewport() {
	c.viewport.apply(c.Device.Driver(),
		float64(c.Width),
		float64(c.Height),
		float64(c.Width)/normalizeDevicePixelRatio(c.devicePixelRatio),
		float64(c.Height)/normalizeDevicePixelRatio(c.devicePixelRatio),
	)
}

func (c *Context) OnKeydown(f func(input.KeyCode)) {
	c.onKeyDown = chainCallback(c.onKeyDown, f)
}

func (c *Context) OnKeyup(f func(input.KeyCode)) {
	c.onKeyUp = chainCallback(c.onKeyUp, f)
}

func (c *Context) OnMouseDown(f func(x, y float64, button input.MouseButton)) {
	c.onMouseDown = chainCallback3(c.onMouseDown, func(x, y float64, button input.MouseButton) {
		x, y = c.GetLogicalXY(x, y)
		f(x, y, button)
	})
}

func (c *Context) OnMouseUp(f func(x, y float64, button input.MouseButton)) {
	c.onMouseUp = chainCallback3(c.onMouseUp, func(x, y float64, button input.MouseButton) {
		x, y = c.GetLogicalXY(x, y)
		f(x, y, button)
	})
}

func (c *Context) OnMouseMove(f func(x, y float64)) {
	c.onMouseMove = chainCallback2(c.onMouseMove, func(x, y float64) {
		x, y = c.GetLogicalXY(x, y)
		f(x, y)
	})
}

func (c *Context) OnMouseWheel(f func(xoff float64, yoff float64)) {
	c.onMouseWheel = chainCallback2(c.onMouseWheel, f)
}

func (c *Context) OnTouchStart(f func(x, y float64)) {
	c.onTouchStart = chainCallback2(c.onTouchStart, func(x, y float64) {
		x, y = c.GetLogicalXY(x, y)
		f(x, y)
	})
}

func (c *Context) OnTouchMove(f func(x, y float64)) {
	c.onTouchMove = chainCallback2(c.onTouchMove, func(x, y float64) {
		x, y = c.GetLogicalXY(x, y)
		f(x, y)
	})
}

func (c *Context) OnTouchEnd(f func(x, y float64)) {
	c.onTouchEnd = chainCallback2(c.onTouchEnd, func(x, y float64) {
		x, y = c.GetLogicalXY(x, y)
		f(x, y)
	})
}

func (c *Context) OnFilesDropped(func(x, y float64, files []string)) {}

func (c *Context) LoopGame(f func() (bool, error)) error {
	c.update = f
	return nil
}

func (c *Context) SetTitle(title string) {}

func (c *Context) SetIcon(data []byte) error { return nil }

func (c *Context) SetVsyncEnabled(enable bool) { c.Device.SetVSync(enable) }

func (c *Context) SetTargetFPS(fps int) {
	c.framePacer.setTargetFPS(fps)
}

func (c *Context) Destroy() {
	c.destroyRenderer()
}

func Update() {
	if context == nil || context.update == nil {
		return
	}
	if !context.framePacer.ready(time.Now()) {
		return
	}
	context.ActivateRender()
	rendered, err := context.update()
	if err != nil {
		panic(err)
	}
	if rendered {
		if err := context.Device.Present(); err != nil {
			panic(err)
		}
	}
}

func SetNativeSurface(surface uintptr, width, height int) error {
	if surface == 0 || width <= 0 || height <= 0 {
		return nil
	}
	driver, err := render.NewAndroidDriver(render.NativeSurface{
		Kind:   render.NativeSurfaceAndroid,
		Window: surface,
		Width:  width,
		Height: height,
	})
	if err != nil {
		return err
	}
	if err := context.setRenderDriver(driver); err != nil {
		driver.Destroy()
		return err
	}
	context.Width, context.Height = width, height
	context.nativeSurface = surface
	context.adjustViewport()
	return nil
}

func ClearNativeSurface() {
	if context != nil {
		context.destroyRenderer()
		context.nativeSurface = 0
	}
}

func SetSurfaceSize(width, height int) {
	if context == nil {
		return
	}
	context.SetSurfaceSize(width, height)
}

func SetDevicePixelRatio(ratio float64) {
	if context == nil {
		return
	}
	context.SetDevicePixelRatio(ratio)
}

func DispatchKeyDown(key input.KeyCode) {
	if context == nil || context.onKeyDown == nil {
		return
	}
	context.onKeyDown(key)
}

func DispatchKeyUp(key input.KeyCode) {
	if context == nil || context.onKeyUp == nil {
		return
	}
	context.onKeyUp(key)
}

func DispatchTextInput(session uint64, text string) {
	if !activeTextInputSession(session) {
		return
	}
	context.emitTextInput(text)
}

func DispatchTextComposition(session uint64, event input.TextComposition) {
	if !activeTextInputSession(session) {
		return
	}
	context.emitTextComposition(event)
}

func DispatchTextCommand(session uint64, command input.TextCommand) {
	if !activeTextInputSession(session) {
		return
	}
	context.emitTextCommand(command)
}

func activeTextInputSession(session uint64) bool {
	return context != nil && context.acceptsTextInputSession(session)
}

func TextInputConfigValue() uint64 {
	if context == nil {
		return 0
	}
	return context.TextInputConfig().Value()
}

func TextInputInitialValue() string {
	if context == nil {
		return ""
	}
	return context.TextInputConfig().InitialValue
}

func TextInputSelectionStart() int {
	if context == nil {
		return 0
	}
	return context.TextInputConfig().SelectionStart
}

func TextInputSelectionEnd() int {
	if context == nil {
		return 0
	}
	return context.TextInputConfig().SelectionEnd
}

func ApplyTextSelection(session uint64, start, end int) {
	if context != nil {
		context.applyTextSelection(session, start, end)
	}
}

func DispatchMouseDown(x, y float64, button input.MouseButton) {
	if context == nil || context.onMouseDown == nil {
		return
	}
	context.onMouseDown(x, y, button)
}

func DispatchMouseUp(x, y float64, button input.MouseButton) {
	if context == nil || context.onMouseUp == nil {
		return
	}
	context.onMouseUp(x, y, button)
}

func DispatchMouseMove(x, y float64) {
	if context == nil || context.onMouseMove == nil {
		return
	}
	context.onMouseMove(x, y)
}

func DispatchMouseWheel(xoff, yoff float64) {
	if context == nil || context.onMouseWheel == nil {
		return
	}
	context.onMouseWheel(xoff, yoff)
}

func DispatchTouchStart(x, y float64) {
	if context == nil || context.onTouchStart == nil {
		return
	}
	context.onTouchStart(x, y)
}

func DispatchTouchMove(x, y float64) {
	if context == nil || context.onTouchMove == nil {
		return
	}
	context.onTouchMove(x, y)
}

func DispatchTouchEnd(x, y float64) {
	if context == nil || context.onTouchEnd == nil {
		return
	}
	context.onTouchEnd(x, y)
}
