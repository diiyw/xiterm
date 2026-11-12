package font

import (
	"encoding/binary"
	"testing"
)

func testUnicodeEmojiCmap() []byte {
	data := make([]byte, 12+16+3*12)
	put16, put32 := binary.BigEndian.PutUint16, binary.BigEndian.PutUint32
	put16(data[2:], 1)
	put16(data[6:], 4) // Unicode platform, full repertoire
	put32(data[8:], 12)
	put16(data[12:], 12)
	put32(data[16:], uint32(len(data)-12))
	put32(data[24:], 3)
	for i, r := range []rune{'✅', '❌', '🙂'} {
		group := data[28+i*12:]
		put32(group, uint32(r))
		put32(group[4:], uint32(r))
		put32(group[8:], 1)
	}
	return data
}

func TestUnicodePlatformCharacterMaps(t *testing.T) {
	for _, format := range []int{4, 12} {
		var data []byte
		if format == 4 {
			data = buildTestCmapTable()
			binary.BigEndian.PutUint16(data[4:], 0)
			binary.BigEndian.PutUint16(data[6:], 3)
		} else {
			data = testUnicodeEmojiCmap()
		}
		mapping, err := parseCmap(data)
		if err != nil {
			t.Fatalf("Unicode format %d: %v", format, err)
		}
		chars := "A"
		if format == 12 {
			chars = "✅❌🙂"
		}
		for _, r := range chars {
			if mapping.GlyphIndex(r) != 1 {
				t.Errorf("Unicode format %d: lost %U", format, r)
			}
		}
		if mapping.GlyphIndex('\U0010ffff') != 0 {
			t.Fatal("unsupported character acquired a glyph")
		}
	}
}

func TestCharacterMapPrefersFullUnicodeCoverage(t *testing.T) {
	bmp, full := buildTestCmapTable()[12:], testUnicodeEmojiCmap()[12:]
	data := make([]byte, 20+len(bmp)+len(full))
	put16, put32 := binary.BigEndian.PutUint16, binary.BigEndian.PutUint32
	put16(data[2:], 2)
	put16(data[6:], 4)
	put32(data[8:], uint32(20+len(bmp)))
	put16(data[12:], 3)
	put16(data[14:], 1)
	put32(data[16:], 20)
	copy(data[20:], bmp)
	copy(data[20+len(bmp):], full)
	mapping, err := parseCmap(data)
	if err != nil || mapping.GlyphIndex('🙂') != 1 {
		t.Fatalf("full Unicode map lost to Windows BMP map: %v", err)
	}
}

func TestCharacterMapBinaryRangeBoundaries(t *testing.T) {
	// Three segments, including an indirect glyph array with a delta and a
	// missing glyph. Lookup must still read the original array after parsing.
	data := make([]byte, 46)
	put := binary.BigEndian.PutUint16
	put(data, 4)
	put(data[2:], uint16(len(data)))
	put(data[6:], 6)
	for i, value := range []uint16{'B', 'c', 0xffff} {
		put(data[14+i*2:], value)
	}
	for i, value := range []uint16{'A', 'a', 0xffff} {
		put(data[22+i*2:], value)
	}
	put(data[28:], 0xffc0) // 1 - 'A', modulo 65536
	put(data[30:], 5)
	put(data[32:], 1)
	put(data[36:], 4)
	put(data[40:], 10)
	put(data[44:], 30)
	bmp, err := parseCmapFormat4(data)
	if err != nil {
		t.Fatal(err)
	}
	for r, want := range map[rune]uint16{'@': 0, 'A': 1, 'B': 2, 'C': 0, '`': 0, 'a': 15, 'b': 0, 'c': 35, 'd': 0, 0xffff: 0, 0x10041: 0, -1: 0} {
		if got := bmp.GlyphIndex(r); got != want {
			t.Errorf("format 4 %U = %d, want %d", r, got, want)
		}
	}
	put(data[44:], 40)
	if bmp.GlyphIndex('c') != 45 {
		t.Fatal("indirect lookup stopped using the original glyph array")
	}
	put(data[36:], 0xfffe)
	badOffset, err := parseCmapFormat4(data)
	if err != nil || badOffset.GlyphIndex('a') != 0 {
		t.Fatal("out-of-bounds indirect glyph offset was not rejected safely")
	}
	put(data[24:], 'B')
	if _, err := parseCmapFormat4(data); err == nil {
		t.Fatal("overlapping segments accepted by binary lookup parser")
	}

	data = make([]byte, 52)
	put32 := binary.BigEndian.PutUint32
	put(data, 12)
	put32(data[4:], uint32(len(data)))
	put32(data[12:], 3)
	for i, group := range []cmapFormat12Group{{0x20, 0x22, 5}, {'A', 'B', 10}, {0x10000, 0x10002, 20}} {
		put32(data[16+i*12:], group.StartCharCode)
		put32(data[20+i*12:], group.EndCharCode)
		put32(data[24+i*12:], group.StartGlyphID)
	}
	full, err := parseCmapFormat12(data)
	if err != nil {
		t.Fatal(err)
	}
	for r, want := range map[rune]uint16{0x1f: 0, 0x20: 5, 0x22: 7, 0x23: 0, 'A': 10, 'B': 11, 'C': 0, 0xffff: 0, 0x10000: 20, 0x10002: 22, 0x10003: 0, -1: 0} {
		if got := full.GlyphIndex(r); got != want {
			t.Errorf("format 12 %U = %d, want %d", r, got, want)
		}
	}
	put32(data[28:], 0x22)
	if _, err := parseCmapFormat12(data); err == nil {
		t.Fatal("overlapping groups accepted by binary lookup parser")
	}
	put32(data[12:], 0xffffffff)
	if _, err := parseCmapFormat12(data); err == nil {
		t.Fatal("overflowing group count was accepted")
	}
}
