//go:build js && wasm && (wechat || onix_webgl) && !onix_directx && !onix_metal && !onix_vulkan && !onix_opengl && !onix_webgpu

package render

import (
	_ "embed"
	"errors"
	"fmt"
	"syscall/js"
	"unsafe"
)

const (
	webGLArrayBuffer         = 0x8892
	webGLElementArrayBuffer  = 0x8893
	webGLDynamicDraw         = 0x88E8
	webGLStaticDraw          = 0x88E4
	webGLFloat               = 0x1406
	webGLUnsignedByte        = 0x1401
	webGLUnsignedInt         = 0x1405
	webGLUnsignedShort       = 0x1403
	webGLTriangles           = 0x0004
	webGLTexture2D           = 0x0DE1
	webGLTexture0            = 0x84C0
	webGLTexture1            = 0x84C1
	webGLTexture2            = 0x84C2
	webGLRGBA                = 0x1908
	webGLRed                 = 0x1903
	webGLLinear              = 0x2601
	webGLLinearMipmapLinear  = 0x2703
	webGLClampToEdge         = 0x812F
	webGLTextureMinFilter    = 0x2801
	webGLTextureMagFilter    = 0x2800
	webGLUnpackAlignment     = 0x0CF5
	webGLTextureWrapS        = 0x2802
	webGLTextureWrapT        = 0x2803
	webGLNearest             = 0x2600
	webGLVertexShader        = 0x8B31
	webGLFragmentShader      = 0x8B30
	webGLCompileStatus       = 0x8B81
	webGLLinkStatus          = 0x8B82
	webGLColorBufferBit      = 0x00004000
	webGLDepthBufferBit      = 0x00000100
	webGLBlend               = 0x0BE2
	webGLDepthTest           = 0x0B71
	webGLGreater             = 0x0204
	webGLScissorTest         = 0x0C11
	webGLSamples             = 0x80A9
	webGLFramebuffer         = 0x8D40
	webGLReadFramebuffer     = 0x8CA8
	webGLDrawFramebuffer     = 0x8CA9
	webGLRenderbuffer        = 0x8D41
	webGLFramebufferComplete = 0x8CD5
	webGLColorAttachment0    = 0x8CE0
	webGLDepthAttachment     = 0x8D00
	webGLDepthComponent24    = 0x81A6
	webGLRGBA8               = 0x8058
	webGLR8                  = 0x8229
	webGLOne                 = 1
	webGLR32UI               = 0x8236
	webGLRedInteger          = 0x8D94
	webGLColor               = 0x1800
	webGLRenderer            = 0x1F01
	webGLSrcColor            = 0x0300
	webGLOneMinusSrcColor    = 0x0301
	webGLSrcAlpha            = 0x0302
	webGLOneMinusSrcAlpha    = 0x0303
	webGLDstAlpha            = 0x0304
	webGLOneMinusDstAlpha    = 0x0305
	webGLDstColor            = 0x0306
	webGLOneMinusDstColor    = 0x0307
	webGLSrcAlphaSaturate    = 0x0308
	webGLConstantColor       = 0x8001
	webGLOneMinusConstantClr = 0x8002
	webGLConstantAlpha       = 0x8003
	webGLOneMinusConstantA   = 0x8004
	webGLFramesInFlight      = 3
)

const (
	webGLShapeInstanceBytes    = int(unsafe.Sizeof(ShapeInstance2D{}))
	webGLShapeLocalModelOffset = int(unsafe.Offsetof(ShapeInstance2D{}.LocalModel))
	webGLShapeFillOffset       = int(unsafe.Offsetof(ShapeInstance2D{}.Fill))
	webGLShapeBorderOffset     = int(unsafe.Offsetof(ShapeInstance2D{}.Border))
	webGLShapeParamsOffset     = int(unsafe.Offsetof(ShapeInstance2D{}.Params))
	webGLShapeKindOffset       = int(unsafe.Offsetof(ShapeInstance2D{}.Kind))
)

type webGLProgram struct {
	value           js.Value
	uniforms        map[string]js.Value
	generatedWGSL   bool
	projectionBytes js.Value
	projectionData  js.Value
	projection      [16]float32
	projectionSet   bool
	meshSamplersSet bool
	spriteCoverage  bool
}

type webGLTexture struct {
	value  js.Value
	filter TextureFilter
	format TextureFormat
}

type webGLRenderTarget struct {
	framebuffer js.Value
	depth       js.Value
	color       TextureHandle
	descriptor  RenderTargetDescriptor
}

type webGLMesh struct {
	vertexBuffer js.Value
	indexBuffer  js.Value
	vertexCount  int
	indexCount   int
}

type webGLGeometry struct {
	vao            js.Value
	instanceBuffer js.Value
	instanceBytes  int
	instanceOffset int
	instanceStride int
	instanceUsed   int
	retention      bufferRetention
}

type webGLMeshGeometry struct {
	vao            js.Value
	vertexBuffer   js.Value
	indexBuffer    js.Value
	instanceBuffer js.Value
}

type webGL2Driver struct {
	meshTextureTable js.Value
	meshDrawHelper   js.Value
	meshDrawReset    js.Value
	meshCommandBytes js.Value
	meshCommandData  js.Value
	meshCommand      [20]uint32
	driverStatistics
	meshUniformData webGLUniformData
	gl              js.Value
	astcExtension   js.Value
	deviceName      string
	width           int
	height          int

	quadVBO           js.Value
	quadEBO           js.Value
	spriteGeometry    [webGLFramesInFlight]webGLGeometry
	shapeGeometry     [webGLFramesInFlight]webGLGeometry
	meshBuiltins      [meshVariantCount - 1]webGLProgram
	programs          [3]webGLProgram
	pickingProgram    webGLProgram
	meshGeometry      webGLMeshGeometry
	textures          GenerationPool[TextureResource, webGLTexture]
	targets           GenerationPool[RenderTargetResource, *webGLRenderTarget]
	meshes            GenerationPool[MeshResource, webGLMesh]
	meshShaders       GenerationPool[MeshShaderResource, *webGLProgram]
	material2DShaders GenerationPool[Material2DShaderResource, *webGLProgram]
	viewport          Rect
	scissor           *Rect
	frameIndex        int
	instanceData      js.Value
	textureData       js.Value
	textureBatchCall  js.Value
	textureCommandsJS js.Value
	textureCommands   []uint32
	textureBatchPix   []byte
	textureBatchUsed  int
	textureBatchKeep  bufferRetention
	mipmapUpdates     map[TextureHandle]webGLTexture
	currentProgram    js.Value
	boundVAO          js.Value
	boundBuffer       js.Value
	boundTexture      js.Value
	activeTarget      RenderTargetHandle
	readbackPixels    js.Value
	readbackBytes     js.Value
	readbackFormat    RenderTargetColorFormat
	uintClear         js.Value
	blend             BlendState
	blendSet          bool
	clear             Color
	clearSet          bool
	scissorEnabled    bool
	blendEnabled      bool
	depthEnabled      bool
	surfaceTarget     *webGLRenderTarget
	surfaceSamples    int
	surfaceMSAA       [3]js.Value // framebuffer, color renderbuffer, depth renderbuffer
	surfaceProgram    webGLProgram

	instanceDataRetention   scratchRetention
	textureDataRetention    scratchRetention
	textureCommandRetention scratchRetention
}

func NewWebGL2Driver(context js.Value) (Driver, error) {
	if context.IsNull() || context.IsUndefined() {
		return nil, errors.New("render: WebGL2 context is unavailable")
	}
	width := context.Get("drawingBufferWidth").Int()
	height := context.Get("drawingBufferHeight").Int()
	driver := &webGL2Driver{
		gl:             context,
		astcExtension:  context.Call("getExtension", "WEBGL_compressed_texture_astc"),
		deviceName:     webGLDeviceName(context),
		surfaceSamples: context.Call("getParameter", webGLSamples).Int(),
		width:          width,
		height:         height,
		frameIndex:     webGLFramesInFlight - 1,
		viewport: Rect{
			Width:  int32(width),
			Height: int32(height),
		},
	}
	for pipeline, sources := range [2][2]string{
		{spriteMultiVertexGLSL, spriteMultiFragmentGLSL},
		{webGLShapeVertex, webGLShapeFragment},
	} {
		program, err := driver.createProgram(sources[0], sources[1])
		if err != nil {
			driver.Destroy()
			return nil, err
		}
		driver.programs[pipeline] = program
	}
	driver.createQuadResources()
	driver.createMeshResources()
	context.Call("enable", webGLBlend)
	context.Call("pixelStorei", webGLUnpackAlignment, 1)
	driver.blendEnabled = true
	context.Call("depthFunc", webGLGreater)
	driver.SetBlend(PremultipliedAlphaBlend)
	driver.useProgram(PipelineSprite)
	driver.gl.Call("activeTexture", webGLTexture0)
	driver.uniformInt(&driver.programs[PipelineSprite], "tex", 0)
	driver.uniformInt(&driver.programs[PipelineSprite], "tex1", 1)
	driver.uniformInt(&driver.programs[PipelineSprite], "tex2", 2)
	driver.uniformInt(&driver.programs[PipelineSprite], "tex3", 3)
	return driver, nil
}

func (d *webGL2Driver) RendererInfo() RendererInfo {
	return RendererInfo{Backend: "WebGL2", Device: d.deviceName, RenderTargetBottomLeft: true}
}

func webGLDeviceName(context js.Value) string {
	extension := context.Call("getExtension", "WEBGL_debug_renderer_info")
	if extension.Truthy() {
		parameter := extension.Get("UNMASKED_RENDERER_WEBGL")
		if parameter.Type() == js.TypeNumber {
			if value := context.Call("getParameter", parameter).String(); value != "" {
				return value
			}
		}
	}
	return context.Call("getParameter", webGLRenderer).String()
}

func (d *webGL2Driver) surfaceFramebuffer() js.Value {
	if d.surfaceMSAA[0].Truthy() {
		return d.surfaceMSAA[0]
	}
	if d.surfaceTarget != nil {
		return d.surfaceTarget.framebuffer
	}
	return js.Null()
}

func (d *webGL2Driver) blitSurface(source, destination js.Value) {
	if d.scissorEnabled {
		d.gl.Call("disable", webGLScissorTest)
	}
	d.gl.Call("bindFramebuffer", webGLReadFramebuffer, source)
	d.gl.Call("bindFramebuffer", webGLDrawFramebuffer, destination)
	d.gl.Call("blitFramebuffer", 0, 0, d.width, d.height, 0, 0, d.width, d.height, webGLColorBufferBit, webGLNearest)
	d.gl.Call("bindFramebuffer", webGLFramebuffer, destination)
	if d.scissorEnabled {
		d.gl.Call("enable", webGLScissorTest)
	}
}

// Multisampled default framebuffers cannot be blit destinations in WebGL2.
// Present with a full-screen triangle; texelFetch preserves orientation and alpha.
func (d *webGL2Driver) copySurfaceColor(color TextureHandle, framebuffer js.Value) {
	d.gl.Call("bindFramebuffer", webGLFramebuffer, framebuffer)
	depth, blend := d.depthEnabled, d.blendEnabled
	d.setDepthEnabled(false)
	d.setBlendEnabled(false)
	if d.scissorEnabled {
		d.gl.Call("disable", webGLScissorTest)
	}
	d.gl.Call("viewport", 0, 0, d.width, d.height)
	d.useRawProgram(&d.surfaceProgram)
	d.bindVertexArray(js.Null())
	texture, _ := d.textures.Get(color)
	d.bindTexture(texture.value)
	d.gl.Call("drawArrays", webGLTriangles, 0, 3)
	d.gl.Call("viewport", d.viewport.X, d.viewport.Y, d.viewport.Width, d.viewport.Height)
	d.setDepthEnabled(depth)
	d.setBlendEnabled(blend)
	if d.scissorEnabled {
		d.gl.Call("enable", webGLScissorTest)
	}
}

func (d *webGL2Driver) deleteSurfaceMSAA(objects [3]js.Value) {
	if objects[0].Truthy() {
		d.gl.Call("deleteFramebuffer", objects[0])
	}
	for _, object := range objects[1:] {
		if object.Truthy() {
			d.gl.Call("deleteRenderbuffer", object)
		}
	}
}

func (d *webGL2Driver) ensureSurfaceDepth(preserveColor bool) error {
	if target := d.surfaceTarget; target != nil && target.descriptor.Width == d.width && target.descriptor.Height == d.height {
		return nil
	}
	if !d.surfaceProgram.value.Truthy() {
		program, err := d.createProgram(`#version 300 es
void main() { vec2 p=vec2(float((gl_VertexID<<1)&2),float(gl_VertexID&2)); gl_Position=vec4(p*2.0-1.0,0.0,1.0); }`, `#version 300 es
precision highp float;
uniform sampler2D tex;
out vec4 color;
void main() { color=texelFetch(tex,ivec2(gl_FragCoord.xy),0); }`)
		if err != nil {
			return err
		}
		d.surfaceProgram = program
		d.useRawProgram(&d.surfaceProgram)
		d.uniformInt(&d.surfaceProgram, "tex", 0)
	}
	target, err := d.createRenderTarget(RenderTargetDescriptor{Width: d.width, Height: d.height, Depth: d.surfaceSamples == 0})
	if err != nil {
		return err
	}
	var msaa [3]js.Value
	framebuffer := target.framebuffer
	if d.surfaceSamples > 0 {
		msaa[0] = d.gl.Call("createFramebuffer")
		msaa[1] = d.gl.Call("createRenderbuffer")
		msaa[2] = d.gl.Call("createRenderbuffer")
		if !msaa[0].Truthy() || !msaa[1].Truthy() || !msaa[2].Truthy() {
			d.deleteSurfaceMSAA(msaa)
			d.deleteRenderTarget(target, true)
			return errors.New("render: WebGL2 could not create multisampled surface resources")
		}
		d.gl.Call("bindFramebuffer", webGLFramebuffer, msaa[0])
		for index, format := range [...]int{webGLRGBA8, webGLDepthComponent24} {
			d.gl.Call("bindRenderbuffer", webGLRenderbuffer, msaa[index+1])
			d.gl.Call("renderbufferStorageMultisample", webGLRenderbuffer, d.surfaceSamples, format, d.width, d.height)
			attachment := webGLColorAttachment0
			if index == 1 {
				attachment = webGLDepthAttachment
			}
			d.gl.Call("framebufferRenderbuffer", webGLFramebuffer, attachment, webGLRenderbuffer, msaa[index+1])
		}
		d.gl.Call("bindRenderbuffer", webGLRenderbuffer, js.Null())
		if d.gl.Call("checkFramebufferStatus", webGLFramebuffer).Int() != webGLFramebufferComplete {
			d.deleteSurfaceMSAA(msaa)
			d.deleteRenderTarget(target, true)
			d.gl.Call("bindFramebuffer", webGLFramebuffer, d.surfaceFramebuffer())
			return errors.New("render: WebGL2 multisampled surface framebuffer is incomplete")
		}
		framebuffer = msaa[0]
	}
	if preserveColor {
		d.blitSurface(d.surfaceFramebuffer(), target.framebuffer)
		if d.surfaceSamples > 0 {
			d.copySurfaceColor(target.color, framebuffer)
		}
		if d.scissorEnabled {
			d.gl.Call("disable", webGLScissorTest)
		}
		d.gl.Call("depthMask", true)
		d.gl.Call("clearDepth", 0)
		d.gl.Call("clear", webGLDepthBufferBit)
		if d.scissorEnabled {
			d.gl.Call("enable", webGLScissorTest)
		}
	}
	d.deleteSurfaceMSAA(d.surfaceMSAA)
	d.deleteRenderTarget(d.surfaceTarget, true)
	d.surfaceMSAA = msaa
	d.surfaceTarget = target
	d.gl.Call("bindFramebuffer", webGLFramebuffer, framebuffer)
	return nil
}

func (d *webGL2Driver) trimScratch() {
	trimJSUpload(&d.textureData, &d.textureDataRetention)
	trimJSUpload(&d.instanceData, &d.instanceDataRetention)
	trimJSUpload(&d.textureCommandsJS, &d.textureCommandRetention)
	d.textureBatchPix = (&d.textureBatchKeep).trimUploadBuffer(d.textureBatchPix, d.textureBatchUsed, 16<<10)
	d.textureBatchUsed = 0
}

func (d *webGL2Driver) BeginFrame(clear Color) error {
	if d.activeTarget.Valid() {
		return errors.New("render: WebGL2 render target is still active")
	}
	if d.surfaceTarget != nil {
		if err := d.ensureSurfaceDepth(false); err != nil {
			return err
		}
	}
	d.gl.Call("bindFramebuffer", webGLFramebuffer, d.surfaceFramebuffer())
	d.frameIndex = (d.frameIndex + 1) % webGLFramesInFlight
	for _, geometry := range [...]*webGLGeometry{&d.spriteGeometry[d.frameIndex], &d.shapeGeometry[d.frameIndex]} {
		d.beginInstanceUploads(geometry)
	}
	restoreScissor := d.scissorEnabled
	if restoreScissor {
		d.gl.Call("disable", webGLScissorTest)
		d.scissorEnabled = false
	}
	if !d.clearSet || d.clear != clear {
		d.gl.Call("clearColor", clear.R, clear.G, clear.B, clear.A)
		d.clear = clear
		d.clearSet = true
	}
	mask := webGLColorBufferBit
	if d.surfaceTarget != nil {
		d.gl.Call("depthMask", true)
		d.gl.Call("clearDepth", 0)
		mask |= webGLDepthBufferBit
	}
	d.gl.Call("clear", mask)
	if restoreScissor && d.scissor != nil {
		d.applyScissor(*d.scissor)
	}
	d.trimScratch()
	d.beginFrameStatistics(d.width, d.height)
	return nil
}

func (d *webGL2Driver) SetViewport(viewport Rect) {
	if d.viewport == viewport {
		return
	}
	d.viewport = viewport
	d.gl.Call("viewport", viewport.X, viewport.Y, viewport.Width, viewport.Height)
}

func (d *webGL2Driver) Viewport() Rect { return d.viewport }

func (d *webGL2Driver) SetBlend(blend BlendState) error {
	if d.blendSet && d.blend == blend {
		return nil
	}
	d.setRawBlend(blend)
	d.blend = blend
	d.blendSet = true
	return nil
}

func (d *webGL2Driver) SetScissor(scissor *Rect) {
	if scissor == nil {
		d.scissor = nil
		if d.scissorEnabled {
			d.gl.Call("disable", webGLScissorTest)
			d.scissorEnabled = false
		}
		return
	}
	if d.scissor != nil && *d.scissor == *scissor && d.scissorEnabled {
		return
	}
	copy := *scissor
	d.scissor = &copy
	d.applyScissor(copy)
}

func (d *webGL2Driver) applyScissor(scissor Rect) {
	if !d.scissorEnabled {
		d.gl.Call("enable", webGLScissorTest)
		d.scissorEnabled = true
	}
	height := d.height
	if target, ok := d.targets.Get(d.activeTarget); ok && target != nil {
		height = target.descriptor.Height
	}
	d.gl.Call("scissor", scissor.X, height-int(scissor.Y+scissor.Height), scissor.Width, scissor.Height)
}

func (d *webGL2Driver) CreateTexture(descriptor TextureDescriptor, pixels []byte) (TextureHandle, error) {
	if err := descriptor.Validate(pixels); err != nil {
		return TextureHandle{}, err
	}
	texture := d.gl.Call("createTexture")
	if !texture.Truthy() {
		return TextureHandle{}, errors.New("render: WebGL2 could not create texture")
	}
	d.bindTexture(texture)
	d.setTextureParameters(descriptor.Filter, descriptor.WrapU, descriptor.WrapV)
	internalFormat, format := webGLRGBA, webGLRGBA
	if descriptor.Format == TextureFormatSRGBA8 {
		internalFormat = 0x8C43 // SRGB8_ALPHA8
	}
	if descriptor.Format == TextureFormatR8 {
		internalFormat, format = webGLR8, webGLRed
	}
	compressedInternal, compressed := descriptor.Format.GLCompressedInternalFormat()
	if compressed {
		if descriptor.Format != TextureFormatASTC4x4 && descriptor.Format != TextureFormatASTC4x4SRGB {
			d.gl.Call("deleteTexture", texture)
			return TextureHandle{}, fmt.Errorf("render: WebGL2 compressed format %s is unsupported", descriptor.Format.Name())
		}
		if !d.astcExtension.Truthy() {
			d.gl.Call("deleteTexture", texture)
			return TextureHandle{}, errors.New("render: WebGL2 ASTC texture extension is unavailable")
		}
		data := copyBytesToJS(&d.textureData, &d.textureDataRetention, pixels)
		for level, mip := range descriptor.MipChain {
			d.gl.Call("compressedTexImage2D", webGLTexture2D, level, compressedInternal, mip.Width, mip.Height, 0,
				data, mip.Offset, mip.Bytes)
		}
	} else {
		data := copyBytesToJS(&d.textureData, &d.textureDataRetention, pixels)
		d.gl.Call("texImage2D", webGLTexture2D, 0, internalFormat, descriptor.Width, descriptor.Height, 0, format, webGLUnsignedByte, data)
		if descriptor.Filter == TextureFilterLinearMipmapLinear {
			d.gl.Call("generateMipmap", webGLTexture2D)
		}
	}
	d.recordTextureUploadStatistics(1, uint64(len(pixels)))
	return d.insertTexture(webGLTexture{value: texture, filter: descriptor.Filter, format: descriptor.Format}), nil
}

func (d *webGL2Driver) UpdateTextures(updates []TextureUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	if len(updates) >= 8 && len(updates) <= 512 {
		length := 0
		batch := true
		for _, update := range updates {
			texture, ok := d.textures.Get(update.Texture)
			region := update.Region
			texelBytes := texture.format.TexelBytes()
			if !ok || texelBytes == 0 || region.X < 0 || region.Y < 0 || region.Width <= 0 || region.Height <= 0 ||
				len(update.Pixels) != region.Width*region.Height*texelBytes {
				return errors.New("render: invalid WebGL2 texture update")
			}
			length += len(update.Pixels)
			if length > 256<<10 {
				batch = false
				break
			}
		}
		if batch {
			return d.updateTexturesBatch(updates, length)
		}
	}
	defer func() {
		// Release JS texture references on success and on partial failure.
		if len(d.mipmapUpdates) > 4096 {
			d.mipmapUpdates = nil
		} else {
			clear(d.mipmapUpdates)
		}
	}()
	var uploadBytes uint64
	for _, update := range updates {
		texture, ok := d.textures.Get(update.Texture)
		if !ok {
			return fmt.Errorf("render: invalid WebGL2 texture handle %s", update.Texture)
		}
		region := update.Region
		if texelBytes := texture.format.TexelBytes(); texelBytes == 0 || region.Width <= 0 || region.Height <= 0 || len(update.Pixels) != region.Width*region.Height*texelBytes {
			return errors.New("render: invalid WebGL2 texture update")
		}
		d.bindTexture(texture.value)
		data := copyBytesToJS(&d.textureData, &d.textureDataRetention, update.Pixels)
		format := webGLRGBA
		if texture.format == TextureFormatR8 {
			format = webGLRed
		}
		d.gl.Call("texSubImage2D", webGLTexture2D, 0, region.X, region.Y, region.Width, region.Height, format, webGLUnsignedByte, data)
		uploadBytes += uint64(len(update.Pixels))
		if texture.filter == TextureFilterLinearMipmapLinear {
			if d.mipmapUpdates == nil {
				d.mipmapUpdates = make(map[TextureHandle]webGLTexture)
			}
			d.mipmapUpdates[update.Texture] = texture
		}
	}
	for _, texture := range d.mipmapUpdates {
		d.bindTexture(texture.value)
		d.gl.Call("generateMipmap", webGLTexture2D)
	}
	d.recordTextureUploadStatistics(len(updates), uploadBytes)
	return nil
}

// Small region batches cross the Go/JS boundary once for their pixels and
// once for the command list. WebGL still uploads each region in source order.
func (d *webGL2Driver) updateTexturesBatch(updates []TextureUpdate, length int) error {
	d.textureBatchPix = packTexturePixels(d.textureBatchPix, updates, length)
	d.textureBatchUsed = max(d.textureBatchUsed, length)
	d.textureCommands = d.textureCommands[:0]
	offset := 0
	for _, update := range updates {
		texture, _ := d.textures.Get(update.Texture)
		format := uint32(webGLRGBA)
		if texture.format == TextureFormatR8 {
			format = webGLRed
		}
		mipmap := uint32(0)
		if texture.filter == TextureFilterLinearMipmapLinear {
			mipmap = 1
		}
		d.textureCommands = append(d.textureCommands, update.Texture.ID(), uint32(update.Region.X), uint32(update.Region.Y),
			uint32(update.Region.Width), uint32(update.Region.Height), format, uint32(offset), mipmap)
		offset += len(update.Pixels)
	}
	if !d.textureBatchCall.Truthy() {
		d.textureBatchCall = js.Global().Get("Function").New("gl", "textures", webGLTextureBatchHelper).Invoke(d.gl, d.meshTextureTable)
	}
	commands := copyBytesToJS(&d.textureCommandsJS, &d.textureCommandRetention, sliceBytes(d.textureCommands))
	pixels := copyBytesToJS(&d.textureData, &d.textureDataRetention, d.textureBatchPix)
	d.textureBatchCall.Invoke(commands, pixels, len(updates))
	d.boundTexture = js.Undefined()
	d.recordTextureUploadStatistics(len(updates), uint64(length))
	return nil
}

const webGLTextureBatchHelper = `
const mipmaps = new Set();
return (commandBytes, pixels, count) => {
 const commands = new Uint32Array(commandBytes.buffer, commandBytes.byteOffset, count*8);
 let bound = null;
 mipmaps.clear();
 gl.activeTexture(0x84c0);
 for (let i=0; i<count; i++) {
  const at=i*8, texture=textures[commands[at]];
  if (bound !== texture) { gl.bindTexture(0x0de1, texture); bound=texture; }
  gl.texSubImage2D(0x0de1, 0, commands[at+1], commands[at+2], commands[at+3], commands[at+4], commands[at+5], 0x1401, pixels, commands[at+6]);
  if (commands[at+7]) mipmaps.add(texture);
 }
 for (const texture of mipmaps) {
  if (bound !== texture) { gl.bindTexture(0x0de1, texture); bound=texture; }
  gl.generateMipmap(0x0de1);
 }
};`

func (d *webGL2Driver) DestroyTexture(handle TextureHandle) error {
	texture, ok := d.textures.Get(handle)
	if !ok {
		return fmt.Errorf("render: invalid WebGL2 texture handle %s", handle)
	}
	d.gl.Call("deleteTexture", texture.value)
	if d.boundTexture.Truthy() && d.boundTexture.Equal(texture.value) {
		d.boundTexture = js.Undefined()
	}
	d.textures.Remove(handle)
	d.meshTextureTable.SetIndex(int(handle.ID()), js.Null())
	return nil
}

func (d *webGL2Driver) CreateRenderTarget(descriptor RenderTargetDescriptor) (RenderTargetHandle, TextureHandle, error) {
	if err := descriptor.Validate(); err != nil {
		return RenderTargetHandle{}, TextureHandle{}, err
	}
	if d.activeTarget.Valid() {
		return RenderTargetHandle{}, TextureHandle{}, errors.New("render: cannot create a WebGL2 render target during a target pass")
	}
	target, err := d.createRenderTarget(descriptor)
	if err != nil {
		return RenderTargetHandle{}, TextureHandle{}, err
	}
	return d.targets.Insert(target), target.color, nil
}

func (d *webGL2Driver) ResizeRenderTarget(handle RenderTargetHandle, descriptor RenderTargetDescriptor) (TextureHandle, error) {
	if err := descriptor.Validate(); err != nil {
		return TextureHandle{}, err
	}
	target, ok := d.targets.Get(handle)
	if !ok || target == nil || d.activeTarget.Valid() {
		return TextureHandle{}, fmt.Errorf("render: invalid WebGL2 render target resize %s", handle)
	}
	replacement, err := d.createRenderTarget(descriptor)
	if err != nil {
		return TextureHandle{}, err
	}
	d.deleteRenderTarget(target, true)
	*target = *replacement
	return target.color, nil
}

func (d *webGL2Driver) DestroyRenderTarget(handle RenderTargetHandle) error {
	target, ok := d.targets.Get(handle)
	if !ok || target == nil || d.activeTarget == handle {
		return fmt.Errorf("render: invalid WebGL2 render target destruction %s", handle)
	}
	d.deleteRenderTarget(target, true)
	d.targets.Remove(handle)
	return nil
}

func (d *webGL2Driver) BeginRenderTarget(handle RenderTargetHandle, clear RenderTargetClear) error {
	target, ok := d.targets.Get(handle)
	if !ok || target == nil || d.activeTarget.Valid() {
		return fmt.Errorf("render: invalid WebGL2 render target begin %s", handle)
	}
	d.gl.Call("bindFramebuffer", webGLFramebuffer, target.framebuffer)
	d.activeTarget = handle
	restoreScissor := d.scissorEnabled
	if restoreScissor {
		d.gl.Call("disable", webGLScissorTest)
		d.scissorEnabled = false
	}
	if clear.ClearColor {
		if target.descriptor.ColorFormat == RenderTargetUint32 {
			if !d.uintClear.Truthy() {
				d.uintClear = js.Global().Get("Uint32Array").New(4)
			}
			d.uintClear.SetIndex(0, uint32(max(clear.Color.R, 0)))
			d.uintClear.SetIndex(1, uint32(max(clear.Color.G, 0)))
			d.uintClear.SetIndex(2, uint32(max(clear.Color.B, 0)))
			d.uintClear.SetIndex(3, uint32(max(clear.Color.A, 0)))
			d.gl.Call("clearBufferuiv", webGLColor, 0, d.uintClear)
		} else {
			d.gl.Call("clearColor", clear.Color.R, clear.Color.G, clear.Color.B, clear.Color.A)
			d.clear, d.clearSet = clear.Color, true
			d.gl.Call("clear", webGLColorBufferBit)
		}
	}
	if clear.ClearDepth && target.descriptor.Depth {
		d.gl.Call("depthMask", true)
		d.gl.Call("clearDepth", clear.Depth)
		d.gl.Call("clear", webGLDepthBufferBit)
	}
	if restoreScissor && d.scissor != nil {
		d.applyScissor(*d.scissor)
	}
	return nil
}

func (d *webGL2Driver) EndRenderTarget() error {
	if !d.activeTarget.Valid() {
		return errors.New("render: no WebGL2 render target is active")
	}
	d.gl.Call("bindFramebuffer", webGLFramebuffer, d.surfaceFramebuffer())
	d.activeTarget = RenderTargetHandle{}
	if d.scissorEnabled && d.scissor != nil {
		d.applyScissor(*d.scissor)
	}
	return nil
}

func (d *webGL2Driver) ReadRenderTarget(handle RenderTargetHandle, region RenderTargetRegion, destination []byte) error {
	target, ok := d.targets.Get(handle)
	if !ok || target == nil || d.activeTarget.Valid() {
		return fmt.Errorf("render: invalid WebGL2 render target readback %s", handle)
	}
	d.gl.Call("bindFramebuffer", webGLFramebuffer, target.framebuffer)
	defer d.gl.Call("bindFramebuffer", webGLFramebuffer, d.surfaceFramebuffer())
	length := len(destination)
	if !d.readbackPixels.Truthy() || d.readbackFormat != target.descriptor.ColorFormat || d.readbackPixels.Get("byteLength").Int() < length {
		capacity := nextBufferCapacity(length)
		if target.descriptor.ColorFormat == RenderTargetUint32 {
			d.readbackPixels = js.Global().Get("Uint32Array").New(capacity / 4)
		} else {
			d.readbackPixels = js.Global().Get("Uint8Array").New(capacity)
		}
		d.readbackBytes = js.Global().Get("Uint8Array").New(d.readbackPixels.Get("buffer"), d.readbackPixels.Get("byteOffset"), capacity)
		d.readbackFormat = target.descriptor.ColorFormat
	}
	format, pixelType := webGLRGBA, webGLUnsignedByte
	if target.descriptor.ColorFormat == RenderTargetUint32 {
		format, pixelType = webGLRedInteger, webGLUnsignedInt
	} else if target.descriptor.ColorFormat == RenderTargetR8 {
		format = webGLRed
	}
	y := target.descriptor.Height - region.Y - region.Height
	d.gl.Call("pixelStorei", 0x0D05, 1) // PACK_ALIGNMENT: R8 rows can have arbitrary widths.
	d.gl.Call("readPixels", region.X, y, region.Width, region.Height, format, pixelType, d.readbackPixels)
	js.CopyBytesToGo(destination, d.readbackBytes)
	flipRenderTargetRows(destination, region.Width*target.descriptor.ColorFormat.TexelBytes(), region.Height)
	return nil
}

func (d *webGL2Driver) createRenderTarget(descriptor RenderTargetDescriptor) (*webGLRenderTarget, error) {
	texture := d.gl.Call("createTexture")
	framebuffer := d.gl.Call("createFramebuffer")
	if !texture.Truthy() || !framebuffer.Truthy() {
		if texture.Truthy() {
			d.gl.Call("deleteTexture", texture)
		}
		if framebuffer.Truthy() {
			d.gl.Call("deleteFramebuffer", framebuffer)
		}
		return nil, errors.New("render: WebGL2 could not create render target resources")
	}
	d.bindTexture(texture)
	internalFormat, format, pixelType := webGLRGBA8, webGLRGBA, webGLUnsignedByte
	if descriptor.ColorFormat == RenderTargetUint32 {
		internalFormat, format, pixelType = webGLR32UI, webGLRedInteger, webGLUnsignedInt
		d.gl.Call("texParameteri", webGLTexture2D, webGLTextureMinFilter, webGLNearest)
		d.gl.Call("texParameteri", webGLTexture2D, webGLTextureMagFilter, webGLNearest)
		d.gl.Call("texParameteri", webGLTexture2D, webGLTextureWrapS, webGLClampToEdge)
		d.gl.Call("texParameteri", webGLTexture2D, webGLTextureWrapT, webGLClampToEdge)
	} else {
		if descriptor.ColorFormat == RenderTargetR8 {
			internalFormat, format = webGLR8, webGLRed
		}
		d.setTextureParameters(TextureFilterLinear, TextureAddressClamp, TextureAddressClamp)
	}
	d.gl.Call("texImage2D", webGLTexture2D, 0, internalFormat, descriptor.Width, descriptor.Height, 0, format, pixelType, js.Null())
	d.gl.Call("bindFramebuffer", webGLFramebuffer, framebuffer)
	d.gl.Call("framebufferTexture2D", webGLFramebuffer, webGLColorAttachment0, webGLTexture2D, texture, 0)
	depth := js.Undefined()
	if descriptor.Depth {
		depth = d.gl.Call("createRenderbuffer")
		if depth.Truthy() {
			d.gl.Call("bindRenderbuffer", webGLRenderbuffer, depth)
			d.gl.Call("renderbufferStorage", webGLRenderbuffer, webGLDepthComponent24, descriptor.Width, descriptor.Height)
			d.gl.Call("framebufferRenderbuffer", webGLFramebuffer, webGLDepthAttachment, webGLRenderbuffer, depth)
		}
	}
	complete := depth.Truthy() || !descriptor.Depth
	complete = complete && d.gl.Call("checkFramebufferStatus", webGLFramebuffer).Int() == webGLFramebufferComplete
	d.gl.Call("bindRenderbuffer", webGLRenderbuffer, js.Null())
	d.gl.Call("bindFramebuffer", webGLFramebuffer, d.surfaceFramebuffer())
	if !complete {
		d.gl.Call("deleteTexture", texture)
		d.gl.Call("deleteFramebuffer", framebuffer)
		if depth.Truthy() {
			d.gl.Call("deleteRenderbuffer", depth)
		}
		return nil, errors.New("render: WebGL2 framebuffer is incomplete")
	}
	textureFormat := TextureFormatRGBA8
	if descriptor.ColorFormat == RenderTargetR8 {
		textureFormat = TextureFormatR8
	}
	color := d.insertTexture(webGLTexture{value: texture, filter: TextureFilterLinear, format: textureFormat})
	return &webGLRenderTarget{framebuffer: framebuffer, depth: depth, color: color, descriptor: descriptor}, nil
}

func (d *webGL2Driver) deleteRenderTarget(target *webGLRenderTarget, removeColor bool) {
	if target == nil {
		return
	}
	if target.framebuffer.Truthy() {
		d.gl.Call("deleteFramebuffer", target.framebuffer)
	}
	if target.depth.Truthy() {
		d.gl.Call("deleteRenderbuffer", target.depth)
	}
	if removeColor {
		if texture, ok := d.textures.Remove(target.color); ok {
			d.meshTextureTable.SetIndex(int(target.color.ID()), js.Null())
			d.gl.Call("deleteTexture", texture.value)
			if d.boundTexture.Truthy() && d.boundTexture.Equal(texture.value) {
				d.boundTexture = js.Undefined()
			}
		}
	}
	target.framebuffer = js.Undefined()
	target.depth = js.Undefined()
	target.color = TextureHandle{}
}

func (d *webGL2Driver) DrawSprites(batch SpriteBatch) error {
	if len(batch.Instances) == 0 || len(batch.Draws) == 0 {
		return nil
	}
	scratch := spriteUploadBuffers.get()
	defer spriteUploadBuffers.put(scratch)
	scratch.data = scratch.data[:0]
	var sprites uint64
	for _, draw := range batch.Draws {
		sprites += uint64(draw.InstanceCount)
		if draw.InstanceCount == 0 || uint64(draw.FirstInstance)+uint64(draw.InstanceCount) > uint64(len(batch.Instances)) {
			return errors.New("render: invalid WebGL2 sprite draw range")
		}
		for slot := range SpriteTextureSlots {
			if _, ok := d.textures.Get(draw.TextureAt(slot)); !ok {
				return fmt.Errorf("render: invalid WebGL2 texture handle %s", draw.TextureAt(slot))
			}
		}
		scratch.data = appendSpriteDrawInstances(scratch.data, batch.Instances, draw)
	}
	d.setDepthEnabled(false)
	d.setBlendEnabled(true)
	geometry := &d.spriteGeometry[d.frameIndex]
	d.bindVertexArray(geometry.vao)
	data, byteLength := (&d.instanceDataRetention).copySliceToJS(&d.instanceData, scratch.data)
	d.uploadInstances(geometry, data, byteLength)
	packedOffset := 0
	var currentProgram *webGLProgram
	for _, draw := range batch.Draws {
		program := &d.programs[PipelineSprite]
		if draw.Shader.Valid() {
			var ok bool
			program, ok = d.material2DShaders.Get(draw.Shader)
			if !ok {
				return fmt.Errorf("render: invalid WebGL2 Material2D shader handle %s", draw.Shader)
			}
		}
		if program != currentProgram {
			currentProgram = program
			d.useRawProgram(program)
			d.setProjection(program, batch.Projection)
		}
		texture, ok := d.textures.Get(draw.Texture)
		if !ok {
			return fmt.Errorf("render: invalid WebGL2 texture handle %s", draw.Texture)
		}
		if draw.InstanceCount == 0 || uint64(draw.FirstInstance)+uint64(draw.InstanceCount) > uint64(len(batch.Instances)) {
			return errors.New("render: invalid WebGL2 sprite draw range")
		}
		d.bindTexture(texture.value)
		var coverage [SpriteTextureSlots]float32
		if texture.format == TextureFormatR8 {
			coverage[0] = 1
		}
		if !draw.Shader.Valid() {
			for slot := 1; slot < SpriteTextureSlots; slot++ {
				texture, _ := d.textures.Get(draw.TextureAt(slot))
				d.gl.Call("activeTexture", webGLTexture0+slot)
				d.gl.Call("bindTexture", webGLTexture2D, texture.value)
				if texture.format == TextureFormatR8 {
					coverage[slot] = 1
				}
			}
			d.gl.Call("activeTexture", webGLTexture0)
		}
		if !draw.Shader.Valid() || program.spriteCoverage {
			d.gl.Call("uniform4f", d.uniform(program, "coverage"), coverage[0], coverage[1], coverage[2], coverage[3])
		}
		packedOffset = int(spriteInstanceOffset(uint64(packedOffset), draw))
		d.setSpriteInstanceOffset(geometry, packedOffset, draw.Shader.Valid())
		packedOffset += int(draw.InstanceCount) * spriteInstanceStride(draw.Shader.Valid())
		d.gl.Call("drawElementsInstanced", webGLTriangles, 6, webGLUnsignedShort, 0, int(draw.InstanceCount))
	}
	d.recordSpriteStatistics(batch, sprites, uint64(len(scratch.data)))
	return nil
}

func (d *webGL2Driver) CreateMaterial2DShader(descriptor ShaderDescriptor) (Material2DShaderHandle, error) {
	handle, err := d.createWebGLShader(descriptor, webGLSpriteVertex, webGLSpriteFragment, "Material2D", &d.material2DShaders)
	if err == nil {
		program, _ := d.material2DShaders.Get(handle)
		program.spriteCoverage = descriptor.WebGL2FragmentSource == ""
	}
	return handle, err
}

func (d *webGL2Driver) DestroyMaterial2DShader(handle Material2DShaderHandle) error {
	return d.destroyWebGLShader(handle, "Material2D", &d.material2DShaders)
}

func (d *webGL2Driver) DrawShapes(batch ShapeBatch) error {
	if len(batch.Instances) == 0 {
		return nil
	}
	d.setDepthEnabled(false)
	d.setBlendEnabled(true)
	program := &d.programs[PipelineShape]
	d.useProgram(PipelineShape)
	d.setProjection(program, batch.Projection)
	geometry := &d.shapeGeometry[d.frameIndex]
	d.bindVertexArray(geometry.vao)
	data, byteLength := (&d.instanceDataRetention).copySliceToJS(&d.instanceData, batch.Instances)
	d.uploadInstances(geometry, data, byteLength)
	d.gl.Call("drawElementsInstanced", webGLTriangles, 6, webGLUnsignedShort, 0, len(batch.Instances))
	d.recordShapeStatistics(batch)
	return nil
}

func (d *webGL2Driver) CreateMesh(vertices []Vertex3D, indices []uint32) (MeshHandle, error) {
	if len(vertices) == 0 || len(indices) == 0 {
		return MeshHandle{}, errors.New("render: WebGL2 mesh data is empty")
	}
	mesh := webGLMesh{
		vertexBuffer: d.gl.Call("createBuffer"),
		indexBuffer:  d.gl.Call("createBuffer"),
		vertexCount:  len(vertices),
		indexCount:   len(indices),
	}
	if !mesh.vertexBuffer.Truthy() || !mesh.indexBuffer.Truthy() {
		d.deleteMesh(mesh)
		return MeshHandle{}, errors.New("render: WebGL2 could not create mesh buffers")
	}
	d.bindVertexArray(js.Null())
	vertexData, vertexBytes := (&d.instanceDataRetention).copySliceToJS(&d.instanceData, vertices)
	d.bindArrayBuffer(mesh.vertexBuffer)
	d.gl.Call("bufferData", webGLArrayBuffer, vertexData, webGLStaticDraw, 0, vertexBytes)
	indexData, indexBytes := (&d.instanceDataRetention).copySliceToJS(&d.instanceData, indices)
	d.gl.Call("bindBuffer", webGLElementArrayBuffer, mesh.indexBuffer)
	d.gl.Call("bufferData", webGLElementArrayBuffer, indexData, webGLStaticDraw, 0, indexBytes)
	return d.meshes.Insert(mesh), nil
}

func (d *webGL2Driver) DestroyMesh(handle MeshHandle) error {
	mesh, ok := d.meshes.Remove(handle)
	if !ok {
		return fmt.Errorf("render: invalid WebGL2 mesh handle %s", handle)
	}
	d.deleteMesh(mesh)
	return nil
}

func (d *webGL2Driver) DrawMeshes(batch MeshBatch3D) error {
	if len(batch.Instances) == 0 || len(batch.Draws) == 0 {
		return nil
	}
	counts, err := d.validateMeshBatch(&batch, false)
	if err != nil {
		return err
	}
	if !d.activeTarget.Valid() {
		if err := d.ensureSurfaceDepth(true); err != nil {
			return err
		}
	}

	d.setDepthEnabled(true)
	d.setBlendEnabled(false)
	d.gl.Call("depthMask", true)
	d.bindVertexArray(d.meshGeometry.vao)

	d.uploadMeshBatch(&batch)

	d.ensureMeshDrawHelper()
	d.meshDrawReset.Invoke()
	transparent := false
	blendMode := MeshBlendMode(^uint32(0))
	var currentProgram *webGLProgram
	boundGeometry := false
	boundMesh := MeshHandle{}
	for _, draw := range batch.Draws {
		if !boundGeometry || draw.Mesh != boundMesh {
			vertexBuffer := d.meshGeometry.vertexBuffer
			indexBuffer := d.meshGeometry.indexBuffer
			if draw.Mesh.Valid() {
				mesh, _ := d.meshes.Get(draw.Mesh)
				vertexBuffer = mesh.vertexBuffer
				indexBuffer = mesh.indexBuffer
			}
			d.configureMeshVertexAttributes(vertexBuffer)
			d.gl.Call("bindBuffer", webGLElementArrayBuffer, indexBuffer)
			boundGeometry = true
			boundMesh = draw.Mesh
		}
		program := &d.programs[PipelineMesh]
		if draw.Shader.Valid() {
			program, _ = d.meshShaders.Get(draw.Shader)
		} else {
			variant := meshBuiltinVariant(&batch, &draw)
			if variant != meshVariantFull {
				program = &d.meshBuiltins[variant-1]
			}
			if !program.value.Truthy() {
				source := [meshVariantCount]string{webGLMeshFragment, webGLMeshUnlitFragment, webGLMeshDepthFragment, webGLMeshBasicFragment}[variant]
				compiled, err := d.createProgram(webGLMeshVertex, source)
				if err != nil {
					return err
				}
				*program = compiled
			}
		}
		if program != currentProgram {
			currentProgram = program
			d.useRawProgram(program)
			d.setMeshSamplers(program)
			d.setProjection(program, batch.ViewProjection)
			d.uniformVec4(program, "cameraPosition", batch.CameraPosition)
			d.uniformVec4(program, "ambientLight", batch.AmbientLight)
			d.uniformVec4(program, "environment", batch.Lighting.Environment)
			d.uniformVec4(program, "lightDirection", batch.LightDirection)
			d.uniformVec4(program, "directionalLight", batch.DirectionalLight)
			var lightUniforms [meshLightUniformFloatCount]float32
			writeMeshLightUniforms(lightUniforms[:], batch.Lights)
			writeMeshLighting(lightUniforms[:], batch.Lighting)
			lightData := d.meshUniformData.write(lightUniforms[:])
			d.gl.Call("uniformMatrix4fv", d.uniform(program, "shadowMatrices"), false, lightData, meshShadowMatrixOffset, meshShadowRectOffset-meshShadowMatrixOffset)
			d.uniformVec4Array(program, "shadowRects", lightData, meshShadowRectOffset, meshShadowParamsOffset-meshShadowRectOffset)
			d.uniformVec4Array(program, "shadowParams", lightData, meshShadowParamsOffset, meshLightShadowOffset-meshShadowParamsOffset)
			d.uniformVec4Array(program, "lightShadows", lightData, meshLightShadowOffset, meshOcclusionInverseOffset-meshLightShadowOffset)
			d.uniformVec4(program, "cameraForward", [4]float32(lightUniforms[meshLightingOffset+8:meshLightRightOffset]))
			d.uniformVec4(program, "renderParams", [4]float32(lightUniforms[meshLightingOffset+4:meshLightingOffset+8]))
			d.gl.Call("uniformMatrix4fv", d.uniform(program, "occlusionInverseProjection"), false, lightData, meshOcclusionInverseOffset, meshLightUniformFloatCount-meshOcclusionInverseOffset)
			d.uniformVec4(program, "lightMeta", [4]float32(lightUniforms[32+MaxMeshLights*16:meshLightingOffset]))
			d.uniformVec4Array(program, "lightPosition", lightData, 32, MaxMeshLights*4)
			d.uniformVec4Array(program, "lightDirections", lightData, 32+MaxMeshLights*4, MaxMeshLights*4)
			d.uniformVec4Array(program, "lightColors", lightData, 32+MaxMeshLights*8, MaxMeshLights*4)
			d.uniformVec4Array(program, "lightRight", lightData, meshLightRightOffset, meshLightUpOffset-meshLightRightOffset)
			d.uniformVec4Array(program, "lightUp", lightData, meshLightUpOffset, meshShadowMatrixOffset-meshLightUpOffset)
			d.uniformVec4Array(program, "lightParams", lightData, 32+MaxMeshLights*12, MaxMeshLights*4)
			d.gl.Call("uniform1i", d.uniform(program, "lightCount"), int(lightUniforms[32+MaxMeshLights*16]))
		}
		isTransparent := draw.AlphaMode == 2
		if isTransparent != transparent {
			transparent = isTransparent
			d.setBlendEnabled(transparent)
			d.gl.Call("depthMask", !transparent)
		}
		if isTransparent && draw.BlendMode != blendMode {
			blendMode = draw.BlendMode
			d.setMeshBlendMode(blendMode)
		}
		d.meshCommand[0] = draw.Texture.ID()
		d.meshCommand[1], d.meshCommand[2] = draw.Texture.ID(), draw.Texture.ID()
		if draw.NormalTexture.Valid() {
			d.meshCommand[1] = draw.NormalTexture.ID()
		}
		if draw.MetallicRoughnessTexture.Valid() {
			d.meshCommand[2] = draw.MetallicRoughnessTexture.ID()
		}
		d.uniformInt(program, "hasNormalTexture", boolIntWebGL(draw.NormalTexture.Valid()))
		d.uniformInt(program, "hasMetallicRoughnessTexture", boolIntWebGL(draw.MetallicRoughnessTexture.Valid()))
		for slot, handle := range draw.ExtraTextures(&batch.Lighting) {
			d.meshCommand[3+slot] = draw.Texture.ID()
			if handle.Valid() {
				if _, ok := d.textures.Get(handle); !ok {
					return fmt.Errorf("render: invalid surface texture %s", handle)
				}
				d.meshCommand[3+slot] = handle.ID()
			}
		}
		d.meshCommand[16], d.meshCommand[17] = draw.FirstInstance, draw.FirstIndex
		d.meshCommand[18], d.meshCommand[19] = draw.IndexCount, draw.InstanceCount
		d.bindArrayBuffer(d.meshGeometry.instanceBuffer)
		js.CopyBytesToJS(d.meshCommandBytes, sliceBytes(d.meshCommand[:]))
		d.meshDrawHelper.Invoke(d.meshCommandData)
		texture, _ := d.textures.Get(draw.Texture)
		d.boundTexture = texture.value
	}
	if blendMode <= MeshBlendScreen && d.blendSet {
		d.setRawBlend(d.blend)
	}
	d.gl.Call("depthMask", true)
	d.recordMeshStatistics(batch, counts)
	return nil
}

func (d *webGL2Driver) DrawMeshPicking(batch MeshBatch3D) error {
	if len(batch.Instances) == 0 || len(batch.Draws) == 0 {
		return nil
	}
	target, ok := d.targets.Get(d.activeTarget)
	if !ok || target == nil || target.descriptor.ColorFormat != RenderTargetUint32 {
		return errors.New("render: WebGL2 mesh picking requires an active R32Uint target")
	}
	if _, err := d.validateMeshBatch(&batch, true); err != nil {
		return err
	}
	if !d.pickingProgram.value.Truthy() {
		program, err := d.createProgram(webGLPickingVertex, webGLPickingFragment)
		if err != nil {
			return err
		}
		d.pickingProgram = program
	}
	d.setDepthEnabled(true)
	d.setBlendEnabled(false)
	d.gl.Call("depthMask", true)
	d.bindVertexArray(d.meshGeometry.vao)
	d.uploadMeshBatch(&batch)
	d.useRawProgram(&d.pickingProgram)
	d.setProjection(&d.pickingProgram, batch.ViewProjection)
	boundGeometry := false
	boundMesh := MeshHandle{}
	for _, draw := range batch.Draws {
		if !boundGeometry || draw.Mesh != boundMesh {
			vertexBuffer := d.meshGeometry.vertexBuffer
			indexBuffer := d.meshGeometry.indexBuffer
			if draw.Mesh.Valid() {
				mesh, _ := d.meshes.Get(draw.Mesh)
				vertexBuffer, indexBuffer = mesh.vertexBuffer, mesh.indexBuffer
			}
			d.configureMeshVertexAttributes(vertexBuffer)
			d.gl.Call("bindBuffer", webGLElementArrayBuffer, indexBuffer)
			boundGeometry, boundMesh = true, draw.Mesh
		}
		d.bindArrayBuffer(d.meshGeometry.instanceBuffer)
		d.configureMeshInstanceAttributes(int(draw.FirstInstance)*int(unsafe.Sizeof(Instance3D{})), false)
		d.gl.Call("drawElementsInstanced", webGLTriangles, int(draw.IndexCount), webGLUnsignedInt, int(draw.FirstIndex)*4, int(draw.InstanceCount))
	}
	return nil
}

func (d *webGL2Driver) validateMeshBatch(batch *MeshBatch3D, picking bool) (meshDrawStatistics, error) {
	var counts meshDrawStatistics
	kind, meshLabel := "mesh", "mesh"
	if picking {
		kind, meshLabel = "picking", "picking mesh"
	}
	dynamic := len(batch.Vertices) != 0 || len(batch.Indices) != 0
	if dynamic && (len(batch.Vertices) == 0 || len(batch.Indices) == 0) {
		return counts, fmt.Errorf("render: incomplete WebGL2 dynamic %s data", meshLabel)
	}
	for _, draw := range batch.Draws {
		indexCount := len(batch.Indices)
		if draw.Mesh.Valid() {
			mesh, ok := d.meshes.Get(draw.Mesh)
			if !ok {
				return counts, fmt.Errorf("render: invalid WebGL2 %s handle %s", meshLabel, draw.Mesh)
			}
			indexCount = mesh.indexCount
		} else if !dynamic {
			return counts, fmt.Errorf("render: WebGL2 %s draw has no geometry", kind)
		}
		if draw.IndexCount == 0 || uint64(draw.FirstIndex)+uint64(draw.IndexCount) > uint64(indexCount) ||
			draw.BaseVertex != 0 || draw.InstanceCount == 0 || uint64(draw.FirstInstance)+uint64(draw.InstanceCount) > uint64(len(batch.Instances)) ||
			!picking && (draw.AlphaMode > 2 || draw.BlendMode > MeshBlendScreen) {
			return counts, fmt.Errorf("render: invalid WebGL2 %s draw range", kind)
		}
		if picking {
			continue
		}
		counts.add(draw)
		if _, ok := d.textures.Get(draw.Texture); !ok {
			return counts, fmt.Errorf("render: invalid WebGL2 texture handle %s", draw.Texture)
		}
		if draw.NormalTexture.Valid() {
			if _, ok := d.textures.Get(draw.NormalTexture); !ok {
				return counts, fmt.Errorf("render: invalid WebGL2 normal texture handle %s", draw.NormalTexture)
			}
		}
		if draw.MetallicRoughnessTexture.Valid() {
			if _, ok := d.textures.Get(draw.MetallicRoughnessTexture); !ok {
				return counts, fmt.Errorf("render: invalid WebGL2 metallic roughness texture handle %s", draw.MetallicRoughnessTexture)
			}
		}
		if draw.Shader.Valid() {
			if _, ok := d.meshShaders.Get(draw.Shader); !ok {
				return counts, fmt.Errorf("render: invalid WebGL2 mesh shader handle %s", draw.Shader)
			}
		}
	}
	return counts, nil
}

func (d *webGL2Driver) uploadMeshBatch(batch *MeshBatch3D) {
	if len(batch.Vertices) != 0 {
		vertices, vertexBytes := (&d.instanceDataRetention).copySliceToJS(&d.instanceData, batch.Vertices)
		d.bindArrayBuffer(d.meshGeometry.vertexBuffer)
		d.gl.Call("bufferData", webGLArrayBuffer, vertices, webGLDynamicDraw, 0, vertexBytes)
		indices, indexBytes := (&d.instanceDataRetention).copySliceToJS(&d.instanceData, batch.Indices)
		d.gl.Call("bindBuffer", webGLElementArrayBuffer, d.meshGeometry.indexBuffer)
		d.gl.Call("bufferData", webGLElementArrayBuffer, indices, webGLDynamicDraw, 0, indexBytes)
	}
	instances, instanceBytes := (&d.instanceDataRetention).copySliceToJS(&d.instanceData, batch.Instances)
	d.bindArrayBuffer(d.meshGeometry.instanceBuffer)
	d.gl.Call("bufferData", webGLArrayBuffer, instances, webGLDynamicDraw, 0, instanceBytes)
}

func (d *webGL2Driver) setMeshBlendMode(mode MeshBlendMode) {
	source, destination := BlendOne, BlendOneMinusSrcAlpha
	switch mode {
	case MeshBlendAdditive:
		destination = BlendOne
	case MeshBlendMultiply:
		source = BlendDstColor
	case MeshBlendScreen:
		// Spine's PMA screen mode uses the same factors as normal blending.
	}
	d.setRawBlend(BlendState{RGB: Blend{Src: source, Dst: destination}, Alpha: Blend{Src: source, Dst: destination}})
}

func (d *webGL2Driver) setRawBlend(blend BlendState) {
	d.gl.Call("blendColor", blend.Constant.R, blend.Constant.G, blend.Constant.B, blend.Constant.A)
	d.gl.Call("blendFuncSeparate", webGLBlendFactor(blend.RGB.Src), webGLBlendFactor(blend.RGB.Dst), webGLBlendFactor(blend.Alpha.Src), webGLBlendFactor(blend.Alpha.Dst))
}

func (d *webGL2Driver) CreateMeshShader(descriptor ShaderDescriptor) (MeshShaderHandle, error) {
	return d.createWebGLShader(descriptor, webGLMeshVertex, webGLMeshFragment, "mesh", &d.meshShaders)
}

func (d *webGL2Driver) DestroyMeshShader(handle MeshShaderHandle) error {
	return d.destroyWebGLShader(handle, "mesh", &d.meshShaders)
}

func (d *webGL2Driver) createWebGLShader[R any](descriptor ShaderDescriptor, defaultVertex, defaultFragment, kind string, pool *GenerationPool[R, *webGLProgram]) (Handle[R], error) {
	if err := descriptor.Validate(); err != nil {
		return Handle[R]{}, err
	}
	if descriptor.WebGL2VertexSource == "" && descriptor.WebGL2FragmentSource == "" {
		return Handle[R]{}, fmt.Errorf("render: %s shader has no WebGL2 source", kind)
	}
	vertex := descriptor.WebGL2VertexSource
	if vertex == "" {
		vertex = defaultVertex
	}
	fragment := descriptor.WebGL2FragmentSource
	if fragment == "" {
		fragment = defaultFragment
	}
	program, err := d.createProgram(vertex, fragment)
	if err != nil {
		return Handle[R]{}, err
	}
	d.useRawProgram(&program)
	d.uniformInt(&program, "tex", 0)
	if descriptor.WebGL2Generated {
		program.generatedWGSL = true
		d.uniformInt(&program, "_group_0_binding_1_vs", 0)
		d.uniformInt(&program, "_group_0_binding_1_fs", 0)
	}
	return pool.Insert(&program), nil
}

func (d *webGL2Driver) destroyWebGLShader[R any](handle Handle[R], kind string, pool *GenerationPool[R, *webGLProgram]) error {
	program, ok := pool.Remove(handle)
	if !ok {
		return fmt.Errorf("render: invalid WebGL2 %s shader handle %s", kind, handle)
	}
	if d.currentProgram.Truthy() && d.currentProgram.Equal(program.value) {
		d.currentProgram = js.Undefined()
	}
	d.gl.Call("deleteProgram", program.value)
	return nil
}

func (d *webGL2Driver) Present() error {
	if d.activeTarget.Valid() {
		return errors.New("render: cannot present while a WebGL2 render target is active")
	}
	if framebuffer := d.surfaceFramebuffer(); framebuffer.Truthy() {
		if d.surfaceSamples > 0 {
			d.blitSurface(framebuffer, d.surfaceTarget.framebuffer)
		}
		d.copySurfaceColor(d.surfaceTarget.color, js.Null())
		d.gl.Call("bindFramebuffer", webGLFramebuffer, framebuffer)
	}
	return nil
}

func (d *webGL2Driver) ResizeSurface(width, height int) error {
	if width <= 0 || height <= 0 {
		return errors.New("render: WebGL2 surface dimensions must be positive")
	}
	d.width, d.height = width, height
	return nil
}

func (d *webGL2Driver) SetVSync(bool) {
	// Browser presentation is synchronized by requestAnimationFrame.
}

func (d *webGL2Driver) Destroy() {
	d.deleteSurfaceMSAA(d.surfaceMSAA)
	d.surfaceMSAA = [3]js.Value{}
	if d.surfaceProgram.value.Truthy() {
		d.gl.Call("deleteProgram", d.surfaceProgram.value)
	}
	d.surfaceProgram = webGLProgram{}
	d.deleteRenderTarget(d.surfaceTarget, true)
	d.surfaceTarget = nil
	d.targets.Range(func(target *webGLRenderTarget) bool {
		d.deleteRenderTarget(target, false)
		return true
	})
	d.textures.Range(func(texture webGLTexture) bool {
		d.gl.Call("deleteTexture", texture.value)
		return true
	})
	d.meshShaders.Range(func(program *webGLProgram) bool {
		d.gl.Call("deleteProgram", program.value)
		return true
	})
	d.material2DShaders.Range(func(program *webGLProgram) bool {
		d.gl.Call("deleteProgram", program.value)
		return true
	})
	d.meshes.Range(func(mesh webGLMesh) bool {
		d.deleteMesh(mesh)
		return true
	})
	for index := range d.meshBuiltins {
		if d.meshBuiltins[index].value.Truthy() {
			d.gl.Call("deleteProgram", d.meshBuiltins[index].value)
		}
		d.meshBuiltins[index] = webGLProgram{}
	}
	for _, program := range d.programs {
		if program.value.Truthy() {
			d.gl.Call("deleteProgram", program.value)
		}
	}
	if d.pickingProgram.value.Truthy() {
		d.gl.Call("deleteProgram", d.pickingProgram.value)
	}
	for _, value := range []js.Value{d.quadVBO, d.quadEBO} {
		if value.Truthy() {
			d.gl.Call("deleteBuffer", value)
		}
	}
	for _, value := range []js.Value{d.meshGeometry.vertexBuffer, d.meshGeometry.indexBuffer, d.meshGeometry.instanceBuffer} {
		if value.Truthy() {
			d.gl.Call("deleteBuffer", value)
		}
	}
	if d.meshGeometry.vao.Truthy() {
		d.gl.Call("deleteVertexArray", d.meshGeometry.vao)
	}
	for index := range webGLFramesInFlight {
		for _, geometry := range []*webGLGeometry{&d.spriteGeometry[index], &d.shapeGeometry[index]} {
			if geometry.vao.Truthy() {
				d.gl.Call("deleteVertexArray", geometry.vao)
			}
			if geometry.instanceBuffer.Truthy() {
				d.gl.Call("deleteBuffer", geometry.instanceBuffer)
			}
		}
	}
	d.textures.Clear()
	d.targets.Clear()
	d.meshes.Clear()
	d.meshShaders.Clear()
	d.material2DShaders.Clear()
	d.instanceData = js.Undefined()
	d.textureData = js.Undefined()
	d.textureBatchCall = js.Undefined()
	d.textureCommandsJS = js.Undefined()
	d.textureCommands = nil
	d.textureBatchPix = nil
	d.meshUniformData = webGLUniformData{}
	d.mipmapUpdates = nil
	d.readbackPixels = js.Undefined()
	d.readbackBytes = js.Undefined()
	d.uintClear = js.Undefined()
	d.meshTextureTable = js.Undefined()
	d.meshDrawHelper = js.Undefined()
	d.meshDrawReset = js.Undefined()
	d.meshCommandBytes = js.Undefined()
	d.meshCommandData = js.Undefined()
}

func (d *webGL2Driver) createMeshResources() {
	d.meshGeometry = webGLMeshGeometry{
		vao: d.gl.Call("createVertexArray"), vertexBuffer: d.gl.Call("createBuffer"),
		indexBuffer: d.gl.Call("createBuffer"), instanceBuffer: d.gl.Call("createBuffer"),
	}
	d.gl.Call("bindVertexArray", d.meshGeometry.vao)
	d.gl.Call("bindBuffer", webGLArrayBuffer, d.meshGeometry.vertexBuffer)
	for _, attribute := range []struct{ index, size, offset int }{{0, 3, 0}, {1, 3, 12}, {2, 2, 24}, {15, 4, 32}} {
		d.gl.Call("vertexAttribPointer", attribute.index, attribute.size, webGLFloat, false, int(unsafe.Sizeof(Vertex3D{})), attribute.offset)
		d.gl.Call("enableVertexAttribArray", attribute.index)
	}
	d.gl.Call("bindBuffer", webGLElementArrayBuffer, d.meshGeometry.indexBuffer)
	d.gl.Call("bindBuffer", webGLArrayBuffer, d.meshGeometry.instanceBuffer)
	d.configureMeshInstanceAttributes(0, true)
	d.boundVAO = js.Undefined()
	d.boundBuffer = js.Undefined()
}

func (d *webGL2Driver) configureMeshVertexAttributes(buffer js.Value) {
	d.bindArrayBuffer(buffer)
	for _, attribute := range []struct{ index, size, offset int }{{0, 3, 0}, {1, 3, 12}, {2, 2, 24}, {15, 4, 32}} {
		d.gl.Call("vertexAttribPointer", attribute.index, attribute.size, webGLFloat, false, int(unsafe.Sizeof(Vertex3D{})), attribute.offset)
	}
}

func (d *webGL2Driver) configureMeshInstanceAttributes(base int, initialize bool) {
	stride := int(unsafe.Sizeof(Instance3D{}))
	for _, attribute := range []struct{ index, offset int }{
		{3, 0}, {4, 16}, {5, 32}, {6, 48}, {7, 64}, {8, 80}, {9, 96}, {10, 112},
		{11, 128}, {12, 144}, {13, 160}, {14, 176},
	} {
		d.gl.Call("vertexAttribPointer", attribute.index, 4, webGLFloat, false, stride, base+attribute.offset)
		if initialize {
			d.gl.Call("enableVertexAttribArray", attribute.index)
			d.gl.Call("vertexAttribDivisor", attribute.index, 1)
		}
	}
}

func (d *webGL2Driver) deleteMesh(mesh webGLMesh) {
	for _, buffer := range []js.Value{mesh.vertexBuffer, mesh.indexBuffer} {
		if buffer.Truthy() {
			d.gl.Call("deleteBuffer", buffer)
		}
	}
}

func (d *webGL2Driver) setDepthEnabled(enabled bool) {
	if d.depthEnabled == enabled {
		return
	}
	d.depthEnabled = enabled
	if enabled {
		d.gl.Call("enable", webGLDepthTest)
	} else {
		d.gl.Call("disable", webGLDepthTest)
	}
}

func (d *webGL2Driver) setBlendEnabled(enabled bool) {
	if d.blendEnabled == enabled {
		return
	}
	d.blendEnabled = enabled
	if enabled {
		d.gl.Call("enable", webGLBlend)
	} else {
		d.gl.Call("disable", webGLBlend)
	}
}

func (d *webGL2Driver) createQuadResources() {
	d.quadVBO = d.gl.Call("createBuffer")
	d.quadEBO = d.gl.Call("createBuffer")
	d.gl.Call("bindBuffer", webGLArrayBuffer, d.quadVBO)
	vertices, vertexBytes := (&d.instanceDataRetention).copySliceToJS(&d.instanceData, []float32{
		0, 0, 0, 0, 0,
		1, 0, 0, 1, 0,
		1, 1, 0, 1, 1,
		0, 1, 0, 0, 1,
	})
	d.gl.Call("bufferData", webGLArrayBuffer, vertices, webGLStaticDraw, 0, vertexBytes)
	d.gl.Call("bindBuffer", webGLElementArrayBuffer, d.quadEBO)
	indices, indexBytes := (&d.instanceDataRetention).copySliceToJS(&d.instanceData, []uint16{0, 1, 2, 0, 2, 3})
	d.gl.Call("bufferData", webGLElementArrayBuffer, indices, webGLStaticDraw, 0, indexBytes)
	for index := range webGLFramesInFlight {
		d.spriteGeometry[index] = d.createGeometry(false)
		d.shapeGeometry[index] = d.createGeometry(true)
	}
	d.boundVAO = js.Undefined()
	d.boundBuffer = js.Undefined()
}

func (d *webGL2Driver) createGeometry(shape bool) webGLGeometry {
	geometry := webGLGeometry{
		vao:            d.gl.Call("createVertexArray"),
		instanceBuffer: d.gl.Call("createBuffer"),
	}
	d.gl.Call("bindVertexArray", geometry.vao)
	d.gl.Call("bindBuffer", webGLArrayBuffer, d.quadVBO)
	d.gl.Call("vertexAttribPointer", 0, 3, webGLFloat, false, 20, 0)
	d.gl.Call("enableVertexAttribArray", 0)
	d.gl.Call("vertexAttribPointer", 1, 2, webGLFloat, false, 20, 12)
	d.gl.Call("enableVertexAttribArray", 1)
	d.gl.Call("bindBuffer", webGLElementArrayBuffer, d.quadEBO)
	d.gl.Call("bindBuffer", webGLArrayBuffer, geometry.instanceBuffer)
	if shape {
		d.configureShapeAttributes()
	} else {
		d.configureSpriteAttributes(0, true, true)
	}
	return geometry
}

func (d *webGL2Driver) configureSpriteAttributes(base int, initialize, custom bool) {
	for _, attribute := range []struct {
		index int
		size  int
	}{
		{2, 3}, {3, 3}, {4, 3}, {5, 3}, {6, 4},
		{7, 4}, {8, 4}, {9, 4}, {10, 4}, {11, 1},
	} {
		d.gl.Call("vertexAttribPointer", attribute.index, attribute.size, webGLFloat, false, spriteInstanceStride(custom), base+spriteAttributeOffset(attribute.index, custom))
		if initialize {
			d.gl.Call("enableVertexAttribArray", attribute.index)
			d.gl.Call("vertexAttribDivisor", attribute.index, 1)
		}
	}
}

func (d *webGL2Driver) configureShapeAttributes() {
	for _, attribute := range []struct {
		index  int
		size   int
		offset int
	}{
		{2, 3, 0}, {3, 3, 12},
		{4, 3, webGLShapeLocalModelOffset}, {5, 3, webGLShapeLocalModelOffset + 12},
		{6, 4, webGLShapeFillOffset}, {7, 4, webGLShapeBorderOffset},
		{8, 4, webGLShapeParamsOffset},
	} {
		d.gl.Call("vertexAttribPointer", attribute.index, attribute.size, webGLFloat, false, webGLShapeInstanceBytes, attribute.offset)
		d.gl.Call("enableVertexAttribArray", attribute.index)
		d.gl.Call("vertexAttribDivisor", attribute.index, 1)
	}
	d.gl.Call("vertexAttribIPointer", 9, 1, webGLUnsignedInt, webGLShapeInstanceBytes, webGLShapeKindOffset)
	d.gl.Call("enableVertexAttribArray", 9)
	d.gl.Call("vertexAttribDivisor", 9, 1)
}

func (d *webGL2Driver) useProgram(pipeline Pipeline) {
	d.useRawProgram(&d.programs[pipeline])
}

func (d *webGL2Driver) useRawProgram(program *webGLProgram) {
	if d.currentProgram.Truthy() && d.currentProgram.Equal(program.value) {
		return
	}
	d.gl.Call("useProgram", program.value)
	d.currentProgram = program.value
}

func (d *webGL2Driver) bindVertexArray(vao js.Value) {
	if d.boundVAO.Truthy() && d.boundVAO.Equal(vao) {
		return
	}
	d.gl.Call("bindVertexArray", vao)
	d.boundVAO = vao
}

func (d *webGL2Driver) bindArrayBuffer(buffer js.Value) {
	if d.boundBuffer.Truthy() && d.boundBuffer.Equal(buffer) {
		return
	}
	d.gl.Call("bindBuffer", webGLArrayBuffer, buffer)
	d.boundBuffer = buffer
}

func (d *webGL2Driver) bindTexture(texture js.Value) {
	if d.boundTexture.Truthy() && d.boundTexture.Equal(texture) {
		return
	}
	d.gl.Call("bindTexture", webGLTexture2D, texture)
	d.boundTexture = texture
}

// Instance storage is orphaned per batch so browsers can preserve busy GPU
// ranges. Measure the largest batch, including batches before the last draw.
func (d *webGL2Driver) beginInstanceUploads(geometry *webGLGeometry) {
	if capacity := geometry.retention.trimCapacity(geometry.instanceBytes, geometry.instanceUsed, 1024); capacity != 0 {
		geometry.instanceBytes = nextBufferCapacity(capacity)
		d.bindArrayBuffer(geometry.instanceBuffer)
		d.gl.Call("bufferData", webGLArrayBuffer, geometry.instanceBytes, webGLDynamicDraw)
	}
	geometry.instanceUsed = 0
}

func (d *webGL2Driver) uploadInstances(geometry *webGLGeometry, data js.Value, byteLength int) {
	geometry.instanceUsed = max(geometry.instanceUsed, byteLength)
	d.bindArrayBuffer(geometry.instanceBuffer)
	if byteLength > geometry.instanceBytes {
		geometry.instanceBytes = nextBufferCapacity(byteLength)
	}
	// Orphaning avoids waiting for draws that still consume the previous storage.
	d.gl.Call("bufferData", webGLArrayBuffer, geometry.instanceBytes, webGLDynamicDraw)
	d.gl.Call("bufferSubData", webGLArrayBuffer, 0, data, 0, byteLength)
}

func (d *webGL2Driver) setSpriteInstanceOffset(geometry *webGLGeometry, base int, custom bool) {
	if geometry.instanceOffset == base && geometry.instanceStride == spriteInstanceStride(custom) {
		return
	}
	d.configureSpriteAttributes(base, false, custom)
	geometry.instanceOffset = base
	geometry.instanceStride = spriteInstanceStride(custom)
}

func (d *webGL2Driver) setProjection(program *webGLProgram, projection [16]float32) {
	if program.projectionSet && program.projection == projection {
		return
	}
	if !program.projectionBytes.Truthy() {
		program.projectionBytes = js.Global().Get("Uint8Array").New(int(unsafe.Sizeof(projection)))
		program.projectionData = js.Global().Get("Float32Array").New(program.projectionBytes.Get("buffer"), 0, len(projection))
	}
	js.CopyBytesToJS(program.projectionBytes, sliceBytes(projection[:]))
	d.gl.Call("uniformMatrix4fv", d.uniform(program, "projection"), false, program.projectionData)
	if program.generatedWGSL {
		d.gl.Call("uniformMatrix4fv", d.uniform(program, "_immediates_binding_vs.projection"), false, program.projectionData)
		d.gl.Call("uniformMatrix4fv", d.uniform(program, "_immediates_binding_fs.projection"), false, program.projectionData)
	}
	program.projection = projection
	program.projectionSet = true
}

func (d *webGL2Driver) createProgram(vertexSource, fragmentSource string) (webGLProgram, error) {
	vertex, err := d.compileShader(webGLVertexShader, vertexSource)
	if err != nil {
		return webGLProgram{}, err
	}
	defer d.gl.Call("deleteShader", vertex)
	fragment, err := d.compileShader(webGLFragmentShader, fragmentSource)
	if err != nil {
		return webGLProgram{}, err
	}
	defer d.gl.Call("deleteShader", fragment)
	program := d.gl.Call("createProgram")
	d.gl.Call("attachShader", program, vertex)
	d.gl.Call("attachShader", program, fragment)
	d.gl.Call("linkProgram", program)
	if !d.gl.Call("getProgramParameter", program, webGLLinkStatus).Bool() {
		log := d.gl.Call("getProgramInfoLog", program).String()
		d.gl.Call("deleteProgram", program)
		return webGLProgram{}, fmt.Errorf("render: WebGL2 link failed: %s", log)
	}
	return webGLProgram{value: program, uniforms: make(map[string]js.Value)}, nil
}

func (d *webGL2Driver) compileShader(kind int, source string) (js.Value, error) {
	shader := d.gl.Call("createShader", kind)
	d.gl.Call("shaderSource", shader, source)
	d.gl.Call("compileShader", shader)
	if !d.gl.Call("getShaderParameter", shader, webGLCompileStatus).Bool() {
		log := d.gl.Call("getShaderInfoLog", shader).String()
		d.gl.Call("deleteShader", shader)
		return js.Null(), fmt.Errorf("render: WebGL2 shader compilation failed: %s", log)
	}
	return shader, nil
}

func (d *webGL2Driver) uniform(program *webGLProgram, name string) js.Value {
	if location, ok := program.uniforms[name]; ok {
		return location
	}
	location := d.gl.Call("getUniformLocation", program.value, name)
	program.uniforms[name] = location
	return location
}

func (d *webGL2Driver) uniformInt(program *webGLProgram, name string, value int) {
	d.gl.Call("uniform1i", d.uniform(program, name), value)
}

func (d *webGL2Driver) uniformVec4(program *webGLProgram, name string, value [4]float32) {
	d.gl.Call("uniform4f", d.uniform(program, name), value[0], value[1], value[2], value[3])
	if program.generatedWGSL {
		if generatedName := webGLGeneratedUniformName("_immediates_binding_vs", name); generatedName != "" {
			d.gl.Call("uniform4f", d.uniform(program, generatedName), value[0], value[1], value[2], value[3])
		}
		if generatedName := webGLGeneratedUniformName("_immediates_binding_fs", name); generatedName != "" {
			d.gl.Call("uniform4f", d.uniform(program, generatedName), value[0], value[1], value[2], value[3])
		}
	}
}

func (d *webGL2Driver) uniformVec4Array(program *webGLProgram, name string, data js.Value, offset, length int) {
	d.gl.Call("uniform4fv", d.uniform(program, name), data, offset, length)
}

// Each upload copies into a retained typed array; GL consumes it synchronously.
type webGLUniformData struct{ bytes, floats js.Value }

func (data *webGLUniformData) write(values []float32) js.Value {
	if !data.bytes.Truthy() || data.bytes.Get("byteLength").Int() != len(values)*4 {
		data.bytes = js.Global().Get("Uint8Array").New(len(values) * 4)
		data.floats = js.Global().Get("Float32Array").New(data.bytes.Get("buffer"), 0, len(values))
	}
	js.CopyBytesToJS(data.bytes, sliceBytes(values))
	return data.floats
}

func webGLGeneratedUniformName(block, name string) string {
	if block == "_immediates_binding_vs" {
		switch name {
		case "cameraPosition":
			return "_immediates_binding_vs.cameraPosition"
		case "ambientLight":
			return "_immediates_binding_vs.ambientLight"
		case "lightDirection":
			return "_immediates_binding_vs.lightDirection"
		case "directionalLight":
			return "_immediates_binding_vs.directionalLight"
		}
	} else {
		switch name {
		case "cameraPosition":
			return "_immediates_binding_fs.cameraPosition"
		case "ambientLight":
			return "_immediates_binding_fs.ambientLight"
		case "lightDirection":
			return "_immediates_binding_fs.lightDirection"
		case "directionalLight":
			return "_immediates_binding_fs.directionalLight"
		}
	}
	return ""
}

func (d *webGL2Driver) setTextureParameters(filter TextureFilter, wrapU, wrapV TextureAddressMode) {
	minFilter := webGLLinear
	if filter == TextureFilterLinearMipmapLinear {
		minFilter = webGLLinearMipmapLinear
	}
	d.gl.Call("texParameteri", webGLTexture2D, webGLTextureMinFilter, minFilter)
	d.gl.Call("texParameteri", webGLTexture2D, webGLTextureMagFilter, webGLLinear)
	d.gl.Call("texParameteri", webGLTexture2D, webGLTextureWrapS, wrapU.glValue())
	d.gl.Call("texParameteri", webGLTexture2D, webGLTextureWrapT, wrapV.glValue())
}

func (retention *scratchRetention) copySliceToJS[T any](storage *js.Value, values []T) (js.Value, int) {
	data := sliceBytes(values)
	return copyBytesToJS(storage, retention, data), len(data)
}

func webGLBlendFactor(factor BlendFactor) int {
	return [...]int{
		0,
		1,
		webGLSrcColor,
		webGLOneMinusSrcColor,
		webGLSrcAlpha,
		webGLOneMinusSrcAlpha,
		webGLDstAlpha,
		webGLOneMinusDstAlpha,
		webGLDstColor,
		webGLOneMinusDstColor,
		webGLSrcAlphaSaturate,
		webGLConstantColor,
		webGLOneMinusConstantClr,
		webGLConstantAlpha,
		webGLOneMinusConstantA,
	}[factor]
}

const webGLSpriteVertex = `#version 300 es
layout(location=0) in vec3 position;
layout(location=1) in vec2 textureCoord;
layout(location=2) in vec3 model0;
layout(location=3) in vec3 model1;
layout(location=4) in vec3 texture0;
layout(location=5) in vec3 texture1;
layout(location=6) in vec4 instanceColor;
layout(location=7) in vec4 custom0;
layout(location=8) in vec4 custom1;
layout(location=9) in vec4 custom2;
layout(location=10) in vec4 custom3;
out vec2 _vs2fs_location0;
out vec4 _vs2fs_location1;
flat out vec4 _vs2fs_location2;
flat out vec4 _vs2fs_location3;
flat out vec4 _vs2fs_location4;
flat out vec4 _vs2fs_location5;
uniform mat4 projection;
void main() {
    vec3 uv = vec3(textureCoord, 1.0);
    _vs2fs_location0 = vec2(dot(texture0, uv), dot(texture1, uv));
    _vs2fs_location1 = instanceColor;
	_vs2fs_location2 = custom0;
	_vs2fs_location3 = custom1;
	_vs2fs_location4 = custom2;
	_vs2fs_location5 = custom3;
    vec3 p = vec3(position.xy, 1.0);
    gl_Position = projection * vec4(dot(model0, p), dot(model1, p), 0.0, 1.0);
}`

const webGLSpriteFragment = `#version 300 es
precision mediump float;
in vec2 _vs2fs_location0;
in vec4 _vs2fs_location1;
flat in vec4 _vs2fs_location2;
flat in vec4 _vs2fs_location3;
flat in vec4 _vs2fs_location4;
flat in vec4 _vs2fs_location5;
uniform sampler2D tex;
uniform vec4 coverage;
out vec4 color;
void main() {
    vec4 sampled = texture(tex, _vs2fs_location0);
    if (coverage.x > 0.5) sampled = vec4(1.0, 1.0, 1.0, sampled.r);
    color = sampled * _vs2fs_location1;
    color.rgb *= color.a;
}`

//go:embed shaders/gles_mesh.vert
var webGLMeshVertex string

const webGLPickingVertex = `#version 300 es
layout(location=0) in vec3 position;
layout(location=3) in vec4 model0;
layout(location=4) in vec4 model1;
layout(location=5) in vec4 model2;
layout(location=6) in vec4 model3;
layout(location=14) in vec4 custom3;
uniform mat4 projection;
flat out uint pickingID;
void main() {
	pickingID = floatBitsToUint(custom3.w);
	gl_Position = projection * mat4(model0, model1, model2, model3) * vec4(position, 1.0);
	gl_Position.z = gl_Position.z * 2.0 - gl_Position.w;
}`

const webGLPickingFragment = `#version 300 es
precision highp float;
precision highp int;
flat in uint pickingID;
layout(location=0) out uint outPickingID;
void main() {
	outPickingID = pickingID;
}`

//go:embed shaders/gles_mesh.frag
var webGLMeshFragment string

//go:embed shaders/gles_mesh_unlit.frag
var webGLMeshUnlitFragment string

//go:embed shaders/gles_mesh_depth.frag
var webGLMeshDepthFragment string

//go:embed shaders/gles_mesh_basic.frag
var webGLMeshBasicFragment string

const webGLShapeVertex = `#version 300 es
layout(location=0) in vec3 position;
layout(location=1) in vec2 localCoord;
layout(location=2) in vec3 model0;
layout(location=3) in vec3 model1;
layout(location=4) in vec3 local0;
layout(location=5) in vec3 local1;
layout(location=6) in vec4 fillColor;
layout(location=7) in vec4 borderColor;
layout(location=8) in vec4 shapeParams;
layout(location=9) in uint shapeKind;
out vec2 vLocal;
flat out vec4 vFill;
flat out vec4 vBorder;
flat out vec4 vParams;
flat out uint vKind;
uniform mat4 projection;
void main() {
    vec3 uv = vec3(localCoord, 1.0);
    vLocal = vec2(dot(local0, uv), dot(local1, uv)) * shapeParams.xy;
    vFill = fillColor;
    vBorder = borderColor;
    vParams = shapeParams;
    vKind = shapeKind;
    vec3 p = vec3(position.xy, 1.0);
    gl_Position = projection * vec4(dot(model0, p), dot(model1, p), 0.0, 1.0);
}`

const webGLShapeFragment = `#version 300 es
precision highp float;
precision highp int;
in vec2 vLocal;
flat in vec4 vFill;
flat in vec4 vBorder;
flat in vec4 vParams;
flat in uint vKind;
out vec4 color;

float sdRoundedBox(vec2 point, vec2 size, float radius) {
    radius = clamp(radius, 0.0, min(size.x, size.y) * 0.5);
    vec2 q = abs(point) - size * 0.5 + radius;
    return length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - radius;
}

float sdSegment(vec2 point, vec2 a, vec2 b) {
    vec2 pa = point - a;
    vec2 ba = b - a;
    float h = clamp(dot(pa, ba) / dot(ba, ba), 0.0, 1.0);
    return length(pa - ba * h);
}

float coverage(float distance) {
    return clamp(0.5 - distance / max(fwidth(distance), 0.0001), 0.0, 1.0);
}

vec4 premultiply(vec4 value) {
    return vec4(value.rgb * value.a, value.a);
}

void main() {
    vec2 size = vParams.xy;
    if (vKind == 2u) {
        vec2 a = size * vec2(0.18, 0.56);
        vec2 b = size * vec2(0.42, 0.78);
        vec2 c = size * vec2(0.84, 0.22);
        float thickness = max(min(size.x, size.y) * 0.14, 1.4);
        float distance = min(sdSegment(vLocal, a, b), sdSegment(vLocal, b, c)) - thickness * 0.5;
        color = premultiply(vFill) * coverage(distance);
        return;
    }
    if (vKind == 4u) {
        float direction = vParams.w;
        vec2 a, b, d;
        if (direction < 0.5) { a=vec2(size.x*.5,size.y*.40); b=vec2(size.x*.20,size.y*.73); d=vec2(size.x*.80,size.y*.73); }
        else if (direction < 1.5) { a=vec2(size.x*.20,size.y*.20); b=vec2(size.x*.80,size.y*.20); d=vec2(size.x*.5,size.y*.50); }
        else if (direction < 2.5) { a=vec2(size.x*.73,size.y*.20); b=vec2(size.x*.73,size.y*.80); d=vec2(size.x*.40,size.y*.5); }
        else { a=vec2(size.x*.27,size.y*.20); b=vec2(size.x*.27,size.y*.80); d=vec2(size.x*.60,size.y*.5); }
        float e0=(b.x-a.x)*(vLocal.y-a.y)-(b.y-a.y)*(vLocal.x-a.x);
        float e1=(d.x-b.x)*(vLocal.y-b.y)-(d.y-b.y)*(vLocal.x-b.x);
        float e2=(a.x-d.x)*(vLocal.y-d.y)-(a.y-d.y)*(vLocal.x-d.x);
        float orientation=sign((b.x-a.x)*(d.y-a.y)-(b.y-a.y)*(d.x-a.x));
        float distance=(orientation>0.0 ? -min(min(e0,e1),e2) : max(max(e0,e1),e2))/max(length(b-a),1.0);
        color=premultiply(vFill)*step(distance, 0.0);
        return;
    }

    vec2 point = vLocal - size * 0.5;
    float outer = coverage(sdRoundedBox(point, size, vParams.z));
    float borderWidth = min(max(vParams.w, 0.0), min(size.x, size.y) * 0.5);
    if (borderWidth <= 0.0) {
        color = premultiply(vFill) * outer;
        return;
    }
    vec2 innerSize = max(size - borderWidth * 2.0, vec2(0.0));
    float inner = min(innerSize.x, innerSize.y) > 0.0
        ? coverage(sdRoundedBox(point, innerSize, max(vParams.z - borderWidth, 0.0)))
        : 0.0;
    color = mix(premultiply(vBorder), premultiply(vFill), inner) * outer;
}`

func boolIntWebGL(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (d *webGL2Driver) insertTexture(texture webGLTexture) TextureHandle {
	if !d.meshTextureTable.Truthy() {
		d.meshTextureTable = js.Global().Get("Array").New()
	}
	handle := d.textures.Insert(texture)
	d.meshTextureTable.SetIndex(int(handle.ID()), texture.value)
	return handle
}

func (d *webGL2Driver) setMeshSamplers(program *webGLProgram) {
	if program.meshSamplersSet {
		return
	}
	for slot, name := range [...]string{"tex", "normalTex", "metallicRoughnessTex", "occlusionTex", "emissiveTex", "specularTex", "specularColorTex", "environmentTex", "shadowTex", "screenDepthTex", "clearcoatTex", "clearcoatRoughnessTex", "clearcoatNormalTex", "sheenColorTex", "sheenRoughnessTex", "anisotropyTex"} {
		d.uniformInt(program, name, slot)
	}
	program.meshSamplersSet = true
}

func (d *webGL2Driver) ensureMeshDrawHelper() {
	if d.meshDrawHelper.Truthy() {
		return
	}
	d.meshCommandBytes = js.Global().Get("Uint8Array").New(80)
	d.meshCommandData = js.Global().Get("Uint32Array").New(d.meshCommandBytes.Get("buffer"))
	helper := js.Global().Get("Function").New("gl", "textures", "stride", webGLMeshDrawHelper).Invoke(d.gl, d.meshTextureTable, int(unsafe.Sizeof(Instance3D{})))
	d.meshDrawHelper, d.meshDrawReset = helper.Get("draw"), helper.Get("reset")
}

const webGLMeshDrawHelper = `
const bound = new Array(16);
return {
 reset() { bound.fill(null); },
 draw(command) {
  for (let slot=0; slot<16; slot++) {
   const texture=textures[command[slot]];
   if (bound[slot] !== texture) {
    gl.activeTexture(0x84c0+slot); gl.bindTexture(0x0de1, texture); bound[slot]=texture;
   }
  }
  gl.activeTexture(0x84c0);
  const base=command[16]*stride;
  for (let attribute=3; attribute<=14; attribute++) gl.vertexAttribPointer(attribute,4,0x1406,false,stride,base+(attribute-3)*16);
  gl.drawElementsInstanced(0x0004,command[18],0x1405,command[17]*4,command[19]);
 }
};`
