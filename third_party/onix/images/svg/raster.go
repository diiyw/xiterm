package svg

import (
	"image"
	"image/color"
	"math"
	"slices"
)

type renderer struct {
	img *image.NRGBA
}

const coverageSampleGrid = 8

func (r *renderer) Fill(path vectorPath, paint fillPaint) {
	if !paint.valid || len(path.subpaths) == 0 {
		return
	}
	if paint.gradient == nil && paint.color.A == 0 {
		return
	}
	bounds := path.Bounds()
	minX, minY, maxX, maxY := r.pixelBounds(bounds, 0)
	if minX >= maxX || minY >= maxY {
		return
	}
	coverage := make([]float64, maxX-minX)
	crossings := make([]fillCrossing, 0, len(path.fillSegments))
	spans := make([]fillSpan, 0, len(path.fillSegments))
	for y := minY; y < maxY; y++ {
		clear(coverage)
		for sy := 0; sy < coverageSampleGrid; sy++ {
			sampleY := float64(y) + (float64(sy)+0.5)/coverageSampleGrid
			spans, crossings = fillSpans(path.fillSegments, sampleY, crossings[:0], spans[:0])
			for _, span := range spans {
				left, right := math.Max(span.minX, float64(minX)), math.Min(span.maxX, float64(maxX))
				for x, end := int(math.Floor(left)), int(math.Ceil(right)); x < end; x++ {
					coverage[x-minX] += math.Min(right, float64(x+1)) - math.Max(left, float64(x))
				}
			}
		}
		for x := minX; x < maxX; x++ {
			value := math.Min(1, coverage[x-minX]/coverageSampleGrid)
			if value == 0 {
				continue
			}
			clr := paint.color
			if paint.gradient != nil {
				// A gradient varies per pixel, so it is sampled at the pixel center.
				if clr = paint.at(float64(x)+0.5, float64(y)+0.5); clr.A == 0 {
					continue
				}
			}
			r.blend(x, y, clr, value)
		}
	}
}

type fillCrossing struct {
	x     float64
	delta int
}

type fillSpan struct {
	minX float64
	maxX float64
}

func fillSpans(segments []pathSegment, y float64, crossings []fillCrossing, spans []fillSpan) ([]fillSpan, []fillCrossing) {
	for _, segment := range segments {
		if segment.a.y <= y {
			if segment.b.y <= y {
				continue
			}
			crossings = append(crossings, fillCrossing{
				x:     segment.a.x + (y-segment.a.y)*segment.dx/segment.dy,
				delta: 1,
			})
			continue
		}
		if segment.b.y > y {
			continue
		}
		crossings = append(crossings, fillCrossing{
			x:     segment.a.x + (y-segment.a.y)*segment.dx/segment.dy,
			delta: -1,
		})
	}
	slices.SortFunc(crossings, func(a, b fillCrossing) int {
		if a.x < b.x {
			return -1
		}
		if a.x > b.x {
			return 1
		}
		return 0
	})
	winding := 0
	var previousX float64
	havePrevious := false
	for index := 0; index < len(crossings); {
		x := crossings[index].x
		if havePrevious && winding != 0 && previousX < x {
			spans = append(spans, fillSpan{minX: previousX, maxX: x})
		}
		delta := 0
		for index < len(crossings) && crossings[index].x == x {
			delta += crossings[index].delta
			index++
		}
		winding -= delta
		previousX = x
		havePrevious = true
	}
	return spans, crossings
}

func (r *renderer) Stroke(path vectorPath, width float64, paint fillPaint, lineCap strokeLineCap, lineJoin strokeLineJoin, miterLimit float64) {
	if !paint.valid || width <= 0 || len(path.subpaths) == 0 {
		return
	}
	if paint.gradient == nil && paint.color.A == 0 {
		return
	}
	radius := width * 0.5
	padding := radius
	if lineJoin == strokeLineJoinMiter {
		padding *= math.Max(1, miterLimit)
	}
	minX, minY, maxX, maxY := r.pixelBounds(path.Bounds(), padding)
	if minX >= maxX || minY >= maxY {
		return
	}
	raster := newStrokeRaster(path, minY, maxY, radius, lineCap, lineJoin, miterLimit)
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			if !strokePixelCandidate(&raster, x, y) {
				continue
			}
			coverage := strokeCoverage(&raster, x, y)
			if coverage == 0 {
				continue
			}
			clr := paint.color
			if paint.gradient != nil {
				if clr = paint.at(float64(x)+0.5, float64(y)+0.5); clr.A == 0 {
					continue
				}
			}
			r.blend(x, y, clr, coverage)
		}
	}
}

func strokePixelCandidate(raster *strokeRaster, x, y int) bool {
	row := y - raster.minY
	if row < 0 || row >= len(raster.segmentRows) {
		return false
	}
	minX, maxX := float64(x), float64(x+1)
	for _, index := range raster.segmentRows[row] {
		segment := raster.segments[index]
		if maxX >= segment.minX-raster.radius && minX <= segment.maxX+raster.radius {
			return true
		}
	}
	for _, index := range raster.primitiveRows[row] {
		bounds := raster.primitives[index].bounds
		if maxX >= bounds.X && minX <= bounds.X+bounds.Width {
			return true
		}
	}
	return false
}

type strokePrimitiveKind uint8

const (
	strokePrimitivePolygon strokePrimitiveKind = iota
	strokePrimitiveCircle
)

type strokePrimitive struct {
	points [4]point
	bounds Rect
	center point
	radius float64
	kind   strokePrimitiveKind
	count  uint8
}

type strokeRaster struct {
	segments      []pathSegment
	primitives    []strokePrimitive
	segmentRows   [][]int
	primitiveRows [][]int
	minY          int
	radius        float64
}

func newStrokeRaster(path vectorPath, minY, maxY int, radius float64, lineCap strokeLineCap, lineJoin strokeLineJoin, miterLimit float64) strokeRaster {
	raster := strokeRaster{
		segments:      path.segments,
		segmentRows:   make([][]int, maxY-minY),
		primitiveRows: make([][]int, maxY-minY),
		minY:          minY,
		radius:        radius,
	}
	for index, segment := range raster.segments {
		if segment.length2 == 0 {
			continue
		}
		bounds := Rect{
			X: segment.minX - radius, Y: segment.minY - radius,
			Width: segment.maxX - segment.minX + radius*2, Height: segment.maxY - segment.minY + radius*2,
		}
		indexStrokeRows(raster.segmentRows, minY, maxY, bounds, index)
	}
	for _, subpath := range path.subpaths {
		points := subpath.points
		if len(points) == 0 {
			continue
		}
		if len(points) == 1 {
			appendPointCap(&raster, points[0], radius, lineCap, minY, maxY)
			continue
		}
		if subpath.closed {
			for index := range points {
				appendStrokeJoin(&raster, points[(index+len(points)-1)%len(points)], points[index], points[(index+1)%len(points)], radius, lineJoin, miterLimit, minY, maxY)
			}
			continue
		}
		for index := 1; index < len(points)-1; index++ {
			appendStrokeJoin(&raster, points[index-1], points[index], points[index+1], radius, lineJoin, miterLimit, minY, maxY)
		}
		appendStrokeCaps(&raster, points, radius, lineCap, minY, maxY)
	}
	return raster
}

func appendStrokeJoin(raster *strokeRaster, previous, vertex, next point, radius float64, lineJoin strokeLineJoin, miterLimit float64, minY, maxY int) {
	incoming, incomingOK := unitDirection(previous, vertex)
	outgoing, outgoingOK := unitDirection(vertex, next)
	if !incomingOK || !outgoingOK {
		return
	}
	turn := cross(incoming, outgoing)
	if math.Abs(turn) < 1e-12 {
		if lineJoin == strokeLineJoinRound && incoming.x*outgoing.x+incoming.y*outgoing.y < 0 {
			appendStrokePrimitive(raster, circleStrokePrimitive(vertex, radius), minY, maxY)
		}
		return
	}
	if lineJoin == strokeLineJoinRound {
		appendStrokePrimitive(raster, circleStrokePrimitive(vertex, radius), minY, maxY)
		return
	}

	side := -1.0
	if turn < 0 {
		side = 1
	}
	firstNormal := point{x: -incoming.y * radius * side, y: incoming.x * radius * side}
	secondNormal := point{x: -outgoing.y * radius * side, y: outgoing.x * radius * side}
	first := point{x: vertex.x + firstNormal.x, y: vertex.y + firstNormal.y}
	second := point{x: vertex.x + secondNormal.x, y: vertex.y + secondNormal.y}
	if lineJoin == strokeLineJoinMiter {
		delta := point{x: second.x - first.x, y: second.y - first.y}
		distance := cross(delta, outgoing) / turn
		miter := point{x: first.x + incoming.x*distance, y: first.y + incoming.y*distance}
		miterLength := math.Hypot(miter.x-vertex.x, miter.y-vertex.y)
		if miterLength-radius <= 1.0/(coverageSampleGrid*8) {
			return
		}
		if miterLength <= radius*miterLimit {
			appendStrokePrimitive(raster, polygonStrokePrimitive(vertex, first, miter, second), minY, maxY)
			return
		}
	}
	appendStrokePrimitive(raster, polygonStrokePrimitive(vertex, first, second), minY, maxY)
}

func appendStrokeCaps(raster *strokeRaster, points []point, radius float64, lineCap strokeLineCap, minY, maxY int) {
	switch lineCap {
	case strokeLineCapRound:
		appendStrokePrimitive(raster, circleStrokePrimitive(points[0], radius), minY, maxY)
		appendStrokePrimitive(raster, circleStrokePrimitive(points[len(points)-1], radius), minY, maxY)
	case strokeLineCapSquare:
		if direction, ok := unitDirection(points[0], points[1]); ok {
			appendStrokePrimitive(raster, squareStrokeCap(points[0], direction, -radius), minY, maxY)
		}
		if direction, ok := unitDirection(points[len(points)-2], points[len(points)-1]); ok {
			appendStrokePrimitive(raster, squareStrokeCap(points[len(points)-1], direction, radius), minY, maxY)
		}
	}
}

func appendPointCap(raster *strokeRaster, center point, radius float64, lineCap strokeLineCap, minY, maxY int) {
	switch lineCap {
	case strokeLineCapRound:
		appendStrokePrimitive(raster, circleStrokePrimitive(center, radius), minY, maxY)
	case strokeLineCapSquare:
		appendStrokePrimitive(raster, polygonStrokePrimitive(
			point{x: center.x - radius, y: center.y - radius},
			point{x: center.x + radius, y: center.y - radius},
			point{x: center.x + radius, y: center.y + radius},
			point{x: center.x - radius, y: center.y + radius},
		), minY, maxY)
	}
}

func squareStrokeCap(vertex, direction point, extension float64) strokePrimitive {
	normal := point{x: -direction.y, y: direction.x}
	outer := point{x: vertex.x + direction.x*extension, y: vertex.y + direction.y*extension}
	radius := math.Abs(extension)
	return polygonStrokePrimitive(
		point{x: vertex.x - normal.x*radius, y: vertex.y - normal.y*radius},
		point{x: outer.x - normal.x*radius, y: outer.y - normal.y*radius},
		point{x: outer.x + normal.x*radius, y: outer.y + normal.y*radius},
		point{x: vertex.x + normal.x*radius, y: vertex.y + normal.y*radius},
	)
}

func circleStrokePrimitive(center point, radius float64) strokePrimitive {
	return strokePrimitive{
		kind: strokePrimitiveCircle, center: center, radius: radius,
		bounds: Rect{X: center.x - radius, Y: center.y - radius, Width: radius * 2, Height: radius * 2},
	}
}

func polygonStrokePrimitive(points ...point) strokePrimitive {
	primitive := strokePrimitive{kind: strokePrimitivePolygon, count: uint8(len(points))}
	copy(primitive.points[:], points)
	minX, minY, maxX, maxY := points[0].x, points[0].y, points[0].x, points[0].y
	for _, value := range points[1:] {
		minX, minY = math.Min(minX, value.x), math.Min(minY, value.y)
		maxX, maxY = math.Max(maxX, value.x), math.Max(maxY, value.y)
	}
	primitive.bounds = Rect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
	return primitive
}

func appendStrokePrimitive(raster *strokeRaster, primitive strokePrimitive, minY, maxY int) {
	index := len(raster.primitives)
	raster.primitives = append(raster.primitives, primitive)
	indexStrokeRows(raster.primitiveRows, minY, maxY, primitive.bounds, index)
}

func indexStrokeRows(rows [][]int, minY, maxY int, bounds Rect, index int) {
	start := max(minY, int(math.Floor(bounds.Y)))
	end := min(maxY, int(math.Ceil(bounds.Y+bounds.Height)))
	for row := start; row < end; row++ {
		rows[row-minY] = append(rows[row-minY], index)
	}
}

func strokeCoverage(raster *strokeRaster, x, y int) float64 {
	inside := 0
	for sampleY := 0; sampleY < coverageSampleGrid; sampleY++ {
		for sampleX := 0; sampleX < coverageSampleGrid; sampleX++ {
			value := point{
				x: float64(x) + (float64(sampleX)+0.5)/coverageSampleGrid,
				y: float64(y) + (float64(sampleY)+0.5)/coverageSampleGrid,
			}
			if pointInStroke(raster, value) {
				inside++
			}
		}
	}
	return float64(inside) / float64(coverageSampleGrid*coverageSampleGrid)
}

func pointInStroke(raster *strokeRaster, value point) bool {
	row := int(math.Floor(value.y)) - raster.minY
	if row < 0 || row >= len(raster.segmentRows) {
		return false
	}
	for _, index := range raster.segmentRows[row] {
		if pointInStrokeSegment(raster.segments[index], value, raster.radius) {
			return true
		}
	}
	for _, index := range raster.primitiveRows[row] {
		if pointInStrokePrimitive(raster.primitives[index], value) {
			return true
		}
	}
	return false
}

func pointInStrokeSegment(segment pathSegment, value point, radius float64) bool {
	if value.x < segment.minX-radius || value.x > segment.maxX+radius || value.y < segment.minY-radius || value.y > segment.maxY+radius {
		return false
	}
	dx, dy := value.x-segment.a.x, value.y-segment.a.y
	projection := dx*segment.dx + dy*segment.dy
	if projection < 0 || projection > segment.length2 {
		return false
	}
	distance := dx*segment.dy - dy*segment.dx
	return distance*distance <= radius*radius*segment.length2
}

func pointInStrokePrimitive(primitive strokePrimitive, value point) bool {
	if value.x < primitive.bounds.X || value.x > primitive.bounds.X+primitive.bounds.Width || value.y < primitive.bounds.Y || value.y > primitive.bounds.Y+primitive.bounds.Height {
		return false
	}
	if primitive.kind == strokePrimitiveCircle {
		dx, dy := value.x-primitive.center.x, value.y-primitive.center.y
		return dx*dx+dy*dy <= primitive.radius*primitive.radius
	}
	var direction float64
	for index := 0; index < int(primitive.count); index++ {
		start := primitive.points[index]
		end := primitive.points[(index+1)%int(primitive.count)]
		valueCross := cross(point{x: end.x - start.x, y: end.y - start.y}, point{x: value.x - start.x, y: value.y - start.y})
		if math.Abs(valueCross) < 1e-12 {
			continue
		}
		if direction == 0 {
			direction = valueCross
			continue
		}
		if direction*valueCross < 0 {
			return false
		}
	}
	return true
}

func unitDirection(start, end point) (point, bool) {
	dx, dy := end.x-start.x, end.y-start.y
	length := math.Hypot(dx, dy)
	if length == 0 {
		return point{}, false
	}
	return point{x: dx / length, y: dy / length}, true
}

func cross(left, right point) float64 {
	return left.x*right.y - left.y*right.x
}

func (r *renderer) pixelBounds(bounds Rect, padding float64) (int, int, int, int) {
	rect := r.img.Bounds()
	minX := int(math.Floor(bounds.X - padding))
	minY := int(math.Floor(bounds.Y - padding))
	maxX := int(math.Ceil(bounds.X + bounds.Width + padding))
	maxY := int(math.Ceil(bounds.Y + bounds.Height + padding))
	if minX < rect.Min.X {
		minX = rect.Min.X
	}
	if minY < rect.Min.Y {
		minY = rect.Min.Y
	}
	if maxX > rect.Max.X {
		maxX = rect.Max.X
	}
	if maxY > rect.Max.Y {
		maxY = rect.Max.Y
	}
	return minX, minY, maxX, maxY
}

func (r *renderer) blend(x, y int, src color.NRGBA, coverage float64) {
	if coverage <= 0 {
		return
	}
	i := r.img.PixOffset(x, y)
	dst := color.NRGBA{
		R: r.img.Pix[i],
		G: r.img.Pix[i+1],
		B: r.img.Pix[i+2],
		A: r.img.Pix[i+3],
	}
	out := blendSourceOver(dst, src, coverage)
	r.img.Pix[i] = out.R
	r.img.Pix[i+1] = out.G
	r.img.Pix[i+2] = out.B
	r.img.Pix[i+3] = out.A
}

func (r *renderer) Layer(opacity float64, render func(*renderer)) {
	if opacity <= 0 {
		return
	}
	if opacity >= 1 {
		render(r)
		return
	}
	layerImage := image.NewNRGBA(r.img.Bounds())
	layer := renderer{img: layerImage}
	render(&layer)
	for y := layerImage.Bounds().Min.Y; y < layerImage.Bounds().Max.Y; y++ {
		for x := layerImage.Bounds().Min.X; x < layerImage.Bounds().Max.X; x++ {
			offset := layerImage.PixOffset(x, y)
			if layerImage.Pix[offset+3] == 0 {
				continue
			}
			r.blend(x, y, color.NRGBA{
				R: layerImage.Pix[offset],
				G: layerImage.Pix[offset+1],
				B: layerImage.Pix[offset+2],
				A: layerImage.Pix[offset+3],
			}, opacity)
		}
	}
}

func (r *renderer) Clip(renderMask, renderContent func(*renderer)) {
	maskImage := image.NewNRGBA(r.img.Bounds())
	renderMask(&renderer{img: maskImage})
	layerImage := image.NewNRGBA(r.img.Bounds())
	renderContent(&renderer{img: layerImage})
	for y := layerImage.Bounds().Min.Y; y < layerImage.Bounds().Max.Y; y++ {
		for x := layerImage.Bounds().Min.X; x < layerImage.Bounds().Max.X; x++ {
			offset := layerImage.PixOffset(x, y)
			if layerImage.Pix[offset+3] == 0 || maskImage.Pix[offset+3] == 0 {
				continue
			}
			r.blend(x, y, color.NRGBA{
				R: layerImage.Pix[offset],
				G: layerImage.Pix[offset+1],
				B: layerImage.Pix[offset+2],
				A: layerImage.Pix[offset+3],
			}, float64(maskImage.Pix[offset+3])/255)
		}
	}
}

func blendSourceOver(dst, src color.NRGBA, coverage float64) color.NRGBA {
	srcA := (float64(src.A) / 255) * clamp01(coverage)
	dstA := float64(dst.A) / 255
	outA := srcA + dstA*(1-srcA)
	if outA == 0 {
		return color.NRGBA{}
	}
	srcR := float64(src.R) / 255
	srcG := float64(src.G) / 255
	srcB := float64(src.B) / 255
	dstR := float64(dst.R) / 255
	dstG := float64(dst.G) / 255
	dstB := float64(dst.B) / 255
	outR := (srcR*srcA + dstR*dstA*(1-srcA)) / outA
	outG := (srcG*srcA + dstG*dstA*(1-srcA)) / outA
	outB := (srcB*srcA + dstB*dstA*(1-srcA)) / outA
	return color.NRGBA{
		R: uint8(math.Round(outR * 255)),
		G: uint8(math.Round(outG * 255)),
		B: uint8(math.Round(outB * 255)),
		A: uint8(math.Round(outA * 255)),
	}
}

func fillImage(img *image.NRGBA, clr color.Color) {
	c := color.NRGBAModel.Convert(clr).(color.NRGBA)
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			i := img.PixOffset(x, y)
			img.Pix[i] = c.R
			img.Pix[i+1] = c.G
			img.Pix[i+2] = c.B
			img.Pix[i+3] = c.A
		}
	}
}
