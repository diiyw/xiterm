// Skia-derived LCD filter and mask preblend mathematics.
// See resources/SKIA_LICENSE for the upstream BSD license.

package font

import (
	"image"
	"math"
)

// LCDColor identifies Skia's eight canonical foreground intensities per
// channel. Coverage correction depends on foreground color, not background.
// Quantization bounds glyph-cache variants to 512 colors.
type LCDColor uint16

func LCDColorFromRGB(r, g, b uint8) LCDColor {
	return LCDColor(r>>5)<<6 | LCDColor(g>>5)<<3 | LCDColor(b>>5)
}

// Chromium's Linux Skia build uses gamma=1.2 and contrast=0.2. The scaler
// stores gamma in unsigned 2.6 fixed point (76/64) before constructing LUTs.
// See chromium/skia/BUILD.gn and Skia SkScalerContextRec / SkMaskGamma.
var lcdPreblend = buildLCDPreblend(76.0/64, 51.0/255)

func buildLCDPreblend(gamma, contrast float64) [8][256]uint8 {
	var tables [8][256]uint8
	for level := 0; level < 8; level++ {
		intensity := (level << 5) | (level << 2) | (level >> 1)
		source := float64(intensity) / 255
		destination := 1 - source
		linearSource, linearDestination := math.Pow(source, gamma), math.Pow(destination, gamma)
		for value := 0; value < 256; value++ {
			coverage := float64(value) / 255
			coverage += coverage * (1 - coverage) * contrast * linearDestination
			output := math.Pow(linearSource*coverage+linearDestination*(1-coverage), 1/gamma)
			corrected := (output - destination) / (source - destination)
			tables[level][value] = uint8(math.Round(max(0, min(1, corrected)) * 255))
		}
	}
	return tables
}

// PreblendLCD converts raw RGB coverage into the mask used by the existing
// component blend passes. Alpha remains opaque. Grayscale masks are unchanged.
func (color LCDColor) PreblendLCD(bitmap *image.RGBA) {
	red, green, blue := &lcdPreblend[(color>>6)&7], &lcdPreblend[(color>>3)&7], &lcdPreblend[color&7]
	for y := 0; y < bitmap.Rect.Dy(); y++ {
		row := bitmap.Pix[y*bitmap.Stride : y*bitmap.Stride+bitmap.Rect.Dx()*4]
		for x := 0; x < len(row); x += 4 {
			row[x], row[x+1], row[x+2] = lcdBlendCoverage(red[row[x]]), lcdBlendCoverage(green[row[x+1]]), lcdBlendCoverage(blue[row[x+2]])
		}
	}
}

// Skia's raster LCD blitter converts all three RGB565 mask channels to five
// bits, then expands 0..31 to 0..32 before blending. Encode that coverage in
// the RGBA atlas; the GPU blend retains the destination alpha.
func lcdBlendCoverage(value uint8) uint8 {
	scale := int(value >> 3)
	scale += scale >> 4
	return uint8((scale*255 + 16) / 32)
}

// GrayColorFromRGB selects Skia's A8 luminance bucket without prematurely
// quantizing the individual foreground channels.
func GrayColorFromRGB(r, g, b uint8) LCDColor {
	lum := uint8((int(r)*54 + int(g)*183 + int(b)*19) >> 8)
	return LCDColorFromRGB(lum, lum, lum)
}

// PreblendGray applies coverage gamma without LCD's five-bit quantization.
func (color LCDColor) PreblendGray(bitmap *image.Alpha) {
	table := &lcdPreblend[(color>>3)&7]
	for y := 0; y < bitmap.Rect.Dy(); y++ {
		row := bitmap.Pix[y*bitmap.Stride : y*bitmap.Stride+bitmap.Rect.Dx()]
		for x, value := range row {
			row[x] = table[value]
		}
	}
}
