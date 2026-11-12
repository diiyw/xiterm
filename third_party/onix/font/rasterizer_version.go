package font

// RasterizerVersion identifies the built-in pure Go rasterizer in visual
// reports. It does not require cgo or a system font library.
func RasterizerVersion() string {
	return "Onix pure Go outline rasterizer; unhinted; Skia 4x LCD filter"
}
