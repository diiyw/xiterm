//go:build !onix_release

package graphics

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// PreprocessAtlas compiles TexturePacker JSON when data is atlas metadata.
func PreprocessAtlas(data []byte) (compiled []byte, atlas bool, err error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, false, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return nil, false, nil
	}
	if _, hasFrames := fields["frames"]; !hasFrames {
		return nil, false, nil
	}
	if _, hasMeta := fields["meta"]; !hasMeta {
		return nil, false, nil
	}
	metadata, err := parseAtlasSource(data)
	if err != nil {
		return nil, true, err
	}
	compiled, err = MarshalAtlas(metadata)
	return compiled, true, err
}

func parseAtlasSource(data []byte) (*AtlasMetadata, error) {
	var encoded struct {
		Frames json.RawMessage `json:"frames"`
		Meta   AtlasMeta       `json:"meta"`
	}
	if err := json.Unmarshal(data, &encoded); err != nil {
		return nil, fmt.Errorf("decode atlas metadata: %w", err)
	}
	frames, err := parseAtlasFrames(encoded.Frames)
	if err != nil {
		return nil, err
	}
	return &AtlasMetadata{Frames: frames, Meta: encoded.Meta}, nil
}

func parseAtlasFrames(data []byte) ([]AtlasFrame, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("decode atlas frames: %w", err)
	}

	var frames []AtlasFrame
	switch token {
	case json.Delim('['):
		for decoder.More() {
			var frame AtlasFrame
			if err := decoder.Decode(&frame); err != nil {
				return nil, fmt.Errorf("decode atlas frame: %w", err)
			}
			frames = append(frames, frame)
		}
	case json.Delim('{'):
		for decoder.More() {
			name, err := decoder.Token()
			if err != nil {
				return nil, fmt.Errorf("decode atlas frame name: %w", err)
			}
			filename, ok := name.(string)
			if !ok {
				return nil, fmt.Errorf("atlas frame name must be a string")
			}
			var frame AtlasFrame
			if err := decoder.Decode(&frame); err != nil {
				return nil, fmt.Errorf("decode atlas frame %q: %w", filename, err)
			}
			if frame.Filename == "" {
				frame.Filename = filename
			}
			frames = append(frames, frame)
		}
	default:
		return nil, fmt.Errorf("atlas frames must be an array or object")
	}
	return frames, nil
}
