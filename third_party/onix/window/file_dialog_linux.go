//go:build linux && cgo && !android && !ios && !js && !wasm && !onix_drm

package window

func openNativeFileDialog(_ *Context, _ FileDialogOptions) ([]string, error) {
	return nil, ErrFileDialogUnavailable
}
