package assets

import _ "embed"

// JetBrains Mono NL Nerd Font Mono v3.5.1; redistribution license in fonts/LICENSE.txt.
//
//go:embed fonts/JetBrainsMonoNLNerdFontMono-Regular.ttf
var Regular []byte

//go:embed fonts/JetBrainsMonoNLNerdFontMono-Bold.ttf
var Bold []byte

//go:embed icons/xiterm.png
var IconPNG []byte

// IsNerdGlyph covers the Unicode private-use ranges used by patched icon fonts.
func IsNerdGlyph(r rune) bool {
	return r >= 0xe000 && r <= 0xf8ff || r >= 0xf0000 && r <= 0xffffd || r >= 0x100000 && r <= 0x10fffd
}
