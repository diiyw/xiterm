package scene

import (
	"math"

	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/input"
	"github.com/go-gl/mathgl/mgl32"
)

const defaultUIScrollStep = 10

// UIScroll stores allocation-free scrolling state for an absolute-positioned
// UI viewport. Position is the non-positive offset applied by the caller to
// its content transform.
type UIScroll struct {
	Position     mgl32.Vec2
	Momentum     mgl32.Vec2
	ViewportSize mgl32.Vec2
	ContentSize  mgl32.Vec2
	WheelStep    float32
	// ScrollbarSize enables native scrollbar-thumb dragging for this viewport.
	// A zero value keeps the legacy content-drag behavior for bare UIScroll
	// components that do not render scrollbar parts.
	ScrollbarSize         float32
	ScrollbarMinThumbSize float32
	ScrollbarThin         bool
	// Overlay bars lie inside the viewport without reserving content space.
	ScrollbarOverlay bool
	DisableInput     bool
	DisableDrag      bool
	// DisableContentDrag leaves left-button gestures to text selection while
	// preserving scrollbar dragging, the wheel and middle-button panning.
	DisableContentDrag bool
	ConsumeWheel       bool
	Horizontal         bool
	Vertical           bool

	pointerOrigin mgl32.Vec2
	scrollOrigin  mgl32.Vec2
	momentumTime  float32
	dragging      bool
	dragAxis      uint8
	initialized   bool
}

const (
	uiScrollDragContent uint8 = iota
	uiScrollDragHorizontal
	uiScrollDragVertical
	// uiScrollDragPan is the middle-button gesture: content dragging that skips the
	// scrollbar probe and releases without momentum.
	uiScrollDragPan
)

type UIScrollContainerData struct {
	Scroll       *UIScroll
	ViewportSize mgl32.Vec2
	ContentSize  mgl32.Vec2
	Horizontal   bool
	Vertical     bool
}

func UIScrollSystem(name string) ecs.SystemConfig {
	if name == "" {
		name = "ui_scroll"
	}
	return ecs.SystemConfig{Name: name, System: updateUIScroll}
}

func GetUIScrollContainerData(world *ecs.World, entity ecs.Entity) (UIScrollContainerData, bool) {
	scroll := world.GetMut[UIScroll](entity)
	if scroll == nil {
		return UIScrollContainerData{}, false
	}
	viewport := scroll.ViewportSize
	if viewport == (mgl32.Vec2{}) {
		if size, exists := world.Get[Size2D](entity); exists {
			viewport = mgl32.Vec2{size.Width, size.Height}
		}
	}
	return UIScrollContainerData{
		Scroll: scroll, ViewportSize: viewport, ContentSize: scroll.ContentSize,
		Horizontal: scroll.Horizontal, Vertical: scroll.Vertical,
	}, true
}

func GetUIScrollOffset(world *ecs.World, entity ecs.Entity) (mgl32.Vec2, bool) {
	scroll, ok := world.Get[UIScroll](entity)
	if !ok {
		return mgl32.Vec2{}, false
	}
	return scroll.Position, true
}

type uiScrollCandidate struct {
	entity   ecs.Entity
	depth    int
	stack    UIStackIndex
	captured bool
}

func updateUIScroll(context *ecs.SystemContext) error {
	world := context.World
	if world == nil {
		return nil
	}
	inputState, ok := world.GetResource[*input.State]()
	if !ok || inputState == nil {
		return nil
	}
	pointer := inputState.Pointer()
	deltaTime := float32(0)
	if clock, exists := world.GetResource[*Time](); exists {
		deltaTime = clock.DeltaSeconds()
	}
	scale := uiScrollScaleFactor(world)
	request, _ := world.GetResource[*RenderRequest]()
	wheel := pointer.WheelX != 0 || pointer.WheelY != 0
	panHeld := inputState.MouseHeld(input.MouseMiddle)
	selected := uiScrollCandidate{}

	for entity, access := range world.Query2[UIScroll, Visibility]().All {
		scroll := access.A.Ref()
		visibility := access.B.Ref()

		if scroll == nil || visibility == nil || !visibility.Visible {
			continue
		}
		previous := scroll.Position
		var size Size2D
		if authoredSize, exists := world.Get[Size2D](entity); exists {
			size = authoredSize
		}
		viewport := scroll.ViewportSize
		if viewport == (mgl32.Vec2{}) {
			viewport = mgl32.Vec2{size.Width, size.Height}
		}
		content := scroll.ContentSize
		if !scroll.initialized {
			scroll.initialized = true
		}
		if scroll.DisableInput || (scroll.DisableDrag && scroll.dragAxis != uiScrollDragPan) {
			scroll.Momentum = mgl32.Vec2{}
			scroll.dragging, scroll.dragAxis = false, uiScrollDragContent
		}
		held := pointer.Down
		if scroll.dragAxis == uiScrollDragPan {
			held = panHeld
		}
		if (!held || scroll.DisableInput) && scroll.dragging {
			if scroll.dragAxis == uiScrollDragContent {
				diff := scroll.Position.Sub(scroll.scrollOrigin)
				if scroll.momentumTime > 0 {
					if absUIFloat(diff.X()) > 10 {
						scroll.Momentum[0] = diff.X() / (scroll.momentumTime * 25)
					}
					if absUIFloat(diff.Y()) > 10 {
						scroll.Momentum[1] = diff.Y() / (scroll.momentumTime * 25)
					}
				}
			}
			scroll.pointerOrigin, scroll.scrollOrigin = mgl32.Vec2{}, mgl32.Vec2{}
			scroll.momentumTime, scroll.dragging, scroll.dragAxis = 0, false, uiScrollDragContent
		}
		scroll.Position = scroll.Position.Add(scroll.Momentum)
		scroll.Momentum = scroll.Momentum.Mul(.95)
		for axis := range 2 {
			if absUIFloat(scroll.Momentum[axis]) < .1 || wheel {
				scroll.Momentum[axis] = 0
			}
		}
		clampUIScroll(scroll, viewport, content)
		if scroll.Position != previous {
			request.Request()
		}
		canX := scroll.Horizontal && content.X() > viewport.X()
		canY := scroll.Vertical && content.Y() > viewport.Y()
		acceptsInput := canX || canY || scroll.ConsumeWheel && wheel
		captured := scroll.dragging && (pointer.Down || scroll.dragAxis == uiScrollDragPan)
		width, height := size.Width, size.Height
		if width <= 0 {
			width = viewport.X()
		}
		if height <= 0 {
			height = viewport.Y()
		}
		if !scroll.DisableInput && acceptsInput && (captured || pointInUIScrollContainer(world, entity, width, height, scale, pointer.X, pointer.Y)) {
			stack, _ := world.Get[UIStackIndex](entity)
			candidate := uiScrollCandidate{entity: entity, depth: uiScrollDepth(world, entity), stack: stack, captured: captured}
			if selected.entity == 0 || candidate.captured && !selected.captured || candidate.captured == selected.captured &&
				(candidate.stack > selected.stack || candidate.stack == selected.stack &&
					(candidate.depth > selected.depth || candidate.depth == selected.depth && candidate.entity > selected.entity)) {
				selected = candidate
			}
		}
	}

	if selected.entity == 0 {
		return nil
	}
	scroll := world.GetMut[UIScroll](selected.entity)
	previous := scroll.Position
	size, _ := world.Get[Size2D](selected.entity)
	viewport := scroll.ViewportSize
	if viewport == (mgl32.Vec2{}) {
		viewport = mgl32.Vec2{size.Width, size.Height}
	}
	content := scroll.ContentSize
	canX := scroll.Horizontal && content.X() > viewport.X()
	canY := scroll.Vertical && content.Y() > viewport.Y()
	step := scroll.WheelStep
	if step <= 0 {
		step = defaultUIScrollStep
	}
	wheelMoved := false
	if wheel {
		// Chromium chains an axis its target cannot absorb to the nearest
		// scrollable ancestor, so nested overflow containers keep both axes live
		// instead of letting the innermost one swallow the axis it cannot move.
		for axis, delta := range [2]float32{pointer.WheelX, pointer.WheelY} {
			target := selected.entity
			if !selected.captured {
				target = uiScrollWheelTarget(world, selected.entity, axis)
			}
			wheelMoved = moveUIScrollAxis(world, target, axis, delta, step) || wheelMoved
		}
		if wheelMoved || scroll.ConsumeWheel {
			inputState.ConsumePointerWheel()
		}
	}
	pan := panHeld && !scroll.DisableInput
	if (pointer.Down && !scroll.DisableDrag) || pan {
		position := mgl32.Vec2{pointer.X, pointer.Y}
		if pointer.Pressed && !pan && scroll.ScrollbarSize > 0 {
			if localX, localY, ok := uiScrollPointerLocal(world, selected.entity, size.Width, size.Height, scale, pointer.X, pointer.Y); ok {
				scroll.dragAxis = uiScrollScrollbarAxis(scroll, viewport, content, size.Width, size.Height, localX, localY)
				if scroll.dragAxis != uiScrollDragContent {
					scroll.pointerOrigin, scroll.scrollOrigin, scroll.dragging = position, scroll.Position, true
					scroll.Momentum = mgl32.Vec2{}
				}
			}
		}
		if !scroll.dragging && (pan || !scroll.DisableContentDrag) {
			axis := uiScrollDragContent
			if pan {
				axis = uiScrollDragPan
			}
			scroll.pointerOrigin, scroll.scrollOrigin, scroll.dragging, scroll.dragAxis = position, scroll.Position, true, axis
		} else if scroll.dragging {
			old := scroll.Position
			drag := position.Sub(scroll.pointerOrigin).Mul(1 / scale)
			switch scroll.dragAxis {
			case uiScrollDragHorizontal:
				track := UIScrollTrackSize(scroll, viewport).X()
				thumb := ScrollThumbSize(track, viewport.X(), content.X(), scroll.ScrollbarMinThumbSize)
				scroll.Position[0] = scroll.scrollOrigin.X() - drag.X()*scrollMaxThumbRatio(track, thumb, content.X()-viewport.X())
			case uiScrollDragVertical:
				track := UIScrollTrackSize(scroll, viewport).Y()
				thumb := ScrollThumbSize(track, viewport.Y(), content.Y(), scroll.ScrollbarMinThumbSize)
				scroll.Position[1] = scroll.scrollOrigin.Y() - drag.Y()*scrollMaxThumbRatio(track, thumb, content.Y()-viewport.Y())
			default:
				if canX {
					scroll.Position[0] = scroll.scrollOrigin.X() + drag.X()
				}
				if canY {
					scroll.Position[1] = scroll.scrollOrigin.Y() + drag.Y()
				}
			}
			clampUIScroll(scroll, viewport, content)
			moved := scroll.Position.Sub(old)
			if scroll.dragAxis != uiScrollDragContent {
				scroll.momentumTime = 0
			} else if absUIFloat(moved.X()) < .1 && absUIFloat(moved.Y()) < .1 && scroll.momentumTime > .15 {
				scroll.pointerOrigin, scroll.scrollOrigin = position, scroll.Position
				scroll.momentumTime = 0
			} else {
				scroll.momentumTime += deltaTime
			}
		}
	}
	clampUIScroll(scroll, viewport, content)
	if scroll.Position != previous || wheelMoved {
		request.Request()
	}
	return nil
}

func uiScrollScaleFactor(world *ecs.World) float32 {
	if viewport, ok := world.GetResource[ViewportSize](); ok && viewport.ScaleFactor > 0 {
		return viewport.ScaleFactor
	}
	return 1
}

func clampUIScroll(scroll *UIScroll, viewport, content mgl32.Vec2) {
	maximum := content.Sub(viewport)
	for axis, enabled := range []bool{scroll.Horizontal, scroll.Vertical} {
		if !enabled {
			scroll.Position[axis], scroll.Momentum[axis] = 0, 0
			continue
		}
		scroll.Position[axis] = min(max(scroll.Position[axis], -max(maximum[axis], 0)), 0)
	}
}

// Wheel axes chain only through the selected container's ancestors. Overlapping
// containers behind a popup must never receive its wheel input.
func uiScrollWheelTarget(world *ecs.World, entity ecs.Entity, axis int) ecs.Entity {
	for remaining := world.EntityCount(); entity != 0 && remaining > 0; remaining-- {
		if scroll := world.GetRef[UIScroll](entity); scroll != nil && !scroll.DisableInput {
			viewport := scroll.ViewportSize
			if viewport == (mgl32.Vec2{}) {
				if size, ok := world.Get[Size2D](entity); ok {
					viewport = mgl32.Vec2{size.Width, size.Height}
				}
			}
			enabled := scroll.Horizontal
			if axis == 1 {
				enabled = scroll.Vertical
			}
			if enabled && scroll.ContentSize[axis] > viewport[axis] {
				return entity
			}
		}
		parent, ok := world.Get[Parent](entity)
		if !ok {
			break
		}
		entity = parent.Entity
	}
	return 0
}

// moveUIScrollAxis applies one wheel axis to a container and reports whether the
// offset moved.
func moveUIScrollAxis(world *ecs.World, entity ecs.Entity, axis int, delta, step float32) bool {
	if entity == 0 || delta == 0 {
		return false
	}
	scroll := world.GetMut[UIScroll](entity)
	if scroll == nil {
		return false
	}
	viewport := scroll.ViewportSize
	if viewport == (mgl32.Vec2{}) {
		if size, exists := world.Get[Size2D](entity); exists {
			viewport = mgl32.Vec2{size.Width, size.Height}
		}
	}
	before := scroll.Position
	scroll.Position[axis] += delta * step
	clampUIScroll(scroll, viewport, scroll.ContentSize)
	return scroll.Position != before
}

func uiScrollPointerLocal(world *ecs.World, entity ecs.Entity, width, height, scale, x, y float32) (float32, float32, bool) {
	if transform, ok := world.Get[UIGlobalTransform](entity); ok && transform.Matrix != (mgl32.Mat4{}) {
		base := quadBaseTransform(transform.Matrix, anchorForEntity2D(world, entity), width, height, true)
		if absUIFloat(base.Det()) <= 1e-8 {
			return 0, 0, false
		}
		local := base.Inv().Mul4x1(mgl32.Vec4{x, y, 0, 1})
		if local.W() == 0 {
			return 0, 0, false
		}
		local = local.Mul(1 / local.W())
		return local.X(), local.Y(), true
	}
	if transform, ok := world.Get[UITransform](entity); ok {
		if scale <= 0 {
			scale = 1
		}
		return (x - transform.Translation.X.Value) / scale, (y - transform.Translation.Y.Value) / scale, true
	}
	if scale <= 0 {
		scale = 1
	}
	return x / scale, y / scale, true
}

// UIScrollTrackSize excludes the shared corner when both overlay bars are visible.
func UIScrollTrackSize(scroll *UIScroll, viewport mgl32.Vec2) mgl32.Vec2 {
	if scroll.ScrollbarOverlay {
		if scroll.Vertical {
			viewport[0] = max(viewport[0]-scroll.ScrollbarSize, 0)
		}
		if scroll.Horizontal {
			viewport[1] = max(viewport[1]-scroll.ScrollbarSize, 0)
		}
	}
	return viewport
}

func uiScrollScrollbarAxis(scroll *UIScroll, viewport, content mgl32.Vec2, width, height, x, y float32) uint8 {
	if scroll == nil || scroll.ScrollbarSize <= 0 {
		return uiScrollDragContent
	}
	bar := scroll.ScrollbarSize
	track := UIScrollTrackSize(scroll, viewport)
	width, height = max(width, viewport.X()), max(height, viewport.Y())
	if scroll.Vertical && content.Y() > viewport.Y() && x >= width-bar && x <= width && y >= 0 && y <= track.Y() {
		return uiScrollDragVertical
	}
	if scroll.Horizontal && content.X() > viewport.X() && y >= height-bar && y <= height && x >= 0 && x <= track.X() {
		return uiScrollDragHorizontal
	}
	return uiScrollDragContent
}

// UIScrollScrollbarActive keeps a thumb expanded while hovered or captured.
// axis is 0 for horizontal and 1 for vertical.
func UIScrollScrollbarActive(world *ecs.World, entity ecs.Entity, axis uint8) bool {
	scroll := world.GetRef[UIScroll](entity)
	if scroll == nil {
		return false
	}
	want := uiScrollDragHorizontal + axis
	if scroll.dragging && scroll.dragAxis == want {
		return true
	}
	state, ok := world.GetResource[*input.State]()
	if !ok || state == nil {
		return false
	}
	size, _ := world.Get[Size2D](entity)
	pointer := state.Pointer()
	if !pointInUIScrollContainer(world, entity, size.Width, size.Height, uiScrollScaleFactor(world), pointer.X, pointer.Y) {
		return false
	}
	x, y, ok := uiScrollPointerLocal(world, entity, size.Width, size.Height, uiScrollScaleFactor(world), pointer.X, pointer.Y)
	return ok && uiScrollScrollbarAxis(scroll, scroll.ViewportSize, scroll.ContentSize, size.Width, size.Height, x, y) == want
}

// ScrollThumbSize resolves the proportional scrollbar thumb within its track.
func ScrollThumbSize(trackSize, viewportSize, contentSize, minThumbSize float32) float32 {
	if trackSize <= 0 {
		return 0
	}
	if contentSize <= 0 || viewportSize <= 0 || contentSize <= viewportSize {
		return trackSize
	}
	if minThumbSize <= 0 {
		minThumbSize = 24
	}
	return min(max(trackSize*(viewportSize/contentSize), minThumbSize), trackSize)
}

func scrollMaxThumbRatio(trackSize, thumbSize, maxScroll float32) float32 {
	if maxScroll <= 0 || trackSize <= thumbSize {
		return 0
	}
	return maxScroll / (trackSize - thumbSize)
}

func pointInUIScrollContainer(world *ecs.World, entity ecs.Entity, width, height, scale, x, y float32) bool {
	transform, ok := world.Get[UIGlobalTransform](entity)
	var minX, minY, maxX, maxY float32
	valid := false
	if ok && transform.Matrix != (mgl32.Mat4{}) {
		minX, minY, maxX, maxY, valid = boundsFromBase2D(transform.Matrix, width, height)
	} else if authored, exists := world.Get[UITransform](entity); exists {
		minX, minY, maxX, maxY = authored.Translation.X.Value, authored.Translation.Y.Value,
			authored.Translation.X.Value+width, authored.Translation.Y.Value+height
		valid = true
	}
	if !valid || float32Invalid(x) || float32Invalid(y) || x < minX || x > maxX || y < minY || y > maxY {
		return false
	}
	if ok {
		// Resolved clipping already accounts for ClipExempt2D popup surfaces.
		clip := transform.renderClip2D()
		if clip.valid && (x < clip.minX || x > clip.maxX || y < clip.minY || y > clip.maxY) {
			// Classic bars sit outside their own content clip. They still
			// inherit the enclosing container's clip, just like their visuals.
			scroll := world.GetRef[UIScroll](entity)
			if scroll == nil || scroll.ScrollbarOverlay {
				return false
			}
			localX, localY, local := uiScrollPointerLocal(world, entity, width, height, scale, x, y)
			if !local || uiScrollScrollbarAxis(scroll, scroll.ViewportSize, scroll.ContentSize, width, height, localX, localY) == uiScrollDragContent {
				return false
			}
			parent, _ := world.Get[Parent](entity)
			if ancestor, exists := world.Get[UIGlobalTransform](parent.Entity); exists {
				clip = ancestor.renderClip2D()
				return !clip.valid || x >= clip.minX && x <= clip.maxX && y >= clip.minY && y <= clip.maxY
			}
		}
	}
	return true
}

func float32Invalid(value float32) bool {
	return math.IsNaN(float64(value)) || math.IsInf(float64(value), 0)
}

func absUIFloat(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

func uiScrollDepth(world *ecs.World, entity ecs.Entity) int {
	depth := 0
	for remaining := world.EntityCount(); remaining > 0; remaining-- {
		parent, ok := world.Get[Parent](entity)
		if !ok || parent.Entity == 0 {
			break
		}
		depth++
		entity = parent.Entity
	}
	return depth
}
