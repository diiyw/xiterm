//go:build cgo && !android && !ios && !onix_drm && windows && !onix_metal && !onix_vulkan && !onix_opengl && !onix_webgpu && !onix_webgl

package window

import "github.com/onix-engine/onix/render"

func newDesktopRenderDriver(surface render.NativeSurface) (render.Driver, error) {
	return render.NewDirectXDriver(surface)
}
