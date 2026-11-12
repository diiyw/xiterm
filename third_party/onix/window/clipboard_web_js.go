//go:build js && !wechat

package window

func (c *Context) SetClipboardText(text string) bool {
	if c == nil || !c.textInput.Truthy() {
		return false
	}
	previous := c.textInput.Get("value").String()
	c.textInput.Set("value", text)
	c.textInput.Call("select")
	ok := c.document.Call("execCommand", "copy").Bool()
	c.textInput.Set("value", previous)
	c.resetTextInputBuffer()
	return ok
}

func (c *Context) ClipboardText() (string, bool) { return "", false }
