//go:build windows && cgo

package window

import (
	"errors"
	"fmt"
	"image"
	"math"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/onix-engine/onix/input"
	"github.com/onix-engine/onix/render"
)

const (
	cwUseDefault          = ^0x7fffffff
	swShow                = 5
	pmRemove              = 0x0001
	wsOverlappedWindow    = 0x00CF0000
	swpNoZOrder           = 0x0004
	swpNoActivate         = 0x0010
	smCXScreen            = 0
	smCYScreen            = 1
	monitorDefaultNearest = 2

	wmClose               = 0x0010
	wmDestroy             = 0x0002
	wmKillFocus           = 0x0008
	wmQuit                = 0x0012
	wmGetMinMaxInfo       = 0x0024
	wmCancelMode          = 0x001F
	wmNCCalcSize          = 0x0083
	wmNCHitTest           = 0x0084
	wmSetIcon             = 0x0080
	wmSize                = 0x0005
	wmDpiChanged          = 0x02E0
	wmKeyDown             = 0x0100
	wmKeyUp               = 0x0101
	wmChar                = 0x0102
	wmSysKeyDown          = 0x0104
	wmSysKeyUp            = 0x0105
	wmSysChar             = 0x0106
	wmUnichar             = 0x0109
	wmImeStartComposition = 0x010D
	wmImeEndComposition   = 0x010E
	wmImeComposition      = 0x010F
	wmImeSetContext       = 0x0281
	wmMouseMove           = 0x0200
	wmMouseActivate       = 0x0021
	wmLButtonDown         = 0x0201
	wmLButtonUp           = 0x0202
	wmRButtonDown         = 0x0204
	wmRButtonUp           = 0x0205
	wmMButtonDown         = 0x0207
	wmMButtonUp           = 0x0208
	wmMouseWheel          = 0x020A
	wmCaptureChanged      = 0x0215
	wmDropFiles           = 0x0233
	sizeMinimized         = 1
	maActivate            = 1

	vkShift      = 0x10
	vkControl    = 0x11
	vkMenu       = 0x12
	vkLeft       = 0x25
	vkUp         = 0x26
	vkRight      = 0x27
	vkDown       = 0x28
	vkEscape     = 0x1B
	vkReturn     = 0x0D
	vkTab        = 0x09
	vkBack       = 0x08
	vkInsert     = 0x2D
	vkDelete     = 0x2E
	vkHome       = 0x24
	vkEnd        = 0x23
	vkPrior      = 0x21
	vkNext       = 0x22
	vkCaps       = 0x14
	vkScroll     = 0x91
	vkNumLock    = 0x90
	vkPrint      = 0x2C
	vkPause      = 0x13
	vkProcessKey = 0xE5
	vkLShift     = 0xA0
	vkRShift     = 0xA1
	vkLControl   = 0xA2
	vkRControl   = 0xA3
	vkLMenu      = 0xA4
	vkRMenu      = 0xA5
	vkLWin       = 0x5B
	vkRWin       = 0x5C
	vkApps       = 0x5D

	iconSmall = 0
	iconBig   = 1

	biRGB           = 0
	defaultDPI      = 96
	logicalPixelsX  = 88
	dpiPerMonitorV2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2

	gcsCompStr   = 0x0008
	gcsResultStr = 0x0800

	iscShowUICompositionWindow = 0x80000000
	unicodeNoChar              = 0xffff
	niCompositionString        = 0x0015
	cpsCancel                  = 0x0004
	cfsExclude                 = 0x0080
	cfUnicodeText              = 13
	gmemMoveable               = 0x0002
)

type (
	atom     uint16
	handle   uintptr
	hwnd     handle
	hcursor  handle
	hicon    handle
	hbitmap  handle
	hbrush   handle
	hmenu    handle
	hinst    handle
	hmonitor handle
)

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   hinst
	Icon       hicon
	Cursor     hcursor
	Background hbrush
	MenuName   *uint16
	ClassName  *uint16
	IconSm     hicon
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type rgbQuad struct {
	Blue     byte
	Green    byte
	Red      byte
	Reserved byte
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]rgbQuad
}

type iconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  hbitmap
	HbmColor hbitmap
}

type point struct {
	X, Y int32
}

type minMaxInfo struct {
	Reserved     point
	MaxSize      point
	MaxPosition  point
	MinTrackSize point
	MaxTrackSize point
}

type candidateForm struct {
	Index        uint32
	Style        uint32
	CurrentPos   point
	ExcludedArea rect
}

type msg struct {
	Hwnd    hwnd
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type rect struct {
	Left, Top, Right, Bottom int32
}

type monitorInfo struct {
	Size    uint32
	Monitor rect
	Work    rect
	Flags   uint32
}

var (
	user32DLL = syscall.NewLazyDLL("user32.dll")

	kernel32 = syscall.NewLazyDLL("kernel32.dll").Handle()
	user32   = user32DLL.Handle()
	gdi32    = syscall.NewLazyDLL("gdi32.dll").Handle()
	imm32    = syscall.NewLazyDLL("imm32.dll").Handle()
	shell32  = syscall.NewLazyDLL("shell32.dll").Handle()

	// The message pump bypasses the registered wrappers below: a pumped event
	// otherwise pays six allocations, and the queue drains every frame.
	peekMessageProc      = user32DLL.NewProc("PeekMessageW")
	translateMessageProc = user32DLL.NewProc("TranslateMessage")
	dispatchMessageProc  = user32DLL.NewProc("DispatchMessageW")

	getModuleHandleW            func(moduleName *uint16) hinst
	registerClassExW            func(class *wndClassEx) atom
	createWindowExW             func(exStyle uint32, className, windowName *uint16, style uint32, x, y, width, height int32, parent hwnd, menu hmenu, instance hinst, param unsafe.Pointer) hwnd
	adjustWindowRect            func(r *rect, style uint32, hasMenu bool) bool
	showWindow                  func(window hwnd, cmdShow int32) bool
	defWindowProcW              func(window hwnd, msg uint32, wParam, lParam uintptr) uintptr
	destroyWindow               func(window hwnd) bool
	waitMessage                 func() bool
	postMessageW                func(uintptr, uint32, uintptr, uintptr) bool
	msgWaitForMultipleObjectsEx func(uint32, uintptr, uint32, uint32, uint32) uint32
	getClientRect               func(window hwnd, r *rect) bool
	setWindowTextW              func(window hwnd, text *uint16) bool
	sendMessageW                func(window hwnd, msg uint32, wParam, lParam uintptr) uintptr
	setWindowPos                func(window hwnd, insertAfter hwnd, x, y, width, height int32, flags uint32) bool
	getSystemMetrics            func(index int32) int32
	setCapture                  func(window hwnd) hwnd
	getCursorPos                func(point *point) bool
	monitorFromPoint            func(point uint64, flags uint32) hmonitor
	monitorFromWindow           func(window hwnd, flags uint32) hmonitor
	getMonitorInfoW             func(monitor hmonitor, info *monitorInfo) bool
	loadCursorW                 func(instance hinst, cursorName uintptr) hcursor
	immGetContext               func(window hwnd) uintptr
	immGetCompositionStringW    func(imc uintptr, index uint32, buf unsafe.Pointer, length uint32) int32
	immNotifyIME                func(imc uintptr, action, index, value uint32) bool
	immSetCandidateWindow       func(imc uintptr, form *candidateForm) bool
	immReleaseContext           func(window hwnd, imc uintptr) bool
	getDC                       func(window hwnd) uintptr
	releaseDC                   func(window hwnd, dc uintptr) int32
	getDeviceCaps               func(dc uintptr, index int32) int32
	createBitmap                func(width, height int32, planes, bitCount uint32, bits unsafe.Pointer) hbitmap
	createDIBSection            func(dc uintptr, info *bitmapInfo, usage uint32, bits *unsafe.Pointer, section uintptr, offset uint32) hbitmap
	deleteObject                func(object handle) bool
	createIconIndirect          func(info *iconInfo) hicon
	destroyIcon                 func(icon hicon) bool
	openClipboard               func(window hwnd) bool
	closeClipboard              func() bool
	emptyClipboard              func() bool
	setClipboardData            func(format uint32, memory handle) handle
	getClipboardData            func(format uint32) handle
	globalAlloc                 func(flags uint32, bytes uintptr) handle
	globalLock                  func(memory handle) unsafe.Pointer
	globalUnlock                func(memory handle) bool
	globalSize                  func(memory handle) uintptr
	globalFree                  func(memory handle) handle
	dragAcceptFiles             func(window hwnd, accept bool)
	dragQueryFileW              func(drop uintptr, index uint32, path *uint16, capacity uint32) uint32
	dragQueryPoint              func(drop uintptr, point *point) bool
	dragFinish                  func(drop uintptr)

	creatingContext *Context
	windowContexts  = make(map[hwnd]*Context)
)

func init() {
	purego.RegisterLibFunc(&getModuleHandleW, kernel32, "GetModuleHandleW")
	purego.RegisterLibFunc(&registerClassExW, user32, "RegisterClassExW")
	purego.RegisterLibFunc(&createWindowExW, user32, "CreateWindowExW")
	purego.RegisterLibFunc(&adjustWindowRect, user32, "AdjustWindowRect")
	purego.RegisterLibFunc(&showWindow, user32, "ShowWindow")
	purego.RegisterLibFunc(&defWindowProcW, user32, "DefWindowProcW")
	purego.RegisterLibFunc(&destroyWindow, user32, "DestroyWindow")
	purego.RegisterLibFunc(&waitMessage, user32, "WaitMessage")
	purego.RegisterLibFunc(&postMessageW, user32, "PostMessageW")
	purego.RegisterLibFunc(&msgWaitForMultipleObjectsEx, user32, "MsgWaitForMultipleObjectsEx")
	purego.RegisterLibFunc(&getClientRect, user32, "GetClientRect")
	purego.RegisterLibFunc(&setWindowTextW, user32, "SetWindowTextW")
	purego.RegisterLibFunc(&sendMessageW, user32, "SendMessageW")
	purego.RegisterLibFunc(&setWindowPos, user32, "SetWindowPos")
	purego.RegisterLibFunc(&getSystemMetrics, user32, "GetSystemMetrics")
	purego.RegisterLibFunc(&setCapture, user32, "SetCapture")
	purego.RegisterLibFunc(&getCursorPos, user32, "GetCursorPos")
	purego.RegisterLibFunc(&monitorFromPoint, user32, "MonitorFromPoint")
	purego.RegisterLibFunc(&monitorFromWindow, user32, "MonitorFromWindow")
	purego.RegisterLibFunc(&getMonitorInfoW, user32, "GetMonitorInfoW")
	purego.RegisterLibFunc(&loadCursorW, user32, "LoadCursorW")
	purego.RegisterLibFunc(&immGetContext, imm32, "ImmGetContext")
	purego.RegisterLibFunc(&immGetCompositionStringW, imm32, "ImmGetCompositionStringW")
	purego.RegisterLibFunc(&immNotifyIME, imm32, "ImmNotifyIME")
	purego.RegisterLibFunc(&immSetCandidateWindow, imm32, "ImmSetCandidateWindow")
	purego.RegisterLibFunc(&immReleaseContext, imm32, "ImmReleaseContext")
	purego.RegisterLibFunc(&getDC, user32, "GetDC")
	purego.RegisterLibFunc(&releaseDC, user32, "ReleaseDC")
	purego.RegisterLibFunc(&getDeviceCaps, gdi32, "GetDeviceCaps")
	purego.RegisterLibFunc(&createBitmap, gdi32, "CreateBitmap")
	purego.RegisterLibFunc(&createDIBSection, gdi32, "CreateDIBSection")
	purego.RegisterLibFunc(&deleteObject, gdi32, "DeleteObject")
	purego.RegisterLibFunc(&createIconIndirect, user32, "CreateIconIndirect")
	purego.RegisterLibFunc(&destroyIcon, user32, "DestroyIcon")
	purego.RegisterLibFunc(&openClipboard, user32, "OpenClipboard")
	purego.RegisterLibFunc(&closeClipboard, user32, "CloseClipboard")
	purego.RegisterLibFunc(&emptyClipboard, user32, "EmptyClipboard")
	purego.RegisterLibFunc(&setClipboardData, user32, "SetClipboardData")
	purego.RegisterLibFunc(&getClipboardData, user32, "GetClipboardData")
	purego.RegisterLibFunc(&globalAlloc, kernel32, "GlobalAlloc")
	purego.RegisterLibFunc(&globalLock, kernel32, "GlobalLock")
	purego.RegisterLibFunc(&globalUnlock, kernel32, "GlobalUnlock")
	purego.RegisterLibFunc(&globalSize, kernel32, "GlobalSize")
	purego.RegisterLibFunc(&globalFree, kernel32, "GlobalFree")
	purego.RegisterLibFunc(&dragAcceptFiles, shell32, "DragAcceptFiles")
	purego.RegisterLibFunc(&dragQueryFileW, shell32, "DragQueryFileW")
	purego.RegisterLibFunc(&dragQueryPoint, shell32, "DragQueryPoint")
	purego.RegisterLibFunc(&dragFinish, shell32, "DragFinish")
}

func (c *Context) SetClipboardText(text string) bool {
	if c == nil || !openClipboard(c.window) {
		return false
	}
	defer closeClipboard()
	if !emptyClipboard() {
		return false
	}
	value, err := syscall.UTF16FromString(text)
	if err != nil {
		return false
	}
	memory := globalAlloc(gmemMoveable, uintptr(len(value)*2))
	if memory == 0 {
		return false
	}
	pointer := globalLock(memory)
	if pointer == nil {
		globalFree(memory)
		return false
	}
	copy(unsafe.Slice((*uint16)(pointer), len(value)), value)
	globalUnlock(memory)
	if setClipboardData(cfUnicodeText, memory) == 0 {
		globalFree(memory)
		return false
	}
	return true
}

func (c *Context) ClipboardText() (string, bool) {
	if c == nil || !openClipboard(c.window) {
		return "", false
	}
	defer closeClipboard()
	memory := getClipboardData(cfUnicodeText)
	if memory == 0 {
		return "", false
	}
	pointer := globalLock(memory)
	if pointer == nil {
		return "", false
	}
	defer globalUnlock(memory)
	capacity := int(globalSize(memory) / 2)
	if capacity == 0 {
		return "", false
	}
	units := unsafe.Slice((*uint16)(pointer), capacity)
	length := 0
	for length < len(units) && units[length] != 0 {
		length++
	}
	return string(utf16.Decode(units[:length])), true
}

type Context struct {
	closeRequestSupport
	renderContext
	iconDecoderSupport
	onKeyup        func(input.KeyCode)
	downKeys       map[input.KeyCode]struct{}
	onMouseWheel   func(xoff, yoff float64)
	downMouse      map[input.MouseButton]struct{}
	onMouseUp      func(x, y float64, button input.MouseButton)
	onMouseDown    func(x, y float64, button input.MouseButton)
	onMouseMove    func(x, y float64)
	onKeydown      func(input.KeyCode)
	onTouchStart   func(x, y float64)
	onTouchMove    func(x, y float64)
	onFilesDropped func(x, y float64, files []string)
	onTouchEnd     func(x, y float64)
	decorations    DecorationMode
	cursorSupport
	loopControl
	inputRegion []InputRegionRect
	framePacer  framePacer
	mouseCaptureSupport
	textInputSupport
	viewport            viewportState
	message             msg
	rawMouse            rawMouseInput
	messageSlots        [5]uintptr
	mouseX              float64
	Height              int
	mouseY              float64
	Width               int
	icon                hicon
	fullscreenStyle     uintptr
	window              hwnd
	devicePixelRatio    float64
	cornerRadius        int
	fullscreenRect      rect
	rawMouseSize        uint32
	highSurrogate       uint16
	transparent         bool
	resizePending       bool
	customCorners       bool
	roundedCorners      bool
	border              bool
	shadow              bool
	resizable           bool
	fullscreen          bool
	shown               bool
	shouldClose         bool
	fullscreenMaximized bool
	minimized           bool
}

// NativeHandle returns the host HWND for embedding platform controls.
func (c *Context) NativeHandle() uintptr {
	if c == nil {
		return 0
	}
	return uintptr(c.window)
}

func NewContext(width, height int, title string, options Options) *Context {
	return newContext(width, height, title, options.normalized())
}

func newContext(width, height int, title string, options Options) *Context {
	runtime.LockOSThread()
	enableHighDPIAwareness()

	dpi := currentSystemDPI()
	clientWidth := scaleByDPI(width, dpi)
	clientHeight := scaleByDPI(height, dpi)

	c := &Context{
		Width:            clientWidth,
		Height:           clientHeight,
		devicePixelRatio: devicePixelRatioFromDPI(dpi),
		viewport:         newViewportState(width, height),
		framePacer:       newFramePacer(),
		transparent:      options.Transparent,
		decorations:      options.Decorations,
		cornerRadius:     options.CornerRadius,
		customCorners:    options.Decorations == DecorationNone && !options.Transparent && options.CornerRadius > 8,
		border:           options.Border,
		shadow:           options.Shadow,
		resizable:        options.Resizable,
	}
	c.closeRequestSupport.close = func() {
		c.shouldClose = true
		c.Wake()
	}
	c.applyCursor = c.applyPointerCursor
	c.textInputSupport.applyConfig = c.applyTextInputConfig
	creatingContext = c
	defer func() { creatingContext = nil }()

	className := mustUTF16Ptr("OnixWindowClass")
	instance := getModuleHandleW(nil)
	cursor := loadCursorW(0, uintptr(32512))
	wc := &wndClassEx{
		Size:      uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:   syscall.NewCallback(wndProc),
		Instance:  instance,
		Cursor:    cursor,
		ClassName: className,
	}
	registerClassExW(wc)

	windowStyle := nativeWindowStyle(options.Decorations, options.Resizable)
	extendedStyle := uint32(0)
	if options.Transparent || c.customCorners {
		// The Vulkan swapchain supplies the alpha-composited surface. A GDI
		// redirection bitmap would add an opaque backing when DWM activates it.
		extendedStyle = wsExNoRedirectionBitmap
	}
	windowRect := &rect{Left: 0, Top: 0, Right: int32(clientWidth), Bottom: int32(clientHeight)}
	adjustWindowRectForDPI(windowRect, windowStyle, dpi)
	c.window = createWindowExW(
		extendedStyle,
		className,
		mustUTF16Ptr(title),
		windowStyle,
		cwUseDefault,
		cwUseDefault,
		windowRect.Right-windowRect.Left,
		windowRect.Bottom-windowRect.Top,
		0,
		0,
		instance,
		nil,
	)
	if c.window == 0 {
		panic(errors.New("CreateWindowExW failed"))
	}
	dragAcceptFiles(c.window, true)
	windowWidth := windowRect.Right - windowRect.Left
	windowHeight := windowRect.Bottom - windowRect.Top
	workArea := rect{Right: getSystemMetrics(smCXScreen), Bottom: getSystemMetrics(smCYScreen)}
	cursorPosition := point{}
	if getCursorPos(&cursorPosition) {
		info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
		packedCursorPosition := uint64(uint32(cursorPosition.X)) | uint64(uint32(cursorPosition.Y))<<32
		if monitor := monitorFromPoint(packedCursorPosition, monitorDefaultNearest); monitor != 0 && getMonitorInfoW(monitor, &info) {
			workArea = info.Work
		}
	}
	windowX, windowY := centeredWindowPosition(workArea, windowWidth, windowHeight)
	setWindowPos(c.window, 0, windowX, windowY, 0, 0, swpNoSize|swpNoZOrder|swpNoActivate)
	c.SetRoundedCorners(options.Decorations == DecorationNone && !options.Transparent)
	c.applyNonClientAppearance()
	c.devicePixelRatio = devicePixelRatioFromDPI(windowDPI(c.window))
	c.updateClientSize()
	if options.Transparent || c.customCorners {
		if err := c.enableSurfaceAlpha(); err != nil {
			panic(err)
		}
	}

	driver, err := newDesktopRenderDriver(render.NativeSurface{
		Kind:        render.NativeSurfaceWin32,
		Window:      uintptr(c.window),
		Width:       c.Width,
		Height:      c.Height,
		Transparent: options.Transparent || c.customCorners,
	})
	if err != nil {
		panic(err)
	}
	if err := c.setRenderDriver(driver); err != nil {
		driver.Destroy()
		panic(err)
	}
	c.applyRoundedCorners()
	c.adjustViewport()
	c.applyMouseCapture = c.captureMouse
	c.wake = func() { postMessageW(uintptr(c.window), 0, 0, 0) }
	return c
}

func centeredWindowPosition(area rect, width, height int32) (int32, int32) {
	return area.Left + (area.Right-area.Left-width)/2, area.Top + (area.Bottom-area.Top-height)/2
}

func (c *Context) updateClientSize() {
	r := &rect{}
	if getClientRect(c.window, r) {
		c.Width = int(r.Right - r.Left)
		c.Height = int(r.Bottom - r.Top)
	}
	if c.window != 0 {
		c.devicePixelRatio = devicePixelRatioFromDPI(windowDPI(c.window))
	}
}

func (c *Context) adjustViewport() {
	if !c.viewport.apply(c.Device.Driver(),
		float64(c.Width),
		float64(c.Height),
		float64(c.Width),
		float64(c.Height),
	) {
		return
	}
	if c.inputRegion != nil {
		c.applyWindowRegion()
	}
}

// OnFilesDropped registers a callback for files dropped onto the window.
// Coordinates are in logical viewport units. The files slice is only valid
// for the duration of the callback and must be copied when retained.
func (c *Context) OnFilesDropped(f func(x, y float64, files []string)) {
	c.onFilesDropped = chainCallback3(c.onFilesDropped, f)
}

func (c *Context) SetTitle(title string) {
	setWindowTextW(c.window, mustUTF16Ptr(title))
}

func (c *Context) SetIcon(data []byte) error {
	if c.window == 0 || len(data) == 0 {
		return nil
	}

	iconImage, err := loadIconRGBA(data, c.decoder)
	if err != nil {
		return err
	}
	iconHandle, err := createWindowIcon(iconImage)
	if err != nil {
		return fmt.Errorf("create Windows icon: %w", err)
	}

	sendMessageW(c.window, wmSetIcon, iconSmall, uintptr(iconHandle))
	sendMessageW(c.window, wmSetIcon, iconBig, uintptr(iconHandle))

	if c.icon != 0 {
		destroyIcon(c.icon)
	}
	c.icon = iconHandle
	return nil
}

func (c *Context) SetVsyncEnabled(enable bool) {
	c.Device.SetVSync(enable)
}

func (c *Context) SetTargetFPS(fps int) {
	c.framePacer.setTargetFPS(fps)
}

func (c *Context) LoopGame(f func() (bool, error)) error {
	for !c.shouldClose {
		c.ActivateRender()
		if wait, idle := c.idleWait(); idle {
			msgWaitForMultipleObjectsEx(0, 0, uint32(idleTimeoutMillis(wait)), 0x04ff, 0x0004)
			c.framePacer.reset()
		}
		wasMinimized := c.minimized
		if quit := c.pollEvents(); quit {
			break
		}
		if c.minimized {
			c.waitForEvents()
			continue
		}
		immediateFrame := wasMinimized || c.resizePending
		if c.resizePending {
			if err := c.Device.ResizeSurface(c.Width, c.Height); err != nil {
				return err
			}
			// Input regions and surface masks use the final client bounds after
			// resize and DPI messages have been dispatched.
			if c.inputRegion != nil {
				c.applyWindowRegion()
			}
			c.applyRoundedCorners()
			c.adjustViewport()
			c.resizePending = false
		}
		if immediateFrame {
			c.framePacer.reset()
		} else {
			c.framePacer.wait()
		}
		rendered, err := f()
		if err != nil {
			return err
		}
		if rendered {
			if err := c.Device.Present(); err != nil {
				return err
			}
			c.show()
		}
	}
	return nil
}

func (c *Context) show() {
	if c.shown || c.window == 0 {
		return
	}
	showWindow(c.window, swShow)
	c.shown = true
}

func (c *Context) waitForEvents() {
	if c.minimized {
		waitMessage()
	}
}

func (c *Context) Destroy() {
	c.wakeMutex.Lock()
	c.wake = nil
	c.wakeMutex.Unlock()
	c.SetMouseCaptured(false)
	c.destroyRenderer()
	if c.icon != 0 {
		destroyIcon(c.icon)
		c.icon = 0
	}
	if c.window != 0 {
		destroyWindow(c.window)
		c.window = 0
		c.shown = false
	}
}

// pollEvents drains the thread queue with a stack argument array. The retained
// message keeps the raw pointer the Win32 calls write through stable.
func (c *Context) pollEvents() bool {
	c.messageSlots[0] = uintptr(unsafe.Pointer(&c.message))
	c.messageSlots[4] = pmRemove
	for {
		if r, _, _ := purego.SyscallN(peekMessageProc.Addr(), c.messageSlots[:]...); r == 0 {
			return false
		}
		if c.message.Message == wmQuit {
			c.shouldClose = true
			return true
		}
		purego.SyscallN(translateMessageProc.Addr(), c.messageSlots[:1]...)
		purego.SyscallN(dispatchMessageProc.Addr(), c.messageSlots[:1]...)
	}
}

func wndProc(window hwnd, message uint32, wParam, lParam uintptr) uintptr {
	c := windowContexts[window]
	if c == nil && creatingContext != nil {
		c = creatingContext
		windowContexts[window] = c
	}
	if c == nil {
		return defWindowProcW(window, message, wParam, lParam)
	}

	switch message {
	case wmInput:
		c.handleRawMouse(lParam)
		return defWindowProcW(window, message, wParam, lParam)
	case 0x0003: // WM_MOVE
		if c.mouseCaptured && !c.confineMouse() {
			c.SetMouseCaptured(false)
		}
	case wmGetMinMaxInfo:
		if c.decorations == DecorationNone && c.resizable {
			info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
			if monitor := monitorFromWindow(window, monitorDefaultNearest); monitor != 0 && getMonitorInfoW(monitor, &info) {
				applyMaximizedWorkArea((*minMaxInfo)(unsafe.Pointer(lParam)), info)
				return 0
			}
		}
	case wmNCCalcSize:
		if c.decorations == DecorationNone && wParam != 0 {
			return 0
		}
	case wmNCHitTest:
		if c.decorations == DecorationNone && c.resizable {
			return c.undecoratedHitTest(lParam)
		}
	case wmClose:
		if !c.acceptCloseRequest() {
			return 0
		}
		c.shouldClose = true
		return 0
	case wmDestroy:
		c.shouldClose = true
		delete(windowContexts, window)
		return 0
	case wmDpiChanged:
		c.RequestFrame()
		c.devicePixelRatio = devicePixelRatioFromDPI(uint32(wParam & 0xffff))
		suggested := (*rect)(unsafe.Pointer(lParam))
		setWindowPos(
			c.window,
			0,
			suggested.Left,
			suggested.Top,
			suggested.Right-suggested.Left,
			suggested.Bottom-suggested.Top,
			swpNoZOrder|swpNoActivate,
		)
		c.updateClientSize()
		c.resizePending = true
		c.adjustViewport()
		return 0
	case wmSize:
		c.RequestFrame()
		c.minimized = wParam == sizeMinimized
		c.updateClientSize()
		if !c.minimized {
			c.resizePending = true
		}
		c.adjustViewport()
		if c.mouseCaptured && (c.minimized || !c.confineMouse()) {
			c.SetMouseCaptured(false)
		}
		return 0
	case wmKeyDown, wmSysKeyDown:
		if key := mapVirtualKey(wParam, lParam); key != 0 {
			if c.TextInputConfig().Active {
				switch key {
				case input.KeyBackspace:
					c.emitTextCommand(input.TextCommandBackspace)
				case input.KeyEnter:
					c.emitTextCommand(input.TextCommandEnter)
				}
			}
			c.pressKey(key)
		}
		return 0
	case wmKeyUp, wmSysKeyUp:
		if key := mapVirtualKey(wParam, lParam); key != 0 {
			c.releaseKey(key)
		}
		return 0
	case wmKillFocus:
		c.SetMouseCaptured(false)
		c.releaseKeys()
		c.releaseMouseButtons()
	case wmCancelMode:
		// WM_CANCELMODE also occurs while Windows changes button capture. The
		// explicit relative mouse capture remains valid until WM_CAPTURECHANGED.
		c.releaseMouseButtons()
	case wmCaptureChanged:
		if lParam != uintptr(c.window) {
			c.SetMouseCaptured(false)
		}
		c.releaseMouseButtons()
	case wmImeSetContext:
		if lParam&iscShowUICompositionWindow != 0 {
			lParam &^= iscShowUICompositionWindow
		}
	case wmImeStartComposition:
		if c.TextInputConfig().Active {
			c.emitTextComposition(input.TextComposition{Active: true})
			return 0
		}
	case wmImeComposition:
		if !c.TextInputConfig().Active {
			break
		}
		if lParam == 0 {
			c.emitTextComposition(input.TextComposition{})
			return 0
		}
		if lParam&gcsResultStr != 0 {
			c.emitIMEString(gcsResultStr, true)
		}
		if lParam&gcsCompStr != 0 {
			c.emitIMEString(gcsCompStr, false)
		}
		return 0
	case wmImeEndComposition:
		c.emitTextComposition(input.TextComposition{})
		return 0
	case wmChar:
		c.emitChar(wParam)
		return 0
	case wmUnichar:
		if wParam == unicodeNoChar {
			return 1
		}
		if r := rune(wParam); r >= 0x20 && r != 0x7f {
			c.emitTextInput(string(r))
		}
		return 0
	case wmSetCursor:
		// Preserve native titlebar and window-edge resize cursors.
		if hwnd(wParam) == c.window && uint16(lParam) == 1 && !c.mouseCaptured {
			c.setPointerCursor()
			return 1
		}
	case wmMouseMove:
		if c.mouseCaptured {
			return 0
		}
		x, y := mouseXY(lParam)
		x1, y1 := c.GetLogicalXY(x, y)
		c.mouseX, c.mouseY = x1, y1
		if c.onMouseMove != nil {
			c.onMouseMove(x1, y1)
		}
		return 0
	case wmMouseActivate:
		return maActivate
	case wmLButtonDown, wmRButtonDown, wmMButtonDown:
		if !c.mouseCaptured {
			setCapture(c.window)
		}
		x, y := mouseXY(lParam)
		x1, y1 := c.GetLogicalXY(x, y)
		c.pressMouse(mouseButton(message), x1, y1)
		return 0
	case wmLButtonUp, wmRButtonUp, wmMButtonUp:
		x, y := mouseXY(lParam)
		x1, y1 := c.GetLogicalXY(x, y)
		c.releaseMouse(mouseButton(message), x1, y1)
		if len(c.downMouse) == 0 && !c.mouseCaptured {
			releaseCapture()
		}
		return 0
	case wmMouseWheel:
		if c.onMouseWheel != nil {
			delta := int16((wParam >> 16) & 0xffff)
			c.onMouseWheel(0, float64(delta)/120.0)
		}
		return 0
	case wmDropFiles:
		c.handleDropFiles(wParam)
		return 0
	}

	return defWindowProcW(window, message, wParam, lParam)
}

func (c *Context) handleDropFiles(drop uintptr) {
	if drop == 0 {
		return
	}
	defer dragFinish(drop)
	if c.onFilesDropped == nil {
		return
	}
	count := dragQueryFileW(drop, ^uint32(0), nil, 0)
	if count == 0 {
		return
	}
	files := make([]string, 0, count)
	for index := uint32(0); index < count; index++ {
		length := dragQueryFileW(drop, index, nil, 0)
		if length == 0 || length > 1<<20 {
			continue
		}
		buffer := make([]uint16, length+1)
		if dragQueryFileW(drop, index, &buffer[0], uint32(len(buffer))) == 0 {
			continue
		}
		if path := normalizeDroppedPath(windowsUTF16String(buffer)); path != "" {
			files = append(files, path)
		}
	}
	if len(files) == 0 {
		return
	}
	position := point{}
	if !dragQueryPoint(drop, &position) {
		position = point{X: int32(c.mouseX), Y: int32(c.mouseY)}
	}
	x, y := c.GetLogicalXY(float64(position.X), float64(position.Y))
	c.onFilesDropped(x, y, files)
}

func (c *Context) pressKey(key input.KeyCode) {
	if c.downKeys == nil {
		c.downKeys = make(map[input.KeyCode]struct{})
	}
	c.downKeys[key] = struct{}{}
	if c.onKeydown != nil {
		c.onKeydown(key)
	}
}

func (c *Context) releaseKey(key input.KeyCode) {
	delete(c.downKeys, key)
	if c.onKeyup != nil {
		c.onKeyup(key)
	}
}

func (c *Context) releaseKeys() {
	for key := range c.downKeys {
		if c.onKeyup != nil {
			c.onKeyup(key)
		}
	}
	clear(c.downKeys)
}

func (c *Context) pressMouse(button input.MouseButton, x, y float64) {
	if c.downMouse == nil {
		c.downMouse = make(map[input.MouseButton]struct{})
	}
	c.downMouse[button] = struct{}{}
	c.mouseX, c.mouseY = x, y
	if c.onMouseDown != nil {
		c.onMouseDown(x, y, button)
	}
}

func (c *Context) releaseMouse(button input.MouseButton, x, y float64) {
	c.mouseX, c.mouseY = x, y
	if _, held := c.downMouse[button]; !held {
		return
	}
	delete(c.downMouse, button)
	if c.onMouseUp != nil {
		c.onMouseUp(x, y, button)
	}
}

func (c *Context) releaseMouseButtons() {
	for button := range c.downMouse {
		c.releaseMouse(button, c.mouseX, c.mouseY)
	}
}

func applyMaximizedWorkArea(target *minMaxInfo, info monitorInfo) {
	if target == nil {
		return
	}
	target.MaxPosition = point{X: info.Work.Left - info.Monitor.Left, Y: info.Work.Top - info.Monitor.Top}
	target.MaxSize = point{X: info.Work.Right - info.Work.Left, Y: info.Work.Bottom - info.Work.Top}
	target.MaxTrackSize = target.MaxSize
}

func createWindowIcon(img *image.RGBA) (hicon, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return 0, errors.New("empty image")
	}

	var pixels unsafe.Pointer
	info := bitmapInfo{
		Header: bitmapInfoHeader{
			Size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
			Width:       int32(width),
			Height:      -int32(height),
			Planes:      1,
			BitCount:    32,
			Compression: biRGB,
			SizeImage:   uint32(width * height * 4),
		},
	}
	colorBitmap := createDIBSection(0, &info, 0, &pixels, 0, 0)
	if colorBitmap == 0 || pixels == nil {
		return 0, errors.New("CreateDIBSection failed")
	}
	defer deleteObject(handle(colorBitmap))

	colorBytes := rgbaToBGRA(img)
	copy(unsafe.Slice((*byte)(pixels), len(colorBytes)), colorBytes)

	maskStride := ((width + 31) / 32) * 4
	maskBits := make([]byte, maskStride*height)
	maskBitmap := createBitmap(int32(width), int32(height), 1, 1, unsafe.Pointer(&maskBits[0]))
	if maskBitmap == 0 {
		return 0, errors.New("CreateBitmap failed")
	}
	defer deleteObject(handle(maskBitmap))

	icon := createIconIndirect(&iconInfo{
		FIcon:    1,
		HbmMask:  maskBitmap,
		HbmColor: colorBitmap,
	})
	if icon == 0 {
		return 0, errors.New("CreateIconIndirect failed")
	}
	return icon, nil
}

func mouseXY(lParam uintptr) (float64, float64) {
	x := int16(lParam & 0xffff)
	y := int16((lParam >> 16) & 0xffff)
	return float64(x), float64(y)
}

func mouseButton(message uint32) input.MouseButton {
	switch message {
	case wmLButtonDown, wmLButtonUp:
		return input.MouseLeft
	case wmRButtonDown, wmRButtonUp:
		return input.MouseRight
	case wmMButtonDown, wmMButtonUp:
		return input.MouseMiddle
	default:
		return input.MouseNone
	}
}

func (c *Context) emitIMEString(kind uint32, committed bool) {
	if c.window == 0 {
		return
	}
	imc := immGetContext(c.window)
	if imc == 0 {
		return
	}
	defer immReleaseContext(c.window, imc)
	length := immGetCompositionStringW(imc, kind, nil, 0)
	if length <= 0 {
		if !committed {
			c.emitTextComposition(input.TextComposition{Active: true})
		}
		return
	}
	buffer := make([]uint16, int(length)/2)
	if len(buffer) == 0 {
		return
	}
	if immGetCompositionStringW(imc, kind, unsafe.Pointer(&buffer[0]), uint32(length)) <= 0 {
		return
	}
	text := string(utf16.Decode(buffer))
	if committed {
		c.emitTextInput(text)
		return
	}
	c.emitTextComposition(input.TextComposition{Text: text, Active: true})
}

func (c *Context) emitChar(wParam uintptr) {
	if wParam >= 0xd800 && wParam <= 0xdbff {
		c.highSurrogate = uint16(wParam)
		return
	}
	var r rune
	if wParam >= 0xdc00 && wParam <= 0xdfff {
		if c.highSurrogate != 0 {
			r = utf16.DecodeRune(rune(c.highSurrogate), rune(wParam))
		}
	} else {
		r = rune(wParam) & 0xffff
	}
	c.highSurrogate = 0
	if r >= 0x20 && r != 0x7f {
		c.emitTextInput(string(r))
	}
}

func (c *Context) applyTextInputConfig(previous, current input.TextInputConfig) {
	c.highSurrogate = 0
	if previous.Active && (!current.Active || previous.Revision != current.Revision) {
		c.cancelComposition()
	}
	if current.Active {
		c.positionIMECandidate(current.Caret)
	}
}

func (c *Context) positionIMECandidate(caret input.TextInputRect) {
	if c.window == 0 || caret.Height <= 0 {
		return
	}
	inputRect, ok := c.viewport.InputRect(float64(caret.X), float64(caret.Y), float64(max(caret.Width, 1)), float64(caret.Height))
	if !ok {
		return
	}
	left := int32(math.Round(inputRect.X))
	top := int32(math.Round(inputRect.Y))
	right := int32(math.Round(inputRect.X + inputRect.Width))
	bottom := int32(math.Round(inputRect.Y + inputRect.Height))

	imc := immGetContext(c.window)
	if imc == 0 {
		return
	}
	defer immReleaseContext(c.window, imc)
	immSetCandidateWindow(imc, &candidateForm{
		Style:        cfsExclude,
		CurrentPos:   point{X: left, Y: top},
		ExcludedArea: rect{Left: left, Top: top, Right: right, Bottom: bottom},
	})
}

func (c *Context) cancelComposition() {
	if c.window == 0 {
		return
	}
	imc := immGetContext(c.window)
	if imc == 0 {
		return
	}
	immNotifyIME(imc, niCompositionString, cpsCancel, 0)
	immReleaseContext(c.window, imc)
	c.emitTextComposition(input.TextComposition{})
}

func mapVirtualKey(wParam, lParam uintptr) input.KeyCode {
	switch uint32(wParam) {
	case vkProcessKey:
		return 0
	case vkEscape:
		return input.KeyEscape
	case vkReturn:
		return input.KeyEnter
	case vkTab:
		return input.KeyTab
	case vkBack:
		return input.KeyBackspace
	case vkInsert:
		return input.KeyInsert
	case vkDelete:
		return input.KeyDelete
	case vkRight:
		return input.KeyRight
	case vkLeft:
		return input.KeyLeft
	case vkDown:
		return input.KeyDown
	case vkUp:
		return input.KeyUp
	case vkPrior:
		return input.KeyPageUp
	case vkNext:
		return input.KeyPageDown
	case vkHome:
		return input.KeyHome
	case vkEnd:
		return input.KeyEnd
	case vkCaps:
		return input.KeyCapsLock
	case vkScroll:
		return input.KeyScrollLock
	case vkNumLock:
		return input.KeyNumLock
	case vkPrint:
		return input.KeyPrintScreen
	case vkPause:
		return input.KeyPause
	case vkShift:
		if byte(lParam>>16) == 0x36 {
			return input.KeyRightShift
		}
		return input.KeyLeftShift
	case vkControl:
		if lParam&(1<<24) != 0 {
			return input.KeyRightControl
		}
		return input.KeyLeftControl
	case vkMenu:
		if lParam&(1<<24) != 0 {
			return input.KeyRightAlt
		}
		return input.KeyLeftAlt
	case vkLShift:
		return input.KeyLeftShift
	case vkRShift:
		return input.KeyRightShift
	case vkLControl:
		return input.KeyLeftControl
	case vkRControl:
		return input.KeyRightControl
	case vkLMenu:
		return input.KeyLeftAlt
	case vkRMenu:
		return input.KeyRightAlt
	case vkLWin:
		return input.KeyLeftSuper
	case vkRWin:
		return input.KeyRightSuper
	case vkApps:
		return input.KeyMenu
	}
	vk := uint32(wParam)
	if vk >= 'A' && vk <= 'Z' {
		return input.KeyCode(vk)
	}
	if vk >= '0' && vk <= '9' {
		return input.KeyCode(vk)
	}
	if vk >= 0x70 && vk <= 0x88 {
		return input.KeyCode(int(input.KeyF1) + int(vk-0x70))
	}
	return input.KeyCode(vk)
}

func enableHighDPIAwareness() {
	setProcessDpiAwarenessContext := syscall.NewLazyDLL("user32.dll").NewProc("SetProcessDpiAwarenessContext")
	if err := setProcessDpiAwarenessContext.Find(); err == nil {
		ret, _, _ := setProcessDpiAwarenessContext.Call(dpiPerMonitorV2)
		if ret != 0 {
			return
		}
	}

	setProcessDPIAware := syscall.NewLazyDLL("user32.dll").NewProc("SetProcessDPIAware")
	if err := setProcessDPIAware.Find(); err == nil {
		setProcessDPIAware.Call()
	}
}

func currentSystemDPI() uint32 {
	getDpiForSystem := syscall.NewLazyDLL("user32.dll").NewProc("GetDpiForSystem")
	if err := getDpiForSystem.Find(); err == nil {
		ret, _, _ := getDpiForSystem.Call()
		if ret != 0 {
			return uint32(ret)
		}
	}

	screenDC := getDC(0)
	if screenDC != 0 {
		defer releaseDC(0, screenDC)
		if dpi := getDeviceCaps(screenDC, logicalPixelsX); dpi > 0 {
			return uint32(dpi)
		}
	}

	return defaultDPI
}

func windowDPI(window hwnd) uint32 {
	if window != 0 {
		getDpiForWindow := syscall.NewLazyDLL("user32.dll").NewProc("GetDpiForWindow")
		if err := getDpiForWindow.Find(); err == nil {
			ret, _, _ := getDpiForWindow.Call(uintptr(window))
			if ret != 0 {
				return uint32(ret)
			}
		}
	}
	return currentSystemDPI()
}

func adjustWindowRectForDPI(r *rect, style uint32, dpi uint32) {
	adjustWindowRectExForDpi := syscall.NewLazyDLL("user32.dll").NewProc("AdjustWindowRectExForDpi")
	if err := adjustWindowRectExForDpi.Find(); err == nil {
		ret, _, _ := adjustWindowRectExForDpi.Call(
			uintptr(unsafe.Pointer(r)),
			uintptr(style),
			0,
			0,
			uintptr(dpi),
		)
		if ret != 0 {
			return
		}
	}
	adjustWindowRect(r, style, false)
}

func scaleByDPI(size int, dpi uint32) int {
	if size <= 0 {
		return size
	}
	return int(math.Round(float64(size) * devicePixelRatioFromDPI(dpi)))
}

func devicePixelRatioFromDPI(dpi uint32) float64 {
	if dpi == 0 {
		return 1
	}
	return float64(dpi) / defaultDPI
}

func mustUTF16Ptr(s string) *uint16 {
	ptr, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		panic(err)
	}
	return ptr
}
