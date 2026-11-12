package scene

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

type PerspectiveProjection struct {
	Fov  float32
	Near float32
}

func NewPerspectiveProjection() PerspectiveProjection {
	return PerspectiveProjection{Fov: float32(math.Pi / 4), Near: 0.1}
}

func (p PerspectiveProjection) Matrix(aspect float32) mgl32.Mat4 {
	if p.Fov <= 0 {
		p.Fov = float32(math.Pi / 4)
	}
	if p.Near <= 0 {
		p.Near = 0.1
	}
	f := float32(1 / math.Tan(float64(p.Fov)*0.5))
	return mgl32.Mat4{
		f / aspect, 0, 0, 0,
		0, f, 0, 0,
		0, 0, 0, -1,
		0, 0, p.Near, 0,
	}
}
