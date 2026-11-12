package font

import (
	"encoding/binary"
	"fmt"
)

type cmap interface {
	GlyphIndex(r rune) uint16
}

type cmapFormat4 struct {
	EndCodes      []uint16
	StartCodes    []uint16
	IDDeltas      []int16
	IDRangeOffset []uint16
	Data          []byte
}

type cmapFormat12Group struct {
	StartCharCode uint32
	EndCharCode   uint32
	StartGlyphID  uint32
}

type cmapFormat12 struct {
	data []byte
}

func parseCmap(data []byte) (cmap, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("cmap table too short")
	}
	numTables := int(binary.BigEndian.Uint16(data[2:4]))
	if len(data) < 4+numTables*8 {
		return nil, fmt.Errorf("cmap encoding records truncated")
	}

	var format12Offset uint32
	var format4Offset uint32
	for i := 0; i < numTables; i++ {
		offset := 4 + i*8
		platformID := binary.BigEndian.Uint16(data[offset : offset+2])
		encodingID := binary.BigEndian.Uint16(data[offset+2 : offset+4])
		subtableOffset := binary.BigEndian.Uint32(data[offset+4 : offset+8])
		if int(subtableOffset)+2 > len(data) {
			continue
		}
		format := binary.BigEndian.Uint16(data[subtableOffset : subtableOffset+2])
		// Apple fonts can expose only Unicode-platform subtables. Prefer full
		// Unicode coverage to BMP coverage regardless of the platform record.
		unicode := platformID == 0 && encodingID <= 4
		if (unicode || platformID == 3 && encodingID == 10) && format == 12 {
			format12Offset = subtableOffset
		}
		if (unicode || platformID == 3 && (encodingID == 1 || encodingID == 0)) && format == 4 {
			format4Offset = subtableOffset
		}
	}

	if format12Offset != 0 {
		return parseCmapFormat12(data[format12Offset:])
	}
	if format4Offset != 0 {
		return parseCmapFormat4(data[format4Offset:])
	}

	return nil, fmt.Errorf("no supported cmap subtable found")
}

func parseCmapFormat4(data []byte) (*cmapFormat4, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("cmap format 4 too short")
	}
	length := int(binary.BigEndian.Uint16(data[2:4]))
	if length > len(data) {
		return nil, fmt.Errorf("cmap format 4 length out of bounds")
	}
	segCountX2 := binary.BigEndian.Uint16(data[6:8])
	if segCountX2 == 0 || segCountX2%2 != 0 {
		return nil, fmt.Errorf("cmap format 4 invalid segment count")
	}
	segCount := int(segCountX2 / 2)
	needed := 16 + segCount*8
	if length < needed {
		return nil, fmt.Errorf("cmap format 4 truncated segments")
	}

	endCodes := make([]uint16, segCount)
	startCodes := make([]uint16, segCount)
	idDeltas := make([]int16, segCount)
	idRangeOffset := make([]uint16, segCount)

	offset := 14
	for i := 0; i < segCount; i++ {
		endCodes[i] = binary.BigEndian.Uint16(data[offset+i*2 : offset+i*2+2])
	}
	offset += segCount * 2
	offset += 2
	for i := 0; i < segCount; i++ {
		startCodes[i] = binary.BigEndian.Uint16(data[offset+i*2 : offset+i*2+2])
		if startCodes[i] > endCodes[i] || i > 0 && startCodes[i] <= endCodes[i-1] {
			return nil, fmt.Errorf("cmap format 4 unordered or overlapping segments")
		}
	}
	offset += segCount * 2
	for i := 0; i < segCount; i++ {
		idDeltas[i] = int16(binary.BigEndian.Uint16(data[offset+i*2 : offset+i*2+2]))
	}
	offset += segCount * 2
	for i := 0; i < segCount; i++ {
		idRangeOffset[i] = binary.BigEndian.Uint16(data[offset+i*2 : offset+i*2+2])
	}

	return &cmapFormat4{
		EndCodes:      endCodes,
		StartCodes:    startCodes,
		IDDeltas:      idDeltas,
		IDRangeOffset: idRangeOffset,
		Data:          data[:length],
	}, nil
}

func parseCmapFormat12(data []byte) (*cmapFormat12, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("cmap format 12 too short")
	}
	length := binary.BigEndian.Uint32(data[4:8])
	if length < 16 || uint64(length) > uint64(len(data)) {
		return nil, fmt.Errorf("cmap format 12 length out of bounds")
	}
	numGroups := binary.BigEndian.Uint32(data[12:16])
	if numGroups > (length-16)/12 {
		return nil, fmt.Errorf("cmap format 12 truncated groups")
	}
	groups := data[16 : 16+int(numGroups)*12]
	var previous uint32
	for i := range int(numGroups) {
		offset := i * 12
		start := binary.BigEndian.Uint32(groups[offset:])
		end := binary.BigEndian.Uint32(groups[offset+4:])
		if start > end || i > 0 && start <= previous {
			return nil, fmt.Errorf("cmap format 12 unordered or overlapping groups")
		}
		previous = end
	}
	return &cmapFormat12{data: groups}, nil
}

func (c *cmapFormat4) GlyphIndex(r rune) uint16 {
	if uint32(r) > 0xffff {
		return 0
	}
	code := uint16(r)
	lo, hi := 0, len(c.EndCodes)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if c.EndCodes[mid] < code {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	i := lo
	if i == len(c.EndCodes) || code < c.StartCodes[i] {
		return 0
	}
	if c.IDRangeOffset[i] == 0 {
		return uint16(int(code) + int(c.IDDeltas[i]))
	}
	idRangeOffsetPos := 16 + len(c.EndCodes)*6 + i*2
	glyphOffset := idRangeOffsetPos + int(c.IDRangeOffset[i]) + int(code-c.StartCodes[i])*2
	if glyphOffset+2 > len(c.Data) {
		return 0
	}
	glyph := binary.BigEndian.Uint16(c.Data[glyphOffset : glyphOffset+2])
	if glyph == 0 {
		return 0
	}
	return uint16(int(glyph) + int(c.IDDeltas[i]))
}

func (c *cmapFormat12) GlyphIndex(r rune) uint16 {
	code := uint32(r)
	lo, hi := 0, len(c.data)/12
	for lo < hi {
		mid := lo + (hi-lo)/2
		if binary.BigEndian.Uint32(c.data[mid*12+4:]) < code {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == len(c.data)/12 {
		return 0
	}
	group := c.data[lo*12:]
	start := binary.BigEndian.Uint32(group)
	if code < start {
		return 0
	}
	return uint16(binary.BigEndian.Uint32(group[8:]) + code - start)
}
