package graphics

import "math"

// SRGBToLinear and LinearToSRGB leave HDR values unclipped. Alpha is never
// transferred through these functions.
func SRGBToLinear(value float32) float32 {
	if value <= 0.04045 {
		return value / 12.92
	}
	return float32(math.Pow(float64((value+0.055)/1.055), 2.4))
}

func LinearToSRGB(value float32) float32 {
	if value <= 0.0031308 {
		return value * 12.92
	}
	return 1.055*float32(math.Pow(float64(value), 1/2.4)) - 0.055
}

func (color ColorRGBA) Linear() ColorRGBA {
	return ColorRGBA{R: SRGBToLinear(color.R), G: SRGBToLinear(color.G), B: SRGBToLinear(color.B), A: color.A}
}
