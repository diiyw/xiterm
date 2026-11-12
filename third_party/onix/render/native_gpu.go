//go:build cgo && ((windows && !onix_metal && !onix_vulkan && !onix_opengl && !onix_webgpu && !onix_webgl) || ((onix_vulkan || android) && (windows || linux || android || (darwin && !ios)) && !onix_directx && !onix_metal && !onix_opengl && !onix_webgpu && !onix_webgl))

package render

/*
// These entry points consume or copy their Go arguments before returning.
#cgo noescape onix_gpu_begin_frame
#cgo nocallback onix_gpu_begin_frame
#cgo noescape onix_gpu_begin_render_target
#cgo nocallback onix_gpu_begin_render_target
#cgo noescape onix_gpu_create_render_target
#cgo nocallback onix_gpu_create_render_target
#cgo noescape onix_gpu_resize_render_target
#cgo nocallback onix_gpu_resize_render_target
#cgo noescape onix_gpu_draw_sprites
#cgo nocallback onix_gpu_draw_sprites
#cgo noescape onix_gpu_draw_shapes
#cgo nocallback onix_gpu_draw_shapes
#cgo noescape onix_gpu_draw_meshes
#cgo nocallback onix_gpu_draw_meshes
#cgo noescape onix_gpu_draw_mesh_picking
#cgo nocallback onix_gpu_draw_mesh_picking
#cgo noescape onix_gpu_update_textures
#cgo nocallback onix_gpu_update_textures
#cgo noescape onix_gpu_prepare_texture_updates
#cgo nocallback onix_gpu_prepare_texture_updates
#cgo nocallback onix_gpu_commit_texture_updates
#cgo noescape onix_gpu_buffer_memory
#cgo nocallback onix_gpu_buffer_memory
#include "native_gpu.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

type nativeGPUDriver struct {
	driverStatistics
	native     *C.OnixGPUDriver
	deviceName string
	nativeDrawState
	spritePush     C.OnixGPUSpritePush
	shapePush      C.OnixGPUSpritePush
	lighting3D     [meshLightUniformFloatCount]float32
	updates        []C.OnixGPUTextureUpdate
	width          int
	height         int
	viewport       Rect
	viewportSet    bool
	blend          BlendState
	blendSet       bool
	scissor        Rect
	scissorEnabled bool
	scissorSet     bool

	updateRetention scratchRetention
}

func newNativeGPUDriver(surface NativeSurface) (Driver, error) {
	if surface.Kind == 0 || surface.Width <= 0 || surface.Height <= 0 {
		return nil, errors.New("render: invalid GPU surface")
	}
	if surface.Kind == NativeSurfaceDRM {
		if surface.DRMFD < 0 || surface.DRMConnectorID == 0 || surface.Transparent {
			return nil, errors.New("render: invalid GPU DRM surface")
		}
	} else if surface.Window == 0 {
		return nil, errors.New("render: invalid GPU window")
	}
	native := C.onix_gpu_create(
		C.OnixGPUSurface{
			kind:             C.int(surface.Kind),
			display:          surface.Display,
			window:           C.uintptr_t(surface.Window),
			width:            C.uint32_t(surface.Width),
			height:           C.uint32_t(surface.Height),
			transparent:      C.int(boolInt(surface.Transparent)),
			drm_fd:           C.int32_t(surface.DRMFD),
			drm_connector_id: C.uint32_t(surface.DRMConnectorID),
			drm_refresh_rate: C.uint32_t(surface.DRMRefreshRate),
		},
		nativeGPUShader(nativeGPUShaders[2]),
		nativeGPUShader(nativeGPUShaders[3]),
		nativeGPUShader(nativeGPUShaders[4]),
		nativeGPUShader(nativeGPUShaders[5]),
		nativeGPUShader(nativeGPUShaders[6]),
		nativeGPUShader(nativeGPUShaders[7]),
		nativeGPUShader(nativeGPUShaders[8]),
		nativeGPUShader(nativeGPUShaders[9]),
		nativeGPUShader(nativeGPUShaders[10]),
		nativeGPUShader(nativeGPUShaders[11]),
		nativeGPUShader(nativeGPUShaders[12]),
	)
	if native == nil {
		return nil, nativeGPUError("create driver")
	}
	deviceName := C.GoString(C.onix_gpu_device_name(native))
	loadNativePipelineCache(native)
	return &nativeGPUDriver{
		native:      native,
		deviceName:  deviceName,
		width:       surface.Width,
		height:      surface.Height,
		viewport:    Rect{Width: int32(surface.Width), Height: int32(surface.Height)},
		viewportSet: true,
	}, nil
}

func (d *nativeGPUDriver) BufferMemoryStatistics() BufferMemoryStatistics {
	if d.native == nil {
		return BufferMemoryStatistics{}
	}
	var memory C.OnixBufferMemory
	C.onix_gpu_buffer_memory(d.native, &memory)
	return BufferMemoryStatistics{
		InstanceBytes:      uint64(memory.instances),
		StagingBytes:       uint64(memory.staging),
		ReadbackBytes:      uint64(memory.readback),
		NativeScratchBytes: uint64(memory.scratch),
		ScratchBytes: sliceCapacityBytes(d.draws) + sliceCapacityBytes(d.draws3D) +
			sliceCapacityBytes(d.deformationWords) + sliceCapacityBytes(d.updates),
	}
}

func (d *nativeGPUDriver) RendererInfo() RendererInfo {
	return RendererInfo{Backend: CompiledBackend.String(), Device: d.deviceName}
}

func (d *nativeGPUDriver) BeginFrame(clear Color) error {
	values := [4]C.float{C.float(clear.R), C.float(clear.G), C.float(clear.B), C.float(clear.A)}
	if C.onix_gpu_begin_frame(d.native, &values[0]) == 0 {
		return nativeGPUError("begin frame")
	}
	d.updates = (&d.updateRetention).trimScratch(d.updates, 32)
	d.trimScratch()
	d.beginFrameStatistics(d.width, d.height)
	return nil
}

func (d *nativeGPUDriver) SetViewport(viewport Rect) {
	if d.viewportSet && d.viewport == viewport {
		return
	}
	d.viewport = viewport
	d.viewportSet = true
	C.onix_gpu_set_viewport(
		d.native,
		C.int32_t(viewport.X),
		C.int32_t(viewport.Y),
		C.int32_t(viewport.Width),
		C.int32_t(viewport.Height),
	)
}

func (d *nativeGPUDriver) Viewport() Rect { return d.viewport }

func (d *nativeGPUDriver) SetBlend(blend BlendState) error {
	mode, ok := spriteBlendMode(blend)
	if !ok {
		return fmt.Errorf("render: GPU blend state is not compiled: %+v", blend)
	}
	if d.blendSet && d.blend == blend {
		return nil
	}
	d.blend = blend
	d.blendSet = true
	C.onix_gpu_set_sprite_blend(d.native, C.int(mode), C.float(blend.Constant.R), C.float(blend.Constant.G), C.float(blend.Constant.B), C.float(blend.Constant.A))
	return nil
}

func (d *nativeGPUDriver) SetScissor(scissor *Rect) {
	if scissor == nil {
		if d.scissorSet && !d.scissorEnabled {
			return
		}
		d.scissorSet = true
		d.scissorEnabled = false
		C.onix_gpu_set_scissor(d.native, 0, 0, 0, 0, 0)
		return
	}
	if d.scissorSet && d.scissorEnabled && d.scissor == *scissor {
		return
	}
	d.scissor = *scissor
	d.scissorSet = true
	d.scissorEnabled = true
	C.onix_gpu_set_scissor(
		d.native,
		1,
		C.int32_t(scissor.X),
		C.int32_t(scissor.Y),
		C.int32_t(scissor.Width),
		C.int32_t(scissor.Height),
	)
}

func (d *nativeGPUDriver) CreateTexture(descriptor TextureDescriptor, pixels []byte) (TextureHandle, error) {
	if err := descriptor.Validate(pixels); err != nil {
		return TextureHandle{}, err
	}
	// A compressed payload already carries its chain because the device cannot
	// generate mips for block data; an uncompressed payload carries the base
	// level and the driver blits the rest of the chain from it.
	levels := 1
	if descriptor.Format.Compressed() {
		levels = descriptor.MipLevels()
	} else if descriptor.Filter == TextureFilterLinearMipmapLinear {
		levels = MipLevels(descriptor.Width, descriptor.Height)
	}
	handle := C.onix_gpu_create_texture_with_format(
		d.native,
		C.uint32_t(descriptor.Width),
		C.uint32_t(descriptor.Height),
		C.uint32_t(levels),
		C.uint32_t(descriptor.Format),
		C.int(boolInt(descriptor.Filter == TextureFilterLinearMipmapLinear)),
		(*C.uint8_t)(unsafe.Pointer(&pixels[0])),
		C.size_t(len(pixels)),
		C.uint32_t(descriptor.WrapU), C.uint32_t(descriptor.WrapV),
	)
	if handle == 0 {
		return TextureHandle{}, nativeGPUError("create texture")
	}
	d.recordTextureUploadStatistics(1, uint64(len(pixels)))
	return d.textures.Insert(uint32(handle)), nil
}

func (d *nativeGPUDriver) CreateExternalTexture(descriptor ExternalTextureDescriptor) (TextureHandle, error) {
	if err := descriptor.Validate(); err != nil {
		return TextureHandle{}, err
	}
	if descriptor.Kind == ExternalTextureDMABuf {
		handle := C.onix_gpu_create_external_dmabuf_texture(d.native, C.int(descriptor.Handle), C.uint32_t(descriptor.Width), C.uint32_t(descriptor.Height), C.uint32_t(descriptor.Stride), C.uint32_t(descriptor.Offset), C.uint64_t(descriptor.Modifier))
		if handle == 0 {
			return TextureHandle{}, nativeGPUError("import DMA-BUF texture")
		}
		return d.textures.Insert(uint32(handle)), nil
	}
	if descriptor.Kind != ExternalTextureD3D11 || descriptor.Format != ExternalTextureBGRA8 {
		return TextureHandle{}, errors.New("render: GPU external texture requires a D3D11 BGRA surface")
	}
	handle := C.onix_gpu_create_external_d3d11_texture(
		d.native,
		C.uintptr_t(descriptor.Handle),
		C.uint32_t(descriptor.Width),
		C.uint32_t(descriptor.Height),
	)
	if handle == 0 {
		return TextureHandle{}, nativeGPUError("import D3D11 texture")
	}
	return d.textures.Insert(uint32(handle)), nil
}

func (d *nativeGPUDriver) RefreshExternalTexture(handle TextureHandle) error {
	native, ok := d.textures.Get(handle)
	if !ok {
		return fmt.Errorf("render: invalid GPU texture handle %s", handle)
	}
	if C.onix_gpu_refresh_external_texture(d.native, C.uint32_t(native)) == 0 {
		return nativeGPUError("refresh D3D11 texture")
	}
	return nil
}

func (d *nativeGPUDriver) UpdateTextures(updates []TextureUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	d.updates = (&d.updateRetention).resetScratch(d.updates)
	var length int
	var uploadBytes uint64
	for _, update := range updates {
		native, ok := d.textures.Get(update.Texture)
		region := update.Region
		if !ok || region.X < 0 || region.Y < 0 || region.Width <= 0 || region.Height <= 0 ||
			len(update.Pixels) == 0 {
			return errors.New("render: invalid GPU texture update")
		}
		// GPU buffer-to-image offsets must be aligned even for R8 payloads.
		offset := (length + 3) &^ 3
		if offset < length || len(update.Pixels) > int(^uint(0)>>1)-offset {
			return errors.New("render: GPU texture updates are too large")
		}
		length = offset + len(update.Pixels)
		uploadBytes += uint64(len(update.Pixels))
		d.updates = append(d.updates, C.OnixGPUTextureUpdate{
			texture:     C.uint32_t(native),
			x:           C.int32_t(region.X),
			y:           C.int32_t(region.Y),
			width:       C.uint32_t(region.Width),
			height:      C.uint32_t(region.Height),
			data_offset: C.uint64_t(offset),
			data_length: C.uint64_t(len(update.Pixels)),
		})
	}
	if len(updates) == 1 {
		pixels := updates[0].Pixels
		if C.onix_gpu_update_textures(d.native, &d.updates[0], 1, (*C.uint8_t)(unsafe.Pointer(&pixels[0])), C.size_t(len(pixels))) == 0 {
			return nativeGPUError("update textures")
		}
	} else {
		pixels := C.onix_gpu_prepare_texture_updates(d.native, &d.updates[0], C.uint32_t(len(d.updates)), C.size_t(length))
		if pixels == nil {
			return nativeGPUError("prepare texture updates")
		}
		// Only native mapped memory is queued; Go pixel pointers stay in Go.
		staging := unsafe.Slice((*byte)(unsafe.Pointer(pixels)), length)
		for index, update := range updates {
			offset := int(d.updates[index].data_offset)
			copy(staging[offset:offset+len(update.Pixels)], update.Pixels)
		}
		if C.onix_gpu_commit_texture_updates(d.native) == 0 {
			return nativeGPUError("commit texture updates")
		}
	}
	d.recordTextureUploadStatistics(len(updates), uploadBytes)
	return nil
}

func (d *nativeGPUDriver) DestroyTexture(handle TextureHandle) error {
	return (&d.textures).destroyNativeResource(handle, "GPU texture", func(native uint32) error {
		if C.onix_gpu_destroy_texture(d.native, C.uint32_t(native)) == 0 {
			return nativeGPUError("destroy texture")
		}
		return nil
	})
}

func (d *nativeGPUDriver) CreateRenderTarget(descriptor RenderTargetDescriptor) (RenderTargetHandle, TextureHandle, error) {
	if err := descriptor.Validate(); err != nil {
		return RenderTargetHandle{}, TextureHandle{}, err
	}
	var color C.uint32_t
	target := C.onix_gpu_create_render_target(
		d.native,
		C.uint32_t(descriptor.Width),
		C.uint32_t(descriptor.Height),
		C.uint32_t(descriptor.ColorFormat),
		C.int(boolInt(descriptor.Depth)),
		&color,
	)
	if target == 0 || color == 0 {
		return RenderTargetHandle{}, TextureHandle{}, nativeGPUError("create render target")
	}
	colorHandle := d.textures.Insert(uint32(color))
	targetHandle := d.targets.Insert(nativeRenderTarget{native: uint32(target), color: colorHandle})
	return targetHandle, colorHandle, nil
}

func (d *nativeGPUDriver) ResizeRenderTarget(handle RenderTargetHandle, descriptor RenderTargetDescriptor) (TextureHandle, error) {
	if err := descriptor.Validate(); err != nil {
		return TextureHandle{}, err
	}
	target := d.targets.getRef(handle)
	if target == nil {
		return TextureHandle{}, fmt.Errorf("render: invalid GPU render target handle %s", handle)
	}
	var color C.uint32_t
	if C.onix_gpu_resize_render_target(
		d.native,
		C.uint32_t(target.native),
		C.uint32_t(descriptor.Width),
		C.uint32_t(descriptor.Height),
		C.uint32_t(descriptor.ColorFormat),
		C.int(boolInt(descriptor.Depth)),
		&color,
	) == 0 || color == 0 {
		return TextureHandle{}, nativeGPUError("resize render target")
	}
	d.textures.Remove(target.color)
	target.color = d.textures.Insert(uint32(color))
	return target.color, nil
}

func (d *nativeGPUDriver) DestroyRenderTarget(handle RenderTargetHandle) error {
	target := d.targets.getRef(handle)
	if target == nil {
		return fmt.Errorf("render: invalid GPU render target handle %s", handle)
	}
	if C.onix_gpu_destroy_render_target(d.native, C.uint32_t(target.native)) == 0 {
		return nativeGPUError("destroy render target")
	}
	d.textures.Remove(target.color)
	d.targets.Remove(handle)
	return nil
}

func (d *nativeGPUDriver) BeginRenderTarget(handle RenderTargetHandle, clear RenderTargetClear) error {
	target := d.targets.getRef(handle)
	if target == nil {
		return fmt.Errorf("render: invalid GPU render target handle %s", handle)
	}
	color := [4]C.float{C.float(clear.Color.R), C.float(clear.Color.G), C.float(clear.Color.B), C.float(clear.Color.A)}
	if C.onix_gpu_begin_render_target(
		d.native,
		C.uint32_t(target.native),
		&color[0],
		C.float(clear.Depth),
		C.int(boolInt(clear.ClearColor)),
		C.int(boolInt(clear.ClearDepth)),
	) == 0 {
		return nativeGPUError("begin render target")
	}
	return nil
}

func (d *nativeGPUDriver) EndRenderTarget() error {
	if C.onix_gpu_end_render_target(d.native) == 0 {
		return nativeGPUError("end render target")
	}
	return nil
}

func (d *nativeGPUDriver) ReadRenderTarget(handle RenderTargetHandle, region RenderTargetRegion, destination []byte) error {
	target := d.targets.getRef(handle)
	if target == nil {
		return fmt.Errorf("render: invalid GPU render target readback %s", handle)
	}
	if C.onix_gpu_read_render_target(d.native, C.uint32_t(target.native), C.int32_t(region.X), C.int32_t(region.Y), C.uint32_t(region.Width), C.uint32_t(region.Height), (*C.uint8_t)(unsafe.Pointer(&destination[0])), C.size_t(len(destination))) == 0 {
		return nativeGPUError("read render target")
	}
	return nil
}

func (d *nativeGPUDriver) DrawSprites(batch SpriteBatch) error {
	if len(batch.Instances) == 0 || len(batch.Draws) == 0 {
		return nil
	}
	sprites, err := d.packSpriteDraws(&batch, compiledNativeBackend)
	if err != nil {
		return err
	}
	push := &d.spritePush
	copyFloat32ToC(push.projection[:], batch.Projection[:])
	if C.onix_gpu_draw_sprites(
		d.native,
		push,
		(*C.OnixGPUInstance)(unsafe.Pointer(&batch.Instances[0])),
		C.uint32_t(len(batch.Instances)),
		&d.draws[0],
		C.uint32_t(len(d.draws)),
	) == 0 {
		return nativeGPUError("draw sprites")
	}
	d.recordSpriteStatistics(batch, sprites, d.spriteBytes)
	return nil
}

func (d *nativeGPUDriver) CreateMaterial2DShader(descriptor ShaderDescriptor) (Material2DShaderHandle, error) {
	return createNativeGPUShader(descriptor, nativeGPUShaders[0], nativeGPUShaders[1], "Material2D", &d.shaders2D, func(vertex, fragment C.OnixGPUShader) C.uint32_t {
		return C.onix_gpu_create_material2d_shader(d.native, vertex, fragment)
	})
}

func (d *nativeGPUDriver) DestroyMaterial2DShader(handle Material2DShaderHandle) error {
	return (&d.shaders2D).destroyNativeResource(handle, "GPU Material2D shader", func(shader uint32) error {
		if C.onix_gpu_destroy_material2d_shader(d.native, C.uint32_t(shader)) == 0 {
			return nativeGPUError("destroy Material2D shader")
		}
		return nil
	})
}

func createNativeGPUShader[R any](descriptor ShaderDescriptor, defaultVertex, defaultFragment []byte, kind string, pool *GenerationPool[R, uint32], create func(C.OnixGPUShader, C.OnixGPUShader) C.uint32_t) (Handle[R], error) {
	if err := descriptor.Validate(); err != nil {
		return Handle[R]{}, err
	}
	vertex, fragment := descriptor.VulkanVertexSPIRV, descriptor.VulkanFragmentSPIRV
	if CompiledBackend == BackendDirectX {
		vertex, fragment = descriptor.DirectXVertexHLSL, descriptor.DirectXFragmentHLSL
	}
	if len(vertex) == 0 && len(fragment) == 0 {
		return Handle[R]{}, fmt.Errorf("render: %s shader has no %s source", kind, CompiledBackend)
	}
	if len(vertex) == 0 {
		vertex = defaultVertex
	}
	if len(fragment) == 0 {
		fragment = defaultFragment
	}
	shader := create(nativeGPUShader(vertex), nativeGPUShader(fragment))
	if shader == 0 {
		return Handle[R]{}, nativeGPUError("create " + kind + " shader")
	}
	return pool.Insert(uint32(shader)), nil
}

func (d *nativeGPUDriver) DrawShapes(batch ShapeBatch) error {
	if len(batch.Instances) == 0 {
		return nil
	}
	push := &d.shapePush
	copyFloat32ToC(push.projection[:], batch.Projection[:])
	if C.onix_gpu_draw_shapes(
		d.native,
		push,
		(*C.OnixGPUShapeInstance)(unsafe.Pointer(&batch.Instances[0])),
		C.uint32_t(len(batch.Instances)),
	) == 0 {
		return nativeGPUError("draw shapes")
	}
	d.recordShapeStatistics(batch)
	return nil
}

func (d *nativeGPUDriver) CreateMesh(vertices []Vertex3D, indices []uint32) (MeshHandle, error) {
	if len(vertices) == 0 || len(indices) == 0 {
		return MeshHandle{}, errors.New("render: GPU mesh data is empty")
	}
	mesh := C.onix_gpu_create_mesh(
		d.native,
		(*C.OnixGPUVertex3D)(unsafe.Pointer(&vertices[0])), C.uint32_t(len(vertices)),
		(*C.uint32_t)(unsafe.Pointer(&indices[0])), C.uint32_t(len(indices)),
	)
	if mesh == 0 {
		return MeshHandle{}, nativeGPUError("create mesh")
	}
	return d.meshes.Insert(nativeMesh{native: uint32(mesh)}), nil
}

func (d *nativeGPUDriver) DestroyMesh(handle MeshHandle) error {
	mesh := d.meshes.getRef(handle)
	if mesh == nil {
		return fmt.Errorf("render: invalid GPU mesh handle %s", handle)
	}
	if C.onix_gpu_destroy_mesh(d.native, C.uint32_t(mesh.native)) == 0 {
		return nativeGPUError("destroy mesh")
	}
	d.meshes.Remove(handle)
	return nil
}

func (d *nativeGPUDriver) DrawMeshes(batch MeshBatch3D) error {
	return d.drawMeshes(batch, false)
}

func (d *nativeGPUDriver) DrawMeshPicking(batch MeshBatch3D) error {
	if len(batch.Instances) == 0 || len(batch.Draws) == 0 {
		return nil
	}
	err := d.packMeshPickingDraws(&batch, compiledNativeBackend)
	if err != nil {
		return err
	}
	dynamic := len(batch.Vertices) != 0 || len(batch.Indices) != 0
	push := nativeGPUMeshPush(&batch)
	var vertices *C.OnixGPUVertex3D
	var indices *C.uint32_t
	if dynamic {
		vertices = (*C.OnixGPUVertex3D)(unsafe.Pointer(&batch.Vertices[0]))
		indices = (*C.uint32_t)(unsafe.Pointer(&batch.Indices[0]))
	}
	if C.onix_gpu_draw_mesh_picking(
		d.native,
		&push,
		vertices, C.uint32_t(len(batch.Vertices)),
		indices, C.uint32_t(len(batch.Indices)),
		(*C.OnixGPUInstance3D)(unsafe.Pointer(&batch.Instances[0])), C.uint32_t(len(batch.Instances)),
		&d.draws3D[0], C.uint32_t(len(d.draws3D)),
	) == 0 {
		return nativeGPUError("draw mesh picking")
	}
	return nil
}

func nativeGPUMeshPush(batch *MeshBatch3D) C.OnixGPUMeshPush {
	push := C.OnixGPUMeshPush{}
	// Backend vertex shaders own clip-space conversion for the native viewport.
	copyFloat32ToC(push.view_projection[:], batch.ViewProjection[:])
	copyFloat32ToC(push.camera_position[:], batch.CameraPosition[:])
	copyFloat32ToC(push.ambient_light[:], batch.AmbientLight[:])
	copyFloat32ToC(push.light_direction[:], batch.LightDirection[:])
	copyFloat32ToC(push.directional_light[:], batch.DirectionalLight[:])
	return push
}

func (d *nativeGPUDriver) DrawMeshesDeformed(batch MeshBatch3D) error {
	var err error
	d.deformationWords = (&d.deformationRetention).resetScratch(d.deformationWords)
	d.deformationWords, err = packMeshDeformations(d.deformationWords, batch, d)
	if err != nil {
		return fmt.Errorf("render: invalid GPU deformation: %w", err)
	}
	return d.drawMeshes(batch, true)
}

func (d *nativeGPUDriver) drawMeshes(batch MeshBatch3D, gpuDeformation bool) error {
	if len(batch.Instances) == 0 || len(batch.Draws) == 0 {
		return nil
	}
	counts, err := d.packMeshDraws(&batch, compiledNativeBackend, gpuDeformation)
	if err != nil {
		return err
	}
	dynamic := len(batch.Vertices) != 0 || len(batch.Indices) != 0
	push := nativeGPUMeshPush(&batch)
	var vertices *C.OnixGPUVertex3D
	var indices *C.uint32_t
	if dynamic {
		vertices = (*C.OnixGPUVertex3D)(unsafe.Pointer(&batch.Vertices[0]))
		indices = (*C.uint32_t)(unsafe.Pointer(&batch.Indices[0]))
	}
	lighting := d.lighting3D[:]
	copy(lighting[:16], batch.ViewProjection[:])
	writeMeshSceneUniforms(lighting, &batch)
	if C.onix_gpu_draw_meshes(
		d.native,
		&push,
		vertices, C.uint32_t(len(batch.Vertices)),
		indices, C.uint32_t(len(batch.Indices)),
		(*C.OnixGPUInstance3D)(unsafe.Pointer(&batch.Instances[0])), C.uint32_t(len(batch.Instances)),
		(*C.uint32_t)(unsafe.Pointer(&d.deformationWords[0])), C.uint32_t(len(d.deformationWords)),
		&d.draws3D[0], C.uint32_t(len(d.draws3D)),
		(*C.float)(unsafe.Pointer(&lighting[0])),
	) == 0 {
		return nativeGPUError("draw meshes")
	}
	d.recordMeshStatistics(batch, counts)
	return nil
}

func (d *nativeGPUDriver) CreateMeshShader(descriptor ShaderDescriptor) (MeshShaderHandle, error) {
	return createNativeGPUShader(descriptor, nativeGPUShaders[6], nativeGPUShaders[7], "mesh", &d.shaders, func(vertex, fragment C.OnixGPUShader) C.uint32_t {
		return C.onix_gpu_create_mesh_shader(d.native, vertex, fragment)
	})
}

func (d *nativeGPUDriver) DestroyMeshShader(handle MeshShaderHandle) error {
	return (&d.shaders).destroyNativeResource(handle, "GPU mesh shader", func(shader uint32) error {
		if C.onix_gpu_destroy_mesh_shader(d.native, C.uint32_t(shader)) == 0 {
			return nativeGPUError("destroy mesh shader")
		}
		return nil
	})
}

func (d *nativeGPUDriver) Present() error {
	if C.onix_gpu_present(d.native) == 0 {
		return nativeGPUError("present")
	}
	return nil
}

func (d *nativeGPUDriver) ResizeSurface(width, height int) error {
	if width <= 0 || height <= 0 || (d.width == width && d.height == height) {
		return nil
	}
	if C.onix_gpu_resize(d.native, C.uint32_t(width), C.uint32_t(height)) == 0 {
		return nativeGPUError("resize surface")
	}
	d.width, d.height = width, height
	d.viewportSet = false
	return nil
}

func (d *nativeGPUDriver) SetVSync(enabled bool) {
	value := C.int(0)
	if enabled {
		value = 1
	}
	C.onix_gpu_set_vsync(d.native, value)
}

func (d *nativeGPUDriver) SetSurfaceCornerRadius(radius int) {
	C.onix_gpu_set_corner_radius(d.native, C.int(radius))
}

func (d *nativeGPUDriver) Destroy() {
	if d.native != nil {
		saveNativePipelineCache(d.native)
		C.onix_gpu_destroy(d.native)
		d.native = nil
		d.textures.Clear()
		d.targets.Clear()
		d.meshes.Clear()
		d.shaders.Clear()
		d.shaders2D.Clear()
	}
}

func nativeGPUShader(data []byte) C.OnixGPUShader {
	return C.OnixGPUShader{
		code: (*C.uint32_t)(unsafe.Pointer(&data[0])),
		size: C.size_t(len(data)),
	}
}

func nativeGPUError(operation string) error {
	message := C.GoString(C.onix_gpu_last_error())
	if message == "" {
		message = "unknown GPU error"
	}
	return fmt.Errorf("render: %s: %s", operation, message)
}

func (d *nativeGPUDriver) deformationLayout(handle MeshHandle) (meshDeformationLayout, bool) {
	mesh := d.meshes.getRef(handle)
	if mesh == nil {
		return meshDeformationLayout{}, false
	}
	return mesh.deformation, mesh.deformation.vertexCount != 0
}

func (d *nativeGPUDriver) SetMeshDeformation(handle MeshHandle, descriptor MeshDeformationDescriptor) error {
	mesh := d.meshes.getRef(handle)
	if mesh == nil {
		return fmt.Errorf("render: invalid mesh handle %s", handle)
	}
	scratch := meshDeformationBuffers.get()
	defer meshDeformationBuffers.put(scratch)
	words, layout, err := packMeshDeformationStatic(scratch.data, descriptor)
	scratch.data = words
	if err != nil {
		return err
	}
	if C.onix_gpu_set_mesh_deformation(d.native, C.uint32_t(mesh.native), (*C.uint32_t)(unsafe.Pointer(&words[0])), C.uint32_t(len(words)), C.uint32_t(descriptor.VertexCount)) == 0 {
		return nativeGPUError("upload mesh deformation")
	}
	mesh.deformation = layout
	return nil
}
