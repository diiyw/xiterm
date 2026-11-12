package window

import "math"

type ViewportMode string

const (
	ViewportModeContain     ViewportMode = "contain"
	ViewportModeCover       ViewportMode = "cover"
	ViewportModeStretch     ViewportMode = "stretch"
	ViewportModeFixedWidth  ViewportMode = "fixed-width"
	ViewportModeFixedHeight ViewportMode = "fixed-height"
	ViewportModeExpand      ViewportMode = "expand"
	ViewportModeAdaptive    ViewportMode = "adaptive"
	ViewportModeInteger     ViewportMode = "integer-scale"
)

type viewportRect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

type viewportLayout struct {
	RenderViewport viewportRect
	InputViewport  viewportRect
	ViewWidth      int
	ViewHeight     int
}

type viewportState struct {
	mode       ViewportMode
	layout     viewportLayout
	baseWidth  int
	baseHeight int
}

func newViewportState(width, height int) viewportState {
	state := viewportState{}
	state.SetViewport(width, height)
	return state
}

func (s *viewportState) SetViewport(width, height int) {
	s.baseWidth = width
	s.baseHeight = height
	if s.layout.ViewWidth <= 0 {
		s.layout.ViewWidth = width
	}
	if s.layout.ViewHeight <= 0 {
		s.layout.ViewHeight = height
	}
}

func (s *viewportState) SetMode(mode ViewportMode) {
	s.mode = normalizeViewportMode(mode)
}

func (s *viewportState) Update(renderSurfaceWidth, renderSurfaceHeight, inputSurfaceWidth, inputSurfaceHeight float64) (viewportLayout, bool) {
	layout, ok := computeViewportLayout(s.baseWidth, s.baseHeight, s.mode, renderSurfaceWidth, renderSurfaceHeight, inputSurfaceWidth, inputSurfaceHeight)
	if !ok {
		return viewportLayout{}, false
	}
	s.layout = layout
	return layout, true
}

func (s viewportState) ViewSize() (int, int) {
	if s.layout.ViewWidth > 0 && s.layout.ViewHeight > 0 {
		return s.layout.ViewWidth, s.layout.ViewHeight
	}
	return s.baseWidth, s.baseHeight
}

func (s viewportState) LogicalXY(x, y float64) (float64, float64) {
	return logicalXYFromViewport(x, y, s.layout.ViewWidth, s.layout.ViewHeight, s.layout.InputViewport)
}

func (s viewportState) InputRect(x, y, width, height float64) (viewportRect, bool) {
	viewport := s.layout.InputViewport
	viewWidth, viewHeight := s.ViewSize()
	if viewWidth <= 0 || viewHeight <= 0 || viewport.Width <= 0 || viewport.Height <= 0 {
		return viewportRect{}, false
	}
	scaleX := viewport.Width / float64(viewWidth)
	scaleY := viewport.Height / float64(viewHeight)
	return viewportRect{
		X:      viewport.X + x*scaleX,
		Y:      viewport.Y + y*scaleY,
		Width:  width * scaleX,
		Height: height * scaleY,
	}, true
}

func normalizeViewportMode(mode ViewportMode) ViewportMode {
	switch mode {
	case "", ViewportModeContain:
		return ViewportModeContain
	case ViewportModeCover,
		ViewportModeStretch,
		ViewportModeFixedWidth,
		ViewportModeFixedHeight,
		ViewportModeExpand,
		ViewportModeAdaptive,
		ViewportModeInteger:
		return mode
	default:
		return ViewportModeContain
	}
}

func computeViewportLayout(baseWidth, baseHeight int, mode ViewportMode, renderSurfaceWidth, renderSurfaceHeight, inputSurfaceWidth, inputSurfaceHeight float64) (viewportLayout, bool) {
	if baseWidth <= 0 || baseHeight <= 0 || renderSurfaceWidth <= 0 || renderSurfaceHeight <= 0 || inputSurfaceWidth <= 0 || inputSurfaceHeight <= 0 {
		return viewportLayout{}, false
	}

	mode = normalizeViewportMode(mode)
	layout := viewportLayout{}

	switch mode {
	case ViewportModeContain:
		layout.ViewWidth = baseWidth
		layout.ViewHeight = baseHeight
		layout.RenderViewport = fitViewportRect(baseWidth, baseHeight, renderSurfaceWidth, renderSurfaceHeight, false)
	case ViewportModeCover:
		layout.ViewWidth = baseWidth
		layout.ViewHeight = baseHeight
		layout.RenderViewport = fitViewportRect(baseWidth, baseHeight, renderSurfaceWidth, renderSurfaceHeight, true)
	case ViewportModeStretch:
		layout.ViewWidth = baseWidth
		layout.ViewHeight = baseHeight
		layout.RenderViewport = fullViewportRect(renderSurfaceWidth, renderSurfaceHeight)
	case ViewportModeFixedWidth:
		layout.ViewWidth = baseWidth
		layout.ViewHeight = max(1, roundedViewportSize(float64(baseWidth)*renderSurfaceHeight/renderSurfaceWidth))
		layout.RenderViewport = fullViewportRect(renderSurfaceWidth, renderSurfaceHeight)
	case ViewportModeFixedHeight:
		layout.ViewHeight = baseHeight
		layout.ViewWidth = max(1, roundedViewportSize(float64(baseHeight)*renderSurfaceWidth/renderSurfaceHeight))
		layout.RenderViewport = fullViewportRect(renderSurfaceWidth, renderSurfaceHeight)
	case ViewportModeExpand:
		scale := min(renderSurfaceWidth/float64(baseWidth), renderSurfaceHeight/float64(baseHeight))
		if scale <= 0 {
			return viewportLayout{}, false
		}
		layout.ViewWidth = max(baseWidth, roundedViewportSize(renderSurfaceWidth/scale))
		layout.ViewHeight = max(baseHeight, roundedViewportSize(renderSurfaceHeight/scale))
		layout.RenderViewport = fullViewportRect(renderSurfaceWidth, renderSurfaceHeight)
	case ViewportModeAdaptive:
		layout.ViewWidth = max(baseWidth, roundedViewportSize(inputSurfaceWidth))
		layout.ViewHeight = max(baseHeight, roundedViewportSize(inputSurfaceHeight))
		layout.RenderViewport = fullViewportRect(renderSurfaceWidth, renderSurfaceHeight)
	case ViewportModeInteger:
		scale := math.Floor(min(renderSurfaceWidth/float64(baseWidth), renderSurfaceHeight/float64(baseHeight)))
		if scale < 1 {
			scale = 1
		}
		layout.ViewWidth = baseWidth
		layout.ViewHeight = baseHeight
		layout.RenderViewport = centeredViewportRect(float64(baseWidth)*scale, float64(baseHeight)*scale, renderSurfaceWidth, renderSurfaceHeight)
	}

	layout.InputViewport = scaleViewportRect(
		layout.RenderViewport,
		renderSurfaceWidth,
		renderSurfaceHeight,
		inputSurfaceWidth,
		inputSurfaceHeight,
	)
	return layout, true
}

func fitViewportRect(viewWidth, viewHeight int, surfaceWidth, surfaceHeight float64, cover bool) viewportRect {
	targetAspect := float64(viewWidth) / float64(viewHeight)
	surfaceAspect := surfaceWidth / surfaceHeight

	if cover {
		if surfaceAspect >= targetAspect {
			return centeredViewportRect(surfaceWidth, surfaceWidth/targetAspect, surfaceWidth, surfaceHeight)
		}
		return centeredViewportRect(surfaceHeight*targetAspect, surfaceHeight, surfaceWidth, surfaceHeight)
	}

	if surfaceAspect >= targetAspect {
		return centeredViewportRect(surfaceHeight*targetAspect, surfaceHeight, surfaceWidth, surfaceHeight)
	}
	return centeredViewportRect(surfaceWidth, surfaceWidth/targetAspect, surfaceWidth, surfaceHeight)
}

func fullViewportRect(surfaceWidth, surfaceHeight float64) viewportRect {
	return viewportRect{
		Width:  surfaceWidth,
		Height: surfaceHeight,
	}
}

func centeredViewportRect(width, height, surfaceWidth, surfaceHeight float64) viewportRect {
	return viewportRect{
		X:      (surfaceWidth - width) * 0.5,
		Y:      (surfaceHeight - height) * 0.5,
		Width:  width,
		Height: height,
	}
}

func scaleViewportRect(rect viewportRect, fromWidth, fromHeight, toWidth, toHeight float64) viewportRect {
	if fromWidth <= 0 || fromHeight <= 0 || toWidth <= 0 || toHeight <= 0 {
		return viewportRect{}
	}

	scaleX := toWidth / fromWidth
	scaleY := toHeight / fromHeight
	return viewportRect{
		X:      rect.X * scaleX,
		Y:      rect.Y * scaleY,
		Width:  rect.Width * scaleX,
		Height: rect.Height * scaleY,
	}
}

func logicalXYFromViewport(x, y float64, viewWidth, viewHeight int, viewport viewportRect) (float64, float64) {
	if viewWidth == 0 || viewHeight == 0 || viewport.Width <= 0 || viewport.Height <= 0 {
		return x, y
	}

	x = (x - viewport.X) * float64(viewWidth) / viewport.Width
	y = (y - viewport.Y) * float64(viewHeight) / viewport.Height
	return x, y
}

func roundedViewportSize(value float64) int {
	return int(math.Round(value))
}

func normalizeDevicePixelRatio(value float64) float64 {
	if value <= 0 {
		return 1
	}
	return value
}
