//go:build cgo && darwin && !ios && (darwin || ios) && !onix_directx && !onix_vulkan && !onix_opengl && !onix_webgpu && !onix_webgl

package window

import "github.com/onix-engine/onix/render"

func newMacRenderDriver(layer, view uintptr, width, height int) (render.Driver, error) {
	return render.NewMetalDriver(render.MetalSurface{Layer: layer, Width: width, Height: height})
}
