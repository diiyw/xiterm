package text

import "github.com/onix-engine/onix/font"

func LoadFont(data []byte) (*Font, error) {
	fnt, err := font.Parse(data)
	if err != nil {
		return nil, err
	}
	return NewFont(fnt), nil
}
