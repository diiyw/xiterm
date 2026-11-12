package font

import (
	"encoding/binary"
	"fmt"
	"slices"
)

const (
	flagOnCurve         = 0x01
	flagXShortVector    = 0x02
	flagYShortVector    = 0x04
	flagRepeat          = 0x08
	flagXSameOrPositive = 0x10
	flagYSameOrPositive = 0x20

	compositeArg1And2AreWords   = 0x0001
	compositeArgsAreXYValues    = 0x0002
	compositeWeHaveAScale       = 0x0008
	compositeMoreComponents     = 0x0020
	compositeWeHaveAnXAndYScale = 0x0040
	compositeWeHaveATwoByTwo    = 0x0080
	compositeWeHaveInstructions = 0x0100
)

type Point struct {
	X       int16
	Y       int16
	OnCurve bool
}

type Bounds struct {
	XMin int16
	YMin int16
	XMax int16
	YMax int16
}

type SimpleGlyph struct {
	EndPtsOfContours []uint16
	Instructions     []byte
	Points           []Point
}

type CompositeComponent struct {
	Transform [4]float64
	GlyphID   uint16
	Arg1      int16
	Arg2      int16
	UseXY     bool
}

type CompositeGlyph struct {
	Components   []CompositeComponent
	Instructions []byte
}

type Glyph struct {
	Simple           *SimpleGlyph
	Composite        *CompositeGlyph
	Bounds           Bounds
	GlyphID          uint16
	NumberOfContours int16
}

func parseGlyph(data []byte, glyphID uint16) (*Glyph, error) {
	if len(data) == 0 {
		return &Glyph{GlyphID: glyphID}, nil
	}
	if len(data) < 10 {
		return nil, fmt.Errorf("glyf entry too short")
	}
	glyph := &Glyph{
		GlyphID:          glyphID,
		NumberOfContours: int16(binary.BigEndian.Uint16(data[0:2])),
		Bounds: Bounds{
			XMin: int16(binary.BigEndian.Uint16(data[2:4])),
			YMin: int16(binary.BigEndian.Uint16(data[4:6])),
			XMax: int16(binary.BigEndian.Uint16(data[6:8])),
			YMax: int16(binary.BigEndian.Uint16(data[8:10])),
		},
	}

	if glyph.NumberOfContours >= 0 {
		simple, err := parseSimpleGlyph(data[10:], glyph.NumberOfContours)
		if err != nil {
			return nil, err
		}
		glyph.Simple = simple
		return glyph, nil
	}

	composite, err := parseCompositeGlyph(data[10:])
	if err != nil {
		return nil, err
	}
	glyph.Composite = composite
	return glyph, nil
}

func parseSimpleGlyph(data []byte, numberOfContours int16) (*SimpleGlyph, error) {
	contourCount := int(numberOfContours)
	if len(data) < contourCount*2+2 {
		return nil, fmt.Errorf("simple glyph truncated contours")
	}
	endPts := make([]uint16, contourCount)
	offset := 0
	for i := 0; i < contourCount; i++ {
		endPts[i] = binary.BigEndian.Uint16(data[offset : offset+2])
		offset += 2
	}
	instructionLength := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if len(data) < offset+instructionLength {
		return nil, fmt.Errorf("simple glyph truncated instructions")
	}
	instructions := append([]byte(nil), data[offset:offset+instructionLength]...)
	offset += instructionLength

	pointCount := 0
	if contourCount > 0 {
		pointCount = int(endPts[len(endPts)-1]) + 1
	}
	var localFlags [256]byte
	flags := localFlags[:0]
	if pointCount > cap(flags) {
		scratch := acquireGlyphRasterScratch()
		defer releaseGlyphRasterScratch(scratch)
		scratch.mask = slices.Grow(scratch.mask[:0], pointCount)[:pointCount]
		flags = scratch.mask[:0]
	}
	for len(flags) < pointCount {
		if offset >= len(data) {
			return nil, fmt.Errorf("simple glyph truncated flags")
		}
		flag := data[offset]
		offset++
		flags = append(flags, flag)
		if flag&flagRepeat != 0 {
			if offset >= len(data) {
				return nil, fmt.Errorf("simple glyph truncated repeat flag")
			}
			repeat := int(data[offset])
			offset++
			if repeat > pointCount-len(flags) {
				return nil, fmt.Errorf("simple glyph repeat exceeds point count")
			}
			for i := 0; i < repeat; i++ {
				flags = append(flags, flag)
			}
		}
	}

	points := make([]Point, pointCount)
	var x int16
	for i, flag := range flags {
		delta, nextOffset, err := readGlyphCoordinate(data, offset, flag, flagXShortVector, flagXSameOrPositive)
		if err != nil {
			return nil, err
		}
		offset = nextOffset
		x += delta
		points[i].X, points[i].OnCurve = x, flag&flagOnCurve != 0
	}

	var y int16
	for i, flag := range flags {
		delta, nextOffset, err := readGlyphCoordinate(data, offset, flag, flagYShortVector, flagYSameOrPositive)
		if err != nil {
			return nil, err
		}
		offset = nextOffset
		y += delta
		points[i].Y = y
	}

	return &SimpleGlyph{EndPtsOfContours: endPts, Instructions: instructions, Points: points}, nil
}

func readGlyphCoordinate(data []byte, offset int, flag, shortFlag, sameFlag byte) (int16, int, error) {
	if flag&shortFlag != 0 {
		if offset >= len(data) {
			return 0, offset, fmt.Errorf("glyph coordinate truncated")
		}
		value := int16(data[offset])
		offset++
		if flag&sameFlag != 0 {
			return value, offset, nil
		}
		return -value, offset, nil
	}
	if flag&sameFlag != 0 {
		return 0, offset, nil
	}
	if offset+2 > len(data) {
		return 0, offset, fmt.Errorf("glyph coordinate truncated")
	}
	value := int16(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2
	return value, offset, nil
}

func parseCompositeGlyph(data []byte) (*CompositeGlyph, error) {
	components := []CompositeComponent{}
	offset := 0
	for {
		if offset+4 > len(data) {
			return nil, fmt.Errorf("composite glyph truncated")
		}
		flags := binary.BigEndian.Uint16(data[offset : offset+2])
		glyphID := binary.BigEndian.Uint16(data[offset+2 : offset+4])
		offset += 4

		argSize := 2
		if flags&compositeArg1And2AreWords != 0 {
			argSize = 4
		}
		if offset+argSize > len(data) {
			return nil, fmt.Errorf("composite glyph truncated args")
		}

		var arg1, arg2 int16
		if flags&compositeArg1And2AreWords != 0 {
			arg1 = int16(binary.BigEndian.Uint16(data[offset : offset+2]))
			arg2 = int16(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		} else {
			arg1 = int16(int8(data[offset]))
			arg2 = int16(int8(data[offset+1]))
		}
		offset += argSize

		component := CompositeComponent{GlyphID: glyphID, Arg1: arg1, Arg2: arg2, UseXY: flags&compositeArgsAreXYValues != 0, Transform: [4]float64{1, 0, 0, 1}}
		var err error
		component.Transform, offset, err = parseCompositeTransform(data, offset, flags)
		if err != nil {
			return nil, err
		}
		components = append(components, component)

		if flags&compositeMoreComponents == 0 {
			break
		}
	}

	instructions := []byte(nil)
	if offset+2 <= len(data) && len(components) > 0 {
		flags := uint16(0)
		for _, component := range components {
			_ = component
		}
		// The instructions bit only matters on the last component; re-read from previous loop state is unnecessary here.
		_ = flags
	}
	if offset+2 <= len(data) {
		instructionLength := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		if offset+2+instructionLength <= len(data) {
			instructions = append([]byte(nil), data[offset+2:offset+2+instructionLength]...)
		}
	}

	return &CompositeGlyph{Components: components, Instructions: instructions}, nil
}

func parseCompositeTransform(data []byte, offset int, flags uint16) ([4]float64, int, error) {
	transform := [4]float64{1, 0, 0, 1}
	switch {
	case flags&compositeWeHaveATwoByTwo != 0:
		if offset+8 > len(data) {
			return transform, offset, fmt.Errorf("composite glyph truncated 2x2 transform")
		}
		transform[0] = f2dot14(data[offset : offset+2])
		transform[1] = f2dot14(data[offset+2 : offset+4])
		transform[2] = f2dot14(data[offset+4 : offset+6])
		transform[3] = f2dot14(data[offset+6 : offset+8])
		offset += 8
	case flags&compositeWeHaveAnXAndYScale != 0:
		if offset+4 > len(data) {
			return transform, offset, fmt.Errorf("composite glyph truncated xy scale")
		}
		transform[0] = f2dot14(data[offset : offset+2])
		transform[3] = f2dot14(data[offset+2 : offset+4])
		offset += 4
	case flags&compositeWeHaveAScale != 0:
		if offset+2 > len(data) {
			return transform, offset, fmt.Errorf("composite glyph truncated scale")
		}
		scale := f2dot14(data[offset : offset+2])
		transform[0] = scale
		transform[3] = scale
		offset += 2
	}
	return transform, offset, nil
}

func f2dot14(data []byte) float64 {
	return float64(int16(binary.BigEndian.Uint16(data))) / 16384.0
}
