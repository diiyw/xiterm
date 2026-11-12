//go:build cgo && (onix_vulkan || android) && (windows || linux || android || (darwin && !ios)) && !onix_directx && !onix_metal && !onix_opengl && !onix_webgpu && !onix_webgl

package render

/*
#cgo windows LDFLAGS: -static -lole32 -luuid
#cgo linux,!android LDFLAGS: -l:libvulkan.so.1
#cgo linux,onix_drm CFLAGS: -DONIX_VK_DRM
#cgo android LDFLAGS: -lvulkan -landroid
#cgo darwin LDFLAGS: -lMoltenVK -framework Foundation -framework QuartzCore -framework Metal -lc++
#include "native_gpu.h"
*/
import "C"
import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"
)

//go:embed shaders/vulkan_sprite.vert.spv
var vulkanSpriteVertex []byte

//go:embed shaders/vulkan_sprite.frag.spv
var vulkanSpriteFragment []byte

//go:embed shaders/vulkan_sprite_multi.vert.spv
var vulkanSpriteMultiVertex []byte

//go:embed shaders/vulkan_sprite_multi.frag.spv
var vulkanSpriteMultiFragment []byte

//go:embed shaders/vulkan_shape.vert.spv
var vulkanShapeVertex []byte

//go:embed shaders/vulkan_shape.frag.spv
var vulkanShapeFragment []byte

//go:embed shaders/vulkan_mesh.vert.spv
var vulkanMeshVertex []byte

//go:embed shaders/vulkan_mesh.frag.spv
var vulkanMeshFragment []byte

//go:embed shaders/vulkan_mesh_unlit.frag.spv
var vulkanMeshUnlitFragment []byte

//go:embed shaders/vulkan_mesh_depth.frag.spv
var vulkanMeshDepthFragment []byte

//go:embed shaders/vulkan_mesh_basic.frag.spv
var vulkanMeshBasicFragment []byte

//go:embed shaders/vulkan_picking.vert.spv
var vulkanPickingVertex []byte

//go:embed shaders/vulkan_picking.frag.spv
var vulkanPickingFragment []byte

var nativeGPUShaders = [13][]byte{vulkanSpriteVertex, vulkanSpriteFragment, vulkanSpriteMultiVertex, vulkanSpriteMultiFragment, vulkanShapeVertex, vulkanShapeFragment, vulkanMeshVertex, vulkanMeshFragment, vulkanMeshUnlitFragment, vulkanMeshDepthFragment, vulkanMeshBasicFragment, vulkanPickingVertex, vulkanPickingFragment}

const compiledNativeBackend = nativeVulkan

func NewVulkanDriver(surface NativeSurface) (Driver, error) { return newNativeGPUDriver(surface) }

const vulkanPipelineCacheLimit = 32 << 20

func vulkanPipelineCachePath(native *C.OnixGPUDriver) string {
	directory, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	uuid := unsafe.Slice((*byte)(unsafe.Pointer(C.onix_gpu_pipeline_cache_uuid(native))), 16)
	return filepath.Join(directory, "onix", "vulkan", fmt.Sprintf("%x.bin", uuid))
}

func loadNativePipelineCache(native *C.OnixGPUDriver) {
	path := vulkanPipelineCachePath(native)
	info, err := os.Stat(path)
	if err != nil || info.Size() < sha256.Size+32 || info.Size() > vulkanPipelineCacheLimit+sha256.Size {
		return
	}
	data, err := os.ReadFile(path)
	if err == nil && len(data) >= sha256.Size+32 && len(data) <= vulkanPipelineCacheLimit+sha256.Size && sha256.Sum256(data[sha256.Size:]) == [sha256.Size]byte(data[:sha256.Size]) {
		C.onix_gpu_load_pipeline_cache(native, unsafe.Pointer(&data[sha256.Size]), C.size_t(len(data)-sha256.Size))
	}
}

func saveNativePipelineCache(native *C.OnixGPUDriver) {
	path := vulkanPipelineCachePath(native)
	if path == "" {
		return
	}
	size := C.onix_gpu_pipeline_cache_data(native, nil, 0)
	if size < 32 || size > vulkanPipelineCacheLimit {
		return
	}
	data := make([]byte, sha256.Size+int(size))
	size = C.onix_gpu_pipeline_cache_data(native, unsafe.Pointer(&data[sha256.Size]), size)
	if size == 0 || os.MkdirAll(filepath.Dir(path), 0700) != nil {
		return
	}
	file, err := os.CreateTemp(filepath.Dir(path), "pipeline-*")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	data = data[:sha256.Size+int(size)]
	digest := sha256.Sum256(data[sha256.Size:])
	copy(data[:sha256.Size], digest[:])
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr == nil && closeErr == nil {
		_ = os.Rename(file.Name(), path)
	}
}
