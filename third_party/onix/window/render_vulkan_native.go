//go:build cgo && !android && !ios && !onix_drm && (onix_vulkan || android) && (windows || linux || android || (darwin && !ios)) && !onix_directx && !onix_metal && !onix_opengl && !onix_webgpu && !onix_webgl

package window

import "github.com/onix-engine/onix/render"

func newDesktopRenderDriver(surface render.NativeSurface) (render.Driver, error) {
	return render.NewVulkanDriver(surface)
}
