package font

import (
	"fmt"
	"slices"
	"weak"
)

type OutlinePoint struct {
	X float64
	Y float64
}

type OutlineContour []OutlinePoint

type Outline struct {
	Contours []OutlineContour
	Bounds   Bounds
}

func (f *Face) Outline(glyphID uint16) (*Outline, error) {
	return f.loadOutline(glyphID, 0)
}

func (f *Face) loadOutline(glyphID uint16, depth int) (*Outline, error) {
	if f == nil {
		return nil, fmt.Errorf("face is nil")
	}
	f.cacheMu.RLock()
	if outline := f.outlineCache[glyphID].Value(); outline != nil {
		f.cacheMu.RUnlock()
		return outline, nil
	}
	f.cacheMu.RUnlock()

	var (
		outline *Outline
		err     error
	)
	if f.italicSource != nil {
		outline, err = f.italicOutline(glyphID)
	} else if f.CFF != nil {
		outline, err = f.CFF.outline(glyphID)
	} else {
		outline, err = f.outline(glyphID, depth)
	}
	if err != nil {
		return nil, err
	}
	if outline == nil {
		return nil, nil
	}
	f.cacheMu.Lock()
	f.ensureCachesLocked()
	if cached := f.outlineCache[glyphID].Value(); cached != nil {
		f.cacheMu.Unlock()
		return cached, nil
	}
	// Rasterized masks and atlas glyphs no longer need their source contours.
	// Share outlines while callers use them without pinning them for the face's
	// lifetime; a later size or subpixel phase can regenerate a collected one.
	if len(f.outlineCache) >= 4096 {
		clear(f.outlineCache)
	}
	f.outlineCache[glyphID] = weak.Make(outline)
	f.cacheMu.Unlock()
	return outline, nil
}

func (f *Face) outline(glyphID uint16, depth int) (*Outline, error) {
	if depth > 8 {
		return nil, fmt.Errorf("composite glyph recursion too deep")
	}
	glyph, err := f.Glyph(glyphID)
	if err != nil {
		return nil, err
	}
	outline := &Outline{Bounds: glyph.Bounds}
	if glyph.Simple != nil {
		contours, err := simpleGlyphContours(glyph.Simple)
		if err != nil {
			return nil, err
		}
		outline.Contours = contours
		return outline, nil
	}
	if glyph.Composite == nil {
		return outline, nil
	}

	for _, component := range glyph.Composite.Components {
		child, err := f.loadOutline(component.GlyphID, depth+1)
		if err != nil {
			return nil, err
		}
		for _, contour := range child.Contours {
			transformed := make(OutlineContour, len(contour))
			for i, point := range contour {
				transformed[i] = OutlinePoint{
					X: component.Transform[0]*point.X + component.Transform[1]*point.Y + float64(component.Arg1),
					Y: component.Transform[2]*point.X + component.Transform[3]*point.Y + float64(component.Arg2),
				}
			}
			outline.Contours = append(outline.Contours, transformed)
		}
	}
	return outline, nil
}

func simpleGlyphContours(glyph *SimpleGlyph) ([]OutlineContour, error) {
	contours := make([]OutlineContour, 0, len(glyph.EndPtsOfContours))
	start := 0
	for _, end := range glyph.EndPtsOfContours {
		if int(end) >= len(glyph.Points) || start > int(end)+1 {
			return nil, fmt.Errorf("simple glyph contour indices out of range")
		}
		points := glyph.Points[start : int(end)+1]
		contour, err := flattenContour(points, 0.5)
		if err != nil {
			return nil, err
		}
		contours = append(contours, contour)
		start = int(end) + 1
	}
	return contours, nil
}

func flattenContour(points []Point, tolerance float64) (OutlineContour, error) {
	if len(points) == 0 {
		return nil, nil
	}
	// TrueType omits on-curve points between consecutive off-curve points.
	// Materialize those implied midpoints before flattening so a long run of
	// control points remains a smooth sequence of quadratic segments.
	var local [256]contourPoint
	expanded := local[:0]
	if count := len(points) * 2; count > cap(expanded) {
		scratch := acquireGlyphRasterScratch()
		defer releaseGlyphRasterScratch(scratch)
		scratch.expanded = slices.Grow(scratch.expanded[:0], count)[:count]
		expanded = scratch.expanded[:0]
	}
	for index, point := range points {
		expanded = append(expanded, contourPoint{point: toOutlinePoint(point), onCurve: point.OnCurve})
		next := points[(index+1)%len(points)]
		if !point.OnCurve && !next.OnCurve {
			expanded = append(expanded, contourPoint{point: midpoint(toOutlinePoint(point), toOutlinePoint(next)), onCurve: true})
		}
	}
	startIndex := 0
	for index, point := range expanded {
		if point.onCurve {
			startIndex = index
			break
		}
	}
	start := expanded[startIndex].point
	current := start
	contour := OutlineContour{current}
	for offset := 1; offset < len(expanded); {
		point := expanded[(startIndex+offset)%len(expanded)]
		if point.onCurve {
			next := point.point
			contour = append(contour, next)
			current = next
			offset++
			continue
		}
		next := expanded[(startIndex+offset+1)%len(expanded)]
		if !next.onCurve {
			return nil, fmt.Errorf("contour contains consecutive off-curve points")
		}
		end := next.point
		contour = appendQuadratic(contour, current, point.point, end, tolerance)
		current = end
		offset += 2
	}

	if len(contour) > 1 {
		last := contour[len(contour)-1]
		if last != contour[0] {
			contour = append(contour, contour[0])
		}
	}
	return contour, nil
}

func toOutlinePoint(point Point) OutlinePoint {
	return OutlinePoint{X: float64(point.X), Y: float64(point.Y)}
}

func midpoint(a, b OutlinePoint) OutlinePoint {
	return OutlinePoint{X: (a.X + b.X) * 0.5, Y: (a.Y + b.Y) * 0.5}
}

func appendQuadratic(dst OutlineContour, p0, p1, p2 OutlinePoint, tolerance float64) OutlineContour {
	if quadraticFlatEnough(p0, p1, p2, tolerance) {
		return append(dst, p2)
	}
	p01 := midpoint(p0, p1)
	p12 := midpoint(p1, p2)
	p012 := midpoint(p01, p12)
	dst = appendQuadratic(dst, p0, p01, p012, tolerance)
	return appendQuadratic(dst, p012, p12, p2, tolerance)
}

func quadraticFlatEnough(p0, p1, p2 OutlinePoint, tolerance float64) bool {
	dx := p0.X - 2*p1.X + p2.X
	dy := p0.Y - 2*p1.Y + p2.Y
	return dx*dx+dy*dy <= tolerance*tolerance*4
}
