package font

import (
	"encoding/binary"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"weak"
)

func TestCFFIndexParsingDoesNotAllocatePerGlyph(t *testing.T) {
	const count = 60000
	data := make([]byte, 3+(count+1)*4+count)
	binary.BigEndian.PutUint16(data, count)
	data[2] = 4
	for i := 0; i <= count; i++ {
		binary.BigEndian.PutUint32(data[3+i*4:], uint32(i+1))
	}
	for i := 3 + (count+1)*4; i < len(data); i++ {
		data[i] = 14
	}
	if allocations := testing.AllocsPerRun(10, func() {
		index, _, err := parseCFFIndex(data, 0)
		if err != nil || index.Len() != count || index.At(59999)[0] != 14 {
			t.Fatal("large borrowed INDEX changed its objects")
		}
	}); allocations != 0 {
		t.Fatalf("CFF index allocated %.0f times", allocations)
	}
}

func TestCmap12BorrowsValidatedGroups(t *testing.T) {
	data := make([]byte, 40)
	binary.BigEndian.PutUint16(data, 12)
	binary.BigEndian.PutUint32(data[4:], uint32(len(data)))
	binary.BigEndian.PutUint32(data[12:], 2)
	for i, values := range [][3]uint32{{65, 90, 1}, {0x4e00, 0x4e02, 100}} {
		for j, value := range values {
			binary.BigEndian.PutUint32(data[16+i*12+j*4:], value)
		}
	}
	cmap, err := parseCmapFormat12(data)
	if err != nil {
		t.Fatal(err)
	}
	for r, want := range map[rune]uint16{'A': 1, 'Z': 26, '中': 0, '\u4e01': 101, 0x10ffff: 0} {
		if got := cmap.GlyphIndex(r); got != want {
			t.Fatalf("%U = %d, want %d", r, got, want)
		}
	}
	if &cmap.data[0] != &data[16] {
		t.Fatal("cmap duplicated the mapped group table")
	}
	binary.BigEndian.PutUint32(data[28:], 80)
	if _, err := parseCmapFormat12(data); err == nil {
		t.Fatal("overlapping cmap groups were accepted")
	}
}

func TestCFFFDSelectKeepsCompactGlyphAssignments(t *testing.T) {
	// Format 3: glyphs 0..1 use dictionary 0; glyphs 2..4 use dictionary 1.
	assignments, err := parseCFFFDSelect([]byte{3, 0, 2, 0, 0, 0, 0, 2, 1, 0, 5}, 0, 5, 2)
	if err != nil {
		t.Fatal(err)
	}
	for i, got := range assignments {
		want := uint8(0)
		if i >= 2 {
			want = 1
		}
		if got != want {
			t.Fatalf("glyph %d dictionary = %d", i, got)
		}
	}
}

func TestBitmapBudgetAndIdleTrimPreserveReturnedMasks(t *testing.T) {
	face := new(Face)
	face.ensureCachesLocked()
	first := &GlyphBitmap{Image: image.NewAlpha(image.Rect(0, 0, 512, 512))}
	first.Image.Pix[0] = 123
	for i := range 12 {
		bitmap := &GlyphBitmap{Image: image.NewAlpha(image.Rect(0, 0, 512, 512))}
		if i == 0 {
			bitmap = first
		}
		face.cacheBitmap(glyphBitmapCacheKey{glyphID: uint16(i)}, bitmap)
	}
	if face.bitmapBytes > 1<<20 || len(face.glyphBitmapCache) > 4 {
		t.Fatal("CPU bitmap cache bypassed its memory budget")
	}
	face.TrimCaches()
	if face.bitmapBytes != 0 || face.glyphBitmapCache != nil || first.Image.Pix[0] != 123 {
		t.Fatal("idle trim retained cache memory or invalidated a returned mask")
	}
	italic := face.Italic()
	italic.ensureCachesLocked()
	italic.cacheBitmap(glyphBitmapCacheKey{}, first)
	face.TrimCaches()
	if italic.glyphBitmapCache != nil {
		t.Fatal("idle trim retained synthetic italic masks")
	}
}

func TestFallbackSourceReleasesUnusedFaceAndCanReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fallback.otf")
	if err := os.WriteFile(path, buildTestOTFCFF(), 0600); err != nil {
		t.Fatal(err)
	}
	source := &fallbackSource{font: SystemFont{Path: path}}
	load := func() weak.Pointer[Face] {
		glyph := source.glyph('A')
		if glyph.face == nil || glyph.id == 0 {
			t.Fatal("fallback did not load")
		}
		return weak.Make(glyph.face)
	}
	previous := load()
	runtime.GC()
	if previous.Value() != nil {
		t.Fatal("fallback source pinned an unused face")
	}
	glyph := source.glyph('A')
	if glyph.face == nil || glyph.id == 0 {
		t.Fatal("collected fallback could not reload")
	}
	defer glyph.face.Close()
	runtime.KeepAlive(source)
}

type collectingCmap struct{ owner weak.Pointer[Face] }

func (c collectingCmap) GlyphIndex(rune) uint16 {
	runtime.GC()
	if c.owner.Value() == nil {
		return 0
	}
	return 1
}

//go:noinline
func lookupWithOnlyReceiverAlive() uint16 {
	face := new(Face)
	face.Cmap = collectingCmap{owner: weak.Make(face)}
	return face.GlyphIndex('界')
}

func TestBorrowedCmapKeepsFaceAliveDuringLookup(t *testing.T) {
	if lookupWithOnlyReceiverAlive() != 1 {
		t.Fatal("font backing could be released during a cmap lookup")
	}
}

func TestFallbackIdleTrimPreservesLiveFaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fallback.otf")
	if err := os.WriteFile(path, buildTestOTFCFF(), 0600); err != nil {
		t.Fatal(err)
	}
	source := &fallbackSource{font: SystemFont{Path: path}}
	set := &fallbackSet{sources: []*fallbackSource{source}, glyphs: make(map[rune]resolvedGlyph)}
	set.once.Do(func() {})
	style := fallbackStyle{weight: 999}
	systemFallbacks.Lock()
	systemFallbacks.styles[style] = set
	systemFallbacks.Unlock()
	defer func() {
		systemFallbacks.Lock()
		delete(systemFallbacks.styles, style)
		systemFallbacks.Unlock()
	}()
	glyph := set.resolve('A')
	if glyph.face == nil || glyph.id == 0 {
		t.Fatal("fallback did not load")
	}
	defer glyph.face.Close()
	TrimSystemFallbackCaches()
	if set.glyphs != nil || set.order != nil {
		t.Fatal("idle trim retained fallback resolutions")
	}
	if glyph.face.GlyphIndex('A') != glyph.id {
		t.Fatal("idle trim closed a live fallback")
	}
	if next := set.resolve('A'); next.face != glyph.face || next.id != glyph.id {
		t.Fatal("live weak fallback was not reused")
	}
}
