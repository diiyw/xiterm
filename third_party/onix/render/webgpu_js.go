//go:build js && wasm && !wechat && !onix_directx && !onix_metal && !onix_vulkan && !onix_opengl && !onix_webgl

package render

import (
	_ "embed"
	"errors"
	"fmt"
	"syscall/js"
	"unsafe"
)

const (
	webGPUFramesInFlight = 3
	webGPUBufferMapRead  = 1
	webGPUBufferCopyDst  = 8
	webGPUBufferIndex    = 16
	webGPUBufferVertex   = 32
	webGPUBufferUniform  = 64
	webGPUBufferStorage  = 128
	webGPUTextureCopySrc = 1
	webGPUTextureCopyDst = 2
	webGPUTextureBinding = 4
	webGPUTextureRender  = 16
	webGPUShaderVertex   = 1
	webGPUShaderFragment = 2
)

const webGPUStaticUploadMaxBytes = 4 << 20

type webGPUBuffer struct {
	value    js.Value
	capacity int
}

type webGPUFrame struct {
	buffer     webGPUBuffer
	retired    []js.Value
	retention  bufferRetention
	offset     int
	generation uint64
}

func (frame *webGPUFrame) destroyRetired() {
	for index, resource := range frame.retired {
		if resource.Truthy() {
			resource.Call("destroy")
		}
		frame.retired[index] = js.Value{}
	}
	frame.retired = frame.retired[:0]
}

type webGPUTextureBindGroup struct {
	value      js.Value
	generation uint64
}

type webGPUTexturePair struct {
	extra             [MeshExtraTextureSlots]*webGPUTexture
	base              *webGPUTexture
	normal            *webGPUTexture
	metallicRoughness *webGPUTexture
}

func (pair webGPUTexturePair) textures() [3 + MeshExtraTextureSlots]*webGPUTexture {
	result := [3 + MeshExtraTextureSlots]*webGPUTexture{pair.base, pair.normal, pair.metallicRoughness}
	copy(result[3:], pair.extra[:])
	return result
}

// Each cached material owns its texture links; no per-texture maps or separate
// adjacency nodes are allocated. Removing a material unlinks all fixed slots.
type webGPUMaterialBinding struct {
	links  [3 + MeshExtraTextureSlots]webGPUMaterialLink
	groups [webGPUFramesInFlight]webGPUTextureBindGroup
}

type webGPUMaterialLink struct {
	binding        *frameCacheEntry[webGPUTexturePair, webGPUMaterialBinding]
	previous, next *webGPUMaterialLink
}

type webGPUShapeBindGroup struct {
	value      js.Value
	generation uint64
}

type webGPUTexture struct {
	value      js.Value
	view       js.Value
	sampler    js.Value
	materials  *webGPUMaterialLink
	bindGroups [webGPUFramesInFlight]webGPUTextureBindGroup
	format     TextureFormat
}

type webGPUMesh struct {
	deformationBuffer js.Value
	vertexBuffer      js.Value
	indexBuffer       js.Value
	bindGroups        *frameCacheEntry[webGPUDataKey, webGPUDataBinding]
	vertexCount       int
	indexCount        int
	deformation       meshDeformationLayout
}

type webGPUPipelineKey struct {
	blend           BlendState
	depth, coverage bool
}

type webGPUShader struct {
	meshPipelines  [3][5]js.Value
	vertex         js.Value
	fragment       js.Value
	blendPipelines map[webGPUPipelineKey]js.Value
	pipelineKind   Pipeline
}

type webGPUDepthBuffer struct {
	value  js.Value
	view   js.Value
	width  int
	height int
}

type webGPUColorBuffer struct {
	value  js.Value
	view   js.Value
	width  int
	height int
}

type webGPURenderTarget struct {
	clearValue           js.Value
	colorAttachment      js.Value
	depthAttachment      js.Value
	renderPassDescriptor js.Value
	readback             webGPUBuffer
	multisample          webGPUColorBuffer
	depth                webGPUDepthBuffer
	descriptor           RenderTargetDescriptor
	color                TextureHandle
}

type webGPUDriver struct {
	programs              [3]*webGPUShader
	meshBuiltins          [meshVariantCount - 1]*webGPUShader
	depthAttachment       js.Value
	renderPassDescriptor  js.Value
	emptyDeformation      js.Value
	meshDataLayout        js.Value
	texturePipelineLayout js.Value
	shapeLayout           js.Value
	pickingPipeline       js.Value
	textureLayout         js.Value
	quadIndex             js.Value
	quadBuffer            js.Value
	queue                 js.Value
	device                js.Value
	spritePipelineLayout  js.Value
	meshTextureLayout     js.Value
	blendConstant         js.Value
	frame                 js.Value
	shapePipelineLayout   js.Value
	colorAttachment       js.Value
	context               js.Value
	clearValue            js.Value
	dynamicOffsets        js.Value
	meshPipelineLayout    js.Value
	textureData           js.Value
	uploadData            js.Value
	shapeBindGroups       [webGPUFramesInFlight]webGPUShapeBindGroup
	format                string
	deviceName            string
	multisample           webGPUColorBuffer
	depth                 webGPUDepthBuffer
	deformationWords      []uint32
	deformationUniforms   []int
	spriteUniformOffsets  []int
	textures              GenerationPool[TextureResource, *webGPUTexture]
	material2DShaders     GenerationPool[Material2DShaderResource, *webGPUShader]
	targets               GenerationPool[RenderTargetResource, *webGPURenderTarget]
	meshShaders           GenerationPool[MeshShaderResource, *webGPUShader]
	meshes                GenerationPool[MeshResource, *webGPUMesh]
	meshBindGroups        frameCache[webGPUTexturePair, webGPUMaterialBinding]
	meshDataGroups        frameCache[webGPUDataKey, webGPUDataBinding]
	frames                [webGPUFramesInFlight]webGPUFrame
	driverStatistics
	deformationWordsRetention     scratchRetention
	spriteUniformOffsetsRetention scratchRetention
	deformationUniformsRetention  scratchRetention
	textureDataRetention          scratchRetention
	uploadDataRetention           scratchRetention
	cacheFrame                    uint64
	deformationOffset             int
	deformationSize               int
	frameIndex                    int
	height                        int
	width                         int
	sampleCount                   int
	spriteUniformData             [16 * 64]float32
	blend                         BlendState
	viewport                      Rect
	scissor                       Rect
	activeTarget                  RenderTargetHandle
	scissorSet                    bool
	astcSupported                 bool
	scissorEmpty                  bool
	viewportEmpty                 bool
	blendSet                      bool
	frameActive                   bool
	surfaceDepthEnabled           bool
}

// NewWebGPUDriver creates the browser renderer. Device acquisition is awaited
// here so the rest of the engine keeps its synchronous render-device contract.
func NewWebGPUDriver(canvas js.Value, transparent, antialias bool) (Driver, error) {
	if canvas.IsNull() || canvas.IsUndefined() {
		return nil, errors.New("render: WebGPU canvas is unavailable")
	}
	if secure := js.Global().Get("isSecureContext"); secure.Type() == js.TypeBoolean && !secure.Bool() {
		return nil, errors.New("render: WebGPU requires a secure context; use HTTPS or http://localhost")
	}
	navigator := js.Global().Get("navigator")
	if navigator.IsNull() || navigator.IsUndefined() {
		return nil, errors.New("render: WebGPU is unavailable")
	}
	gpu := navigator.Get("gpu")
	if gpu.IsNull() || gpu.IsUndefined() {
		return nil, errors.New("render: WebGPU is unavailable")
	}
	adapter, err := awaitWebGPU(gpu.Call("requestAdapter"))
	if err != nil {
		return nil, fmt.Errorf("render: request WebGPU adapter: %w", err)
	}
	if adapter.IsNull() || adapter.IsUndefined() {
		return nil, errors.New("render: WebGPU adapter is unavailable")
	}
	astcSupported := false
	features := adapter.Get("features")
	if !features.IsNull() && !features.IsUndefined() {
		astcSupported = features.Call("has", "texture-compression-astc").Bool()
	}
	var devicePromise js.Value
	if astcSupported {
		devicePromise = adapter.Call("requestDevice", webGPUObject("requiredFeatures", webGPUArray("texture-compression-astc")))
	} else {
		devicePromise = adapter.Call("requestDevice")
	}
	device, err := awaitWebGPU(devicePromise)
	if err != nil {
		return nil, fmt.Errorf("render: request WebGPU device: %w", err)
	}
	context := canvas.Call("getContext", "webgpu")
	if context.IsNull() || context.IsUndefined() {
		return nil, errors.New("render: WebGPU canvas context is unavailable")
	}
	format := gpu.Call("getPreferredCanvasFormat").String()
	configuration := webGPUObject("device", device, "format", format, "alphaMode", webGPUAlphaMode(transparent))
	context.Call("configure", configuration)
	width, height := canvas.Get("width").Int(), canvas.Get("height").Int()
	if width <= 0 || height <= 0 {
		return nil, errors.New("render: WebGPU canvas dimensions must be positive")
	}
	d := &webGPUDriver{
		device: device, queue: device.Get("queue"), context: context, format: format, astcSupported: astcSupported,
		deviceName: webGPUDeviceName(adapter), width: width, height: height,
		viewport:       Rect{Width: int32(width), Height: int32(height)},
		sampleCount:    webGPUSampleCount(antialias),
		frameIndex:     webGPUFramesInFlight - 1,
		dynamicOffsets: webGPUArray(0),
		blendConstant:  webGPUObject("r", 0, "g", 0, "b", 0, "a", 0),
	}
	d.textureLayout, d.shapeLayout = d.createBindGroupLayouts()
	d.texturePipelineLayout = d.createPipelineLayout(d.textureLayout)
	d.meshPipelineLayout = d.createPipelineLayout(d.meshTextureLayout, d.meshDataLayout)
	d.emptyDeformation = d.device.Call("createBuffer", webGPUObject("size", 4, "usage", webGPUBufferStorage|webGPUBufferCopyDst))
	d.spritePipelineLayout = d.createPipelineLayout(d.textureLayout, d.textureLayout, d.textureLayout, d.textureLayout)
	d.shapePipelineLayout = d.createPipelineLayout(d.shapeLayout)
	d.createFrameDescriptors()
	d.frame = js.Global().Get("__onixWebGPUFrame").New(d.context, d.device, d.queue, d.colorAttachment, d.depthAttachment, d.renderPassDescriptor)
	if err := d.createStaticResources(); err != nil {
		d.Destroy()
		return nil, err
	}
	programs := [3][2]string{{spriteMultiWGSL, spriteMultiWGSL}, {webGPUShapeVertex, webGPUShapeFragment}, {webGPUMeshVertex, webGPUMeshFragment}}
	for pipeline, source := range programs {
		shader, err := d.newShader(Pipeline(pipeline), source[0], source[1])
		if err != nil {
			d.Destroy()
			return nil, err
		}
		d.programs[pipeline] = shader
		if pipeline != int(PipelineMesh) {
			if _, err := d.pipeline(shader, d.blendOrDefault()); err != nil {
				d.Destroy()
				return nil, err
			}
		}
	}
	return d, nil
}

func webGPUAlphaMode(transparent bool) string {
	if transparent {
		return "premultiplied"
	}
	return "opaque"
}

func webGPUSampleCount(antialias bool) int {
	if antialias {
		return 4
	}
	return 1
}

func webGPUDeviceName(adapter js.Value) string {
	info := adapter.Get("info")
	if info.Truthy() {
		for _, key := range []string{"description", "device", "vendor"} {
			if value := info.Get(key); value.Type() == js.TypeString && value.String() != "" {
				return value.String()
			}
		}
	}
	return "WebGPU adapter"
}

func awaitWebGPU(promise js.Value) (js.Value, error) {
	if promise.IsNull() || promise.IsUndefined() {
		return js.Null(), errors.New("promise is unavailable")
	}
	type result struct {
		value js.Value
		err   error
	}
	results := make(chan result, 1)
	resolve := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		value := js.Null()
		if len(args) != 0 {
			value = args[0]
		}
		results <- result{value: value}
		return nil
	})
	reject := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		message := "unknown WebGPU promise error"
		if len(args) != 0 {
			message = webGPUErrorMessage(args[0])
		}
		results <- result{err: errors.New(message)}
		return nil
	})
	defer resolve.Release()
	defer reject.Release()
	promise.Call("then", resolve, reject)
	resolved := <-results
	return resolved.value, resolved.err
}

func webGPUErrorMessage(value js.Value) string {
	if value.IsNull() || value.IsUndefined() {
		return "unknown WebGPU error"
	}
	if message := value.Get("message"); message.Type() == js.TypeString && message.String() != "" {
		return message.String()
	}
	return value.String()
}

func webGPUObject(fields ...any) js.Value {
	object := js.Global().Get("Object").New()
	for index := 0; index+1 < len(fields); index += 2 {
		object.Set(fields[index].(string), fields[index+1])
	}
	return object
}

func webGPUArray(values ...any) js.Value {
	array := js.Global().Get("Array").New(len(values))
	for index, value := range values {
		array.SetIndex(index, value)
	}
	return array
}

func (d *webGPUDriver) RendererInfo() RendererInfo {
	return RendererInfo{Backend: "WebGPU", Device: d.deviceName}
}

func (d *webGPUDriver) trimScratch() {
	trimJSUpload(&d.textureData, &d.textureDataRetention)
	trimJSUpload(&d.uploadData, &d.uploadDataRetention)
	d.deformationWords = (&d.deformationWordsRetention).trimScratch(d.deformationWords, 1024)
	d.deformationUniforms = (&d.deformationUniformsRetention).trimScratch(d.deformationUniforms, 64)
	d.spriteUniformOffsets = (&d.spriteUniformOffsetsRetention).trimScratch(d.spriteUniformOffsets, 64)
}

func (d *webGPUDriver) BeginFrame(clear Color) error {
	if d.frameActive || d.activeTarget.Valid() {
		return errors.New("render: WebGPU frame is already active")
	}
	d.frameIndex = (d.frameIndex + 1) % webGPUFramesInFlight
	frame := &d.frames[d.frameIndex]
	frame.destroyRetired()
	if capacity := frame.retention.trimCapacity(frame.buffer.capacity, frame.offset, 1024); capacity != 0 {
		d.replaceFrameBuffer(frame, nextBufferCapacity(capacity))
	}
	frame.offset = 0
	depthView := js.Null()
	if d.surfaceDepthEnabled {
		d.ensureDepth()
		depthView = d.depth.view
	}
	resolveView := js.Null()
	if d.sampleCount > 1 {
		d.ensureMultisample()
		resolveView = d.multisample.view
	}
	d.clearValue.Set("r", clear.R)
	d.clearValue.Set("g", clear.G)
	d.clearValue.Set("b", clear.B)
	d.clearValue.Set("a", clear.A)
	d.frame.Call("begin", resolveView, depthView)
	d.frameActive = true
	d.applyViewport()
	d.applyScissor()
	d.blendSet = false
	d.trimScratch()
	d.cacheFrame++
	d.trimBindingCaches()
	d.beginFrameStatistics(d.width, d.height)
	return nil
}

func (d *webGPUDriver) SetViewport(viewport Rect) {
	d.viewport = viewport
	if d.frameActive {
		d.applyViewport()
	}
}

func (d *webGPUDriver) Viewport() Rect { return d.viewport }

func (d *webGPUDriver) SetBlend(blend BlendState) error {
	if !webGPUBlendSupported(blend) {
		return fmt.Errorf("render: WebGPU blend state is not supported: %+v", blend)
	}
	d.blend = blend
	d.blendSet = true
	d.blendConstant.Set("r", blend.Constant.R)
	d.blendConstant.Set("g", blend.Constant.G)
	d.blendConstant.Set("b", blend.Constant.B)
	d.blendConstant.Set("a", blend.Constant.A)
	return nil
}

func (d *webGPUDriver) SetScissor(scissor *Rect) {
	if scissor == nil {
		d.scissorSet = false
		d.scissorEmpty = false
	} else {
		d.scissor = *scissor
		d.scissorSet = true
		d.scissorEmpty = scissor.Width <= 0 || scissor.Height <= 0
	}
	if d.frameActive {
		d.applyScissor()
	}
}

func (d *webGPUDriver) applyScissor() {
	width, height := d.surfaceSize()
	if !d.scissorSet {
		d.scissorEmpty = false
		d.frame.Call("setScissorRect", 0, 0, width, height)
		return
	}
	x := max(0, min(d.scissor.X, int32(width)))
	y := max(0, min(d.scissor.Y, int32(height)))
	right := max(x, min(d.scissor.X+d.scissor.Width, int32(width)))
	bottom := max(y, min(d.scissor.Y+d.scissor.Height, int32(height)))
	d.scissorEmpty = right <= x || bottom <= y
	if d.scissorEmpty {
		d.frame.Call("setScissorRect", 0, 0, 1, 1)
		return
	}
	d.frame.Call("setScissorRect", x, y, right-x, bottom-y)
}

func (d *webGPUDriver) applyViewport() {
	viewport, ok := d.effectiveViewport()
	d.viewportEmpty = !ok
	if !ok {
		viewport = Rect{Width: 1, Height: 1}
	}
	d.frame.Call("setViewport", viewport.X, viewport.Y, viewport.Width, viewport.Height, 0, 1)
}

func (d *webGPUDriver) effectiveViewport() (Rect, bool) {
	width, height := d.surfaceSize()
	left := max(int64(0), int64(d.viewport.X))
	top := max(int64(0), int64(d.viewport.Y))
	right := min(int64(width), int64(d.viewport.X)+int64(d.viewport.Width))
	bottom := min(int64(height), int64(d.viewport.Y)+int64(d.viewport.Height))
	if right <= left || bottom <= top {
		return Rect{}, false
	}
	return Rect{X: int32(left), Y: int32(top), Width: int32(right - left), Height: int32(bottom - top)}, true
}

func (d *webGPUDriver) surfaceSize() (int, int) {
	if target, ok := d.targets.Get(d.activeTarget); ok && target != nil {
		return target.descriptor.Width, target.descriptor.Height
	}
	return d.width, d.height
}

func (d *webGPUDriver) transformProjection(projection [16]float32) [16]float32 {
	viewport, ok := d.effectiveViewport()
	if !ok || viewport == d.viewport {
		return projection
	}
	sx := float32(d.viewport.Width) / float32(viewport.Width)
	sy := float32(d.viewport.Height) / float32(viewport.Height)
	tx := (2*float32(d.viewport.X-viewport.X)+float32(d.viewport.Width))/float32(viewport.Width) - 1
	ty := 1 - (2*float32(d.viewport.Y-viewport.Y)+float32(d.viewport.Height))/float32(viewport.Height)
	result := projection
	for column := range 4 {
		row := column * 4
		result[row] = sx*projection[row] + tx*projection[row+3]
		result[row+1] = sy*projection[row+1] + ty*projection[row+3]
	}
	return result
}

func (d *webGPUDriver) CreateTexture(descriptor TextureDescriptor, pixels []byte) (TextureHandle, error) {
	if err := descriptor.Validate(pixels); err != nil {
		return TextureHandle{}, err
	}
	format := "rgba8unorm"
	if descriptor.Format == TextureFormatSRGBA8 {
		format = "rgba8unorm-srgb"
	}
	if descriptor.Format == TextureFormatR8 {
		format = "r8unorm"
	}
	_, compressed := descriptor.Format.GLCompressedInternalFormat()
	if compressed {
		if descriptor.Format != TextureFormatASTC4x4 && descriptor.Format != TextureFormatASTC4x4SRGB {
			return TextureHandle{}, fmt.Errorf("render: WebGPU compressed format %s is unsupported", descriptor.Format.Name())
		}
		if !d.astcSupported {
			return TextureHandle{}, errors.New("render: WebGPU ASTC texture feature is unavailable")
		}
		format = "astc-4x4-unorm"
		if descriptor.Format.SRGB() {
			format = "astc-4x4-unorm-srgb"
		}
	}
	levels := 1
	if compressed {
		levels = descriptor.MipLevels()
	}
	texture := d.device.Call("createTexture", webGPUObject(
		"size", webGPUArray(descriptor.Width, descriptor.Height, 1),
		"format", format, "mipLevelCount", levels,
		"sampleCount", 1, "dimension", "2d",
		"usage", webGPUTextureCopyDst|webGPUTextureBinding,
	))
	if texture.IsNull() || texture.IsUndefined() {
		return TextureHandle{}, errors.New("render: WebGPU could not create texture")
	}
	resource := &webGPUTexture{value: texture, view: texture.Call("createView"), format: descriptor.Format}
	mipmapFilter := "nearest"
	if descriptor.Filter == TextureFilterLinearMipmapLinear {
		mipmapFilter = "linear"
	}
	resource.sampler = d.device.Call("createSampler", webGPUObject("magFilter", "linear", "minFilter", "linear", "mipmapFilter", mipmapFilter, "addressModeU", descriptor.WrapU.webGPUName(), "addressModeV", descriptor.WrapV.webGPUName()))
	var err error
	if compressed {
		err = d.writeCompressedTexture(resource, descriptor, pixels)
	} else {
		err = d.writeTexture(resource, TextureRegion{Width: descriptor.Width, Height: descriptor.Height}, pixels)
	}
	if err != nil {
		texture.Call("destroy")
		return TextureHandle{}, err
	}
	d.recordTextureUploadStatistics(1, uint64(len(pixels)))
	return d.textures.Insert(resource), nil
}

func (d *webGPUDriver) writeCompressedTexture(texture *webGPUTexture, descriptor TextureDescriptor, pixels []byte) error {
	data := copyBytesToJS(&d.textureData, &d.textureDataRetention, pixels)
	for level, mip := range descriptor.MipChain {
		rowBytes := ((mip.Width + 3) / 4) * 16
		rows := (mip.Height + 3) / 4
		levelData := data.Call("subarray", mip.Offset, mip.Offset+mip.Bytes)
		destination := webGPUObject("texture", texture.value, "origin", webGPUArray(0, 0, 0), "mipLevel", level)
		layout := webGPUObject("offset", 0, "bytesPerRow", rowBytes, "rowsPerImage", rows)
		d.queue.Call("writeTexture", destination, levelData, layout, webGPUArray(mip.Width, mip.Height, 1))
	}
	return nil
}

func (d *webGPUDriver) writeTexture(texture *webGPUTexture, region TextureRegion, pixels []byte) error {
	bytesPerPixel := texture.format.TexelBytes()
	if bytesPerPixel == 0 || region.X < 0 || region.Y < 0 || region.Width <= 0 || region.Height <= 0 || len(pixels) != region.Width*region.Height*bytesPerPixel {
		return errors.New("render: invalid WebGPU texture update")
	}
	data := copyBytesToJS(&d.textureData, &d.textureDataRetention, pixels)
	destination := webGPUObject("texture", texture.value, "origin", webGPUArray(region.X, region.Y, 0), "mipLevel", 0)
	layout := webGPUObject("offset", 0, "bytesPerRow", region.Width*bytesPerPixel, "rowsPerImage", region.Height)
	size := webGPUArray(region.Width, region.Height, 1)
	d.queue.Call("writeTexture", destination, data, layout, size)
	return nil
}

func (d *webGPUDriver) UpdateTextures(updates []TextureUpdate) error {
	for _, update := range updates {
		texture, ok := d.textures.Get(update.Texture)
		if !ok {
			return fmt.Errorf("render: invalid WebGPU texture handle %s", update.Texture)
		}
		if err := d.writeTexture(texture, update.Region, update.Pixels); err != nil {
			return err
		}
	}
	if len(updates) != 0 {
		d.recordTextureUpdates(updates)
	}
	return nil
}

func (d *webGPUDriver) DestroyTexture(handle TextureHandle) error {
	texture, ok := d.textures.Remove(handle)
	if !ok {
		return fmt.Errorf("render: invalid WebGPU texture handle %s", handle)
	}
	d.deleteTexture(texture, false)
	return nil
}

func (d *webGPUDriver) CreateRenderTarget(descriptor RenderTargetDescriptor) (RenderTargetHandle, TextureHandle, error) {
	if err := descriptor.Validate(); err != nil {
		return RenderTargetHandle{}, TextureHandle{}, err
	}
	if d.activeTarget.Valid() {
		return RenderTargetHandle{}, TextureHandle{}, errors.New("render: cannot create a WebGPU render target during a target pass")
	}
	target, err := d.createRenderTarget(descriptor)
	if err != nil {
		return RenderTargetHandle{}, TextureHandle{}, err
	}
	return d.targets.Insert(target), target.color, nil
}

func (d *webGPUDriver) ResizeRenderTarget(handle RenderTargetHandle, descriptor RenderTargetDescriptor) (TextureHandle, error) {
	if err := descriptor.Validate(); err != nil {
		return TextureHandle{}, err
	}
	target, ok := d.targets.Get(handle)
	if !ok || target == nil || d.activeTarget.Valid() {
		return TextureHandle{}, fmt.Errorf("render: invalid WebGPU render target resize %s", handle)
	}
	replacement, err := d.createRenderTarget(descriptor)
	if err != nil {
		return TextureHandle{}, err
	}
	d.deleteRenderTarget(target, true)
	*target = *replacement
	return target.color, nil
}

func (d *webGPUDriver) DestroyRenderTarget(handle RenderTargetHandle) error {
	target, ok := d.targets.Get(handle)
	if !ok || target == nil || d.activeTarget == handle {
		return fmt.Errorf("render: invalid WebGPU render target destruction %s", handle)
	}
	d.deleteRenderTarget(target, true)
	d.targets.Remove(handle)
	return nil
}

func (d *webGPUDriver) BeginRenderTarget(handle RenderTargetHandle, clear RenderTargetClear) error {
	target, ok := d.targets.Get(handle)
	if !d.frameActive || !ok || target == nil || d.activeTarget.Valid() {
		return fmt.Errorf("render: invalid WebGPU render target begin %s", handle)
	}
	target.clearValue.Set("r", clear.Color.R)
	target.clearValue.Set("g", clear.Color.G)
	target.clearValue.Set("b", clear.Color.B)
	target.clearValue.Set("a", clear.Color.A)
	if clear.ClearColor {
		target.colorAttachment.Set("loadOp", "clear")
	} else {
		target.colorAttachment.Set("loadOp", "load")
	}
	if target.descriptor.Depth {
		target.depthAttachment.Set("depthClearValue", clear.Depth)
		if clear.ClearDepth {
			target.depthAttachment.Set("depthLoadOp", "clear")
		} else {
			target.depthAttachment.Set("depthLoadOp", "load")
		}
	}
	d.frame.Call("beginTarget", target.renderPassDescriptor)
	d.activeTarget = handle
	d.applyViewport()
	d.applyScissor()
	return nil
}

func (d *webGPUDriver) EndRenderTarget() error {
	if !d.frameActive || !d.activeTarget.Valid() {
		return errors.New("render: no WebGPU render target is active")
	}
	d.frame.Call("endTarget")
	d.activeTarget = RenderTargetHandle{}
	d.applyViewport()
	d.applyScissor()
	return nil
}

func (d *webGPUDriver) ReadRenderTarget(handle RenderTargetHandle, region RenderTargetRegion, destination []byte) error {
	target, ok := d.targets.Get(handle)
	if d.frameActive || d.activeTarget.Valid() || !ok || target == nil {
		return fmt.Errorf("render: invalid WebGPU render target readback %s", handle)
	}
	texture := d.textureForDraw(target.color)
	if texture == nil || !texture.value.Truthy() {
		return errors.New("render: WebGPU render target color texture is unavailable")
	}
	rowBytes := region.Width * target.descriptor.ColorFormat.TexelBytes()
	bytesPerRow := align(rowBytes, 256)
	required := bytesPerRow * region.Height
	if !target.readback.value.Truthy() || target.readback.capacity < required {
		if target.readback.value.Truthy() {
			target.readback.value.Call("destroy")
		}
		capacity := align(nextBufferCapacity(required), 256)
		target.readback = webGPUBuffer{
			value:    d.device.Call("createBuffer", webGPUObject("size", capacity, "usage", webGPUBufferMapRead|webGPUBufferCopyDst)),
			capacity: capacity,
		}
	}
	encoder := d.device.Call("createCommandEncoder")
	encoder.Call("copyTextureToBuffer",
		webGPUObject("texture", texture.value, "origin", webGPUObject("x", region.X, "y", region.Y, "z", 0)),
		webGPUObject("buffer", target.readback.value, "offset", 0, "bytesPerRow", bytesPerRow, "rowsPerImage", region.Height),
		webGPUObject("width", region.Width, "height", region.Height, "depthOrArrayLayers", 1),
	)
	d.queue.Call("submit", webGPUArray(encoder.Call("finish")))
	if _, err := awaitWebGPU(target.readback.value.Call("mapAsync", webGPUBufferMapRead, 0, required)); err != nil {
		return fmt.Errorf("render: WebGPU render target map: %w", err)
	}
	mapped := target.readback.value.Call("getMappedRange", 0, required)
	bytes := js.Global().Get("Uint8Array").New(mapped)
	for row := range region.Height {
		source := row * bytesPerRow
		js.CopyBytesToGo(destination[row*rowBytes:(row+1)*rowBytes], bytes.Call("subarray", source, source+rowBytes))
	}
	target.readback.value.Call("unmap")
	return nil
}

func (d *webGPUDriver) createRenderTarget(descriptor RenderTargetDescriptor) (*webGPURenderTarget, error) {
	format := d.format
	sampleCount := d.sampleCount
	if descriptor.ColorFormat == RenderTargetUint32 {
		format = "r32uint"
		sampleCount = 1
	} else if descriptor.ColorFormat == RenderTargetR8 {
		format, sampleCount = "r8unorm", 1
	}
	colorValue := d.device.Call("createTexture", webGPUObject(
		"size", webGPUArray(descriptor.Width, descriptor.Height, 1), "format", format,
		"sampleCount", 1, "dimension", "2d", "usage", webGPUTextureCopySrc|webGPUTextureBinding|webGPUTextureRender,
	))
	if !colorValue.Truthy() {
		return nil, errors.New("render: WebGPU could not create render target color texture")
	}
	colorTexture := &webGPUTexture{value: colorValue, view: colorValue.Call("createView")}
	if descriptor.ColorFormat == RenderTargetR8 {
		colorTexture.format = TextureFormatR8
	}
	filter := "linear"
	if descriptor.ColorFormat == RenderTargetUint32 {
		filter = "nearest"
	}
	colorTexture.sampler = d.device.Call("createSampler", webGPUObject("magFilter", filter, "minFilter", filter, "addressModeU", "clamp-to-edge", "addressModeV", "clamp-to-edge"))
	target := &webGPURenderTarget{descriptor: descriptor}
	target.color = d.textures.Insert(colorTexture)
	colorView := colorTexture.view
	resolveView := js.Undefined()
	if sampleCount > 1 {
		value := d.device.Call("createTexture", webGPUObject(
			"size", webGPUArray(descriptor.Width, descriptor.Height, 1), "format", format,
			"sampleCount", sampleCount, "usage", webGPUTextureRender,
		))
		target.multisample = webGPUColorBuffer{value: value, view: value.Call("createView"), width: descriptor.Width, height: descriptor.Height}
		colorView, resolveView = target.multisample.view, colorTexture.view
	}
	target.clearValue = webGPUObject("r", 0, "g", 0, "b", 0, "a", 0)
	target.colorAttachment = webGPUObject("view", colorView, "clearValue", target.clearValue, "loadOp", "clear", "storeOp", "store")
	if resolveView.Truthy() {
		target.colorAttachment.Set("resolveTarget", resolveView)
	}
	if descriptor.Depth {
		value := d.device.Call("createTexture", webGPUObject(
			"size", webGPUArray(descriptor.Width, descriptor.Height, 1), "format", "depth24plus",
			"sampleCount", sampleCount, "usage", webGPUTextureRender,
		))
		target.depth = webGPUDepthBuffer{value: value, view: value.Call("createView"), width: descriptor.Width, height: descriptor.Height}
		target.depthAttachment = webGPUObject("view", target.depth.view, "depthClearValue", 0, "depthLoadOp", "clear", "depthStoreOp", "store")
		target.renderPassDescriptor = webGPUObject("colorAttachments", webGPUArray(target.colorAttachment), "depthStencilAttachment", target.depthAttachment)
	} else {
		target.renderPassDescriptor = webGPUObject("colorAttachments", webGPUArray(target.colorAttachment))
	}
	return target, nil
}

func (d *webGPUDriver) deleteTexture(texture *webGPUTexture, retire bool) {
	if texture == nil {
		return
	}
	for texture.materials != nil {
		d.meshBindGroups.remove(texture.materials.binding, unlinkWebGPUMaterial)
	}
	if retire && d.frameActive {
		d.frames[d.frameIndex].retired = append(d.frames[d.frameIndex].retired, texture.value)
	} else if texture.value.Truthy() {
		texture.value.Call("destroy")
	}
}

func (d *webGPUDriver) deleteRenderTarget(target *webGPURenderTarget, removeColor bool) {
	if target == nil {
		return
	}
	retire := d.frameActive
	for _, value := range []js.Value{target.multisample.value, target.depth.value} {
		if !value.Truthy() {
			continue
		}
		if retire {
			d.frames[d.frameIndex].retired = append(d.frames[d.frameIndex].retired, value)
		} else {
			value.Call("destroy")
		}
	}
	if target.readback.value.Truthy() {
		target.readback.value.Call("destroy")
	}
	if removeColor {
		if texture, ok := d.textures.Remove(target.color); ok {
			d.deleteTexture(texture, retire)
		}
	}
	target.color = TextureHandle{}
	target.multisample = webGPUColorBuffer{}
	target.depth = webGPUDepthBuffer{}
	target.readback = webGPUBuffer{}
}

func (d *webGPUDriver) DrawSprites(batch SpriteBatch) error {
	if len(batch.Instances) == 0 || len(batch.Draws) == 0 {
		return nil
	}
	if !d.frameActive {
		return errors.New("render: WebGPU draw is outside a frame")
	}
	if d.scissorEmpty || d.viewportEmpty {
		return nil
	}
	d.frame.Call("setBlendConstant", d.blendConstant)
	scratch := spriteUploadBuffers.get()
	defer spriteUploadBuffers.put(scratch)
	scratch.data = scratch.data[:0]
	var sprites uint64
	for _, draw := range batch.Draws {
		sprites += uint64(draw.InstanceCount)
		if draw.InstanceCount == 0 || uint64(draw.FirstInstance)+uint64(draw.InstanceCount) > uint64(len(batch.Instances)) {
			return errors.New("render: invalid WebGPU sprite draw range")
		}
		for slot := range SpriteTextureSlots {
			if _, ok := d.textures.Get(draw.TextureAt(slot)); !ok {
				return fmt.Errorf("render: invalid WebGPU texture handle %s", draw.TextureAt(slot))
			}
		}
		if draw.Shader.Valid() {
			if _, ok := d.material2DShaders.Get(draw.Shader); !ok {
				return fmt.Errorf("render: invalid WebGPU Material2D shader handle %s", draw.Shader)
			}
		}
		scratch.data = appendSpriteDrawInstances(scratch.data, batch.Instances, draw)
	}
	instanceOffset, uniformOffsets := d.uploadSprite2D(batch.Projection, scratch.data, batch.Draws)
	frame := &d.frames[d.frameIndex]
	d.frame.Call("setVertexBuffer", 0, d.quadBuffer)
	packedOffset := 0
	d.frame.Call("setIndexBuffer", d.quadIndex, "uint16")
	var current *webGPUShader
	for index, draw := range batch.Draws {
		program := d.programs[PipelineSprite]
		if draw.Shader.Valid() {
			var ok bool
			program, ok = d.material2DShaders.Get(draw.Shader)
			if !ok {
				return fmt.Errorf("render: invalid WebGPU Material2D shader handle %s", draw.Shader)
			}
		}
		texture, _ := d.textures.Get(draw.Texture)
		pipeline, err := d.pipeline(program, d.blendOrDefault())
		if err != nil {
			return err
		}
		if program != current {
			d.frame.Call("setPipeline", pipeline)
			current = program
		}
		d.setTextureBindGroup(texture, uniformOffsets[index])
		if !draw.Shader.Valid() {
			for slot := 1; slot < SpriteTextureSlots; slot++ {
				texture, _ := d.textures.Get(draw.TextureAt(slot))
				d.frame.Call("setBindGroup", slot, d.textureBindGroup(texture), d.dynamicOffsets)
			}
		}
		packedOffset = int(spriteInstanceOffset(uint64(packedOffset), draw))
		firstInstance := uint32(0)
		bindingOffset := instanceOffset + packedOffset
		if draw.Shader.Valid() {
			firstInstance = draw.FirstInstance
			bindingOffset -= int(firstInstance) * spriteCustomInstanceBytes
		}
		d.frame.Call("setVertexBuffer", 1, frame.buffer.value, bindingOffset)
		d.frame.Call("drawIndexed", 6, draw.InstanceCount, 0, 0, firstInstance)
		packedOffset += int(draw.InstanceCount) * spriteInstanceStride(draw.Shader.Valid())
	}
	d.recordSpriteStatistics(batch, sprites, uint64(len(scratch.data)))
	return nil
}

func (d *webGPUDriver) DrawShapes(batch ShapeBatch) error {
	if len(batch.Instances) == 0 {
		return nil
	}
	if !d.frameActive {
		return errors.New("render: WebGPU draw is outside a frame")
	}
	if d.scissorEmpty || d.viewportEmpty {
		return nil
	}
	instanceOffset, uniformOffset := d.upload2D(batch.Projection, batch.Instances)
	frame := &d.frames[d.frameIndex]
	program := d.programs[PipelineShape]
	pipeline, err := d.pipeline(program, d.blendOrDefault())
	if err != nil {
		return err
	}
	d.frame.Call("setPipeline", pipeline)
	d.frame.Call("setVertexBuffer", 0, d.quadBuffer)
	d.frame.Call("setVertexBuffer", 1, frame.buffer.value, instanceOffset)
	d.frame.Call("setIndexBuffer", d.quadIndex, "uint16")
	d.setShapeBindGroup(uniformOffset)
	d.frame.Call("drawIndexed", 6, len(batch.Instances), 0, 0, 0)
	d.recordShapeStatistics(batch)
	return nil
}

func (d *webGPUDriver) CreateMesh(vertices []Vertex3D, indices []uint32) (MeshHandle, error) {
	if len(vertices) == 0 || len(indices) == 0 {
		return MeshHandle{}, errors.New("render: WebGPU mesh data is empty")
	}
	vertexBuffer := d.device.Call("createBuffer", webGPUObject("size", roundup4(len(sliceBytes(vertices))), "usage", webGPUBufferVertex|webGPUBufferCopyDst))
	indexBuffer := d.device.Call("createBuffer", webGPUObject("size", roundup4(len(sliceBytes(indices))), "usage", webGPUBufferIndex|webGPUBufferCopyDst))
	if vertexBuffer.IsNull() || indexBuffer.IsNull() {
		if vertexBuffer.Truthy() {
			vertexBuffer.Call("destroy")
		}
		if indexBuffer.Truthy() {
			indexBuffer.Call("destroy")
		}
		return MeshHandle{}, errors.New("render: WebGPU could not create mesh buffers")
	}
	d.writeStaticBuffer(vertexBuffer, sliceBytes(vertices))
	d.writeStaticBuffer(indexBuffer, sliceBytes(indices))
	return d.meshes.Insert(&webGPUMesh{vertexBuffer: vertexBuffer, indexBuffer: indexBuffer, indexCount: len(indices), vertexCount: len(vertices)}), nil
}

func (d *webGPUDriver) DestroyMesh(handle MeshHandle) error {
	mesh, ok := d.meshes.Remove(handle)
	if !ok {
		return fmt.Errorf("render: invalid WebGPU mesh handle %s", handle)
	}
	for _, buffer := range [3]js.Value{mesh.vertexBuffer, mesh.indexBuffer, mesh.deformationBuffer} {
		if !buffer.Truthy() {
			continue
		}
		if d.frameActive {
			d.frames[d.frameIndex].retired = append(d.frames[d.frameIndex].retired, buffer)
		} else {
			buffer.Call("destroy")
		}
	}
	for mesh.bindGroups != nil {
		d.meshDataGroups.remove(mesh.bindGroups, unlinkWebGPUData)
	}
	return nil
}

func (d *webGPUDriver) DrawMeshes(batch MeshBatch3D) error {
	return d.drawMeshes(batch, false)
}

func (d *webGPUDriver) DrawMeshPicking(batch MeshBatch3D) error {
	if len(batch.Instances) == 0 || len(batch.Draws) == 0 {
		return nil
	}
	target, ok := d.targets.Get(d.activeTarget)
	if !d.frameActive || !ok || target == nil || (target.descriptor.ColorFormat != RenderTargetUint32 || !target.descriptor.Depth) {
		return errors.New("render: WebGPU mesh picking requires an active R32Uint target with depth")
	}
	if d.scissorEmpty || d.viewportEmpty {
		return nil
	}
	dynamic := len(batch.Vertices) != 0 || len(batch.Indices) != 0
	if dynamic && (len(batch.Vertices) == 0 || len(batch.Indices) == 0) {
		return errors.New("render: incomplete WebGPU dynamic picking mesh data")
	}
	for _, draw := range batch.Draws {
		indexCount := len(batch.Indices)
		if draw.Mesh.Valid() {
			mesh, exists := d.meshes.Get(draw.Mesh)
			if !exists {
				return fmt.Errorf("render: invalid WebGPU picking mesh handle %s", draw.Mesh)
			}
			indexCount = mesh.indexCount
		} else if !dynamic {
			return errors.New("render: WebGPU picking draw has no geometry")
		}
		if draw.IndexCount == 0 || uint64(draw.FirstIndex)+uint64(draw.IndexCount) > uint64(indexCount) ||
			draw.InstanceCount == 0 || uint64(draw.FirstInstance)+uint64(draw.InstanceCount) > uint64(len(batch.Instances)) {
			return errors.New("render: invalid WebGPU picking draw range")
		}
	}
	if !d.pickingPipeline.Truthy() {
		if err := d.createPickingPipeline(); err != nil {
			return err
		}
	}
	vertexOffset, indexOffset, instanceOffset, uniformOffset := d.uploadMesh(batch, false)
	frame := &d.frames[d.frameIndex]
	d.frame.Call("setVertexBuffer", 1, frame.buffer.value, instanceOffset)
	d.frame.Call("setPipeline", d.pickingPipeline)
	for _, draw := range batch.Draws {
		if draw.Mesh.Valid() {
			mesh, _ := d.meshes.Get(draw.Mesh)
			d.frame.Call("setVertexBuffer", 0, mesh.vertexBuffer)
			d.frame.Call("setIndexBuffer", mesh.indexBuffer, "uint32")
		} else {
			d.frame.Call("setVertexBuffer", 0, frame.buffer.value, vertexOffset)
			d.frame.Call("setIndexBuffer", frame.buffer.value, "uint32", indexOffset)
		}
		d.setShapeBindGroup(uniformOffset)
		d.frame.Call("drawIndexed", draw.IndexCount, draw.InstanceCount, draw.FirstIndex, draw.BaseVertex, draw.FirstInstance)
	}
	return nil
}

func (d *webGPUDriver) DrawMeshesDeformed(batch MeshBatch3D) error {
	return d.drawMeshes(batch, true)
}

func (d *webGPUDriver) drawMeshes(batch MeshBatch3D, gpuDeformation bool) error {
	if len(batch.Instances) == 0 || len(batch.Draws) == 0 {
		return nil
	}
	if !d.frameActive {
		return errors.New("render: WebGPU draw is outside a frame")
	}
	if d.scissorEmpty || d.viewportEmpty {
		return nil
	}
	dynamic := len(batch.Vertices) != 0 || len(batch.Indices) != 0
	if dynamic && (len(batch.Vertices) == 0 || len(batch.Indices) == 0) {
		return errors.New("render: incomplete WebGPU dynamic mesh data")
	}
	var counts meshDrawStatistics
	for _, draw := range batch.Draws {
		counts.add(draw)
		indexCount := len(batch.Indices)
		if draw.Mesh.Valid() {
			mesh, ok := d.meshes.Get(draw.Mesh)
			if !ok {
				return fmt.Errorf("render: invalid WebGPU mesh handle %s", draw.Mesh)
			}
			indexCount = mesh.indexCount
		} else if !dynamic {
			return errors.New("render: WebGPU mesh draw has no geometry")
		}
		if draw.IndexCount == 0 || uint64(draw.FirstIndex)+uint64(draw.IndexCount) > uint64(indexCount) || draw.InstanceCount == 0 || uint64(draw.FirstInstance)+uint64(draw.InstanceCount) > uint64(len(batch.Instances)) || draw.AlphaMode > 2 || draw.BlendMode > MeshBlendScreen {
			return errors.New("render: invalid WebGPU mesh draw range")
		}
		if _, ok := d.textures.Get(draw.Texture); !ok {
			return fmt.Errorf("render: invalid WebGPU texture handle %s", draw.Texture)
		}
		if draw.NormalTexture.Valid() {
			if _, ok := d.textures.Get(draw.NormalTexture); !ok {
				return fmt.Errorf("render: invalid WebGPU normal texture handle %s", draw.NormalTexture)
			}
		}
		if draw.MetallicRoughnessTexture.Valid() {
			if _, ok := d.textures.Get(draw.MetallicRoughnessTexture); !ok {
				return fmt.Errorf("render: invalid WebGPU metallic roughness texture handle %s", draw.MetallicRoughnessTexture)
			}
		}
		if draw.Shader.Valid() {
			if _, ok := d.meshShaders.Get(draw.Shader); !ok {
				return fmt.Errorf("render: invalid WebGPU mesh shader handle %s", draw.Shader)
			}
		}
	}
	if gpuDeformation {
		var err error
		d.deformationWords = (&d.deformationWordsRetention).resetScratch(d.deformationWords)
		d.deformationWords, err = packMeshDeformations(d.deformationWords, batch, d)
		if err != nil {
			return fmt.Errorf("render: invalid WebGPU deformation: %w", err)
		}
	}
	d.requireSurfaceDepth()
	vertexOffset, indexOffset, instanceOffset, uniformOffset := d.uploadMesh(batch, gpuDeformation)
	frame := &d.frames[d.frameIndex]
	if dynamic {
		d.frame.Call("setVertexBuffer", 0, frame.buffer.value, vertexOffset)
		d.frame.Call("setIndexBuffer", frame.buffer.value, "uint32", indexOffset)
	}
	d.frame.Call("setVertexBuffer", 1, frame.buffer.value, instanceOffset)
	transparent := false
	blendMode := MeshBlendMode(^uint32(0))
	for _, draw := range batch.Draws {
		if draw.Mesh.Valid() {
			mesh, _ := d.meshes.Get(draw.Mesh)
			d.frame.Call("setVertexBuffer", 0, mesh.vertexBuffer)
			d.frame.Call("setIndexBuffer", mesh.indexBuffer, "uint32")
		} else {
			d.frame.Call("setVertexBuffer", 0, frame.buffer.value, vertexOffset)
			d.frame.Call("setIndexBuffer", frame.buffer.value, "uint32", indexOffset)
		}
		program := d.programs[PipelineMesh]
		if draw.Shader.Valid() {
			program, _ = d.meshShaders.Get(draw.Shader)
		} else if variant := meshBuiltinVariant(&batch, &draw); variant != meshVariantFull {
			program = d.meshBuiltins[variant-1]
			if program == nil {
				source := [meshVariantCount - 1]string{webGPUMeshUnlitFragment, webGPUMeshDepthFragment, webGPUMeshBasicFragment}[variant-1]
				var err error
				program, err = d.newShader(PipelineMesh, "", source)
				if err != nil {
					return err
				}
				d.meshBuiltins[variant-1] = program
			}
		}
		isTransparent := draw.AlphaMode == 2
		if isTransparent != transparent {
			transparent = isTransparent
		}
		if isTransparent {
			blendMode = draw.BlendMode
		}
		pipeline, err := d.meshPipeline(program, boolInt(isTransparent), blendModeOrNormal(blendMode))
		if err != nil {
			return err
		}
		d.frame.Call("setPipeline", pipeline)
		normal := d.textureForDraw(draw.Texture)
		if draw.NormalTexture.Valid() {
			normal = d.textureForDraw(draw.NormalTexture)
		}
		metallicRoughness := d.textureForDraw(draw.Texture)
		if draw.MetallicRoughnessTexture.Valid() {
			metallicRoughness = d.textureForDraw(draw.MetallicRoughnessTexture)
		}
		drawUniformOffset := uniformOffset
		if gpuDeformation && draw.DeformationIndex != 0 {
			drawUniformOffset = d.deformationUniforms[draw.DeformationIndex-1]
		}
		var extra [MeshExtraTextureSlots]*webGPUTexture
		for slot, handle := range draw.ExtraTextures(&batch.Lighting) {
			extra[slot] = d.textureForDraw(draw.Texture)
			if handle.Valid() {
				if _, ok := d.textures.Get(handle); !ok {
					return fmt.Errorf("render: invalid surface texture %s", handle)
				}
				extra[slot] = d.textureForDraw(handle)
			}
		}
		d.setMeshTextureBindGroup(d.textureForDraw(draw.Texture), normal, metallicRoughness, extra, drawUniformOffset)
		var deformationMesh *webGPUMesh
		if gpuDeformation && draw.DeformationIndex != 0 {
			deformationMesh, _ = d.meshes.Get(draw.Mesh)
		}
		d.setMeshDataBindGroup(deformationMesh)
		d.frame.Call("drawIndexed", draw.IndexCount, draw.InstanceCount, draw.FirstIndex, draw.BaseVertex, draw.FirstInstance)
	}
	d.frame.Call("setStencilReference", 0)
	d.recordMeshStatistics(batch, counts)
	return nil
}

func (d *webGPUDriver) CreateMaterial2DShader(descriptor ShaderDescriptor) (Material2DShaderHandle, error) {
	shader, err := d.createCustomShader(PipelineSprite, descriptor, "Material2D")
	if err != nil {
		return Material2DShaderHandle{}, err
	}
	if _, err := d.pipeline(shader, d.blendOrDefault()); err != nil {
		return Material2DShaderHandle{}, err
	}
	return d.material2DShaders.Insert(shader), nil
}

func (d *webGPUDriver) DestroyMaterial2DShader(handle Material2DShaderHandle) error {
	if _, ok := d.material2DShaders.Remove(handle); !ok {
		return fmt.Errorf("render: invalid WebGPU Material2D shader handle %s", handle)
	}
	return nil
}

func (d *webGPUDriver) CreateMeshShader(descriptor ShaderDescriptor) (MeshShaderHandle, error) {
	shader, err := d.createCustomShader(PipelineMesh, descriptor, "mesh")
	if err != nil {
		return MeshShaderHandle{}, err
	}
	return d.meshShaders.Insert(shader), nil
}

func (d *webGPUDriver) DestroyMeshShader(handle MeshShaderHandle) error {
	if _, ok := d.meshShaders.Remove(handle); !ok {
		return fmt.Errorf("render: invalid WebGPU mesh shader handle %s", handle)
	}
	return nil
}

func (d *webGPUDriver) createCustomShader(kind Pipeline, descriptor ShaderDescriptor, label string) (*webGPUShader, error) {
	if err := descriptor.Validate(); err != nil {
		return nil, err
	}
	vertex, fragment := descriptor.WebGPUVertexSource, descriptor.WebGPUFragmentSource
	if vertex == "" && fragment == "" {
		return nil, fmt.Errorf("render: %s shader has no WebGPU source", label)
	}
	if kind == PipelineSprite {
		if vertex == "" {
			vertex = webGPUSpriteVertex
		}
		if fragment == "" {
			fragment = webGPUSpriteFragment
		}
	}
	shader, err := d.newShader(kind, vertex, fragment)
	if err != nil {
		return nil, err
	}
	return shader, nil
}

func (d *webGPUDriver) newShader(kind Pipeline, vertexSource, fragmentSource string) (*webGPUShader, error) {
	shader := &webGPUShader{pipelineKind: kind, blendPipelines: make(map[webGPUPipelineKey]js.Value)}
	if base := d.programs[kind]; base != nil {
		shader.vertex, shader.fragment = base.vertex, base.fragment
	}
	if vertexSource != "" {
		shader.vertex = d.device.Call("createShaderModule", webGPUObject("code", vertexSource))
	}
	if fragmentSource != "" {
		shader.fragment = d.device.Call("createShaderModule", webGPUObject("code", fragmentSource))
	}
	if !shader.vertex.Truthy() || !shader.fragment.Truthy() {
		return nil, errors.New("render: WebGPU shader is missing a stage")
	}
	return shader, nil
}

func (d *webGPUDriver) Present() error {
	if !d.frameActive {
		return nil
	}
	if d.activeTarget.Valid() {
		return errors.New("render: cannot present while a WebGPU render target is active")
	}
	d.frame.Call("present")
	d.frameActive = false
	return nil
}

func (d *webGPUDriver) ResizeSurface(width, height int) error {
	if width <= 0 || height <= 0 {
		return errors.New("render: WebGPU surface dimensions must be positive")
	}
	if d.width == width && d.height == height {
		return nil
	}
	d.width, d.height = width, height
	if d.depth.value.Truthy() {
		d.depth.value.Call("destroy")
	}
	d.depth = webGPUDepthBuffer{}
	if d.multisample.value.Truthy() {
		d.multisample.value.Call("destroy")
	}
	d.multisample = webGPUColorBuffer{}
	if d.viewport.Width <= 0 || d.viewport.Height <= 0 {
		d.viewport = Rect{Width: int32(width), Height: int32(height)}
	}
	return nil
}

func (d *webGPUDriver) SetVSync(bool) {}

func (d *webGPUDriver) Destroy() {
	if d.frame.Truthy() {
		d.frame.Call("destroy")
		d.frame = js.Undefined()
		d.frameActive = false
	}
	if d.depth.value.Truthy() {
		d.depth.value.Call("destroy")
	}
	if d.multisample.value.Truthy() {
		d.multisample.value.Call("destroy")
	}
	if d.quadBuffer.Truthy() {
		d.quadBuffer.Call("destroy")
	}
	if d.quadIndex.Truthy() {
		d.quadIndex.Call("destroy")
	}
	d.targets.Range(func(target *webGPURenderTarget) bool {
		d.deleteRenderTarget(target, false)
		return true
	})
	d.textures.Range(func(texture *webGPUTexture) bool {
		texture.value.Call("destroy")
		return true
	})
	d.meshes.Range(func(mesh *webGPUMesh) bool {
		mesh.vertexBuffer.Call("destroy")
		mesh.indexBuffer.Call("destroy")
		if mesh.deformationBuffer.Truthy() {
			mesh.deformationBuffer.Call("destroy")
		}
		return true
	})
	for index := range d.frames {
		if d.frames[index].buffer.value.Truthy() {
			d.frames[index].buffer.value.Call("destroy")
		}
		d.frames[index].destroyRetired()
		d.frames[index].retired = nil
	}
	if d.emptyDeformation.Truthy() {
		d.emptyDeformation.Call("destroy")
	}
	d.meshDataGroups.clear(unlinkWebGPUData)
	d.meshBindGroups.clear(unlinkWebGPUMaterial)
	d.textures.Clear()
	d.targets.Clear()
	d.meshes.Clear()
	d.meshBuiltins = [meshVariantCount - 1]*webGPUShader{}
	d.meshShaders.Clear()
	d.material2DShaders.Clear()
	if d.context.Truthy() {
		d.context.Call("unconfigure")
	}
	if d.device.Truthy() {
		d.device.Call("destroy")
	}
	d.uploadData = js.Undefined()
	d.textureData = js.Undefined()
}

func (d *webGPUDriver) createStaticResources() error {
	quad := []float32{0, 0, 0, 0, 0, 1, 0, 0, 1, 0, 1, 1, 0, 1, 1, 0, 1, 0, 0, 1}
	indices := []uint16{0, 1, 2, 0, 2, 3}
	d.quadBuffer = d.device.Call("createBuffer", webGPUObject("size", len(sliceBytes(quad)), "usage", webGPUBufferVertex|webGPUBufferCopyDst))
	d.quadIndex = d.device.Call("createBuffer", webGPUObject("size", len(sliceBytes(indices)), "usage", webGPUBufferIndex|webGPUBufferCopyDst))
	if d.quadBuffer.IsNull() || d.quadIndex.IsNull() {
		return errors.New("render: WebGPU could not create quad buffers")
	}
	d.writeStaticBuffer(d.quadBuffer, sliceBytes(quad))
	d.writeStaticBuffer(d.quadIndex, sliceBytes(indices))
	for index := range d.frames {
		d.frames[index].buffer = d.createFrameBuffer(1024)
		d.frames[index].generation = 1
	}
	return nil
}

func (d *webGPUDriver) createFrameDescriptors() {
	d.clearValue = webGPUObject("r", 0, "g", 0, "b", 0, "a", 1)
	d.colorAttachment = webGPUObject("view", js.Null(), "clearValue", d.clearValue, "loadOp", "clear", "storeOp", "store")
	d.depthAttachment = webGPUObject("view", js.Null(), "depthClearValue", 0, "depthLoadOp", "clear", "depthStoreOp", "store")
	d.renderPassDescriptor = webGPUObject("colorAttachments", webGPUArray(d.colorAttachment), "depthStencilAttachment", d.depthAttachment)
}

func (d *webGPUDriver) createFrameBuffer(capacity int) webGPUBuffer {
	return webGPUBuffer{value: d.device.Call("createBuffer", webGPUObject("size", capacity, "usage", webGPUBufferCopyDst|webGPUBufferVertex|webGPUBufferIndex|webGPUBufferUniform|webGPUBufferStorage)), capacity: capacity}
}

func (d *webGPUDriver) hasDepth() bool {
	if target, ok := d.targets.Get(d.activeTarget); ok && target != nil {
		return target.descriptor.Depth
	}
	return d.surfaceDepthEnabled
}

func (d *webGPUDriver) requireSurfaceDepth() {
	if d.activeTarget.Valid() || d.surfaceDepthEnabled {
		return
	}
	d.ensureDepth()
	d.surfaceDepthEnabled = true
	// The color attachment (including MSAA storage) is preserved across passes.
	d.frame.Call("requireDepth", d.depth.view)
	d.applyViewport()
	d.applyScissor()
}

func (d *webGPUDriver) ensureDepth() {
	if d.depth.value.Truthy() && d.depth.width == d.width && d.depth.height == d.height {
		return
	}
	if d.depth.value.Truthy() {
		d.depth.value.Call("destroy")
	}
	value := d.device.Call("createTexture", webGPUObject("size", webGPUArray(d.width, d.height, 1), "format", "depth24plus", "sampleCount", d.sampleCount, "usage", webGPUTextureRender))
	d.depth = webGPUDepthBuffer{value: value, view: value.Call("createView"), width: d.width, height: d.height}
}

func (d *webGPUDriver) ensureMultisample() {
	if d.multisample.value.Truthy() && d.multisample.width == d.width && d.multisample.height == d.height {
		return
	}
	if d.multisample.value.Truthy() {
		d.multisample.value.Call("destroy")
	}
	value := d.device.Call("createTexture", webGPUObject("size", webGPUArray(d.width, d.height, 1), "format", d.format, "sampleCount", d.sampleCount, "usage", webGPUTextureRender))
	d.multisample = webGPUColorBuffer{value: value, view: value.Call("createView"), width: d.width, height: d.height}
}

func (d *webGPUDriver) upload2D[T any](projection [16]float32, instances []T) (int, int) {
	instanceBytes := sliceBytes(instances)
	uniform := [32]float32{}
	projection = d.transformProjection(projection)
	copy(uniform[:16], projection[:])
	return d.upload(uniform[:], instanceBytes)
}

// uploadSprite2D shares one uniform per four-bit coverage mask. All draws in
// a batch use the same projection; at most sixteen aligned slots are uploaded.
func (d *webGPUDriver) uploadSprite2D(projection [16]float32, instanceBytes []byte, draws []SpriteDraw) (int, []int) {
	frame := &d.frames[d.frameIndex]
	instanceOffset := align(frame.offset, 4)
	uniformOffset := align(instanceOffset+len(instanceBytes), 256)
	projection = d.transformProjection(projection)
	d.spriteUniformOffsets = growSlice((&d.spriteUniformOffsetsRetention).resetScratch(d.spriteUniformOffsets), len(draws))
	var offsets [16]int
	var seen uint16
	count := 0
	for index, draw := range draws {
		mask := uint8(0)
		for slot := range SpriteTextureSlots {
			if texture, ok := d.textures.Get(draw.TextureAt(slot)); ok && texture.format == TextureFormatR8 {
				mask |= 1 << slot
			}
		}
		if seen&(1<<mask) == 0 {
			seen |= 1 << mask
			offsets[mask] = uniformOffset + count*256
			uniform := d.spriteUniformData[count*64 : (count+1)*64]
			clear(uniform)
			copy(uniform[:16], projection[:])
			for slot := range SpriteTextureSlots {
				uniform[16+slot] = float32((mask >> slot) & 1)
			}
			count++
		}
		d.spriteUniformOffsets[index] = offsets[mask]
	}
	frame.offset = uniformOffset + count*256
	d.reserve(frame.offset)
	d.writeBuffer(instanceOffset, instanceBytes)
	d.writeBuffer(uniformOffset, sliceBytes(d.spriteUniformData[:count*64]))
	return instanceOffset, d.spriteUniformOffsets
}

func growSlice[T any](values []T, length int) []T {
	if cap(values) < length {
		values = make([]T, length)
	} else {
		values = values[:length]
	}
	return values
}

func (d *webGPUDriver) uploadMesh(batch MeshBatch3D, gpuDeformation bool) (int, int, int, int) {
	uniform := [meshLightUniformFloatCount]float32{}
	projection := d.transformProjection(batch.ViewProjection)
	copy(uniform[0:16], projection[:])
	writeMeshSceneUniforms(uniform[:], &batch)
	if gpuDeformation {
		// directionalLight.w is unused by lighting and carries the one-based
		// deformation header index for each deformed draw.
		uniform[31] = 0
	}
	vertexBytes, indexBytes := sliceBytes(batch.Vertices), sliceBytes(batch.Indices)
	frame := &d.frames[d.frameIndex]
	vertexOffset := align(frame.offset, 4)
	indexOffset := align(vertexOffset+len(vertexBytes), 4)
	instanceOffset := align(indexOffset+len(indexBytes), 4)
	storageOffset := align(instanceOffset+len(sliceBytes(batch.Instances)), 256)
	storageBytes := 0
	if gpuDeformation && len(batch.Deformations) != 0 {
		d.deformationUniforms = (&d.deformationUniformsRetention).resetScratch(d.deformationUniforms)
		storageBytes = len(sliceBytes(d.deformationWords))
	}
	d.deformationOffset = storageOffset
	d.deformationSize = max(storageBytes, 4)
	uniformOffset := align(storageOffset+storageBytes, 256)
	deformationUniformCount := 0
	if gpuDeformation {
		deformationUniformCount = len(batch.Deformations)
	}
	required := uniformOffset + meshLightUniformStride + deformationUniformCount*meshLightUniformStride
	d.reserve(required)
	frame.offset = required
	d.writeBuffer(vertexOffset, vertexBytes)
	d.writeBuffer(indexOffset, indexBytes)
	d.writeBuffer(instanceOffset, sliceBytes(batch.Instances))
	if storageBytes != 0 {
		d.writeBuffer(storageOffset, sliceBytes(d.deformationWords))
	}
	d.writeBuffer(uniformOffset, sliceBytes(uniform[:]))
	if deformationUniformCount != 0 {
		for index := range batch.Deformations {
			uniform[31] = float32(index + 1)
			offset := uniformOffset + (index+1)*meshLightUniformStride
			d.deformationUniforms = append(d.deformationUniforms, offset)
			d.writeBuffer(offset, sliceBytes(uniform[:]))
		}
	}
	return vertexOffset, indexOffset, instanceOffset, uniformOffset
}

func (d *webGPUDriver) upload(uniform []float32, instances []byte) (int, int) {
	frame := &d.frames[d.frameIndex]
	instanceOffset := align(frame.offset, 4)
	uniformOffset := align(instanceOffset+len(instances), 256)
	d.reserve(uniformOffset + 128)
	frame.offset = uniformOffset + 128
	d.writeBuffer(instanceOffset, instances)
	d.writeBuffer(uniformOffset, sliceBytes(uniform))
	return instanceOffset, uniformOffset
}

func (d *webGPUDriver) reserve(required int) {
	frame := &d.frames[d.frameIndex]
	if required <= frame.buffer.capacity {
		return
	}
	d.replaceFrameBuffer(frame, nextBufferCapacity(required))
}

func (d *webGPUDriver) replaceFrameBuffer(frame *webGPUFrame, capacity int) {
	frame.retired = append(frame.retired, frame.buffer.value)
	frame.buffer = d.createFrameBuffer(capacity)
	frame.generation++
}

func (d *webGPUDriver) writeBuffer(offset int, data []byte) {
	if len(data) == 0 {
		return
	}
	d.queue.Call("writeBuffer", d.frames[d.frameIndex].buffer.value, offset, copyBytesToJS(&d.uploadData, &d.uploadDataRetention, data), 0, len(data))
}

func (d *webGPUDriver) writeStaticBuffer(buffer js.Value, data []byte) {
	if len(data) > webGPUStaticUploadMaxBytes {
		// A one-off large mesh must not enlarge the persistent upload scratch.
		d.queue.Call("writeBuffer", buffer, 0, bytesToJS(data))
		return
	}
	// Share the frame upload array. The explicit size excludes unused capacity
	// when the next upload is smaller than the previous one.
	d.queue.Call("writeBuffer", buffer, 0, copyBytesToJS(&d.uploadData, &d.uploadDataRetention, data), 0, len(data))
}

func (d *webGPUDriver) textureBindGroup(texture *webGPUTexture) js.Value {
	frame := &d.frames[d.frameIndex]
	cached := &texture.bindGroups[d.frameIndex]
	if cached.generation == frame.generation && cached.value.Truthy() {
		return cached.value
	}
	cached.value = d.device.Call("createBindGroup", webGPUObject("layout", d.textureLayout, "entries", webGPUArray(
		webGPUObject("binding", 0, "resource", webGPUObject("buffer", frame.buffer.value, "offset", 0, "size", 128)),
		webGPUObject("binding", 1, "resource", texture.view), webGPUObject("binding", 2, "resource", texture.sampler),
		webGPUObject("binding", 3, "resource", texture.view), webGPUObject("binding", 4, "resource", texture.sampler),
		webGPUObject("binding", 5, "resource", webGPUObject("buffer", frame.buffer.value, "offset", 0, "size", 4)),
		webGPUObject("binding", 6, "resource", texture.view), webGPUObject("binding", 7, "resource", texture.sampler),
	)))
	cached.generation = frame.generation
	return cached.value
}

func (d *webGPUDriver) setMeshTextureBindGroup(texture, normal, metallicRoughness *webGPUTexture, extra [MeshExtraTextureSlots]*webGPUTexture, uniformOffset int) {
	frame := &d.frames[d.frameIndex]
	pair := webGPUTexturePair{base: texture, normal: normal, metallicRoughness: metallicRoughness, extra: extra}
	entry, created := d.meshBindGroups.getOrCreate(pair, d.cacheFrame)
	binding := &entry.value
	if created {
		for index, owner := range pair.textures() {
			link := &binding.links[index]
			*link = webGPUMaterialLink{binding: entry, next: owner.materials}
			if link.next != nil {
				link.next.previous = link
			}
			owner.materials = link
		}
	}
	cached := &binding.groups
	if cached[d.frameIndex].generation != frame.generation || !cached[d.frameIndex].value.Truthy() {
		cached[d.frameIndex].value = d.device.Call("createBindGroup", webGPUObject("layout", d.meshTextureLayout, "entries", webGPUArray(
			webGPUObject("binding", 0, "resource", webGPUObject("buffer", frame.buffer.value, "offset", 0, "size", meshLightUniformByteCount)),
			webGPUObject("binding", 1, "resource", texture.view), webGPUObject("binding", 2, "resource", texture.sampler),
			webGPUObject("binding", 3, "resource", normal.view), webGPUObject("binding", 4, "resource", normal.sampler),
			webGPUObject("binding", 6, "resource", metallicRoughness.view), webGPUObject("binding", 7, "resource", metallicRoughness.sampler),
			webGPUObject("binding", 8, "resource", extra[0].view), webGPUObject("binding", 9, "resource", extra[0].sampler),
			webGPUObject("binding", 10, "resource", extra[1].view), webGPUObject("binding", 11, "resource", extra[1].sampler),
			webGPUObject("binding", 12, "resource", extra[2].view), webGPUObject("binding", 13, "resource", extra[2].sampler),
			webGPUObject("binding", 14, "resource", extra[3].view), webGPUObject("binding", 15, "resource", extra[3].sampler),
			webGPUObject("binding", 16, "resource", extra[4].view), webGPUObject("binding", 17, "resource", extra[4].sampler),
			webGPUObject("binding", 18, "resource", extra[5].view), webGPUObject("binding", 19, "resource", extra[5].sampler),
			webGPUObject("binding", 20, "resource", extra[6].view), webGPUObject("binding", 21, "resource", extra[6].sampler),
			webGPUObject("binding", 22, "resource", extra[7].view), webGPUObject("binding", 23, "resource", extra[7].sampler),
			webGPUObject("binding", 24, "resource", extra[8].view), webGPUObject("binding", 25, "resource", extra[8].sampler),
			webGPUObject("binding", 26, "resource", extra[9].view), webGPUObject("binding", 27, "resource", extra[9].sampler),
			webGPUObject("binding", 28, "resource", extra[10].view), webGPUObject("binding", 29, "resource", extra[10].sampler),
			webGPUObject("binding", 30, "resource", extra[11].view), webGPUObject("binding", 31, "resource", extra[11].sampler),
			webGPUObject("binding", 32, "resource", extra[12].view), webGPUObject("binding", 33, "resource", extra[12].sampler),
		)))
		cached[d.frameIndex].generation = frame.generation
	}
	d.dynamicOffsets.SetIndex(0, uniformOffset)
	d.frame.Call("setBindGroup", 0, cached[d.frameIndex].value, d.dynamicOffsets)
}

func (d *webGPUDriver) setTextureBindGroup(texture *webGPUTexture, uniformOffset int) {
	d.dynamicOffsets.SetIndex(0, uniformOffset)
	d.frame.Call("setBindGroup", 0, d.textureBindGroup(texture), d.dynamicOffsets)
}

func (d *webGPUDriver) setShapeBindGroup(uniformOffset int) {
	frame := &d.frames[d.frameIndex]
	cached := &d.shapeBindGroups[d.frameIndex]
	if cached.generation != frame.generation || !cached.value.Truthy() {
		cached.value = d.device.Call("createBindGroup", webGPUObject("layout", d.shapeLayout, "entries", webGPUArray(webGPUObject("binding", 0, "resource", webGPUObject("buffer", frame.buffer.value, "offset", 0, "size", 128)))))
		cached.generation = frame.generation
	}
	d.dynamicOffsets.SetIndex(0, uniformOffset)
	d.frame.Call("setBindGroup", 0, cached.value, d.dynamicOffsets)
}

func (d *webGPUDriver) blendOrDefault() BlendState {
	if d.blendSet {
		return d.blend
	}
	return PremultipliedAlphaBlend
}

// Pipeline creation inside an active frame must not yield to the browser:
// canvas textures expire at the end of the current rendering task.
func (d *webGPUDriver) createPipeline(descriptor js.Value) (pipeline js.Value, err error) {
	if !d.frameActive {
		return awaitWebGPU(d.device.Call("createRenderPipelineAsync", descriptor))
	}
	defer func() {
		if failure := recover(); failure != nil {
			if failure, ok := failure.(js.Error); ok {
				pipeline, err = js.Null(), failure
			} else {
				panic(failure)
			}
		}
	}()
	return d.device.Call("createRenderPipeline", descriptor), nil
}

func (d *webGPUDriver) pipeline(shader *webGPUShader, blend BlendState) (js.Value, error) {
	// Blend constants are dynamic and must not create a pipeline per text color.
	blend.Constant = Color{}
	key := webGPUPipelineKey{blend: blend, depth: d.hasDepth(), coverage: d.r8Target()}
	if value, ok := shader.blendPipelines[key]; ok && value.Truthy() {
		return value, nil
	}
	descriptor := d.pipelineDescriptor(shader, false, blend, 0)
	pipeline, err := d.createPipeline(descriptor)
	if err != nil {
		return js.Null(), fmt.Errorf("render: WebGPU pipeline: %w", err)
	}
	shader.blendPipelines[key] = pipeline
	return pipeline, nil
}

func (d *webGPUDriver) meshPipeline(shader *webGPUShader, transparent int, mode MeshBlendMode) (js.Value, error) {
	index := 0
	if transparent != 0 {
		index = int(mode) + 1
	}
	depth := boolInt(d.hasDepth())
	if d.r8Target() {
		depth = 2
	}
	if pipeline := shader.meshPipelines[depth][index]; pipeline.Truthy() {
		return pipeline, nil
	}
	blend := meshBlend(mode)
	descriptor := d.pipelineDescriptor(shader, true, blend, transparent)
	pipeline, err := d.createPipeline(descriptor)
	if err != nil {
		return js.Null(), fmt.Errorf("render: WebGPU mesh pipeline: %w", err)
	}
	shader.meshPipelines[depth][index] = pipeline
	return pipeline, nil
}

func (d *webGPUDriver) createPickingPipeline() error {
	vertex := d.device.Call("createShaderModule", webGPUObject("code", webGPUPickingVertex))
	fragment := d.device.Call("createShaderModule", webGPUObject("code", webGPUPickingFragment))
	descriptor := webGPUObject(
		"layout", d.shapePipelineLayout,
		"vertex", webGPUObject("module", vertex, "entryPoint", "mesh_vertex", "buffers", d.meshVertexLayouts()),
		"fragment", webGPUObject("module", fragment, "entryPoint", "mesh_fragment", "targets", webGPUArray(webGPUObject("format", "r32uint", "writeMask", 1))),
		"primitive", webGPUObject("topology", "triangle-list", "cullMode", "none"),
		"multisample", webGPUObject("count", 1),
		"depthStencil", webGPUObject("format", "depth24plus", "depthWriteEnabled", true, "depthCompare", "greater"),
	)
	pipeline, err := d.createPipeline(descriptor)
	if err != nil {
		return fmt.Errorf("render: WebGPU picking pipeline: %w", err)
	}
	d.pickingPipeline = pipeline
	return nil
}

func (d *webGPUDriver) pipelineDescriptor(shader *webGPUShader, mesh bool, blend BlendState, transparent int) js.Value {
	layout := d.shapePipelineLayout
	if mesh || shader.pipelineKind == PipelineSprite {
		layout = d.texturePipelineLayout
	}
	if shader == d.programs[PipelineSprite] {
		layout = d.spritePipelineLayout
	}
	vertexBuffers := d.spriteVertexLayouts(shader != d.programs[PipelineSprite])
	if shader.pipelineKind == PipelineShape {
		vertexBuffers = d.shapeVertexLayouts()
	}
	if mesh {
		layout = d.meshPipelineLayout
		vertexBuffers = d.meshVertexLayouts()
	}
	vertex := webGPUObject("module", shader.vertex, "entryPoint", webGPUEntryPoint(shader.pipelineKind, true), "buffers", vertexBuffers)
	if mesh && transparent == 0 {
		blend = BlendState{}
	}
	fragment := webGPUObject("module", shader.fragment, "entryPoint", webGPUEntryPoint(shader.pipelineKind, false), "targets", webGPUArray(d.colorTarget(blend, transparent)))
	descriptor := webGPUObject("layout", layout, "vertex", vertex, "fragment", fragment, "primitive", webGPUObject("topology", "triangle-list", "cullMode", "none"))
	sampleCount := d.sampleCount
	if d.r8Target() {
		sampleCount = 1
	}
	descriptor.Set("multisample", webGPUObject("count", sampleCount))
	if d.hasDepth() && mesh {
		descriptor.Set("depthStencil", webGPUObject("format", "depth24plus", "depthWriteEnabled", transparent == 0, "depthCompare", "greater"))
	} else if d.hasDepth() {
		descriptor.Set("depthStencil", webGPUObject("format", "depth24plus", "depthWriteEnabled", false, "depthCompare", "always"))
	}
	return descriptor
}

func webGPUEntryPoint(kind Pipeline, vertex bool) string {
	switch kind {
	case PipelineShape:
		if vertex {
			return "shape_vertex"
		}
		return "shape_fragment"
	case PipelineMesh:
		if vertex {
			return "mesh_vertex"
		}
		return "mesh_fragment"
	default:
		if vertex {
			return "sprite_vertex"
		}
		return "sprite_fragment"
	}
}

func (d *webGPUDriver) r8Target() bool {
	target, ok := d.targets.Get(d.activeTarget)
	return ok && target != nil && target.descriptor.ColorFormat == RenderTargetR8
}

func (d *webGPUDriver) colorTarget(blend BlendState, transparent int) js.Value {
	format, mask := d.format, 15
	if d.r8Target() {
		format, mask = "r8unorm", 1
	}
	if transparent != 0 || blend != (BlendState{}) {
		return webGPUObject("format", format, "blend", webGPUBlend(blend), "writeMask", mask)
	}
	return webGPUObject("format", format, "writeMask", mask)
}

func webGPUBlend(blend BlendState) js.Value {
	return webGPUObject("color", webGPUObject("srcFactor", webGPUBlendFactor(blend.RGB.Src), "dstFactor", webGPUBlendFactor(blend.RGB.Dst), "operation", "add"), "alpha", webGPUObject("srcFactor", webGPUBlendFactor(blend.Alpha.Src), "dstFactor", webGPUBlendFactor(blend.Alpha.Dst), "operation", "add"))
}

func meshBlend(mode MeshBlendMode) BlendState {
	source, destination := BlendOne, BlendOneMinusSrcAlpha
	switch mode {
	case MeshBlendAdditive:
		destination = BlendOne
	case MeshBlendMultiply:
		source = BlendDstColor
	}
	return BlendState{RGB: Blend{Src: source, Dst: destination}, Alpha: Blend{Src: source, Dst: destination}}
}

func blendModeOrNormal(mode MeshBlendMode) MeshBlendMode {
	if mode > MeshBlendScreen {
		return MeshBlendNormal
	}
	return mode
}

func webGPUBlendSupported(blend BlendState) bool {
	_, left := webGPUBlendFactorOK(blend.RGB.Src)
	_, right := webGPUBlendFactorOK(blend.RGB.Dst)
	_, alphaLeft := webGPUBlendFactorOK(blend.Alpha.Src)
	_, alphaRight := webGPUBlendFactorOK(blend.Alpha.Dst)
	return left && right && alphaLeft && alphaRight
}

func webGPUBlendFactorOK(factor BlendFactor) (string, bool) {
	value := webGPUBlendFactor(factor)
	return value, value != ""
}

func webGPUBlendFactor(factor BlendFactor) string {
	switch factor {
	case BlendZero:
		return "zero"
	case BlendOne:
		return "one"
	case BlendSrcColor:
		return "src"
	case BlendOneMinusSrcColor:
		return "one-minus-src"
	case BlendSrcAlpha:
		return "src-alpha"
	case BlendOneMinusSrcAlpha:
		return "one-minus-src-alpha"
	case BlendDstAlpha:
		return "dst-alpha"
	case BlendOneMinusDstAlpha:
		return "one-minus-dst-alpha"
	case BlendDstColor:
		return "dst"
	case BlendOneMinusDstColor:
		return "one-minus-dst"
	case BlendSrcAlphaSaturate:
		return "src-alpha-saturated"
	case BlendConstantColor, BlendConstantAlpha:
		return "constant"
	case BlendOneMinusConstantColor, BlendOneMinusConstantAlpha:
		return "one-minus-constant"
	default:
		return ""
	}
}

func (d *webGPUDriver) createBindGroupLayouts() (js.Value, js.Value) {
	texture := d.device.Call("createBindGroupLayout", webGPUObject("entries", webGPUArray(
		webGPUObject("binding", 0, "visibility", webGPUShaderVertex|webGPUShaderFragment, "buffer", webGPUObject("type", "uniform", "hasDynamicOffset", true, "minBindingSize", 0)),
		webGPUObject("binding", 1, "visibility", webGPUShaderVertex|webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 2, "visibility", webGPUShaderVertex|webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
		webGPUObject("binding", 3, "visibility", webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 4, "visibility", webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
		webGPUObject("binding", 5, "visibility", webGPUShaderVertex, "buffer", webGPUObject("type", "read-only-storage", "minBindingSize", 4)),
		webGPUObject("binding", 6, "visibility", webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 7, "visibility", webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
	)))
	d.meshTextureLayout = d.device.Call("createBindGroupLayout", webGPUObject("entries", webGPUArray(
		webGPUObject("binding", 0, "visibility", webGPUShaderVertex|webGPUShaderFragment, "buffer", webGPUObject("type", "uniform", "hasDynamicOffset", true, "minBindingSize", 0)),
		webGPUObject("binding", 1, "visibility", webGPUShaderVertex|webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 2, "visibility", webGPUShaderVertex|webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
		webGPUObject("binding", 3, "visibility", webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 4, "visibility", webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
		webGPUObject("binding", 6, "visibility", webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 7, "visibility", webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
		webGPUObject("binding", 8, "visibility", webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 9, "visibility", webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
		webGPUObject("binding", 10, "visibility", webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 11, "visibility", webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
		webGPUObject("binding", 12, "visibility", webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 13, "visibility", webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
		webGPUObject("binding", 14, "visibility", webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 15, "visibility", webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
		webGPUObject("binding", 16, "visibility", webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 17, "visibility", webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
		webGPUObject("binding", 18, "visibility", webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 19, "visibility", webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
		webGPUObject("binding", 20, "visibility", webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 21, "visibility", webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
		webGPUObject("binding", 22, "visibility", webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 23, "visibility", webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
		webGPUObject("binding", 24, "visibility", webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 25, "visibility", webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
		webGPUObject("binding", 26, "visibility", webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 27, "visibility", webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
		webGPUObject("binding", 28, "visibility", webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 29, "visibility", webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
		webGPUObject("binding", 30, "visibility", webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 31, "visibility", webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
		webGPUObject("binding", 32, "visibility", webGPUShaderFragment, "texture", webGPUObject("sampleType", "float")),
		webGPUObject("binding", 33, "visibility", webGPUShaderFragment, "sampler", webGPUObject("type", "filtering")),
	)))
	d.meshDataLayout = d.device.Call("createBindGroupLayout", webGPUObject("entries", webGPUArray(
		webGPUObject("binding", 0, "visibility", webGPUShaderVertex, "buffer", webGPUObject("type", "read-only-storage", "hasDynamicOffset", true, "minBindingSize", 4)),
		webGPUObject("binding", 1, "visibility", webGPUShaderVertex, "buffer", webGPUObject("type", "read-only-storage", "minBindingSize", 4)),
	)))
	shape := d.device.Call("createBindGroupLayout", webGPUObject("entries", webGPUArray(webGPUObject("binding", 0, "visibility", webGPUShaderVertex|webGPUShaderFragment, "buffer", webGPUObject("type", "uniform", "hasDynamicOffset", true, "minBindingSize", 128)))))
	return texture, shape
}

func (d *webGPUDriver) createPipelineLayout(groups ...any) js.Value {
	return d.device.Call("createPipelineLayout", webGPUObject("bindGroupLayouts", webGPUArray(groups...)))
}

func (d *webGPUDriver) spriteVertexLayouts(custom bool) js.Value {
	return webGPUArray(webGPUVertexBuffer(20, "vertex", webGPUAttributes(
		webGPUAttribute(0, 0, "float32x3"), webGPUAttribute(1, 12, "float32x2"))), webGPUVertexBuffer(spriteInstanceStride(custom), "instance", webGPUAttributes(
		webGPUAttribute(2, 0, "float32x3"), webGPUAttribute(3, 12, "float32x3"), webGPUAttribute(4, 24, "float32x3"), webGPUAttribute(5, 36, "float32x3"), webGPUAttribute(6, 48, "float32x4"), webGPUAttribute(7, 64, "float32x4"), webGPUAttribute(8, 80, "float32x4"), webGPUAttribute(9, spriteAttributeOffset(9, custom), "float32x4"), webGPUAttribute(10, spriteAttributeOffset(10, custom), "float32x4"), webGPUAttribute(11, spriteAttributeOffset(11, custom), "float32"))))
}

func (d *webGPUDriver) shapeVertexLayouts() js.Value {
	return webGPUArray(webGPUVertexBuffer(20, "vertex", webGPUAttributes(webGPUAttribute(0, 0, "float32x3"), webGPUAttribute(1, 12, "float32x2"))), webGPUVertexBuffer(100, "instance", webGPUAttributes(webGPUAttribute(2, 0, "float32x3"), webGPUAttribute(3, 12, "float32x3"), webGPUAttribute(4, 24, "float32x3"), webGPUAttribute(5, 36, "float32x3"), webGPUAttribute(6, 48, "float32x4"), webGPUAttribute(7, 64, "float32x4"), webGPUAttribute(8, 80, "float32x4"), webGPUAttribute(9, 96, "uint32"))))
}

func (d *webGPUDriver) meshVertexLayouts() js.Value {
	return webGPUArray(webGPUVertexBuffer(int(unsafe.Sizeof(Vertex3D{})), "vertex", webGPUAttributes(webGPUAttribute(0, 0, "float32x3"), webGPUAttribute(1, 12, "float32x3"), webGPUAttribute(2, 24, "float32x2"), webGPUAttribute(15, 32, "float32x4"))), webGPUVertexBuffer(192, "instance", webGPUAttributes(
		webGPUAttribute(3, 0, "float32x4"), webGPUAttribute(4, 16, "float32x4"), webGPUAttribute(5, 32, "float32x4"), webGPUAttribute(6, 48, "float32x4"), webGPUAttribute(7, 64, "float32x4"), webGPUAttribute(8, 80, "float32x4"), webGPUAttribute(9, 96, "float32x4"), webGPUAttribute(10, 112, "float32x4"), webGPUAttribute(11, 128, "float32x4"), webGPUAttribute(12, 144, "float32x4"), webGPUAttribute(13, 160, "float32x4"), webGPUAttribute(14, 176, "float32x4"))))
}

func webGPUVertexBuffer(stride int, step string, attributes js.Value) js.Value {
	return webGPUObject("arrayStride", stride, "stepMode", step, "attributes", attributes)
}

func webGPUAttribute(location, offset int, format string) js.Value {
	return webGPUObject("shaderLocation", location, "offset", offset, "format", format)
}

func webGPUAttributes(values ...js.Value) js.Value {
	array := js.Global().Get("Array").New(len(values))
	for index, value := range values {
		array.SetIndex(index, value)
	}
	return array
}

func align(value, alignment int) int { return (value + alignment - 1) / alignment * alignment }
func roundup4(value int) int         { return align(value, 4) }
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (d *webGPUDriver) textureForDraw(handle TextureHandle) *webGPUTexture {
	value, _ := d.textures.Get(handle)
	return value
}

func bytesToJS(values []byte) js.Value {
	array := js.Global().Get("Uint8Array").New(len(values))
	if len(values) != 0 {
		js.CopyBytesToJS(array, values)
	}
	return array
}

const webGPUSpriteVertex = `struct Uniforms {
  projection: mat4x4<f32>,
  padding: array<vec4<f32>, 4>,
};
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
struct Input {
  @location(0) position: vec3<f32>, @location(1) textureCoord: vec2<f32>,
  @location(2) model0: vec3<f32>, @location(3) model1: vec3<f32>,
  @location(4) texture0: vec3<f32>, @location(5) texture1: vec3<f32>,
  @location(6) color: vec4<f32>, @location(7) custom0: vec4<f32>, @location(8) custom1: vec4<f32>,
  @location(9) custom2: vec4<f32>, @location(10) custom3: vec4<f32>,
};
struct Output {
  @builtin(position) position: vec4<f32>, @location(0) textureCoord: vec2<f32>, @location(1) color: vec4<f32>,
  @location(2) @interpolate(flat) custom0: vec4<f32>, @location(3) @interpolate(flat) custom1: vec4<f32>,
  @location(4) @interpolate(flat) custom2: vec4<f32>, @location(5) @interpolate(flat) custom3: vec4<f32>,
};
@vertex fn sprite_vertex(input: Input) -> Output {
  var out: Output;
  let p = vec3<f32>(input.position.xy, 1.0);
  out.position = uniforms.projection * vec4<f32>(dot(input.model0, p), dot(input.model1, p), 0.0, 1.0);
  let uv = vec3<f32>(input.textureCoord, 1.0);
  out.textureCoord = vec2<f32>(dot(input.texture0, uv), dot(input.texture1, uv));
  out.color = input.color; out.custom0 = input.custom0; out.custom1 = input.custom1; out.custom2 = input.custom2; out.custom3 = input.custom3;
  return out;
}`

const webGPUSpriteFragment = `struct Uniforms {
  projection: mat4x4<f32>,
  padding: array<vec4<f32>, 4>,
};
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@group(0) @binding(1) var tex: texture_2d<f32>;
@group(0) @binding(2) var texSampler: sampler;
struct Input { @location(0) textureCoord: vec2<f32>, @location(1) color: vec4<f32>, @location(2) @interpolate(flat) custom0: vec4<f32>, @location(3) @interpolate(flat) custom1: vec4<f32>, @location(4) @interpolate(flat) custom2: vec4<f32>, @location(5) @interpolate(flat) custom3: vec4<f32> };
@fragment fn sprite_fragment(input: Input) -> @location(0) vec4<f32> {
  let sampled = textureSample(tex, texSampler, input.textureCoord);
  let texel = select(sampled, vec4<f32>(1.0, 1.0, 1.0, sampled.r), uniforms.padding[0].x > 0.5);
  let color = texel * input.color;
  return vec4<f32>(color.rgb * color.a, color.a);
}`

const webGPUShapeVertex = `struct Uniforms { projection: mat4x4<f32>, padding: array<vec4<f32>, 4> };
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
struct Input { @location(0) position: vec3<f32>, @location(1) localCoord: vec2<f32>, @location(2) model0: vec3<f32>, @location(3) model1: vec3<f32>, @location(4) local0: vec3<f32>, @location(5) local1: vec3<f32>, @location(6) fill: vec4<f32>, @location(7) border: vec4<f32>, @location(8) params: vec4<f32>, @location(9) kind: u32 };
struct Output { @builtin(position) position: vec4<f32>, @location(0) local: vec2<f32>, @location(1) @interpolate(flat) fill: vec4<f32>, @location(2) @interpolate(flat) border: vec4<f32>, @location(3) @interpolate(flat) params: vec4<f32>, @location(4) @interpolate(flat) kind: u32 };
@vertex fn shape_vertex(input: Input) -> Output { var out: Output; let p = vec3<f32>(input.position.xy, 1.0); let uv = vec3<f32>(input.localCoord, 1.0); out.position = uniforms.projection * vec4<f32>(dot(input.model0, p), dot(input.model1, p), 0.0, 1.0); out.local = vec2<f32>(dot(input.local0, uv), dot(input.local1, uv)) * input.params.xy; out.fill = input.fill; out.border = input.border; out.params = input.params; out.kind = input.kind; return out; }`

const webGPUShapeFragment = `struct Input { @location(0) local: vec2<f32>, @location(1) @interpolate(flat) fill: vec4<f32>, @location(2) @interpolate(flat) border: vec4<f32>, @location(3) @interpolate(flat) params: vec4<f32>, @location(4) @interpolate(flat) kind: u32 };
fn roundedBox(point: vec2<f32>, size: vec2<f32>, radius: f32) -> f32 { let r = clamp(radius, 0.0, min(size.x, size.y) * 0.5); let q = abs(point) - size * 0.5 + r; return length(max(q, vec2<f32>(0.0))) + min(max(q.x, q.y), 0.0) - r; }
fn segment(point: vec2<f32>, a: vec2<f32>, b: vec2<f32>) -> f32 { let pa = point - a; let ba = b - a; let h = clamp(dot(pa, ba) / dot(ba, ba), 0.0, 1.0); return length(pa - ba * h); }
fn coverage(distance: f32, derivative: f32) -> f32 { return clamp(0.5 - distance / max(derivative, 0.0001), 0.0, 1.0); }
fn premultiply(value: vec4<f32>) -> vec4<f32> { return vec4<f32>(value.rgb * value.a, value.a); }
@fragment fn shape_fragment(input: Input) -> @location(0) vec4<f32> {
  let size = input.params.xy;
  let point = input.local - size * 0.5;
  let direction = input.params.w;
  var triangleA = vec2<f32>(size.x * 0.5, size.y * 0.40);
  var triangleB = vec2<f32>(size.x * 0.20, size.y * 0.73);
  var triangleC = vec2<f32>(size.x * 0.80, size.y * 0.73);
  if (direction >= 0.5 && direction < 1.5) { triangleA = vec2<f32>(size.x * 0.20, size.y * 0.20); triangleB = vec2<f32>(size.x * 0.80, size.y * 0.20); triangleC = vec2<f32>(size.x * 0.5, size.y * 0.50); }
  if (direction >= 1.5 && direction < 2.5) { triangleA = vec2<f32>(size.x * 0.73, size.y * 0.20); triangleB = vec2<f32>(size.x * 0.73, size.y * 0.80); triangleC = vec2<f32>(size.x * 0.40, size.y * 0.5); }
  if (direction >= 2.5) { triangleA = vec2<f32>(size.x * 0.27, size.y * 0.20); triangleB = vec2<f32>(size.x * 0.27, size.y * 0.80); triangleC = vec2<f32>(size.x * 0.60, size.y * 0.5); }
  let edge0 = (triangleB.x - triangleA.x) * (input.local.y - triangleA.y) - (triangleB.y - triangleA.y) * (input.local.x - triangleA.x);
  let edge1 = (triangleC.x - triangleB.x) * (input.local.y - triangleB.y) - (triangleC.y - triangleB.y) * (input.local.x - triangleB.x);
  let edge2 = (triangleA.x - triangleC.x) * (input.local.y - triangleC.y) - (triangleA.y - triangleC.y) * (input.local.x - triangleC.x);
  let orientation = sign((triangleB.x - triangleA.x) * (triangleC.y - triangleA.y) - (triangleB.y - triangleA.y) * (triangleC.x - triangleA.x));
  let triangleDistance = select(max(max(edge0, edge1), edge2), -min(min(edge0, edge1), edge2), orientation > 0.0) / max(length(triangleB - triangleA), 1.0);
  let triangle = select(0.0, 1.0, triangleDistance <= 0.0);
  let checkA = size * vec2<f32>(0.18, 0.56);
  let checkB = size * vec2<f32>(0.42, 0.78);
  let checkC = size * vec2<f32>(0.84, 0.22);
  let thickness = max(min(size.x, size.y) * 0.14, 1.4);
  let checkDistance = min(segment(input.local, checkA, checkB), segment(input.local, checkB, checkC)) - thickness * 0.5;
  let outerDistance = roundedBox(point, size, input.params.z);
  let borderWidth = min(max(input.params.w, 0.0), min(size.x, size.y) * 0.5);
  let innerSize = max(size - borderWidth * 2.0, vec2<f32>(0.0));
  let innerDistance = roundedBox(point, innerSize, max(input.params.z - borderWidth, 0.0));
  let check = coverage(checkDistance, fwidth(checkDistance));
  let outer = coverage(outerDistance, fwidth(outerDistance));
  let inner = coverage(innerDistance, fwidth(innerDistance));
  if (input.kind == 2u) { return premultiply(input.fill) * check; }
  if (input.kind == 4u) { return premultiply(input.fill) * triangle; }
  if (borderWidth <= 0.0) { return premultiply(input.fill) * outer; }
  let hasInner = min(innerSize.x, innerSize.y) > 0.0;
  return mix(premultiply(input.border), premultiply(input.fill), select(0.0, inner, hasInner)) * outer;
}`

const webGPUMeshVertex = `struct Uniforms { projection: mat4x4<f32>, cameraPosition: vec4<f32>, ambientLight: vec4<f32>, lightDirection: vec4<f32>, directionalLight: vec4<f32>, lightPosition: array<vec4<f32>, 16>, lightDirections: array<vec4<f32>, 16>, lightColors: array<vec4<f32>, 16>, lightParams: array<vec4<f32>, 16>, lightMeta: vec4<f32> };
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@group(1) @binding(0) var<storage, read> deformation: array<u32>;
@group(1) @binding(1) var<storage, read> staticDeformation: array<u32>;
fn deformationFloat(value: u32) -> f32 { return bitcast<f32>(value); }
fn deformationMatrix(offset: u32) -> mat4x4<f32> {
  return mat4x4<f32>(vec4<f32>(deformationFloat(deformation[offset]), deformationFloat(deformation[offset + 1u]), deformationFloat(deformation[offset + 2u]), deformationFloat(deformation[offset + 3u])),
    vec4<f32>(deformationFloat(deformation[offset + 4u]), deformationFloat(deformation[offset + 5u]), deformationFloat(deformation[offset + 6u]), deformationFloat(deformation[offset + 7u])),
    vec4<f32>(deformationFloat(deformation[offset + 8u]), deformationFloat(deformation[offset + 9u]), deformationFloat(deformation[offset + 10u]), deformationFloat(deformation[offset + 11u])),
    vec4<f32>(deformationFloat(deformation[offset + 12u]), deformationFloat(deformation[offset + 13u]), deformationFloat(deformation[offset + 14u]), deformationFloat(deformation[offset + 15u])));
}
struct Input { @location(0) position: vec3<f32>, @location(1) normal: vec3<f32>, @location(2) textureCoord: vec2<f32>, @location(15) tangent: vec4<f32>, @location(3) model0: vec4<f32>, @location(4) model1: vec4<f32>, @location(5) model2: vec4<f32>, @location(6) model3: vec4<f32>, @location(7) baseColor: vec4<f32>, @location(8) emissiveMetallic: vec4<f32>, @location(9) material: vec4<f32>, @location(10) uvTransform: vec4<f32>, @location(11) custom0: vec4<f32>, @location(12) custom1: vec4<f32>, @location(13) custom2: vec4<f32>, @location(14) custom3: vec4<f32>, @builtin(vertex_index) vertexIndex: u32 };
struct Output { @builtin(position) position: vec4<f32>, @location(0) worldPosition: vec3<f32>, @location(1) normal: vec3<f32>, @location(2) textureCoord: vec2<f32>, @location(3) baseColor: vec4<f32>, @location(4) emissiveMetallic: vec4<f32>, @location(5) @interpolate(flat) material: vec4<f32>, @location(6) @interpolate(flat) custom0: vec4<f32>, @location(7) @interpolate(flat) custom1: vec4<f32>, @location(8) @interpolate(flat) custom2: vec4<f32>, @location(9) @interpolate(flat) custom3: vec4<f32>, @location(10) tangent: vec4<f32> };
@vertex fn mesh_vertex(input: Input) -> Output {
  var out: Output;
  let spine = input.material.x < -1.5;
  var base = input.baseColor;
  var custom = input.custom0;
  var position = input.position;
  var normal = input.normal;
  var tangent = input.tangent.xyz;
  var tangentSign = input.tangent.w;
  let deformationIndex = u32(uniforms.directionalLight.w + 0.5);
  if (!spine && deformationIndex != 0u) {
    let header = (deformationIndex - 1u) * 8u;
    let skinOffset = deformation[header];
    let morphOffset = deformation[header + 1u];
    let jointOffset = deformation[header + 2u];
    let weightOffset = deformation[header + 3u];
    let targetCount = deformation[header + 5u];
    let jointCount = deformation[header + 6u];
    if (morphOffset != 0u) {
      for (var morphTarget = 0u; morphTarget < targetCount; morphTarget++) {
        let delta = morphOffset + (morphTarget * deformation[header + 4u] + input.vertexIndex) * 9u;
        let weight = deformationFloat(deformation[weightOffset + morphTarget]);
        if (weight == 0.0) { continue; }
        position += vec3<f32>(deformationFloat(staticDeformation[delta]), deformationFloat(staticDeformation[delta + 1u]), deformationFloat(staticDeformation[delta + 2u])) * weight;
        normal += vec3<f32>(deformationFloat(staticDeformation[delta + 3u]), deformationFloat(staticDeformation[delta + 4u]), deformationFloat(staticDeformation[delta + 5u])) * weight;
        tangent += vec3<f32>(deformationFloat(staticDeformation[delta+6u]),deformationFloat(staticDeformation[delta+7u]),deformationFloat(staticDeformation[delta+8u]))*weight;
      }
    }
    if (skinOffset != 0u && jointCount != 0u) {
      let sourcePosition = vec4<f32>(position, 1.0);
      let sourceNormal = vec4<f32>(normal, 0.0);
      let sourceTangent = vec4<f32>(tangent,0.0);
      var skinMatrix = mat4x4<f32>(vec4<f32>(0.0),vec4<f32>(0.0),vec4<f32>(0.0),vec4<f32>(0.0));
      var skinWeight = 0.0;
      for (var influence = 0u; influence < 4u; influence++) {
        let weight = deformationFloat(staticDeformation[skinOffset + input.vertexIndex * 8u + 4u + influence]);
        if (weight > 0.0) {
          let joint = staticDeformation[skinOffset + input.vertexIndex * 8u + influence];
          if (joint < jointCount) {
            let matrix = deformationMatrix(jointOffset + joint * 16u);
            skinMatrix += matrix*weight;
            skinWeight += weight;
          }
        }
      }
      if (skinWeight > 0.0) {
        position = (skinMatrix*sourcePosition).xyz;
        let cofactor=mat3x3<f32>(cross(skinMatrix[1].xyz,skinMatrix[2].xyz),cross(skinMatrix[2].xyz,skinMatrix[0].xyz),cross(skinMatrix[0].xyz,skinMatrix[1].xyz));
        let flip=select(-1.0,1.0,dot(skinMatrix[0].xyz,cofactor[0])>=0.0);
        normal=normalize(cofactor*sourceNormal.xyz)*flip;
        tangent=(skinMatrix*sourceTangent).xyz;
        tangentSign*=flip;
      }
    }
  }
  if (spine) { let packedA = u32(input.normal.x); let packedB = u32(input.normal.z); base = vec4<f32>(vec3<f32>(f32(packedA & 255u), f32((packedA >> 8u) & 255u), f32((packedA >> 16u) & 255u)) / 255.0, input.normal.y); custom = vec4<f32>(vec3<f32>(f32(packedB & 255u), f32((packedB >> 8u) & 255u), f32((packedB >> 16u) & 255u)) / 255.0, input.custom0.a); }
  let model = mat4x4<f32>(input.model0, input.model1, input.model2, input.model3);
  let world = model * vec4<f32>(position, 1.0);
  out.position = uniforms.projection * world;
  if ((u32(input.material.w+0.5)&2048u)!=0u) { out.position.z=0.000001*out.position.w; }
  out.worldPosition = world.xyz;
  let cofactor = mat3x3<f32>(cross(input.model1.xyz, input.model2.xyz), cross(input.model2.xyz, input.model0.xyz), cross(input.model0.xyz, input.model1.xyz));
  let determinant = dot(input.model0.xyz, cofactor[0]);
  out.normal = select(normalize(cofactor * normal) * select(-1.0, 1.0, determinant >= 0.0), vec3<f32>(0.0, 0.0, 1.0), spine);
  out.tangent = vec4<f32>((model*vec4<f32>(tangent,0.0)).xyz,tangentSign*select(-1.0,1.0,determinant>=0.0));
  out.textureCoord = input.textureCoord * input.uvTransform.xy + input.uvTransform.zw;
  out.baseColor = base; out.emissiveMetallic = input.emissiveMetallic; out.material = input.material; out.custom0 = custom; out.custom1 = input.custom1; out.custom2 = input.custom2; out.custom3 = input.custom3; return out;
}`

const webGPUPickingVertex = `struct Uniforms { projection: mat4x4<f32> };
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
struct Input { @location(0) position: vec3<f32>, @location(3) model0: vec4<f32>, @location(4) model1: vec4<f32>, @location(5) model2: vec4<f32>, @location(6) model3: vec4<f32>, @location(14) custom3: vec4<f32> };
struct Output { @builtin(position) position: vec4<f32>, @location(0) @interpolate(flat) pickingID: u32 };
@vertex fn mesh_vertex(input: Input) -> Output {
	var out: Output;
	out.position = uniforms.projection * mat4x4<f32>(input.model0, input.model1, input.model2, input.model3) * vec4<f32>(input.position, 1.0);
	out.pickingID = bitcast<u32>(input.custom3.w);
	return out;
}`

const webGPUPickingFragment = `struct Input { @location(0) @interpolate(flat) pickingID: u32 };
@fragment fn mesh_fragment(input: Input) -> @location(0) u32 { return input.pickingID; }`

//go:embed shaders/mesh.frag.wgsl
var webGPUMeshFragment string

//go:embed shaders/mesh_unlit.frag.wgsl
var webGPUMeshUnlitFragment string

//go:embed shaders/mesh_depth.frag.wgsl
var webGPUMeshDepthFragment string

//go:embed shaders/mesh_basic.frag.wgsl
var webGPUMeshBasicFragment string

func (d *webGPUDriver) deformationLayout(handle MeshHandle) (meshDeformationLayout, bool) {
	mesh, ok := d.meshes.Get(handle)
	if !ok {
		return meshDeformationLayout{}, false
	}
	return mesh.deformation, mesh.deformation.vertexCount != 0
}

func (d *webGPUDriver) SetMeshDeformation(handle MeshHandle, descriptor MeshDeformationDescriptor) error {
	mesh, ok := d.meshes.Get(handle)
	if !ok || mesh.deformationBuffer.Truthy() || uint64(mesh.vertexCount) != uint64(descriptor.VertexCount) {
		return fmt.Errorf("render: invalid static mesh deformation %s", handle)
	}
	scratch := meshDeformationBuffers.get()
	defer meshDeformationBuffers.put(scratch)
	words, layout, err := packMeshDeformationStatic(scratch.data, descriptor)
	scratch.data = words
	if err != nil {
		return err
	}
	mesh.deformationBuffer = d.device.Call("createBuffer", webGPUObject("size", len(words)*4, "usage", webGPUBufferStorage|webGPUBufferCopyDst))
	d.writeStaticBuffer(mesh.deformationBuffer, sliceBytes(words))
	mesh.deformation = layout
	return nil
}

func (d *webGPUDriver) setMeshDataBindGroup(mesh *webGPUMesh) {
	frame := &d.frames[d.frameIndex]
	entry, created := d.meshDataGroups.getOrCreate(webGPUDataKey{mesh: mesh, size: d.deformationSize}, d.cacheFrame)
	if created && mesh != nil {
		entry.value.next = mesh.bindGroups
		if mesh.bindGroups != nil {
			mesh.bindGroups.value.previous = entry
		}
		mesh.bindGroups = entry
	}
	cached := &entry.value.groups
	if cached[d.frameIndex].generation != frame.generation || !cached[d.frameIndex].value.Truthy() {
		staticBuffer := d.emptyDeformation
		if mesh != nil {
			staticBuffer = mesh.deformationBuffer
		}
		cached[d.frameIndex].value = d.device.Call("createBindGroup", webGPUObject("layout", d.meshDataLayout, "entries", webGPUArray(
			webGPUObject("binding", 0, "resource", webGPUObject("buffer", frame.buffer.value, "offset", 0, "size", d.deformationSize)),
			webGPUObject("binding", 1, "resource", webGPUObject("buffer", staticBuffer)),
		)))
		cached[d.frameIndex].generation = frame.generation
	}
	d.dynamicOffsets.SetIndex(0, d.deformationOffset)
	d.frame.Call("setBindGroup", 1, cached[d.frameIndex].value, d.dynamicOffsets)
}
