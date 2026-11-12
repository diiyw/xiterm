package svg

import "image"

// The sprite pipeline samples straight alpha and premultiplies afterwards.
// Extend edge colors through transparent texels so filtering and mip generation
// do not mix the contour with transparent black. Alpha and visible colors stay
// intact. The two dependent sweeps compute Manhattan distance in linear time.
func extendTransparentColors(img *image.NRGBA) {
	width, height := img.Rect.Dx(), img.Rect.Dy()
	if width == 0 || height == 0 {
		return
	}
	// Most fully opaque SVGs need no scratch storage.
	transparent := false
	visible := false
	for offset := 3; offset < len(img.Pix); offset += 4 {
		transparent = transparent || img.Pix[offset] == 0
		visible = visible || img.Pix[offset] != 0
		if transparent && visible {
			break
		}
	}
	if !transparent || !visible {
		return
	}
	distances := make([]int, width*height)
	copyColor := func(target, source int) {
		copy(img.Pix[target*4:target*4+3], img.Pix[source*4:source*4+3])
	}
	for y := range height {
		for x := range width {
			index := y*width + x
			if img.Pix[index*4+3] != 0 {
				continue
			}
			distances[index] = width + height
			if x > 0 && distances[index-1]+1 < distances[index] {
				distances[index] = distances[index-1] + 1
				copyColor(index, index-1)
			}
			if y > 0 && distances[index-width]+1 < distances[index] {
				distances[index] = distances[index-width] + 1
				copyColor(index, index-width)
			}
		}
	}
	for y := height - 1; y >= 0; y-- {
		for x := width - 1; x >= 0; x-- {
			index := y*width + x
			if distances[index] == 0 {
				continue
			}
			if x+1 < width && distances[index+1]+1 < distances[index] {
				distances[index] = distances[index+1] + 1
				copyColor(index, index+1)
			}
			if y+1 < height && distances[index+width]+1 < distances[index] {
				distances[index] = distances[index+width] + 1
				copyColor(index, index+width)
			}
		}
	}
}
