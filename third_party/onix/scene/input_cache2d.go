package scene

import (
	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/internal/reuse"
)

type inputPickChange2D uint8

const (
	inputPickCameras2D inputPickChange2D = 1 << iota
	inputPickTargets2D
	inputPickTransforms2D
)

type inputPickInput2D struct {
	revision func(*ecs.World) (uint64, uint64)
	change   inputPickChange2D
}

func pickInput2D[T any](change inputPickChange2D) inputPickInput2D {
	return inputPickInput2D{revision: (*ecs.World).ComponentValueRevision[T], change: change}
}

// Transform inputs lead the table so only their membership revisions need storage.
const inputPickTransformCount2D = 2

var inputPickInputs2D = [...]inputPickInput2D{
	pickInput2D[GlobalTransform](inputPickCameras2D | inputPickTransforms2D),
	pickInput2D[UIGlobalTransform](inputPickTransforms2D),
	pickInput2D[Camera](inputPickCameras2D), pickInput2D[Projection](inputPickCameras2D), pickInput2D[Camera2D](inputPickCameras2D),
	pickInput2D[InputTarget2D](inputPickTargets2D), pickInput2D[InputBlocker2D](inputPickTargets2D),
	pickInput2D[Visibility](inputPickTargets2D),
	pickInput2D[Activity](inputPickCameras2D | inputPickTargets2D), pickInput2D[Parent](inputPickCameras2D | inputPickTargets2D),
	pickInput2D[RenderLayers](inputPickCameras2D | inputPickTargets2D),
	pickInput2D[UITargetCamera](inputPickTargets2D), pickInput2D[PainterGroup2D](inputPickTargets2D),
	pickInput2D[UIStackIndex](inputPickTargets2D), pickInput2D[RenderOrder2D](inputPickTargets2D),
	pickInput2D[Anchor](inputPickTargets2D), pickInput2D[Sprite2D](inputPickTargets2D), pickInput2D[SpriteLayers2D](inputPickTargets2D),
}

// Each execution scope owns its pick snapshot. Value revisions ignore row
// migrations while still detecting writes/removals within the same ECS tick.
type inputPickCache2D struct {
	renderState      *RenderWorld
	changes          ecs.SystemContext
	targets          []inputPickTarget2D
	cameras          []inputCamera2D
	lookup           inputCaptureLookup2D
	versions         [len(inputPickInputs2D)]uint64
	retention        [2]reuse.Retention
	transformMembers [inputPickTransformCount2D]uint64
	registryRevision uint64
	scopeVersion     uint64
	pipelineRevision uint64
	defaultUICamera  ecs.Entity
	generation       uint64
	lastRun          ecs.Tick
	initialized      bool
}

func (cache *inputPickCache2D) refresh(world *ecs.World, entity, camera ecs.Entity) bool {
	cache.targets = cache.retention[0].Trim(cache.targets, 64)
	cache.cameras = cache.retention[1].Trim(cache.cameras, 4)
	var versions [len(inputPickInputs2D)]uint64
	var members [inputPickTransformCount2D]uint64
	var changes inputPickChange2D
	var transformWrites uint64
	for i, input := range inputPickInputs2D {
		version, membership := input.revision(world)
		versions[i] = version
		if cache.versions[i] != version {
			changes |= input.change
		}
		if i < len(members) {
			members[i] = membership
			transformWrites += version - cache.versions[i]
		}
	}
	if cache.transformMembers != members {
		changes |= inputPickTargets2D
	}
	_, scopeVersion := world.QueryScopeKey()
	state, _ := world.GetResource[*RenderWorld]()
	registryRevision, pipelineRevision := uint64(0), uint64(0)
	if state != nil {
		registryRevision = state.registry2D
		for _, source := range state.cameraSettings {
			if source.revision != nil {
				pipelineRevision += source.revision(world)
			}
		}
	}
	if !cache.initialized || cache.scopeVersion != scopeVersion {
		changes |= inputPickCameras2D | inputPickTargets2D
	}
	if cache.renderState != state || cache.registryRevision != registryRevision || cache.pipelineRevision != pipelineRevision {
		changes |= inputPickCameras2D
	}
	if changes == 0 && cache.lookup.entity == entity && cache.lookup.cameraEntity == camera {
		return false
	}
	lookup := inputCaptureLookup2D{entity: entity, cameraEntity: camera}
	if changes&inputPickCameras2D != 0 {
		previous := cache.cameras
		cache.cameras, cache.defaultUICamera = collectInputCamerasInto2D(world, cache.cameras, &lookup)
		clearSnapshotTail(cache.cameras, len(previous))
	} else {
		lookup.defaultUICamera = cache.defaultUICamera
		for _, candidate := range cache.cameras {
			if candidate.entity == camera {
				lookup.camera, lookup.cameraFound = candidate, true
				break
			}
		}
	}
	if changes&inputPickTargets2D == 0 && changes&inputPickTransforms2D != 0 {
		since := cache.lastRun
		if since == world.ChangeTick() {
			since--
		}
		cache.changes = world.ChangesSince(since)
		if transformWrites > uint64(max(16, len(cache.targets)/4)) ||
			!cache.refreshTargetTransforms[GlobalTransform](world, &cache.changes, false) ||
			!cache.refreshTargetTransforms[UIGlobalTransform](world, &cache.changes, true) {
			changes |= inputPickTargets2D
		}
	}
	if changes&inputPickTargets2D != 0 {
		previous := cache.targets
		cache.targets = collectInputTargetsInto2D(world, cache.targets, &lookup)
		clearSnapshotTail(cache.targets, len(previous))
	} else if lookup.cameraFound && entity != 0 {
		for i := range cache.targets {
			lookup.consider(&cache.targets[i])
		}
	}
	if changes != 0 {
		cache.versions, cache.transformMembers = versions, members
		cache.scopeVersion = scopeVersion
		cache.renderState, cache.pipelineRevision = state, pipelineRevision
		cache.registryRevision = registryRevision
		cache.lastRun, cache.initialized = world.ChangeTick(), true
		cache.generation++
	}
	cache.lookup = lookup
	return changes != 0
}

// Reuse the sorted snapshot when only transforms changed. Its existing sort
// keys locate records without another entity index. A depth change falls back
// to collection/sorting; changes to clipping and the matrix preserve order.
func (cache *inputPickCache2D) refreshTargetTransforms[T renderGlobalTransform2D](world *ecs.World, changes *ecs.SystemContext, ui bool) bool {
	if len(cache.targets) == 0 {
		return true
	}
	hierarchy := frameRenderHierarchy2DCachePtr(world)
	for entity, access := range world.Query[T](changes.Changed[T]()).All {
		if !world.Has[InputTarget2D](entity) && !world.Has[InputBlocker2D](entity) {
			continue
		}
		if visibility := world.GetRef[Visibility](entity); visibility == nil || !visibility.Visible || !IsActive(world, entity) {
			continue
		}
		transform := access.Get()
		matrix := transform.renderMatrix2D()
		info := renderEntityInfo2D(world, entity, ui, 0, hierarchy)
		key := inputPickTarget2D{entity: entity, ui: ui, group: info.group, sortIndex: info.sortIndex, depth: matrix[14]}
		low, high := 0, len(cache.targets)
		for low < high {
			mid := int(uint(low+high) >> 1)
			if compareInputPickTargetKeys2D(&cache.targets[mid], &key) < 0 {
				low = mid + 1
			} else {
				high = mid
			}
		}
		if low == len(cache.targets) || cache.targets[low].entity != entity || cache.targets[low].ui != ui {
			return false
		}
		key.sequence = cache.targets[low].sequence
		if compareInputPickTargetKeys2D(&cache.targets[low], &key) != 0 {
			return false
		}
		for i := low; i < len(cache.targets) && cache.targets[i].entity == entity && cache.targets[i].ui == ui; i++ {
			target := &cache.targets[i]
			target.clip = transform.renderClip2D()
			target.prepare(matrix)
		}
	}
	return true
}
