//go:build android || ios

package window

func (c *Context) SetClipboardText(string) bool  { return false }
func (c *Context) ClipboardText() (string, bool) { return "", false }
