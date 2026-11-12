package graphics

import (
	"errors"
	"image"
	"sync/atomic"

	"github.com/onix-engine/onix/render"
)

var (
	errTextureDevice    = errors.New("graphics: texture has no CPU payload for this device")
	textureBatchCounter atomic.Uint64
)

type Texture struct {
	err                error
	externalDevice     *render.Device
	renderTarget       *render.RenderTarget
	source             *Texture
	colorOwner         *colorTextureEntry
	device             *render.Device
	residencies        render.DeviceValues[render.TextureHandle]
	updates            []render.TextureUpdate
	pix                []byte
	Width              int
	contentRevision    uint64
	OffsetX            int
	SourceHeight       int
	geometryRevision   uint64
	SourceWidth        int
	Height             int
	batchKey           uint64
	OffsetY            int
	mipLevels          int
	handle             render.TextureHandle
	MaxV               float32
	rasterScaleY       float32
	U, V               float32
	Angle              float32
	MaxU               float32
	rasterScaleX       float32
	mipmap             bool
	external           bool
	usage              ResourceUsage
	format             render.TextureFormat
	wrapU              render.TextureAddressMode
	premultipliedAlpha bool
	zeroInitialized    bool
	wrapV              render.TextureAddressMode
}

var textureRevision atomic.Uint64

// TextureRevision signals that per-texture dependencies may have changed.
// Content updates do not necessarily invalidate retained geometry.
func TextureRevision() uint64 { return textureRevision.Load() }

// ContentRevision is shared by atlas views of the same backing texture.
func (t *Texture) ContentRevision() uint64 {
	if base := t.baseTexture(); base != nil {
		return max(t.contentRevision, base.contentRevision)
	}
	return 0
}

// GeometryRevision changes when logical bounds or resource lifetime change.
// Pixel updates preserve this revision, including external surface refreshes.
func (t *Texture) GeometryRevision() uint64 {
	if base := t.baseTexture(); base != nil {
		return max(t.geometryRevision, base.geometryRevision)
	}
	return 0
}

// DynamicContent identifies textures produced outside the retained paint
// lifecycle. Their consumers must be redrawn even without component writes.
func (t *Texture) DynamicContent() bool {
	base := t.baseTexture()
	return base != nil && (base.external || base.renderTarget != nil)
}

func CreateTextureByImageNoMipmap(im *image.RGBA) *Texture {
	return CreateTextureByImageWithUsage(im, false, ResourceUsageDefault)
}

// CreateTextureFromExternal imports a platform-owned surface without copying
// pixels through Go memory. The active render driver must implement
// render.ExternalTextureDriver.
func CreateTextureFromExternal(descriptor render.ExternalTextureDescriptor) (*Texture, error) {
	if err := descriptor.Validate(); err != nil {
		return nil, err
	}
	driver, ok := render.Current().(render.ExternalTextureDriver)
	if !ok {
		return nil, errors.New("graphics: active render driver cannot import external textures")
	}
	handle, err := driver.CreateExternalTexture(descriptor)
	if err != nil {
		return nil, err
	}
	if !handle.Valid() {
		return nil, errors.New("graphics: render driver returned an invalid external texture")
	}
	texture := &Texture{
		Width: descriptor.Width, Height: descriptor.Height,
		SourceWidth: descriptor.Width, SourceHeight: descriptor.Height,
		MaxU: 1, MaxV: 1, batchKey: nextTextureBatchKey(),
		usage: ResourceUsageGPU, handle: handle, device: render.ActiveDevice(), external: true, externalDevice: render.ActiveDevice(),
	}
	texture.deviceValues().Set(texture.device, handle)
	return texture, nil
}

// CreateTextureFromRenderTarget exposes a target color attachment to 2D/UI
// rendering without transferring ownership of the attachment. Construction is
// CPU-only; RenderHandleFor realizes the attachment on the supplied device.
func CreateTextureFromRenderTarget(target *render.RenderTarget) (*Texture, error) {
	if target == nil {
		return nil, errors.New("graphics: nil render target")
	}
	if !target.Valid() {
		return nil, errors.New("graphics: invalid render target")
	}
	descriptor := target.Descriptor()
	if descriptor.ColorFormat == render.RenderTargetUint32 {
		return nil, errors.New("graphics: integer render targets cannot be sampled as canvas textures")
	}
	texture := &Texture{
		Width: descriptor.Width, Height: descriptor.Height,
		SourceWidth: descriptor.Width, SourceHeight: descriptor.Height,
		MaxU: 1, MaxV: 1, batchKey: nextTextureBatchKey(), usage: ResourceUsageGPU,
		renderTarget: target,
	}
	if descriptor.ColorFormat == render.RenderTargetR8 {
		texture.format = render.TextureFormatR8
	}
	return texture, nil
}

// RefreshExternal marks a newly produced version of an imported surface ready
// for rendering. The native texture handle and graphics.Texture remain stable.
func (t *Texture) RefreshExternal() error {
	if t == nil || !t.external || t.externalDevice.Driver() == nil {
		return errors.New("graphics: invalid external texture refresh")
	}
	t.contentRevision = textureRevision.Add(1)
	handle, valid := t.deviceValues().Get(t.externalDevice)
	if !valid {
		return errors.New("graphics: invalid external texture refresh")
	}
	driver, ok := t.externalDevice.Driver().(render.ExternalTextureRefreshDriver)
	if !ok {
		return errors.New("graphics: active render driver cannot refresh external textures")
	}
	return driver.RefreshExternalTexture(handle)
}

func CreateTextureByImageWithUsage(im *image.RGBA, mipmap bool, usage ResourceUsage) *Texture {
	return createTextureByImageWithUsage(im, mipmap, usage, false)
}

// CreateTextureBySharedImage shares a tightly packed image's pixels with the
// texture's CPU shadow. Other image layouts are copied. The caller must
// synchronize image writes with texture use and call UpdateRegions after edits.
// Resizing requires a new texture and a separate pixel buffer for the old one.
func CreateTextureBySharedImage(im *image.RGBA, mipmap bool, usage ResourceUsage) *Texture {
	return createTextureByOwnedImageWithUsage(im, mipmap, usage)
}

// CreateTextureR8 creates a single-channel unorm texture. The pixel slice is
// retained for device recreation when usage includes ResourceUsageCPU.
func CreateTextureR8(width, height int, pixels []byte, mipmap bool, usage ResourceUsage) *Texture {
	if width <= 0 || height <= 0 || len(pixels) != width*height {
		return &Texture{batchKey: nextTextureBatchKey(), usage: usage, format: render.TextureFormatR8}
	}
	return newTextureWithFormat(width, height, mipmap, usage, pixels, render.TextureFormatR8)
}

// CreateEmptyTexture creates a zero-filled, single-level texture. Its CPU shadow
// grows only through the last updated texel; the untouched suffix remains zero.
// Device recreation expands this compact shadow into a temporary upload payload.
// GPU storage is created on first use, including any pending region updates.
func CreateEmptyTexture(width, height int, format render.TextureFormat, usage ResourceUsage) *Texture {
	texture := &Texture{batchKey: nextTextureBatchKey(), usage: usage, format: format}
	if width <= 0 || height <= 0 || format.TexelBytes() == 0 {
		return texture
	}
	texture.Width, texture.SourceWidth = width, width
	texture.Height, texture.SourceHeight = height, height
	texture.MaxU, texture.MaxV = 1, 1
	texture.zeroInitialized = true
	return texture
}

func createTextureByOwnedImageWithUsage(im *image.RGBA, mipmap bool, usage ResourceUsage) *Texture {
	if im == nil {
		return &Texture{batchKey: nextTextureBatchKey(), usage: usage}
	}
	return createTextureByOwnedPixels(im.Bounds(), im.Stride, im.Pix, mipmap, usage)
}

func createTextureByImageWithUsage(im *image.RGBA, mipmap bool, usage ResourceUsage, takeOwnership bool) *Texture {
	if im == nil {
		return &Texture{batchKey: nextTextureBatchKey(), usage: usage}
	}
	width, height := im.Bounds().Dx(), im.Bounds().Dy()
	if takeOwnership {
		return createTextureByOwnedPixels(im.Bounds(), im.Stride, im.Pix, mipmap, usage)
	}
	pixels := make([]byte, width*height*4)
	for y := range height {
		sourceOffset := im.PixOffset(im.Bounds().Min.X, im.Bounds().Min.Y+y)
		source := im.Pix[sourceOffset : sourceOffset+width*4]
		copy(pixels[y*width*4:], source)
	}
	return newTexture(width, height, mipmap, usage, pixels)
}

func createTextureByOwnedPixels(bounds image.Rectangle, stride int, source []byte, mipmap bool, usage ResourceUsage) *Texture {
	width, height := bounds.Dx(), bounds.Dy()
	if bounds.Min == (image.Point{}) && stride == width*4 && len(source) == width*height*4 {
		return newTexture(width, height, mipmap, usage, source)
	}
	pixels := make([]byte, width*height*4)
	for y := range height {
		sourceOffset := y * stride
		copy(pixels[y*width*4:], source[sourceOffset:sourceOffset+width*4])
	}
	return newTexture(width, height, mipmap, usage, pixels)
}

func newTexture(width, height int, mipmap bool, usage ResourceUsage, pixels []byte) *Texture {
	return newTextureWithFormat(width, height, mipmap, usage, pixels, render.TextureFormatRGBA8)
}

func newTextureWithFormat(width, height int, mipmap bool, usage ResourceUsage, pixels []byte, format render.TextureFormat) *Texture {
	texture := &Texture{
		Width:        width,
		Height:       height,
		SourceWidth:  width,
		SourceHeight: height,
		MaxU:         1,
		MaxV:         1,
		batchKey:     nextTextureBatchKey(),
		mipmap:       mipmap,
		usage:        usage,
		format:       format,
		pix:          pixels,
	}
	return texture
}

// Format reports the texture's texel layout.
func (t *Texture) Format() render.TextureFormat {
	if t == nil {
		return render.TextureFormatRGBA8
	}
	return t.baseTexture().format
}

func (t *Texture) Dispose() error {
	if t == nil {
		return nil
	}
	t.contentRevision = textureRevision.Add(1)
	t.geometryRevision = t.contentRevision
	if t.source != nil {
		t.source = nil
		t.Width = 0
		t.Height = 0
		return nil
	}
	if t.renderTarget != nil {
		t.renderTarget = nil
		t.handle = render.TextureHandle{}
		t.device = nil
		t.Width = 0
		t.Height = 0
		return nil
	}
	if err := t.destroyHandle(); err != nil {
		return err
	}
	t.Width = 0
	t.Height = 0
	t.pix = nil
	t.zeroInitialized = false
	t.err = nil
	t.updates = nil
	t.externalDevice = nil
	return nil
}

type TextureRegionUpdate struct {
	Pixels []byte
	X      int
	Y      int
	Width  int
	Height int
}

func (t *Texture) UpdateRegions(updates []TextureRegionUpdate) {
	base := t.baseTexture()
	if base == nil || len(updates) == 0 {
		return
	}
	// Block-compressed texels are not independently addressable, so a partial
	// region cannot be re-encoded in place.
	texelBytes := base.format.TexelBytes()
	if texelBytes == 0 {
		return
	}
	pixelCount := len(base.pix)
	for _, update := range updates {
		if update.X < 0 || update.Y < 0 || update.Width <= 0 || update.Height <= 0 ||
			update.X+update.Width > base.Width || update.Y+update.Height > base.Height ||
			len(update.Pixels) != update.Width*update.Height*texelBytes {
			return
		}
		pixelCount = max(pixelCount, ((update.Y+update.Height-1)*base.Width+update.X+update.Width)*texelBytes)
	}

	wasResident := false
	_ = base.deviceValues().Each(func(_ *render.Device, _ render.TextureHandle) error { wasResident = true; return nil })
	if !base.usage.Has(ResourceUsageCPU) && len(base.pix) == 0 && !base.zeroInitialized && !wasResident {
		return
	}
	base.contentRevision = textureRevision.Add(1)
	retainPixels := base.usage.Has(ResourceUsageCPU) || !wasResident
	if retainPixels {
		if !base.zeroInitialized {
			pixelCount = base.Width * base.Height * texelBytes
		}
		if pixelCount > cap(base.pix) {
			pixels := make([]byte, pixelCount, min(base.payloadBytes(), max(pixelCount, 2*cap(base.pix))))
			copy(pixels, base.pix)
			base.pix = pixels
		} else {
			base.pix = base.pix[:pixelCount]
		}
		for _, update := range updates {
			// Generated rasters already write the shared CPU shadow. Only the
			// driver upload is needed when source and destination rows coincide.
			if update.Width == base.Width && &base.pix[update.Y*base.Width*texelBytes] == &update.Pixels[0] {
				continue
			}
			rowSize := update.Width * texelBytes
			for row := range update.Height {
				dstOffset := ((update.Y+row)*base.Width + update.X) * texelBytes
				srcOffset := row * rowSize
				copy(base.pix[dstOffset:dstOffset+rowSize], update.Pixels[srcOffset:srcOffset+rowSize])
			}
		}
	}

	base.err = nil
	base.err = base.deviceValues().Each(func(device *render.Device, handle render.TextureHandle) error {
		if cap(base.updates) < len(updates) {
			base.updates = make([]render.TextureUpdate, len(updates))
		} else {
			base.updates = base.updates[:len(updates)]
		}
		for index, update := range updates {
			base.updates[index] = render.TextureUpdate{Texture: handle,
				Region: render.TextureRegion{X: update.X, Y: update.Y, Width: update.Width, Height: update.Height}, Pixels: update.Pixels}
		}
		return device.Driver().UpdateTextures(base.updates)
	})
	clear(base.updates)
	base.updates = base.updates[:0]
	if wasResident && base.err == nil && !base.usage.Has(ResourceUsageCPU) {
		base.pix = nil
	}
}

func (t *Texture) Valid() bool {
	return t.ValidFor(render.ActiveDevice())
}

func (t *Texture) ValidFor(device *render.Device) bool {
	base := t.baseTexture()
	if base == nil {
		return false
	}
	if base.renderTarget != nil {
		if !base.renderTarget.Valid() {
			return false
		}
		base.syncTargetSize()
		return true
	}
	if base.Width <= 0 || base.Height <= 0 {
		return false
	}
	if base.hasPixelPayload() {
		return true
	}
	handle, ok := base.deviceValues().Get(device)
	return ok && handle.Valid()
}

func (t *Texture) hasPixelPayload() bool {
	return len(t.pix) == t.payloadBytes() || t.zeroInitialized
}

// payloadLevels reports how many mip levels the CPU payload holds. Uncompressed
// textures hold exactly one, because the device generates the chain; compressed
// textures ship a prebuilt chain.
func (t *Texture) payloadLevels() int {
	if t.format.Compressed() {
		return max(t.mipLevels, 1)
	}
	return 1
}

// payloadBytes reports the encoded size of the texture's CPU pixel payload.
func (t *Texture) payloadBytes() int {
	return t.format.ChainSize(t.Width, t.Height, t.payloadLevels())
}

// MipLevels reports the depth the uploaded texture will have. It differs from
// payloadLevels for uncompressed textures, whose chain the device builds.
func (t *Texture) MipLevels() int {
	if t == nil {
		return 0
	}
	base := t.baseTexture()
	if base.format.Compressed() {
		return max(base.mipLevels, 1)
	}
	if base.mipmap {
		return render.MipLevels(base.Width, base.Height)
	}
	return 1
}

func (t *Texture) RenderHandle() render.TextureHandle {
	return t.RenderHandleFor(render.ActiveDevice())
}

func (t *Texture) RenderHandleFor(device *render.Device) render.TextureHandle {
	base := t.baseTexture()
	if base == nil {
		return render.TextureHandle{}
	}
	base.ensureUploadedFor(device)
	return base.handle
}

func (t *Texture) UploadError() error {
	base := t.baseTexture()
	if base == nil {
		return nil
	}
	return base.err
}

func (t *Texture) BatchKey() uint64 {
	if t == nil {
		return 0
	}
	return t.batchKey
}

func (t *Texture) PremultipliedAlpha() bool {
	return t != nil && t.premultipliedAlpha
}

func (t *Texture) Mipmapped() bool {
	return t != nil && t.mipmap
}

func NewSubTexture(parent *Texture, width, height int, u, v, maxU, maxV, angle float32) *Texture {
	texture := &Texture{
		Width:        width,
		Height:       height,
		SourceWidth:  width,
		SourceHeight: height,
		U:            u,
		V:            v,
		MaxU:         maxU,
		MaxV:         maxV,
		Angle:        angle,
		batchKey:     nextTextureBatchKey(),
	}
	if parent != nil {
		texture.batchKey = parent.batchKey
		texture.source = parent
	}
	return texture
}

// SetSourceRect records the untrimmed logical size and the trimmed frame offset.
// Atlas formats use this information to draw packed sprites without stretching
// the cropped pixels across the original sprite bounds.
func (t *Texture) SetSourceRect(width, height, offsetX, offsetY int) {
	if t == nil {
		return
	}
	t.contentRevision = textureRevision.Add(1)
	t.geometryRevision = t.contentRevision
	if width <= 0 {
		width = t.Width
	}
	if height <= 0 {
		height = t.Height
	}
	t.SourceWidth = width
	t.SourceHeight = height
	t.OffsetX = offsetX
	t.OffsetY = offsetY
	t.rasterScaleX = 0
	t.rasterScaleY = 0
}

// SetLogicalSize records the layout size of a higher-resolution texture.
// Unlike SetSourceRect, it preserves the complete texture as drawable content.
func (t *Texture) SetLogicalSize(width, height int) {
	if t == nil {
		return
	}
	t.contentRevision = textureRevision.Add(1)
	t.geometryRevision = t.contentRevision
	if width <= 0 {
		width = t.Width
	}
	if height <= 0 {
		height = t.Height
	}
	t.SourceWidth = width
	t.SourceHeight = height
	t.OffsetX = 0
	t.OffsetY = 0
	if t.Width > 0 {
		t.rasterScaleX = float32(t.Width) / float32(width)
	}
	if t.Height > 0 {
		t.rasterScaleY = float32(t.Height) / float32(height)
	}
}

func (t *Texture) Trimmed() bool {
	return t != nil && (t.OffsetX != 0 || t.OffsetY != 0 || t.SourceWidth != t.packedLogicalWidth() || t.SourceHeight != t.packedLogicalHeight())
}

// RasterScale returns the physical pixels represented by one logical texture unit.
func (t *Texture) RasterScale() (x, y float32) {
	if t == nil {
		return 1, 1
	}
	x, y = t.rasterScaleX, t.rasterScaleY
	if x <= 0 {
		x = 1
	}
	if y <= 0 {
		y = 1
	}
	return x, y
}

func (t *Texture) packedLogicalWidth() int {
	if t == nil {
		return 0
	}
	scale, _ := t.RasterScale()
	return int(float32(t.Width)/scale + .5)
}

func (t *Texture) packedLogicalHeight() int {
	if t == nil {
		return 0
	}
	_, scale := t.RasterScale()
	return int(float32(t.Height)/scale + .5)
}

func (t *Texture) LogicalWidth() int {
	if t == nil {
		return 0
	}
	if t.SourceWidth > 0 {
		return t.SourceWidth
	}
	return t.Width
}

func (t *Texture) LogicalHeight() int {
	if t == nil {
		return 0
	}
	if t.SourceHeight > 0 {
		return t.SourceHeight
	}
	return t.Height
}

func (t *Texture) syncTargetSize() {
	descriptor := t.renderTarget.Descriptor()
	if t.Width != descriptor.Width || t.Height != descriptor.Height {
		t.Width, t.Height = descriptor.Width, descriptor.Height
		t.SourceWidth, t.SourceHeight = descriptor.Width, descriptor.Height
		t.contentRevision = textureRevision.Add(1)
		t.geometryRevision = t.contentRevision
	}
}

func (t *Texture) ensureUploadedFor(device *render.Device) {
	if t == nil {
		return
	}
	if t.renderTarget != nil {
		t.syncTargetSize()
		t.handle, t.err = t.renderTarget.ColorTextureFor(device)
		t.device = device
		return
	}
	if handle, ok := t.deviceValues().Get(device); ok {
		t.device, t.handle, t.err = device, handle, nil
		return
	}
	t.device, t.handle = device, render.TextureHandle{}
	if !t.usage.Has(ResourceUsageGPU) || t.Width <= 0 || t.Height <= 0 || !t.hasPixelPayload() {
		if t.Width > 0 && t.usage.Has(ResourceUsageGPU) && len(t.pix) == 0 && device.Driver() != nil {
			t.err = errTextureDevice
		}
		return
	}
	driver := device.Driver()
	if driver == nil {
		return
	}
	// A mipmap sampler filter is only valid when the texture actually has a
	// chain; asking for one on a single-level texture samples undefined data.
	filter := render.TextureFilterLinear
	if t.mipmap && t.MipLevels() > 1 {
		filter = render.TextureFilterLinearMipmapLinear
	}
	pixels := t.pix
	if t.zeroInitialized && len(pixels) < t.payloadBytes() {
		scratch := acquireTextureUploadScratch(t.payloadBytes())
		defer releaseTextureUploadScratch(scratch)
		pixels = scratch.pixels
		copy(pixels, t.pix)
		clear(pixels[len(t.pix):])
	}
	t.handle, t.err = driver.CreateTexture(render.TextureDescriptor{
		WrapU: t.wrapU, WrapV: t.wrapV,
		Width:  t.Width,
		Height: t.Height,
		Filter: filter,
		Format: t.format,
		Levels: t.payloadLevels(),
	}, pixels)
	if t.err == nil {
		t.deviceValues().Set(device, t.handle)
		if !t.usage.Has(ResourceUsageCPU) {
			t.pix = nil
			t.zeroInitialized = false
		}
	}
}

func (t *Texture) destroyHandle() error {
	if t != nil && t.renderTarget != nil {
		t.handle = render.TextureHandle{}
		t.device = nil
		return nil
	}
	if t == nil {
		return nil
	}
	if err := t.deviceValues().Release(func(device *render.Device, handle render.TextureHandle) error {
		return device.Driver().DestroyTexture(handle)
	}); err != nil {
		return err
	}
	t.handle = render.TextureHandle{}
	t.device = nil
	return nil
}

func (t *Texture) baseTexture() *Texture {
	if t == nil || t.source == nil {
		return t
	}
	return t.source.baseTexture()
}

func nextTextureBatchKey() uint64 {
	return textureBatchCounter.Add(1)
}

// AddressModes reports the sampler addressing shared by all views of a texture.
func (t *Texture) AddressModes() (render.TextureAddressMode, render.TextureAddressMode) {
	if t == nil {
		return render.TextureAddressClamp, render.TextureAddressClamp
	}
	base := t.baseTexture()
	return base.wrapU, base.wrapV
}
