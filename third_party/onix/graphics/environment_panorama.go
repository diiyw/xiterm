package graphics

import (
	"fmt"
	"image"
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// EnvironmentImages keeps the visible panorama at its original resolution,
// independently of the lower-resolution GGX/diffuse/BRDF lighting atlas.
// Both images carry RGBE radiance; the atlas's BRDF tile is ordinary RGBA8.
type EnvironmentImages struct{ Panorama, Atlas *image.NRGBA }

type panoramaLevel struct {
	pixels        []mgl32.Vec3
	width, height int
}

func (p panoramaLevel) Size() (int, int)       { return p.width, p.height }
func (p panoramaLevel) At(x, y int) mgl32.Vec3 { return p.pixels[y*p.width+x] }

type filteredPanorama[S LinearPanorama] struct {
	source        S
	levels        []panoramaLevel
	width, height int
	diffuse       [9]mgl32.Vec3
}

func (p *filteredPanorama[S]) Radiance(n mgl32.Vec3) mgl32.Vec3 { return SamplePanorama(p.source, n) }

// BakePanoramaEnvironment builds solid-angle-weighted radiance mips before
// convolution. A sample's PDF determines its footprint, preventing small HDR
// lights from becoming isolated bright pixels in rough reflection levels.
func BakePanoramaEnvironment[S LinearPanorama](source S, width, samples int) (EnvironmentImages, error) {
	w, h := source.Size()
	if w < 2 || h < 1 || w != 2*h {
		return EnvironmentImages{}, fmt.Errorf("graphics: environment panorama must have a 2:1 aspect ratio")
	}
	panorama := image.NewNRGBA(image.Rect(0, 0, w, h))
	filtered := &filteredPanorama[S]{source: source, width: w, height: h}
	for mw, mh := w, h; mw > 1 || mh > 1; {
		mw, mh = max(1, (mw+1)/2), max(1, (mh+1)/2)
		filtered.levels = append(filtered.levels, panoramaLevel{width: mw, height: mh, pixels: make([]mgl32.Vec3, mw*mh)})
	}
	// Encode the original image while accumulating its first filtered level.
	// Spherical row areas preserve energy at the poles and under downsampling.
	encoder := rgbeEnvironmentEncoder{}
	first := &filtered.levels[0]
	longitude := make([][5]float64, w)
	deltaPhi := 2 * math.Pi / float64(w)
	for x := range w {
		a, b := 2*math.Pi*(float64(x)/float64(w)-.5), 2*math.Pi*(float64(x+1)/float64(w)-.5)
		longitude[x] = [5]float64{math.Sin(b) - math.Sin(a), math.Cos(a) - math.Cos(b), deltaPhi/2 + (math.Sin(2*b)-math.Sin(2*a))/4, deltaPhi/2 - (math.Sin(2*b)-math.Sin(2*a))/4, (math.Sin(b)*math.Sin(b) - math.Sin(a)*math.Sin(a)) / 2}
	}
	var coefficients [9][3]float64
	for y := range h {
		theta0, theta1 := math.Pi*float64(y)/float64(h), math.Pi*float64(y+1)/float64(h)
		c0, c1, s0, s1 := math.Cos(theta0), math.Cos(theta1), math.Sin(theta0), math.Sin(theta1)
		integralCosSquared := (c0*c0*c0 - c1*c1*c1) / 3
		latitude := [6]float64{c0 - c1, (theta1-theta0)/2 - (math.Sin(2*theta1)-math.Sin(2*theta0))/4, (c0*c0 - c1*c1) / 2, (s1*s1*s1 - s0*s0*s0) / 3, c0 - c1 - integralCosSquared, integralCosSquared}
		weight := float32(math.Cos(math.Pi*float64(y)/float64(h)) - math.Cos(math.Pi*float64(y+1)/float64(h)))
		for x := range w {
			value := source.At(x, y)
			integrals := environmentSHIntegrals(longitude[x], latitude, deltaPhi)
			for coefficient, integral := range integrals {
				for channel, v := range value {
					coefficients[coefficient][channel] += integral * float64(v)
				}
			}
			encoder.encode(panorama.Pix[(y*w+x)*4:], value)
			i := (y/2)*first.width + x/2
			first.pixels[i] = first.pixels[i].Add(value.Mul(weight))
		}
	}
	for i, coefficient := range coefficients {
		kernel := float64(1)
		if i > 0 {
			kernel = 2.0 / 3
		}
		if i > 3 {
			kernel = .25
		}
		filtered.diffuse[i] = mgl32.Vec3{float32(coefficient[0] * kernel), float32(coefficient[1] * kernel), float32(coefficient[2] * kernel)}
	}
	for y := range first.height {
		rowWeight := float32(math.Cos(math.Pi*float64(y*2)/float64(h)) - math.Cos(math.Pi*float64(min(h, y*2+2))/float64(h)))
		for x := range first.width {
			first.pixels[y*first.width+x] = first.pixels[y*first.width+x].Mul(1 / (rowWeight * float32(min(2, w-x*2))))
		}
	}
	for level := 1; level < len(filtered.levels); level++ {
		prev, next := filtered.levels[level-1], &filtered.levels[level]
		// The last odd row/column covers less area than its neighbors.
		stride := 1 << level
		for y := range next.height {
			y0, y1 := y*2, min(prev.height, y*2+2)
			total := float32(math.Cos(math.Pi*float64(y0*stride)/float64(h)) - math.Cos(math.Pi*float64(min(h, y1*stride))/float64(h)))
			for x := range next.width {
				x0, x1 := x*2, min(prev.width, x*2+2)
				columns := min(w, x1*stride) - x0*stride
				var sum mgl32.Vec3
				for sy := y0; sy < y1; sy++ {
					rowWeight := float32(math.Cos(math.Pi*float64(sy*stride)/float64(h)) - math.Cos(math.Pi*float64(min(h, (sy+1)*stride))/float64(h)))
					for sx := x0; sx < x1; sx++ {
						weight := rowWeight * float32(min(stride, w-sx*stride))
						sum = sum.Add(prev.At(sx, sy).Mul(weight))
					}
				}
				next.pixels[y*next.width+x] = sum.Mul(1 / (total * float32(columns)))
			}
		}
	}
	atlas, err := bakeEnvironment(filtered, width, samples, encoder, filteredEnvironmentSampler[S]{})
	if err != nil {
		return EnvironmentImages{}, err
	}
	return EnvironmentImages{Panorama: panorama, Atlas: atlas}, nil
}

type filteredEnvironmentSampler[S LinearPanorama] struct{}

func (filteredEnvironmentSampler[S]) sample(p *filteredPanorama[S], n mgl32.Vec3, solidAngle float32) mgl32.Vec3 {
	// The equirectangular pixel area varies with latitude. Cap the footprint
	// by the finite row area at the pole rather than dividing by zero.
	sinTheta := math.Sqrt(max(0, 1-float64(n.Y())*float64(n.Y())))
	texelArea := 2 * math.Pi * math.Pi * max(sinTheta, math.Sin(math.Pi/(2*float64(p.height)))) / float64(p.width*p.height)
	lod := max(0, min(float64(len(p.levels)), .5*math.Log2(float64(solidAngle)/texelArea)))
	lower := int(lod)
	upper := min(len(p.levels), lower+1)
	sample := func(level int) mgl32.Vec3 {
		if level == 0 {
			return p.Radiance(n)
		}
		return SamplePanorama(p.levels[level-1], n)
	}
	a := sample(lower)
	if lower == upper {
		return a
	}
	t := float32(lod - float64(lower))
	return a.Mul(1 - t).Add(sample(upper).Mul(t))
}

// Nine SH coefficients integrate diffuse illumination over every input texel,
// eliminating the variance caused by tracing a few rays towards tiny lights.
// GGX at roughness 1 has the same normalized cosine kernel as diffuse IBL.
func environmentSHBasis(n mgl32.Vec3) [9]float32 {
	x, y, z := n.X(), n.Y(), n.Z()
	return [9]float32{.2820947918, .4886025119 * x, .4886025119 * y, .4886025119 * z, 1.0925484306 * x * y, 1.0925484306 * y * z, 1.0925484306 * x * z, .3153915653 * (3*z*z - 1), .5462742153 * (x*x - y*y)}
}
func (filteredEnvironmentSampler[S]) diffuse(p *filteredPanorama[S], n mgl32.Vec3) (mgl32.Vec3, bool) {
	var value mgl32.Vec3
	for i, b := range environmentSHBasis(n) {
		value = value.Add(p.diffuse[i].Mul(b))
	}
	for i := range value {
		value[i] = max(0, value[i])
	}
	return value, true
}

// Integrate the SH basis over a whole equirectangular texel. Center samples
// would inject false directional light even into a constant 2x1 panorama.
func environmentSHIntegrals(phi [5]float64, theta [6]float64, deltaPhi float64) [9]float64 {
	area := theta[0] * deltaPhi
	return [9]float64{.2820947918 * area, .4886025119 * theta[1] * phi[0], .4886025119 * theta[2] * deltaPhi, .4886025119 * theta[1] * phi[1], 1.0925484306 * theta[3] * phi[0], 1.0925484306 * theta[3] * phi[1], 1.0925484306 * theta[4] * phi[4], .3153915653 * (3*theta[4]*phi[3] - area), .5462742153 * (theta[4]*phi[2] - theta[5]*deltaPhi)}
}
