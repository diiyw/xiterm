package scene

import (
	"fmt"

	"github.com/onix-engine/onix/ecs"
)

// This file contains playback for clips whose bindings were resolved to real
// scene entities during plan compilation. It shares the typed sampling
// primitives with direct animators, but never parses Binding.Path or performs
// a name lookup during a frame.

func applyBoundAnimator(world *ecs.World, animator *Animator, bindings *animationComponentBindings) error {
	if animator == nil || animator.Clip == nil {
		return nil
	}
	clip := animator.Clip
	time := animator.Time()
	floatTargets, textureTargets, boolTargets := animator.boundTargetSlices()
	floatCursors, textureCursors, boolCursors := animator.cursorSlices()
	animator.prepareBoundRotationBaselines(world)

	for trackIndex := range clip.TextureTracks {
		track := &clip.TextureTracks[trackIndex]
		entity, ok := animator.boundTarget(textureTargets, trackIndex, track.root)
		if !ok {
			return animationTargetError("texture", trackIndex)
		}
		value, sampled := track.sampleDiscreteAt(time, &textureCursors[trackIndex])
		if !sampled {
			continue
		}
		access := bindings.sprite.Access(world.Row(entity))
		sprite := access.Ref()
		if sprite == nil {
			return animationEntityComponentError(entity, "Sprite2D")
		}
		if sprite.Texture != value || sprite.Slice != nil {
			sprite = access.Mut()
			sprite.Texture, sprite.Slice = value, nil
		}
	}

	for trackIndex := range clip.FloatTracks {
		track := &clip.FloatTracks[trackIndex]
		entity, ok := animator.boundTarget(floatTargets, trackIndex, track.root)
		if !ok {
			return animationTargetError("float", trackIndex)
		}
		if isAnimationRotationTarget(track.target) {
			if trackIndex >= len(animator.rotationGroupFirst) || animator.rotationGroupFirst[trackIndex] != trackIndex {
				continue
			}
			rotation := animator.rotationBaseline(trackIndex)
			for candidateIndex := trackIndex; candidateIndex >= 0; candidateIndex = animator.rotationGroupNext[candidateIndex] {
				candidate := &clip.FloatTracks[candidateIndex]
				value, sampled := sampleAnimationFloatTrackAt(candidate, time, &floatCursors[candidateIndex])
				if sampled {
					rotation.set(candidate.target, value)
				}
			}
			if rotation.any() {
				components, err := animationComponentsForTarget(world, entity, animationNeedsTransform, bindings)
				if err != nil {
					return err
				}
				if err := applyRotationAnimationTarget(&components, rotation); err != nil {
					return err
				}
			}
			continue
		}
		value, sampled := sampleAnimationFloatTrackAt(track, time, &floatCursors[trackIndex])
		if !sampled {
			continue
		}
		components, err := animationComponentsForTarget(world, entity, animationRequirement(track.target), bindings)
		if err != nil {
			return err
		}
		if err := applyFloatAnimationTarget(&components, track.target, value); err != nil {
			return err
		}
	}

	for trackIndex := range clip.BoolTracks {
		track := &clip.BoolTracks[trackIndex]
		entity, ok := animator.boundTarget(boolTargets, trackIndex, track.root)
		if !ok {
			return animationTargetError("bool", trackIndex)
		}
		value, sampled := track.sampleDiscreteAt(time, &boolCursors[trackIndex])
		if !sampled {
			continue
		}
		access := bindings.visibility.Access(world.Row(entity))
		if access.Ref() == nil {
			return animationEntityComponentError(entity, "Visibility")
		}
		ecs.CommitComponent(access, Visibility{Visible: value})
	}
	return nil
}

func (a *Animator) prepareBoundRotationBaselines(world *ecs.World) {
	if a == nil || a.Clip == nil || !a.Clip.hasRotationTracks || a.rotationBindingsReady {
		return
	}
	trackCount := len(a.Clip.FloatTracks)
	a.rotationBaselines = reuseAnimationSlice(a.rotationBaselines, trackCount)
	a.rotationGroupFirst = reuseAnimationSlice(a.rotationGroupFirst, trackCount)
	a.rotationGroupNext = reuseAnimationSlice(a.rotationGroupNext, trackCount)
	for index := range a.rotationGroupFirst {
		a.rotationGroupFirst[index] = -1
		a.rotationGroupNext[index] = -1
	}
	floatTargets, _, _ := a.boundTargetSlices()
	for index, track := range a.Clip.FloatTracks {
		if !isAnimationRotationTarget(track.target) {
			continue
		}
		entity, ok := a.boundTarget(floatTargets, index, track.root)
		if !ok {
			continue
		}
		first := -1
		for previous := 0; previous < index; previous++ {
			previousTrack := a.Clip.FloatTracks[previous]
			if !isAnimationRotationTarget(previousTrack.target) {
				continue
			}
			previousEntity, previousOK := a.boundTarget(floatTargets, previous, previousTrack.root)
			if previousOK && previousEntity == entity {
				first = a.rotationGroupFirst[previous]
				break
			}
		}
		if first < 0 {
			a.rotationGroupFirst[index] = index
			a.rotationBaselines[index] = animationRotationValuesForEntity(world, entity)
			continue
		}
		a.rotationGroupFirst[index] = first
		last := first
		for next := a.rotationGroupNext[last]; next >= 0; next = a.rotationGroupNext[last] {
			last = next
		}
		a.rotationGroupNext[last] = index
	}
	a.rotationBindingsReady = true
}

func (a *Animator) rotationBaseline(index int) animationRotationValues {
	if a == nil || index < 0 || index >= len(a.rotationBaselines) {
		return animationRotationValues{}
	}
	value := a.rotationBaselines[index]
	value.hasX, value.hasY, value.hasZ = false, false, false
	return value
}

func (a *Animator) boundTarget(targets []ecs.Entity, index int, root bool) (ecs.Entity, bool) {
	if index >= 0 && index < len(targets) && targets[index] != 0 {
		return targets[index], true
	}
	if root && a != nil && a.owner != 0 && len(targets) == 0 {
		return a.owner, true
	}
	return 0, false
}

func isAnimationRotationTarget(target animationTargetCode) bool {
	return target == animationTargetRotationX || target == animationTargetRotationY || target == animationTargetRotationZ
}

func animationComponentsForTarget(world *ecs.World, entity ecs.Entity, requirements uint8, bindings *animationComponentBindings) (animationTargetComponents, error) {
	if entity == 0 {
		return animationTargetComponents{}, animationEntityComponentError(entity, "target")
	}
	return animationComponents(world, entity, requirements, bindings)
}

func animationTargetError(kind string, index int) error {
	return fmt.Errorf("ecs: %s animation track %d has no bound target", kind, index)
}

func animationEntityComponentError(entity ecs.Entity, component string) error {
	return fmt.Errorf("ecs: animation target entity %d requires %s", entity, component)
}
