//go:build (darwin || ios || windows || linux || android) && cgo

package render

/*
#include "native_draw.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

type nativeBackend uint8

const (
	nativeMetal nativeBackend = iota
	nativeVulkan
	nativeDirectX
)

func (backend nativeBackend) name() string {
	return [...]string{"Metal", "Vulkan", "DirectX"}[backend]
}

type nativeMeshUniforms = C.OnixNativeMeshUniforms

func nativeMeshUniformData(uniforms *nativeMeshUniforms) []float32 {
	return unsafe.Slice((*float32)(unsafe.Pointer(uniforms)), meshLightUniformFloatCount)
}

// Native records are pointer-free and retained between draws. Sharing their C
// definitions lets both backends pass the packed storage without conversion.
type nativeDrawState struct {
	draws                []C.OnixNativeSpriteDraw
	draws3D              []C.OnixNativeMeshDraw3D
	deformationWords     []uint32
	textures             GenerationPool[TextureResource, uint32]
	targets              GenerationPool[RenderTargetResource, nativeRenderTarget]
	meshes               GenerationPool[MeshResource, nativeMesh]
	shaders              GenerationPool[MeshShaderResource, uint32]
	shaders2D            GenerationPool[Material2DShaderResource, uint32]
	drawRetention        scratchRetention
	meshDrawRetention    scratchRetention
	deformationRetention scratchRetention
	spriteBytes          uint64
}

func (d *nativeDrawState) trimScratch() {
	d.draws = (&d.drawRetention).trimScratch(d.draws, 64)
	d.draws3D = (&d.meshDrawRetention).trimScratch(d.draws3D, 64)
	d.deformationWords = (&d.deformationRetention).trimScratch(d.deformationWords, 1024)
}

func (d *nativeDrawState) packSpriteDraws(batch *SpriteBatch, backend nativeBackend) (uint64, error) {
	d.draws = (&d.drawRetention).resetScratch(d.draws)
	d.spriteBytes = 0
	var sprites uint64
	for _, draw := range batch.Draws {
		sprites += uint64(draw.InstanceCount)
		d.spriteBytes = spriteInstanceOffset(d.spriteBytes, draw) + uint64(draw.InstanceCount)*uint64(spriteInstanceStride(draw.Shader.Valid()))
		shader := uint32(0)
		if draw.Shader.Valid() {
			var ok bool
			shader, ok = d.shaders2D.Get(draw.Shader)
			if !ok {
				return 0, nativeResourceError(backend, "render: invalid %s Material2D shader handle %s", draw.Shader)
			}
		}
		texture, ok := d.textures.Get(draw.Texture)
		if !ok {
			return 0, nativeResourceError(backend, "render: invalid %s texture handle %s", draw.Texture)
		}
		nativeDraw := C.OnixNativeSpriteDraw{
			shader:         C.uint32_t(shader),
			texture:        C.uint32_t(texture),
			first_instance: C.uint32_t(draw.FirstInstance),
			instance_count: C.uint32_t(draw.InstanceCount),
		}
		for slot := 1; slot < SpriteTextureSlots; slot++ {
			value, ok := d.textures.Get(draw.TextureAt(slot))
			if !ok {
				return 0, nativeResourceError(backend, "render: invalid %s sprite texture %s", draw.TextureAt(slot))
			}
			nativeDraw.additional_textures[slot-1] = C.uint32_t(value)
		}
		d.draws = append(d.draws, nativeDraw)
	}
	return sprites, nil
}

func (d *nativeDrawState) packMeshDraws(batch *MeshBatch3D, backend nativeBackend, gpuDeformation bool) (meshDrawStatistics, error) {
	var counts meshDrawStatistics
	dynamic := len(batch.Vertices) != 0 || len(batch.Indices) != 0
	if dynamic && (len(batch.Vertices) == 0 || len(batch.Indices) == 0) {
		return counts, nativeGeometryError(backend, "render: incomplete %s dynamic mesh data")
	}
	d.draws3D = (&d.meshDrawRetention).resetScratch(d.draws3D)
	if !gpuDeformation {
		d.deformationWords = (&d.deformationRetention).resetScratch(d.deformationWords)
	}
	if len(d.deformationWords) == 0 {
		d.deformationWords = append(d.deformationWords, 0)
	}
	for _, draw := range batch.Draws {
		counts.add(draw)
		deformationIndex := uint32(0)
		if gpuDeformation {
			deformationIndex = draw.DeformationIndex
		}
		mesh := uint32(0)
		if draw.Mesh.Valid() {
			resource := d.meshes.getRef(draw.Mesh)
			if resource == nil {
				return counts, nativeResourceError(backend, "render: invalid %s mesh handle %s", draw.Mesh)
			}
			mesh = resource.native
		} else if !dynamic {
			return counts, nativeGeometryError(backend, "render: %s mesh draw has no geometry")
		}
		shader := uint32(0)
		if draw.Shader.Valid() {
			var ok bool
			shader, ok = d.shaders.Get(draw.Shader)
			if !ok {
				return counts, nativeResourceError(backend, "render: invalid %s mesh shader handle %s", draw.Shader)
			}
		}
		texture, ok := d.textures.Get(draw.Texture)
		if !ok {
			return counts, nativeResourceError(backend, "render: invalid %s texture handle %s", draw.Texture)
		}
		normalTexture := uint32(0)
		if draw.NormalTexture.Valid() {
			value, exists := d.textures.Get(draw.NormalTexture)
			if !exists {
				return counts, nativeResourceError(backend, "render: invalid %s normal texture handle %s", draw.NormalTexture)
			}
			normalTexture = uint32(value)
		}
		metallicRoughnessTexture := uint32(0)
		if draw.MetallicRoughnessTexture.Valid() {
			value, exists := d.textures.Get(draw.MetallicRoughnessTexture)
			if !exists {
				return counts, nativeResourceError(backend, "render: invalid %s metallic roughness texture handle %s", draw.MetallicRoughnessTexture)
			}
			metallicRoughnessTexture = uint32(value)
		}
		var extraTextures [MeshExtraTextureSlots]C.uint32_t
		for slot, handle := range draw.ExtraTextures(&batch.Lighting) {
			if handle.Valid() {
				value, ok := d.textures.Get(handle)
				if !ok {
					return counts, fmt.Errorf("render: invalid surface texture %s", handle)
				}
				extraTextures[slot] = C.uint32_t(value)
			}
		}
		d.draws3D = append(d.draws3D, C.OnixNativeMeshDraw3D{
			extra_textures:             extraTextures,
			mesh:                       C.uint32_t(mesh),
			shader:                     C.uint32_t(shader),
			texture:                    C.uint32_t(texture),
			normal_texture:             C.uint32_t(normalTexture),
			metallic_roughness_texture: C.uint32_t(metallicRoughnessTexture),
			first_index:                C.uint32_t(draw.FirstIndex),
			index_count:                C.uint32_t(draw.IndexCount),
			base_vertex:                C.int32_t(draw.BaseVertex),
			first_instance:             C.uint32_t(draw.FirstInstance),
			instance_count:             C.uint32_t(draw.InstanceCount),
			alpha_mode:                 C.uint32_t(draw.AlphaMode),
			blend_mode:                 C.uint32_t(draw.BlendMode),
			deformation_index:          C.uint32_t(deformationIndex),
		})
	}
	return counts, nil
}

func (d *nativeDrawState) packMeshPickingDraws(batch *MeshBatch3D, backend nativeBackend) error {
	dynamic := len(batch.Vertices) != 0 || len(batch.Indices) != 0
	if dynamic && (len(batch.Vertices) == 0 || len(batch.Indices) == 0) {
		return nativeGeometryError(backend, "render: incomplete %s dynamic mesh picking data")
	}
	d.draws3D = (&d.meshDrawRetention).resetScratch(d.draws3D)
	for _, draw := range batch.Draws {
		mesh := uint32(0)
		if draw.Mesh.Valid() {
			resource := d.meshes.getRef(draw.Mesh)
			if resource == nil {
				return nativeResourceError(backend, "render: invalid %s mesh handle %s", draw.Mesh)
			}
			mesh = resource.native
		} else if !dynamic {
			return nativeGeometryError(backend, "render: %s mesh picking draw has no geometry")
		}
		d.draws3D = append(d.draws3D, C.OnixNativeMeshDraw3D{
			mesh:           C.uint32_t(mesh),
			first_index:    C.uint32_t(draw.FirstIndex),
			index_count:    C.uint32_t(draw.IndexCount),
			base_vertex:    C.int32_t(draw.BaseVertex),
			first_instance: C.uint32_t(draw.FirstInstance),
			instance_count: C.uint32_t(draw.InstanceCount),
		})
	}
	return nil
}

// Keep error formatting outside the draw loops so it cannot enlarge their
// stack frames or instruction footprint on successful submissions.
//
//go:noinline
func nativeResourceError[R any](backend nativeBackend, format string, handle Handle[R]) error {
	return fmt.Errorf(format, backend.name(), handle)
}

//go:noinline
func nativeGeometryError(backend nativeBackend, format string) error {
	return fmt.Errorf(format, backend.name())
}
