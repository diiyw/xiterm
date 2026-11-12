package font

import (
	"encoding/binary"
	"fmt"
)

type LongHorMetric struct {
	AdvanceWidth    uint16
	LeftSideBearing int16
}

func parseHmtx(data []byte, numGlyphs, numberOfHMetrics uint16) ([]LongHorMetric, error) {
	if numberOfHMetrics == 0 {
		return nil, fmt.Errorf("hhea numberOfHMetrics is zero")
	}
	metrics := make([]LongHorMetric, numGlyphs)
	requiredLong := int(numberOfHMetrics) * 4
	if len(data) < requiredLong {
		return nil, fmt.Errorf("hmtx table too short for long metrics")
	}

	var lastAdvance uint16
	for i := 0; i < int(numberOfHMetrics); i++ {
		offset := i * 4
		metrics[i] = LongHorMetric{
			AdvanceWidth:    binary.BigEndian.Uint16(data[offset : offset+2]),
			LeftSideBearing: int16(binary.BigEndian.Uint16(data[offset+2 : offset+4])),
		}
		lastAdvance = metrics[i].AdvanceWidth
	}

	lsbOffset := requiredLong
	for i := int(numberOfHMetrics); i < int(numGlyphs); i++ {
		entryOffset := lsbOffset + (i-int(numberOfHMetrics))*2
		if entryOffset+2 > len(data) {
			return nil, fmt.Errorf("hmtx table too short for left side bearings")
		}
		metrics[i] = LongHorMetric{
			AdvanceWidth:    lastAdvance,
			LeftSideBearing: int16(binary.BigEndian.Uint16(data[entryOffset : entryOffset+2])),
		}
	}

	return metrics, nil
}
