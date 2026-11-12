package font

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
)

func CollectionIndex(data []byte, name string) (int, error) {
	if len(data) < 4 || string(data[:4]) != "ttcf" {
		return 0, nil
	}
	if len(data) < 12 {
		return 0, fmt.Errorf("font collection header truncated")
	}
	fontCount := int(binary.BigEndian.Uint32(data[8:12]))
	for index := range fontCount {
		font, err := parseSFNT(data, index)
		if err != nil {
			return 0, err
		}
		nameTable, err := font.table("name")
		if err != nil {
			continue
		}
		for _, candidate := range fontNames(nameTable) {
			if strings.EqualFold(candidate, name) {
				return index, nil
			}
		}
	}
	return 0, fmt.Errorf("font collection has no face named %q", name)
}

func fontNames(data []byte) []string {
	if len(data) < 6 {
		return nil
	}
	count := int(binary.BigEndian.Uint16(data[2:4]))
	stringsOffset := int(binary.BigEndian.Uint16(data[4:6]))
	if len(data) < 6+count*12 || stringsOffset > len(data) {
		return nil
	}
	names := make([]string, 0, count)
	for index := range count {
		record := data[6+index*12 : 18+index*12]
		platformID := binary.BigEndian.Uint16(record[0:2])
		nameID := binary.BigEndian.Uint16(record[6:8])
		if nameID != 1 && nameID != 4 && nameID != 6 && nameID != 16 {
			continue
		}
		length := int(binary.BigEndian.Uint16(record[8:10]))
		offset := stringsOffset + int(binary.BigEndian.Uint16(record[10:12]))
		if offset < 0 || length < 0 || offset+length > len(data) {
			continue
		}
		value := decodeFontName(data[offset:offset+length], platformID)
		if value != "" {
			names = append(names, value)
		}
	}
	return names
}

func decodeFontName(data []byte, platformID uint16) string {
	if platformID != 0 && platformID != 3 {
		return strings.TrimSpace(string(data))
	}
	if len(data)%2 != 0 {
		return ""
	}
	encoded := make([]uint16, len(data)/2)
	for index := range encoded {
		encoded[index] = binary.BigEndian.Uint16(data[index*2 : index*2+2])
	}
	return strings.TrimSpace(string(utf16.Decode(encoded)))
}
