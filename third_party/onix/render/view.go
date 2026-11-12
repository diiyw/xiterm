package render

type RenderViewMode uint8

const (
	RenderViewLit RenderViewMode = iota
	RenderViewUnlit
	RenderViewWireframe
	RenderViewPicking
)

// RenderView binds typed camera data to an optional output target. The camera
// payload remains owned by the caller, so render does not depend on ECS types.
type RenderView[T any] struct {
	Camera   T
	Target   *RenderTarget
	Viewport Rect
	Clear    RenderTargetClear
	Order    int
	Mode     RenderViewMode
	Enabled  bool
}

func NewRenderView[T any](camera T) RenderView[T] {
	return RenderView[T]{
		Camera:  camera,
		Clear:   RenderTargetClear{Depth: 0, ClearColor: true, ClearDepth: true},
		Mode:    RenderViewLit,
		Enabled: true,
	}
}

func (view RenderView[T]) Size(fallback Rect) (int, int) {
	if view.Target != nil {
		return view.Target.Size()
	}
	viewport := view.Viewport
	if viewport.Empty() {
		viewport = fallback
	}
	return int(viewport.Width), int(viewport.Height)
}
