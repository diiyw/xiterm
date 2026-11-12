//go:build !wechat && js && wasm && (wechat || onix_webgl) && !onix_directx && !onix_metal && !onix_vulkan && !onix_opengl && !onix_webgpu

package window

import (
	"github.com/onix-engine/onix/render"
	"syscall/js"
)

func newBrowserRenderDriver(canvas js.Value, transparent, antialias bool) (render.Driver, error) {
	return render.NewWebGL2Driver(canvas.Call("getContext", "webgl2", map[string]any{
		"alpha": transparent, "antialias": antialias, "premultipliedAlpha": true,
	}))
}
