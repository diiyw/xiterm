package graphics

import (
	"fmt"
	"math"
)

// LightUnit follows Unity HDRP's photometric input conventions. Distances and
// light dimensions are meters; shader RGB values are linear photometric values.
type LightUnit string

const (
	LightLumen   LightUnit = "lumen"
	LightCandela LightUnit = "candela"
	LightLux     LightUnit = "lux"
	LightNits    LightUnit = "nits"
	LightEV100   LightUnit = "ev100"
)

// ValidateLightIntensity validates a value before converting it. EV100 is
// logarithmic: negative values are valid and zero EV does not turn a light off.
func ValidateLightIntensity(value float32, unit LightUnit, units ...LightUnit) error {
	if !finitePhotometric(value) || unit != LightEV100 && value < 0 || unit == LightEV100 && (value < -32 || value > 64) {
		return fmt.Errorf("invalid light intensity %g %s", value, unit)
	}
	for _, allowed := range units {
		if unit == allowed {
			return nil
		}
	}
	return fmt.Errorf("unsupported light unit %q", unit)
}

// PunctualCandela converts point/cone/pyramid inputs to luminous intensity.
// solidAngle is 4*pi for a point light or a spotlight without a reflector.
func PunctualCandela(value float32, unit LightUnit, solidAngle, luxDistance float32) float32 {
	switch unit {
	case LightLumen:
		return value / solidAngle
	case LightCandela:
		return value
	case LightLux:
		return value * luxDistance * luxDistance
	case LightEV100:
		return float32(math.Exp2(float64(value))) * 0.125
	default:
		return 0
	}
}

// AreaNits uses HDRP's projected area convention: pi*width*height for a
// rectangle, pi*pi*radius*radius for a disc, and 4*pi*length for a tube.
func AreaNits(value float32, unit LightUnit, projectedArea float32) float32 {
	switch unit {
	case LightLumen:
		return value / projectedArea
	case LightNits:
		return value
	case LightEV100:
		return float32(math.Exp2(float64(value))) * 0.125
	default:
		return 0
	}
}

// IntensityFromPhysical is the inverse conversion used when changing units in
// an inspector. The caller supplies the light's solid angle or projected area.
func IntensityFromPhysical(value float32, unit LightUnit, measure, luxDistance float32) float32 {
	switch unit {
	case LightLumen:
		return value * measure
	case LightLux:
		return value / (luxDistance * luxDistance)
	case LightEV100:
		return float32(math.Log2(float64(value) * 8))
	default:
		return value
	}
}

type ExposureMode string

const (
	ExposureFixed    ExposureMode = "fixed"
	ExposurePhysical ExposureMode = "physical"
)

// Exposure applies to the entire lit scene, including emission, sky and IBL.
// The zero value uses fixed EV100 13. Explicit fixed mode permits EV100 zero.
type Exposure struct {
	Mode         ExposureMode `json:"mode,omitempty"`
	EV100        float32      `json:"ev100,omitempty"`
	Compensation float32      `json:"compensation,omitempty"`
	Aperture     float32      `json:"aperture,omitempty"`
	ShutterSpeed float32      `json:"shutterSpeed,omitempty"` // seconds
	ISO          float32      `json:"iso,omitempty"`
}

func NewExposure() Exposure {
	return Exposure{Mode: ExposureFixed, EV100: 13, Aperture: 16, ShutterSpeed: 1.0 / 125, ISO: 100}
}

func (e Exposure) Validate() error {
	if e.Mode != "" && e.Mode != ExposureFixed && e.Mode != ExposurePhysical {
		return fmt.Errorf("unknown exposure mode %q", e.Mode)
	}
	for _, v := range [...]float32{e.EV100, e.Compensation, e.Aperture, e.ShutterSpeed, e.ISO} {
		if !finitePhotometric(v) {
			return fmt.Errorf("exposure values must be finite")
		}
	}
	if e.Mode == ExposurePhysical && (e.Aperture <= 0 || e.ShutterSpeed <= 0 || e.ISO <= 0) {
		return fmt.Errorf("physical exposure requires positive aperture, shutter speed and ISO")
	}
	ev := e.Value() - e.Compensation
	if ev < -32 || ev > 64 {
		return fmt.Errorf("effective exposure must be between -32 and 64 EV100")
	}
	return nil
}

func (e Exposure) Value() float32 {
	if e.Mode == "" {
		return 13
	}
	if e.Mode == ExposurePhysical {
		return float32(math.Log2(float64(e.Aperture) * float64(e.Aperture) / float64(e.ShutterSpeed) * 100 / float64(e.ISO)))
	}
	return e.EV100
}

// Multiplier uses HDRP's standard lens calibration (1 / (1.2 * 2^EV100)).
func (e Exposure) Multiplier() float32 {
	return float32(math.Exp2(float64(e.Compensation-e.Value())) / 1.2)
}

func finitePhotometric(v float32) bool { return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0) }
