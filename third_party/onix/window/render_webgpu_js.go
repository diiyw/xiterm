//go:build !wechat && js && wasm && !wechat && !onix_directx && !onix_metal && !onix_vulkan && !onix_opengl && !onix_webgl

package window

import (
	"github.com/onix-engine/onix/render"
	"syscall/js"
)

func newBrowserRenderDriver(canvas js.Value, transparent, antialias bool) (render.Driver, error) {
	return render.NewWebGPUDriver(canvas, transparent, antialias)
}
