// Package svg decodes and rasterizes SVG documents without external
// dependencies.
package svg

import (
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"io"
	"strings"
)

type Rect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

type Document struct {
	definitions         map[string]element
	clipPaths           map[string]groupElement
	gradients           map[string]*gradient
	children            []element
	style               stylePatch
	transform           matrix
	ViewBox             Rect
	PreserveAspectRatio PreserveAspectRatio
	Width               float64
	Height              float64
	HasViewBox          bool
}

type Options struct {
	Background color.Color
	Width      int
	Height     int
}

type PreserveAspectRatio struct {
	X     float64
	Y     float64
	None  bool
	Slice bool
}

func Decode(r io.Reader) (*Document, error) {
	decoder := xml.NewDecoder(r)
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("svg: missing root element")
			}
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if localName(start.Name) != "svg" {
			return nil, fmt.Errorf("svg: expected <svg>, got <%s>", localName(start.Name))
		}
		return parseDocument(decoder, start)
	}
}

func DecodeConfig(r io.Reader) (image.Config, error) {
	doc, err := Decode(r)
	if err != nil {
		return image.Config{}, err
	}
	width, height := doc.outputSize(Options{})
	return image.Config{ColorModel: color.NRGBAModel, Width: width, Height: height}, nil
}

func Render(r io.Reader, opts Options) (*image.NRGBA, error) {
	doc, err := Decode(r)
	if err != nil {
		return nil, err
	}
	return doc.Render(opts)
}

func RenderString(src string, opts Options) (*image.NRGBA, error) {
	return Render(strings.NewReader(src), opts)
}

func (d *Document) Render(opts Options) (*image.NRGBA, error) {
	if d == nil {
		return nil, fmt.Errorf("svg: document is nil")
	}
	width, height := d.outputSize(opts)
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("svg: invalid render size %dx%d", width, height)
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	if opts.Background != nil {
		fillImage(img, opts.Background)
	}

	state := renderState{
		style:     d.style.apply(defaultStyle()),
		transform: viewportMatrix(d.viewport(), width, height, d.PreserveAspectRatio).Mul(d.transform),
		context: &renderContext{
			definitions: d.definitions,
			clipPaths:   d.clipPaths,
			gradients:   d.gradients,
		},
	}
	r := renderer{img: img}
	render := func(r *renderer) {
		for _, child := range d.children {
			child.render(r, state)
		}
	}
	if opacity := d.style.opacityValue(); opacity < 1 {
		r.Layer(opacity, render)
	} else {
		render(&r)
	}
	return img, nil
}

func (d *Document) Rasterize(width, height int) (*image.NRGBA, error) {
	return d.Render(Options{Width: width, Height: height})
}

func parseDocument(decoder *xml.Decoder, start xml.StartElement) (*Document, error) {
	attrs := attrsOf(start.Attr)
	doc := &Document{
		Width:               parseLength(attrs["width"], 0),
		Height:              parseLength(attrs["height"], 0),
		PreserveAspectRatio: parsePreserveAspectRatio(attrs["preserveaspectratio"]),
		style:               parseStylePatch(start.Attr),
		transform:           parseTransform(attrs["transform"]),
	}
	if viewBox, ok := parseViewBox(attrs["viewbox"]); ok {
		doc.ViewBox = viewBox
		doc.HasViewBox = true
	}
	if doc.Width <= 0 && doc.HasViewBox {
		doc.Width = doc.ViewBox.Width
	}
	if doc.Height <= 0 && doc.HasViewBox {
		doc.Height = doc.ViewBox.Height
	}
	if doc.Width <= 0 {
		doc.Width = 300
	}
	if doc.Height <= 0 {
		doc.Height = 150
	}

	parser := documentParser{decoder: decoder}
	children, err := parser.parseChildren("svg")
	if err != nil {
		return nil, err
	}
	doc.children = children
	doc.definitions = parser.definitions
	doc.clipPaths = parser.clipPaths
	doc.gradients = resolveGradients(parser.gradients, doc.viewport())
	return doc, nil
}

func (d *Document) outputSize(opts Options) (int, int) {
	width := opts.Width
	height := opts.Height
	naturalWidth := roundPositive(d.Width)
	naturalHeight := roundPositive(d.Height)
	if width <= 0 && height <= 0 {
		width = naturalWidth
		height = naturalHeight
	}
	if width <= 0 && height > 0 {
		width = int(float64(height) * d.aspect())
	}
	if height <= 0 && width > 0 {
		height = int(float64(width) / d.aspect())
	}
	return width, height
}

func (d *Document) aspect() float64 {
	if d.Height > 0 {
		return d.Width / d.Height
	}
	if d.HasViewBox && d.ViewBox.Height > 0 {
		return d.ViewBox.Width / d.ViewBox.Height
	}
	return 1
}

func (d *Document) viewport() Rect {
	if d.HasViewBox {
		return d.ViewBox
	}
	return Rect{Width: d.Width, Height: d.Height}
}

type documentParser struct {
	decoder     *xml.Decoder
	definitions map[string]element
	clipPaths   map[string]groupElement
	gradients   map[string]*gradientDef
}

func (p *documentParser) parseChildren(end string) ([]element, error) {
	var children []element
	for {
		token, err := p.decoder.Token()
		if err != nil {
			if err == io.EOF {
				return children, nil
			}
			return nil, err
		}
		switch token := token.(type) {
		case xml.StartElement:
			child, err := p.parseElement(token)
			if err != nil {
				return nil, err
			}
			if child != nil {
				children = append(children, child)
			}
		case xml.EndElement:
			if localName(token.Name) == end {
				return children, nil
			}
		}
	}
}

func (p *documentParser) parseElement(start xml.StartElement) (element, error) {
	attrs := attrsOf(start.Attr)
	var (
		parsed element
		err    error
	)
	switch name := localName(start.Name); name {
	case "g", "svg":
		var children []element
		children, err = p.parseChildren(name)
		if err != nil {
			return nil, err
		}
		parsed = groupElement{properties: parseElementProperties(start), children: children}
	case "defs":
		_, err = p.parseChildren(name)
		return nil, err
	case "lineargradient", "radialgradient":
		definition := parseGradientDefinition(start, name == "lineargradient")
		definition.stops, err = p.parseGradientChildren(name)
		if err != nil {
			return nil, err
		}
		// A gradient without stops of its own inherits them from its href, so an
		// empty stop list must not override the referenced one.
		definition.stopsSet = len(definition.stops) > 0
		if id := strings.TrimSpace(attrs["id"]); id != "" {
			if p.gradients == nil {
				p.gradients = make(map[string]*gradientDef)
			}
			p.gradients[id] = definition
		}
		return nil, nil
	case "clippath":
		var children []element
		children, err = p.parseChildren(name)
		if err != nil {
			return nil, err
		}
		if id := strings.TrimSpace(attrs["id"]); id != "" {
			if p.clipPaths == nil {
				p.clipPaths = make(map[string]groupElement)
			}
			p.clipPaths[id] = groupElement{properties: parseElementProperties(start), children: children}
		}
		return nil, nil
	case "use":
		err = p.decoder.Skip()
		parsed = useElement{
			properties: parseElementProperties(start),
			reference:  localURLReference(attrs["href"]),
			x:          parseLength(attrs["x"], 0),
			y:          parseLength(attrs["y"], 0),
		}
	case "rect", "circle", "ellipse", "line", "polyline", "polygon", "path":
		parsed, err = parseShape(start)
		if skipErr := p.decoder.Skip(); skipErr != nil && err == nil {
			err = skipErr
		}
	default:
		return nil, p.decoder.Skip()
	}
	if err != nil {
		return nil, err
	}
	if id := strings.TrimSpace(attrs["id"]); id != "" && parsed != nil {
		if p.definitions == nil {
			p.definitions = make(map[string]element)
		}
		p.definitions[id] = parsed
	}
	return parsed, nil
}

type renderContext struct {
	definitions map[string]element
	clipPaths   map[string]groupElement
	gradients   map[string]*gradient
	active      map[string]struct{}
}

func (c *renderContext) renderReference(r *renderer, id string, state renderState) {
	target, ok := c.definitions[id]
	if !ok || !c.enter("use:"+id) {
		return
	}
	defer c.leave("use:" + id)
	target.render(r, state)
}

func (c *renderContext) renderClipped(r *renderer, id string, transform matrix, content func(*renderer)) {
	clip, ok := c.clipPaths[id]
	if !ok {
		content(r)
		return
	}
	if !c.enter("clip:" + id) {
		return
	}
	defer c.leave("clip:" + id)
	r.Clip(func(mask *renderer) {
		clip.render(mask, renderState{
			style:     defaultStyle(),
			transform: transform,
			context:   c,
			clipping:  true,
		})
	}, content)
}

func (c *renderContext) enter(key string) bool {
	if _, exists := c.active[key]; exists {
		return false
	}
	if c.active == nil {
		c.active = make(map[string]struct{})
	}
	c.active[key] = struct{}{}
	return true
}

func (c *renderContext) leave(key string) {
	delete(c.active, key)
}

func presentationProperty(attrs []xml.Attr, name string) string {
	value := ""
	inline := ""
	for _, attr := range attrs {
		switch localName(attr.Name) {
		case name:
			value = attr.Value
		case "style":
			inline = attr.Value
		}
	}
	for _, property := range strings.Split(inline, ";") {
		key, candidate, ok := strings.Cut(property, ":")
		if ok && strings.EqualFold(strings.TrimSpace(key), name) {
			value = strings.TrimSpace(candidate)
		}
	}
	return value
}

func localURLReference(raw string) string {
	value := strings.TrimSpace(raw)
	if len(value) >= 5 && strings.EqualFold(value[:4], "url(") && value[len(value)-1] == ')' {
		value = strings.TrimSpace(value[4 : len(value)-1])
		value = strings.Trim(value, "\"'")
	}
	if !strings.HasPrefix(value, "#") {
		return ""
	}
	return strings.TrimSpace(value[1:])
}

func attrsOf(attrs []xml.Attr) map[string]string {
	out := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		out[localName(attr.Name)] = attr.Value
	}
	return out
}

func localName(name xml.Name) string {
	return strings.ToLower(name.Local)
}

func roundPositive(v float64) int {
	if v <= 0 {
		return 0
	}
	return int(v + 0.5)
}

func parsePreserveAspectRatio(raw string) PreserveAspectRatio {
	value := strings.TrimSpace(raw)
	if value == "" {
		return PreserveAspectRatio{X: 0.5, Y: 0.5}
	}
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return PreserveAspectRatio{X: 0.5, Y: 0.5}
	}
	if parts[0] == "none" {
		return PreserveAspectRatio{None: true}
	}
	preserve := PreserveAspectRatio{X: 0.5, Y: 0.5}
	if strings.Contains(parts[0], "xMin") {
		preserve.X = 0
	} else if strings.Contains(parts[0], "xMax") {
		preserve.X = 1
	}
	if strings.Contains(parts[0], "YMin") {
		preserve.Y = 0
	} else if strings.Contains(parts[0], "YMax") {
		preserve.Y = 1
	}
	if len(parts) > 1 && parts[1] == "slice" {
		preserve.Slice = true
	}
	return preserve
}
