//go:build !wechat

package window

import (
	"math"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/onix-engine/onix/input"
)

const textInputSentinel = "\u200B"

type Context struct {
	textInputSupport
	js.Value
	canvas js.Value
	renderContext
	closeRequestSupport
	iconDecoderSupport
	textInput          js.Value
	document           js.Value
	animationFrameChan chan struct{}
	mouseCaptureMove   js.Func
	mouseCaptureError  js.Func
	loopControl
	mouseCaptureClick  js.Func
	mouseCaptureChange js.Func
	cursorSupport
	mouseCaptureSupport
	options               Options
	viewport              viewportState
	devicePixelRatio      float64
	logicalHeight         int
	logicalWidth          int
	Height                int
	Width                 int
	composing             bool
	mouseCaptureRequested bool
}

// NativeHandle is zero for browser contexts.
func (c *Context) NativeHandle() uintptr { return 0 }

func NewContext(width, height int, title string, options Options) *Context {
	c := &Context{
		Value:              js.Global(),
		document:           js.Global().Get("document"),
		devicePixelRatio:   js.Global().Get("devicePixelRatio").Float(),
		animationFrameChan: make(chan struct{}, 1),
		viewport:           newViewportState(width, height),
		options:            options.normalized(),
	}
	c.textInputSupport.applyConfig = c.applyTextInputConfig
	c.document.Set("title", title)
	// viewport
	meta := c.document.Call("createElement", "meta")
	meta.Set("name", "viewport")
	meta.Set("content", "width=device-width, initial-scale="+strconv.FormatFloat(1/c.devicePixelRatio, 'f', 1, 64)+", user-scalable=no")
	c.document.Get("head").Call("appendChild", meta)
	c.canvas = c.document.Call("createElement", "canvas")
	if c.document.Get("body").IsUndefined() {
		body := c.document.Call("createElement", "body")
		c.document.Set("body", body)
	}
	c.document.Get("body").Call("appendChild", c.canvas)
	c.applyCursor = func(cursor input.Cursor) { c.canvas.Get("style").Set("cursor", cursor.String()) }
	c.initMouseCapture()
	c.textInput = c.document.Call("createElement", "textarea")
	c.initTextInputElement()
	// Initialize the selected renderer after sizing the canvas drawing buffer.
	c.Init()
	driver, err := newBrowserRenderDriver(c.canvas, c.options.Transparent, c.options.Antialias)
	if err != nil {
		panic(err)
	}
	if err := c.setRenderDriver(driver); err != nil {
		panic(err)
	}
	c.Call("addEventListener", "resize", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		c.Init()
		return nil
	}))
	c.document.Call("addEventListener", "mouseup", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		c.syncTextInputFocus()
		return nil
	}))
	c.document.Call("addEventListener", "touchend", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		c.syncTextInputFocus()
		return nil
	}))
	// Request first animation frame.
	c.animationFrameChan <- struct{}{}
	return c
}

func (c *Context) initTextInputElement() {
	if !c.textInput.Truthy() {
		return
	}
	c.textInput.Set("autocapitalize", "off")
	c.textInput.Set("spellcheck", false)
	c.textInput.Set("translate", "no")
	c.textInput.Set("wrap", "off")
	style := c.textInput.Get("style")
	style.Set("position", "absolute")
	style.Set("left", "0")
	style.Set("top", "0")
	style.Set("opacity", "0")
	style.Set("resize", "none")
	style.Set("pointerEvents", "none")
	style.Set("overflow", "hidden")
	style.Set("width", "1px")
	style.Set("height", "1px")
	c.document.Get("body").Call("appendChild", c.textInput)

	c.textInput.Call("addEventListener", "compositionstart", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		c.composing = true
		c.emitTextComposition(input.TextComposition{Active: true})
		return nil
	}))
	c.textInput.Call("addEventListener", "compositionupdate", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		c.emitTextComposition(input.TextComposition{Text: args[0].Get("data").String(), Active: true})
		return nil
	}))
	c.textInput.Call("addEventListener", "compositionend", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		c.composing = false
		c.emitTextComposition(input.TextComposition{})
		return nil
	}))
	c.textInput.Call("addEventListener", "beforeinput", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		event := args[0]
		if event.Get("isComposing").Truthy() || c.composing {
			return nil
		}
		inputType := event.Get("inputType").String()
		switch {
		case strings.HasPrefix(inputType, "delete") && strings.Contains(inputType, "Backward"):
			event.Call("preventDefault")
			c.emitTextCommand(input.TextCommandBackspace)
		case inputType == "insertLineBreak" || inputType == "insertParagraph":
			event.Call("preventDefault")
			c.emitTextCommand(input.TextCommandEnter)
		}
		return nil
	}))
	c.textInput.Call("addEventListener", "input", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		event := args[0]
		if event.Get("isComposing").Truthy() || c.composing {
			return nil
		}
		inputType := event.Get("inputType").String()
		if strings.HasPrefix(inputType, "delete") {
			if strings.Contains(inputType, "Backward") {
				c.emitTextCommand(input.TextCommandBackspace)
			}
			c.resetTextInputBuffer()
			return nil
		}
		if inputType == "insertLineBreak" || inputType == "insertParagraph" {
			c.emitTextCommand(input.TextCommandEnter)
			c.resetTextInputBuffer()
			return nil
		}
		data := event.Get("data")
		text := strings.ReplaceAll(c.textInput.Get("value").String(), textInputSentinel, "")
		if data.Type() == js.TypeString {
			text = data.String()
		}
		c.emitTextInput(text)
		c.resetTextInputBuffer()
		return nil
	}))
}

func (c *Context) resetTextInputBuffer() {
	if c == nil || !c.textInput.Truthy() {
		return
	}
	c.textInput.Set("value", textInputSentinel)
	c.textInput.Call("setSelectionRange", 1, 1)
}

func (c *Context) focusTextInput() {
	if c == nil || !c.textInput.Truthy() || !c.TextInputConfig().Active {
		return
	}
	if !c.document.Get("activeElement").Equal(c.textInput) {
		c.textInput.Call("focus")
	}
}

func (c *Context) blurTextInput() {
	if c == nil || !c.textInput.Truthy() {
		return
	}
	if c.document.Get("activeElement").Equal(c.textInput) {
		c.textInput.Call("blur")
	}
}

func (c *Context) syncTextInputFocus() {
	if c.TextInputConfig().Active {
		c.focusTextInput()
		return
	}
	c.blurTextInput()
}

func (c *Context) applyTextInputConfig(previous, current input.TextInputConfig) {
	if !current.Active {
		if c.composing {
			c.composing = false
			c.emitTextComposition(input.TextComposition{})
		}
		c.textInput.Set("value", "")
		c.blurTextInput()
		return
	}
	if previous.Revision != current.Revision {
		if previous.Active {
			c.blurTextInput()
		}
		c.composing = false
		c.emitTextComposition(input.TextComposition{})
		c.resetTextInputBuffer()
	}
	switch current.Kind {
	case input.TextInputDecimal:
		c.textInput.Set("inputMode", "decimal")
	case input.TextInputInteger:
		c.textInput.Set("inputMode", "numeric")
	default:
		c.textInput.Set("inputMode", "text")
	}
	if current.Multiline {
		c.textInput.Set("enterKeyHint", "enter")
	} else {
		c.textInput.Set("enterKeyHint", "done")
	}
	c.focusTextInput()
}

func (c *Context) Init() {
	c.RequestFrame()
	c.devicePixelRatio = c.Get("devicePixelRatio").Float()
	if c.devicePixelRatio <= 0 {
		c.devicePixelRatio = 1
	}
	c.logicalWidth = c.Get("innerWidth").Int()
	c.logicalHeight = c.Get("innerHeight").Int()
	surfaceScale := min(c.devicePixelRatio, c.options.MaxDevicePixelRatio) * c.options.RenderScale
	c.Width = max(1, int(math.Round(float64(c.logicalWidth)*surfaceScale)))
	c.Height = max(1, int(math.Round(float64(c.logicalHeight)*surfaceScale)))
	c.canvas.Set("width", c.Width)
	c.canvas.Set("height", c.Height)
	style := c.canvas.Get("style")
	style.Set("width", strconv.Itoa(c.logicalWidth)+"px")
	style.Set("height", strconv.Itoa(c.logicalHeight)+"px")
	style.Set("display", "block")
	bodyStyle := c.document.Get("body").Get("style")
	bodyStyle.Set("margin", "0")
	bodyStyle.Set("overflow", "hidden")
	c.adjustViewport()
	if c.Device.Driver() != nil {
		if err := c.Device.ResizeSurface(c.Width, c.Height); err != nil {
			panic(err)
		}
	}
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
	c.document.Call("addEventListener", "keydown", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		keyEvent := args[0]
		if keyEvent.Get("isComposing").Truthy() || keyEvent.Get("keyCode").Int() == 229 {
			return nil
		}
		f(toKey(keyEvent))
		return nil
	}))
}

func (c *Context) OnKeyup(f func(input.KeyCode)) {
	c.document.Call("addEventListener", "keyup", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		keyEvent := args[0]
		if keyEvent.Get("isComposing").Truthy() || keyEvent.Get("keyCode").Int() == 229 {
			return nil
		}
		f(toKey(keyEvent))
		return nil
	}))
}

func (c *Context) OnMouseDown(f func(x, y float64, button input.MouseButton)) {
	c.document.Call("addEventListener", "mousedown", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		mouseEvent := args[0]
		x, y := jsInputPosition(c, mouseEvent)
		button := input.MouseButton(mouseEvent.Get("button").Int())
		f(x, y, button)
		return nil
	}))
}

func (c *Context) OnMouseUp(f func(x, y float64, button input.MouseButton)) {
	c.document.Call("addEventListener", "mouseup", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		mouseEvent := args[0]
		x, y := jsInputPosition(c, mouseEvent)
		button := input.MouseButton(mouseEvent.Get("button").Int())
		f(x, y, button)
		return nil
	}))
}

func (c *Context) OnMouseMove(f func(x, y float64)) {
	c.document.Call("addEventListener", "mousemove", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if c.mouseCaptured {
			return nil
		}
		mouseEvent := args[0]
		x, y := jsInputPosition(c, mouseEvent)
		f(x, y)
		return nil
	}))
}

func (c *Context) OnMouseWheel(f func(xoff float64, yoff float64)) {
	c.document.Call("addEventListener", "wheel", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		f(jsWheelOffset(args[0]))
		return nil
	}))
}

func (c *Context) OnTouchStart(f func(x, y float64)) {
	c.document.Call("addEventListener", "touchstart", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		forEachJSTouch(c, args[0], "touches", f)
		return nil
	}))
}

func (c *Context) OnTouchMove(f func(x, y float64)) {
	c.document.Call("addEventListener", "touchmove", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		forEachJSTouch(c, args[0], "touches", f)
		return nil
	}))
}

func (c *Context) OnTouchEnd(f func(x, y float64)) {
	c.document.Call("addEventListener", "touchend", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		forEachJSTouch(c, args[0], "changedTouches", f)
		return nil
	}))
}

// OnFilesDropped registers a callback for files dropped onto the canvas.
// Browser security exposes file names rather than local filesystem paths.
func (c *Context) OnFilesDropped(f func(x, y float64, files []string)) {
	handleDrop := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) == 0 {
			return nil
		}
		event := args[0]
		event.Call("preventDefault")
		if event.Type() == js.TypeObject && event.Get("type").String() == "dragover" {
			return nil
		}
		files := jsDroppedFiles(event)
		if len(files) == 0 {
			return nil
		}
		x, y := jsInputPosition(c, event)
		f(x, y, files)
		return nil
	})
	c.document.Call("addEventListener", "dragover", handleDrop)
	c.document.Call("addEventListener", "drop", handleDrop)
}

func (c *Context) SetTitle(title string) {
	c.document.Set("title", title)
}

func (c *Context) SetIcon(data []byte) error { return nil }

func (c *Context) SetVsyncEnabled(enable bool) {
	c.Device.SetVSync(enable)
}

func (c *Context) SetTargetFPS(int) {
	// requestAnimationFrame owns the window cadence; App controls render pacing.
}

func (c *Context) Destroy() {
	c.destroyMouseCapture()
	c.destroyRenderer()
}
