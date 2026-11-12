package font

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestParseOTFCFF(t *testing.T) {
	face, err := Parse(buildTestOTFCFF())
	if err != nil {
		t.Fatalf("parse otf cff: %v", err)
	}
	if face.CFF == nil {
		t.Fatal("face.CFF = nil")
	}
	if got := face.GlyphIndex('A'); got != 1 {
		t.Fatalf("glyph index for 'A' = %d", got)
	}

	outline, err := face.Outline(1)
	if err != nil {
		t.Fatalf("outline glyph 1: %v", err)
	}
	if len(outline.Contours) != 1 {
		t.Fatalf("contour count = %d", len(outline.Contours))
	}
	if outline.Bounds.XMax <= outline.Bounds.XMin || outline.Bounds.YMax <= outline.Bounds.YMin {
		t.Fatalf("outline bounds = %#v", outline.Bounds)
	}

	bitmap, err := face.RasterizeGlyph(1, 32)
	if err != nil {
		t.Fatalf("rasterize glyph 1: %v", err)
	}
	if bitmap == nil || bitmap.Image == nil {
		t.Fatal("bitmap is nil")
	}
	nonZero := 0
	for _, alpha := range bitmap.Image.Pix {
		if alpha != 0 {
			nonZero++
		}
	}
	if nonZero == 0 {
		t.Fatal("bitmap has no painted pixels")
	}
}

func TestParseCFFIndexBorrowsSourceData(t *testing.T) {
	data := buildCFFIndex([][]byte{[]byte("first"), []byte("second")})
	objects, _, err := parseCFFIndex(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	objects.At(0)[0] = 'F'
	if !bytes.Contains(data, []byte("First")) {
		t.Fatal("CFF INDEX object did not share the source buffer")
	}
	if cap(objects.At(0)) != len(objects.At(0)) {
		t.Fatal("CFF INDEX object can append into the next object")
	}
}

func TestCFFType2AdditionalOperators(t *testing.T) {
	tests := []struct {
		name       string
		charstring []byte
	}{
		{name: "hhcurveto", charstring: []byte{189, 159, 159, 189, 27, 14}},
		{name: "vvcurveto", charstring: []byte{189, 159, 159, 189, 26, 14}},
		{name: "hvcurveto", charstring: []byte{189, 159, 159, 189, 31, 14}},
		{name: "vhcurveto", charstring: []byte{189, 159, 159, 189, 30, 14}},
		{name: "rcurveline", charstring: []byte{189, 139, 159, 159, 139, 159, 189, 139, 24, 14}},
		{name: "rlinecurve", charstring: []byte{189, 139, 159, 139, 159, 159, 139, 159, 189, 139, 25, 14}},
		{name: "hflex", charstring: []byte{159, 159, 159, 159, 159, 159, 159, 12, 34, 14}},
		{name: "flex", charstring: []byte{149, 159, 149, 149, 149, 129, 149, 129, 149, 149, 149, 159, 139, 12, 35, 14}},
		{name: "hflex1", charstring: []byte{149, 159, 149, 149, 149, 149, 149, 129, 149, 12, 36, 14}},
		{name: "flex1", charstring: []byte{149, 159, 149, 149, 149, 129, 149, 129, 149, 149, 159, 12, 37, 14}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			charstring := append([]byte{139, 139, 21}, tt.charstring...)
			font := &cffFont{CharStrings: testCFFIndex(t, [][]byte{charstring})}
			outline, err := font.outline(0)
			if err != nil {
				t.Fatalf("outline: %v", err)
			}
			if len(outline.Contours) == 0 {
				t.Fatal("outline has no contours")
			}
			if len(outline.Contours[0]) < 4 {
				t.Fatalf("contour too small: %d", len(outline.Contours[0]))
			}
		})
	}
}

func TestCFFType2ReturnPreservesStack(t *testing.T) {
	localSubrs := make([][]byte, 108)
	localSubrs[107] = []byte{159, 139, 11}
	font := &cffFont{
		CharStrings: testCFFIndex(t, [][]byte{{139, 139, 21, 139, 10, 5, 14}}),
		LocalSubrs:  testCFFIndex(t, localSubrs),
	}

	outline, err := font.outline(0)
	if err != nil {
		t.Fatalf("outline: %v", err)
	}
	if len(outline.Contours) != 1 || len(outline.Contours[0]) < 2 {
		t.Fatalf("unexpected contours: %#v", outline.Contours)
	}
	end := outline.Contours[0][len(outline.Contours[0])-2]
	if end.X != 20 || end.Y != 0 {
		t.Fatalf("end point = (%v, %v)", end.X, end.Y)
	}
}

func TestCFFType2MoveWidthAndCurvetoOptionalArguments(t *testing.T) {
	tests := []struct {
		name       string
		charstring []byte
		wantEndX   float64
		wantEndY   float64
		wantStartX float64
		wantStartY float64
	}{
		{
			name:       "hmoveto width trimmed",
			charstring: []byte{189, 149, 22, 159, 7, 14},
			wantStartX: 10,
			wantStartY: 0,
			wantEndX:   10,
			wantEndY:   20,
		},
		{
			name:       "vmoveto width trimmed",
			charstring: []byte{189, 149, 4, 159, 6, 14},
			wantStartX: 0,
			wantStartY: 10,
			wantEndX:   20,
			wantEndY:   10,
		},
		{
			name:       "hhcurveto optional dy1",
			charstring: []byte{149, 159, 159, 149, 159, 27, 14},
			wantStartX: 0,
			wantStartY: 0,
			wantEndX:   60,
			wantEndY:   20,
		},
		{
			name:       "vvcurveto optional dx1",
			charstring: []byte{149, 159, 159, 149, 159, 26, 14},
			wantStartX: 0,
			wantStartY: 0,
			wantEndX:   30,
			wantEndY:   50,
		},
		{
			name:       "hvcurveto final dxf",
			charstring: []byte{159, 159, 149, 149, 149, 31, 14},
			wantStartX: 0,
			wantStartY: 0,
			wantEndX:   50,
			wantEndY:   20,
		},
		{
			name:       "vhcurveto final dyf",
			charstring: []byte{159, 149, 149, 159, 149, 30, 14},
			wantStartX: 0,
			wantStartY: 0,
			wantEndX:   30,
			wantEndY:   40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			font := &cffFont{CharStrings: testCFFIndex(t, [][]byte{tt.charstring})}
			outline, err := font.outline(0)
			if err != nil {
				t.Fatalf("outline: %v", err)
			}
			if len(outline.Contours) != 1 || len(outline.Contours[0]) < 2 {
				t.Fatalf("unexpected contours: %#v", outline.Contours)
			}
			contour := outline.Contours[0]
			start := contour[0]
			end := contour[len(contour)-2]
			if start.X != tt.wantStartX || start.Y != tt.wantStartY {
				t.Fatalf("start point = (%v, %v)", start.X, start.Y)
			}
			if end.X != tt.wantEndX || end.Y != tt.wantEndY {
				t.Fatalf("end point = (%v, %v)", end.X, end.Y)
			}
		})
	}
}

func buildTestOTFCFF() []byte {
	tables := map[string][]byte{
		"head": buildTestHeadTable(),
		"hhea": buildTestHheaTable(),
		"maxp": buildTestMaxpTable(),
		"hmtx": buildTestHmtxTable(),
		"cmap": buildTestCmapTable(),
		"CFF ": buildTestCFFTable(),
	}
	order := []string{"head", "hhea", "maxp", "hmtx", "cmap", "CFF "}

	directorySize := 12 + len(order)*16
	offset := directorySize
	type record struct {
		tag    string
		offset int
		length int
	}
	records := make([]record, 0, len(order))
	for _, tag := range order {
		data := tables[tag]
		records = append(records, record{tag: tag, offset: offset, length: len(data)})
		offset += align4(len(data))
	}

	font := make([]byte, offset)
	binary.BigEndian.PutUint32(font[0:4], 0x4f54544f)
	binary.BigEndian.PutUint16(font[4:6], uint16(len(order)))
	for i, record := range records {
		entryOffset := 12 + i*16
		copy(font[entryOffset:entryOffset+4], []byte(record.tag))
		binary.BigEndian.PutUint32(font[entryOffset+8:entryOffset+12], uint32(record.offset))
		binary.BigEndian.PutUint32(font[entryOffset+12:entryOffset+16], uint32(record.length))
		copy(font[record.offset:record.offset+record.length], tables[record.tag])
	}

	return font
}

func align4(n int) int {
	return (n + 3) &^ 3
}

func buildTestHeadTable() []byte {
	data := make([]byte, 54)
	binary.BigEndian.PutUint16(data[18:20], 1000)
	binary.BigEndian.PutUint16(data[36:38], 0)
	binary.BigEndian.PutUint16(data[38:40], 0)
	binary.BigEndian.PutUint16(data[40:42], 150)
	binary.BigEndian.PutUint16(data[42:44], 150)
	binary.BigEndian.PutUint16(data[50:52], 0)
	return data
}

func buildTestHheaTable() []byte {
	data := make([]byte, 36)
	descent := int16(-200)
	binary.BigEndian.PutUint16(data[4:6], 800)
	binary.BigEndian.PutUint16(data[6:8], uint16(descent))
	binary.BigEndian.PutUint16(data[10:12], 200)
	binary.BigEndian.PutUint16(data[34:36], 2)
	return data
}

func buildTestMaxpTable() []byte {
	data := make([]byte, 6)
	binary.BigEndian.PutUint16(data[4:6], 2)
	return data
}

func buildTestHmtxTable() []byte {
	data := make([]byte, 8)
	binary.BigEndian.PutUint16(data[0:2], 200)
	binary.BigEndian.PutUint16(data[2:4], 0)
	binary.BigEndian.PutUint16(data[4:6], 200)
	binary.BigEndian.PutUint16(data[6:8], 0)
	return data
}

func buildTestCmapTable() []byte {
	format4 := make([]byte, 32)
	primaryDelta := int16(-64)
	binary.BigEndian.PutUint16(format4[0:2], 4)
	binary.BigEndian.PutUint16(format4[2:4], uint16(len(format4)))
	binary.BigEndian.PutUint16(format4[6:8], 4)
	binary.BigEndian.PutUint16(format4[8:10], 4)
	binary.BigEndian.PutUint16(format4[10:12], 1)
	binary.BigEndian.PutUint16(format4[12:14], 0)
	binary.BigEndian.PutUint16(format4[14:16], 65)
	binary.BigEndian.PutUint16(format4[16:18], 0xffff)
	binary.BigEndian.PutUint16(format4[20:22], 65)
	binary.BigEndian.PutUint16(format4[22:24], 0xffff)
	binary.BigEndian.PutUint16(format4[24:26], uint16(primaryDelta))
	binary.BigEndian.PutUint16(format4[26:28], 1)

	data := make([]byte, 12+len(format4))
	binary.BigEndian.PutUint16(data[2:4], 1)
	binary.BigEndian.PutUint16(data[4:6], 3)
	binary.BigEndian.PutUint16(data[6:8], 1)
	binary.BigEndian.PutUint32(data[8:12], 12)
	copy(data[12:], format4)
	return data
}

func buildTestCFFTable() []byte {
	nameIndex := buildCFFIndex([][]byte{[]byte("Test")})
	charset := []byte{0, 0x00, 0x01}
	charStrings := buildCFFIndex([][]byte{
		{14},
		{239, 239, 21, 189, 139, 5, 139, 189, 5, 89, 139, 5, 14},
	})
	stringIndex := buildCFFIndex(nil)
	globalSubrs := buildCFFIndex(nil)

	header := []byte{1, 0, 4, 1}
	charsetOffset := len(header) + len(nameIndex) + 13 + len(stringIndex) + len(globalSubrs)
	charStringsOffset := charsetOffset + len(charset)
	topDict := buildCFFIndex([][]byte{buildTestTopDict(charsetOffset, charStringsOffset)})

	var data bytes.Buffer
	data.Write(header)
	data.Write(nameIndex)
	data.Write(topDict)
	data.Write(stringIndex)
	data.Write(globalSubrs)
	data.Write(charset)
	data.Write(charStrings)
	return data.Bytes()
}

func buildCFFIndex(objects [][]byte) []byte {
	if len(objects) == 0 {
		return []byte{0, 0}
	}
	var data bytes.Buffer
	binary.Write(&data, binary.BigEndian, uint16(len(objects)))
	data.WriteByte(1)
	offset := 1
	data.WriteByte(byte(offset))
	for _, object := range objects {
		offset += len(object)
		data.WriteByte(byte(offset))
	}
	for _, object := range objects {
		data.Write(object)
	}
	return data.Bytes()
}

func buildTestTopDict(charsetOffset, charStringsOffset int) []byte {
	return []byte{
		28, byte(charsetOffset >> 8), byte(charsetOffset), 15,
		28, byte(charStringsOffset >> 8), byte(charStringsOffset), 17,
	}
}

func testCFFIndex(t *testing.T, objects [][]byte) cffIndex {
	t.Helper()
	index, _, err := parseCFFIndex(buildCFFIndex(objects), 0)
	if err != nil {
		t.Fatal(err)
	}
	return index
}
