package scene

import "math"

const (
	AnimationEaseInSine       AnimationTween = "easeInSine"
	AnimationEaseOutSine      AnimationTween = "easeOutSine"
	AnimationEaseInOutSine    AnimationTween = "easeInOutSine"
	AnimationEaseInCubic      AnimationTween = "easeInCubic"
	AnimationEaseOutCubic     AnimationTween = "easeOutCubic"
	AnimationEaseInOutCubic   AnimationTween = "easeInOutCubic"
	AnimationEaseInQuart      AnimationTween = "easeInQuart"
	AnimationEaseOutQuart     AnimationTween = "easeOutQuart"
	AnimationEaseInOutQuart   AnimationTween = "easeInOutQuart"
	AnimationEaseInQuint      AnimationTween = "easeInQuint"
	AnimationEaseOutQuint     AnimationTween = "easeOutQuint"
	AnimationEaseInOutQuint   AnimationTween = "easeInOutQuint"
	AnimationEaseInExpo       AnimationTween = "easeInExpo"
	AnimationEaseOutExpo      AnimationTween = "easeOutExpo"
	AnimationEaseInOutExpo    AnimationTween = "easeInOutExpo"
	AnimationEaseInCirc       AnimationTween = "easeInCirc"
	AnimationEaseOutCirc      AnimationTween = "easeOutCirc"
	AnimationEaseInOutCirc    AnimationTween = "easeInOutCirc"
	AnimationEaseInBack       AnimationTween = "easeInBack"
	AnimationEaseOutBack      AnimationTween = "easeOutBack"
	AnimationEaseInOutBack    AnimationTween = "easeInOutBack"
	AnimationEaseInElastic    AnimationTween = "easeInElastic"
	AnimationEaseOutElastic   AnimationTween = "easeOutElastic"
	AnimationEaseInOutElastic AnimationTween = "easeInOutElastic"
	AnimationEaseInBounce     AnimationTween = "easeInBounce"
	AnimationEaseOutBounce    AnimationTween = "easeOutBounce"
	AnimationEaseInOutBounce  AnimationTween = "easeInOutBounce"
)

type animationEasingFamily uint8
type animationEasingDirection uint8

const (
	animationEasingNone animationEasingFamily = iota
	animationEasingQuad
	animationEasingSine
	animationEasingCubic
	animationEasingQuart
	animationEasingQuint
	animationEasingExpo
	animationEasingCirc
	animationEasingBack
	animationEasingElastic
	animationEasingBounce
)

const (
	animationEasingIn animationEasingDirection = iota
	animationEasingOut
	animationEasingInOut
)

func animationEasing(tween AnimationTween) (animationEasingFamily, animationEasingDirection) {
	switch tween {
	case AnimationEaseIn:
		return animationEasingQuad, animationEasingIn
	case AnimationEaseOut:
		return animationEasingQuad, animationEasingOut
	case AnimationEaseInOut:
		return animationEasingQuad, animationEasingInOut
	case AnimationEaseInSine:
		return animationEasingSine, animationEasingIn
	case AnimationEaseOutSine:
		return animationEasingSine, animationEasingOut
	case AnimationEaseInOutSine:
		return animationEasingSine, animationEasingInOut
	case AnimationEaseInCubic:
		return animationEasingCubic, animationEasingIn
	case AnimationEaseOutCubic:
		return animationEasingCubic, animationEasingOut
	case AnimationEaseInOutCubic:
		return animationEasingCubic, animationEasingInOut
	case AnimationEaseInQuart:
		return animationEasingQuart, animationEasingIn
	case AnimationEaseOutQuart:
		return animationEasingQuart, animationEasingOut
	case AnimationEaseInOutQuart:
		return animationEasingQuart, animationEasingInOut
	case AnimationEaseInQuint:
		return animationEasingQuint, animationEasingIn
	case AnimationEaseOutQuint:
		return animationEasingQuint, animationEasingOut
	case AnimationEaseInOutQuint:
		return animationEasingQuint, animationEasingInOut
	case AnimationEaseInExpo:
		return animationEasingExpo, animationEasingIn
	case AnimationEaseOutExpo:
		return animationEasingExpo, animationEasingOut
	case AnimationEaseInOutExpo:
		return animationEasingExpo, animationEasingInOut
	case AnimationEaseInCirc:
		return animationEasingCirc, animationEasingIn
	case AnimationEaseOutCirc:
		return animationEasingCirc, animationEasingOut
	case AnimationEaseInOutCirc:
		return animationEasingCirc, animationEasingInOut
	case AnimationEaseInBack:
		return animationEasingBack, animationEasingIn
	case AnimationEaseOutBack:
		return animationEasingBack, animationEasingOut
	case AnimationEaseInOutBack:
		return animationEasingBack, animationEasingInOut
	case AnimationEaseInElastic:
		return animationEasingElastic, animationEasingIn
	case AnimationEaseOutElastic:
		return animationEasingElastic, animationEasingOut
	case AnimationEaseInOutElastic:
		return animationEasingElastic, animationEasingInOut
	case AnimationEaseInBounce:
		return animationEasingBounce, animationEasingIn
	case AnimationEaseOutBounce:
		return animationEasingBounce, animationEasingOut
	case AnimationEaseInOutBounce:
		return animationEasingBounce, animationEasingInOut
	default:
		return animationEasingNone, animationEasingIn
	}
}

func sampleAnimationEasing(family animationEasingFamily, direction animationEasingDirection, progress float32) float32 {
	if progress == 0 || progress == 1 {
		return progress
	}
	switch direction {
	case animationEasingOut:
		return 1 - sampleAnimationEasingIn(family, direction, 1-progress)
	case animationEasingInOut:
		if progress < .5 {
			return sampleAnimationEasingIn(family, direction, 2*progress) * .5
		}
		return 1 - sampleAnimationEasingIn(family, direction, 2-2*progress)*.5
	default:
		return sampleAnimationEasingIn(family, direction, progress)
	}
}

func sampleAnimationEasingIn(family animationEasingFamily, direction animationEasingDirection, progress float32) float32 {
	switch family {
	case animationEasingQuad:
		return progress * progress
	case animationEasingSine:
		return float32(1 - math.Cos(float64(progress)*math.Pi*.5))
	case animationEasingCubic:
		return progress * progress * progress
	case animationEasingQuart:
		squared := progress * progress
		return squared * squared
	case animationEasingQuint:
		squared := progress * progress
		return squared * squared * progress
	case animationEasingExpo:
		return float32(math.Exp2(10*float64(progress) - 10))
	case animationEasingCirc:
		return float32(1 - math.Sqrt(max(1-float64(progress)*float64(progress), 0)))
	case animationEasingBack:
		overshoot := float32(1.70158)
		if direction == animationEasingInOut {
			overshoot *= 1.525
		}
		return progress * progress * ((overshoot+1)*progress - overshoot)
	case animationEasingElastic:
		period, phase := float64(3), float64(10.75)
		if direction == animationEasingInOut {
			period, phase = 4.5, 11.125
		}
		x := float64(progress)
		return float32(-math.Exp2(10*x-10) * math.Sin((10*x-phase)*2*math.Pi/period))
	case animationEasingBounce:
		return 1 - sampleAnimationBounceOut(1-progress)
	default:
		return progress
	}
}

func sampleAnimationBounceOut(progress float32) float32 {
	const scale, period = float32(7.5625), float32(2.75)
	switch {
	case progress < 1/period:
		return scale * progress * progress
	case progress < 2/period:
		progress -= 1.5 / period
		return scale*progress*progress + .75
	case progress < 2.5/period:
		progress -= 2.25 / period
		return scale*progress*progress + .9375
	default:
		progress -= 2.625 / period
		return scale*progress*progress + .984375
	}
}
