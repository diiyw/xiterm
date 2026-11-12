package window

import (
	"fmt"
	"image"
	"image/draw"
)

func loadIconRGBA(data []byte, decoder IconDecoder) (*image.RGBA, error) {
	return loadIconAs(data, decoder, image.NewRGBA)
}

func loadIconNRGBA(data []byte, decoder IconDecoder) (*image.NRGBA, error) {
	return loadIconAs(data, decoder, image.NewNRGBA)
}

func loadIconAs[T draw.Image](data []byte, decoder IconDecoder, create func(image.Rectangle) T) (T, error) {
	var zero T
	if decoder == nil {
		return zero, fmt.Errorf("decode window icon: decoder is not configured")
	}
	src, err := decoder.DecodeIcon(data)
	if err != nil {
		return zero, fmt.Errorf("decode window icon: %w", err)
	}

	bounds := src.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return zero, fmt.Errorf("decode window icon: empty image")
	}

	dst := create(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Src)
	return dst, nil
}

func nrgbaToARGB32(img *image.NRGBA) []uint32 {
	bounds := img.Bounds()
	pixels := make([]uint32, bounds.Dx()*bounds.Dy())
	index := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		row := img.PixOffset(bounds.Min.X, y)
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r := uint32(img.Pix[row+0])
			g := uint32(img.Pix[row+1])
			b := uint32(img.Pix[row+2])
			a := uint32(img.Pix[row+3])
			pixels[index] = (a << 24) | (r << 16) | (g << 8) | b
			index++
			row += 4
		}
	}
	return pixels
}

func rgbaToBGRA(img *image.RGBA) []byte {
	bounds := img.Bounds()
	pixels := make([]byte, bounds.Dx()*bounds.Dy()*4)
	index := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		row := img.PixOffset(bounds.Min.X, y)
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixels[index+0] = img.Pix[row+2]
			pixels[index+1] = img.Pix[row+1]
			pixels[index+2] = img.Pix[row+0]
			pixels[index+3] = img.Pix[row+3]
			index += 4
			row += 4
		}
	}
	return pixels
}
