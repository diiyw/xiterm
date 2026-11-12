//go:build js || (cgo && (windows || linux || darwin || android || ios))

package window

import (
	"github.com/onix-engine/onix/graphics"
	"github.com/onix-engine/onix/render"
)

type renderContext struct {
	Device   *render.Device
	canvas2D *graphics.Canvas2D
}

func (c *renderContext) setRenderDriver(driver render.Driver) error {
	device, err := render.NewDevice(driver)
	if err != nil {
		return err
	}
	c.destroyRenderer()
	c.Device = device
	device.Activate()
	return nil
}

// ActivateRender selects this window's immediate rendering context. Windows
// share a rendering thread; resource residency and lifetime stay per device.
func (c *Context) ActivateRender() {
	c.Device.Activate()
	width, height := c.GetViewport()
	c.Canvas2D(width, height)
}

func (c *Context) Canvas2D(width, height int) *graphics.Canvas2D {
	if c.canvas2D == nil {
		c.canvas2D = graphics.NewCanvas2D(c.Device, width, height)
	} else {
		c.canvas2D.Resize(width, height)
	}
	graphics.SetCurrentContext(c.canvas2D)
	return c.canvas2D
}

func (c *renderContext) destroyRenderer() {
	if c.canvas2D != nil {
		_ = c.canvas2D.FinishFrame()
		if graphics.CurrentContext() == c.canvas2D {
			graphics.SetCurrentContext(nil)
		}
		c.canvas2D = nil
	}
	c.Device.Destroy()
	c.Device = nil
}
