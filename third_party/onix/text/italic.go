package text

// Italic shares a cached font with synthetic oblique glyphs. Existing italic
// faces are returned unchanged. Repeated calls do not allocate.
func (f *Font) Italic() *Font {
	if f == nil {
		return nil
	}
	face := f.Face.Italic()
	if face == f.Face {
		return f
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.italicVariant == nil {
		f.italicVariant = &Font{Face: face}
	}
	return f.italicVariant
}
