package font

import (
	"encoding/binary"
	"fmt"
)

type Maxp struct {
	NumGlyphs uint16
}

func parseMaxp(data []byte) (Maxp, error) {
	if len(data) < 6 {
		return Maxp{}, fmt.Errorf("maxp table too short")
	}
	return Maxp{NumGlyphs: binary.BigEndian.Uint16(data[4:6])}, nil
}
