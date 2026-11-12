//go:build cgo && windows && !onix_metal && !onix_vulkan && !onix_opengl && !onix_webgpu && !onix_webgl

package render

/*
#cgo LDFLAGS: -ld3d11 -ldxgi -ld3dcompiler -ldcomp -lole32 -luuid
#include "native_gpu.h"
*/
import "C"
import _ "embed"

//go:embed shaders/directx_sprite.vert.cso
var directXSpriteVertex []byte

//go:embed shaders/directx_sprite.frag.cso
var directXSpriteFragment []byte

//go:embed shaders/directx_sprite_multi.vert.cso
var directXSpriteMultiVertex []byte

//go:embed shaders/directx_sprite_multi.frag.cso
var directXSpriteMultiFragment []byte

//go:embed shaders/directx_shape.vert.cso
var directXShapeVertex []byte

//go:embed shaders/directx_shape.frag.cso
var directXShapeFragment []byte

//go:embed shaders/directx_mesh.vert.cso
var directXMeshVertex []byte

//go:embed shaders/directx_mesh.frag.cso
var directXMeshFragment []byte

//go:embed shaders/directx_mesh_unlit.frag.cso
var directXMeshUnlitFragment []byte

//go:embed shaders/directx_mesh_depth.frag.cso
var directXMeshDepthFragment []byte

//go:embed shaders/directx_mesh_basic.frag.cso
var directXMeshBasicFragment []byte

//go:embed shaders/directx_picking.vert.cso
var directXPickingVertex []byte

//go:embed shaders/directx_picking.frag.cso
var directXPickingFragment []byte

var nativeGPUShaders = [13][]byte{directXSpriteVertex, directXSpriteFragment, directXSpriteMultiVertex, directXSpriteMultiFragment, directXShapeVertex, directXShapeFragment, directXMeshVertex, directXMeshFragment, directXMeshUnlitFragment, directXMeshDepthFragment, directXMeshBasicFragment, directXPickingVertex, directXPickingFragment}

const compiledNativeBackend = nativeDirectX

func NewDirectXDriver(surface NativeSurface) (Driver, error) { return newNativeGPUDriver(surface) }
func loadNativePipelineCache(native *C.OnixGPUDriver)        {}
func saveNativePipelineCache(native *C.OnixGPUDriver)        {}
