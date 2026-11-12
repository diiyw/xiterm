package svg

import (
	"encoding/xml"
	"fmt"
	"image/color"
	"math"
	"strings"
)

type element interface {
	render(*renderer, renderState)
}

type renderState struct {
	context   *renderContext
	style     computedStyle
	transform matrix
	clipping  bool
}

// paint resolves a fill or stroke to the colors of a shape in device space.
// Solid paints keep a single color; a gradient reference is resolved against the
// shape's bounding box in user space.
func (s renderState) paint(p paint, bounds Rect, current color.NRGBA, opacity float64) (fillPaint, bool) {
	switch p.kind {
	case paintReference:
		if s.context == nil {
			return fillPaint{}, false
		}
		return s.context.gradientPaint(p.reference, s.transform, bounds, opacity)
	case paintCurrentColor:
		return solidPaint(multiplyAlpha(current, opacity)), true
	case paintColor:
		return solidPaint(multiplyAlpha(p.color, opacity)), true
	default:
		return fillPaint{}, false
	}
}

type elementProperties struct {
	clipPath  string
	style     stylePatch
	transform matrix
}

func parseElementProperties(start xml.StartElement) elementProperties {
	attrs := attrsOf(start.Attr)
	return elementProperties{
		style:     parseStylePatch(start.Attr),
		transform: parseTransform(attrs["transform"]),
		clipPath:  localURLReference(presentationProperty(start.Attr, "clip-path")),
	}
}

func (p elementProperties) render(r *renderer, state renderState, content func(*renderer, renderState)) {
	state.style = p.style.apply(state.style)
	if state.style.hidden {
		return
	}
	state.transform = state.transform.Mul(p.transform)
	draw := func(target *renderer) { content(target, state) }
	if p.clipPath != "" && state.context != nil {
		unclipped := draw
		draw = func(target *renderer) {
			state.context.renderClipped(target, p.clipPath, state.transform, unclipped)
		}
	}
	if opacity := p.style.opacityValue(); !state.clipping && opacity < 1 {
		r.Layer(opacity, draw)
	} else {
		draw(r)
	}
}

type groupElement struct {
	children   []element
	properties elementProperties
}

func (g groupElement) render(r *renderer, state renderState) {
	g.properties.render(r, state, func(r *renderer, state renderState) {
		for _, child := range g.children {
			child.render(r, state)
		}
	})
}

type shapeElement struct {
	path       vectorPath
	properties elementProperties
	fillable   bool
	strokable  bool
}

func (s shapeElement) render(r *renderer, state renderState) {
	s.properties.render(r, state, func(r *renderer, state renderState) {
		path := s.path.Transform(state.transform)
		if state.clipping {
			if s.fillable {
				r.Fill(path, solidPaint(color.NRGBA{R: 255, G: 255, B: 255, A: 255}))
			}
			return
		}
		style := state.style
		var bounds Rect
		if (s.fillable && style.fill.kind == paintReference) ||
			(s.strokable && style.strokeWidth > 0 && style.stroke.kind == paintReference) {
			bounds = s.path.Bounds()
		}
		if s.fillable {
			if fill, ok := state.paint(style.fill, bounds, style.color, style.fillOpacity); ok {
				r.Fill(path, fill)
			}
		}
		if s.strokable && style.strokeWidth > 0 {
			if stroke, ok := state.paint(style.stroke, bounds, style.color, style.strokeOpacity); ok {
				r.Stroke(path, style.strokeWidth*state.transform.StrokeScale(), stroke, style.strokeLineCap, style.strokeLineJoin, style.strokeMiterLimit)
			}
		}
	})
}

type useElement struct {
	reference  string
	properties elementProperties
	x          float64
	y          float64
}

func (u useElement) render(r *renderer, state renderState) {
	u.properties.render(r, state, func(r *renderer, state renderState) {
		if state.context == nil || u.reference == "" {
			return
		}
		state.transform = state.transform.Mul(translateMatrix(u.x, u.y))
		state.context.renderReference(r, u.reference, state)
	})
}

func parseShape(start xml.StartElement) (element, error) {
	attrs := attrsOf(start.Attr)
	properties := parseElementProperties(start)

	switch localName(start.Name) {
	case "rect":
		x := parseLength(attrs["x"], 0)
		y := parseLength(attrs["y"], 0)
		width := parseLength(attrs["width"], 0)
		height := parseLength(attrs["height"], 0)
		if width <= 0 || height <= 0 {
			return nil, nil
		}
		rx := parseLength(attrs["rx"], 0)
		ry := parseLength(attrs["ry"], 0)
		return shapeElement{properties: properties, path: rectPath(x, y, width, height, rx, ry), fillable: true, strokable: true}, nil
	case "circle":
		cx := parseLength(attrs["cx"], 0)
		cy := parseLength(attrs["cy"], 0)
		r := parseLength(attrs["r"], 0)
		if r <= 0 {
			return nil, nil
		}
		return shapeElement{properties: properties, path: ellipsePath(cx, cy, r, r), fillable: true, strokable: true}, nil
	case "ellipse":
		cx := parseLength(attrs["cx"], 0)
		cy := parseLength(attrs["cy"], 0)
		rx := parseLength(attrs["rx"], 0)
		ry := parseLength(attrs["ry"], 0)
		if rx <= 0 || ry <= 0 {
			return nil, nil
		}
		return shapeElement{properties: properties, path: ellipsePath(cx, cy, rx, ry), fillable: true, strokable: true}, nil
	case "line":
		x1 := parseLength(attrs["x1"], 0)
		y1 := parseLength(attrs["y1"], 0)
		x2 := parseLength(attrs["x2"], 0)
		y2 := parseLength(attrs["y2"], 0)
		return shapeElement{properties: properties, path: linePath(point{x: x1, y: y1}, point{x: x2, y: y2}), strokable: true}, nil
	case "polyline":
		points, err := parsePoints(attrs["points"])
		if err != nil || len(points) == 0 {
			return nil, err
		}
		return shapeElement{properties: properties, path: polyPath(points, false), fillable: true, strokable: true}, nil
	case "polygon":
		points, err := parsePoints(attrs["points"])
		if err != nil || len(points) == 0 {
			return nil, err
		}
		return shapeElement{properties: properties, path: polyPath(points, true), fillable: true, strokable: true}, nil
	case "path":
		path, err := parsePath(attrs["d"])
		if err != nil {
			return nil, err
		}
		return shapeElement{properties: properties, path: path, fillable: true, strokable: true}, nil
	default:
		return nil, nil
	}
}

func parsePoints(raw string) ([]point, error) {
	values, err := parseNumberList(raw)
	if err != nil {
		return nil, err
	}
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("svg: points needs x/y pairs")
	}
	points := make([]point, 0, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		points = append(points, point{x: values[i], y: values[i+1]})
	}
	return points, nil
}

func rectPath(x, y, width, height, rx, ry float64) vectorPath {
	if rx <= 0 && ry <= 0 {
		return polyPath([]point{
			{x: x, y: y},
			{x: x + width, y: y},
			{x: x + width, y: y + height},
			{x: x, y: y + height},
		}, true)
	}
	if rx <= 0 {
		rx = ry
	}
	if ry <= 0 {
		ry = rx
	}
	rx = math.Min(rx, width/2)
	ry = math.Min(ry, height/2)
	var path vectorPath
	path.MoveTo(point{x: x + rx, y: y})
	path.LineTo(point{x: x + width - rx, y: y})
	path.appendArc(point{x: x + width - rx, y: y + ry}, rx, ry, -math.Pi/2, 0, roundedRectArcSegments)
	path.LineTo(point{x: x + width, y: y + height - ry})
	path.appendArc(point{x: x + width - rx, y: y + height - ry}, rx, ry, 0, math.Pi/2, roundedRectArcSegments)
	path.LineTo(point{x: x + rx, y: y + height})
	path.appendArc(point{x: x + rx, y: y + height - ry}, rx, ry, math.Pi/2, math.Pi, roundedRectArcSegments)
	path.LineTo(point{x: x, y: y + ry})
	path.appendArc(point{x: x + rx, y: y + ry}, rx, ry, math.Pi, math.Pi*1.5, roundedRectArcSegments)
	path.Close()
	return path
}

const roundedRectArcSegments = 16

func ellipsePath(cx, cy, rx, ry float64) vectorPath {
	const segments = 64
	points := make([]point, 0, segments)
	for i := 0; i < segments; i++ {
		angle := float64(i) * math.Pi * 2 / segments
		points = append(points, point{x: cx + math.Cos(angle)*rx, y: cy + math.Sin(angle)*ry})
	}
	return polyPath(points, true)
}

func linePath(a, b point) vectorPath {
	return polyPath([]point{a, b}, false)
}

func polyPath(points []point, closed bool) vectorPath {
	path := vectorPath{}
	if len(points) == 0 {
		return path
	}
	path.MoveTo(points[0])
	for _, p := range points[1:] {
		path.LineTo(p)
	}
	if closed {
		path.Close()
	}
	return path
}

func parseLength(raw string, fallback float64) float64 {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	if strings.HasSuffix(value, "%") {
		number, err := parseFloat(strings.TrimSpace(strings.TrimSuffix(value, "%")))
		if err != nil {
			return fallback
		}
		if fallback > 0 {
			return fallback * number / 100
		}
		return number
	}
	units := []struct {
		suffix string
		scale  float64
	}{
		{"px", 1},
		{"pt", 96.0 / 72.0},
		{"pc", 16},
		{"mm", 96.0 / 25.4},
		{"cm", 96.0 / 2.54},
		{"in", 96},
	}
	for _, unit := range units {
		if strings.HasSuffix(value, unit.suffix) {
			number, err := parseFloat(strings.TrimSpace(strings.TrimSuffix(value, unit.suffix)))
			if err != nil {
				return fallback
			}
			return number * unit.scale
		}
	}
	number, err := parseFloat(value)
	if err != nil {
		return fallback
	}
	return number
}

func parseViewBox(raw string) (Rect, bool) {
	values, err := parseNumberList(raw)
	if err != nil || len(values) != 4 || values[2] <= 0 || values[3] <= 0 {
		return Rect{}, false
	}
	return Rect{X: values[0], Y: values[1], Width: values[2], Height: values[3]}, true
}
