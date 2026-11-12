package graphics

import (
	"bytes"
	"fmt"
	pathpkg "path"
	"strings"
)

// ImageDecoder decodes a scene image. Concrete implementations live in
// graphics/codec subpackages so unused image formats are not linked.
type ImageDecoder interface {
	DecodeImage([]byte, string, TextureOptions) (*Texture, error)
}

// ImageFormatDecoder decodes one filename extension in a static decoder set.
type ImageFormatDecoder interface {
	ImageExtension() string
	DecodeImage([]byte, TextureOptions) (*Texture, error)
}

// ImageContainerDecoder claims payloads by their leading bytes rather than by
// the extension they are addressed by.
//
// A build packs platform-specific texture containers in place of the source
// image, so the same path carries either the source encoding or the container
// depending on how that platform was packed. The extension therefore no longer
// identifies the payload, and a decoder that owns a container magic is asked
// first.
type ImageContainerDecoder interface {
	ImageFormatDecoder
	ImageContainerMagic() []byte
}

// NoImageDecoder fills an unused slot in ImageDecoderSet.
type NoImageDecoder struct{}

func (NoImageDecoder) ImageExtension() string { return "" }

func (NoImageDecoder) DecodeImage(_ []byte, _ TextureOptions) (*Texture, error) {
	return nil, fmt.Errorf("image decoder is not configured")
}

// ImageDecoderSet is a compile-time image format collection. It performs no
// registration and does not allocate while selecting a decoder.
type ImageDecoderSet[A, B, C, D, E ImageFormatDecoder] struct{}

// ImageDecoderWith extends a static collection with one additional format.
// Container magic takes precedence over the authored filename extension.
type ImageDecoderWith[Base ImageDecoder, Extra ImageFormatDecoder] struct{}

func (ImageDecoderWith[Base, Extra]) DecodeImage(data []byte, name string, options TextureOptions) (*Texture, error) {
	if texture, err, claimed := (ImageDecoderWith[Base, Extra]{}).DecodeImageContainer(data, options); claimed {
		return texture, err
	}
	if texture, matched, err := decodeImageFormat[Extra](data, strings.ToLower(pathpkg.Ext(name)), options); matched {
		return texture, err
	}
	var base Base
	return base.DecodeImage(data, name, options)
}

type imageContainerCollection interface {
	DecodeImageContainer([]byte, TextureOptions) (*Texture, error, bool)
}

func (ImageDecoderWith[Base, Extra]) DecodeImageContainer(data []byte, options TextureOptions) (*Texture, error, bool) {
	if texture, err, claimed := decodeImageContainer[Extra](data, options); claimed {
		return texture, err, true
	}
	var base Base
	if collection, ok := any(base).(imageContainerCollection); ok {
		return collection.DecodeImageContainer(data, options)
	}
	return nil, nil, false
}

func (ImageDecoderSet[A, B, C, D, E]) DecodeImageContainer(data []byte, options TextureOptions) (*Texture, error, bool) {
	if texture, err, claimed := decodeImageContainer[A](data, options); claimed {
		return texture, err, true
	}
	if texture, err, claimed := decodeImageContainer[B](data, options); claimed {
		return texture, err, true
	}
	if texture, err, claimed := decodeImageContainer[C](data, options); claimed {
		return texture, err, true
	}
	if texture, err, claimed := decodeImageContainer[D](data, options); claimed {
		return texture, err, true
	}
	if texture, err, claimed := decodeImageContainer[E](data, options); claimed {
		return texture, err, true
	}
	return nil, nil, false
}

func (ImageDecoderSet[A, B, C, D, E]) DecodeImage(data []byte, path string, options TextureOptions) (*Texture, error) {
	if texture, err, claimed := (ImageDecoderSet[A, B, C, D, E]{}).DecodeImageContainer(data, options); claimed {
		return texture, err
	}
	extension := strings.ToLower(pathpkg.Ext(path))
	var decodeErr error
	if texture, matched, err := decodeImageFormat[A](data, extension, options); matched {
		if err == nil {
			return texture, nil
		}
		decodeErr = err
	}
	if texture, matched, err := decodeImageFormat[B](data, extension, options); matched {
		if err == nil {
			return texture, nil
		}
		decodeErr = err
	}
	if texture, matched, err := decodeImageFormat[C](data, extension, options); matched {
		if err == nil {
			return texture, nil
		}
		decodeErr = err
	}
	if texture, matched, err := decodeImageFormat[D](data, extension, options); matched {
		if err == nil {
			return texture, nil
		}
		decodeErr = err
	}
	if texture, matched, err := decodeImageFormat[E](data, extension, options); matched {
		if err == nil {
			return texture, nil
		}
		decodeErr = err
	}
	if decodeErr != nil {
		return nil, decodeErr
	}
	return nil, fmt.Errorf("unsupported image format %q", extension)
}

// decodeImageContainer reports whether D claims the payload by magic. A claimed
// payload never falls through to another decoder: bytes carrying a container
// magic are that container, so a failure is corruption rather than a mismatch.
func decodeImageContainer[D ImageFormatDecoder](data []byte, options TextureOptions) (*Texture, error, bool) {
	var decoder D
	container, ok := any(decoder).(ImageContainerDecoder)
	if !ok {
		return nil, nil, false
	}
	claim := container.ImageContainerMagic()
	if len(claim) == 0 || len(data) < len(claim) || !bytes.Equal(data[:len(claim)], claim) {
		return nil, nil, false
	}
	texture, err := container.DecodeImage(data, options)
	return texture, err, true
}

func decodeImageFormat[D ImageFormatDecoder](data []byte, extension string, options TextureOptions) (*Texture, bool, error) {
	var decoder D
	if extension == "" || extension != decoder.ImageExtension() {
		return nil, false, nil
	}
	texture, err := decoder.DecodeImage(data, options)
	return texture, true, err
}
