package font

import (
	"image"
	"slices"
	"sync"
)

type contourPoint struct {
	point   OutlinePoint
	onCurve bool
}

type glyphRasterScratch struct {
	points   []OutlinePoint
	contours []OutlineContour
	mask     []byte
	expanded []contourPoint
}

const glyphRasterScratchByteLimit = 1 << 20

var glyphRasterScratchPool = sync.Pool{New: func() any { return &glyphRasterScratch{} }}

func acquireGlyphRasterScratch() *glyphRasterScratch {
	return glyphRasterScratchPool.Get().(*glyphRasterScratch)
}

func releaseGlyphRasterScratch(scratch *glyphRasterScratch) {
	if cap(scratch.points)*16+cap(scratch.contours)*24+cap(scratch.mask)+cap(scratch.expanded)*24 > glyphRasterScratchByteLimit {
		*scratch = glyphRasterScratch{}
	}
	glyphRasterScratchPool.Put(scratch)
}

func (scratch *glyphRasterScratch) scaleOutline(contours []OutlineContour, scale float64, bounds image.Rectangle, shiftX, shiftY, scaleX float64) []OutlineContour {
	count := 0
	for _, contour := range contours {
		count += len(contour)
	}
	scratch.points = slices.Grow(scratch.points[:0], count)[:count]
	clear(scratch.contours)
	scratch.contours = slices.Grow(scratch.contours[:0], len(contours))[:len(contours)]
	offset := 0
	for index, contour := range contours {
		mapped := scratch.points[offset : offset+len(contour)]
		for i, point := range contour {
			mapped[i] = OutlinePoint{
				X: (point.X*scale + shiftX - float64(bounds.Min.X)) * scaleX,
				Y: float64(bounds.Max.Y) - point.Y*scale + shiftY,
			}
		}
		scratch.contours[index] = mapped
		offset += len(contour)
	}
	return scratch.contours
}
