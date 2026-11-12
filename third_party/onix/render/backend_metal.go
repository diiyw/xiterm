//go:build (darwin || ios) && !onix_directx && !onix_vulkan && !onix_opengl && !onix_webgpu && !onix_webgl

package render

const CompiledBackend Backend = BackendMetal
