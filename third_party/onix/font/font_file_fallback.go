//go:build android || ios || (!linux && !darwin && !windows)

package font

import "os"

func loadFontFile(path string) (*fontBacking, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &fontBacking{data: data}, nil
}
