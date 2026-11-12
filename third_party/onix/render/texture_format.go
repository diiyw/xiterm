package render

// TextureFormat identifies the texel layout used by a sampled texture.
// RGBA8 remains the zero value so existing descriptors keep their behavior.
type TextureFormat uint8

const (
	TextureFormatRGBA8 TextureFormat = iota
	TextureFormatR8
	// TextureFormatSRGBA8 decodes RGB from sRGB when sampled; alpha stays linear.
	TextureFormatSRGBA8

	// Block-compressed formats encode independent 4x4 texel blocks, so a
	// texture smaller than the block size still occupies a whole block.
	TextureFormatBC1         // RGB, 4 bits per texel
	TextureFormatBC1SRGB     // BC1 sampled as sRGB
	TextureFormatBC3         // RGB with interpolated alpha, 8 bits per texel
	TextureFormatBC3SRGB     // BC3 sampled as sRGB
	TextureFormatBC4         // single channel, 4 bits per texel
	TextureFormatBC5         // two channel, 8 bits per texel
	TextureFormatBC7         // RGBA, 8 bits per texel
	TextureFormatBC7SRGB     // BC7 sampled as sRGB
	TextureFormatASTC4x4     // RGBA, 8 bits per texel
	TextureFormatASTC4x4SRGB // ASTC 4x4 sampled as sRGB
)

// TextureBlock is the texel granularity of a format. Uncompressed formats use a
// 1x1 block; block-compressed formats use 4x4.
type TextureBlock struct {
	Width  int
	Height int
	Bytes  int
}

type textureFormatInfo struct {
	name  string
	block TextureBlock
	srgb  bool
}

var textureFormats = [...]textureFormatInfo{
	TextureFormatRGBA8:       {block: TextureBlock{1, 1, 4}, name: "rgba8"},
	TextureFormatR8:          {block: TextureBlock{1, 1, 1}, name: "r8"},
	TextureFormatSRGBA8:      {block: TextureBlock{1, 1, 4}, srgb: true, name: "srgba8"},
	TextureFormatBC1:         {block: TextureBlock{4, 4, 8}, name: "bc1"},
	TextureFormatBC1SRGB:     {block: TextureBlock{4, 4, 8}, srgb: true, name: "bc1-srgb"},
	TextureFormatBC3:         {block: TextureBlock{4, 4, 16}, name: "bc3"},
	TextureFormatBC3SRGB:     {block: TextureBlock{4, 4, 16}, srgb: true, name: "bc3-srgb"},
	TextureFormatBC4:         {block: TextureBlock{4, 4, 8}, name: "bc4"},
	TextureFormatBC5:         {block: TextureBlock{4, 4, 16}, name: "bc5"},
	TextureFormatBC7:         {block: TextureBlock{4, 4, 16}, name: "bc7"},
	TextureFormatBC7SRGB:     {block: TextureBlock{4, 4, 16}, srgb: true, name: "bc7-srgb"},
	TextureFormatASTC4x4:     {block: TextureBlock{4, 4, 16}, name: "astc-4x4"},
	TextureFormatASTC4x4SRGB: {block: TextureBlock{4, 4, 16}, srgb: true, name: "astc-4x4-srgb"},
}

func (f TextureFormat) info() textureFormatInfo {
	if !f.Valid() {
		return textureFormatInfo{}
	}
	return textureFormats[f]
}

func (f TextureFormat) Valid() bool { return int(f) < len(textureFormats) }

// Block reports the texel granularity and encoded size of one block.
func (f TextureFormat) Block() TextureBlock { return f.info().block }

// Compressed reports whether the format stores 4x4 blocks. Compressed textures
// cannot be updated in place, because a partial region does not cover whole
// blocks.
func (f TextureFormat) Compressed() bool { return f.info().block.Width > 1 }

// TexelBytes reports the encoded size of one texel. Compressed formats report 0
// because their texels are not independently addressable; use LevelSize or
// ChainSize for those.
func (f TextureFormat) TexelBytes() int {
	block := f.Block()
	if block.Width != 1 || block.Height != 1 {
		return 0
	}
	return block.Bytes
}

// SRGB reports whether sampling decodes RGB from sRGB. Alpha stays linear.
func (f TextureFormat) SRGB() bool { return f.info().srgb }

// SRGBVariant returns the format that samples the same blocks with the
// requested colour interpretation.
//
// A block-compressed format and its sRGB variant store identical blocks; only
// the internal format chosen at upload differs, which is what makes the device
// decode gamma. A packed payload therefore does not have to be re-encoded to
// serve a sampler that wants the other interpretation. Formats without a
// variant, and R8 which carries data rather than colour, are returned as-is.
func (f TextureFormat) SRGBVariant(srgb bool) TextureFormat {
	if !f.Valid() || f.info().srgb == srgb {
		return f
	}
	switch f {
	case TextureFormatRGBA8:
		return TextureFormatSRGBA8
	case TextureFormatSRGBA8:
		return TextureFormatRGBA8
	case TextureFormatBC1:
		return TextureFormatBC1SRGB
	case TextureFormatBC1SRGB:
		return TextureFormatBC1
	case TextureFormatBC3:
		return TextureFormatBC3SRGB
	case TextureFormatBC3SRGB:
		return TextureFormatBC3
	case TextureFormatBC7:
		return TextureFormatBC7SRGB
	case TextureFormatBC7SRGB:
		return TextureFormatBC7
	case TextureFormatASTC4x4:
		return TextureFormatASTC4x4SRGB
	case TextureFormatASTC4x4SRGB:
		return TextureFormatASTC4x4
	default:
		return f
	}
}

// Name reports a stable identifier used by tooling and error messages.
func (f TextureFormat) Name() string {
	if name := f.info().name; name != "" {
		return name
	}
	return "invalid"
}

// GLCompressedInternalFormat maps a compressed format to the GL internal format
// constant shared by OpenGL ES and WebGL2. The second result is false for
// uncompressed formats, which take the texel upload path instead.
func (f TextureFormat) GLCompressedInternalFormat() (int, bool) {
	switch f {
	case TextureFormatBC1:
		return 0x83F0, true // COMPRESSED_RGB_S3TC_DXT1
	case TextureFormatBC1SRGB:
		return 0x8C4C, true // COMPRESSED_SRGB_S3TC_DXT1
	case TextureFormatBC3:
		return 0x83F3, true // COMPRESSED_RGBA_S3TC_DXT5
	case TextureFormatBC3SRGB:
		return 0x8C4F, true // COMPRESSED_SRGB_ALPHA_S3TC_DXT5
	case TextureFormatBC4:
		return 0x8DBB, true // COMPRESSED_RED_RGTC1
	case TextureFormatBC5:
		return 0x8DBD, true // COMPRESSED_RG_RGTC2
	case TextureFormatBC7:
		return 0x8E8C, true // COMPRESSED_RGBA_BPTC_UNORM
	case TextureFormatBC7SRGB:
		return 0x8E8D, true // COMPRESSED_SRGB_ALPHA_BPTC_UNORM
	case TextureFormatASTC4x4:
		return 0x93B0, true // COMPRESSED_RGBA_ASTC_4x4_KHR
	case TextureFormatASTC4x4SRGB:
		return 0x93D0, true // COMPRESSED_SRGB8_ALPHA8_ASTC_4x4_KHR
	default:
		return 0, false
	}
}

// LevelSize reports the encoded byte size of one mip level. Dimensions that are
// not a whole number of blocks round up, matching every native API.
func (f TextureFormat) LevelSize(width, height int) int {
	block := f.Block()
	if block.Bytes == 0 || width <= 0 || height <= 0 {
		return 0
	}
	blocks := ((width + block.Width - 1) / block.Width) * ((height + block.Height - 1) / block.Height)
	return blocks * block.Bytes
}

// MipLevels reports how many levels a full chain down to 1x1 contains.
func MipLevels(width, height int) int {
	if width <= 0 || height <= 0 {
		return 0
	}
	levels := 1
	for width > 1 || height > 1 {
		width, height = max(1, width/2), max(1, height/2)
		levels++
	}
	return levels
}

// ChainSize reports the encoded byte size of a full or partial mip chain.
// Levels above the natural depth are clamped.
func (f TextureFormat) ChainSize(width, height, levels int) int {
	levels = min(max(levels, 1), MipLevels(width, height))
	total := 0
	for range levels {
		total += f.LevelSize(width, height)
		width, height = max(1, width/2), max(1, height/2)
	}
	return total
}
