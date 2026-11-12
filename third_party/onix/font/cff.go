package font

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	cffOpCharset     = 15
	cffOpCharStrings = 17
	cffOpPrivate     = 18
	cffOpSubrs       = 19
	cffOpFDArray     = (12 << 8) | 36
	cffOpFDSelect    = (12 << 8) | 37
)

type cffFont struct {
	Names       []string
	TopDict     map[int][]float64
	Charset     []uint16
	CharStrings cffIndex
	GlobalSubrs cffIndex
	LocalSubrs  cffIndex
	FDSelect    []uint8
	FontDicts   []cffFontDict
}

type cffFontDict struct {
	Dict       map[int][]float64
	Private    map[int][]float64
	LocalSubrs cffIndex
}

type cffType2State struct {
	stack     []float64
	current   OutlineContour
	contours  []OutlineContour
	x         float64
	y         float64
	hintCount int
}

func parseCFF(data []byte, numGlyphs uint16) (*cffFont, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("cff header too short")
	}
	hdrSize := int(data[2])
	if hdrSize < 4 || hdrSize > len(data) {
		return nil, fmt.Errorf("invalid cff header size %d", hdrSize)
	}

	offset := hdrSize
	nameIndex, nextOffset, err := parseCFFIndex(data, offset)
	if err != nil {
		return nil, fmt.Errorf("parse cff name index: %w", err)
	}
	offset = nextOffset

	topDictIndex, nextOffset, err := parseCFFIndex(data, offset)
	if err != nil {
		return nil, fmt.Errorf("parse cff top dict index: %w", err)
	}
	if topDictIndex.Len() == 0 {
		return nil, fmt.Errorf("cff top dict index is empty")
	}
	offset = nextOffset

	_, nextOffset, err = parseCFFIndex(data, offset)
	if err != nil {
		return nil, fmt.Errorf("parse cff string index: %w", err)
	}
	offset = nextOffset

	globalSubrs, _, err := parseCFFIndex(data, offset)
	if err != nil {
		return nil, fmt.Errorf("parse cff global subrs: %w", err)
	}

	topDict, err := parseCFFDict(topDictIndex.At(0))
	if err != nil {
		return nil, fmt.Errorf("parse cff top dict: %w", err)
	}

	charStringsOffset, ok := dictInt(topDict, cffOpCharStrings, 0)
	if !ok {
		return nil, fmt.Errorf("cff top dict missing CharStrings offset")
	}
	charStrings, _, err := parseCFFIndex(data, charStringsOffset)
	if err != nil {
		return nil, fmt.Errorf("parse cff charstrings: %w", err)
	}
	if charStrings.Len() == 0 {
		return nil, fmt.Errorf("cff charstrings index is empty")
	}
	if numGlyphs != 0 && charStrings.Len() != int(numGlyphs) {
		return nil, fmt.Errorf("cff charstrings count %d does not match maxp glyph count %d", charStrings.Len(), numGlyphs)
	}

	charsetOffset, _ := dictInt(topDict, cffOpCharset, 0)
	charset := make([]uint16, charStrings.Len())
	if charStrings.Len() > 0 {
		charset[0] = 0
	}
	if charsetOffset > 0 && charStrings.Len() > 1 {
		parsedCharset, err := parseCFFCharset(data, charsetOffset, charStrings.Len())
		if err != nil {
			return nil, fmt.Errorf("parse cff charset: %w", err)
		}
		charset = parsedCharset
	}

	localSubrs := cffIndex{}
	if privateSize, ok := dictInt(topDict, cffOpPrivate, 0); ok {
		privateOffset, ok := dictInt(topDict, cffOpPrivate, 1)
		if !ok {
			return nil, fmt.Errorf("cff top dict private offset missing")
		}
		localSubrs, _, err = parseCFFPrivateLocalSubrs(data, privateOffset, privateSize)
		if err != nil {
			return nil, err
		}
	}

	fdSelect := []uint8(nil)
	fontDicts := []cffFontDict(nil)
	if fdArrayOffset, ok := dictInt(topDict, cffOpFDArray, 0); ok {
		fdArrayData, _, err := parseCFFIndex(data, fdArrayOffset)
		if err != nil {
			return nil, fmt.Errorf("parse cff fdarray: %w", err)
		}
		fontDicts = make([]cffFontDict, fdArrayData.Len())
		for i := range fdArrayData.Len() {
			rawDict := fdArrayData.At(i)
			dict, err := parseCFFDict(rawDict)
			if err != nil {
				return nil, fmt.Errorf("parse cff font dict %d: %w", i, err)
			}
			fontDicts[i].Dict = dict
			if privateSize, ok := dictInt(dict, cffOpPrivate, 0); ok {
				privateOffset, ok := dictInt(dict, cffOpPrivate, 1)
				if !ok {
					return nil, fmt.Errorf("cff font dict %d private offset missing", i)
				}
				localSubrs, privateDict, err := parseCFFPrivateLocalSubrs(data, privateOffset, privateSize)
				if err != nil {
					return nil, fmt.Errorf("parse cff font dict %d private dict: %w", i, err)
				}
				fontDicts[i].Private = privateDict
				fontDicts[i].LocalSubrs = localSubrs
			}
		}
		fdSelectOffset, ok := dictInt(topDict, cffOpFDSelect, 0)
		if !ok {
			return nil, fmt.Errorf("cff top dict missing FDSelect offset")
		}
		fdSelect, err = parseCFFFDSelect(data, fdSelectOffset, charStrings.Len(), len(fontDicts))
		if err != nil {
			return nil, fmt.Errorf("parse cff fdselect: %w", err)
		}
	}

	names := make([]string, nameIndex.Len())
	for i := range nameIndex.Len() {
		names[i] = string(nameIndex.At(i))
	}

	return &cffFont{
		Names:       names,
		TopDict:     topDict,
		Charset:     charset,
		CharStrings: charStrings,
		GlobalSubrs: globalSubrs,
		LocalSubrs:  localSubrs,
		FDSelect:    fdSelect,
		FontDicts:   fontDicts,
	}, nil
}

func parseCFFPrivateLocalSubrs(data []byte, privateOffset, privateSize int) (cffIndex, map[int][]float64, error) {
	if privateOffset < 0 || privateSize < 0 || privateOffset+privateSize > len(data) {
		return cffIndex{}, nil, fmt.Errorf("cff private dict out of bounds")
	}
	privateDict, err := parseCFFDict(data[privateOffset : privateOffset+privateSize])
	if err != nil {
		return cffIndex{}, nil, fmt.Errorf("parse cff private dict: %w", err)
	}
	localSubrs := cffIndex{}
	if subrsRelativeOffset, ok := dictInt(privateDict, cffOpSubrs, 0); ok {
		localSubrsOffset := privateOffset + subrsRelativeOffset
		localSubrs, _, err = parseCFFIndex(data, localSubrsOffset)
		if err != nil {
			return cffIndex{}, nil, fmt.Errorf("parse cff local subrs: %w", err)
		}
	}
	return localSubrs, privateDict, nil
}

// CFF INDEX tables remain in their mapped font backing. A slice header per
// glyph/subroutine otherwise costs several MiB for a single CJK face.
type cffIndex struct {
	data    []byte
	offsets []byte
	count   int
	offSize int
}

func (index cffIndex) Len() int { return index.count }

func (index cffIndex) At(i int) []byte {
	start := readUint(index.offsets[i*index.offSize:], index.offSize) - 1
	end := readUint(index.offsets[(i+1)*index.offSize:], index.offSize) - 1
	return index.data[start:end:end]
}

func parseCFFIndex(data []byte, offset int) (cffIndex, int, error) {
	if offset < 0 || offset > len(data)-2 {
		return cffIndex{}, offset, fmt.Errorf("cff index truncated")
	}
	count := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if count == 0 {
		return cffIndex{}, offset, nil
	}
	if offset >= len(data) {
		return cffIndex{}, offset, fmt.Errorf("cff index missing offsize")
	}
	offSize := int(data[offset])
	offset++
	if offSize < 1 || offSize > 4 {
		return cffIndex{}, offset, fmt.Errorf("invalid cff offsize %d", offSize)
	}
	end := offset + (count+1)*offSize
	if end > len(data) {
		return cffIndex{}, offset, fmt.Errorf("cff index offsets truncated")
	}
	offsets := data[offset:end]
	previous := readUint(offsets, offSize)
	if previous != 1 {
		return cffIndex{}, end, fmt.Errorf("cff index first offset must be 1")
	}
	for i := 1; i <= count; i++ {
		next := readUint(offsets[i*offSize:], offSize)
		if next < previous || next-1 > len(data)-end {
			return cffIndex{}, end, fmt.Errorf("cff index object out of bounds")
		}
		previous = next
	}
	return cffIndex{data: data[end : end+previous-1], offsets: offsets, count: count, offSize: offSize}, end + previous - 1, nil
}

func readUint(data []byte, size int) int {
	value := 0
	for i := 0; i < size; i++ {
		value = value<<8 | int(data[i])
	}
	return value
}

func parseCFFDict(data []byte) (map[int][]float64, error) {
	dict := map[int][]float64{}
	operands := []float64{}
	for i := 0; i < len(data); {
		b := data[i]
		if isCFFDictNumber(b) {
			value, next, err := parseCFFDictNumber(data, i)
			if err != nil {
				return nil, err
			}
			operands = append(operands, value)
			i = next
			continue
		}

		op := int(b)
		i++
		if b == 12 {
			if i >= len(data) {
				return nil, fmt.Errorf("cff dict escaped operator truncated")
			}
			op = (int(b) << 8) | int(data[i])
			i++
		}
		dict[op] = append([]float64(nil), operands...)
		operands = operands[:0]
	}
	if len(operands) != 0 {
		return nil, fmt.Errorf("cff dict ended with dangling operands")
	}
	return dict, nil
}

func isCFFDictNumber(b byte) bool {
	return b == 28 || b == 29 || b == 30 || b >= 32
}

func parseCFFDictNumber(data []byte, offset int) (float64, int, error) {
	if offset >= len(data) {
		return 0, offset, fmt.Errorf("cff dict number truncated")
	}
	b := data[offset]
	switch {
	case b >= 32 && b <= 246:
		return float64(int(b) - 139), offset + 1, nil
	case b >= 247 && b <= 250:
		if offset+1 >= len(data) {
			return 0, offset, fmt.Errorf("cff dict number truncated")
		}
		return float64((int(b)-247)*256 + int(data[offset+1]) + 108), offset + 2, nil
	case b >= 251 && b <= 254:
		if offset+1 >= len(data) {
			return 0, offset, fmt.Errorf("cff dict number truncated")
		}
		return float64(-(int(b)-251)*256 - int(data[offset+1]) - 108), offset + 2, nil
	case b == 28:
		if offset+2 >= len(data) {
			return 0, offset, fmt.Errorf("cff dict shortint truncated")
		}
		return float64(int16(binary.BigEndian.Uint16(data[offset+1 : offset+3]))), offset + 3, nil
	case b == 29:
		if offset+4 >= len(data) {
			return 0, offset, fmt.Errorf("cff dict longint truncated")
		}
		return float64(int32(binary.BigEndian.Uint32(data[offset+1 : offset+5]))), offset + 5, nil
	case b == 30:
		return parseCFFRealNumber(data, offset)
	default:
		return 0, offset, fmt.Errorf("unsupported cff dict number byte %d", b)
	}
}

func parseCFFRealNumber(data []byte, offset int) (float64, int, error) {
	if offset >= len(data) || data[offset] != 30 {
		return 0, offset, fmt.Errorf("invalid cff real number")
	}
	offset++
	var builder strings.Builder
	for {
		if offset >= len(data) {
			return 0, offset, fmt.Errorf("cff real number truncated")
		}
		for _, nibble := range []byte{data[offset] >> 4, data[offset] & 0x0f} {
			switch nibble {
			case 0x0:
				builder.WriteByte('0')
			case 0x1:
				builder.WriteByte('1')
			case 0x2:
				builder.WriteByte('2')
			case 0x3:
				builder.WriteByte('3')
			case 0x4:
				builder.WriteByte('4')
			case 0x5:
				builder.WriteByte('5')
			case 0x6:
				builder.WriteByte('6')
			case 0x7:
				builder.WriteByte('7')
			case 0x8:
				builder.WriteByte('8')
			case 0x9:
				builder.WriteByte('9')
			case 0xA:
				builder.WriteByte('.')
			case 0xB:
				builder.WriteString("E")
			case 0xC:
				builder.WriteString("E-")
			case 0xE:
				builder.WriteByte('-')
			case 0xF:
				value, err := strconv.ParseFloat(builder.String(), 64)
				if err != nil {
					return 0, offset + 1, fmt.Errorf("parse cff real number %q: %w", builder.String(), err)
				}
				return value, offset + 1, nil
			default:
				return 0, offset + 1, fmt.Errorf("unsupported cff real nibble %x", nibble)
			}
		}
		offset++
	}
}

func dictInt(dict map[int][]float64, op int, index int) (int, bool) {
	values, ok := dict[op]
	if !ok || index >= len(values) {
		return 0, false
	}
	return int(math.Round(values[index])), true
}

func parseCFFCharset(data []byte, offset, glyphCount int) ([]uint16, error) {
	if offset >= len(data) {
		return nil, fmt.Errorf("cff charset offset out of bounds")
	}
	charset := make([]uint16, glyphCount)
	charset[0] = 0
	if glyphCount == 1 {
		return charset, nil
	}

	format := data[offset]
	offset++
	position := 1
	switch format {
	case 0:
		needed := (glyphCount - 1) * 2
		if offset+needed > len(data) {
			return nil, fmt.Errorf("cff charset format 0 truncated")
		}
		for i := 1; i < glyphCount; i++ {
			charset[i] = binary.BigEndian.Uint16(data[offset : offset+2])
			offset += 2
		}
	case 1:
		for position < glyphCount {
			if offset+3 > len(data) {
				return nil, fmt.Errorf("cff charset format 1 truncated")
			}
			first := binary.BigEndian.Uint16(data[offset : offset+2])
			nLeft := int(data[offset+2])
			offset += 3
			for i := 0; i <= nLeft && position < glyphCount; i++ {
				charset[position] = first + uint16(i)
				position++
			}
		}
	case 2:
		for position < glyphCount {
			if offset+4 > len(data) {
				return nil, fmt.Errorf("cff charset format 2 truncated")
			}
			first := binary.BigEndian.Uint16(data[offset : offset+2])
			nLeft := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
			offset += 4
			for i := 0; i <= nLeft && position < glyphCount; i++ {
				charset[position] = first + uint16(i)
				position++
			}
		}
	default:
		return nil, fmt.Errorf("unsupported cff charset format %d", format)
	}
	return charset, nil
}

func parseCFFFDSelect(data []byte, offset, glyphCount, fontDictCount int) ([]uint8, error) {
	if offset >= len(data) {
		return nil, fmt.Errorf("cff fdselect offset out of bounds")
	}
	format := data[offset]
	offset++
	selects := make([]uint8, glyphCount)
	switch format {
	case 0:
		if offset+glyphCount > len(data) {
			return nil, fmt.Errorf("cff fdselect format 0 truncated")
		}
		for i := 0; i < glyphCount; i++ {
			selects[i] = data[offset+i]
			if selects[i] < 0 || int(selects[i]) >= fontDictCount {
				return nil, fmt.Errorf("cff fdselect font dict index %d out of range", selects[i])
			}
		}
	case 3:
		if offset+2 > len(data) {
			return nil, fmt.Errorf("cff fdselect format 3 truncated range count")
		}
		rangeCount := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2
		if offset+rangeCount*3+2 > len(data) {
			return nil, fmt.Errorf("cff fdselect format 3 truncated ranges")
		}
		type fdRange struct {
			first int
			fd    int
		}
		ranges := make([]fdRange, rangeCount)
		for i := 0; i < rangeCount; i++ {
			ranges[i] = fdRange{
				first: int(binary.BigEndian.Uint16(data[offset : offset+2])),
				fd:    int(data[offset+2]),
			}
			offset += 3
			if ranges[i].fd < 0 || ranges[i].fd >= fontDictCount {
				return nil, fmt.Errorf("cff fdselect font dict index %d out of range", ranges[i].fd)
			}
		}
		sentinel := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		for i, current := range ranges {
			limit := sentinel
			if i+1 < len(ranges) {
				limit = ranges[i+1].first
			}
			if current.first > limit || limit > glyphCount {
				return nil, fmt.Errorf("cff fdselect invalid range %d..%d", current.first, limit)
			}
			for glyphID := current.first; glyphID < limit; glyphID++ {
				selects[glyphID] = uint8(current.fd)
			}
		}
	default:
		return nil, fmt.Errorf("unsupported cff fdselect format %d", format)
	}
	return selects, nil
}

func (c *cffFont) outline(glyphID uint16) (*Outline, error) {
	if c == nil {
		return nil, fmt.Errorf("cff font is nil")
	}
	if int(glyphID) >= c.CharStrings.Len() {
		return nil, fmt.Errorf("cff glyph id %d out of range", glyphID)
	}
	localSubrs, err := c.localSubrsForGlyph(glyphID)
	if err != nil {
		return nil, err
	}
	state := &cffType2State{}
	if err := c.execType2Charstring(c.CharStrings.At(int(glyphID)), localSubrs, state, 0); err != nil {
		return nil, err
	}
	state.finishContour()
	return &Outline{Contours: state.contours, Bounds: outlineBounds(state.contours)}, nil
}

func (c *cffFont) localSubrsForGlyph(glyphID uint16) (cffIndex, error) {
	if len(c.FDSelect) == 0 {
		return c.LocalSubrs, nil
	}
	if int(glyphID) >= len(c.FDSelect) {
		return cffIndex{}, fmt.Errorf("cff glyph id %d fdselect out of range", glyphID)
	}
	fdIndex := int(c.FDSelect[glyphID])
	if fdIndex < 0 || fdIndex >= len(c.FontDicts) {
		return cffIndex{}, fmt.Errorf("cff glyph id %d font dict %d out of range", glyphID, fdIndex)
	}
	return c.FontDicts[fdIndex].LocalSubrs, nil
}

func (c *cffFont) execType2Charstring(data []byte, localSubrs cffIndex, state *cffType2State, depth int) error {
	if depth > 16 {
		return fmt.Errorf("cff subroutine recursion too deep")
	}
	for i := 0; i < len(data); {
		b := data[i]
		if isType2Number(b) {
			value, next, err := parseType2Number(data, i)
			if err != nil {
				return err
			}
			state.stack = append(state.stack, value)
			i = next
			continue
		}

		switch b {
		case 1, 3, 18, 23:
			args := trimType2StemWidth(state.stack)
			state.hintCount += len(args) / 2
			state.stack = state.stack[:0]
			i++
		case 19, 20:
			args := trimType2StemWidth(state.stack)
			state.hintCount += len(args) / 2
			state.stack = state.stack[:0]
			i++
			maskBytes := (state.hintCount + 7) / 8
			if i+maskBytes > len(data) {
				return fmt.Errorf("cff hint mask truncated")
			}
			i += maskBytes
		case 4:
			args := trimType2MoveWidth(state.stack, 1)
			if len(args) != 1 {
				return fmt.Errorf("cff vmoveto expected 1 argument, got %d", len(args))
			}
			state.finishContour()
			state.y += args[0]
			state.startContour()
			state.stack = state.stack[:0]
			i++
		case 5:
			if len(state.stack)%2 != 0 {
				return fmt.Errorf("cff rlineto expected pairs, got %d arguments", len(state.stack))
			}
			state.startContour()
			for j := 0; j < len(state.stack); j += 2 {
				state.lineTo(state.x+state.stack[j], state.y+state.stack[j+1])
			}
			state.stack = state.stack[:0]
			i++
		case 6:
			state.startContour()
			horizontal := true
			for _, delta := range state.stack {
				if horizontal {
					state.lineTo(state.x+delta, state.y)
				} else {
					state.lineTo(state.x, state.y+delta)
				}
				horizontal = !horizontal
			}
			state.stack = state.stack[:0]
			i++
		case 7:
			state.startContour()
			vertical := true
			for _, delta := range state.stack {
				if vertical {
					state.lineTo(state.x, state.y+delta)
				} else {
					state.lineTo(state.x+delta, state.y)
				}
				vertical = !vertical
			}
			state.stack = state.stack[:0]
			i++
		case 8:
			if len(state.stack)%6 != 0 {
				return fmt.Errorf("cff rrcurveto expected groups of 6, got %d arguments", len(state.stack))
			}
			state.startContour()
			for j := 0; j < len(state.stack); j += 6 {
				p0 := OutlinePoint{X: state.x, Y: state.y}
				p1 := OutlinePoint{X: state.x + state.stack[j], Y: state.y + state.stack[j+1]}
				p2 := OutlinePoint{X: p1.X + state.stack[j+2], Y: p1.Y + state.stack[j+3]}
				p3 := OutlinePoint{X: p2.X + state.stack[j+4], Y: p2.Y + state.stack[j+5]}
				curve := flattenCubic(p0, p1, p2, p3, 0.5)
				state.current = append(state.current, curve[1:]...)
				state.x = p3.X
				state.y = p3.Y
			}
			state.stack = state.stack[:0]
			i++
		case 10:
			if len(state.stack) == 0 {
				return fmt.Errorf("cff callsubr missing operand")
			}
			subrIndex := int(state.stack[len(state.stack)-1]) + cffSubrBias(localSubrs.Len())
			state.stack = state.stack[:len(state.stack)-1]
			if subrIndex < 0 || subrIndex >= localSubrs.Len() {
				return fmt.Errorf("cff local subr %d out of range", subrIndex)
			}
			if err := c.execType2Charstring(localSubrs.At(subrIndex), localSubrs, state, depth+1); err != nil {
				return err
			}
			i++
		case 11:
			return nil
		case 14:
			state.finishContour()
			state.stack = state.stack[:0]
			return nil
		case 21:
			args := trimType2MoveWidth(state.stack, 2)
			if len(args) != 2 {
				return fmt.Errorf("cff rmoveto expected 2 arguments, got %d", len(args))
			}
			state.finishContour()
			state.x += args[0]
			state.y += args[1]
			state.startContour()
			state.stack = state.stack[:0]
			i++
		case 22:
			args := trimType2MoveWidth(state.stack, 1)
			if len(args) != 1 {
				return fmt.Errorf("cff hmoveto expected 1 argument, got %d", len(args))
			}
			state.finishContour()
			state.x += args[0]
			state.startContour()
			state.stack = state.stack[:0]
			i++
		case 24:
			if len(state.stack) < 8 || (len(state.stack)-2)%6 != 0 {
				return fmt.Errorf("cff rcurveline expected 6n+2 arguments, got %d", len(state.stack))
			}
			state.startContour()
			for j := 0; j < len(state.stack)-2; j += 6 {
				state.cubicToRelative(state.stack[j], state.stack[j+1], state.stack[j+2], state.stack[j+3], state.stack[j+4], state.stack[j+5])
			}
			state.lineTo(state.x+state.stack[len(state.stack)-2], state.y+state.stack[len(state.stack)-1])
			state.stack = state.stack[:0]
			i++
		case 25:
			if len(state.stack) < 8 || (len(state.stack)-6)%2 != 0 {
				return fmt.Errorf("cff rlinecurve expected 2n+6 arguments, got %d", len(state.stack))
			}
			state.startContour()
			for j := 0; j < len(state.stack)-6; j += 2 {
				state.lineTo(state.x+state.stack[j], state.y+state.stack[j+1])
			}
			j := len(state.stack) - 6
			state.cubicToRelative(state.stack[j], state.stack[j+1], state.stack[j+2], state.stack[j+3], state.stack[j+4], state.stack[j+5])
			state.stack = state.stack[:0]
			i++
		case 26:
			args := state.stack
			if len(args) < 4 || len(args)%4 > 1 {
				return fmt.Errorf("cff vvcurveto expected 4n or 4n+1 arguments, got %d", len(state.stack))
			}
			state.startContour()
			for j := 0; j < len(args); {
				dx1 := 0.0
				if j == 0 && len(args)%4 == 1 {
					dx1 = args[j]
					j++
				}
				if j+4 > len(args) {
					return fmt.Errorf("cff vvcurveto truncated arguments")
				}
				state.cubicToRelative(dx1, args[j], args[j+1], args[j+2], 0, args[j+3])
				j += 4
			}
			state.stack = state.stack[:0]
			i++
		case 27:
			args := state.stack
			if len(args) < 4 || len(args)%4 > 1 {
				return fmt.Errorf("cff hhcurveto expected 4n or 4n+1 arguments, got %d", len(state.stack))
			}
			state.startContour()
			for j := 0; j < len(args); {
				dy1 := 0.0
				if j == 0 && len(args)%4 == 1 {
					dy1 = args[j]
					j++
				}
				if j+4 > len(args) {
					return fmt.Errorf("cff hhcurveto truncated arguments")
				}
				state.cubicToRelative(args[j], dy1, args[j+1], args[j+2], args[j+3], 0)
				j += 4
			}
			state.stack = state.stack[:0]
			i++
		case 29:
			if len(state.stack) == 0 {
				return fmt.Errorf("cff callgsubr missing operand")
			}
			subrIndex := int(state.stack[len(state.stack)-1]) + cffSubrBias(c.GlobalSubrs.Len())
			state.stack = state.stack[:len(state.stack)-1]
			if subrIndex < 0 || subrIndex >= c.GlobalSubrs.Len() {
				return fmt.Errorf("cff global subr %d out of range", subrIndex)
			}
			if err := c.execType2Charstring(c.GlobalSubrs.At(subrIndex), localSubrs, state, depth+1); err != nil {
				return err
			}
			i++
		case 30:
			if len(state.stack) < 4 || len(state.stack)%4 > 1 {
				return fmt.Errorf("cff vhcurveto expected at least 4 arguments, got %d", len(state.stack))
			}
			state.startContour()
			if err := state.curveByAlternatingAxes(state.stack, false); err != nil {
				return err
			}
			state.stack = state.stack[:0]
			i++
		case 31:
			if len(state.stack) < 4 || len(state.stack)%4 > 1 {
				return fmt.Errorf("cff hvcurveto expected at least 4 arguments, got %d", len(state.stack))
			}
			state.startContour()
			if err := state.curveByAlternatingAxes(state.stack, true); err != nil {
				return err
			}
			state.stack = state.stack[:0]
			i++
		case 12:
			i++
			if i >= len(data) {
				return fmt.Errorf("cff escaped operator truncated")
			}
			op := data[i]
			switch op {
			case 34:
				if len(state.stack) != 7 {
					return fmt.Errorf("cff hflex expected 7 arguments, got %d", len(state.stack))
				}
				state.startContour()
				state.cubicToRelative(state.stack[0], 0, state.stack[1], state.stack[2], state.stack[3], state.stack[2])
				state.cubicToRelative(state.stack[4], 0, state.stack[5], -state.stack[2], state.stack[6], -state.stack[2])
			case 35:
				if len(state.stack) != 13 {
					return fmt.Errorf("cff flex expected 13 arguments, got %d", len(state.stack))
				}
				state.startContour()
				state.cubicToRelative(state.stack[0], state.stack[1], state.stack[2], state.stack[3], state.stack[4], state.stack[5])
				state.cubicToRelative(state.stack[6], state.stack[7], state.stack[8], state.stack[9], state.stack[10], state.stack[11])
			case 36:
				if len(state.stack) != 9 {
					return fmt.Errorf("cff hflex1 expected 9 arguments, got %d", len(state.stack))
				}
				state.startContour()
				state.cubicToRelative(state.stack[0], state.stack[1], state.stack[2], state.stack[3], state.stack[4], 0)
				state.cubicToRelative(state.stack[5], 0, state.stack[6], state.stack[7], state.stack[8], -state.stack[1]-state.stack[3]-state.stack[7])
			case 37:
				if len(state.stack) != 11 {
					return fmt.Errorf("cff flex1 expected 11 arguments, got %d", len(state.stack))
				}
				state.startContour()
				dx := state.stack[0] + state.stack[2] + state.stack[4] + state.stack[6] + state.stack[8]
				dy := state.stack[1] + state.stack[3] + state.stack[5] + state.stack[7] + state.stack[9]
				lastDX := state.stack[10]
				lastDY := state.stack[10]
				if math.Abs(dx) > math.Abs(dy) {
					lastDY = -dy
				} else {
					lastDX = -dx
				}
				state.cubicToRelative(state.stack[0], state.stack[1], state.stack[2], state.stack[3], state.stack[4], state.stack[5])
				state.cubicToRelative(state.stack[6], state.stack[7], state.stack[8], state.stack[9], lastDX, lastDY)
			default:
				return fmt.Errorf("unsupported escaped type2 operator %d", op)
			}
			state.stack = state.stack[:0]
			i++
		default:
			return fmt.Errorf("unsupported type2 operator %d", b)
		}
	}
	return nil
}

func isType2Number(b byte) bool {
	return b == 28 || b == 255 || b >= 32
}

func parseType2Number(data []byte, offset int) (float64, int, error) {
	if offset >= len(data) {
		return 0, offset, fmt.Errorf("type2 number truncated")
	}
	b := data[offset]
	switch {
	case b >= 32 && b <= 246:
		return float64(int(b) - 139), offset + 1, nil
	case b >= 247 && b <= 250:
		if offset+1 >= len(data) {
			return 0, offset, fmt.Errorf("type2 number truncated")
		}
		return float64((int(b)-247)*256 + int(data[offset+1]) + 108), offset + 2, nil
	case b >= 251 && b <= 254:
		if offset+1 >= len(data) {
			return 0, offset, fmt.Errorf("type2 number truncated")
		}
		return float64(-(int(b)-251)*256 - int(data[offset+1]) - 108), offset + 2, nil
	case b == 28:
		if offset+2 >= len(data) {
			return 0, offset, fmt.Errorf("type2 shortint truncated")
		}
		return float64(int16(binary.BigEndian.Uint16(data[offset+1 : offset+3]))), offset + 3, nil
	case b == 255:
		if offset+4 >= len(data) {
			return 0, offset, fmt.Errorf("type2 fixed number truncated")
		}
		value := int32(binary.BigEndian.Uint32(data[offset+1 : offset+5]))
		return float64(value) / 65536, offset + 5, nil
	default:
		return 0, offset, fmt.Errorf("unsupported type2 number byte %d", b)
	}
}

func trimType2StemWidth(args []float64) []float64 {
	if len(args)%2 == 1 {
		return args[1:]
	}
	return args
}

func trimType2MoveWidth(args []float64, expected int) []float64 {
	if len(args) == expected+1 {
		return args[1:]
	}
	return args
}

func cffSubrBias(count int) int {
	switch {
	case count < 1240:
		return 107
	case count < 33900:
		return 1131
	default:
		return 32768
	}
}

func (s *cffType2State) startContour() {
	if len(s.current) == 0 {
		s.current = append(s.current, OutlinePoint{X: s.x, Y: s.y})
	}
}

func (s *cffType2State) lineTo(x, y float64) {
	s.x = x
	s.y = y
	s.current = append(s.current, OutlinePoint{X: x, Y: y})
}

func (s *cffType2State) cubicToRelative(dx1, dy1, dx2, dy2, dx3, dy3 float64) {
	p0 := OutlinePoint{X: s.x, Y: s.y}
	p1 := OutlinePoint{X: s.x + dx1, Y: s.y + dy1}
	p2 := OutlinePoint{X: p1.X + dx2, Y: p1.Y + dy2}
	p3 := OutlinePoint{X: p2.X + dx3, Y: p2.Y + dy3}
	curve := flattenCubic(p0, p1, p2, p3, 0.5)
	s.current = append(s.current, curve[1:]...)
	s.x = p3.X
	s.y = p3.Y
}

func (s *cffType2State) curveByAlternatingAxes(args []float64, horizontalFirst bool) error {
	remaining := args
	horizontal := horizontalFirst
	for len(remaining) > 0 {
		if len(remaining) < 4 {
			return fmt.Errorf("cff alternating curveto has trailing %d arguments", len(remaining))
		}
		final := len(remaining) == 5
		if horizontal {
			dx3 := 0.0
			if final {
				dx3 = remaining[4]
			}
			s.cubicToRelative(remaining[0], 0, remaining[1], remaining[2], dx3, remaining[3])
		} else {
			dy3 := 0.0
			if final {
				dy3 = remaining[4]
			}
			s.cubicToRelative(0, remaining[0], remaining[1], remaining[2], remaining[3], dy3)
		}
		if final {
			remaining = remaining[5:]
		} else {
			remaining = remaining[4:]
		}
		horizontal = !horizontal
	}
	return nil
}

func (s *cffType2State) finishContour() {
	if len(s.current) == 0 {
		return
	}
	if s.current[len(s.current)-1] != s.current[0] {
		s.current = append(s.current, s.current[0])
	}
	s.contours = append(s.contours, append(OutlineContour(nil), s.current...))
	s.current = nil
}

func outlineBounds(contours []OutlineContour) Bounds {
	if len(contours) == 0 {
		return Bounds{}
	}
	xMin := contours[0][0].X
	yMin := contours[0][0].Y
	xMax := contours[0][0].X
	yMax := contours[0][0].Y
	for _, contour := range contours {
		for _, point := range contour {
			if point.X < xMin {
				xMin = point.X
			}
			if point.Y < yMin {
				yMin = point.Y
			}
			if point.X > xMax {
				xMax = point.X
			}
			if point.Y > yMax {
				yMax = point.Y
			}
		}
	}
	return Bounds{
		XMin: int16(math.Floor(xMin)),
		YMin: int16(math.Floor(yMin)),
		XMax: int16(math.Ceil(xMax)),
		YMax: int16(math.Ceil(yMax)),
	}
}

func flattenCubic(p0, p1, p2, p3 OutlinePoint, tolerance float64) []OutlinePoint {
	if cubicFlatEnough(p0, p1, p2, p3, tolerance) {
		return []OutlinePoint{p0, p3}
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

func cubicFlatEnough(p0, p1, p2, p3 OutlinePoint, tolerance float64) bool {
	ux := 3*p1.X - 2*p0.X - p3.X
	uy := 3*p1.Y - 2*p0.Y - p3.Y
	vx := 3*p2.X - 2*p3.X - p0.X
	vy := 3*p2.Y - 2*p3.Y - p0.Y
	return math.Max(ux*ux+uy*uy, vx*vx+vy*vy) <= tolerance*tolerance*16
}
