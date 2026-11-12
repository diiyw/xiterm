//go:build js && !wechat

package window

import "syscall/js"

func (c *Context) initMouseCapture() {
	c.applyMouseCapture = func(captured bool) bool {
		if c.canvas.Get("requestPointerLock").Type() != js.TypeFunction {
			return false
		}
		c.mouseCaptureRequested = captured
		if !captured {
			if c.mouseCaptured {
				c.document.Call("exitPointerLock")
			}
			return true
		}
		if !c.mouseCaptured && c.Get("navigator").Get("userActivation").Get("isActive").Bool() {
			c.requestPointerLock()
		}
		return true
	}
	c.mouseCaptureChange = js.FuncOf(func(js.Value, []js.Value) any {
		captured := c.document.Get("pointerLockElement").Equal(c.canvas)
		if captured && !c.mouseCaptureRequested {
			c.document.Call("exitPointerLock")
			return nil
		}
		c.updateMouseCapture(captured)
		if !captured {
			c.mouseCaptureRequested = false
		}
		return nil
	})
	c.mouseCaptureClick = js.FuncOf(func(js.Value, []js.Value) any {
		if c.mouseCaptureRequested && !c.mouseCaptured {
			c.requestPointerLock()
		}
		return nil
	})
	c.mouseCaptureMove = js.FuncOf(func(_ js.Value, args []js.Value) any {
		if c.mouseCaptured {
			c.addMouseDelta(args[0].Get("movementX").Float(), args[0].Get("movementY").Float())
		}
		return nil
	})
	c.mouseCaptureError = js.FuncOf(func(js.Value, []js.Value) any { return nil })
	c.document.Call("addEventListener", "pointerlockchange", c.mouseCaptureChange)
	c.canvas.Call("addEventListener", "mousedown", c.mouseCaptureClick)
	c.document.Call("addEventListener", "mousemove", c.mouseCaptureMove)
}

func (c *Context) requestPointerLock() {
	c.canvas.Call("requestPointerLock").Call("catch", c.mouseCaptureError)
}

func (c *Context) destroyMouseCapture() {
	c.SetMouseCaptured(false)
	c.document.Call("removeEventListener", "pointerlockchange", c.mouseCaptureChange)
	c.canvas.Call("removeEventListener", "mousedown", c.mouseCaptureClick)
	c.document.Call("removeEventListener", "mousemove", c.mouseCaptureMove)
	c.mouseCaptureChange.Release()
	c.mouseCaptureClick.Release()
	c.mouseCaptureMove.Release()
	c.mouseCaptureError.Release()
	c.updateMouseCapture(false)
}
