//go:build windows && cgo

package window

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

const wmInput = 0x00FF

type rawInputDevice struct {
	Page, Usage uint16
	Flags       uint32
	Target      hwnd
}

type rawInputHeader struct {
	Kind, Size     uint32
	Device, WParam uintptr
}

type rawMouseInput struct {
	Header     rawInputHeader
	Flags      uint16
	Padding    uint16
	Buttons    uint32
	RawButtons uint32
	X, Y       int32
	Extra      uint32
}

var (
	registerRawInputDevices func(*rawInputDevice, uint32, uint32) bool
	getRawInputData         func(uintptr, uint32, unsafe.Pointer, *uint32, uint32) uint32
	getForegroundWindow     func() hwnd
	clientToScreen          func(hwnd, *point) bool
	clipCursor              func(*rect) bool
	showCursor              func(bool) int32
)

func init() {
	purego.RegisterLibFunc(&registerRawInputDevices, user32, "RegisterRawInputDevices")
	purego.RegisterLibFunc(&getRawInputData, user32, "GetRawInputData")
	purego.RegisterLibFunc(&getForegroundWindow, user32, "GetForegroundWindow")
	purego.RegisterLibFunc(&clientToScreen, user32, "ClientToScreen")
	purego.RegisterLibFunc(&clipCursor, user32, "ClipCursor")
	purego.RegisterLibFunc(&showCursor, user32, "ShowCursor")
}

func (c *Context) captureMouse(captured bool) bool {
	if c.mouseCaptured == captured {
		return true
	}
	device := rawInputDevice{Page: 1, Usage: 2, Target: c.window}
	if captured {
		if c.window == 0 || getForegroundWindow() != c.window || !c.confineMouse() {
			return false
		}
		if !registerRawInputDevices(&device, 1, uint32(unsafe.Sizeof(device))) {
			clipCursor(nil)
			return false
		}
		setCapture(c.window)
		showCursor(false)
	} else {
		device.Flags, device.Target = 1, 0 // RIDEV_REMOVE requires a null target.
		registerRawInputDevices(&device, 1, uint32(unsafe.Sizeof(device)))
		c.updateMouseCapture(false)
		clipCursor(nil)
		showCursor(true)
		releaseCapture()
	}
	c.updateMouseCapture(captured)
	return true
}

func (c *Context) confineMouse() bool {
	var bounds rect
	var origin point
	if !getClientRect(c.window, &bounds) || !clientToScreen(c.window, &origin) || bounds.Right <= 0 || bounds.Bottom <= 0 {
		return false
	}
	bounds.Left, bounds.Top = origin.X, origin.Y
	bounds.Right += origin.X
	bounds.Bottom += origin.Y
	return clipCursor(&bounds)
}

func (c *Context) handleRawMouse(handle uintptr) {
	if !c.mouseCaptured {
		return
	}
	c.rawMouseSize = uint32(unsafe.Sizeof(c.rawMouse))
	read := getRawInputData(handle, 0x10000003, unsafe.Pointer(&c.rawMouse), &c.rawMouseSize, uint32(unsafe.Sizeof(rawInputHeader{})))
	if read == ^uint32(0) || read < uint32(unsafe.Sizeof(rawInputHeader{}))+24 || c.rawMouse.Header.Kind != 0 || c.rawMouse.Flags&1 != 0 {
		return
	}
	c.addMouseDelta(float64(c.rawMouse.X), float64(c.rawMouse.Y))
}
