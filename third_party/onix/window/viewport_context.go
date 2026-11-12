//go:build js || (cgo && (windows || linux || darwin || android || ios))

package window

import "github.com/onix-engine/onix/render"

func (c *Context) GetViewport() (int, int) {
	return c.viewport.ViewSize()
}

func (c *Context) SetViewport(width, height int) {
	c.viewport.SetViewport(width, height)
	c.adjustViewport()
}

func (c *Context) SetViewportMode(mode ViewportMode) {
	c.viewport.SetMode(mode)
	c.adjustViewport()
}

func (c *Context) GetLogicalXY(x, y float64) (float64, float64) {
	return c.viewport.LogicalXY(x, y)
}

func (s *viewportState) apply(driver render.Driver, renderWidth, renderHeight, inputWidth, inputHeight float64) bool {
	layout, ok := s.Update(renderWidth, renderHeight, inputWidth, inputHeight)
	if !ok {
		return false
	}
	if driver != nil {
		driver.SetViewport(render.Rect{
			X:      int32(layout.RenderViewport.X),
			Y:      int32(layout.RenderViewport.Y),
			Width:  int32(layout.RenderViewport.Width),
			Height: int32(layout.RenderViewport.Height),
		})
	}
	return true
}
