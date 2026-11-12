//go:build !windows && !darwin && !linux && !android && !ios

package font

func platformDefaultFont() (systemFontReference, bool) {
	return systemFontReference{}, false
}
