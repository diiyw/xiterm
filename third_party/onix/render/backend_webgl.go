//go:build js && wasm && (wechat || onix_webgl) && !onix_directx && !onix_metal && !onix_vulkan && !onix_opengl && !onix_webgpu

package render

const CompiledBackend Backend = BackendWebGL
