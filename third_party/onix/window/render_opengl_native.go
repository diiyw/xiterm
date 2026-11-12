//go:build cgo && (onix_opengl || (linux && !android && !onix_vulkan)) && !android && !ios && !onix_drm && (windows || linux)

package window

import "github.com/onix-engine/onix/render"

func newDesktopRenderDriver(surface render.NativeSurface) (render.Driver, error) {
	return render.NewOpenGLDriver(surface)
}
