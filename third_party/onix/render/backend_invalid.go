//go:build (onix_directx && onix_metal) || (onix_directx && onix_vulkan) || (onix_directx && onix_opengl) || (onix_directx && onix_webgpu) || (onix_directx && onix_webgl) || (onix_metal && onix_vulkan) || (onix_metal && onix_opengl) || (onix_metal && onix_webgpu) || (onix_metal && onix_webgl) || (onix_vulkan && onix_opengl) || (onix_vulkan && onix_webgpu) || (onix_vulkan && onix_webgl) || (onix_opengl && onix_webgpu) || (onix_opengl && onix_webgl) || (onix_webgpu && onix_webgl) || (onix_directx && !windows) || (onix_metal && !(darwin || ios)) || (onix_vulkan && !(windows || linux || android || (darwin && !ios))) || (onix_opengl && !(windows || linux || android || (darwin && !ios))) || (onix_webgpu && !(js && wasm && !wechat)) || (onix_webgl && !(js && wasm)) || (wechat && (onix_webgpu || onix_directx || onix_metal || onix_vulkan || onix_opengl))

package render

var _ = select_exactly_one_render_backend_supported_by_the_target_platform
