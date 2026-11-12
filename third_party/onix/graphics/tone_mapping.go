package graphics

import "fmt"

// ToneMapping selects a display curve for exposed linear scene radiance.
// The zero value uses Reinhard. ACES uses the color transforms and RRT/ODT fit of ACESFilmicToneMapping.
type ToneMapping string

const (
	ToneMappingReinhard ToneMapping = "reinhard"
	ToneMappingACES     ToneMapping = "aces"
)

func (t ToneMapping) Validate() error {
	switch t {
	case "", ToneMappingReinhard, ToneMappingACES:
		return nil
	default:
		return fmt.Errorf("tone mapping must be reinhard or aces")
	}
}
