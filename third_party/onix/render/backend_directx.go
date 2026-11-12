//go:build windows && !onix_metal && !onix_vulkan && !onix_opengl && !onix_webgpu && !onix_webgl

package render

const CompiledBackend Backend = BackendDirectX
