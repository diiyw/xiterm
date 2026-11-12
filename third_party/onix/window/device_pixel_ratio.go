//go:build js || (cgo && (windows || darwin || android || ios))

package window

// DevicePixelRatio returns the current platform pixel density.
func (c *Context) DevicePixelRatio() float64 {
	if c == nil {
		return 1
	}
	return normalizeDevicePixelRatio(c.devicePixelRatio)
}
