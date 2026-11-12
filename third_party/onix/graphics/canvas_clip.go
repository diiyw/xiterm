package graphics

type clipRect struct {
	x float32
	y float32
	w float32
	h float32
}

func intersectClipRect(a, b clipRect) clipRect {
	x0 := maxFloat32(a.x, b.x)
	y0 := maxFloat32(a.y, b.y)
	x1 := minFloat32(a.x+a.w, b.x+b.w)
	y1 := minFloat32(a.y+a.h, b.y+b.h)
	if x1 < x0 {
		x1 = x0
	}
	if y1 < y0 {
		y1 = y0
	}
	return clipRect{x: x0, y: y0, w: x1 - x0, h: y1 - y0}
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func minFloat32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
