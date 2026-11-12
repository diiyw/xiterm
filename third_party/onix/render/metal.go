//go:build cgo && (darwin || ios) && !onix_directx && !onix_vulkan && !onix_opengl && !onix_webgpu && !onix_webgl

package render

/*
#cgo darwin LDFLAGS: -framework Foundation -framework Metal -framework QuartzCore -framework CoreVideo -framework IOSurface
#cgo ios LDFLAGS: -framework Foundation -framework Metal -framework QuartzCore -framework CoreVideo -framework IOSurface
// These entry points consume or copy their Go arguments before returning.
#cgo noescape onix_metal_begin_frame
#cgo nocallback onix_metal_begin_frame
#cgo noescape onix_metal_begin_render_target
#cgo nocallback onix_metal_begin_render_target
#cgo noescape onix_metal_create_render_target
#cgo nocallback onix_metal_create_render_target
#cgo noescape onix_metal_resize_render_target
#cgo nocallback onix_metal_resize_render_target
#cgo noescape onix_metal_draw_sprites
#cgo nocallback onix_metal_draw_sprites
#cgo noescape onix_metal_draw_shapes
#cgo nocallback onix_metal_draw_shapes
#cgo noescape onix_metal_draw_meshes
#cgo nocallback onix_metal_draw_meshes
#cgo noescape onix_metal_draw_mesh_picking
#cgo nocallback onix_metal_draw_mesh_picking
#cgo noescape onix_metal_update_textures
#cgo nocallback onix_metal_update_textures
#cgo noescape onix_metal_buffer_memory
#cgo nocallback onix_metal_buffer_memory
#include "metal_native.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

type MetalSurface struct {
	Layer  uintptr
	Width  int
	Height int
}

type metalDriver struct {
	driverStatistics
	native     *C.OnixMetalDriver
	deviceName string
	nativeDrawState
	spriteUniforms C.OnixMetalSpriteUniforms
	updates        []C.OnixMetalTextureUpdate
	textureUpload  nativeTextureUpload
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

func NewMetalDriver(surface MetalSurface) (Driver, error) {
	if surface.Layer == 0 || surface.Width <= 0 || surface.Height <= 0 {
		return nil, errors.New("render: invalid CAMetalLayer surface")
	}
	native := C.onix_metal_create(
		unsafe.Pointer(surface.Layer),
		C.uint32_t(surface.Width),
		C.uint32_t(surface.Height),
	)
	if native == nil {
		return nil, metalError("create driver")
	}
	return &metalDriver{
		native:      native,
		deviceName:  C.GoString(C.onix_metal_device_name(native)),
		width:       surface.Width,
		height:      surface.Height,
		viewport:    Rect{Width: int32(surface.Width), Height: int32(surface.Height)},
		viewportSet: true,
	}, nil
}

func (d *metalDriver) BufferMemoryStatistics() BufferMemoryStatistics {
	if d.native == nil {
		return BufferMemoryStatistics{}
	}
	var memory C.OnixBufferMemory
	C.onix_metal_buffer_memory(d.native, &memory)
	return BufferMemoryStatistics{
		InstanceBytes:      uint64(memory.instances),
		StagingBytes:       uint64(memory.staging),
		ReadbackBytes:      uint64(memory.readback),
		NativeScratchBytes: uint64(memory.scratch) + uint64(d.textureUpload.capacity),
		ScratchBytes: sliceCapacityBytes(d.draws) + sliceCapacityBytes(d.draws3D) +
			sliceCapacityBytes(d.deformationWords) + sliceCapacityBytes(d.updates),
	}
}

func (d *metalDriver) TrimIdleBuffers() error {
	if C.onix_metal_trim_idle(d.native) == 0 {
		return metalError("trim idle buffers")
	}
	d.textureUpload.release()
	d.draws = nil
	d.draws3D = nil
	d.deformationWords = nil
	d.updates = nil
	return nil
}

func (d *metalDriver) RendererInfo() RendererInfo {
	return RendererInfo{Backend: "Metal", Device: d.deviceName}
}

func (d *metalDriver) BeginFrame(clear Color) error {
	values := [4]C.float{C.float(clear.R), C.float(clear.G), C.float(clear.B), C.float(clear.A)}
	if C.onix_metal_begin_frame(d.native, &values[0]) == 0 {
		return metalError("begin frame")
	}
	d.textureUpload.trim()
	d.updates = (&d.updateRetention).trimScratch(d.updates, 32)
	d.trimScratch()
	d.beginFrameStatistics(d.width, d.height)
	return nil
}

func (d *metalDriver) SetViewport(viewport Rect) {
	if d.viewportSet && d.viewport == viewport {
		return
	}
	d.viewport = viewport
	d.viewportSet = true
	C.onix_metal_set_viewport(
		d.native,
		C.int32_t(viewport.X),
		C.int32_t(viewport.Y),
		C.int32_t(viewport.Width),
		C.int32_t(viewport.Height),
	)
}

func (d *metalDriver) Viewport() Rect { return d.viewport }

func (d *metalDriver) SetBlend(blend BlendState) error {
	mode, ok := spriteBlendMode(blend)
	if !ok {
		return fmt.Errorf("render: Metal blend state is not compiled: %+v", blend)
	}
	if d.blendSet && d.blend == blend {
		return nil
	}
	d.blend = blend
	d.blendSet = true
	C.onix_metal_set_sprite_blend(d.native, C.int(mode), C.float(blend.Constant.R), C.float(blend.Constant.G), C.float(blend.Constant.B), C.float(blend.Constant.A))
	return nil
}

func (d *metalDriver) SetScissor(scissor *Rect) {
	if scissor == nil {
		if d.scissorSet && !d.scissorEnabled {
			return
		}
		d.scissorSet = true
		d.scissorEnabled = false
		C.onix_metal_set_scissor(d.native, 0, 0, 0, 0, 0)
		return
	}
	if d.scissorSet && d.scissorEnabled && d.scissor == *scissor {
		return
	}
	d.scissor = *scissor
	d.scissorSet = true
	d.scissorEnabled = true
	C.onix_metal_set_scissor(
		d.native,
		1,
		C.int32_t(scissor.X),
		C.int32_t(scissor.Y),
		C.int32_t(scissor.Width),
		C.int32_t(scissor.Height),
	)
}

func (d *metalDriver) CreateTexture(descriptor TextureDescriptor, pixels []byte) (TextureHandle, error) {
	if err := descriptor.Validate(pixels); err != nil {
		return TextureHandle{}, err
	}
	levels := 1
	if descriptor.Format.Compressed() {
		levels = descriptor.MipLevels()
	} else if descriptor.Filter == TextureFilterLinearMipmapLinear {
		levels = MipLevels(descriptor.Width, descriptor.Height)
	}
	handle := C.onix_metal_create_texture_with_format(
		d.native,
		C.uint32_t(descriptor.Width),
		C.uint32_t(descriptor.Height),
		C.uint32_t(levels),
		C.uint32_t(descriptor.Format),
		C.int(metalBoolInt(descriptor.Filter == TextureFilterLinearMipmapLinear)),
		(*C.uint8_t)(unsafe.Pointer(&pixels[0])),
		C.size_t(len(pixels)),
		C.uint32_t(descriptor.WrapU), C.uint32_t(descriptor.WrapV),
	)
	if handle == 0 {
		return TextureHandle{}, metalError("create texture")
	}
	d.recordTextureUploadStatistics(1, uint64(len(pixels)))
	return d.textures.Insert(uint32(handle)), nil
}

func (d *metalDriver) CreateExternalTexture(descriptor ExternalTextureDescriptor) (TextureHandle, error) {
	if err := descriptor.Validate(); err != nil {
		return TextureHandle{}, err
	}
	if descriptor.Kind != ExternalTextureMetal || descriptor.Format != ExternalTextureBGRA8 {
		return TextureHandle{}, errors.New("render: Metal external texture requires a CoreVideo BGRA surface")
	}
	handle := C.onix_metal_create_external_cv_texture(
		d.native, C.uintptr_t(descriptor.Handle), C.uint32_t(descriptor.Width), C.uint32_t(descriptor.Height),
	)
	if handle == 0 {
		return TextureHandle{}, metalError("import CoreVideo texture")
	}
	return d.textures.Insert(uint32(handle)), nil
}

func (d *metalDriver) RefreshExternalTexture(handle TextureHandle) error {
	native, ok := d.textures.Get(handle)
	if !ok {
		return fmt.Errorf("render: invalid Metal texture handle %s", handle)
	}
	if C.onix_metal_refresh_external_texture(d.native, C.uint32_t(native)) == 0 {
		return metalError("refresh CoreVideo texture")
	}
	return nil
}

func (d *metalDriver) UpdateTextures(updates []TextureUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	d.updates = (&d.updateRetention).resetScratch(d.updates)
	pixels := updates[0].Pixels
	var uploadBytes uint64
	var length int
	for _, update := range updates {
		native, ok := d.textures.Get(update.Texture)
		region := update.Region
		if !ok || region.X < 0 || region.Y < 0 || region.Width <= 0 || region.Height <= 0 ||
			len(update.Pixels) == 0 {
			return errors.New("render: invalid Metal texture update")
		}
		offset := length
		if len(update.Pixels) > int(^uint(0)>>1)-length {
			return errors.New("render: texture updates are too large")
		}
		length += len(update.Pixels)
		uploadBytes += uint64(len(update.Pixels))
		d.updates = append(d.updates, C.OnixMetalTextureUpdate{
			texture:     C.uint32_t(native),
			x:           C.int32_t(region.X),
			y:           C.int32_t(region.Y),
			width:       C.uint32_t(region.Width),
			height:      C.uint32_t(region.Height),
			data_offset: C.uint64_t(offset),
			data_length: C.uint64_t(len(update.Pixels)),
		})
	}
	if len(updates) > 1 {
		var ok bool
		pixels, ok = d.textureUpload.reserve(length)
		if !ok {
			return errors.New("render: Metal texture upload storage is unavailable")
		}
		for index, update := range updates {
			offset := int(d.updates[index].data_offset)
			copy(pixels[offset:offset+len(update.Pixels)], update.Pixels)
		}
	}
	if C.onix_metal_update_textures(
		d.native,
		&d.updates[0],
		C.uint32_t(len(d.updates)),
		(*C.uint8_t)(unsafe.Pointer(&pixels[0])),
		C.size_t(len(pixels)),
	) == 0 {
		return metalError("update textures")
	}
	d.recordTextureUploadStatistics(len(updates), uploadBytes)
	return nil
}

func (d *metalDriver) DestroyTexture(handle TextureHandle) error {
	return (&d.textures).destroyNativeResource(handle, "Metal texture", func(native uint32) error {
		if C.onix_metal_destroy_texture(d.native, C.uint32_t(native)) == 0 {
			return metalError("destroy texture")
		}
		return nil
	})
}

func (d *metalDriver) CreateRenderTarget(descriptor RenderTargetDescriptor) (RenderTargetHandle, TextureHandle, error) {
	if err := descriptor.Validate(); err != nil {
		return RenderTargetHandle{}, TextureHandle{}, err
	}
	var color C.uint32_t
	target := C.onix_metal_create_render_target(
		d.native,
		C.uint32_t(descriptor.Width),
		C.uint32_t(descriptor.Height),
		C.uint32_t(descriptor.ColorFormat),
		C.int(metalBoolInt(descriptor.Depth)),
		&color,
	)
	if target == 0 || color == 0 {
		return RenderTargetHandle{}, TextureHandle{}, metalError("create render target")
	}
	colorHandle := d.textures.Insert(uint32(color))
	targetHandle := d.targets.Insert(nativeRenderTarget{native: uint32(target), color: colorHandle})
	return targetHandle, colorHandle, nil
}

func (d *metalDriver) ResizeRenderTarget(handle RenderTargetHandle, descriptor RenderTargetDescriptor) (TextureHandle, error) {
	if err := descriptor.Validate(); err != nil {
		return TextureHandle{}, err
	}
	target := d.targets.getRef(handle)
	if target == nil {
		return TextureHandle{}, fmt.Errorf("render: invalid Metal render target handle %s", handle)
	}
	var color C.uint32_t
	if C.onix_metal_resize_render_target(
		d.native,
		C.uint32_t(target.native),
		C.uint32_t(descriptor.Width),
		C.uint32_t(descriptor.Height),
		C.uint32_t(descriptor.ColorFormat),
		C.int(metalBoolInt(descriptor.Depth)),
		&color,
	) == 0 || color == 0 {
		return TextureHandle{}, metalError("resize render target")
	}
	d.textures.Remove(target.color)
	target.color = d.textures.Insert(uint32(color))
	return target.color, nil
}

func (d *metalDriver) DestroyRenderTarget(handle RenderTargetHandle) error {
	target := d.targets.getRef(handle)
	if target == nil {
		return fmt.Errorf("render: invalid Metal render target handle %s", handle)
	}
	if C.onix_metal_destroy_render_target(d.native, C.uint32_t(target.native)) == 0 {
		return metalError("destroy render target")
	}
	d.textures.Remove(target.color)
	d.targets.Remove(handle)
	return nil
}

func (d *metalDriver) BeginRenderTarget(handle RenderTargetHandle, clear RenderTargetClear) error {
	target := d.targets.getRef(handle)
	if target == nil {
		return fmt.Errorf("render: invalid Metal render target handle %s", handle)
	}
	color := [4]C.float{C.float(clear.Color.R), C.float(clear.Color.G), C.float(clear.Color.B), C.float(clear.Color.A)}
	if C.onix_metal_begin_render_target(
		d.native,
		C.uint32_t(target.native),
		&color[0],
		C.double(clear.Depth),
		C.int(metalBoolInt(clear.ClearColor)),
		C.int(metalBoolInt(clear.ClearDepth)),
	) == 0 {
		return metalError("begin render target")
	}
	return nil
}

func (d *metalDriver) EndRenderTarget() error {
	if C.onix_metal_end_render_target(d.native) == 0 {
		return metalError("end render target")
	}
	return nil
}

func (d *metalDriver) ReadRenderTarget(handle RenderTargetHandle, region RenderTargetRegion, destination []byte) error {
	target := d.targets.getRef(handle)
	if target == nil {
		return fmt.Errorf("render: invalid Metal render target readback %s", handle)
	}
	if len(destination) == 0 {
		return errors.New("render: Metal render target readback destination is empty")
	}
	if C.onix_metal_read_render_target(
		d.native,
		C.uint32_t(target.native),
		C.int32_t(region.X), C.int32_t(region.Y),
		C.uint32_t(region.Width), C.uint32_t(region.Height),
		(*C.uint8_t)(unsafe.Pointer(&destination[0])), C.size_t(len(destination)),
	) == 0 {
		return metalError("read render target")
	}
	return nil
}

func (d *metalDriver) DrawSprites(batch SpriteBatch) error {
	if len(batch.Instances) == 0 || len(batch.Draws) == 0 {
		return nil
	}
	sprites, err := d.packSpriteDraws(&batch, nativeMetal)
	if err != nil {
		return err
	}
	uniforms := &d.spriteUniforms
	copyFloat32ToMetal(uniforms.projection[:], batch.Projection[:])
	if C.onix_metal_draw_sprites(
		d.native,
		uniforms,
		(*C.OnixMetalInstance)(unsafe.Pointer(&batch.Instances[0])),
		C.uint32_t(len(batch.Instances)),
		&d.draws[0],
		C.uint32_t(len(d.draws)),
	) == 0 {
		return metalError("draw sprites")
	}
	d.recordSpriteStatistics(batch, sprites, d.spriteBytes)
	return nil
}

func (d *metalDriver) CreateMaterial2DShader(descriptor ShaderDescriptor) (Material2DShaderHandle, error) {
	return createMetalShader(descriptor, "Material2D", &d.shaders2D, func(source *C.char, length C.size_t) C.uint32_t {
		return C.onix_metal_create_material2d_shader(d.native, source, length)
	})
}

func (d *metalDriver) DestroyMaterial2DShader(handle Material2DShaderHandle) error {
	return (&d.shaders2D).destroyNativeResource(handle, "Metal Material2D shader", func(shader uint32) error {
		if C.onix_metal_destroy_material2d_shader(d.native, C.uint32_t(shader)) == 0 {
			return metalError("destroy Material2D shader")
		}
		return nil
	})
}

func createMetalShader[R any](descriptor ShaderDescriptor, kind string, pool *GenerationPool[R, uint32], create func(*C.char, C.size_t) C.uint32_t) (Handle[R], error) {
	if err := descriptor.Validate(); err != nil {
		return Handle[R]{}, err
	}
	if descriptor.MetalSource == "" {
		return Handle[R]{}, fmt.Errorf("render: %s shader has no Metal source", kind)
	}
	source := C.CString(descriptor.MetalSource)
	defer C.free(unsafe.Pointer(source))
	shader := create(source, C.size_t(len(descriptor.MetalSource)))
	if shader == 0 {
		return Handle[R]{}, metalError("create " + kind + " shader")
	}
	return pool.Insert(uint32(shader)), nil
}

func (d *metalDriver) DrawShapes(batch ShapeBatch) error {
	if len(batch.Instances) == 0 {
		return nil
	}
	uniforms := C.OnixMetalSpriteUniforms{}
	copyFloat32ToMetal(uniforms.projection[:], batch.Projection[:])
	if C.onix_metal_draw_shapes(
		d.native,
		&uniforms,
		(*C.OnixMetalShapeInstance)(unsafe.Pointer(&batch.Instances[0])),
		C.uint32_t(len(batch.Instances)),
	) == 0 {
		return metalError("draw shapes")
	}
	d.recordShapeStatistics(batch)
	return nil
}

func (d *metalDriver) CreateMesh(vertices []Vertex3D, indices []uint32) (MeshHandle, error) {
	if len(vertices) == 0 || len(indices) == 0 {
		return MeshHandle{}, errors.New("render: Metal mesh data is empty")
	}
	mesh := C.onix_metal_create_mesh(
		d.native,
		(*C.OnixMetalVertex3D)(unsafe.Pointer(&vertices[0])), C.uint32_t(len(vertices)),
		(*C.uint32_t)(unsafe.Pointer(&indices[0])), C.uint32_t(len(indices)),
	)
	if mesh == 0 {
		return MeshHandle{}, metalError("create mesh")
	}
	return d.meshes.Insert(nativeMesh{native: uint32(mesh)}), nil
}

func (d *metalDriver) DestroyMesh(handle MeshHandle) error {
	mesh := d.meshes.getRef(handle)
	if mesh == nil {
		return fmt.Errorf("render: invalid Metal mesh handle %s", handle)
	}
	if C.onix_metal_destroy_mesh(d.native, C.uint32_t(mesh.native)) == 0 {
		return metalError("destroy mesh")
	}
	d.meshes.Remove(handle)
	return nil
}

func (d *metalDriver) DrawMeshes(batch MeshBatch3D) error {
	return d.drawMeshes(batch, false)
}

func (d *metalDriver) DrawMeshPicking(batch MeshBatch3D) error {
	if len(batch.Instances) == 0 || len(batch.Draws) == 0 {
		return nil
	}
	err := d.packMeshPickingDraws(&batch, nativeMetal)
	if err != nil {
		return err
	}
	dynamic := len(batch.Vertices) != 0 || len(batch.Indices) != 0
	uniforms := C.OnixNativeMeshUniforms{}
	copyFloat32ToMetal(uniforms.view_projection[:], batch.ViewProjection[:])
	var vertices *C.OnixMetalVertex3D
	var indices *C.uint32_t
	if dynamic {
		vertices = (*C.OnixMetalVertex3D)(unsafe.Pointer(&batch.Vertices[0]))
		indices = (*C.uint32_t)(unsafe.Pointer(&batch.Indices[0]))
	}
	if C.onix_metal_draw_mesh_picking(
		d.native, &uniforms,
		vertices, C.uint32_t(len(batch.Vertices)),
		indices, C.uint32_t(len(batch.Indices)),
		(*C.OnixMetalInstance3D)(unsafe.Pointer(&batch.Instances[0])), C.uint32_t(len(batch.Instances)),
		&d.draws3D[0], C.uint32_t(len(d.draws3D)),
	) == 0 {
		return metalError("draw mesh picking")
	}
	return nil
}

func (d *metalDriver) DrawMeshesDeformed(batch MeshBatch3D) error {
	var err error
	d.deformationWords = (&d.deformationRetention).resetScratch(d.deformationWords)
	d.deformationWords, err = packMeshDeformations(d.deformationWords, batch, d)
	if err != nil {
		return fmt.Errorf("render: invalid Metal deformation: %w", err)
	}
	return d.drawMeshes(batch, true)
}

func (d *metalDriver) drawMeshes(batch MeshBatch3D, gpuDeformation bool) error {
	if len(batch.Instances) == 0 || len(batch.Draws) == 0 {
		return nil
	}
	counts, err := d.packMeshDraws(&batch, nativeMetal, gpuDeformation)
	if err != nil {
		return err
	}
	dynamic := len(batch.Vertices) != 0 || len(batch.Indices) != 0
	var uniforms nativeMeshUniforms
	data := nativeMeshUniformData(&uniforms)
	copy(data[:16], batch.ViewProjection[:])
	writeMeshSceneUniforms(data, &batch)
	var vertices *C.OnixMetalVertex3D
	var indices *C.uint32_t
	if dynamic {
		vertices = (*C.OnixMetalVertex3D)(unsafe.Pointer(&batch.Vertices[0]))
		indices = (*C.uint32_t)(unsafe.Pointer(&batch.Indices[0]))
	}
	if C.onix_metal_draw_meshes(
		d.native, &uniforms,
		vertices, C.uint32_t(len(batch.Vertices)),
		indices, C.uint32_t(len(batch.Indices)),
		(*C.OnixMetalInstance3D)(unsafe.Pointer(&batch.Instances[0])), C.uint32_t(len(batch.Instances)),
		(*C.uint32_t)(unsafe.Pointer(&d.deformationWords[0])), C.uint32_t(len(d.deformationWords)),
		&d.draws3D[0], C.uint32_t(len(d.draws3D)),
	) == 0 {
		return metalError("draw meshes")
	}
	d.recordMeshStatistics(batch, counts)
	return nil
}

func (d *metalDriver) CreateMeshShader(descriptor ShaderDescriptor) (MeshShaderHandle, error) {
	return createMetalShader(descriptor, "mesh", &d.shaders, func(source *C.char, length C.size_t) C.uint32_t {
		return C.onix_metal_create_mesh_shader(d.native, source, length)
	})
}

func (d *metalDriver) DestroyMeshShader(handle MeshShaderHandle) error {
	return (&d.shaders).destroyNativeResource(handle, "Metal mesh shader", func(shader uint32) error {
		if C.onix_metal_destroy_mesh_shader(d.native, C.uint32_t(shader)) == 0 {
			return metalError("destroy mesh shader")
		}
		return nil
	})
}

func (d *metalDriver) Present() error {
	if C.onix_metal_present(d.native) == 0 {
		return metalError("present")
	}
	return nil
}

func (d *metalDriver) ResizeSurface(width, height int) error {
	if width <= 0 || height <= 0 || (d.width == width && d.height == height) {
		return nil
	}
	if C.onix_metal_resize(d.native, C.uint32_t(width), C.uint32_t(height)) == 0 {
		return metalError("resize surface")
	}
	d.width, d.height = width, height
	d.viewportSet = false
	return nil
}

func (d *metalDriver) SetVSync(enabled bool) {
	value := C.int(0)
	if enabled {
		value = 1
	}
	C.onix_metal_set_vsync(d.native, value)
}

func (d *metalDriver) Destroy() {
	d.textureUpload.release()
	if d.native != nil {
		C.onix_metal_destroy(d.native)
		d.native = nil
		d.textures.Clear()
		d.targets.Clear()
		d.meshes.Clear()
		d.shaders.Clear()
		d.shaders2D.Clear()
	}
}

func metalError(operation string) error {
	message := C.GoString(C.onix_metal_last_error())
	if message == "" {
		message = "unknown Metal error"
	}
	return fmt.Errorf("render: %s: %s", operation, message)
}

func metalBoolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func copyFloat32ToMetal(destination []C.float, source []float32) {
	for index := range source {
		destination[index] = C.float(source[index])
	}
}

func (d *metalDriver) deformationLayout(handle MeshHandle) (meshDeformationLayout, bool) {
	mesh := d.meshes.getRef(handle)
	if mesh == nil {
		return meshDeformationLayout{}, false
	}
	return mesh.deformation, mesh.deformation.vertexCount != 0
}

func (d *metalDriver) SetMeshDeformation(handle MeshHandle, descriptor MeshDeformationDescriptor) error {
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
	if C.onix_metal_set_mesh_deformation(d.native, C.uint32_t(mesh.native), (*C.uint32_t)(unsafe.Pointer(&words[0])), C.uint32_t(len(words)), C.uint32_t(descriptor.VertexCount)) == 0 {
		return metalError("upload mesh deformation")
	}
	mesh.deformation = layout
	return nil
}
