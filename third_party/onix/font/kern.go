package font

import (
	"encoding/binary"
	"fmt"
)

type kernTable map[uint32]int16

func parseKern(data []byte) (kernTable, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("kern table too short")
	}

	version := binary.BigEndian.Uint16(data[0:2])
	if version != 0 {
		return nil, nil
	}

	subtableCount := int(binary.BigEndian.Uint16(data[2:4]))
	offset := 4
	pairs := kernTable{}

	for i := 0; i < subtableCount; i++ {
		if offset+6 > len(data) {
			return nil, fmt.Errorf("kern subtable header truncated")
		}

		length := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		coverage := binary.BigEndian.Uint16(data[offset+4 : offset+6])
		if length < 6 || offset+length > len(data) {
			return nil, fmt.Errorf("kern subtable out of bounds")
		}

		format := byte(coverage >> 8)
		horizontal := coverage&0x0001 != 0
		crossStream := coverage&0x0004 != 0
		if format == 0 && horizontal && !crossStream {
			if err := parseKernFormat0(data[offset:offset+length], pairs); err != nil {
				return nil, err
			}
		}
		offset += length
	}

	if len(pairs) == 0 {
		return nil, nil
	}
	return pairs, nil
}

func parseKernFormat0(data []byte, pairs kernTable) error {
	if len(data) < 14 {
		return fmt.Errorf("kern format 0 table too short")
	}

	pairCount := int(binary.BigEndian.Uint16(data[6:8]))
	offset := 14
	if offset+pairCount*6 > len(data) {
		return fmt.Errorf("kern format 0 pairs truncated")
	}

	for i := 0; i < pairCount; i++ {
		left := binary.BigEndian.Uint16(data[offset : offset+2])
		right := binary.BigEndian.Uint16(data[offset+2 : offset+4])
		value := int16(binary.BigEndian.Uint16(data[offset+4 : offset+6]))
		pairs[kernKey(left, right)] = value
		offset += 6
	}
	return nil
}

func kernKey(left, right uint16) uint32 {
	return uint32(left)<<16 | uint32(right)
}
