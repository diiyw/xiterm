package assets

import "embed"

// UIIcons contains the Tabler SVG sources used by the native interface.
//
//go:embed icons/ui/*.svg
var UIIcons embed.FS
