package font

import (
	"encoding/binary"
	"fmt"
	pathpkg "path"
	"strings"
)

// Decoder decodes one statically selected font format collection.
type Decoder interface {
	DecodeFont([]byte, string) (*Face, error)
}

// FormatDecoder handles one font path extension in DecoderSet.
type FormatDecoder interface {
	MatchesFont([]byte, string) bool
	DecodeFont([]byte, string) (*Face, error)
}

// NoDecoder fills an unused DecoderSet slot.
type NoDecoder struct{}

func (NoDecoder) MatchesFont([]byte, string) bool { return false }

func (NoDecoder) DecodeFont(_ []byte, _ string) (*Face, error) {
	return nil, fmt.Errorf("font decoder is not configured")
}

// SFNTDecoder handles TTF, OTF, and TTC fonts in the core parser.
type SFNTDecoder struct{}

func (SFNTDecoder) MatchesFont(data []byte, path string) bool {
	if len(data) >= 4 {
		signature := string(data[:4])
		if signature == "wOF2" {
			return false
		}
		if signature == "OTTO" || signature == "true" || signature == "ttcf" || binary.BigEndian.Uint32(data[:4]) == 0x00010000 {
			return true
		}
	}
	switch strings.ToLower(pathpkg.Ext(path)) {
	case ".ttf", ".otf", ".ttc":
		return true
	default:
		return false
	}
}

func (SFNTDecoder) DecodeFont(data []byte, _ string) (*Face, error) {
	return Parse(data)
}

// DecoderSet is a compile-time font format collection. It performs no runtime
// registration and does not allocate while selecting a decoder.
type DecoderSet[A, B FormatDecoder] struct{}

func (DecoderSet[A, B]) DecodeFont(data []byte, path string) (*Face, error) {
	if face, matched, err := decodeFontFormat[A](data, path); matched {
		return face, err
	}
	if face, matched, err := decodeFontFormat[B](data, path); matched {
		return face, err
	}
	return nil, fmt.Errorf("unsupported font format %q", pathpkg.Ext(path))
}

func decodeFontFormat[D FormatDecoder](data []byte, path string) (*Face, bool, error) {
	var decoder D
	if !decoder.MatchesFont(data, path) {
		return nil, false, nil
	}
	face, err := decoder.DecodeFont(data, path)
	return face, true, err
}
