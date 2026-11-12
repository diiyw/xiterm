package scene

import "fmt"

// ValidateAnimationTimeCurve checks a track's normalized playback time mapping.
// Spline interpolation needs value tangents and cannot be used as a time curve.
func ValidateAnimationTimeCurve(tween AnimationTween, curve AnimationCurve) error {
	if tween != "" && (!tween.Valid() || tween == AnimationCubicSpline) {
		return fmt.Errorf("animation time curve has unsupported tween %q", tween)
	}
	if tween == AnimationBezier {
		return curve.Validate()
	}
	return nil
}

func (track *AnimationTrack[T]) sampleTime(time float32) float32 {
	keys := track.Keyframes
	if track.TimeTween == "" || track.TimeTween == AnimationLinear || len(keys) < 2 {
		return time
	}
	start, end := keys[0].Time, keys[len(keys)-1].Time
	if time <= start || time >= end || end <= start {
		return time
	}
	progress := sampleAnimationProgress(track.TimeTween, track.TimeCurve, (time-start)/(end-start))
	return start + (end-start)*min(max(progress, 0), 1)
}
