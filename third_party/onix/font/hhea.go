package font

import (
	"encoding/binary"
	"fmt"
)

type Hhea struct {
	Ascent              int16
	Descent             int16
	LineGap             int16
	AdvanceWidthMax     uint16
	MinLeftSideBearing  int16
	MinRightSideBearing int16
	XMaxExtent          int16
	NumberOfHMetrics    uint16
}

func parseHhea(data []byte) (Hhea, error) {
	if len(data) < 36 {
		return Hhea{}, fmt.Errorf("hhea table too short")
	}
	return Hhea{
		Ascent:              int16(binary.BigEndian.Uint16(data[4:6])),
		Descent:             int16(binary.BigEndian.Uint16(data[6:8])),
		LineGap:             int16(binary.BigEndian.Uint16(data[8:10])),
		AdvanceWidthMax:     binary.BigEndian.Uint16(data[10:12]),
		MinLeftSideBearing:  int16(binary.BigEndian.Uint16(data[12:14])),
		MinRightSideBearing: int16(binary.BigEndian.Uint16(data[14:16])),
		XMaxExtent:          int16(binary.BigEndian.Uint16(data[16:18])),
		NumberOfHMetrics:    binary.BigEndian.Uint16(data[34:36]),
	}, nil
}
