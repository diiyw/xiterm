package scene

import (
	"fmt"
	"math"
	"slices"

	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/graphics"
	"github.com/go-gl/mathgl/mgl32"
)

// AnimationTween controls the curve applied from one keyframe to the next.
type AnimationTween string

const (
	AnimationStep        AnimationTween = "step"
	AnimationLinear      AnimationTween = "linear"
	AnimationEaseIn      AnimationTween = "easeIn"
	AnimationEaseOut     AnimationTween = "easeOut"
	AnimationEaseInOut   AnimationTween = "easeInOut"
	AnimationBezier      AnimationTween = "bezier"
	AnimationCubicSpline AnimationTween = "cubicSpline"
)

// Valid reports whether the tween is supported by the runtime sampler.
func (t AnimationTween) Valid() bool {
	switch t {
	case AnimationStep, AnimationLinear, AnimationBezier, AnimationCubicSpline:
		return true
	default:
		family, _ := animationEasing(t)
		return family != animationEasingNone
	}
}

type AnimationTarget string

const (
	AnimationTargetSpriteTexture      AnimationTarget = "sprite.texture"
	AnimationTargetSpriteAlpha        AnimationTarget = "sprite.alpha"
	AnimationTargetTransformPositionX AnimationTarget = "transform.position.x"
	AnimationTargetTransformPositionY AnimationTarget = "transform.position.y"
	AnimationTargetTransformPositionZ AnimationTarget = "transform.position.z"
	AnimationTargetTransformScaleX    AnimationTarget = "transform.scale.x"
	AnimationTargetTransformScaleY    AnimationTarget = "transform.scale.y"
	AnimationTargetTransformScaleZ    AnimationTarget = "transform.scale.z"
	AnimationTargetTransformRotationX AnimationTarget = "transform.rotation.x"
	AnimationTargetTransformRotationY AnimationTarget = "transform.rotation.y"
	AnimationTargetTransformRotationZ AnimationTarget = "transform.rotation.z"
	AnimationTargetVisibility         AnimationTarget = "visibility.visible"
)

type animationTargetCode uint8

const (
	animationTargetInvalid animationTargetCode = iota
	animationTargetSpriteTexture
	animationTargetSpriteAlpha
	animationTargetPositionX
	animationTargetPositionY
	animationTargetPositionZ
	animationTargetScaleX
	animationTargetScaleY
	animationTargetScaleZ
	animationTargetRotationX
	animationTargetRotationY
	animationTargetRotationZ
	animationTargetVisible
)

func compileAnimationTarget(target AnimationTarget) animationTargetCode {
	switch target {
	case AnimationTargetSpriteTexture:
		return animationTargetSpriteTexture
	case AnimationTargetSpriteAlpha:
		return animationTargetSpriteAlpha
	case AnimationTargetTransformPositionX:
		return animationTargetPositionX
	case AnimationTargetTransformPositionY:
		return animationTargetPositionY
	case AnimationTargetTransformPositionZ:
		return animationTargetPositionZ
	case AnimationTargetTransformScaleX:
		return animationTargetScaleX
	case AnimationTargetTransformScaleY:
		return animationTargetScaleY
	case AnimationTargetTransformScaleZ:
		return animationTargetScaleZ
	case AnimationTargetTransformRotationX:
		return animationTargetRotationX
	case AnimationTargetTransformRotationY:
		return animationTargetRotationY
	case AnimationTargetTransformRotationZ:
		return animationTargetRotationZ
	case AnimationTargetVisibility:
		return animationTargetVisible
	default:
		return animationTargetInvalid
	}
}

type AnimationBinding struct {
	Path   string          `json:"path,omitempty"`
	Target AnimationTarget `json:"target"`
}

// AnimationQuaternionSpline preserves glTF quaternion cubic data while the
// public transform bindings continue to target individual Euler components.
// Component is one-based; zero disables quaternion sampling for the key.
type AnimationQuaternionSpline struct {
	Value      [4]float32
	InTangent  [4]float32
	OutTangent [4]float32
	Component  uint8
}

type AnimationKeyframe[T any] struct {
	Value            T
	InTangent        T
	OutTangent       T
	Tween            AnimationTween
	QuaternionSpline AnimationQuaternionSpline
	Curve            AnimationCurve
	Time             float32
	InverseSpan      float32
}

type AnimationTrack[T any] struct {
	Binding AnimationBinding
	// TimeTween remaps elapsed time over the first-to-last keyframe interval.
	// Discrete tracks still select frames; the curve changes their playback pace.
	TimeTween AnimationTween
	Keyframes []AnimationKeyframe[T]
	TimeCurve AnimationCurve
	target    animationTargetCode
	root      bool
}

func NewAnimationTrack[T any](binding AnimationBinding, keyframes []AnimationKeyframe[T]) AnimationTrack[T] {
	return AnimationTrack[T]{Binding: binding, Keyframes: slices.Clone(keyframes)}
}

func (track AnimationTrack[T]) Sample(time float32, interpolate func(T, T, float32) T) (T, bool) {
	var zero T
	keys := track.Keyframes
	if len(keys) == 0 || !isFiniteAnimationValue(time) {
		return zero, false
	}
	time = track.sampleTime(time)
	if time <= keys[0].Time || len(keys) == 1 {
		return keys[0].Value, true
	}
	if time >= keys[len(keys)-1].Time {
		return keys[len(keys)-1].Value, true
	}

	next := nextAnimationKeyframe(keys, time)
	previous := keys[next-1]
	tween := previous.Tween
	if tween == AnimationStep || interpolate == nil {
		return previous.Value, true
	}
	span := keys[next].Time - previous.Time
	if span <= 0 {
		return keys[next].Value, true
	}
	progress := min(max((time-previous.Time)/span, float32(0)), float32(1))
	if tween == AnimationCubicSpline {
		if value, ok := previous.sampleQuaternionSpline(keys[next], span, progress); ok {
			return value, true
		}
		if value, ok := previous.sampleCubicValue(keys[next], span, progress); ok {
			return value, true
		}
	}
	progress = sampleAnimationProgress(tween, previous.Curve, progress)
	return interpolate(previous.Value, keys[next].Value, progress), true
}

func nextAnimationKeyframe[T any](keys []AnimationKeyframe[T], time float32) int {
	low, high := 0, len(keys)
	for low < high {
		middle := int(uint(low+high) >> 1)
		if keys[middle].Time > time {
			high = middle
		} else {
			low = middle + 1
		}
	}
	return low
}

func sampleAnimationFloatTrackAt(track *AnimationTrack[float32], time float32, cursor *int) (float32, bool) {
	if track == nil || len(track.Keyframes) == 0 {
		return 0, false
	}
	time = track.sampleTime(time)
	keys := track.Keyframes
	if time <= keys[0].Time || len(keys) == 1 {
		*cursor = 0
		return keys[0].Value, true
	}
	if time >= keys[len(keys)-1].Time {
		*cursor = len(keys) - 1
		return keys[len(keys)-1].Value, true
	}
	index := *cursor
	if index < 0 || index >= len(keys) || time < keys[index].Time {
		index = nextAnimationKeyframe(keys, time) - 1
	} else {
		for index+1 < len(keys) && time >= keys[index+1].Time {
			index++
		}
	}
	*cursor = index
	previous := keys[index]
	if previous.Tween == AnimationStep {
		return previous.Value, true
	}
	inverseSpan := previous.InverseSpan
	if inverseSpan <= 0 {
		span := keys[index+1].Time - previous.Time
		if span <= 0 {
			return keys[index+1].Value, true
		}
		inverseSpan = 1 / span
	}
	progress := min(max((time-previous.Time)*inverseSpan, float32(0)), float32(1))
	if previous.Tween == AnimationCubicSpline {
		span := float32(1) / inverseSpan
		if value, ok := previous.sampleQuaternionSpline(keys[index+1], span, progress); ok {
			return any(value).(float32), true
		}
		return cubicHermiteFloat32(previous.Value, keys[index+1].Value, previous.OutTangent*span, keys[index+1].InTangent*span, progress), true
	}
	if previous.Tween != "" && previous.Tween != AnimationLinear {
		progress = sampleAnimationProgress(previous.Tween, previous.Curve, progress)
	}
	return previous.Value + (keys[index+1].Value-previous.Value)*progress, true
}

func (left AnimationKeyframe[T]) sampleQuaternionSpline(right AnimationKeyframe[T], span, progress float32) (T, bool) {
	component := left.QuaternionSpline.Component
	if component == 0 || component > 3 || right.QuaternionSpline.Component != component {
		var zero T
		return zero, false
	}
	leftValue, ok := any(left.Value).(float32)
	if !ok {
		var zero T
		return zero, false
	}
	value := [4]float32{}
	for index := range value {
		value[index] = cubicHermiteFloat32(
			left.QuaternionSpline.Value[index], right.QuaternionSpline.Value[index],
			left.QuaternionSpline.OutTangent[index]*span, right.QuaternionSpline.InTangent[index]*span,
			progress,
		)
	}
	length := float32(math.Sqrt(float64(value[0]*value[0] + value[1]*value[1] + value[2]*value[2] + value[3]*value[3])))
	if length <= 1e-8 {
		value = [4]float32{0, 0, 0, 1}
	} else {
		for index := range value {
			value[index] /= length
		}
	}
	euler := animationQuaternionEuler(value)
	result := unwrapAnimationAngle(leftValue, euler[component-1])
	return any(result).(T), true
}

func animationQuaternionEuler(value [4]float32) [3]float32 {
	x, y, z, w := float64(value[0]), float64(value[1]), float64(value[2]), float64(value[3])
	siny := min(max(2*(w*y-z*x), float64(-1)), float64(1))
	return [3]float32{
		float32(math.Atan2(2*(w*x+y*z), 1-2*(x*x+y*y))),
		float32(math.Asin(siny)),
		float32(math.Atan2(2*(w*z+x*y), 1-2*(y*y+z*z))),
	}
}

func unwrapAnimationAngle(reference, value float32) float32 {
	delta := value - reference
	for delta > math.Pi {
		value -= 2 * math.Pi
		delta -= 2 * math.Pi
	}
	for delta < -math.Pi {
		value += 2 * math.Pi
		delta += 2 * math.Pi
	}
	return value
}

func (left AnimationKeyframe[T]) sampleCubicValue(right AnimationKeyframe[T], span, progress float32) (T, bool) {
	if value, ok := any(left.Value).(float32); ok {
		result := cubicHermiteFloat32(value, any(right.Value).(float32), any(left.OutTangent).(float32)*span, any(right.InTangent).(float32)*span, progress)
		return any(result).(T), true
	}
	var zero T
	return zero, false
}

func cubicHermiteFloat32(left, right, outTangent, inTangent, progress float32) float32 {
	t2 := progress * progress
	t3 := t2 * progress
	return (2*t3-3*t2+1)*left + (t3-2*t2+progress)*outTangent + (-2*t3+3*t2)*right + (t3-t2)*inTangent
}

func (track *AnimationTrack[T]) sampleDiscreteAt(time float32, cursor *int) (T, bool) {
	var zero T
	if track == nil || len(track.Keyframes) == 0 {
		return zero, false
	}
	time = track.sampleTime(time)
	keys := track.Keyframes
	if time <= keys[0].Time || len(keys) == 1 {
		*cursor = 0
		return keys[0].Value, true
	}
	if time >= keys[len(keys)-1].Time {
		*cursor = len(keys) - 1
		return keys[len(keys)-1].Value, true
	}
	index := *cursor
	if index < 0 || index >= len(keys) || time < keys[index].Time {
		index = nextAnimationKeyframe(keys, time) - 1
	} else {
		for index+1 < len(keys) && time >= keys[index+1].Time {
			index++
		}
	}
	*cursor = index
	return keys[index].Value, true
}

// SampleAnimationProgress applies a keyframe tween to normalized progress.
// Time is clamped to [0, 1]; Back, Elastic and Bezier may overshoot in value.
func SampleAnimationProgress(tween AnimationTween, curve AnimationCurve, progress float32) float32 {
	progress = min(max(progress, float32(0)), float32(1))
	return sampleAnimationProgress(tween, curve, progress)
}

func sampleAnimationProgress(tween AnimationTween, curve AnimationCurve, progress float32) float32 {
	switch tween {
	case AnimationStep:
		if progress < 1 {
			return 0
		}
		return 1
	case AnimationBezier:
		return curve.Sample(progress)
	default:
		family, direction := animationEasing(tween)
		return sampleAnimationEasing(family, direction, progress)
	}
}

type AnimationClip struct {
	prepareErr            error
	BoolTracks            []AnimationTrack[bool]
	TextureTracks         []AnimationTrack[*graphics.Texture]
	Events                []AnimationEvent
	FloatTracks           []AnimationTrack[float32]
	rootFloatTracks       []int
	rootTextureTracks     []int
	rootBoolTracks        []int
	Duration              float32
	prepared              bool
	rootRequirements      uint8
	hasRotationTracks     bool
	hasRootRotationTracks bool
}

// RequiresSprite2D reports whether root tracks require a Sprite2D target.
func (c *AnimationClip) RequiresSprite2D() (bool, error) {
	if c == nil {
		return false, nil
	}
	if err := c.prepare(); err != nil {
		return false, err
	}
	return c.rootRequirements&animationNeedsSprite != 0, nil
}

// Prepare validates and freezes the clip's compiled track metadata.  Project
// loaders call it once before instances share the clip, keeping playback free
// of setup allocations.
func (c *AnimationClip) Prepare() error {
	return c.prepare()
}

const (
	animationNeedsSprite uint8 = 1 << iota
	animationNeedsTransform
	animationNeedsVisibility
)

func (c *AnimationClip) prepare() error {
	if c == nil || c.prepared {
		if c == nil {
			return nil
		}
		return c.prepareErr
	}
	c.prepared = true
	floatRoots, floatRequirements, err := prepareAnimationTracks(c.FloatTracks, floatAnimationTargetCode)
	if err != nil {
		c.prepareErr = err
		return err
	}
	for trackIndex := range c.FloatTracks {
		for keyIndex, key := range c.FloatTracks[trackIndex].Keyframes {
			if key.Tween == AnimationCubicSpline && (!isFiniteAnimationValue(key.InTangent) || !isFiniteAnimationValue(key.OutTangent)) {
				c.prepareErr = fmt.Errorf("ecs: float animation track %d keyframe %d has non-finite cubic tangents", trackIndex, keyIndex)
				return c.prepareErr
			}
			if key.QuaternionSpline.Component > 3 {
				c.prepareErr = fmt.Errorf("ecs: float animation track %d keyframe %d has invalid quaternion component", trackIndex, keyIndex)
				return c.prepareErr
			}
			if key.QuaternionSpline.Component != 0 {
				for component := range 4 {
					if !isFiniteAnimationValue(key.QuaternionSpline.Value[component]) || !isFiniteAnimationValue(key.QuaternionSpline.InTangent[component]) || !isFiniteAnimationValue(key.QuaternionSpline.OutTangent[component]) {
						c.prepareErr = fmt.Errorf("ecs: float animation track %d keyframe %d quaternion spline is not finite", trackIndex, keyIndex)
						return c.prepareErr
					}
				}
			}
		}
	}
	textureRoots, textureRequirements, err := prepareAnimationTracks(c.TextureTracks, textureAnimationTargetCode)
	if err != nil {
		c.prepareErr = err
		return err
	}
	boolRoots, boolRequirements, err := prepareAnimationTracks(c.BoolTracks, boolAnimationTargetCode)
	if err != nil {
		c.prepareErr = err
		return err
	}
	c.rootFloatTracks = floatRoots
	c.rootTextureTracks = textureRoots
	c.rootBoolTracks = boolRoots
	c.rootRequirements = floatRequirements | textureRequirements | boolRequirements
	for index := range c.FloatTracks {
		track := &c.FloatTracks[index]
		if !isAnimationRotationTarget(track.target) {
			continue
		}
		c.hasRotationTracks = true
		c.hasRootRotationTracks = c.hasRootRotationTracks || track.root
	}
	return nil
}

func prepareAnimationTracks[T any](tracks []AnimationTrack[T], allowed func(animationTargetCode) bool) ([]int, uint8, error) {
	rootCount := 0
	var requirements uint8
	for index := range tracks {
		track := &tracks[index]
		if err := track.prepare(allowed); err != nil {
			return nil, 0, err
		}
		if track.root {
			rootCount++
			requirements |= animationRequirement(track.target)
		}
	}
	if rootCount == 0 {
		return nil, requirements, nil
	}
	roots := make([]int, 0, rootCount)
	for index := range tracks {
		if tracks[index].root {
			roots = append(roots, index)
		}
	}
	return roots, requirements, nil
}

func textureAnimationTargetCode(target animationTargetCode) bool {
	return target == animationTargetSpriteTexture
}

func boolAnimationTargetCode(target animationTargetCode) bool {
	return target == animationTargetVisible
}

func (track *AnimationTrack[T]) prepare(allowed func(animationTargetCode) bool) error {
	target := compileAnimationTarget(track.Binding.Target)
	if !allowed(target) {
		return fmt.Errorf("ecs: unsupported animation target %q", track.Binding.Target)
	}
	if err := ValidateAnimationTimeCurve(track.TimeTween, track.TimeCurve); err != nil {
		return err
	}
	track.target = target
	track.root = track.Binding.Path == ""
	for index := 0; index+1 < len(track.Keyframes); index++ {
		span := track.Keyframes[index+1].Time - track.Keyframes[index].Time
		if span > 0 && track.Keyframes[index].InverseSpan <= 0 {
			track.Keyframes[index].InverseSpan = 1 / span
		}
	}
	return nil
}

func floatAnimationTargetCode(target animationTargetCode) bool {
	return target >= animationTargetSpriteAlpha && target <= animationTargetRotationZ
}

func animationRequirement(target animationTargetCode) uint8 {
	if target == animationTargetSpriteAlpha || target == animationTargetSpriteTexture {
		return animationNeedsSprite
	}
	if target >= animationTargetPositionX && target <= animationTargetRotationZ {
		return animationNeedsTransform
	}
	if target == animationTargetVisible {
		return animationNeedsVisibility
	}
	return 0
}

type AnimationEvent struct {
	Name  string
	Value string
	Time  float32
}

type AnimationEventMessage struct {
	Name   string
	Value  string
	Entity ecs.Entity
}

type Animator struct {
	cursorClip         *AnimationClip
	Clip               *AnimationClip
	rotationGroupNext  []int
	rotationGroupFirst []int
	// Rotation baselines preserve authored axes when a clip animates only a
	// subset of X/Y/Z. They are initialized once per bound target and reused
	// by every subsequent sample.
	rotationBaselines []animationRotationValues
	// Bound targets share one backing slice. Track counts recover the three
	// typed views without retaining three independent allocations per animator.
	boundTargets       []ecs.Entity
	cursors            []int
	previousTime       float64
	floatTargetCount   int
	boolTargetCount    int
	textureTargetCount int
	loops              uint64
	// Bound target entities are compiled by the scene/prefab plan.  They are
	// indexed by the corresponding typed track and let playback touch real
	// nodes without resolving a path or consulting a map each frame.
	owner                 ecs.Entity
	time                  float64
	Speed                 float32
	emitStart             bool
	advanced              bool
	dirty                 bool
	Playing               bool
	Loop                  bool
	rotationBindingsReady bool
}

func NewAnimator(clip *AnimationClip, loop bool) Animator {
	animator := Animator{Clip: clip, Loop: loop, Playing: true, Speed: 1, emitStart: true, dirty: true}
	if clip != nil && clip.prepared && clip.prepareErr == nil {
		animator.cursorClip = clip
		animator.cursors = make([]int, animationClipTrackCount(clip))
	}
	return animator
}

func animationClipTrackCount(clip *AnimationClip) int {
	if clip == nil {
		return 0
	}
	return len(clip.FloatTracks) + len(clip.TextureTracks) + len(clip.BoolTracks)
}

// NewBoundAnimator creates one playback state for a shared clip and binds its
// typed tracks to concrete scene entities.  The target slices are copied once
// at instantiation; subsequent updates perform only ECS component access.
func NewBoundAnimator(clip *AnimationClip, loop bool, owner ecs.Entity, floatTargets, textureTargets, boolTargets []ecs.Entity) Animator {
	animator := NewAnimator(clip, loop)
	animator.owner = owner
	animator.setBoundTargets(floatTargets, textureTargets, boolTargets)
	if animator.IsBound() {
		animator.prepareRotationStorage()
	}
	return animator
}

// BindTargets attaches concrete entities to an existing animator.  It is
// useful for pooled entities whose clip is unchanged but hierarchy IDs are
// assigned only when the pool slot is activated.
func (a *Animator) BindTargets(owner ecs.Entity, floatTargets, textureTargets, boolTargets []ecs.Entity) {
	if a == nil {
		return
	}
	a.owner = owner
	a.setBoundTargets(floatTargets, textureTargets, boolTargets)
	a.resetRotationBaselines()
	if a.IsBound() {
		a.prepareRotationStorage()
	}
	a.dirty = true
}

func (a *Animator) prepareRotationStorage() {
	if a == nil || a.Clip == nil || !a.Clip.prepared || a.Clip.prepareErr != nil || !a.Clip.hasRotationTracks {
		return
	}
	trackCount := len(a.Clip.FloatTracks)
	a.rotationBaselines = reuseAnimationSlice(a.rotationBaselines, trackCount)
	a.rotationGroupFirst = reuseAnimationSlice(a.rotationGroupFirst, trackCount)
	a.rotationGroupNext = reuseAnimationSlice(a.rotationGroupNext, trackCount)
}

func (a *Animator) setBoundTargets(floatTargets, textureTargets, boolTargets []ecs.Entity) {
	if a == nil {
		return
	}
	a.floatTargetCount = len(floatTargets)
	a.textureTargetCount = len(textureTargets)
	a.boolTargetCount = len(boolTargets)
	total := a.floatTargetCount + a.textureTargetCount + a.boolTargetCount
	a.boundTargets = reuseAnimationSlice(a.boundTargets, total)
	next := copy(a.boundTargets, floatTargets)
	next += copy(a.boundTargets[next:], textureTargets)
	copy(a.boundTargets[next:], boolTargets)
}

func (a *Animator) boundTargetSlices() (floatTargets, textureTargets, boolTargets []ecs.Entity) {
	if a == nil {
		return nil, nil, nil
	}
	floatEnd := a.floatTargetCount
	textureEnd := floatEnd + a.textureTargetCount
	boolEnd := textureEnd + a.boolTargetCount
	return a.boundTargets[:floatEnd], a.boundTargets[floatEnd:textureEnd], a.boundTargets[textureEnd:boolEnd]
}

func (a *Animator) cursorSlices() (floatCursors, textureCursors, boolCursors []int) {
	if a == nil || a.cursorClip == nil {
		return nil, nil, nil
	}
	floatEnd := len(a.cursorClip.FloatTracks)
	textureEnd := floatEnd + len(a.cursorClip.TextureTracks)
	boolEnd := textureEnd + len(a.cursorClip.BoolTracks)
	return a.cursors[:floatEnd], a.cursors[floatEnd:textureEnd], a.cursors[textureEnd:boolEnd]
}

func reuseAnimationSlice[T any](dst []T, length int) []T {
	if cap(dst) < length {
		return make([]T, length)
	}
	dst = dst[:length]
	clear(dst)
	return dst
}

func (a *Animator) IsBound() bool {
	return a != nil && (a.owner != 0 || len(a.boundTargets) != 0)
}

func (a *Animator) Time() float32 {
	if a == nil {
		return 0
	}
	return float32(a.time)
}

func (a *Animator) Play() {
	if a != nil {
		a.Playing = true
	}
}

func (a *Animator) Pause() {
	if a != nil {
		a.Playing = false
	}
}

func (a *Animator) Restart() {
	if a == nil {
		return
	}
	a.time = 0
	a.previousTime = 0
	a.loops = 0
	a.advanced = false
	a.emitStart = true
	a.Playing = true
	a.dirty = true
	a.resetRotationBaselines()
}

func (a *Animator) Seek(time float32) {
	if a == nil || a.Clip == nil || !isFiniteAnimationValue(time) {
		return
	}
	a.time = float64(min(max(time, float32(0)), a.Clip.Duration))
	a.previousTime = a.time
	a.loops = 0
	a.advanced = false
	a.emitStart = false
	a.dirty = true
	a.resetRotationBaselines()
}

func (a *Animator) resetRotationBaselines() {
	if a == nil {
		return
	}
	clear(a.rotationBaselines)
	clear(a.rotationGroupFirst)
	clear(a.rotationGroupNext)
	a.rotationBindingsReady = false
}

func (a *Animator) Advance(delta float32) {
	if delta <= 0 || !isFiniteAnimationValue(delta) {
		a.resetAdvanceState()
		return
	}
	a.advanceValid(delta)
}

func (a *Animator) resetAdvanceState() {
	if a != nil {
		a.advanced = false
		a.loops = 0
	}
}

// advanceValid updates playback after the caller has validated the frame
// delta. The update systems use this form so the same delta is not checked for
// every animator in the world.
func (a *Animator) advanceValid(delta float32) {
	a.resetAdvanceState()
	if a == nil || a.Clip == nil || !a.Playing || a.Speed <= 0 || !isFiniteAnimationValue(a.Speed) || a.Clip.Duration <= 0 {
		return
	}
	duration := float64(a.Clip.Duration)
	a.previousTime = a.time
	next := a.time + float64(delta)*float64(a.Speed)
	if a.Loop {
		updated := next
		if next >= duration {
			a.loops = uint64(next / duration)
			updated = math.Mod(next, duration)
		} else {
			a.loops = 0
		}
		a.dirty = a.dirty || updated != a.time
		a.time = updated
		a.advanced = true
		return
	}
	updated := next
	if updated >= duration {
		updated = duration
		a.Playing = false
	}
	a.dirty = a.dirty || updated != a.time
	a.time = updated
	a.advanced = true
}

func (a *Animator) prepareCursors() error {
	if a == nil || a.Clip == nil {
		return nil
	}
	if a.cursorClip == a.Clip {
		return nil
	}
	if err := a.Clip.prepare(); err != nil {
		return err
	}
	a.cursorClip = a.Clip
	a.cursors = reuseAnimationSlice(a.cursors, animationClipTrackCount(a.Clip))
	a.rotationBaselines = reuseAnimationSlice(a.rotationBaselines, 0)
	a.rotationGroupFirst = reuseAnimationSlice(a.rotationGroupFirst, 0)
	a.rotationGroupNext = reuseAnimationSlice(a.rotationGroupNext, 0)
	if a.IsBound() {
		a.prepareRotationStorage()
	}
	a.rotationBindingsReady = false
	a.dirty = true
	return nil
}

func (a *Animator) CurrentTexture() *graphics.Texture {
	if a == nil || a.Clip == nil {
		return nil
	}
	if err := a.Clip.prepare(); err != nil {
		return nil
	}
	for _, trackIndex := range a.Clip.rootTextureTracks {
		track := a.Clip.TextureTracks[trackIndex]
		texture, _ := track.Sample(a.Time(), nil)
		return texture
	}
	return nil
}

// UpdateWorldAnimatorSystem updates animators that own world-space nodes. It
// is scheduled before global transform propagation so animated locals are
// visible to the same frame's hierarchy sync.
func UpdateWorldAnimatorSystem(name string) ecs.SystemConfig {
	if name == "" {
		name = "update_world_animator"
	}
	return ecs.SystemConfig{Name: name, System: updateWorldAnimators}
}

// UpdateUIAnimatorSystem updates UI animators after layout has authored the
// frame's base positions and before UI global transforms are synchronized.
func UpdateUIAnimatorSystem(name string) ecs.SystemConfig {
	if name == "" {
		name = "update_ui_animator"
	}
	return ecs.SystemConfig{Name: name, System: updateUIAnimators}
}

func updateWorldAnimators(context *ecs.SystemContext) error {
	return updateAnimatorsFiltered(context, 1)
}

func updateUIAnimators(context *ecs.SystemContext) error {
	return updateAnimatorsFiltered(context, 2)
}

// space is 1 for world and 2 for UI. Filtering in the query callback avoids a
// second traversal.
func updateAnimatorsFiltered(context *ecs.SystemContext, space uint8) error {
	world := context.World
	if world == nil {
		return nil
	}
	delta := float32(0)
	if clock, ok := world.GetResource[*Time](); ok && clock != nil {
		delta = clock.DeltaSeconds()
	}
	validDelta := delta > 0 && isFiniteAnimationValue(delta)
	var applyErr error
	var bindings *animationComponentBindings
	for entity, access := range world.Query[Animator]().All {
		if !IsActive(world, entity) {
			continue
		}
		animator := access.Ref()

		if animatorSpace(world, entity, animator) != space {
			continue
		}
		animator = access.Mut()
		if validDelta {
			animator.advanceValid(delta)
		} else {
			animator.resetAdvanceState()
		}
		if err := animator.prepareCursors(); err != nil {
			applyErr = err
			break
		}
		if animator.dirty {
			if bindings == nil {
				bindings = world.FrameValue[animationComponentBindingsTag, animationComponentBindings]()
				bindings.bind(world)
			}
			if err := applyAnimator(world, entity, animator, bindings); err != nil {
				applyErr = err
				break
			}
			animator.dirty = false
		}
		emitAnimatorEvents(world, entity, animator)
	}
	return applyErr
}

func animatorSpace(world *ecs.World, entity ecs.Entity, animator *Animator) uint8 {
	if world == nil {
		return 0
	}
	if animator != nil && animator.IsBound() {
		if animator.owner != 0 {
			if world.Has[UITransform](animator.owner) {
				return 2
			}
			if world.Has[Transform](animator.owner) {
				return 1
			}
		}
	}
	if world.Has[UITransform](entity) {
		return 2
	}
	return 1
}

func applyAnimator(world *ecs.World, entity ecs.Entity, animator *Animator, bindings *animationComponentBindings) error {
	if animator == nil || animator.Clip == nil {
		return nil
	}
	// updateAnimatorsFiltered prepares the clip and cursor slices once before
	// entering this sampling path.
	time := animator.Time()
	if !isFiniteAnimationValue(time) {
		return nil
	}
	if animator.IsBound() {
		return applyBoundAnimator(world, animator, bindings)
	}
	floatCursors, textureCursors, boolCursors := animator.cursorSlices()
	targets, err := animationComponents(world, entity, animator.Clip.rootRequirements, bindings)
	if err != nil {
		return err
	}
	for _, trackIndex := range animator.Clip.rootTextureTracks {
		track := &animator.Clip.TextureTracks[trackIndex]
		value, ok := track.sampleDiscreteAt(time, &textureCursors[trackIndex])
		if !ok {
			continue
		}
		// Frame animation replaces any NineSlice belonging to the static texture.
		if sprite := targets.sprite.Ref(); sprite.Texture != value || sprite.Slice != nil {
			sprite = targets.sprite.Mut()
			sprite.Texture, sprite.Slice = value, nil
		}
	}
	rotation := animationRotationValues{}
	if animator.Clip.hasRootRotationTracks {
		rotation = animationRotationValuesForEntity(world, entity)
		rotation.hasX, rotation.hasY, rotation.hasZ = false, false, false
	}
	for _, trackIndex := range animator.Clip.rootFloatTracks {
		track := &animator.Clip.FloatTracks[trackIndex]
		value, ok := sampleAnimationFloatTrackAt(track, time, &floatCursors[trackIndex])
		if !ok {
			continue
		}
		if rotation.set(track.target, value) {
			continue
		}
		if err := applyFloatAnimationTarget(&targets, track.target, value); err != nil {
			return err
		}
	}
	if rotation.any() {
		if err := applyRotationAnimationTarget(&targets, rotation); err != nil {
			return err
		}
	}
	for _, trackIndex := range animator.Clip.rootBoolTracks {
		track := &animator.Clip.BoolTracks[trackIndex]
		value, ok := track.sampleDiscreteAt(time, &boolCursors[trackIndex])
		if !ok {
			continue
		}
		ecs.CommitComponent(targets.visibility, Visibility{Visible: value})
	}
	return nil
}

type animationTargetComponents struct {
	sprite     ecs.ComponentAccess[Sprite2D]
	transform  ecs.ComponentAccess[Transform]
	ui         ecs.ComponentAccess[UITransform]
	visibility ecs.ComponentAccess[Visibility]
}

type animationComponentBindingsTag struct{}
type animationComponentBindings struct {
	sprite     ecs.ComponentBinding[Sprite2D]
	transform  ecs.ComponentBinding[Transform]
	ui         ecs.ComponentBinding[UITransform]
	visibility ecs.ComponentBinding[Visibility]
	ready      bool
}

func (bindings *animationComponentBindings) bind(world *ecs.World) {
	if bindings.ready {
		return
	}
	bindings.sprite = world.BindComponent[Sprite2D]()
	bindings.transform = world.BindComponent[Transform]()
	bindings.ui = world.BindComponent[UITransform]()
	bindings.visibility = world.BindComponent[Visibility]()
	bindings.ready = true
}

func animationComponents(world *ecs.World, entity ecs.Entity, requirements uint8, bindings *animationComponentBindings) (animationTargetComponents, error) {
	row := world.Row(entity)
	result := animationTargetComponents{}
	if requirements&animationNeedsSprite != 0 {
		result.sprite = bindings.sprite.Access(row)
		if result.sprite.Ref() == nil {
			return result, fmt.Errorf("ecs: animation requires Sprite2D")
		}
	}
	if requirements&animationNeedsTransform != 0 {
		result.transform = bindings.transform.Access(row)
		if result.transform.Ref() == nil {
			result.ui = bindings.ui.Access(row)
		}
		if result.transform.Ref() == nil && result.ui.Ref() == nil {
			return result, fmt.Errorf("ecs: animation requires Transform or UITransform")
		}
	}
	if requirements&animationNeedsVisibility != 0 {
		result.visibility = bindings.visibility.Access(row)
		if result.visibility.Ref() == nil {
			return result, fmt.Errorf("ecs: animation requires Visibility")
		}
	}
	return result, nil
}

type animationRotationValues struct {
	x, y, z          float32
	hasX, hasY, hasZ bool
}

func animationRotationValuesForEntity(world *ecs.World, entity ecs.Entity) animationRotationValues {
	if world == nil || entity == 0 {
		return animationRotationValues{}
	}
	if transform, ok := world.Get[Transform](entity); ok {
		return animationRotationValuesFromQuat(transform.Rotation)
	}
	if transform, ok := world.Get[UITransform](entity); ok {
		return animationRotationValues{z: transform.Rotation}
	}
	return animationRotationValues{}
}

func animationRotationValuesFromQuat(rotation mgl32.Quat) animationRotationValues {
	if rotation.Len() == 0 {
		return animationRotationValues{}
	}
	matrix := rotation.Normalize().Mat4()
	// The authored transform order is Rz * Ry * Rx. Extract the same Euler
	// representation so unspecified axes retain their authored values.
	y := float32(math.Asin(float64(clampAnimationFloat(-matrix[2], -1, 1))))
	cy := float32(math.Cos(float64(y)))
	var x, z float32
	if math.Abs(float64(cy)) > 1e-6 {
		x = float32(math.Atan2(float64(matrix[6]), float64(matrix[10])))
		z = float32(math.Atan2(float64(matrix[1]), float64(matrix[0])))
	}
	return animationRotationValues{x: x, y: y, z: z}
}

func clampAnimationFloat(value, low, high float32) float32 {
	return min(max(value, low), high)
}

func (r *animationRotationValues) set(target animationTargetCode, value float32) bool {
	if r == nil {
		return false
	}
	switch target {
	case animationTargetRotationX:
		r.x, r.hasX = value, true
	case animationTargetRotationY:
		r.y, r.hasY = value, true
	case animationTargetRotationZ:
		r.z, r.hasZ = value, true
	default:
		return false
	}
	return true
}

func (r animationRotationValues) any() bool { return r.hasX || r.hasY || r.hasZ }

func applyRotationAnimationTarget(targets *animationTargetComponents, rotation animationRotationValues) error {
	if targets.transform.Ref() != nil {
		rx := mgl32.QuatRotate(rotation.x, mgl32.Vec3{1, 0, 0})
		ry := mgl32.QuatRotate(rotation.y, mgl32.Vec3{0, 1, 0})
		rz := mgl32.QuatRotate(rotation.z, mgl32.Vec3{0, 0, 1})
		rotation := rz.Mul(ry).Mul(rx).Normalize()
		if targets.transform.Ref().Rotation != rotation {
			targets.transform.Mut().Rotation = rotation
		}
		return nil
	}
	if targets.ui.Ref() == nil {
		return fmt.Errorf("ecs: animation rotation requires Transform or UITransform")
	}
	if rotation.hasX || rotation.hasY {
		return fmt.Errorf("ecs: X/Y rotation animation is not available on UITransform")
	}
	if targets.ui.Ref().Rotation != rotation.z {
		targets.ui.Mut().Rotation = rotation.z
	}
	return nil
}

func emitAnimatorEvents(world *ecs.World, entity ecs.Entity, animator *Animator) {
	if animator == nil || animator.Clip == nil || !animator.advanced || len(animator.Clip.Events) == 0 {
		return
	}
	emit := func(event AnimationEvent) {
		world.WriteMessage(AnimationEventMessage{Entity: entity, Name: event.Name, Value: event.Value})
	}
	previous, current := float32(animator.previousTime), animator.Time()
	if animator.emitStart {
		for _, event := range animator.Clip.Events {
			if event.Time == 0 {
				emit(event)
			}
		}
	}
	if animator.loops == 0 {
		for _, event := range animator.Clip.Events {
			if event.Time > previous && event.Time <= current {
				emit(event)
			}
		}
	} else {
		for _, event := range animator.Clip.Events {
			if event.Time > previous {
				emit(event)
			}
		}
		for range animator.loops - 1 {
			for _, event := range animator.Clip.Events {
				emit(event)
			}
		}
		for _, event := range animator.Clip.Events {
			if event.Time <= current {
				emit(event)
			}
		}
	}
	animator.emitStart = false
}

func applyFloatAnimationTarget(targets *animationTargetComponents, target animationTargetCode, value float32) error {
	if target == animationTargetSpriteAlpha {
		setAnimationFloat(targets.sprite, &targets.sprite.Ref().Alpha, value)
		return nil
	}
	var field *float32
	if transform := targets.transform.Ref(); transform != nil {
		switch target {
		case animationTargetPositionX:
			field = &transform.Translation[0]
		case animationTargetPositionY:
			field = &transform.Translation[1]
		case animationTargetPositionZ:
			field = &transform.Translation[2]
		case animationTargetScaleX:
			field = &transform.Scale[0]
		case animationTargetScaleY:
			field = &transform.Scale[1]
		case animationTargetScaleZ:
			field = &transform.Scale[2]
		default:
			return fmt.Errorf("ecs: unsupported float animation target %d", target)
		}
		setAnimationFloat(targets.transform, field, value)
		return nil
	}
	transform := targets.ui.Ref()
	if transform == nil {
		return fmt.Errorf("ecs: animation target %d requires Transform or UITransform", target)
	}
	switch target {
	case animationTargetPositionX:
		field = &transform.Translation.X.Value
	case animationTargetPositionY:
		field = &transform.Translation.Y.Value
	case animationTargetScaleX:
		field = &transform.Scale[0]
	case animationTargetScaleY:
		field = &transform.Scale[1]
	default:
		return fmt.Errorf("ecs: animation target %d is not available on UITransform", target)
	}
	setAnimationFloat(targets.ui, field, value)
	return nil
}

// field belongs to access; mark the component only when its sampled field changes.
func setAnimationFloat[T any](access ecs.ComponentAccess[T], field *float32, value float32) {
	if *field != value {
		access.Mut()
		*field = value
	}
}
