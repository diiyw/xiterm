//go:build !onix_release

package sourcejson

import "encoding/json"

const Enabled = true

func Unmarshal(data []byte, target any) error {
	return json.Unmarshal(data, target)
}

func (m *RawMessage) UnmarshalJSON(data []byte) error {
	*m = append((*m)[:0], data...)
	return nil
}
