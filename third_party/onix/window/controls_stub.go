//go:build android || ios || js || (linux && onix_drm)

package window

func (c *Context) SetDecorations(DecorationMode)    {}
func (c *Context) SetTitleBarHeight(float64)        {}
func (c *Context) SetBorderVisible(bool)            {}
func (c *Context) UsesClientWindowBorder() bool     { return false }
func (c *Context) SetShadowVisible(bool)            {}
func (c *Context) SetTaskbarVisible(bool)           {}
func (c *Context) SetAlwaysOnTop(bool)              {}
func (c *Context) SetInputRegion([]InputRegionRect) {}
func (c *Context) SetResizable(bool)                {}
func (c *Context) Minimize()                        {}
func (c *Context) ToggleMaximize()                  {}
func (c *Context) ToggleFullscreen()                {}
func (c *Context) StartWindowDrag()                 {}
