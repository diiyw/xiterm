package scene

import (
	"cmp"
	"math"
	"slices"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/input"
	"github.com/onix-engine/onix/internal/smallsort"
)

type InputTarget2D struct {
	Width       float32
	Height      float32
	Receiver    ecs.Entity
	Cursor      input.Cursor
	InputFilter InputFilter
	PassThrough bool
	Disabled    bool
	// AllowDisabledInput keeps non-editing inputs (for example
	// scrolling a read-only textarea) available while suppressing editing.
	AllowDisabledInput bool
}

type InputBlocker2D struct {
	Width       float32
	Height      float32
	Receiver    ecs.Entity
	Cursor      input.Cursor
	InputFilter InputFilter
	PassThrough bool
}

type InputFilter uint8

const (
	InputFilterStop InputFilter = iota
	InputFilterPass
	InputFilterIgnore
)

// InputState2D is the input state shared by ordinary render entities
// and composed controls. The input dispatcher is its only writer.
type InputState2D struct {
	Hovered bool
	Pressed bool
}

// InputHandler2D handles events synchronously along the target path.
type InputHandler2D struct {
	Handle func(*InputEventContext)
}

type inputTracker2D struct {
	cursorTarget    ecs.Entity
	active          ecs.Entity
	activeReceiver  ecs.Entity
	activeCamera    ecs.Entity
	hovered         ecs.Entity
	hoveredReceiver ecs.Entity
	hasActive       bool
	pointerDevice   input.PointerDevice
	pointerX        float32
	pointerY        float32
}

// RemapInputEntity2D preserves pointer tracking when a retained UI
// control is rebuilt with a new entity identity during an active input.
func RemapInputEntity2D(world *ecs.World, previous, current ecs.Entity) {
	if world == nil || previous == 0 || current == 0 || previous == current {
		return
	}
	hovered, pressed := false, false
	if tracker, ok := world.GetResource[*inputTracker2D](); ok && tracker != nil {
		if tracker.cursorTarget == previous {
			tracker.cursorTarget = current
		}
		if tracker.activeReceiver == previous {
			tracker.active, tracker.activeReceiver = current, current
			pressed = tracker.hasActive
		} else if tracker.active == previous {
			tracker.active = current
			pressed = tracker.hasActive
		}
		if tracker.hoveredReceiver == previous {
			tracker.hovered, tracker.hoveredReceiver = current, current
			hovered = true
		} else if tracker.hovered == previous {
			tracker.hovered = current
			hovered = true
		}
	}
	if capture, ok := world.GetResource[*InputCapture2D](); ok && capture != nil {
		if capture.Receiver == previous {
			capture.Target, capture.Receiver = current, current
		} else if capture.Target == previous {
			capture.Target = current
		}
	}
	if state := world.GetMut[InputState2D](current); state != nil {
		state.Hovered, state.Pressed = hovered, pressed
	}
}

type inputEventArena2DScratch struct{}

type inputEventArena2D struct {
	values   []InputEvent
	contexts []InputEventContext
	depth    int
}

type inputPickTarget2D struct {
	layers    RenderLayers
	receiver  ecs.Entity
	sortIndex int
	group     uint64
	camera    ecs.Entity
	target    ecs.Entity
	entity    ecs.Entity
	inverse   [9]float32
	clip      clipRect2D
	anchor    Anchor
	height    float32
	width     float32
	depth     float32
	sequence  uint32
	ui        bool
	blocked   bool
}

type inputCamera2D struct {
	layers     RenderLayers
	camera     Camera
	entity     ecs.Entity
	projection Projection
	transform  GlobalTransform
	pipeline   CameraPipeline
}

// Resolve the captured target while collecting the frame's pick snapshot.
// Multiple records can share an entity; capture uses the first visible record
// in pick sort order, just as targetByEntity2D does.
type inputCaptureLookup2D struct {
	target          inputPickTarget2D
	camera          inputCamera2D
	entity          ecs.Entity
	cameraEntity    ecs.Entity
	defaultUICamera ecs.Entity
	cameraFound     bool
	targetFound     bool
}

func (c *inputCaptureLookup2D) consider(target *inputPickTarget2D) {
	if c == nil || !c.cameraFound || target.entity != c.entity || !inputTargetVisibleInCamera(target, c.camera, c.defaultUICamera) {
		return
	}
	if !c.targetFound || compareInputPickTarget2D(*target, c.target) < 0 {
		c.target, c.targetFound = *target, true
	}
}

func (c *inputCaptureLookup2D) resolve(cameras []inputCamera2D, defaultUICamera ecs.Entity, targets []inputPickTarget2D, entity, cameraEntity ecs.Entity, x, y float32) (inputPickTarget2D, float32, float32, bool) {
	if entity != c.entity || cameraEntity != c.cameraEntity {
		// A press or a synchronous callback can replace/remap the capture after
		// collection. Resolve the new identity against the same frame snapshot.
		return targetByEntity2D(cameras, defaultUICamera, targets, entity, cameraEntity, x, y)
	}
	if !c.targetFound {
		return inputPickTarget2D{}, 0, 0, false
	}
	x, y, inViewport := inputCoordsForCamera2D(c.camera, c.target.ui, x, y)
	target := c.target
	target.camera = c.camera.entity
	return target, x, y, inViewport
}

func compareInputCamera2D(a, b inputCamera2D) int {
	if order := cmp.Compare(a.camera.Order, b.camera.Order); order != 0 {
		return order
	}
	return cmp.Compare(a.entity, b.entity)
}

func compareInputPickTarget2D(a, b inputPickTarget2D) int {
	return compareInputPickTargetKeys2D(&a, &b)
}

func compareInputPickTargetKeys2D(a, b *inputPickTarget2D) int {
	if a.ui != b.ui {
		if a.ui {
			return 1
		}
		return -1
	}
	if a.group != b.group {
		return cmp.Compare(a.group, b.group)
	}
	if a.sortIndex != b.sortIndex {
		return cmp.Compare(a.sortIndex, b.sortIndex)
	}
	if a.depth < b.depth {
		return -1
	}
	if a.depth > b.depth {
		return 1
	}
	if a.entity < b.entity {
		return -1
	}
	if a.entity > b.entity {
		return 1
	}
	return cmp.Compare(a.sequence, b.sequence)
}

func DispatchInputEvents2DSystem(name string) ecs.SystemConfig {
	if name == "" {
		name = "dispatch_pointer_events_2d"
	}
	return ecs.SystemConfig{
		Name:   name,
		System: dispatchInputEvents2D,
	}
}

func dispatchInputEvents2D(context *ecs.SystemContext) error {
	world := context.World
	if world == nil {
		return nil
	}
	inputState, ok := world.GetResource[*input.State]()
	if !ok || inputState == nil {
		return nil
	}
	tracker, ok := world.GetResource[*inputTracker2D]()
	if !ok || tracker == nil {
		tracker = &inputTracker2D{}
		world.InsertResource(tracker)
	}
	capture, ok := world.GetResource[*InputCapture2D]()
	if !ok || capture == nil {
		capture = &InputCapture2D{}
		world.InsertResource(capture)
	}
	pointer := inputState.Pointer()
	pointerMoved := tracker.pointerDevice != pointer.Device || tracker.pointerX != pointer.X || tracker.pointerY != pointer.Y
	tracker.pointerDevice, tracker.pointerX, tracker.pointerY = pointer.Device, pointer.X, pointer.Y
	pointerActive := pointerMoved || pointer.Pressed || pointer.Released || pointer.DeltaX != 0 || pointer.DeltaY != 0 || pointer.WheelX != 0 || pointer.WheelY != 0
	cache := world.ScopeValue[inputPickCache2D, inputPickCache2D]()
	changed := cache.refresh(world, tracker.active, tracker.activeCamera)
	if !changed && !pointerActive {
		syncPointerCursor2D(world, inputState, tracker.cursorTarget)
		return nil
	}
	cameras, targets, defaultUICamera := cache.cameras, cache.targets, cache.defaultUICamera
	lookup := cache.lookup
	hovered, hoverX, hoverY, hoverPicked := pickInputTarget2D(cameras, defaultUICamera, targets, pointer.X, pointer.Y)
	tracker.cursorTarget = hovered.entity
	syncPointerCursor2D(world, inputState, tracker.cursorTarget)
	updateInputHover2D(world, tracker, pointer.Device, hovered, hoverX, hoverY, hoverPicked)
	if hoverPicked && (pointer.WheelX != 0 || pointer.WheelY != 0) {
		event := pointerInputEvent(pointer.Device, hovered.receiver, inputEventPath2D(world, hovered.receiver), InputEventWheel, hoverX, hoverY)
		event.WheelX, event.WheelY = pointer.WheelX, pointer.WheelY
		DispatchInputEvent(world, event)
	}
	if pointer.Pressed {
		pressed, pressX, pressY, pressPicked := hovered, hoverX, hoverY, hoverPicked
		if pointer.PressX != pointer.X || pointer.PressY != pointer.Y {
			pressed, pressX, pressY, pressPicked = pickInputTarget2D(cameras, defaultUICamera, targets, pointer.PressX, pointer.PressY)
		}
		tracker.active = pressed.entity
		tracker.activeReceiver = pressed.receiver
		tracker.activeCamera = pressed.camera
		tracker.hasActive = pressPicked
		if pressPicked {
			capture.Target = pressed.entity
			capture.Receiver = pressed.receiver
			capture.Camera = pressed.camera
			setPointerPressed2D(world, pressed.receiver, true)
			DispatchInputEvent(world, pointerInputEvent(pointer.Device, pressed.receiver, inputEventPath2D(world, pressed.receiver), InputEventPress, pressX, pressY))
		}
	}
	if pointer.Down && tracker.hasActive && (pointer.DeltaX != 0 || pointer.DeltaY != 0) {
		_, x, y, picked := lookup.resolve(cameras, defaultUICamera, targets, tracker.active, tracker.activeCamera, pointer.X, pointer.Y)
		if picked {
			event := pointerInputEvent(pointer.Device, tracker.activeReceiver, inputEventPath2D(world, tracker.activeReceiver), InputEventMove, x, y)
			event.DeltaX, event.DeltaY = pointer.DeltaX, pointer.DeltaY
			DispatchInputEvent(world, event)
		}
	}
	if !tracker.hasActive && hoverPicked && (pointer.DeltaX != 0 || pointer.DeltaY != 0) {
		// Hover movement is a normal pointer event too. Modules such as cursor
		// readouts and camera coordinate probes must not require a button press.
		event := pointerInputEvent(pointer.Device, hovered.receiver, inputEventPath2D(world, hovered.receiver), InputEventMove, hoverX, hoverY)
		event.DeltaX, event.DeltaY = pointer.DeltaX, pointer.DeltaY
		DispatchInputEvent(world, event)
	}
	if pointer.Released {
		if tracker.hasActive {
			activeTarget, activeX, activeY, activePicked := lookup.resolve(cameras, defaultUICamera, targets, tracker.active, tracker.activeCamera, pointer.X, pointer.Y)
			if activePicked {
				DispatchInputEvent(world, pointerInputEvent(pointer.Device, tracker.activeReceiver, inputEventPath2D(world, tracker.activeReceiver), InputEventRelease, activeX, activeY))
				if hoverPicked && hovered.entity == activeTarget.entity {
					DispatchInputEvent(world, pointerInputEvent(pointer.Device, tracker.activeReceiver, inputEventPath2D(world, tracker.activeReceiver), InputEventTap, hoverX, hoverY))
				}
			} else {
				DispatchInputEvent(world, pointerInputEvent(pointer.Device, tracker.activeReceiver, inputEventPath2D(world, tracker.activeReceiver), InputEventCancel, pointer.X, pointer.Y))
			}
			setPointerPressed2D(world, tracker.activeReceiver, false)
		}
		tracker.active = 0
		tracker.activeReceiver = 0
		tracker.activeCamera = 0
		tracker.hasActive = false
		capture.Target = 0
		capture.Receiver = 0
		capture.Camera = 0
	}
	return nil
}

// SyncPointerCursor2D refreshes the hovered entity's cursor after UI style updates.
// It reuses the input dispatcher's hit result without another hit-test traversal.
func SyncPointerCursor2D(world *ecs.World) {
	state, _ := world.GetResource[*input.State]()
	tracker, _ := world.GetResource[*inputTracker2D]()
	if state != nil && tracker != nil {
		syncPointerCursor2D(world, state, tracker.cursorTarget)
	}
}

// Use the existing pick result, including non-interactive surfaces that occlude
// controls. Cursor-only style changes are also observed while the pointer is idle.
func syncPointerCursor2D(world *ecs.World, state *input.State, entity ecs.Entity) {
	cursor := input.CursorDefault
	if state.Pointer().Device == input.PointerDeviceMouse {
		if target := world.GetRef[InputTarget2D](entity); target != nil {
			cursor = target.Cursor
		} else if blocker := world.GetRef[InputBlocker2D](entity); blocker != nil {
			cursor = blocker.Cursor
		}
	}
	state.SetCursor(cursor)
}

func updateInputHover2D(world *ecs.World, tracker *inputTracker2D, device input.PointerDevice, target inputPickTarget2D, x, y float32, picked bool) {
	entity, receiver := ecs.Entity(0), ecs.Entity(0)
	if picked {
		entity, receiver = target.entity, target.receiver
	}
	if tracker.hovered == entity && tracker.hoveredReceiver == receiver {
		return
	}
	if tracker.hoveredReceiver != 0 {
		setInputHovered2D(world, tracker.hoveredReceiver, false)
		DispatchInputEvent(world, pointerInputEvent(device, tracker.hoveredReceiver, inputEventPath2D(world, tracker.hoveredReceiver), InputEventLeave, x, y))
	}
	tracker.hovered, tracker.hoveredReceiver = entity, receiver
	if receiver != 0 {
		setInputHovered2D(world, receiver, true)
		DispatchInputEvent(world, pointerInputEvent(device, receiver, inputEventPath2D(world, receiver), InputEventEnter, x, y))
	}
}

func pointerInputEvent(device input.PointerDevice, entity ecs.Entity, path []ecs.Entity, kind InputEventKind, x, y float32) InputEvent {
	button := input.MouseNone
	if device == input.PointerDeviceMouse {
		button = input.MouseLeft
	}
	return InputEvent{Entity: entity, Path: path, Kind: kind, Device: device, Button: button, X: x, Y: y}
}

func setInputHovered2D(world *ecs.World, entity ecs.Entity, value bool) {
	if state := world.GetMut[InputState2D](entity); state != nil {
		state.Hovered = value
	}
}

func setPointerPressed2D(world *ecs.World, entity ecs.Entity, value bool) {
	if state := world.GetMut[InputState2D](entity); state != nil {
		state.Pressed = value
	}
}

// DispatchInputEvent delivers an event to its target path and publishes the result.
// Handlers may stop propagation or consume the event; nested dispatch reuses scratch storage.
func DispatchInputEvent(world *ecs.World, event InputEvent) {
	arena := world.FrameValue[inputEventArena2DScratch, inputEventArena2D]()
	index := len(arena.values)
	arena.values = append(arena.values, event)
	contextIndex := len(arena.contexts)
	arena.contexts = append(arena.contexts, InputEventContext{})
	arena.depth++
	defer func() {
		arena.depth--
		if arena.depth == 0 {
			clear(arena.values)
			arena.values = arena.values[:0]
			clear(arena.contexts)
			arena.contexts = arena.contexts[:0]
		}
	}()
	eventPtr := &arena.values[index]
	path := eventPtr.Path
	context := &arena.contexts[contextIndex]
	context.Event = eventPtr
	context.Target = eventPtr.Entity
	if len(path) == 0 {
		if eventPtr.Entity != 0 {
			dispatchInputHandler2D(world, context, eventPtr.Entity)
		}
		if context.Handled {
			eventPtr.Handled = true
		}
		world.WriteMessage(*eventPtr)
		return
	}
	for _, entity := range path {
		if dispatchInputHandler2D(world, context, entity) {
			if context.Handled {
				eventPtr.Handled = true
			}
			break
		}
	}
	world.WriteMessage(*eventPtr)
}

func dispatchInputHandler2D(world *ecs.World, context *InputEventContext, entity ecs.Entity) bool {
	if !world.IsAlive(entity) || !IsActive(world, entity) {
		return false
	}
	filter := InputFilterPass
	if target := world.GetRef[InputTarget2D](entity); target != nil {
		filter = target.InputFilter
	}
	if filter != InputFilterIgnore {
		if handler := world.GetRef[InputHandler2D](entity); handler != nil && handler.Handle != nil {
			context.Current = entity
			handler.Handle(context)
		}
	}
	return context.Handled || filter == InputFilterStop
}

type inputPathScratch struct{}

func inputEventPath2D(world *ecs.World, entity ecs.Entity) []ecs.Entity {
	if world == nil || entity == 0 {
		return nil
	}
	parent, ok := world.Get[Parent](entity)
	if !ok {
		return nil
	}
	path := world.MessageScratch[inputPathScratch, ecs.Entity]()
	start := len(path.Values)
	for remaining := world.EntityCount() + 1; entity != 0 && remaining > 0; remaining-- {
		if !world.IsAlive(entity) {
			break
		}
		path.Values = append(path.Values, entity)
		entity = parent.Entity
		if entity == 0 {
			break
		}
		parent, ok = world.Get[Parent](entity)
		if !ok {
			if world.IsAlive(entity) {
				path.Values = append(path.Values, entity)
			}
			break
		}
	}
	return path.Values[start:]
}

func collectInputCamerasInto2D(world *ecs.World, cameras []inputCamera2D, capture *inputCaptureLookup2D) ([]inputCamera2D, ecs.Entity) {
	cameras = cameras[:0]
	state, _ := world.GetResource[*RenderWorld]()
	var defaultCamera inputCamera2D
	var fallback inputCamera2D
	for entity, access := range world.Query[Camera]().All {
		if !IsActive(world, entity) {
			continue
		}
		camera := access.Get()

		if !camera.Active {
			continue
		}
		projection, projectionOK := world.Get[Projection](entity)
		transform, transformOK := world.Get[GlobalTransform](entity)
		if !projectionOK || !transformOK {
			continue
		}
		pipeline, _, found := cameraSettingsForEntity(state, world, entity)
		if !found {
			continue
		}
		candidate := inputCamera2D{
			entity: entity, camera: camera, projection: projection, transform: transform,
			pipeline: pipeline, layers: LayersForEntity(world, entity),
		}
		if camera.Fallback {
			fallback = candidate
			continue
		}
		cameras = append(cameras, candidate)
		if pipeline == CameraPipeline2D && (defaultCamera.entity == 0 || compareInputCamera2D(candidate, defaultCamera) < 0) {
			defaultCamera = candidate
		}
		if capture != nil && capture.cameraEntity == entity {
			capture.camera, capture.cameraFound = candidate, true
		}
	}
	if defaultCamera.entity == 0 && fallback.entity != 0 {
		defaultCamera = fallback
		cameras = append(cameras, fallback)
		if capture != nil && capture.cameraEntity == fallback.entity {
			capture.camera, capture.cameraFound = fallback, true
		}
	}
	smallsort.InsertionSortFunc(cameras, compareInputCamera2D)
	if capture != nil {
		capture.defaultUICamera = defaultCamera.entity
	}
	return cameras, defaultCamera.entity
}

func collectInputTargetsInto2D(world *ecs.World, targets []inputPickTarget2D, capture *inputCaptureLookup2D) []inputPickTarget2D {
	targets = targets[:0]
	hierarchy := frameRenderHierarchy2DCachePtr(world)
	targets = hierarchy.collectInputTargetsWithTransform2D[GlobalTransform](world, targets, false, capture)
	targets = hierarchy.collectInputTargetsWithTransform2D[UIGlobalTransform](world, targets, true, capture)
	targets = hierarchy.collectInputBlockersWithTransform2D[GlobalTransform](world, targets, false, capture)
	targets = hierarchy.collectInputBlockersWithTransform2D[UIGlobalTransform](world, targets, true, capture)
	slices.SortFunc(targets, compareInputPickTarget2D)
	return targets
}

func (hierarchy *renderHierarchy2DCache) collectInputTargetsWithTransform2D[T renderGlobalTransform2D](world *ecs.World, targets []inputPickTarget2D, ui bool, capture *inputCaptureLookup2D) []inputPickTarget2D {
	for entity, access := range world.Query3[InputTarget2D, T, Visibility]().All {
		if !IsActive(world, entity) {
			continue
		}
		target := access.A.Ref()
		transform := access.B.Ref()
		visibility := access.C.Ref()

		if !visibility.Visible || target.PassThrough || target.InputFilter == InputFilterIgnore ||
			(target.Disabled && !target.AllowDisabledInput) {
			continue
		}
		width, height := inputDimensions2D(world, entity, target)
		if width <= 0 || height <= 0 {
			continue
		}
		transformMatrix := (*transform).renderMatrix2D()
		info := renderEntityInfo2D(world, entity, ui, 0, hierarchy)
		receiver := target.Receiver
		if receiver == 0 {
			receiver = entity
		}
		targets = append(targets, inputPickTarget2D{
			entity:    entity,
			sequence:  uint32(len(targets)),
			sortIndex: info.sortIndex, group: info.group,
			depth:    transformMatrix[14],
			width:    width,
			height:   height,
			anchor:   anchorForEntity2D(world, entity),
			layers:   info.layers,
			target:   info.target,
			clip:     (*transform).renderClip2D(),
			ui:       ui,
			receiver: receiver,
		})
		targets[len(targets)-1].prepare(transformMatrix)
		if capture != nil && entity == capture.entity {
			capture.consider(&targets[len(targets)-1])
		}
	}
	return targets
}

func (hierarchy *renderHierarchy2DCache) collectInputBlockersWithTransform2D[T renderGlobalTransform2D](world *ecs.World, targets []inputPickTarget2D, ui bool, capture *inputCaptureLookup2D) []inputPickTarget2D {
	for entity, access := range world.Query3[InputBlocker2D, T, Visibility]().All {
		if !IsActive(world, entity) {
			continue
		}
		blocker := access.A.Ref()
		transform := access.B.Ref()
		visibility := access.C.Ref()

		width, height := renderSize2D(world, entity, blocker.Width, blocker.Height)
		if !visibility.Visible || blocker.PassThrough || blocker.InputFilter != InputFilterStop || width <= 0 || height <= 0 {
			continue
		}
		transformMatrix := (*transform).renderMatrix2D()
		info := renderEntityInfo2D(world, entity, ui, 0, hierarchy)
		receiver := blocker.Receiver
		targets = append(targets, inputPickTarget2D{
			entity: entity, sequence: uint32(len(targets)), sortIndex: info.sortIndex, group: info.group, depth: transformMatrix[14], width: width, height: height,
			anchor: anchorForEntity2D(world, entity), layers: info.layers, target: info.target, clip: (*transform).renderClip2D(), ui: ui,
			blocked: receiver == 0, receiver: receiver,
		})
		targets[len(targets)-1].prepare(transformMatrix)
		if capture != nil && entity == capture.entity {
			capture.consider(&targets[len(targets)-1])
		}
	}
	return targets
}

func pickInputTarget2D(cameras []inputCamera2D, defaultUICamera ecs.Entity, targets []inputPickTarget2D, screenX, screenY float32) (inputPickTarget2D, float32, float32, bool) {
	for cameraIndex := len(cameras) - 1; cameraIndex >= 0; cameraIndex-- {
		camera := cameras[cameraIndex]
		if _, _, inViewport := inputCoordsForCamera2D(camera, true, screenX, screenY); !inViewport {
			continue
		}
		var worldX, worldY float32
		worldCoordsReady, worldCoordsValid := false, false
		for targetIndex := len(targets) - 1; targetIndex >= 0; targetIndex-- {
			target := &targets[targetIndex]
			if !inputTargetVisibleInCamera(target, camera, defaultUICamera) {
				continue
			}
			x, y := screenX, screenY
			if !target.ui {
				if !worldCoordsReady {
					worldX, worldY, worldCoordsValid = inputCoordsForCamera2D(camera, false, screenX, screenY)
					worldCoordsReady = true
				}
				if !worldCoordsValid {
					continue
				}
				x, y = worldX, worldY
			}
			if !inputTargetContains2D(target, x, y) {
				continue
			}
			if target.blocked {
				return *target, x, y, false
			}
			picked := *target
			picked.camera = camera.entity
			return picked, x, y, true
		}
	}
	return inputPickTarget2D{}, 0, 0, false
}

func targetByEntity2D(cameras []inputCamera2D, defaultUICamera ecs.Entity, targets []inputPickTarget2D, entity, cameraEntity ecs.Entity, screenX, screenY float32) (inputPickTarget2D, float32, float32, bool) {
	var camera inputCamera2D
	foundCamera := false
	for _, candidate := range cameras {
		if candidate.entity == cameraEntity {
			camera, foundCamera = candidate, true
			break
		}
	}
	if !foundCamera {
		return inputPickTarget2D{}, 0, 0, false
	}
	for i := range targets {
		target := &targets[i]
		if target.entity != entity || !inputTargetVisibleInCamera(target, camera, defaultUICamera) {
			continue
		}
		x, y, inViewport := inputCoordsForCamera2D(camera, target.ui, screenX, screenY)
		picked := *target
		picked.camera = camera.entity
		return picked, x, y, inViewport
	}
	return inputPickTarget2D{}, 0, 0, false
}

func inputTargetVisibleInCamera(target *inputPickTarget2D, camera inputCamera2D, defaultUICamera ecs.Entity) bool {
	if !camera.layers.Intersects(target.layers) {
		return false
	}
	if !target.ui {
		return camera.pipeline == CameraPipeline2D
	}
	targetCamera := target.target
	if targetCamera == 0 {
		targetCamera = defaultUICamera
	}
	return targetCamera == camera.entity
}

func inputCoordsForCamera2D(camera inputCamera2D, ui bool, screenX, screenY float32) (float32, float32, bool) {
	viewport := camera.camera.ResolvedViewport()
	if screenX < viewport.X || screenY < viewport.Y || screenX > viewport.X+viewport.Width || screenY > viewport.Y+viewport.Height {
		return 0, 0, false
	}
	if ui {
		return screenX, screenY, true
	}
	worldPoint, ok := camera.camera.ViewportToWorld2D(camera.transform, camera.projection, mgl32.Vec2{screenX, screenY})
	if !ok {
		return 0, 0, false
	}
	return worldPoint.X(), worldPoint.Y(), true
}

// Prepare geometry once per scene change, instead of inverting every candidate
// for every pointer event. Keep the exact 4D test, including boundary rounding.
func (target *inputPickTarget2D) prepare(matrix mgl32.Mat4) {
	base := quadBaseTransform(matrix, target.anchor, target.width, target.height, target.ui)
	model := base.Mul4(mgl32.Scale3D(target.width, target.height, 1))
	if math.Abs(float64(model.Det())) <= 1e-8 {
		target.inverse = [9]float32{}
		return
	}
	inverse := model.Inv()
	target.inverse = [9]float32{inverse[0], inverse[1], inverse[3], inverse[4], inverse[5], inverse[7], inverse[12], inverse[13], inverse[15]}
}

func inputTargetContains2D(target *inputPickTarget2D, x, y float32) bool {
	if target.clip.valid && (x < target.clip.minX || x > target.clip.maxX || y < target.clip.minY || y > target.clip.maxY) {
		return false
	}
	inverse := target.inverse
	local := mgl32.Vec4{inverse[0]*x + inverse[3]*y + inverse[6], inverse[1]*x + inverse[4]*y + inverse[7], 0, inverse[2]*x + inverse[5]*y + inverse[8]}
	if local.W() == 0 {
		return false
	}
	local = local.Mul(1 / local.W())
	return local.X() >= 0 && local.X() <= 1 && local.Y() >= 0 && local.Y() <= 1
}

func inputDimensions2D(world *ecs.World, entity ecs.Entity, target *InputTarget2D) (float32, float32) {
	if target.Width > 0 && target.Height > 0 {
		return target.Width, target.Height
	}
	var width, height float32
	if sprite := world.GetRef[Sprite2D](entity); sprite != nil {
		width, height = sprite.Size()
	}
	if layers := world.GetRef[SpriteLayers2D](entity); layers != nil {
		for index := range layers.Layers {
			layer := &layers.Layers[index]
			if !layer.Visible || layer.Sprite.Alpha <= 0 {
				continue
			}
			layerWidth, layerHeight := layer.Sprite.Size()
			width, height = max(width, layerWidth), max(height, layerHeight)
		}
	}
	return width, height
}
