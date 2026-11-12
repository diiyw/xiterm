//go:build (onix_vulkan || android) && (windows || linux || android || (darwin && !ios)) && !onix_directx && !onix_metal && !onix_opengl && !onix_webgpu && !onix_webgl

package render

const CompiledBackend Backend = BackendVulkan
