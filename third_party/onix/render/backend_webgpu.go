//go:build js && wasm && !wechat && !onix_directx && !onix_metal && !onix_vulkan && !onix_opengl && !onix_webgl

package render

const CompiledBackend Backend = BackendWebGPU
