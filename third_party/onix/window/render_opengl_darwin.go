//go:build cgo && darwin && !ios && onix_opengl && (windows || linux || android || (darwin && !ios)) && !onix_directx && !onix_metal && !onix_vulkan && !onix_webgpu && !onix_webgl

package window

import "github.com/onix-engine/onix/render"

func newMacRenderDriver(layer, view uintptr, width, height int) (render.Driver, error) {
	return render.NewOpenGLDriver(render.NativeSurface{Window: view, Width: width, Height: height})
}
