package font

import (
	"fmt"
	"os"
)

func Load(path string) (*Face, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read font: %w", err)
	}
	face, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	face.attachBacking(&fontBacking{data: data})
	return face, nil
}
