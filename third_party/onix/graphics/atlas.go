package graphics

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

var atlasMagic = [8]byte{'O', 'N', 'I', 'X', 'A', 'T', 'L', '1'}

type Atlas struct {
	*Texture
	frames map[string]*Texture
}

type AtlasMetadata struct {
	Frames []AtlasFrame
	Meta   AtlasMeta
}

type AtlasMeta struct {
	Format string    `json:"format"`
	Image  string    `json:"image"`
	Size   AtlasSize `json:"size"`
}

type AtlasSize struct {
	W int `json:"w"`
	H int `json:"h"`
}

type AtlasFrame struct {
	Filename         string      `json:"filename"`
	Bounds           AtlasBounds `json:"frame"`
	SourceSize       AtlasSize   `json:"sourceSize"`
	SpriteSourceSize AtlasBounds `json:"spriteSourceSize"`
	Rotated          bool        `json:"rotated"`
}

type AtlasBounds struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

func (a *Atlas) SubTexture(name string) *Texture {
	if a == nil {
		return nil
	}
	return a.frames[name]
}

func ParseAtlas(data []byte) (*AtlasMetadata, error) {
	if !IsPreprocessedAtlas(data) {
		return parseAtlasSource(data)
	}
	decoder := atlasDecoder{data: data[len(atlasMagic):]}
	metadata := new(AtlasMetadata)
	var err error
	if metadata.Meta.Format, err = decoder.string(); err != nil {
		return nil, fmt.Errorf("decode atlas format: %w", err)
	}
	if metadata.Meta.Image, err = decoder.string(); err != nil {
		return nil, fmt.Errorf("decode atlas image: %w", err)
	}
	if metadata.Meta.Size.W, err = decoder.integer(); err != nil {
		return nil, fmt.Errorf("decode atlas width: %w", err)
	}
	if metadata.Meta.Size.H, err = decoder.integer(); err != nil {
		return nil, fmt.Errorf("decode atlas height: %w", err)
	}
	count, err := decoder.length()
	if err != nil {
		return nil, fmt.Errorf("decode atlas frames: %w", err)
	}
	metadata.Frames = make([]AtlasFrame, count)
	for index := range metadata.Frames {
		frame := &metadata.Frames[index]
		if frame.Filename, err = decoder.string(); err != nil {
			return nil, fmt.Errorf("decode atlas frame %d name: %w", index, err)
		}
		values := [...]*int{
			&frame.Bounds.X, &frame.Bounds.Y, &frame.Bounds.W, &frame.Bounds.H,
			&frame.SourceSize.W, &frame.SourceSize.H,
			&frame.SpriteSourceSize.X, &frame.SpriteSourceSize.Y, &frame.SpriteSourceSize.W, &frame.SpriteSourceSize.H,
		}
		for valueIndex, target := range values {
			if *target, err = decoder.integer(); err != nil {
				return nil, fmt.Errorf("decode atlas frame %q value %d: %w", frame.Filename, valueIndex, err)
			}
		}
		if frame.Rotated, err = decoder.bool(); err != nil {
			return nil, fmt.Errorf("decode atlas frame %q rotation: %w", frame.Filename, err)
		}
	}
	if len(decoder.data) != 0 {
		return nil, fmt.Errorf("decode atlas metadata: %d trailing bytes", len(decoder.data))
	}
	return metadata, nil
}

// IsPreprocessedAtlas reports whether data contains Onix's compiled atlas IR.
func IsPreprocessedAtlas(data []byte) bool {
	return len(data) >= len(atlasMagic) && bytes.Equal(data[:len(atlasMagic)], atlasMagic[:])
}

// MarshalAtlas encodes atlas metadata for allocation-light runtime loading.
func MarshalAtlas(metadata *AtlasMetadata) ([]byte, error) {
	if metadata == nil {
		return nil, errors.New("encode atlas metadata: metadata is nil")
	}
	encoder := atlasEncoder{data: append([]byte(nil), atlasMagic[:]...)}
	encoder.string(metadata.Meta.Format)
	encoder.string(metadata.Meta.Image)
	encoder.integer(metadata.Meta.Size.W)
	encoder.integer(metadata.Meta.Size.H)
	encoder.uvarint(uint64(len(metadata.Frames)))
	for index, frame := range metadata.Frames {
		if frame.Filename == "" {
			return nil, fmt.Errorf("encode atlas frame %d: filename is empty", index)
		}
		encoder.string(frame.Filename)
		for _, value := range [...]int{
			frame.Bounds.X, frame.Bounds.Y, frame.Bounds.W, frame.Bounds.H,
			frame.SourceSize.W, frame.SourceSize.H,
			frame.SpriteSourceSize.X, frame.SpriteSourceSize.Y, frame.SpriteSourceSize.W, frame.SpriteSourceSize.H,
		} {
			encoder.integer(value)
		}
		encoder.bool(frame.Rotated)
	}
	return encoder.data, nil
}

func NewAtlas(texture *Texture, metadata *AtlasMetadata) (*Atlas, error) {
	if texture == nil {
		return nil, fmt.Errorf("atlas texture is nil")
	}
	if metadata == nil {
		return nil, fmt.Errorf("atlas metadata is nil")
	}
	atlasWidth, atlasHeight := metadata.Meta.Size.W, metadata.Meta.Size.H
	if atlasWidth <= 0 {
		atlasWidth = texture.Width
	}
	if atlasHeight <= 0 {
		atlasHeight = texture.Height
	}
	if atlasWidth <= 0 || atlasHeight <= 0 {
		return nil, fmt.Errorf("atlas size must be positive")
	}

	frames := make(map[string]*Texture, len(metadata.Frames))
	for _, frame := range metadata.Frames {
		bounds := frame.Bounds
		textureRegion := NewSubTexture(
			texture,
			bounds.W,
			bounds.H,
			float32(bounds.X)/float32(atlasWidth),
			float32(bounds.Y)/float32(atlasHeight),
			float32(bounds.W)/float32(atlasWidth),
			float32(bounds.H)/float32(atlasHeight),
			0,
		)
		if frame.Rotated {
			textureRegion = NewSubTexture(
				texture,
				bounds.W,
				bounds.H,
				float32(bounds.X+bounds.H)/float32(atlasWidth),
				float32(bounds.Y)/float32(atlasHeight),
				float32(bounds.H)/float32(atlasWidth),
				float32(bounds.W)/float32(atlasHeight),
				90,
			)
		}
		textureRegion.SetSourceRect(
			frame.SourceSize.W,
			frame.SourceSize.H,
			frame.SpriteSourceSize.X,
			frame.SpriteSourceSize.Y,
		)
		frames[frame.Filename] = textureRegion
	}
	return &Atlas{Texture: texture, frames: frames}, nil
}

func (m *AtlasMetadata) Frame(name string) (AtlasFrame, bool) {
	if m == nil {
		return AtlasFrame{}, false
	}
	for _, frame := range m.Frames {
		if frame.Filename == name {
			return frame, true
		}
	}
	return AtlasFrame{}, false
}

type atlasEncoder struct{ data []byte }

func (e *atlasEncoder) uvarint(value uint64) {
	var buffer [binary.MaxVarintLen64]byte
	length := binary.PutUvarint(buffer[:], value)
	e.data = append(e.data, buffer[:length]...)
}

func (e *atlasEncoder) integer(value int) {
	var buffer [binary.MaxVarintLen64]byte
	length := binary.PutVarint(buffer[:], int64(value))
	e.data = append(e.data, buffer[:length]...)
}

func (e *atlasEncoder) string(value string) {
	e.uvarint(uint64(len(value)))
	e.data = append(e.data, value...)
}

func (e *atlasEncoder) bool(value bool) {
	if value {
		e.data = append(e.data, 1)
		return
	}
	e.data = append(e.data, 0)
}

type atlasDecoder struct{ data []byte }

func (d *atlasDecoder) take(size int) ([]byte, error) {
	if size < 0 || size > len(d.data) {
		return nil, errors.New("truncated data")
	}
	value := d.data[:size]
	d.data = d.data[size:]
	return value, nil
}

func (d *atlasDecoder) uvarint() (uint64, error) {
	value, length := binary.Uvarint(d.data)
	if length == 0 {
		return 0, errors.New("truncated varint")
	}
	if length < 0 {
		return 0, errors.New("varint overflow")
	}
	d.data = d.data[length:]
	return value, nil
}

func (d *atlasDecoder) integer() (int, error) {
	value, length := binary.Varint(d.data)
	if length == 0 {
		return 0, errors.New("truncated varint")
	}
	if length < 0 {
		return 0, errors.New("varint overflow")
	}
	d.data = d.data[length:]
	converted := int(value)
	if int64(converted) != value {
		return 0, errors.New("integer overflow")
	}
	return converted, nil
}

func (d *atlasDecoder) length() (int, error) {
	value, err := d.uvarint()
	if err != nil {
		return 0, err
	}
	length := int(value)
	if uint64(length) != value || length > len(d.data) {
		return 0, errors.New("length exceeds remaining data")
	}
	return length, nil
}

func (d *atlasDecoder) string() (string, error) {
	length, err := d.length()
	if err != nil {
		return "", err
	}
	value, err := d.take(length)
	return string(value), err
}

func (d *atlasDecoder) bool() (bool, error) {
	value, err := d.take(1)
	if err != nil {
		return false, err
	}
	if value[0] > 1 {
		return false, fmt.Errorf("invalid bool %d", value[0])
	}
	return value[0] == 1, nil
}
