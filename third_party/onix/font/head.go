package font

import (
	"encoding/binary"
	"fmt"
)

type Head struct {
	UnitsPerEm       uint16
	XMin             int16
	YMin             int16
	XMax             int16
	YMax             int16
	IndexToLocFormat int16
}

func parseHead(data []byte) (Head, error) {
	if len(data) < 54 {
		return Head{}, fmt.Errorf("head table too short")
	}
	return Head{
		UnitsPerEm:       binary.BigEndian.Uint16(data[18:20]),
		XMin:             int16(binary.BigEndian.Uint16(data[36:38])),
		YMin:             int16(binary.BigEndian.Uint16(data[38:40])),
		XMax:             int16(binary.BigEndian.Uint16(data[40:42])),
		YMax:             int16(binary.BigEndian.Uint16(data[42:44])),
		IndexToLocFormat: int16(binary.BigEndian.Uint16(data[50:52])),
	}, nil
}
