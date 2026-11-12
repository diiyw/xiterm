package scene

import (
	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/input"
)

// InputFocus is the currently focused ECS entity for keyboard/text input.
// Zero means that no entity owns keyboard focus.
type InputFocus struct {
	Keyboard ecs.Entity
}

// InputCapture2D describes the active 2D input capture. A capture keeps
// receiving move/release/cancel events after the input leaves its bounds.
type InputCapture2D struct {
	Target   ecs.Entity
	Receiver ecs.Entity
	Camera   ecs.Entity
}

// FocusedInputEvent routes one keyboard or text event to the focused entity.
type FocusedInputEvent struct {
	Event   input.Event
	Entity  ecs.Entity
	Handled bool
}

type FocusedInputContext struct {
	World   *ecs.World
	Event   input.Event
	Entity  ecs.Entity
	Handled bool
}

func (c *FocusedInputContext) Consume() {
	if c != nil {
		c.Handled = true
	}
}

// FocusedInputHandler handles keyboard/text input synchronously on the focused entity.
type FocusedInputHandler struct {
	Handle func(*FocusedInputContext)
}
