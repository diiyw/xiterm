package graphics

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"math/bits"

	"github.com/go-gl/mathgl/mgl32"
)

// RadianceSource supplies linear HDR radiance along a world-space direction.
// BakeEnvironment evaluates the concrete source without interface dispatch.
type RadianceSource interface{ Radiance(mgl32.Vec3) mgl32.Vec3 }

// PanoramaRadiance interprets a conventional sRGB equirectangular image.
type PanoramaRadiance struct{ Image image.Image }

func (p PanoramaRadiance) Radiance(n mgl32.Vec3) mgl32.Vec3 {
	bounds := p.Image.Bounds()
	u := float32(math.Atan2(float64(n.Z()), float64(n.X()))/(2*math.Pi) + 0.5)
	v := float32(math.Acos(float64(max(-1, min(1, n.Y())))) / math.Pi)
	x := min(bounds.Dx()-1, int(u*float32(bounds.Dx()))) + bounds.Min.X
	y := min(bounds.Dy()-1, int(v*float32(bounds.Dy()))) + bounds.Min.Y
	c := color.NRGBAModel.Convert(p.Image.At(x, y)).(color.NRGBA)
	return mgl32.Vec3{SRGBToLinear(float32(c.R) / 255), SRGBToLinear(float32(c.G) / 255), SRGBToLinear(float32(c.B) / 255)}
}

// SkyRadiance is an analytic daylight source for projects without a panorama.
// Sun radiance belongs to the directional light; excluding a disk here avoids
// counting it twice in the environment's diffuse illumination.
type SkyRadiance struct{ Zenith, Horizon, Ground mgl32.Vec3 }

func (sky SkyRadiance) Radiance(n mgl32.Vec3) mgl32.Vec3 {
	if n.Y() < 0 {
		t := float32(math.Sqrt(float64(-n.Y())))
		return sky.Horizon.Mul(1 - t).Add(sky.Ground.Mul(t))
	}
	t := float32(math.Sqrt(float64(n.Y())))
	return sky.Horizon.Mul(1 - t).Add(sky.Zenith.Mul(t))
}

// BakeEnvironment creates an RGBA8 atlas: six GGX reflection levels, one
// cosine-weighted diffuse level (irradiance / pi), and a split-sum BRDF LUT.
// RGBM stores linear radiance in [0,16]. This is asset-time work; the result is
// sampled directly at runtime with no per-frame convolution or allocation.
func BakeEnvironment[S RadianceSource](source S, width, samples int) (*image.NRGBA, error) {
	return bakeEnvironment(source, width, samples, rgbmEnvironmentEncoder{}, directEnvironmentSampler[S]{})
}

// BakeHDREnvironment uses a separate exponent per texel, preserving dim
// colors beside bright light sources without normalizing the entire atlas.
// Its first seven tiles use RGBE; the BRDF tile remains linear RGBA8.
func BakeHDREnvironment[S RadianceSource](source S, width, samples int) (*image.NRGBA, error) {
	return bakeEnvironment(source, width, samples, rgbeEnvironmentEncoder{}, directEnvironmentSampler[S]{})
}

type environmentEncoder interface {
	encode([]byte, mgl32.Vec3)
}

type rgbmEnvironmentEncoder struct{}
type rgbeEnvironmentEncoder struct{}

func bakeEnvironment[S RadianceSource, E environmentEncoder, F environmentRadianceSampler[S]](source S, width, samples int, encoder E, filter F) (*image.NRGBA, error) {
	if width < 16 || width > 2048 || width%2 != 0 || samples < 16 || samples > 4096 {
		return nil, fmt.Errorf("graphics: invalid environment bake dimensions or sample count")
	}
	height := width / 2
	result := image.NewNRGBA(image.Rect(0, 0, width, height*8))
	convolution := make([]environmentConvolutionSample, samples)
	for level := 0; level < 8; level++ {
		if level > 0 && level < 7 {
			prepareEnvironmentConvolution(convolution, level)
		}
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				u := max(0, min(1, (float32(x)-0.5)/float32(width-2)))
				v := max(0, min(1, (float32(y)-0.5)/float32(height-2)))
				pixel := result.Pix[((level*height+y)*width+x)*4:]
				if level == 7 {
					ab := integrateEnvironmentBRDF(max(u, 0.001), max(v, 0.045), samples)
					pixel[0], pixel[1], pixel[2], pixel[3] = byte(max(0, min(1, ab[0]))*255+0.5), byte(max(0, min(1, ab[1]))*255+0.5), 0, 255
					continue
				}
				n := environmentDirection(u, v)
				if level == 5 || level == 6 {
					if diffuse, ok := filter.diffuse(source, n); ok {
						encoder.encode(pixel, diffuse)
						continue
					}
				}
				radiance := filter.sample(source, n, float32(2*math.Pi*math.Pi)*float32(math.Sqrt(float64(max(0, 1-n.Y()*n.Y()))))/float32(width*height))
				if level > 0 {
					var sum mgl32.Vec3
					var weight float32
					up := mgl32.Vec3{0, 0, 1}
					if float32(math.Abs(float64(n.Z()))) > .999 {
						up = mgl32.Vec3{1, 0, 0}
					}
					tangent := up.Cross(n).Normalize()
					bitangent := n.Cross(tangent)
					for _, sample := range convolution {
						if sample.weight <= 0 {
							continue
						}
						local := sample.direction
						l := tangent.Mul(local.X()).Add(bitangent.Mul(local.Y())).Add(n.Mul(local.Z()))
						sum = sum.Add(filter.sample(source, l, sample.solidAngle).Mul(sample.weight))
						weight += sample.weight
					}
					if weight > 0 {
						radiance = sum.Mul(1 / weight)
					}
				}
				encoder.encode(pixel, radiance)
			}
		}
	}
	return result, nil
}

func environmentDirection(u, v float32) mgl32.Vec3 {
	theta := float64(u-0.5) * 2 * math.Pi
	phi := float64(v) * math.Pi
	s := float32(math.Sin(phi))
	return mgl32.Vec3{s * float32(math.Cos(theta)), float32(math.Cos(phi)), s * float32(math.Sin(theta))}
}

func environmentSample(i, count int) mgl32.Vec2 {
	return mgl32.Vec2{(float32(i) + 0.5) / float32(count), float32(float64(bits.Reverse32(uint32(i))) * 2.3283064365386963e-10)}
}

func environmentToWorld(n, local mgl32.Vec3) mgl32.Vec3 {
	up := mgl32.Vec3{0, 0, 1}
	if float32(math.Abs(float64(n.Z()))) > 0.999 {
		up = mgl32.Vec3{1, 0, 0}
	}
	t := up.Cross(n).Normalize()
	b := n.Cross(t)
	return t.Mul(local.X()).Add(b.Mul(local.Y())).Add(n.Mul(local.Z())).Normalize()
}

func environmentGGX(n mgl32.Vec3, xi mgl32.Vec2, roughness float32) mgl32.Vec3 {
	a := roughness * roughness
	phi := 2 * math.Pi * float64(xi[0])
	cosTheta := float32(math.Sqrt(float64((1 - xi[1]) / (1 + (a*a-1)*xi[1]))))
	sinTheta := float32(math.Sqrt(float64(max(0, 1-cosTheta*cosTheta))))
	return environmentToWorld(n, mgl32.Vec3{sinTheta * float32(math.Cos(phi)), sinTheta * float32(math.Sin(phi)), cosTheta})
}

func integrateEnvironmentBRDF(nv, roughness float32, samples int) mgl32.Vec2 {
	v := mgl32.Vec3{float32(math.Sqrt(float64(max(0, 1-nv*nv)))), 0, nv}
	n := mgl32.Vec3{0, 0, 1}
	var result mgl32.Vec2
	a := roughness * roughness
	a2 := a * a
	for i := 0; i < samples; i++ {
		h := environmentGGX(n, environmentSample(i, samples), roughness)
		vh := max(v.Dot(h), 0)
		l := h.Mul(2 * vh).Sub(v)
		nl := max(l.Z(), 0)
		nh := max(h.Z(), 0.0001)
		if nl <= 0 {
			continue
		}
		gv := nl * float32(math.Sqrt(float64(nv*nv*(1-a2)+a2)))
		gl := nv * float32(math.Sqrt(float64(nl*nl*(1-a2)+a2)))
		weight := 2 * nl * vh / max((gv+gl)*nh, 0.0001)
		f := float32(math.Pow(float64(1-vh), 5))
		result[0] += (1 - f) * weight
		result[1] += f * weight
	}
	return result.Mul(1 / float32(samples))
}

func (rgbmEnvironmentEncoder) encode(dst []byte, value mgl32.Vec3) {
	maximum := max(value.X(), value.Y(), value.Z())
	m := float32(math.Ceil(float64(max(1.0/255, min(1, maximum/16))*255))) / 255
	for i := range 3 {
		dst[i] = byte(max(0, min(1, value[i]/(m*16)))*255 + 0.5)
	}
	dst[3] = byte(m*255 + 0.5)
}

func (rgbeEnvironmentEncoder) encode(dst []byte, value mgl32.Vec3) {
	maximum := max(value.X(), value.Y(), value.Z())
	if maximum <= 0 {
		clear(dst[:4])
		return
	}
	exponent := max(-120, min(120, int(math.Ceil(math.Log2(float64(maximum))))))
	scale := float32(math.Ldexp(255, -exponent))
	for i := range 3 {
		dst[i] = byte(max(0, min(255, value[i]*scale)) + .5)
	}
	dst[3] = byte(exponent + 128)
}

// The sampler is chosen once through generics, outside the convolution loops.
type environmentRadianceSampler[S RadianceSource] interface {
	sample(S, mgl32.Vec3, float32) mgl32.Vec3
	diffuse(S, mgl32.Vec3) (mgl32.Vec3, bool)
}
type directEnvironmentSampler[S RadianceSource] struct{}

func (directEnvironmentSampler[S]) sample(source S, n mgl32.Vec3, _ float32) mgl32.Vec3 {
	return source.Radiance(n)
}

type environmentConvolutionSample struct {
	direction          mgl32.Vec3
	weight, solidAngle float32
}

func prepareEnvironmentConvolution(dst []environmentConvolutionSample, level int) {
	n := mgl32.Vec3{0, 0, 1}
	roughness := float32(level) / 5
	alphaSquared := roughness * roughness * roughness * roughness
	for i := range dst {
		xi := environmentSample(i, len(dst))
		var l mgl32.Vec3
		weight, pdf := float32(1), float32(0)
		if level == 6 {
			r := float32(math.Sqrt(float64(xi[0])))
			phi := 2 * math.Pi * float64(xi[1])
			l = mgl32.Vec3{r * float32(math.Cos(phi)), r * float32(math.Sin(phi)), float32(math.Sqrt(float64(1 - xi[0])))}
			pdf = l.Z() / math.Pi
		} else {
			h := environmentGGX(n, xi, roughness)
			l = h.Mul(2 * h.Z()).Sub(n)
			weight = max(l.Z(), 0)
			denominator := h.Z()*h.Z()*(alphaSquared-1) + 1
			pdf = alphaSquared / (4 * math.Pi * denominator * denominator)
		}
		dst[i] = environmentConvolutionSample{l, weight, 1 / (float32(len(dst)) * max(pdf, 1e-8))}
	}
}

func (directEnvironmentSampler[S]) diffuse(S, mgl32.Vec3) (mgl32.Vec3, bool) {
	return mgl32.Vec3{}, false
}
