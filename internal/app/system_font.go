package app

import "github.com/onix-engine/onix/text"

// All desktop platforms share UI rendering and system glyph fallback. Only
// discovery of the user's current UI face depends on the operating system.
func loadSystemUIFont() (*text.Font, error) {
	face, err := systemUIFontFace()
	if err != nil {
		return nil, err
	}
	return text.NewFont(face), nil
}
