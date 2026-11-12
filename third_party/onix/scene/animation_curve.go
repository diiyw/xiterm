package scene

import (
	"fmt"
	"math"
)

// AnimationCurve maps normalized time to normalized animation progress using
// a cubic Bezier curve with fixed endpoints (0, 0) and (1, 1).
type AnimationCurve struct {
	X1 float32 `json:"x1"`
	Y1 float32 `json:"y1"`
	X2 float32 `json:"x2"`
	Y2 float32 `json:"y2"`
}

func LinearAnimationCurve() AnimationCurve {
	return AnimationCurve{X2: 1, Y2: 1}
}

func EaseInAnimationCurve() AnimationCurve {
	return AnimationCurve{X1: .42, Y1: 0, X2: 1, Y2: 1}
}

func EaseOutAnimationCurve() AnimationCurve {
	return AnimationCurve{X1: 0, Y1: 0, X2: .58, Y2: 1}
}

func EaseInOutAnimationCurve() AnimationCurve {
	return AnimationCurve{X1: .42, Y1: 0, X2: .58, Y2: 1}
}

func isFiniteAnimationValue[T ~float32 | ~float64](value T) bool {
	converted := float64(value)
	return !math.IsNaN(converted) && !math.IsInf(converted, 0)
}

func (c AnimationCurve) Validate() error {
	for _, value := range [...]float32{c.X1, c.Y1, c.X2, c.Y2} {
		if !isFiniteAnimationValue(value) {
			return fmt.Errorf("animation curve control points must be finite")
		}
	}
	if c.X1 < 0 || c.X1 > 1 || c.X2 < 0 || c.X2 > 1 {
		return fmt.Errorf("animation curve x control points must be between 0 and 1")
	}
	return nil
}

func (c AnimationCurve) Sample(progress float32) float32 {
	progress = min(max(progress, float32(0)), float32(1))
	if c.X1 == c.Y1 && c.X2 == c.Y2 {
		return progress
	}
	if progress == 0 || progress == 1 {
		return progress
	}
	low, high := float32(0), float32(1)
	for range 16 {
		parameter := (low + high) * 0.5
		if cubicBezier(parameter, c.X1, c.X2) < progress {
			low = parameter
		} else {
			high = parameter
		}
	}
	return cubicBezier((low+high)*0.5, c.Y1, c.Y2)
}

func cubicBezier(parameter, control1, control2 float32) float32 {
	inverse := 1 - parameter
	return 3*inverse*inverse*parameter*control1 + 3*inverse*parameter*parameter*control2 + parameter*parameter*parameter
}
