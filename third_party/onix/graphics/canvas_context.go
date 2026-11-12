package graphics

// CurrentContext is the immediate-mode canvas. ECS systems use CanvasFor(world).
func CurrentContext() *Canvas2D {
	return canvas2DContext
}

// SetCurrentContext selects an immediate canvas on the rendering thread.
func SetCurrentContext(ctx *Canvas2D) { canvas2DContext = ctx }

func SetCurrentContextForTesting(ctx *Canvas2D) func() {
	prev := canvas2DContext
	canvas2DContext = ctx
	return func() {
		canvas2DContext = prev
	}
}
