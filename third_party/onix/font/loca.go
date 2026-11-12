package font

import (
	"encoding/binary"
	"fmt"
)

func parseLoca(data []byte, numGlyphs uint16, indexToLocFormat int16) ([]uint32, error) {
	count := int(numGlyphs) + 1
	offsets := make([]uint32, count)

	switch indexToLocFormat {
	case 0:
		if len(data) < count*2 {
			return nil, fmt.Errorf("loca table too short")
		}
		for i := 0; i < count; i++ {
			offsets[i] = uint32(binary.BigEndian.Uint16(data[i*2:i*2+2])) * 2
		}
	case 1:
		if len(data) < count*4 {
			return nil, fmt.Errorf("loca table too short")
		}
		for i := 0; i < count; i++ {
			offsets[i] = binary.BigEndian.Uint32(data[i*4 : i*4+4])
		}
	default:
		return nil, fmt.Errorf("unsupported indexToLocFormat %d", indexToLocFormat)
	}

	return offsets, nil
}
