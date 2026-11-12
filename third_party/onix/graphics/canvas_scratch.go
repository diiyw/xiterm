package graphics

import "reflect"

type scratchValues map[reflect.Type]any

// ScratchValue returns typed storage retained for the lifetime of the canvas.
func (c *Canvas2D) ScratchValue[Tag, T any]() *T {
	if c == nil {
		return nil
	}
	if c.scratch == nil {
		c.scratch = make(scratchValues)
	}
	key := reflect.TypeFor[struct {
		Tag Tag
		Val T
	}]()
	if value, ok := c.scratch[key]; ok {
		return value.(*T)
	}
	value := new(T)
	c.scratch[key] = value
	return value
}
