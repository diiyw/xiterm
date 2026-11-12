package text

import (
	"errors"
	"sync"

	"github.com/onix-engine/onix/font"
)

type Font struct {
	italicVariant *Font
	*font.Face
	caches    map[glyphCacheKey]*GlyphCache
	atlasPool *glyphAtlasPool
	cacheTick uint64
	mu        sync.Mutex
}

var defaultFont = sync.OnceValue(func() *Font {
	face, err := font.LoadDefault()
	if err != nil {
		return nil
	}
	return NewFont(face)
})

// DefaultFont discovers and loads the system UI font once. The shared font
// lives for the process lifetime; callers must not close it. Nil means the
// system has no usable font, so the caller must supply one.
func DefaultFont() *Font { return defaultFont() }

const glyphSizeCacheLimit = 32

type glyphCacheKey struct {
	size float64
	dpi  float64
}

func NewFont(face *font.Face) *Font {
	if face == nil {
		return nil
	}
	face.UseSystemFallback()
	return &Font{Face: face}
}

// FontFace creates a new font face.
func (f *Font) FontFace(size, dpi float64) *font.ScaledFace {
	face, err := font.NewFace(f.Face, &font.FaceOptions{
		Size: size,
		DPI:  dpi,
	})
	if err != nil {
		panic(err)
	}
	return face
}

// MeasureString measures the size of the string.
func (f *Font) MeasureString(face *font.ScaledFace, value string) (int, int, int) {
	if face == nil {
		return 0, 0, 0
	}
	prevC := rune(-1)
	var advance font.Int26_6
	for _, c := range value {
		if prevC >= 0 {
			advance += face.Kern(prevC, c)
		}
		a, _ := face.GlyphAdvance(c)
		advance += a
		prevC = c
	}
	metric := face.Metrics()
	return int(advance >> 6), int(metric.Descent >> 6), int(metric.Height >> 6)
}

func (f *Font) GlyphCache(size, dpi float64) *GlyphCache {
	key := glyphCacheKey{size: size, dpi: dpi}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cacheTick++
	if cache, ok := f.caches[key]; ok {
		cache.lastUse = f.cacheTick
		return cache
	}
	if f.caches == nil {
		f.caches = make(map[glyphCacheKey]*GlyphCache)
	}
	if len(f.caches) >= glyphSizeCacheLimit {
		var oldest glyphCacheKey
		tick := ^uint64(0)
		for key, cache := range f.caches {
			if cache.lastUse < tick {
				oldest, tick = key, cache.lastUse
			}
		}
		// Existing text can still own this cache and its glyph geometry.
		delete(f.caches, oldest)
	}
	if f.atlasPool == nil {
		f.atlasPool = new(glyphAtlasPool)
	}
	cache := newGlyphCache(f, size, dpi, f.atlasPool, false)
	cache.lastUse = f.cacheTick
	f.caches[key] = cache
	return cache
}

func (f *Font) Dispose() error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	caches := f.caches
	f.caches = nil
	pool := f.atlasPool
	f.atlasPool = nil
	italic := f.italicVariant
	f.italicVariant = nil
	f.mu.Unlock()

	errList := make([]error, 0, len(caches))
	if italic != nil {
		if err := italic.Dispose(); err != nil {
			errList = append(errList, err)
		}
	}
	for _, cache := range caches {
		if err := cache.Dispose(); err != nil {
			errList = append(errList, err)
		}
	}
	if pool != nil {
		if err := pool.dispose(); err != nil {
			errList = append(errList, err)
		}
	}
	if err := f.Face.Close(); err != nil {
		errList = append(errList, err)
	}
	return errors.Join(errList...)
}
