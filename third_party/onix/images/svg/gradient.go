package svg

import (
	"encoding/xml"
	"image/color"
	"io"
	"math"
	"strings"
)

// Gradient paint servers. A gradient is resolved once per document: coordinates
// live in gradient space, which is 0..1 bounding-box units for objectBoundingBox
// and user units for userSpaceOnUse.

type gradientSpread uint8

const (
	gradientPad gradientSpread = iota
	gradientReflect
	gradientRepeat
)

// gradientLength is a coordinate or radius. Percentages are stored as fractions
// and scaled by the caller, because they resolve against the viewport for
// userSpaceOnUse and against the bounding box otherwise.
type gradientLength struct {
	value   float64
	percent bool
}

func (l gradientLength) resolve(scale float64) float64 {
	if l.percent {
		return l.value * scale
	}
	return l.value
}

func parseGradientLength(raw string) (gradientLength, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return gradientLength{}, false
	}
	if strings.HasSuffix(value, "%") {
		number, err := parseFloat(strings.TrimSpace(strings.TrimSuffix(value, "%")))
		if err != nil {
			return gradientLength{}, false
		}
		return gradientLength{value: number / 100, percent: true}, true
	}
	number, err := parseFloat(value)
	if err != nil {
		return gradientLength{}, false
	}
	return gradientLength{value: number}, true
}

type gradientStop struct {
	offset float64
	color  color.NRGBA
}

// gradientDef holds the attributes a gradient element declares. Flags record
// which attributes were present, so an element can inherit the rest from the
// gradient it references with href.
type gradientDef struct {
	href                             string
	stops                            []gradientStop
	transform                        matrix
	x2                               gradientLength
	fy                               gradientLength
	fx                               gradientLength
	r                                gradientLength
	cy                               gradientLength
	cx                               gradientLength
	y2                               gradientLength
	x1                               gradientLength
	y1                               gradientLength
	spreadSet                        bool
	spread                           gradientSpread
	x1Set, y1Set, x2Set, y2Set       bool
	linear                           bool
	stopsSet                         bool
	transformSet                     bool
	unitsSet                         bool
	object                           bool
	cxSet, cySet, rSet, fxSet, fySet bool
}

// gradient is a paint server resolved against the document viewport. transform
// maps gradient space to the user space of the shape that references it.
type gradient struct {
	stops     []gradientStop
	transform matrix
	cy        float64
	x2        float64
	y2        float64
	cx        float64
	r         float64
	fx, fy    float64
	y1        float64
	x1        float64
	linear    bool
	spread    gradientSpread
	object    bool
}

// colorAt resolves the color of a point in gradient space.
func (g *gradient) colorAt(value point) color.NRGBA {
	return g.sample(g.parameter(value))
}

func (g *gradient) parameter(value point) float64 {
	if g.linear {
		dx, dy := g.x2-g.x1, g.y2-g.y1
		length := dx*dx + dy*dy
		if length == 0 {
			// A degenerate axis is painted with the last stop.
			return 1
		}
		return ((value.x-g.x1)*dx + (value.y-g.y1)*dy) / length
	}
	if g.r <= 0 {
		return 1
	}
	// The parameter is the ratio along the ray from the focal point to the point
	// where that ray leaves the gradient circle, so it reaches 1 on the circle.
	dx, dy := value.x-g.fx, value.y-g.fy
	ex, ey := g.fx-g.cx, g.fy-g.cy
	quadratic := dx*dx + dy*dy
	if quadratic == 0 {
		return 0
	}
	linear := 2 * (dx*ex + dy*ey)
	constant := ex*ex + ey*ey - g.r*g.r
	discriminant := linear*linear - 4*quadratic*constant
	if discriminant < 0 {
		return 1
	}
	root := (-linear + math.Sqrt(discriminant)) / (2 * quadratic)
	if root <= 0 {
		return 1
	}
	return 1 / root
}

func (g *gradient) sample(value float64) color.NRGBA {
	if len(g.stops) == 0 {
		return color.NRGBA{}
	}
	switch g.spread {
	case gradientRepeat:
		value = math.Mod(value, 1)
		if value < 0 {
			value += 1
		}
	case gradientReflect:
		value = math.Mod(value, 2)
		if value < 0 {
			value += 2
		}
		if value > 1 {
			value = 2 - value
		}
	default:
		value = clamp01(value)
	}
	if value < g.stops[0].offset {
		return g.stops[0].color
	}
	last := g.stops[len(g.stops)-1]
	if value >= last.offset {
		return last.color
	}
	for index := 1; index < len(g.stops); index++ {
		next := g.stops[index]
		if value >= next.offset {
			continue
		}
		previous := g.stops[index-1]
		span := next.offset - previous.offset
		return blendColor(previous.color, next.color, (value-previous.offset)/span)
	}
	return last.color
}

// blendColor interpolates straight alpha channels, as SVG stop interpolation
// does.
func blendColor(from, to color.NRGBA, fraction float64) color.NRGBA {
	mix := func(a, b uint8) uint8 {
		return uint8(math.Round(float64(a) + (float64(b)-float64(a))*fraction))
	}
	return color.NRGBA{R: mix(from.R, to.R), G: mix(from.G, to.G), B: mix(from.B, to.B), A: mix(from.A, to.A)}
}

// fillPaint resolves the color of a device-space pixel. A nil gradient keeps the
// solid fill path free of extra work.
type fillPaint struct {
	gradient *gradient
	inverse  matrix
	opacity  float64
	color    color.NRGBA
	valid    bool
}

func solidPaint(clr color.NRGBA) fillPaint {
	return fillPaint{color: clr, opacity: 1, valid: true}
}

func (p fillPaint) at(x, y float64) color.NRGBA {
	if p.gradient == nil {
		return p.color
	}
	return multiplyAlpha(p.gradient.colorAt(p.inverse.Transform(point{x: x, y: y})), p.opacity)
}

func parseGradientDefinition(start xml.StartElement, linear bool) *gradientDef {
	attrs := attrsOf(start.Attr)
	// The reference is href or the legacy xlink:href, which attrsOf keeps under
	// its local name.
	href := strings.TrimSpace(attrs["href"])
	if href == "" {
		href = strings.TrimSpace(attrs["xlink:href"])
	}
	definition := &gradientDef{linear: linear, href: hrefReference(href)}
	if raw := strings.TrimSpace(attrs["gradientunits"]); raw != "" {
		switch strings.ToLower(raw) {
		case "userspaceonuse":
			definition.object, definition.unitsSet = false, true
		case "objectboundingbox":
			definition.object, definition.unitsSet = true, true
		}
	}
	if raw := strings.TrimSpace(attrs["gradienttransform"]); raw != "" {
		definition.transform, definition.transformSet = parseTransform(raw), true
	}
	if raw := strings.TrimSpace(attrs["spreadmethod"]); raw != "" {
		switch strings.ToLower(raw) {
		case "pad":
			definition.spread, definition.spreadSet = gradientPad, true
		case "reflect":
			definition.spread, definition.spreadSet = gradientReflect, true
		case "repeat":
			definition.spread, definition.spreadSet = gradientRepeat, true
		}
	}
	for name, target := range map[string]*gradientLength{
		"x1": &definition.x1, "y1": &definition.y1, "x2": &definition.x2, "y2": &definition.y2,
		"cx": &definition.cx, "cy": &definition.cy, "r": &definition.r, "fx": &definition.fx, "fy": &definition.fy,
	} {
		length, ok := parseGradientLength(attrs[name])
		if !ok {
			continue
		}
		*target = length
		switch name {
		case "x1":
			definition.x1Set = true
		case "y1":
			definition.y1Set = true
		case "x2":
			definition.x2Set = true
		case "y2":
			definition.y2Set = true
		case "cx":
			definition.cxSet = true
		case "cy":
			definition.cySet = true
		case "r":
			definition.rSet = true
		case "fx":
			definition.fxSet = true
		case "fy":
			definition.fySet = true
		}
	}
	return definition
}

func parseGradientStop(start xml.StartElement) gradientStop {
	stop := gradientStop{
		offset: parseStopOffset(presentationProperty(start.Attr, "offset")),
		color:  color.NRGBA{A: 255},
	}
	if clr, ok := parseColor(presentationProperty(start.Attr, "stop-color")); ok {
		stop.color = clr
	}
	if raw := presentationProperty(start.Attr, "stop-opacity"); raw != "" {
		if alpha, ok := parseAlphaComponent(raw); ok {
			stop.color.A = uint8(math.Round(float64(stop.color.A) * float64(alpha) / 255))
		}
	}
	return stop
}

func parseStopOffset(raw string) float64 {
	value := strings.TrimSpace(raw)
	if strings.HasSuffix(value, "%") {
		number, err := parseFloat(strings.TrimSpace(strings.TrimSuffix(value, "%")))
		if err != nil {
			return 0
		}
		return clamp01(number / 100)
	}
	number, err := parseFloat(value)
	if err != nil {
		return 0
	}
	return clamp01(number)
}

// parseGradientChildren collects the <stop> children of a gradient element.
func (p *documentParser) parseGradientChildren(end string) ([]gradientStop, error) {
	var stops []gradientStop
	for {
		token, err := p.decoder.Token()
		if err != nil {
			if err == io.EOF {
				return stops, nil
			}
			return nil, err
		}
		switch token := token.(type) {
		case xml.StartElement:
			if localName(token.Name) == "stop" {
				stop := parseGradientStop(token)
				// Keep document order, raising decreasing offsets to the previous
				// maximum so coincident stops retain their abrupt color transition.
				if len(stops) > 0 {
					stop.offset = max(stop.offset, stops[len(stops)-1].offset)
				}
				stops = append(stops, stop)
			}
			if err := p.decoder.Skip(); err != nil {
				return nil, err
			}
		case xml.EndElement:
			if localName(token.Name) == end {
				return stops, nil
			}
		}
	}
}

// resolveGradients turns the parsed definitions into paint servers, following
// href chains so that a gradient inherits the attributes it does not declare.
func resolveGradients(definitions map[string]*gradientDef, viewport Rect) map[string]*gradient {
	if len(definitions) == 0 {
		return nil
	}
	resolved := make(map[string]*gradient, len(definitions))
	for id := range definitions {
		if gradient, ok := resolveGradient(definitions, id, viewport); ok {
			resolved[id] = gradient
		}
	}
	return resolved
}

func resolveGradient(definitions map[string]*gradientDef, id string, viewport Rect) (*gradient, bool) {
	// Walk href references, ignoring cycles, so a definition can extend another.
	var chain []*gradientDef
	for current, seen := id, make(map[string]bool); current != "" && !seen[current]; {
		seen[current] = true
		definition := definitions[current]
		if definition == nil {
			break
		}
		chain = append(chain, definition)
		current = definition.href
	}
	if len(chain) == 0 {
		return nil, false
	}
	object := true
	for _, definition := range chain {
		if definition.unitsSet {
			object = definition.object
			break
		}
	}
	// Percentages resolve against the viewport in user space and against the
	// bounding box, which is already 0..1, otherwise.
	scaleX, scaleY, scaleRadius := 1.0, 1.0, 1.0
	if !object {
		scaleX, scaleY = viewport.Width, viewport.Height
		scaleRadius = math.Hypot(viewport.Width, viewport.Height) / math.Sqrt2
	}
	definition := &gradient{
		linear:    chain[0].linear,
		object:    object,
		spread:    gradientPad,
		transform: identityMatrix(),
	}
	// Specification defaults, expressed in gradient space.
	if definition.linear {
		definition.x2 = gradientLength{value: 1, percent: true}.resolve(scaleX)
	} else {
		definition.cx = gradientLength{value: 0.5, percent: true}.resolve(scaleX)
		definition.cy = gradientLength{value: 0.5, percent: true}.resolve(scaleY)
		definition.r = gradientLength{value: 0.5, percent: true}.resolve(scaleRadius)
	}
	// Apply the chain from the referenced gradient outwards, so the referencing
	// element wins.
	var fxSet, fySet bool
	for index := len(chain) - 1; index >= 0; index-- {
		current := chain[index]
		if current.transformSet {
			definition.transform = current.transform
		}
		if current.spreadSet {
			definition.spread = current.spread
		}
		if current.stopsSet {
			definition.stops = current.stops
		}
		if definition.linear {
			if current.x1Set {
				definition.x1 = current.x1.resolve(scaleX)
			}
			if current.y1Set {
				definition.y1 = current.y1.resolve(scaleY)
			}
			if current.x2Set {
				definition.x2 = current.x2.resolve(scaleX)
			}
			if current.y2Set {
				definition.y2 = current.y2.resolve(scaleY)
			}
			continue
		}
		if current.cxSet {
			definition.cx = current.cx.resolve(scaleX)
		}
		if current.cySet {
			definition.cy = current.cy.resolve(scaleY)
		}
		if current.rSet {
			definition.r = current.r.resolve(scaleRadius)
		}
		if current.fxSet {
			definition.fx = current.fx.resolve(scaleX)
			fxSet = true
		}
		if current.fySet {
			definition.fy = current.fy.resolve(scaleY)
			fySet = true
		}
	}
	// The focal point defaults to the center, and follows an inherited center.
	if !definition.linear {
		if !fxSet {
			definition.fx = definition.cx
		}
		if !fySet {
			definition.fy = definition.cy
		}
	}
	return definition, true
}

// gradientPaint builds the paint for a shape. transform maps the shape's user
// space to device space, and bounds is its bounding box in user space.
func (c *renderContext) gradientPaint(id string, transform matrix, bounds Rect, opacity float64) (fillPaint, bool) {
	definition := c.gradients[id]
	if definition == nil {
		return fillPaint{}, false
	}
	// gradientTransform applies in gradient space, so it is the innermost step:
	// a point travels gradient -> gradientTransform -> bounding box -> user space.
	matrix := transform
	if definition.object {
		matrix = matrix.Mul(translateMatrix(bounds.X, bounds.Y)).Mul(scaleMatrix(bounds.Width, bounds.Height))
	}
	matrix = matrix.Mul(definition.transform)
	inverse, ok := matrix.Invert()
	if !ok {
		return fillPaint{}, false
	}
	return fillPaint{gradient: definition, inverse: inverse, opacity: opacity, valid: true}, true
}

// hrefReference reads the local "#id" a gradient href points at.
func hrefReference(raw string) string {
	value := strings.TrimSpace(raw)
	if !strings.HasPrefix(value, "#") {
		return ""
	}
	return strings.TrimSpace(value[1:])
}
