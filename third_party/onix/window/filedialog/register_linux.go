//go:build linux && cgo && !android && !ios && !js && !wasm && !onix_drm

package filedialog

import "github.com/onix-engine/onix/window"

// Register installs the desktop portal implementation for Linux builds.
func Register() {
	window.RegisterFileDialogBackend(open)
}
