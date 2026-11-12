package render

import (
	"errors"
	"fmt"
)

type RenderTargetColorFormat uint8

const (
	RenderTargetRGBA8 RenderTargetColorFormat = iota
	RenderTargetUint32
	RenderTargetR8
)

type RenderTargetDescriptor struct {
	Width       int
	Height      int
	ColorFormat RenderTargetColorFormat
	Depth       bool
}

func (descriptor RenderTargetDescriptor) Validate() error {
	if descriptor.Width <= 0 || descriptor.Height <= 0 {
		return errors.New("render: render target dimensions must be positive")
	}
	if descriptor.ColorFormat > RenderTargetR8 {
		return errors.New("render: invalid render target color format")
	}
	if descriptor.ColorFormat == RenderTargetR8 && descriptor.Depth {
		return errors.New("render: R8 render targets require no depth attachment")
	}
	return nil
}

// TexelBytes returns the byte size used by color attachment readback.
func (format RenderTargetColorFormat) TexelBytes() int {
	if format == RenderTargetR8 {
		return 1
	}
	return 4
}

type RenderTargetClear struct {
	Color      Color
	Depth      float32
	ClearColor bool
	ClearDepth bool
}

type RenderTargetRegion struct {
	// X and Y use the engine's top-left surface origin.
	X      int
	Y      int
	Width  int
	Height int
}

func flipRenderTargetRows(pixels []byte, stride, height int) {
	for top, bottom := 0, height-1; top < bottom; top, bottom = top+1, bottom-1 {
		topOffset := top * stride
		bottomOffset := bottom * stride
		for column := range stride {
			pixels[topOffset+column], pixels[bottomOffset+column] = pixels[bottomOffset+column], pixels[topOffset+column]
		}
	}
}

type RenderTargetDriver interface {
	CreateRenderTarget(RenderTargetDescriptor) (RenderTargetHandle, TextureHandle, error)
	ResizeRenderTarget(RenderTargetHandle, RenderTargetDescriptor) (TextureHandle, error)
	DestroyRenderTarget(RenderTargetHandle) error
	BeginRenderTarget(RenderTargetHandle, RenderTargetClear) error
	EndRenderTarget() error
}

type RenderTargetReadbackDriver interface {
	ReadRenderTarget(RenderTargetHandle, RenderTargetRegion, []byte) error
}

// RenderTarget owns a renderer-neutral offscreen color attachment and optional
// depth attachment. Resize is deferred until the target is next used.
type RenderTarget struct {
	active      *Device
	residencies DeviceValues[targetResidency]
	descriptor  RenderTargetDescriptor
	handle      RenderTargetHandle
	color       TextureHandle
	disposed    bool
}

type targetResidency struct {
	handle     RenderTargetHandle
	color      TextureHandle
	descriptor RenderTargetDescriptor
}

func NewRenderTarget(descriptor RenderTargetDescriptor) (*RenderTarget, error) {
	if err := descriptor.Validate(); err != nil {
		return nil, err
	}
	return &RenderTarget{descriptor: descriptor}, nil
}

func (target *RenderTarget) Descriptor() RenderTargetDescriptor {
	if target == nil {
		return RenderTargetDescriptor{}
	}
	return target.descriptor
}

// Valid reports whether the target can be realized, without accessing a device.
func (target *RenderTarget) Valid() bool {
	return target != nil && !target.disposed && target.descriptor.Width > 0 && target.descriptor.Height > 0
}

func (target *RenderTarget) Size() (int, int) {
	descriptor := target.Descriptor()
	return descriptor.Width, descriptor.Height
}

func (target *RenderTarget) Resize(width, height int) bool {
	if target == nil || target.disposed || width <= 0 || height <= 0 || target.descriptor.Width == width && target.descriptor.Height == height {
		return false
	}
	target.descriptor.Width = width
	target.descriptor.Height = height
	return true
}

func (target *RenderTarget) ColorTexture() (TextureHandle, error) {
	return target.ColorTextureFor(ActiveDevice())
}

func (target *RenderTarget) ColorTextureFor(device *Device) (TextureHandle, error) {
	if err := target.ensureRealizedFor(device); err != nil {
		return TextureHandle{}, err
	}
	return target.color, nil
}

func (target *RenderTarget) Begin(clear RenderTargetClear) error {
	return target.BeginFor(ActiveDevice(), clear)
}

func (target *RenderTarget) BeginFor(device *Device, clear RenderTargetClear) error {
	if err := target.ensureRealizedFor(device); err != nil {
		return err
	}
	driver, ok := device.Driver().(RenderTargetDriver)
	if !ok {
		return errors.New("render: current driver does not support render targets")
	}
	if err := driver.BeginRenderTarget(target.handle, clear); err != nil {
		return err
	}
	target.active = device
	return nil
}

func (target *RenderTarget) End() error {
	if target == nil || target.disposed || target.active.Driver() == nil {
		return errors.New("render: render target is not active")
	}
	driver, ok := target.active.Driver().(RenderTargetDriver)
	if !ok {
		return errors.New("render: current driver does not support render targets")
	}
	err := driver.EndRenderTarget()
	if err == nil {
		target.active = nil
	}
	return err
}

func (target *RenderTarget) Read(region RenderTargetRegion, destination []byte) error {
	if err := target.ensureRealizedFor(ActiveDevice()); err != nil {
		return err
	}
	if region.X < 0 || region.Y < 0 || region.Width <= 0 || region.Height <= 0 || region.X+region.Width > target.descriptor.Width || region.Y+region.Height > target.descriptor.Height {
		return errors.New("render: render target readback region is invalid")
	}
	if len(destination) != region.Width*region.Height*target.descriptor.ColorFormat.TexelBytes() {
		return fmt.Errorf("render: render target readback requires %d bytes, got %d", region.Width*region.Height*target.descriptor.ColorFormat.TexelBytes(), len(destination))
	}
	driver, ok := Current().(RenderTargetReadbackDriver)
	if !ok {
		return errors.New("render: current driver does not support render target readback")
	}
	return driver.ReadRenderTarget(target.handle, region, destination)
}

func (target *RenderTarget) Dispose() error {
	if target == nil || target.disposed {
		return nil
	}
	target.disposed = true
	if err := target.residencies.Release(func(device *Device, value targetResidency) error {
		return device.Driver().(RenderTargetDriver).DestroyRenderTarget(value.handle)
	}); err != nil {
		target.disposed = false
		return err
	}
	target.handle = RenderTargetHandle{}
	target.color = TextureHandle{}
	target.active = nil
	return nil
}

func (target *RenderTarget) ensureRealizedFor(device *Device) error {
	if target == nil {
		return errors.New("render: nil render target")
	}
	if target.disposed {
		return errors.New("render: render target is disposed")
	}
	if err := target.descriptor.Validate(); err != nil {
		return err
	}
	driver, ok := device.Driver().(RenderTargetDriver)
	if !ok {
		return errors.New("render: current driver does not support render targets")
	}
	value, resident := target.residencies.Get(device)
	if !resident {
		handle, color, err := driver.CreateRenderTarget(target.descriptor)
		if err != nil {
			return err
		}
		if !handle.Valid() || !color.Valid() {
			if handle.Valid() {
				_ = driver.DestroyRenderTarget(handle)
			}
			return errors.New("render: driver created an invalid render target")
		}
		target.handle, target.color = handle, color
		target.residencies.Set(device, targetResidency{handle, color, target.descriptor})
		return nil
	}
	target.handle, target.color = value.handle, value.color
	if value.descriptor == target.descriptor {
		return nil
	}
	color, err := driver.ResizeRenderTarget(target.handle, target.descriptor)
	if err != nil {
		return err
	}
	if !color.Valid() {
		return errors.New("render: driver resized a render target without a color texture")
	}
	target.color = color
	target.residencies.Set(device, targetResidency{target.handle, color, target.descriptor})
	return nil
}
