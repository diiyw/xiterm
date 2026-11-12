//go:build cgo && darwin && !ios && (onix_vulkan || android) && (windows || linux || android || (darwin && !ios)) && !onix_directx && !onix_metal && !onix_opengl && !onix_webgpu && !onix_webgl

package window

import "github.com/onix-engine/onix/render"

func newMacRenderDriver(layer, view uintptr, width, height int) (render.Driver, error) {
	return render.NewVulkanDriver(render.NativeSurface{Kind: render.NativeSurfaceMetal, Window: layer, Width: width, Height: height})
}
