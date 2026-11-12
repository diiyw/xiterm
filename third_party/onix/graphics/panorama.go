package graphics

import (
	"github.com/go-gl/mathgl/mgl32"
	"math"
)

// LinearPanorama supplies row-major linear RGB texels.
type LinearPanorama interface {
	Size() (int, int)
	At(int, int) mgl32.Vec3
}

// SamplePanorama bilinearly samples an equirectangular panorama without
// allocating. Longitude wraps; latitude clamps at the poles.
func SamplePanorama[S LinearPanorama](p S, n mgl32.Vec3) mgl32.Vec3 {
	width, height := p.Size()
	u := math.Atan2(float64(n.Z()), float64(n.X()))/(2*math.Pi) + .5
	v := math.Acos(float64(max(-1, min(1, n.Y())))) / math.Pi
	x, y := u*float64(width)-.5, v*float64(height)-.5
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	tx, ty := float32(x-float64(x0)), float32(y-float64(y0))
	x0 = (x0 + width) % width
	x1 := (x0 + 1) % width
	y1 := max(0, min(height-1, y0+1))
	y0 = max(0, min(height-1, y0))
	a := p.At(x0, y0).Mul(1 - tx).Add(p.At(x1, y0).Mul(tx))
	b := p.At(x0, y1).Mul(1 - tx).Add(p.At(x1, y1).Mul(tx))
	return a.Mul(1 - ty).Add(b.Mul(ty))
}
