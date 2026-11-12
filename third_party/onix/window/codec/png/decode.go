// Package png provides the opt-in PNG window icon decoder.
package png

import (
	"bytes"
	"image"
	stdpng "image/png"
)

type Decoder struct{}

func (Decoder) DecodeIcon(data []byte) (image.Image, error) {
	return stdpng.Decode(bytes.NewReader(data))
}
