package scene

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

type tileRange2D struct {
	minX, minY int
	maxX, maxY int
}

func tilemapVisibleRange2D(viewport renderViewport2D, base mgl32.Mat4, tilemap *Tilemap2D) tileRange2D {
	if !viewport.active {
		return tileRange2D{}
	}
	full := tileRange2D{maxX: tilemap.Width, maxY: tilemap.Height}
	matrix := viewport.clipFromLayer.Mul4(base)
	if matrix[3] != 0 || matrix[7] != 0 || matrix[15] == 0 {
		return full
	}
	scale := float64(matrix[15])
	horizontalX, horizontalY := float64(matrix[0])/scale, float64(matrix[1])/scale
	verticalX, verticalY := float64(matrix[4])/scale, float64(matrix[5])/scale
	offsetX, offsetY := float64(matrix[12])/scale, float64(matrix[13])/scale
	determinant := horizontalX*verticalY - verticalX*horizontalY
	if math.Abs(determinant) < 1e-15 || math.IsNaN(determinant) || math.IsInf(determinant, 0) {
		return full
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, corner := range [4][2]float64{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		clipX, clipY := corner[0]-offsetX, corner[1]-offsetY
		localX := (verticalY*clipX - verticalX*clipY) / determinant
		localY := (horizontalX*clipY - horizontalY*clipX) / determinant
		if math.IsNaN(localX) || math.IsNaN(localY) {
			return full
		}
		minX, maxX = min(minX, localX), max(maxX, localX)
		minY, maxY = min(minY, localY), max(maxY, localY)
	}
	return tileRange2D{
		minX: int(max(0, min(float64(tilemap.Width), math.Floor(minX/float64(tilemap.TileWidth))-1))),
		minY: int(max(0, min(float64(tilemap.Height), math.Floor(minY/float64(tilemap.TileHeight))-1))),
		maxX: int(max(0, min(float64(tilemap.Width), math.Ceil(maxX/float64(tilemap.TileWidth))+1))),
		maxY: int(max(0, min(float64(tilemap.Height), math.Ceil(maxY/float64(tilemap.TileHeight))+1))),
	}
}
