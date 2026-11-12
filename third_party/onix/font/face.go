package font

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"runtime"
	"sync"
	"weak"
)

type Face struct {
	italicSource     *Face
	italicVariant    *Face
	glyphFallback    *Face
	Cmap             cmap
	outlineCache     map[uint16]weak.Pointer[Outline]
	Kern             kernTable
	backing          *fontBacking
	glyphBitmapCache map[glyphBitmapCacheKey]*GlyphBitmap
	SFNT             *sfnt
	fallback         *fallbackSet
	CFF              *cffFont
	color            colorGlyphTables
	Glyf             []byte
	Data             []byte
	bitmapOrder      []glyphBitmapCacheKey
	Loca             []uint32
	pairLookups      []pairLookup
	Metrics          []LongHorMetric
	cacheKey         uint64
	weight           int
	bitmapBytes      int
	bitmapHead       int
	cacheMu          sync.RWMutex
	asciiGlyphs      [128]uint16
	Hhea             Hhea
	Head             Head
	Maxp             Maxp
	xHeight          int16
	capHeight        int16
	italic           bool
}

func (f *Face) attachBacking(backing *fontBacking) {
	if f == nil || backing == nil {
		return
	}
	f.backing = backing
	runtime.AddCleanup(f, closeFontBacking, backing)
}

// Close releases the file or memory backing owned by the face. Callers must
// stop using the face and any scaled faces created from it before closing it.
func (f *Face) Close() error {
	if f == nil {
		return nil
	}
	f.cacheMu.Lock()
	backing := f.backing
	f.backing = nil
	f.Data = nil
	f.SFNT = nil
	f.Metrics = nil
	f.Loca = nil
	f.Cmap = nil
	f.Glyf = nil
	f.CFF = nil
	f.Kern = nil
	f.pairLookups = nil
	f.fallback = nil
	f.glyphFallback = nil
	f.color = colorGlyphTables{}
	f.outlineCache = nil
	f.glyphBitmapCache = nil
	f.bitmapOrder, f.bitmapHead, f.bitmapBytes = nil, 0, 0
	f.cacheMu.Unlock()
	return backing.close()
}

func Parse(data []byte) (*Face, error) {
	return ParseIndex(data, 0)
}

func ParseIndex(data []byte, index int) (*Face, error) {
	sfntData, err := parseSFNT(data, index)
	if err != nil {
		return nil, err
	}
	headData, err := sfntData.table("head")
	if err != nil {
		return nil, err
	}
	hheaData, err := sfntData.table("hhea")
	if err != nil {
		return nil, err
	}
	maxpData, err := sfntData.table("maxp")
	if err != nil {
		return nil, err
	}
	hmtxData, err := sfntData.table("hmtx")
	if err != nil {
		return nil, err
	}
	cmapData, err := sfntData.table("cmap")
	if err != nil {
		return nil, err
	}
	head, err := parseHead(headData)
	if err != nil {
		return nil, err
	}
	hhea, err := parseHhea(hheaData)
	if err != nil {
		return nil, err
	}
	maxp, err := parseMaxp(maxpData)
	if err != nil {
		return nil, err
	}
	metrics, err := parseHmtx(hmtxData, maxp.NumGlyphs, hhea.NumberOfHMetrics)
	if err != nil {
		return nil, err
	}
	cmapTable, err := parseCmap(cmapData)
	if err != nil {
		return nil, err
	}

	face := &Face{
		Data:             data,
		SFNT:             sfntData,
		Head:             head,
		Hhea:             hhea,
		Maxp:             maxp,
		Metrics:          metrics,
		Cmap:             cmapTable,
		outlineCache:     map[uint16]weak.Pointer[Outline]{},
		glyphBitmapCache: map[glyphBitmapCacheKey]*GlyphBitmap{},
	}
	face.color.load(sfntData)
	for r := range face.asciiGlyphs {
		face.asciiGlyphs[r] = cmapTable.GlyphIndex(rune(r))
	}
	if os2, err := sfntData.table("OS/2"); err == nil {
		if len(os2) >= 6 {
			face.weight = int(binary.BigEndian.Uint16(os2[4:6]))
		}
		if len(os2) >= 64 {
			face.italic = binary.BigEndian.Uint16(os2[62:64])&0x201 != 0
		}
		if len(os2) >= 90 && binary.BigEndian.Uint16(os2) >= 2 {
			face.xHeight = int16(binary.BigEndian.Uint16(os2[86:88]))
			face.capHeight = int16(binary.BigEndian.Uint16(os2[88:90]))
		}
	}
	if gposData, err := sfntData.table("GPOS"); err == nil {
		face.pairLookups = parsePairLookups(gposData)
	}
	if kernData, err := sfntData.table("kern"); err == nil {
		kern, err := parseKern(kernData)
		if err != nil {
			return nil, err
		}
		face.Kern = kern
	}
	if cffData, err := sfntData.table("CFF "); err == nil {
		cffFont, err := parseCFF(cffData, maxp.NumGlyphs)
		if err != nil {
			return nil, err
		}
		face.CFF = cffFont
		return face, nil
	}

	locaData, err := sfntData.table("loca")
	if err != nil {
		if len(face.color.cbdt) != 0 || len(face.color.sbix) != 0 {
			return face, nil
		}
		return nil, err
	}
	glyfData, err := sfntData.table("glyf")
	if err != nil {
		return nil, err
	}
	loca, err := parseLoca(locaData, maxp.NumGlyphs, head.IndexToLocFormat)
	if err != nil {
		return nil, err
	}
	face.Loca = loca
	face.Glyf = glyfData
	return face, nil
}

func (f *Face) GlyphIndex(r rune) uint16 {
	if f == nil || f.Cmap == nil {
		return 0
	}
	if uint32(r) < uint32(len(f.asciiGlyphs)) {
		return f.asciiGlyphs[r]
	}
	id := f.Cmap.GlyphIndex(r)
	// Borrowed cmap bytes must remain mapped until the lookup completes, even
	// if a weak fallback source is the only other reference to this face.
	runtime.KeepAlive(f)
	return id
}

func (f *Face) ensureCachesLocked() {
	if f.outlineCache == nil {
		f.outlineCache = map[uint16]weak.Pointer[Outline]{}
	}
	if f.glyphBitmapCache == nil {
		f.glyphBitmapCache = map[glyphBitmapCacheKey]*GlyphBitmap{}
	}
}

func (f *Face) CacheKey() uint64 {
	if f == nil {
		return 0
	}
	f.cacheMu.RLock()
	if f.cacheKey != 0 {
		key := f.cacheKey
		f.cacheMu.RUnlock()
		return key
	}
	f.cacheMu.RUnlock()

	h := fnv.New64a()
	_, _ = h.Write(f.Data)
	key := h.Sum64()
	if key == 0 {
		key = 1
	}

	f.cacheMu.Lock()
	if f.cacheKey == 0 {
		f.cacheKey = key
	}
	key = f.cacheKey
	f.cacheMu.Unlock()
	return key
}

func (f *Face) HorizontalMetrics(glyphID uint16) (LongHorMetric, error) {
	if int(glyphID) >= len(f.Metrics) {
		return LongHorMetric{}, fmt.Errorf("glyph id %d out of range", glyphID)
	}
	return f.Metrics[glyphID], nil
}

// CapHeight returns the cap-height metric in font units, using H for fonts
// without the optional OS/2 metric. It does not rasterize a glyph.
func (f *Face) CapHeight() int16 {
	return f.textEdgeHeight(f.capHeight, 'H')
}

// XHeight returns the x-height metric in font units without rasterizing.
func (f *Face) XHeight() int16 {
	return f.textEdgeHeight(f.xHeight, 'x')
}

func (f *Face) textEdgeHeight(height int16, character rune) int16 {
	if height > 0 {
		return height
	}
	if id := f.GlyphIndex(character); id != 0 {
		if bounds, err := f.GlyphBounds(id); err == nil && bounds.YMax > 0 {
			return bounds.YMax
		}
	}
	return f.Hhea.Ascent
}

func (f *Face) Glyph(glyphID uint16) (*Glyph, error) {
	defer runtime.KeepAlive(f)
	if f.CFF != nil {
		return nil, fmt.Errorf("glyph extraction is only available for glyf-based fonts")
	}
	if int(glyphID)+1 >= len(f.Loca) {
		return nil, fmt.Errorf("glyph id %d out of range", glyphID)
	}
	start := f.Loca[glyphID]
	end := f.Loca[glyphID+1]
	if start > end || int(end) > len(f.Glyf) {
		return nil, fmt.Errorf("glyf offsets out of range for glyph %d", glyphID)
	}
	return parseGlyph(f.Glyf[start:end], glyphID)
}

// GlyphBounds reads TrueType ink bounds without decoding or retaining contours.
func (f *Face) GlyphBounds(glyphID uint16) (Bounds, error) {
	defer runtime.KeepAlive(f)
	if f.italicSource != nil {
		bounds, err := f.italicSource.GlyphBounds(glyphID)
		return italicBounds(bounds), err
	}
	if f.CFF != nil {
		outline, err := f.Outline(glyphID)
		if err != nil || outline == nil {
			return Bounds{}, err
		}
		return outline.Bounds, nil
	}
	if int(glyphID)+1 >= len(f.Loca) {
		return Bounds{}, fmt.Errorf("glyph id %d out of range", glyphID)
	}
	start, end := f.Loca[glyphID], f.Loca[glyphID+1]
	if start > end || int(end) > len(f.Glyf) {
		return Bounds{}, fmt.Errorf("glyf offsets out of range for glyph %d", glyphID)
	}
	data := f.Glyf[start:end]
	if len(data) == 0 {
		return Bounds{}, nil
	}
	if len(data) < 10 {
		return Bounds{}, fmt.Errorf("glyf header too short for glyph %d", glyphID)
	}
	return Bounds{
		XMin: int16(binary.BigEndian.Uint16(data[2:4])),
		YMin: int16(binary.BigEndian.Uint16(data[4:6])),
		XMax: int16(binary.BigEndian.Uint16(data[6:8])),
		YMax: int16(binary.BigEndian.Uint16(data[8:10])),
	}, nil
}

// TrimCaches releases regenerable CPU glyph data without invalidating faces,
// outlines or bitmap masks already held by a renderer.
func (f *Face) TrimCaches() {
	if f == nil {
		return
	}
	f.cacheMu.Lock()
	f.outlineCache = nil
	f.glyphBitmapCache = nil
	f.bitmapOrder, f.bitmapHead, f.bitmapBytes = nil, 0, 0
	italic := f.italicVariant
	f.cacheMu.Unlock()
	if italic != nil {
		italic.TrimCaches()
	}
}
