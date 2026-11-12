//go:build !linux || !cgo || android || ios || js || wasm || onix_drm

package filedialog

import "github.com/onix-engine/onix/window"

func open(*window.Context, window.FileDialogOptions) ([]string, error) {
	return nil, window.ErrFileDialogUnavailable
}
