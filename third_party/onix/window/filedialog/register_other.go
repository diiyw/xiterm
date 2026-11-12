//go:build !linux || !cgo || android || ios || js || wasm || onix_drm

package filedialog

// Register is a no-op on targets whose core window package already owns the
// native implementation or has no synchronous file dialog.
func Register() {}
