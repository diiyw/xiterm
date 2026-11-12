//go:build cgo && linux && !android

package window

// DevicePixelRatio returns the current platform pixel density.
func (c *Context) DevicePixelRatio() float64 { return 1 }
