//go:build linux && cgo && onix_drm

package window

/*
#cgo LDFLAGS: -ldl
#include <stdlib.h>
#include "drm_input.h"
#include "drm_native.h"

static int onix_drm_input_poll_batch(OnixDRMInput *input, OnixDRMInputEvent *events, int capacity) {
	onix_drm_input_poll(input);
	int count = 0;
	while (count < capacity && onix_drm_input_pop(input, &events[count]) != 0) {
		count++;
	}
	return count;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"github.com/onix-engine/onix/input"
)

type Context struct {
	renderContext
	loopControl
	cursorSupport
	mouseCaptureSupport
	textInputSupport
	closeRequestSupport
	iconDecoderSupport
	directSignalSupport
	Width        int
	Height       int
	shouldClose  bool
	viewport     viewportState
	framePacer   framePacer
	display      *C.OnixDRMDisplay
	nativeInput  *C.OnixDRMInput
	events       [256]C.OnixDRMInputEvent
	wakePipe     *wakePipe
	onKeydown    func(input.KeyCode)
	onKeyup      func(input.KeyCode)
	onMouseDown  func(float64, float64, input.MouseButton)
	onMouseUp    func(float64, float64, input.MouseButton)
	onMouseMove  func(float64, float64)
	onMouseWheel func(float64, float64)
	onTouchStart func(float64, float64)
	onTouchMove  func(float64, float64)
	onTouchEnd   func(float64, float64)
}

func NewContext(width, height int, _ string, options Options) *Context {
	runtime.LockOSThread()
	if width <= 0 || height <= 0 {
		panic(errors.New("window: invalid DRM viewport dimensions"))
	}
	display, err := openDRMDisplay(options.DRMDevice, options.DRMConnectorID)
	if err != nil {
		panic(err)
	}
	surfaceWidth, surfaceHeight := int(C.onix_drm_width(display)), int(C.onix_drm_height(display))
	driver, err := newDRMDriver(display, surfaceWidth, surfaceHeight)
	if err != nil {
		C.onix_drm_destroy(display)
		panic(fmt.Errorf("window: initialize DRM renderer: %w", err))
	}
	context := &Context{
		Width:       surfaceWidth,
		Height:      surfaceHeight,
		viewport:    newViewportState(width, height),
		framePacer:  newFramePacer(),
		display:     display,
		nativeInput: C.onix_drm_input_create(C.int(surfaceWidth), C.int(surfaceHeight)),
	}
	if err := context.setRenderDriver(driver); err != nil {
		driver.Destroy()
		C.onix_drm_destroy(display)
		panic(err)
	}
	context.closeRequestSupport.close = func() { context.shouldClose = true }
	context.wakePipe, err = context.openWakePipe()
	if err != nil {
		context.Destroy()
		panic(err)
	}
	context.directSignalSupport = newDirectSignalSupport(context.Wake)
	context.adjustViewport()
	return context
}

func openDRMDisplay(configured string, connectorID uint32) (*C.OnixDRMDisplay, error) {
	if configured != "" {
		return createDRMDisplay(configured, connectorID)
	}
	var lastErr error
	for index := range 16 {
		path := fmt.Sprintf("/dev/dri/card%d", index)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		display, err := createDRMDisplay(path, connectorID)
		if err == nil {
			return display, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("window: no DRM primary device found in /dev/dri")
}

func createDRMDisplay(path string, connectorID uint32) (*C.OnixDRMDisplay, error) {
	cpath := C.CString(path)
	display := C.onix_drm_create(cpath, C.uint32_t(connectorID))
	C.free(unsafe.Pointer(cpath))
	if display == nil {
		return nil, fmt.Errorf("%w (%s)", drmError("initialize display"), path)
	}
	return display, nil
}

func drmError(operation string) error {
	message := C.GoString(C.onix_drm_last_error())
	if message == "" {
		message = "unknown DRM error"
	}
	return fmt.Errorf("window: DRM %s: %s", operation, message)
}

func (c *Context) NativeHandle() uintptr {
	if c == nil || c.display == nil {
		return 0
	}
	return drmNativeHandle(c.display)
}

func (c *Context) NativeX11Window() uintptr { return 0 }

func (c *Context) adjustViewport() {
	c.viewport.apply(c.Device.Driver(), float64(c.Width), float64(c.Height), float64(c.Width), float64(c.Height))
}

func (c *Context) OnFilesDropped(func(float64, float64, []string)) {}
func (c *Context) SetTitle(string)                                 {}
func (c *Context) SetIcon([]byte) error                            { return nil }
func (c *Context) SetClipboardText(string) bool                    { return false }
func (c *Context) ClipboardText() (string, bool)                   { return "", false }

func (c *Context) SetVsyncEnabled(enabled bool) { c.Device.SetVSync(enabled) }

func (c *Context) SetTargetFPS(fps int) { c.framePacer.setTargetFPS(fps) }

func (c *Context) DirectInputCapabilities() (keyboard, pointer bool) {
	if c == nil || c.nativeInput == nil {
		return false, false
	}
	return C.onix_drm_input_has_keyboard(c.nativeInput) != 0, C.onix_drm_input_has_pointer(c.nativeInput) != 0
}

func (c *Context) LoopGame(frame func() (bool, error)) error {
	for !c.shouldClose {
		c.ActivateRender()
		if wait, idle := c.idleWait(); idle {
			C.onix_drm_input_wait(c.nativeInput, C.int(c.wakePipe.read), C.int(idleTimeoutMillis(wait)))
			c.wakePipe.drain()
			c.framePacer.reset()
		}
		if c.directSignalSupport.requested() && c.acceptCloseRequest() {
			c.shouldClose = true
			continue
		}
		count := int(C.onix_drm_input_poll_batch(c.nativeInput, &c.events[0], C.int(len(c.events))))
		if count != 0 {
			c.RequestFrame()
		}
		for index := 0; index < count; index++ {
			c.handleInputEvent(c.events[index])
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

func (c *Context) handleInputEvent(event C.OnixDRMInputEvent) {
	x, y := c.GetLogicalXY(float64(event.x), float64(event.y))
	switch event.kind {
	case C.ONIX_DRM_INPUT_KEY_DOWN:
		if c.onKeydown != nil {
			c.onKeydown(input.KeyCode(event.code))
		}
	case C.ONIX_DRM_INPUT_KEY_UP:
		if c.onKeyup != nil {
			c.onKeyup(input.KeyCode(event.code))
		}
	case C.ONIX_DRM_INPUT_MOUSE_MOVE:
		if c.onMouseMove != nil {
			c.onMouseMove(x, y)
		}
	case C.ONIX_DRM_INPUT_MOUSE_DOWN:
		if c.onMouseDown != nil {
			c.onMouseDown(x, y, input.MouseButton(event.code))
		}
	case C.ONIX_DRM_INPUT_MOUSE_UP:
		if c.onMouseUp != nil {
			c.onMouseUp(x, y, input.MouseButton(event.code))
		}
	case C.ONIX_DRM_INPUT_MOUSE_WHEEL:
		if c.onMouseWheel != nil {
			c.onMouseWheel(float64(event.x), float64(event.y))
		}
	case C.ONIX_DRM_INPUT_TOUCH_START:
		if c.onTouchStart != nil {
			c.onTouchStart(x, y)
		}
	case C.ONIX_DRM_INPUT_TOUCH_MOVE:
		if c.onTouchMove != nil {
			c.onTouchMove(x, y)
		}
	case C.ONIX_DRM_INPUT_TOUCH_END:
		if c.onTouchEnd != nil {
			c.onTouchEnd(x, y)
		}
	}
}

func (c *Context) Destroy() {
	c.closeWakePipe(c.wakePipe)
	if c == nil {
		return
	}
	c.directSignalSupport.destroy()
	if c.nativeInput != nil {
		C.onix_drm_input_destroy(c.nativeInput)
		c.nativeInput = nil
	}
	c.destroyRenderer()
	if c.display != nil {
		C.onix_drm_destroy(c.display)
		c.display = nil
	}
}
