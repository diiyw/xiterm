//go:build onix_release

package graphics

import (
	"errors"
)

func parseAtlasSource([]byte) (*AtlasMetadata, error) {
	return nil, errors.New("decode atlas metadata: release builds require preprocessed atlas IR")
}

func PreprocessAtlas([]byte) ([]byte, bool, error) {
	return nil, false, errors.New("atlas preprocessing is unavailable in release builds")
}
