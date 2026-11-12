//go:build cgo && (windows || (darwin && !ios) || (linux && !android && !ios && !js && !wasm))

package window

import "github.com/onix-engine/onix/input"

func (c *Context) OnKeydown(f func(input.KeyCode)) {
	c.onKeydown = chainCallback(c.onKeydown, f)
}

func (c *Context) OnKeyup(f func(input.KeyCode)) {
	c.onKeyup = chainCallback(c.onKeyup, f)
}

func (c *Context) OnMouseDown(f func(x, y float64, button input.MouseButton)) {
	c.onMouseDown = chainCallback3(c.onMouseDown, f)
}

func (c *Context) OnMouseUp(f func(x, y float64, button input.MouseButton)) {
	c.onMouseUp = chainCallback3(c.onMouseUp, f)
}

func (c *Context) OnMouseMove(f func(x, y float64)) {
	c.onMouseMove = chainCallback2(c.onMouseMove, f)
}

func (c *Context) OnMouseWheel(f func(xoff float64, yoff float64)) {
	c.onMouseWheel = chainCallback2(c.onMouseWheel, f)
}

func (c *Context) OnTouchStart(f func(x, y float64)) {
	c.onTouchStart = chainCallback2(c.onTouchStart, f)
}

func (c *Context) OnTouchMove(f func(x, y float64)) {
	c.onTouchMove = chainCallback2(c.onTouchMove, f)
}

func (c *Context) OnTouchEnd(f func(x, y float64)) {
	c.onTouchEnd = chainCallback2(c.onTouchEnd, f)
}
