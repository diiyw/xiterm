//go:build android || ios || js || (linux && (!cgo || onix_drm))

package window

func openNativeFileDialog(_ *Context, _ FileDialogOptions) ([]string, error) {
	return nil, ErrFileDialogUnavailable
}
