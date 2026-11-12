package font

import (
	"encoding/binary"
	"math/bits"
	"runtime"
)

// Pair positioning tables stay in the font buffer. Class kerning is queried
// directly instead of expanding a potentially quadratic glyph-pair map.
type pairLookup struct{ subtables [][]byte }

func u16(data []byte, offset int) (uint16, bool) {
	if offset < 0 || offset > len(data)-2 {
		return 0, false
	}
	return binary.BigEndian.Uint16(data[offset:]), true
}

func parsePairLookups(data []byte) []pairLookup {
	featureOffset, ok := u16(data, 6)
	if !ok {
		return nil
	}
	lookupOffset, ok := u16(data, 8)
	if !ok {
		return nil
	}
	count, ok := u16(data, int(featureOffset))
	if !ok {
		return nil
	}
	lookupCount, ok := u16(data, int(lookupOffset))
	if !ok {
		return nil
	}
	selected := make([]bool, int(lookupCount))
	features := defaultPairFeatures(data, int(count))
	for i := 0; i < int(count); i++ {
		if !features[i] {
			continue
		}
		record := int(featureOffset) + 2 + i*6
		if record+6 > len(data) {
			return nil
		}
		if string(data[record:record+4]) != "kern" {
			continue
		}
		offset, _ := u16(data, record+4)
		feature := int(featureOffset) + int(offset)
		n, ok := u16(data, feature+2)
		if !ok {
			continue
		}
		for j := 0; j < int(n); j++ {
			index, ok := u16(data, feature+4+j*2)
			if ok && int(index) < len(selected) {
				selected[index] = true
			}
		}
	}
	var result []pairLookup
	for index, enabled := range selected {
		if !enabled {
			continue
		}
		offset, ok := u16(data, int(lookupOffset)+2+index*2)
		if !ok {
			continue
		}
		lookup := int(lookupOffset) + int(offset)
		kind, ok := u16(data, lookup)
		if !ok || (kind != 2 && kind != 9) {
			continue
		}
		n, ok := u16(data, lookup+4)
		if !ok {
			continue
		}
		var pair pairLookup
		for j := 0; j < int(n); j++ {
			off, ok := u16(data, lookup+6+j*2)
			if !ok {
				continue
			}
			start := lookup + int(off)
			if kind == 9 {
				format, ok := u16(data, start)
				if !ok || format != 1 {
					continue
				}
				extKind, ok := u16(data, start+2)
				if !ok || extKind != 2 || start+8 > len(data) {
					continue
				}
				ext := uint64(binary.BigEndian.Uint32(data[start+4:]))
				if ext > uint64(len(data)-start) {
					continue
				}
				start += int(ext)
			}
			format, ok := u16(data, start)
			if !ok || (format != 1 && format != 2) {
				continue
			}
			pair.subtables = append(pair.subtables, data[start:])
		}
		if len(pair.subtables) > 0 {
			result = append(result, pair)
		}
	}
	return result
}

func coverageIndex(data []byte, offset int, glyph uint16) int {
	format, ok := u16(data, offset)
	if !ok {
		return -1
	}
	count, ok := u16(data, offset+2)
	if !ok {
		return -1
	}
	stride := 2
	if format == 2 {
		stride = 6
	} else if format != 1 {
		return -1
	}
	if offset+4+int(count)*stride > len(data) {
		return -1
	}
	low, high := 0, int(count)
	for low < high {
		mid := (low + high) / 2
		at := offset + 4 + mid*stride
		first, _ := u16(data, at)
		last := first
		if format == 2 {
			last, _ = u16(data, at+2)
		}
		if glyph < first {
			high = mid
		} else if glyph > last {
			low = mid + 1
		} else {
			if format == 1 {
				return mid
			}
			start, _ := u16(data, at+4)
			return int(start) + int(glyph-first)
		}
	}
	return -1
}

func glyphClass(data []byte, offset int, glyph uint16) uint16 {
	format, ok := u16(data, offset)
	if !ok {
		return 0
	}
	if format == 1 {
		start, ok := u16(data, offset+2)
		if !ok || glyph < start {
			return 0
		}
		count, ok := u16(data, offset+4)
		if !ok || glyph-start >= count {
			return 0
		}
		value, _ := u16(data, offset+6+int(glyph-start)*2)
		return value
	}
	if format != 2 {
		return 0
	}
	count, ok := u16(data, offset+2)
	if !ok || offset+4+int(count)*6 > len(data) {
		return 0
	}
	low, high := 0, int(count)
	for low < high {
		mid := (low + high) / 2
		at := offset + 4 + mid*6
		first, _ := u16(data, at)
		last, _ := u16(data, at+2)
		if glyph < first {
			high = mid
		} else if glyph > last {
			low = mid + 1
		} else {
			value, _ := u16(data, at+4)
			return value
		}
	}
	return 0
}

func pairAdvance(data []byte, left, right uint16) (int, bool) {
	coverage, ok := u16(data, 2)
	if !ok {
		return 0, false
	}
	index := coverageIndex(data, int(coverage), left)
	if index < 0 {
		return 0, false
	}
	format, _ := u16(data, 0)
	value1, ok := u16(data, 4)
	if !ok {
		return 0, false
	}
	value2, ok := u16(data, 6)
	if !ok {
		return 0, false
	}
	size := 2 * (bits.OnesCount16(value1) + bits.OnesCount16(value2))
	if size == 0 {
		return 0, true
	}
	valueOffset := -1
	switch format {
	case 1:
		count, ok := u16(data, 8)
		if !ok || index >= int(count) {
			return 0, false
		}
		set, ok := u16(data, 10+index*2)
		if !ok {
			return 0, false
		}
		pairs, ok := u16(data, int(set))
		if !ok {
			return 0, false
		}
		stride := 2 + size
		if int(set)+2+int(pairs)*stride > len(data) {
			return 0, false
		}
		low, high := 0, int(pairs)
		for low < high {
			mid := (low + high) / 2
			at := int(set) + 2 + mid*stride
			second, _ := u16(data, at)
			if right < second {
				high = mid
			} else if right > second {
				low = mid + 1
			} else {
				valueOffset = at + 2
				break
			}
		}
	case 2:
		class1, ok := u16(data, 8)
		if !ok {
			return 0, false
		}
		class2, ok := u16(data, 10)
		if !ok {
			return 0, false
		}
		count1, ok := u16(data, 12)
		if !ok {
			return 0, false
		}
		count2, ok := u16(data, 14)
		if !ok {
			return 0, false
		}
		first, second := glyphClass(data, int(class1), left), glyphClass(data, int(class2), right)
		if first >= count1 || second >= count2 {
			return 0, false
		}
		valueOffset = 16 + (int(first)*int(count2)+int(second))*size
	}
	if valueOffset < 0 || valueOffset+size > len(data) {
		return 0, false
	}
	if value1&4 == 0 {
		return 0, true
	}
	advance, ok := u16(data, valueOffset+2*bits.OnesCount16(value1&3))
	return int(int16(advance)), ok
}

// GlyphKerning returns horizontal advance adjustment in font design units.
// GPOS kern lookups take precedence over the legacy kern table.
func (f *Face) GlyphKerning(left, right uint16) int {
	defer runtime.KeepAlive(f)
	if f == nil {
		return 0
	}
	if len(f.pairLookups) == 0 {
		return int(f.Kern[kernKey(left, right)])
	}
	adjustment := 0
	for _, lookup := range f.pairLookups {
		for _, table := range lookup.subtables {
			if value, ok := pairAdvance(table, left, right); ok {
				adjustment += value
				break
			}
		}
	}
	return adjustment
}

// The engine currently shapes horizontal default-language Latin runs. Do not
// accidentally merge kerning features belonging to other language systems.
func defaultPairFeatures(data []byte, count int) []bool {
	result := make([]bool, count)
	offset, ok := u16(data, 4)
	if !ok {
		return result
	}
	n, ok := u16(data, int(offset))
	if !ok {
		return result
	}
	script := 0
	for i := 0; i < int(n); i++ {
		at := int(offset) + 2 + i*6
		if at+6 > len(data) {
			return result
		}
		tag := string(data[at : at+4])
		relative, _ := u16(data, at+4)
		if tag == "DFLT" && script == 0 {
			script = int(offset) + int(relative)
		}
		if tag == "latn" {
			script = int(offset) + int(relative)
			break
		}
	}
	if script == 0 {
		return result
	}
	lang, ok := u16(data, script)
	if !ok || lang == 0 {
		return result
	}
	start := script + int(lang)
	required, ok := u16(data, start+2)
	if !ok {
		return result
	}
	if int(required) < count {
		result[required] = true
	}
	n, ok = u16(data, start+4)
	if !ok {
		return result
	}
	for i := 0; i < int(n); i++ {
		index, ok := u16(data, start+6+i*2)
		if ok && int(index) < count {
			result[index] = true
		}
	}
	return result
}
