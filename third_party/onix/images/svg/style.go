package svg

import (
	"encoding/xml"
	"image/color"
	"math"
	"strconv"
	"strings"
)

type paintKind int

const (
	paintUnset paintKind = iota
	paintNone
	paintColor
	paintCurrentColor
	paintReference
)

type paint struct {
	reference string
	kind      paintKind
	color     color.NRGBA
}

type strokeLineCap uint8

const (
	strokeLineCapButt strokeLineCap = iota
	strokeLineCapRound
	strokeLineCapSquare
)

type strokeLineJoin uint8

const (
	strokeLineJoinMiter strokeLineJoin = iota
	strokeLineJoinRound
	strokeLineJoinBevel
)

type computedStyle struct {
	fill             paint
	stroke           paint
	strokeWidth      float64
	strokeMiterLimit float64
	fillOpacity      float64
	strokeOpacity    float64
	color            color.NRGBA
	strokeLineCap    strokeLineCap
	strokeLineJoin   strokeLineJoin
	hidden           bool
}

func defaultStyle() computedStyle {
	return computedStyle{
		fill:             paint{kind: paintColor, color: color.NRGBA{A: 255}},
		stroke:           paint{kind: paintNone},
		color:            color.NRGBA{A: 255},
		strokeWidth:      1,
		strokeMiterLimit: 4,
		fillOpacity:      1,
		strokeOpacity:    1,
	}
}

type stylePatch struct {
	stroke              paint
	fill                paint
	strokeMiterLimit    float64
	strokeOpacity       float64
	fillOpacity         float64
	opacity             float64
	strokeWidth         float64
	color               color.NRGBA
	strokeLineCap       strokeLineCap
	opacitySet          bool
	strokeWidthSet      bool
	strokeLineCapSet    bool
	strokeLineJoin      strokeLineJoin
	strokeLineJoinSet   bool
	colorSet            bool
	strokeMiterLimitSet bool
	strokeSet           bool
	fillOpacitySet      bool
	fillSet             bool
	strokeOpacitySet    bool
	hidden              bool
	hiddenSet           bool
}

func (p stylePatch) apply(base computedStyle) computedStyle {
	if p.fillSet {
		base.fill = p.fill
	}
	if p.strokeSet {
		base.stroke = p.stroke
	}
	if p.colorSet {
		base.color = p.color
	}
	if p.strokeWidthSet {
		base.strokeWidth = p.strokeWidth
	}
	if p.strokeMiterLimitSet {
		base.strokeMiterLimit = p.strokeMiterLimit
	}
	if p.strokeLineCapSet {
		base.strokeLineCap = p.strokeLineCap
	}
	if p.strokeLineJoinSet {
		base.strokeLineJoin = p.strokeLineJoin
	}
	if p.fillOpacitySet {
		base.fillOpacity = p.fillOpacity
	}
	if p.strokeOpacitySet {
		base.strokeOpacity = p.strokeOpacity
	}
	if p.hiddenSet {
		base.hidden = p.hidden
	}
	return base
}

func (p stylePatch) opacityValue() float64 {
	if p.opacitySet {
		return p.opacity
	}
	return 1
}

func parseStylePatch(attrs []xml.Attr) stylePatch {
	var patch stylePatch
	var inline string
	for _, attr := range attrs {
		if localName(attr.Name) == "style" {
			inline = attr.Value
			continue
		}
		patch.applyProperty(localName(attr.Name), attr.Value)
	}
	if inline != "" {
		for _, property := range strings.Split(inline, ";") {
			key, value, ok := strings.Cut(property, ":")
			if !ok {
				continue
			}
			patch.applyProperty(strings.TrimSpace(key), strings.TrimSpace(value))
		}
	}
	return patch
}

func (p *stylePatch) applyProperty(name, raw string) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return
	}
	switch name {
	case "fill":
		if paint, ok := parsePaint(value); ok {
			p.fill = paint
			p.fillSet = true
		}
	case "stroke":
		if paint, ok := parsePaint(value); ok {
			p.stroke = paint
			p.strokeSet = true
		}
	case "color":
		if clr, ok := parseColor(value); ok {
			p.color = clr
			p.colorSet = true
		}
	case "stroke-width":
		p.strokeWidth = math.Max(0, parseLength(value, 1))
		p.strokeWidthSet = true
	case "stroke-miterlimit":
		p.strokeMiterLimit = math.Max(1, parseLength(value, 4))
		p.strokeMiterLimitSet = true
	case "stroke-linecap":
		switch strings.ToLower(value) {
		case "butt":
			p.strokeLineCap = strokeLineCapButt
		case "round":
			p.strokeLineCap = strokeLineCapRound
		case "square":
			p.strokeLineCap = strokeLineCapSquare
		default:
			return
		}
		p.strokeLineCapSet = true
	case "stroke-linejoin":
		switch strings.ToLower(value) {
		case "miter":
			p.strokeLineJoin = strokeLineJoinMiter
		case "round":
			p.strokeLineJoin = strokeLineJoinRound
		case "bevel":
			p.strokeLineJoin = strokeLineJoinBevel
		default:
			return
		}
		p.strokeLineJoinSet = true
	case "opacity":
		p.opacity = clamp01(parseLength(value, 1))
		p.opacitySet = true
	case "fill-opacity":
		p.fillOpacity = clamp01(parseLength(value, 1))
		p.fillOpacitySet = true
	case "stroke-opacity":
		p.strokeOpacity = clamp01(parseLength(value, 1))
		p.strokeOpacitySet = true
	case "display":
		p.hidden = value == "none"
		p.hiddenSet = true
	case "visibility":
		p.hidden = value == "hidden" || value == "collapse"
		p.hiddenSet = true
	}
}

func parsePaint(raw string) (paint, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return paint{}, false
	}
	// A paint server reference is resolved against the document gradients when
	// the shape is painted; an unknown reference paints nothing. Only url(#...)
	// is a reference, because a bare "#f00" is a hex color.
	if strings.HasPrefix(value, "url(") {
		if reference := localURLReference(value); reference != "" {
			return paint{kind: paintReference, reference: reference}, true
		}
		return paint{kind: paintNone}, true
	}
	switch strings.ToLower(value) {
	case "none":
		return paint{kind: paintNone}, true
	case "currentcolor":
		return paint{kind: paintCurrentColor}, true
	}
	clr, ok := parseColor(value)
	if !ok {
		return paint{}, false
	}
	return paint{kind: paintColor, color: clr}, true
}

func parseColor(raw string) (color.NRGBA, bool) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return color.NRGBA{}, false
	}
	if named, ok := namedColors[value]; ok {
		return named, true
	}
	if strings.HasPrefix(value, "#") {
		return parseHexColor(value[1:])
	}
	if strings.HasPrefix(value, "rgb(") && strings.HasSuffix(value, ")") {
		return parseFunctionalColor(value[4 : len(value)-1])
	}
	if strings.HasPrefix(value, "rgba(") && strings.HasSuffix(value, ")") {
		return parseFunctionalColor(value[5 : len(value)-1])
	}
	return color.NRGBA{}, false
}

func parseHexColor(value string) (color.NRGBA, bool) {
	var r, g, b, a uint8 = 0, 0, 0, 255
	switch len(value) {
	case 3, 4:
		nibbles := make([]uint8, len(value))
		for i := range value {
			v, ok := hexNibble(value[i])
			if !ok {
				return color.NRGBA{}, false
			}
			nibbles[i] = v*16 + v
		}
		r, g, b = nibbles[0], nibbles[1], nibbles[2]
		if len(nibbles) == 4 {
			a = nibbles[3]
		}
	case 6, 8:
		bytes := make([]uint8, len(value)/2)
		for i := range bytes {
			parsed, err := strconv.ParseUint(value[i*2:i*2+2], 16, 8)
			if err != nil {
				return color.NRGBA{}, false
			}
			bytes[i] = uint8(parsed)
		}
		r, g, b = bytes[0], bytes[1], bytes[2]
		if len(bytes) == 4 {
			a = bytes[3]
		}
	default:
		return color.NRGBA{}, false
	}
	return color.NRGBA{R: r, G: g, B: b, A: a}, true
}

func parseFunctionalColor(raw string) (color.NRGBA, bool) {
	parts := splitColorArgs(raw)
	if len(parts) != 3 && len(parts) != 4 {
		return color.NRGBA{}, false
	}
	r, ok := parseColorComponent(parts[0])
	if !ok {
		return color.NRGBA{}, false
	}
	g, ok := parseColorComponent(parts[1])
	if !ok {
		return color.NRGBA{}, false
	}
	b, ok := parseColorComponent(parts[2])
	if !ok {
		return color.NRGBA{}, false
	}
	a := uint8(255)
	if len(parts) == 4 {
		alpha, ok := parseAlphaComponent(parts[3])
		if !ok {
			return color.NRGBA{}, false
		}
		a = alpha
	}
	return color.NRGBA{R: r, G: g, B: b, A: a}, true
}

func splitColorArgs(raw string) []string {
	replacer := strings.NewReplacer(",", " ", "/", " ")
	return strings.Fields(replacer.Replace(raw))
}

func parseColorComponent(raw string) (uint8, bool) {
	value := strings.TrimSpace(raw)
	if strings.HasSuffix(value, "%") {
		number, err := parseFloat(strings.TrimSuffix(value, "%"))
		if err != nil {
			return 0, false
		}
		return uint8(math.Round(clamp01(number/100) * 255)), true
	}
	number, err := parseFloat(value)
	if err != nil {
		return 0, false
	}
	return uint8(math.Round(math.Max(0, math.Min(255, number)))), true
}

func parseAlphaComponent(raw string) (uint8, bool) {
	value := strings.TrimSpace(raw)
	if strings.HasSuffix(value, "%") {
		number, err := parseFloat(strings.TrimSuffix(value, "%"))
		if err != nil {
			return 0, false
		}
		return uint8(math.Round(clamp01(number/100) * 255)), true
	}
	number, err := parseFloat(value)
	if err != nil {
		return 0, false
	}
	if number > 1 {
		number /= 255
	}
	return uint8(math.Round(clamp01(number) * 255)), true
}

func hexNibble(b byte) (uint8, bool) {
	switch {
	case '0' <= b && b <= '9':
		return b - '0', true
	case 'a' <= b && b <= 'f':
		return b - 'a' + 10, true
	case 'A' <= b && b <= 'F':
		return b - 'A' + 10, true
	default:
		return 0, false
	}
}

func multiplyAlpha(clr color.NRGBA, opacity float64) color.NRGBA {
	clr.A = uint8(math.Round(float64(clr.A) * clamp01(opacity)))
	return clr
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

var namedColors = map[string]color.NRGBA{
	"black":       {A: 255},
	"silver":      {R: 192, G: 192, B: 192, A: 255},
	"gray":        {R: 128, G: 128, B: 128, A: 255},
	"white":       {R: 255, G: 255, B: 255, A: 255},
	"maroon":      {R: 128, A: 255},
	"red":         {R: 255, A: 255},
	"purple":      {R: 128, B: 128, A: 255},
	"fuchsia":     {R: 255, B: 255, A: 255},
	"green":       {G: 128, A: 255},
	"lime":        {G: 255, A: 255},
	"olive":       {R: 128, G: 128, A: 255},
	"yellow":      {R: 255, G: 255, A: 255},
	"navy":        {B: 128, A: 255},
	"blue":        {B: 255, A: 255},
	"teal":        {G: 128, B: 128, A: 255},
	"aqua":        {G: 255, B: 255, A: 255},
	"transparent": {},
	"orange":      {R: 255, G: 165, A: 255},
	"pink":        {R: 255, G: 192, B: 203, A: 255},
	"brown":       {R: 165, G: 42, B: 42, A: 255},
	"cyan":        {G: 255, B: 255, A: 255},
	"magenta":     {R: 255, B: 255, A: 255},
}
