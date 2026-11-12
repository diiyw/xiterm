//go:build wechat

package window

import (
	"math"
	"strings"
	"syscall/js"

	"github.com/onix-engine/onix/input"
	"github.com/onix-engine/onix/render"
)

type Context struct {
	renderContext
	loopControl
	cursorSupport
	mouseCaptureSupport
	textInputSupport
	closeRequestSupport
	iconDecoderSupport
	Width, Height int
	js.Value
	wx                 js.Value
	canvas             js.Value
	devicePixelRatio   float64
	animationFrameChan chan struct{}
	logicalWidth       int
	logicalHeight      int
	viewport           viewportState
	options            Options
	keyboardValue      string
	keyboardVisible    bool
	keyboardRestarting bool
	restartValue       string
}

// NativeHandle is zero for WeChat contexts.
func (c *Context) NativeHandle() uintptr { return 0 }

func NewContext(width, height int, _ string, options Options) *Context {
	c := &Context{
		Value:              js.Global(),
		wx:                 js.Global().Get("wx"),
		animationFrameChan: make(chan struct{}, 1),
		viewport:           newViewportState(width, height),
		options:            options.normalized(),
	}
	c.textInputSupport.applyConfig = c.applyTextInputConfig
	windowInfo := c.wx.Call("getWindowInfo")
	c.devicePixelRatio = windowInfo.Get("pixelRatio").Float()
	if c.devicePixelRatio <= 0 {
		c.devicePixelRatio = 1
	}
	c.logicalWidth = windowInfo.Get("windowWidth").Int()
	c.logicalHeight = windowInfo.Get("windowHeight").Int()
	surfaceScale := min(c.devicePixelRatio, c.options.MaxDevicePixelRatio) * c.options.RenderScale
	c.Width = max(1, int(math.Round(float64(c.logicalWidth)*surfaceScale)))
	c.Height = max(1, int(math.Round(float64(c.logicalHeight)*surfaceScale)))
	c.canvas = c.wx.Call("createCanvas")
	// Initialize WebGL2 context.
	attr := c.Get("Object").New()
	attr.Set("alpha", c.options.Transparent)
	attr.Set("premultipliedAlpha", true)
	attr.Set("depth", false)
	attr.Set("antialias", c.options.Antialias)
	driver, err := render.NewWebGL2Driver(c.canvas.Call("getContext", "webgl2", attr))
	if err != nil {
		panic(err)
	}
	if err := c.setRenderDriver(driver); err != nil {
		panic(err)
	}
	// Initialize canvas elements.
	c.Init()
	c.initTextInput()
	// Request first animation frame.
	c.animationFrameChan <- struct{}{}
	return c
}

func (c *Context) Init() {
	c.RequestFrame()
	c.canvas.Set("width", c.Width)
	c.canvas.Set("height", c.Height)
	c.adjustViewport()
}

func (c *Context) adjustViewport() {
	c.viewport.apply(c.Device.Driver(),
		float64(c.Width),
		float64(c.Height),
		float64(c.logicalWidth),
		float64(c.logicalHeight),
	)
}

func (c *Context) OnKeydown(f func(input.KeyCode)) {
	c.wx.Call("onKeyDown", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		keyEvent := args[0]
		f(toKey(keyEvent))
		return nil
	}))
}

func (c *Context) OnKeyup(f func(input.KeyCode)) {
	c.wx.Call("onKeyUp", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		keyEvent := args[0]
		f(toKey(keyEvent))
		return nil
	}))
}

func (c *Context) OnMouseDown(f func(x, y float64, button input.MouseButton)) {
	c.wx.Call("onMouseDown", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		mouseEvent := args[0]
		x, y := jsInputPosition(c, mouseEvent)
		button := input.MouseButton(mouseEvent.Get("button").Int())
		f(x, y, button)
		return nil
	}))
}

func (c *Context) OnMouseUp(f func(x, y float64, button input.MouseButton)) {
	c.wx.Call("onMouseUp", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		mouseEvent := args[0]
		x, y := jsInputPosition(c, mouseEvent)
		button := input.MouseButton(mouseEvent.Get("button").Int())
		f(x, y, button)
		return nil
	}))
}

func (c *Context) OnMouseMove(f func(x, y float64)) {
	c.wx.Call("onMouseMove", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		mouseEvent := args[0]
		x, y := jsInputPosition(c, mouseEvent)
		f(x, y)
		return nil
	}))
}

func (c *Context) OnMouseWheel(f func(xoff float64, yoff float64)) {
	c.wx.Call("onWheel", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		f(jsWheelOffset(args[0]))
		return nil
	}))
}

func (c *Context) OnTouchStart(f func(x, y float64)) {
	c.wx.Call("onTouchStart", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		forEachJSTouch(c, args[0], "touches", f)
		return nil
	}))
}

func (c *Context) initTextInput() {
	c.wx.Call("onKeyboardInput", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if c.keyboardRestarting || !c.TextInputConfig().Active {
			return nil
		}
		c.syncKeyboardValue(c.keyboardEventValue(args))
		return nil
	}))
	c.wx.Call("onKeyboardConfirm", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if c.keyboardRestarting || !c.TextInputConfig().Active {
			return nil
		}
		inserted := c.syncKeyboardValue(c.keyboardEventValue(args))
		if !c.TextInputConfig().Multiline || !isLineBreakText(inserted) {
			c.emitTextCommand(input.TextCommandEnter)
		}
		return nil
	}))
	c.wx.Call("onKeyboardComplete", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if c.keyboardRestarting {
			return nil
		}
		if c.TextInputConfig().Active {
			c.syncKeyboardValue(c.keyboardEventValue(args))
		}
		c.keyboardVisible = false
		c.emitTextComposition(input.TextComposition{})
		return nil
	}))
}

func (c *Context) applyTextInputConfig(previous, current input.TextInputConfig) {
	if !current.Active {
		c.keyboardRestarting = false
		c.restartValue = ""
		c.hideKeyboard()
		return
	}
	if previous.Revision != current.Revision {
		if c.keyboardRestarting {
			c.restartValue = current.InitialValue
			return
		}
		if c.keyboardVisible {
			c.keyboardRestarting = true
			c.restartValue = current.InitialValue
			options := c.Get("Object").New()
			var complete js.Func
			complete = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
				complete.Release()
				c.finishKeyboardRestart()
				return nil
			})
			options.Set("complete", complete)
			c.wx.Call("hideKeyboard", options)
			return
		}
		c.keyboardValue = current.InitialValue
	}
	c.showKeyboard()
}

func (c *Context) finishKeyboardRestart() {
	if !c.keyboardRestarting {
		return
	}
	c.keyboardRestarting = false
	c.keyboardVisible = false
	c.keyboardValue = c.restartValue
	c.restartValue = ""
	c.emitTextComposition(input.TextComposition{})
	c.showKeyboard()
}

func (c *Context) showKeyboard() {
	config := c.TextInputConfig()
	if !config.Active || c.keyboardVisible || c.keyboardRestarting {
		return
	}
	options := c.Get("Object").New()
	options.Set("defaultValue", c.keyboardValue)
	options.Set("maxLength", 10000)
	options.Set("multiple", config.Multiline)
	options.Set("confirmHold", false)
	if !config.Multiline {
		options.Set("confirmType", "done")
	}
	c.wx.Call("showKeyboard", options)
	c.keyboardVisible = true
}

func (c *Context) keyboardEventValue(args []js.Value) string {
	if len(args) == 0 {
		return c.keyboardValue
	}
	return args[0].Get("value").String()
}

func (c *Context) hideKeyboard() {
	if !c.keyboardVisible {
		return
	}
	c.wx.Call("hideKeyboard")
	c.keyboardVisible = false
	c.keyboardValue = ""
	c.emitTextComposition(input.TextComposition{})
}

func (c *Context) syncKeyboardValue(value string) string {
	previous := []rune(c.keyboardValue)
	current := []rune(value)
	prefix := 0
	for prefix < len(previous) && prefix < len(current) && previous[prefix] == current[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(previous)-prefix && suffix < len(current)-prefix && previous[len(previous)-1-suffix] == current[len(current)-1-suffix] {
		suffix++
	}
	previousEnd, currentEnd := len(previous)-suffix, len(current)-suffix
	if prefix < previousEnd {
		c.emitTextSelection(input.TextSelection{Start: prefix, End: previousEnd})
		c.emitTextCommand(input.TextCommandBackspace)
	}
	inserted := string(current[prefix:currentEnd])
	if inserted != "" {
		c.emitTextInput(inserted)
	}
	c.keyboardValue = value
	return inserted
}

func isLineBreakText(text string) bool {
	return text != "" && strings.Trim(text, "\r\n") == ""
}

func (c *Context) OnTouchMove(f func(x, y float64)) {
	c.wx.Call("onTouchMove", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		forEachJSTouch(c, args[0], "touches", f)
		return nil
	}))
}

func (c *Context) OnTouchEnd(f func(x, y float64)) {
	c.wx.Call("onTouchEnd", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		forEachJSTouch(c, args[0], "changedTouches", f)
		if c.TextInputConfig().Active && !c.keyboardVisible {
			c.showKeyboard()
		}
		return nil
	}))
}

func (c *Context) OnFilesDropped(func(x, y float64, files []string)) {}

func (c *Context) SetTitle(title string) {
	c.wx.Set("title", title)
}

func (c *Context) SetIcon(data []byte) error { return nil }

func (c *Context) SetVsyncEnabled(enable bool) {
	c.Device.SetVSync(enable)
}

func (c *Context) SetTargetFPS(int) {
	// requestAnimationFrame owns the window cadence; App controls render pacing.
}

func (c *Context) Destroy() {
	c.destroyRenderer()
}
