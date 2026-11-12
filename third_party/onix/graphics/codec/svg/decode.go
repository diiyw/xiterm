package svg

import (
	"bytes"
	"fmt"

	"github.com/onix-engine/onix/graphics"
	onixsvg "github.com/onix-engine/onix/images/svg"
)

// Decoder retains twice the intrinsic resolution and uses mipmaps when reduced.
type Decoder struct{}

func (Decoder) ImageExtension() string { return ".svg" }

func (Decoder) DecodeImage(data []byte, options graphics.TextureOptions) (*graphics.Texture, error) {
	return (ScaledDecoder[DoubleRasterScale]{}).DecodeImage(data, options)
}

func Decode(data []byte, renderOptions onixsvg.Options, textureOptions graphics.TextureOptions) (*graphics.Texture, error) {
	image, err := onixsvg.Render(bytes.NewReader(data), renderOptions)
	if err != nil {
		return nil, err
	}
	if !textureOptions.PremultiplyAlpha {
		extendTransparentColors(image)
	}
	return graphics.TextureFromOwnedNRGBA(image, textureOptions), nil
}

// RasterScale selects a physical resolution multiplier from intrinsic dimensions.
// The texture keeps the SVG's intrinsic size as its logical sprite size.
type RasterScale interface {
	SVGTextureRasterScale(width, height int) int
}

// DoubleRasterScale rasterizes SVG textures at twice their logical dimensions.
type DoubleRasterScale struct{}

func (DoubleRasterScale) SVGTextureRasterScale(_, _ int) int { return 2 }

// ScaledDecoder rasterizes SVG files at the scale selected by S and enables
// mipmaps so minification integrates thin contours instead of undersampling them.
type ScaledDecoder[S RasterScale] struct{}

func (ScaledDecoder[S]) ImageExtension() string { return ".svg" }

func (ScaledDecoder[S]) DecodeImage(data []byte, options graphics.TextureOptions) (*graphics.Texture, error) {
	document, err := onixsvg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var scale S
	options.Mipmap = true
	width, height := int(document.Width+0.5), int(document.Height+0.5)
	return decodeDocument(document, scale.SVGTextureRasterScale(width, height), options)
}

// DecodeAtScale rasterizes an SVG at a higher physical resolution without
// changing its logical dimensions when used by sprites and nine-slice borders.
func DecodeAtScale(data []byte, scale int, textureOptions graphics.TextureOptions) (*graphics.Texture, error) {
	if scale < 1 {
		return nil, fmt.Errorf("svg: raster scale must be at least 1")
	}
	if scale == 1 {
		return Decode(data, onixsvg.Options{}, textureOptions)
	}
	document, err := onixsvg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return decodeDocument(document, scale, textureOptions)
}

func decodeDocument(document *onixsvg.Document, scale int, textureOptions graphics.TextureOptions) (*graphics.Texture, error) {
	if scale < 1 {
		return nil, fmt.Errorf("svg: raster scale must be at least 1")
	}
	width, height := int(document.Width+0.5), int(document.Height+0.5)
	image, err := document.Render(onixsvg.Options{Width: width * scale, Height: height * scale})
	if err != nil {
		return nil, err
	}
	if !textureOptions.PremultiplyAlpha {
		extendTransparentColors(image)
	}
	texture := graphics.TextureFromOwnedNRGBA(image, textureOptions)
	texture.SetLogicalSize(width, height)
	return texture, nil
}
