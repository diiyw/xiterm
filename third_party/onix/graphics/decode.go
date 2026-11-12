package graphics

import (
	"image"
	"image/color"

	"github.com/onix-engine/onix/render"
)

// TextureFromImage converts a decoded image into a texture.
func TextureFromImage(src image.Image, options TextureOptions) *Texture {
	return decodedTexture(src, options)
}

// TextureFromOwnedDecodedImage takes ownership of a decoder's image buffer.
// RGBA and NRGBA images use their pixels in place when tightly packed; other
// image formats are converted into the texture's RGBA storage.
func TextureFromOwnedDecodedImage(src image.Image, options TextureOptions) *Texture {
	switch image := src.(type) {
	case *image.RGBA:
		return TextureFromOwnedImage(image, options)
	case *image.NRGBA:
		return TextureFromOwnedNRGBA(image, options)
	default:
		return decodedTexture(src, options)
	}
}

// TextureFromOwnedImage transfers an RGBA image buffer into a texture.
func TextureFromOwnedImage(src *image.RGBA, options TextureOptions) *Texture {
	if src == nil {
		return createTextureByOwnedImageWithUsage(nil, options.Mipmap, options.Usage)
	}
	if !options.PremultiplyAlpha {
		unpremultiplyRGBAInPlace(src.Pix, src.Rect, src.Stride)
	}
	texture := createTextureByOwnedImageWithUsage(src, options.Mipmap, ResourceUsageCPU)
	return finishDecodedTexture(texture, options)
}

// TextureFromOwnedNRGBA transfers a straight-alpha NRGBA buffer into a texture.
func TextureFromOwnedNRGBA(src *image.NRGBA, options TextureOptions) *Texture {
	if src == nil {
		return &Texture{batchKey: nextTextureBatchKey(), usage: options.Usage}
	}
	if options.PremultiplyAlpha {
		premultiplyNRGBAInPlace(src.Pix, src.Rect, src.Stride)
	}
	texture := createTextureByOwnedPixels(src.Rect, src.Stride, src.Pix, options.Mipmap, ResourceUsageCPU)
	return finishDecodedTexture(texture, options)
}

// StraightRGBA8 copies a decoded image into a straight-alpha RGBA8 buffer.
// Build tooling uses it so a compressed texture is encoded from exactly the
// bytes the runtime would have uploaded for the same source.
func StraightRGBA8(src image.Image) *image.RGBA {
	return decodedRGBA(src, false)
}

func decodedTexture(src image.Image, options TextureOptions) *Texture {
	texture := createTextureByOwnedImageWithUsage(decodedRGBA(src, options.PremultiplyAlpha), options.Mipmap, ResourceUsageCPU)
	return finishDecodedTexture(texture, options)
}

func decodedRGBA(src image.Image, premultipliedAlpha bool) *image.RGBA {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	switch source := src.(type) {
	case *image.NRGBA:
		convertNRGBA(dst, source, premultipliedAlpha)
	case *image.RGBA:
		convertRGBA(dst, source, premultipliedAlpha)
	case *image.Paletted:
		convertPaletted(dst, source, premultipliedAlpha)
	case *image.YCbCr:
		convertYCbCr(dst, source)
	default:
		convertImage(dst, src, premultipliedAlpha)
	}
	return dst
}

func convertNRGBA(dst *image.RGBA, src *image.NRGBA, premultiply bool) {
	bounds := src.Bounds()
	for y := range bounds.Dy() {
		source := src.Pix[src.PixOffset(bounds.Min.X, bounds.Min.Y+y):]
		target := dst.Pix[y*dst.Stride:]
		for x := range bounds.Dx() {
			offset := x * 4
			writeNRGBA(target[offset:], color.NRGBA{R: source[offset], G: source[offset+1], B: source[offset+2], A: source[offset+3]}, premultiply)
		}
	}
}

func convertRGBA(dst *image.RGBA, src *image.RGBA, premultiply bool) {
	bounds := src.Bounds()
	for y := range bounds.Dy() {
		source := src.Pix[src.PixOffset(bounds.Min.X, bounds.Min.Y+y):]
		target := dst.Pix[y*dst.Stride:]
		for x := range bounds.Dx() {
			offset := x * 4
			a := source[offset+3]
			if premultiply || a == 255 {
				copy(target[offset:offset+4], source[offset:offset+4])
				continue
			}
			if a == 0 {
				clear(target[offset : offset+4])
				continue
			}
			target[offset] = unpremultiply(source[offset], a)
			target[offset+1] = unpremultiply(source[offset+1], a)
			target[offset+2] = unpremultiply(source[offset+2], a)
			target[offset+3] = a
		}
	}
}

func convertPaletted(dst *image.RGBA, src *image.Paletted, premultiply bool) {
	palette := make([]color.NRGBA, len(src.Palette))
	for index, value := range src.Palette {
		palette[index] = color.NRGBAModel.Convert(value).(color.NRGBA)
	}
	bounds := src.Bounds()
	for y := range bounds.Dy() {
		source := src.Pix[src.PixOffset(bounds.Min.X, bounds.Min.Y+y):]
		target := dst.Pix[y*dst.Stride:]
		for x := range bounds.Dx() {
			writeNRGBA(target[x*4:], palette[source[x]], premultiply)
		}
	}
}

func convertYCbCr(dst *image.RGBA, src *image.YCbCr) {
	bounds := src.Bounds()
	for y := range bounds.Dy() {
		target := dst.Pix[y*dst.Stride:]
		for x := range bounds.Dx() {
			sourceX, sourceY := bounds.Min.X+x, bounds.Min.Y+y
			r, g, b := color.YCbCrToRGB(src.Y[src.YOffset(sourceX, sourceY)], src.Cb[src.COffset(sourceX, sourceY)], src.Cr[src.COffset(sourceX, sourceY)])
			offset := x * 4
			target[offset], target[offset+1], target[offset+2], target[offset+3] = r, g, b, 255
		}
	}
}

func convertImage(dst *image.RGBA, src image.Image, premultiply bool) {
	bounds := src.Bounds()
	for y := range bounds.Dy() {
		for x := range bounds.Dx() {
			clr := color.NRGBAModel.Convert(src.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			writeNRGBA(dst.Pix[y*dst.Stride+x*4:], clr, premultiply)
		}
	}
}

func writeNRGBA(target []byte, clr color.NRGBA, premultiply bool) {
	if premultiply {
		clr.R = uint8((uint16(clr.R)*uint16(clr.A) + 127) / 255)
		clr.G = uint8((uint16(clr.G)*uint16(clr.A) + 127) / 255)
		clr.B = uint8((uint16(clr.B)*uint16(clr.A) + 127) / 255)
	}
	target[0], target[1], target[2], target[3] = clr.R, clr.G, clr.B, clr.A
}

func unpremultiply(value, alpha uint8) uint8 {
	return uint8(min(255, (uint32(value)*255+uint32(alpha)/2)/uint32(alpha)))
}

func premultiplyNRGBAInPlace(pixels []byte, bounds image.Rectangle, stride int) {
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		row := pixels[(y-bounds.Min.Y)*stride:]
		for x := range bounds.Dx() {
			offset := x * 4
			alpha := uint16(row[offset+3])
			row[offset] = uint8((uint16(row[offset])*alpha + 127) / 255)
			row[offset+1] = uint8((uint16(row[offset+1])*alpha + 127) / 255)
			row[offset+2] = uint8((uint16(row[offset+2])*alpha + 127) / 255)
		}
	}
}

func unpremultiplyRGBAInPlace(pixels []byte, bounds image.Rectangle, stride int) {
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		row := pixels[(y-bounds.Min.Y)*stride:]
		for x := range bounds.Dx() {
			offset := x * 4
			alpha := row[offset+3]
			if alpha == 0 {
				clear(row[offset : offset+4])
				continue
			}
			row[offset] = unpremultiply(row[offset], alpha)
			row[offset+1] = unpremultiply(row[offset+1], alpha)
			row[offset+2] = unpremultiply(row[offset+2], alpha)
		}
	}
}

func finishDecodedTexture(texture *Texture, options TextureOptions) *Texture {
	if options.SRGB {
		texture.format = render.TextureFormatSRGBA8
	}
	texture.premultipliedAlpha = options.PremultiplyAlpha
	texture.usage = options.Usage
	texture.wrapU, texture.wrapV = options.WrapU, options.WrapV
	return texture
}
