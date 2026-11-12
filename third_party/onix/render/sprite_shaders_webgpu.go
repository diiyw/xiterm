//go:build js && wasm && !wechat && !onix_directx && !onix_metal && !onix_vulkan && !onix_opengl && !onix_webgl

package render

import _ "embed"

//go:embed shaders/sprite_multi.wgsl
var spriteMultiWGSL string
