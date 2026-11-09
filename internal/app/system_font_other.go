//go:build !linux

package app

import "github.com/onix-engine/onix/font"

func systemUIFontFace() (*font.Face, error) {
	// Onix queries Windows' active NONCLIENTMETRICS message font and the
	// macOS CoreText system UI face, including its font collection index.
	return font.LoadDefault()
}
