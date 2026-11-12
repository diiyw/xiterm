//go:build !onix_release

package scene

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

func (v Val) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.String())
}

func (v *Val) UnmarshalJSON(data []byte) error {
	if v == nil {
		return errors.New("ecs: cannot unmarshal Val into nil receiver")
	}
	data = bytes.TrimSpace(data)
	var source string
	if err := json.Unmarshal(data, &source); err != nil {
		return fmt.Errorf("ecs: Val must be a string: %w", err)
	}
	parsed, err := ParseVal(source)
	if err != nil {
		return err
	}
	*v = parsed
	return nil
}
