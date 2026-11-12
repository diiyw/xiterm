//go:build onix_release

package sourcejson

import "errors"

const Enabled = false

func Unmarshal([]byte, any) error {
	return errors.New("source JSON is unavailable in release builds")
}
