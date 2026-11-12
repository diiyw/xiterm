package svg

import (
	"fmt"
	"math"
)

type point struct {
	x float64
	y float64
}

type subpath struct {
	points []point
	closed bool
}

type vectorPath struct {
	subpaths     []subpath
	segments     []pathSegment
	fillSegments []pathSegment
}

// pathSegment stores the geometry needed by the SVG rasterizer. Keeping this
// derived data with a transformed path avoids rebuilding it for every pixel
// sample.
type pathSegment struct {
	a, b                   point
	minX, minY, maxX, maxY float64
	dx, dy, length2        float64
}

func parsePath(d string) (vectorPath, error) {
	parser := pathParser{scanner: numberScanner{s: d}}
	return parser.parse()
}

func (p *vectorPath) MoveTo(pt point) {
	p.subpaths = append(p.subpaths, subpath{points: []point{pt}})
}

func (p *vectorPath) LineTo(pt point) {
	if len(p.subpaths) == 0 {
		p.MoveTo(pt)
		return
	}
	last := &p.subpaths[len(p.subpaths)-1]
	last.points = append(last.points, pt)
}

func (p *vectorPath) Close() {
	if len(p.subpaths) == 0 {
		return
	}
	p.subpaths[len(p.subpaths)-1].closed = true
}

func (p vectorPath) Transform(m matrix) vectorPath {
	out := vectorPath{subpaths: make([]subpath, 0, len(p.subpaths))}
	segmentCount := 0
	fillSegmentCount := 0
	for _, sub := range p.subpaths {
		if len(sub.points) > 1 {
			segmentCount += len(sub.points) - 1
			if sub.closed {
				segmentCount++
			}
		}
		if len(sub.points) > 2 {
			fillSegmentCount += len(sub.points)
		}
	}
	out.segments = make([]pathSegment, 0, segmentCount)
	out.fillSegments = make([]pathSegment, 0, fillSegmentCount)
	for _, sub := range p.subpaths {
		next := subpath{closed: sub.closed, points: make([]point, len(sub.points))}
		for i, pt := range sub.points {
			next.points[i] = m.Transform(pt)
		}
		out.subpaths = append(out.subpaths, next)
		for i := 1; i < len(next.points); i++ {
			segment := newPathSegment(next.points[i-1], next.points[i])
			out.segments = append(out.segments, segment)
			if len(next.points) > 2 {
				out.fillSegments = append(out.fillSegments, segment)
			}
		}
		if next.closed && len(next.points) > 1 {
			out.segments = append(out.segments, newPathSegment(next.points[len(next.points)-1], next.points[0]))
		}
		if len(next.points) > 2 {
			out.fillSegments = append(out.fillSegments, newPathSegment(next.points[len(next.points)-1], next.points[0]))
		}
	}
	return out
}

func newPathSegment(a, b point) pathSegment {
	minX, maxX := a.x, b.x
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	minY, maxY := a.y, b.y
	if minY > maxY {
		minY, maxY = maxY, minY
	}
	dx, dy := b.x-a.x, b.y-a.y
	return pathSegment{
		a: a, b: b,
		minX: minX, minY: minY, maxX: maxX, maxY: maxY,
		dx: dx, dy: dy, length2: dx*dx + dy*dy,
	}
}

func (p vectorPath) Bounds() Rect {
	first := true
	var minX, minY, maxX, maxY float64
	for _, sub := range p.subpaths {
		for _, pt := range sub.points {
			if first {
				minX, maxX, minY, maxY = pt.x, pt.x, pt.y, pt.y
				first = false
				continue
			}
			minX = math.Min(minX, pt.x)
			minY = math.Min(minY, pt.y)
			maxX = math.Max(maxX, pt.x)
			maxY = math.Max(maxY, pt.y)
		}
	}
	if first {
		return Rect{}
	}
	return Rect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
}

func (p *vectorPath) appendArc(center point, rx, ry, start, end float64, segments int) {
	if segments <= 0 {
		segments = 1
	}
	for i := 1; i <= segments; i++ {
		t := float64(i) / float64(segments)
		angle := start + (end-start)*t
		p.LineTo(point{x: center.x + math.Cos(angle)*rx, y: center.y + math.Sin(angle)*ry})
	}
}

type pathParser struct {
	scanner numberScanner
	current point
	start   point
	lastC   point
	lastQ   point
	hasC    bool
	hasQ    bool
}

func (p *pathParser) parse() (vectorPath, error) {
	var path vectorPath
	var cmd byte
	for {
		p.scanner.skipSeparator()
		if p.scanner.done() {
			return path, nil
		}
		if isPathCommand(p.scanner.s[p.scanner.i]) {
			cmd = p.scanner.s[p.scanner.i]
			p.scanner.i++
		} else if cmd == 0 {
			return path, fmt.Errorf("svg: path data must start with a command")
		}
		activeCmd := cmd
		argsStart := p.scanner.i

		switch cmd {
		case 'M', 'm':
			if err := p.parseMove(&path, cmd == 'm'); err != nil {
				return path, err
			}
			if cmd == 'm' {
				cmd = 'l'
			} else {
				cmd = 'L'
			}
		case 'L', 'l':
			if err := p.parseLine(&path, cmd == 'l'); err != nil {
				return path, err
			}
		case 'H', 'h':
			if err := p.parseHorizontal(&path, cmd == 'h'); err != nil {
				return path, err
			}
		case 'V', 'v':
			if err := p.parseVertical(&path, cmd == 'v'); err != nil {
				return path, err
			}
		case 'C', 'c':
			if err := p.parseCubic(&path, cmd == 'c'); err != nil {
				return path, err
			}
		case 'S', 's':
			if err := p.parseSmoothCubic(&path, cmd == 's'); err != nil {
				return path, err
			}
		case 'Q', 'q':
			if err := p.parseQuadratic(&path, cmd == 'q'); err != nil {
				return path, err
			}
		case 'T', 't':
			if err := p.parseSmoothQuadratic(&path, cmd == 't'); err != nil {
				return path, err
			}
		case 'A', 'a':
			if err := p.parseArc(&path, cmd == 'a'); err != nil {
				return path, err
			}
		case 'Z', 'z':
			path.Close()
			p.current = p.start
			p.resetControl()
			cmd = 0
		default:
			return path, fmt.Errorf("svg: unsupported path command %q", cmd)
		}
		if activeCmd != 'Z' && activeCmd != 'z' && p.scanner.i == argsStart {
			return path, fmt.Errorf("svg: path command %q requires parameters", activeCmd)
		}
	}
}

func (p *pathParser) parseMove(path *vectorPath, relative bool) error {
	first := true
	for p.scanner.hasNumber() {
		pt, err := p.readPoint(relative)
		if err != nil {
			return err
		}
		if first {
			path.MoveTo(pt)
			p.start = pt
			first = false
		} else {
			path.LineTo(pt)
		}
		p.current = pt
		p.resetControl()
	}
	if first {
		return fmt.Errorf("svg: move command requires a point")
	}
	return nil
}

func (p *pathParser) parseLine(path *vectorPath, relative bool) error {
	for p.scanner.hasNumber() {
		pt, err := p.readPoint(relative)
		if err != nil {
			return err
		}
		path.LineTo(pt)
		p.current = pt
		p.resetControl()
	}
	return nil
}

func (p *pathParser) parseHorizontal(path *vectorPath, relative bool) error {
	for p.scanner.hasNumber() {
		x, err := p.scanner.readNumber()
		if err != nil {
			return err
		}
		if relative {
			x += p.current.x
		}
		p.current.x = x
		path.LineTo(p.current)
		p.resetControl()
	}
	return nil
}

func (p *pathParser) parseVertical(path *vectorPath, relative bool) error {
	for p.scanner.hasNumber() {
		y, err := p.scanner.readNumber()
		if err != nil {
			return err
		}
		if relative {
			y += p.current.y
		}
		p.current.y = y
		path.LineTo(p.current)
		p.resetControl()
	}
	return nil
}

func (p *pathParser) parseCubic(path *vectorPath, relative bool) error {
	for p.scanner.hasNumber() {
		c1, err := p.readPoint(relative)
		if err != nil {
			return err
		}
		c2, err := p.readPoint(relative)
		if err != nil {
			return err
		}
		end, err := p.readPoint(relative)
		if err != nil {
			return err
		}
		for _, pt := range flattenCubic(p.current, c1, c2, end, 0.25)[1:] {
			path.LineTo(pt)
		}
		p.current = end
		p.lastC = c2
		p.hasC = true
		p.hasQ = false
	}
	return nil
}

func (p *pathParser) parseSmoothCubic(path *vectorPath, relative bool) error {
	for p.scanner.hasNumber() {
		c1 := p.current
		if p.hasC {
			c1 = reflectPoint(p.lastC, p.current)
		}
		c2, err := p.readPoint(relative)
		if err != nil {
			return err
		}
		end, err := p.readPoint(relative)
		if err != nil {
			return err
		}
		for _, pt := range flattenCubic(p.current, c1, c2, end, 0.25)[1:] {
			path.LineTo(pt)
		}
		p.current = end
		p.lastC = c2
		p.hasC = true
		p.hasQ = false
	}
	return nil
}

func (p *pathParser) parseQuadratic(path *vectorPath, relative bool) error {
	for p.scanner.hasNumber() {
		c, err := p.readPoint(relative)
		if err != nil {
			return err
		}
		end, err := p.readPoint(relative)
		if err != nil {
			return err
		}
		for _, pt := range flattenQuadratic(p.current, c, end, 0.25)[1:] {
			path.LineTo(pt)
		}
		p.current = end
		p.lastQ = c
		p.hasQ = true
		p.hasC = false
	}
	return nil
}

func (p *pathParser) parseSmoothQuadratic(path *vectorPath, relative bool) error {
	for p.scanner.hasNumber() {
		c := p.current
		if p.hasQ {
			c = reflectPoint(p.lastQ, p.current)
		}
		end, err := p.readPoint(relative)
		if err != nil {
			return err
		}
		for _, pt := range flattenQuadratic(p.current, c, end, 0.25)[1:] {
			path.LineTo(pt)
		}
		p.current = end
		p.lastQ = c
		p.hasQ = true
		p.hasC = false
	}
	return nil
}

func (p *pathParser) parseArc(path *vectorPath, relative bool) error {
	for p.scanner.hasNumber() {
		rx, err := p.scanner.readNumber()
		if err != nil {
			return err
		}
		ry, err := p.scanner.readNumber()
		if err != nil {
			return err
		}
		rotation, err := p.scanner.readNumber()
		if err != nil {
			return err
		}
		large, err := p.scanner.readNumber()
		if err != nil {
			return err
		}
		sweep, err := p.scanner.readNumber()
		if err != nil {
			return err
		}
		end, err := p.readPoint(relative)
		if err != nil {
			return err
		}
		for _, pt := range flattenArc(p.current, end, rx, ry, rotation, large != 0, sweep != 0)[1:] {
			path.LineTo(pt)
		}
		p.current = end
		p.resetControl()
	}
	return nil
}

func (p *pathParser) readPoint(relative bool) (point, error) {
	x, err := p.scanner.readNumber()
	if err != nil {
		return point{}, err
	}
	y, err := p.scanner.readNumber()
	if err != nil {
		return point{}, err
	}
	pt := point{x: x, y: y}
	if relative {
		pt.x += p.current.x
		pt.y += p.current.y
	}
	return pt, nil
}

func (p *pathParser) resetControl() {
	p.hasC = false
	p.hasQ = false
}

func isPathCommand(b byte) bool {
	switch b {
	case 'M', 'm', 'L', 'l', 'H', 'h', 'V', 'v', 'C', 'c', 'S', 's', 'Q', 'q', 'T', 't', 'A', 'a', 'Z', 'z':
		return true
	default:
		return false
	}
}

func reflectPoint(p, around point) point {
	return point{x: around.x*2 - p.x, y: around.y*2 - p.y}
}

func midpoint(a, b point) point {
	return point{x: (a.x + b.x) * 0.5, y: (a.y + b.y) * 0.5}
}

func flattenQuadratic(p0, p1, p2 point, tolerance float64) []point {
	if quadraticFlatEnough(p0, p1, p2, tolerance) {
		return []point{p0, p2}
	}
	p01 := midpoint(p0, p1)
	p12 := midpoint(p1, p2)
	p012 := midpoint(p01, p12)
	left := flattenQuadratic(p0, p01, p012, tolerance)
	right := flattenQuadratic(p012, p12, p2, tolerance)
	return append(left[:len(left)-1], right...)
}

func quadraticFlatEnough(p0, p1, p2 point, tolerance float64) bool {
	dx := p0.x - 2*p1.x + p2.x
	dy := p0.y - 2*p1.y + p2.y
	return dx*dx+dy*dy <= tolerance*tolerance*4
}

func flattenCubic(p0, p1, p2, p3 point, tolerance float64) []point {
	if cubicFlatEnough(p0, p1, p2, p3, tolerance) {
		return []point{p0, p3}
	}
	p01 := midpoint(p0, p1)
	p12 := midpoint(p1, p2)
	p23 := midpoint(p2, p3)
	p012 := midpoint(p01, p12)
	p123 := midpoint(p12, p23)
	p0123 := midpoint(p012, p123)
	left := flattenCubic(p0, p01, p012, p0123, tolerance)
	right := flattenCubic(p0123, p123, p23, p3, tolerance)
	return append(left[:len(left)-1], right...)
}

func cubicFlatEnough(p0, p1, p2, p3 point, tolerance float64) bool {
	ux := 3*p1.x - 2*p0.x - p3.x
	uy := 3*p1.y - 2*p0.y - p3.y
	vx := 3*p2.x - 2*p3.x - p0.x
	vy := 3*p2.y - 2*p3.y - p0.y
	return math.Max(ux*ux+uy*uy, vx*vx+vy*vy) <= tolerance*tolerance*16
}

func flattenArc(start, end point, rx, ry, rotation float64, largeArc, sweep bool) []point {
	rx = math.Abs(rx)
	ry = math.Abs(ry)
	if rx == 0 || ry == 0 || (start.x == end.x && start.y == end.y) {
		return []point{start, end}
	}
	phi := rotation * math.Pi / 180
	cosPhi, sinPhi := math.Cos(phi), math.Sin(phi)
	dx := (start.x - end.x) * 0.5
	dy := (start.y - end.y) * 0.5
	x1p := cosPhi*dx + sinPhi*dy
	y1p := -sinPhi*dx + cosPhi*dy

	lambda := x1p*x1p/(rx*rx) + y1p*y1p/(ry*ry)
	if lambda > 1 {
		scale := math.Sqrt(lambda)
		rx *= scale
		ry *= scale
	}

	rx2, ry2 := rx*rx, ry*ry
	x1p2, y1p2 := x1p*x1p, y1p*y1p
	sign := 1.0
	if largeArc == sweep {
		sign = -1
	}
	denominator := rx2*y1p2 + ry2*x1p2
	coef := 0.0
	if denominator != 0 {
		coef = sign * math.Sqrt(math.Max(0, (rx2*ry2-rx2*y1p2-ry2*x1p2)/denominator))
	}
	cxp := coef * rx * y1p / ry
	cyp := coef * -ry * x1p / rx
	cx := cosPhi*cxp - sinPhi*cyp + (start.x+end.x)*0.5
	cy := sinPhi*cxp + cosPhi*cyp + (start.y+end.y)*0.5

	v1x := (x1p - cxp) / rx
	v1y := (y1p - cyp) / ry
	v2x := (-x1p - cxp) / rx
	v2y := (-y1p - cyp) / ry
	theta1 := vectorAngle(1, 0, v1x, v1y)
	delta := vectorAngle(v1x, v1y, v2x, v2y)
	if !sweep && delta > 0 {
		delta -= math.Pi * 2
	}
	if sweep && delta < 0 {
		delta += math.Pi * 2
	}

	segments := int(math.Ceil(math.Abs(delta) / (math.Pi / 16)))
	if segments < 1 {
		segments = 1
	}
	points := make([]point, 0, segments+1)
	points = append(points, start)
	for i := 1; i <= segments; i++ {
		theta := theta1 + delta*float64(i)/float64(segments)
		x := cx + rx*cosPhi*math.Cos(theta) - ry*sinPhi*math.Sin(theta)
		y := cy + rx*sinPhi*math.Cos(theta) + ry*cosPhi*math.Sin(theta)
		points = append(points, point{x: x, y: y})
	}
	return points
}

func vectorAngle(ux, uy, vx, vy float64) float64 {
	dot := ux*vx + uy*vy
	length := math.Hypot(ux, uy) * math.Hypot(vx, vy)
	if length == 0 {
		return 0
	}
	value := math.Max(-1, math.Min(1, dot/length))
	angle := math.Acos(value)
	if ux*vy-uy*vx < 0 {
		return -angle
	}
	return angle
}
