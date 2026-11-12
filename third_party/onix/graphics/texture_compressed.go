package graphics

import (
	"fmt"

	"github.com/onix-engine/onix/render"
)

// CompressedTextureDescriptor describes a payload that is already in the texel
// layout the device samples. The build produces these; nothing at runtime
// re-encodes them.
type CompressedTextureDescriptor struct {
	Width, Height int
	// Levels is the mip depth carried by the payload. Compressed textures
	// always ship a chain because a device cannot generate mips for block data.
	Levels int
	Format render.TextureFormat
	// Mipmap controls sampler filtering independently from the carried chain.
	Mipmap bool
	// Premultiplied reports colour channels already scaled by alpha, which the
	// encoder baked in and the runtime cannot recover.
	Premultiplied bool
	WrapU, WrapV  render.TextureAddressMode
}

func (d CompressedTextureDescriptor) validate(pixels []byte) error {
	if !d.Format.Valid() || !d.Format.Compressed() {
		return fmt.Errorf("graphics: %s is not a block-compressed format", d.Format.Name())
	}
	if d.Width <= 0 || d.Height <= 0 {
		return fmt.Errorf("graphics: invalid compressed texture size %dx%d", d.Width, d.Height)
	}
	if d.Levels <= 0 || d.Levels > render.MipLevels(d.Width, d.Height) {
		return fmt.Errorf("graphics: %dx%d supports %d mip levels, got %d",
			d.Width, d.Height, render.MipLevels(d.Width, d.Height), d.Levels)
	}
	if expected := d.Format.ChainSize(d.Width, d.Height, d.Levels); len(pixels) != expected {
		return fmt.Errorf("graphics: %s %dx%d with %d levels needs %d bytes, got %d",
			d.Format.Name(), d.Width, d.Height, d.Levels, expected, len(pixels))
	}
	return nil
}

// TextureFromCompressed adopts a prebuilt block-compressed payload. The caller
// transfers ownership of pixels, which must hold the complete mip chain.
//
// The texture is created GPU-only: compressed blocks cannot be read back or
// partially updated, so retaining a CPU copy would only waste memory.
func TextureFromCompressed(descriptor CompressedTextureDescriptor, pixels []byte) (*Texture, error) {
	if err := descriptor.validate(pixels); err != nil {
		return nil, err
	}
	texture := &Texture{
		Width:              descriptor.Width,
		Height:             descriptor.Height,
		SourceWidth:        descriptor.Width,
		SourceHeight:       descriptor.Height,
		MaxU:               1,
		MaxV:               1,
		wrapU:              descriptor.WrapU,
		wrapV:              descriptor.WrapV,
		batchKey:           nextTextureBatchKey(),
		mipmap:             descriptor.Mipmap,
		mipLevels:          descriptor.Levels,
		premultipliedAlpha: descriptor.Premultiplied,
		usage:              ResourceUsageGPU,
		format:             descriptor.Format,
		pix:                pixels,
	}
	return texture, nil
}
