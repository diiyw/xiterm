package window

import (
	"math"
	"slices"
)

// InputRegionRect describes an interactive rectangle in logical viewport coordinates.
type InputRegionRect struct {
	X      int
	Y      int
	Width  int
	Height int
}

func cloneInputRegion(rects []InputRegionRect) []InputRegionRect {
	return slices.Clone(rects)
}

func inputRegionsEqual(left, right []InputRegionRect) bool {
	return (left == nil) == (right == nil) && slices.Equal(left, right)
}

func (s viewportState) nativeInputRegion(rects []InputRegionRect) []InputRegionRect {
	if rects == nil {
		return nil
	}
	native := make([]InputRegionRect, 0, len(rects))
	for _, rect := range rects {
		if rect.Width <= 0 || rect.Height <= 0 {
			continue
		}
		mapped, ok := s.InputRect(float64(rect.X), float64(rect.Y), float64(rect.Width), float64(rect.Height))
		if !ok {
			continue
		}
		left := int(math.Floor(mapped.X))
		top := int(math.Floor(mapped.Y))
		right := int(math.Ceil(mapped.X + mapped.Width))
		bottom := int(math.Ceil(mapped.Y + mapped.Height))
		if right <= left || bottom <= top {
			continue
		}
		native = append(native, InputRegionRect{
			X:      left,
			Y:      top,
			Width:  right - left,
			Height: bottom - top,
		})
	}
	return native
}
