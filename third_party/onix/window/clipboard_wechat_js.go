//go:build wechat

package window

func (c *Context) SetClipboardText(text string) bool {
	if c == nil || !c.wx.Truthy() {
		return false
	}
	options := c.Get("Object").New()
	options.Set("data", text)
	c.wx.Call("setClipboardData", options)
	return true
}

func (c *Context) ClipboardText() (string, bool) { return "", false }
