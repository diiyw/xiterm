package graphics

import (
	"image/color"
	"strconv"
)

type ColorRGBA struct {
	R float32
	G float32
	B float32
	A float32
}

func ColorFromRGBA(r, g, b, a uint8) ColorRGBA {
	return ColorRGBA{
		R: float32(r) / 255,
		G: float32(g) / 255,
		B: float32(b) / 255,
		A: float32(a) / 255,
	}
}

func ColorFromRGB(r, g, b uint8) ColorRGBA {
	return ColorFromRGBA(r, g, b, 255)
}

// GoColor converts the color to a Go color.
func (clr ColorRGBA) GoColor() color.RGBA {
	return color.RGBA{
		R: uint8(clr.R * 255),
		G: uint8(clr.G * 255),
		B: uint8(clr.B * 255),
		A: uint8(clr.A * 255),
	}
}

// Int32 converts the color to an integer.
func (clr ColorRGBA) Int32() int {
	return int(clr.R*255)<<24 | int(clr.G*255)<<16 | int(clr.B*255)<<8 | int(clr.A*255)
}

// String converts the color to a string.
func (clr ColorRGBA) String() string {
	return "rgba(" + strconv.Itoa(int(clr.R*255)) + "," + strconv.Itoa(int(clr.G*255)) + "," + strconv.Itoa(int(clr.B*255)) + "," + strconv.FormatFloat(float64(clr.A), 'f', -1, 32) + ")"
}

var (
	White = ColorFromRGBA(255, 255, 255, 255)
	Black = ColorFromRGBA(0, 0, 0, 255)
)
