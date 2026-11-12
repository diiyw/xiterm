package graphics

import "math"

// The distance shader shades both sides of the contour. Give it a half-pixel
// fringe outside the quad too; otherwise triangle rasterization discards the
// outer half of the coverage ramp, producing stair steps on rotated edges.
func (c *Canvas2D) shapeCoverageQuad(model, local [6]float32) ([6]float32, [6]float32) {
	// LocalModel describes the surviving rectangle after geometric clipping.
	// Never expand an edge introduced by clipping.
	if local[1] != 0 || local[3] != 0 || local[0] <= 0 || local[4] <= 0 || c.width <= 0 || c.height <= 0 {
		return model, local
	}
	driver := c.device.Driver()
	if driver == nil {
		return model, local
	}
	viewport := driver.Viewport()
	sx, sy := float64(viewport.Width)/float64(c.width), float64(viewport.Height)/float64(c.height)
	a, b := float64(model[0])*sx, float64(model[1])*sx
	d, e := float64(model[3])*sy, float64(model[4])*sy
	if c.projected {
		p := c.projection
		// Canvas shapes use affine projections; a varying homogeneous W cannot
		// be represented by a single screen-space pixel footprint.
		if p[3] != 0 || p[7] != 0 || p[15] == 0 {
			return model, local
		}
		sx, sy = float64(viewport.Width)*.5/float64(p[15]), float64(viewport.Height)*.5/float64(p[15])
		a = (float64(p[0])*float64(model[0]) + float64(p[4])*float64(model[3])) * sx
		b = (float64(p[0])*float64(model[1]) + float64(p[4])*float64(model[4])) * sx
		d = (float64(p[1])*float64(model[0]) + float64(p[5])*float64(model[3])) * sy
		e = (float64(p[1])*float64(model[1]) + float64(p[5])*float64(model[4])) * sy
	}
	det := math.Abs(a*e - b*d)
	if det == 0 {
		return model, local
	}
	// Inverse screen Jacobian maps a pixel square into normalized quad space.
	// Its row sums also handle skew, reflections, zoom and nonuniform scaling.
	px, py := (math.Abs(e)+math.Abs(b))*.5/det, (math.Abs(a)+math.Abs(d))*.5/det
	if math.IsNaN(px) || math.IsNaN(py) || px > math.MaxFloat32 || py > math.MaxFloat32 {
		return model, local
	}
	var left, top, right, bottom float32
	if local[2] == 0 {
		left = float32(px)
	}
	if local[5] == 0 {
		top = float32(py)
	}
	if local[2]+local[0] == 1 {
		right = float32(px)
	}
	if local[5]+local[4] == 1 {
		bottom = float32(py)
	}
	fringe := [6]float32{1 + left + right, 0, -left, 0, 1 + top + bottom, -top}
	return multiplyAffine(model, fringe), multiplyAffine(local, fringe)
}
