package font

import (
	"encoding/binary"
	"fmt"
)

type tableRecord struct {
	Tag      string
	Checksum uint32
	Offset   uint32
	Length   uint32
}

type sfnt struct {
	Tables     map[string]tableRecord
	Data       []byte
	ScalerType uint32
}

func parseSFNT(data []byte, index int) (*sfnt, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("sfnt header too short")
	}
	directoryOffset := 0
	if string(data[:4]) == "ttcf" {
		fontCount := int(binary.BigEndian.Uint32(data[8:12]))
		if index < 0 || index >= fontCount {
			return nil, fmt.Errorf("font collection index %d out of range [0, %d)", index, fontCount)
		}
		offsetTableEnd := 12 + fontCount*4
		if fontCount < 1 || offsetTableEnd > len(data) {
			return nil, fmt.Errorf("font collection offset table truncated")
		}
		directoryOffset = int(binary.BigEndian.Uint32(data[12+index*4 : 16+index*4]))
		if directoryOffset < 0 || directoryOffset+12 > len(data) {
			return nil, fmt.Errorf("font collection directory %d out of bounds", index)
		}
	} else if index != 0 {
		return nil, fmt.Errorf("font index must be zero for a single font")
	}

	numTables := int(binary.BigEndian.Uint16(data[directoryOffset+4 : directoryOffset+6]))
	if len(data) < directoryOffset+12+numTables*16 {
		return nil, fmt.Errorf("sfnt table directory truncated")
	}

	font := &sfnt{
		ScalerType: binary.BigEndian.Uint32(data[directoryOffset : directoryOffset+4]),
		Tables:     make(map[string]tableRecord, numTables),
		Data:       data,
	}

	for i := 0; i < numTables; i++ {
		offset := directoryOffset + 12 + i*16
		record := tableRecord{
			Tag:      string(data[offset : offset+4]),
			Checksum: binary.BigEndian.Uint32(data[offset+4 : offset+8]),
			Offset:   binary.BigEndian.Uint32(data[offset+8 : offset+12]),
			Length:   binary.BigEndian.Uint32(data[offset+12 : offset+16]),
		}
		end := int(record.Offset + record.Length)
		if int(record.Offset) > len(data) || end > len(data) || end < int(record.Offset) {
			return nil, fmt.Errorf("table %q out of bounds", record.Tag)
		}
		font.Tables[record.Tag] = record
	}

	return font, nil
}

func (s *sfnt) table(tag string) ([]byte, error) {
	record, ok := s.Tables[tag]
	if !ok {
		return nil, fmt.Errorf("missing table %q", tag)
	}
	return s.Data[record.Offset : record.Offset+record.Length], nil
}
