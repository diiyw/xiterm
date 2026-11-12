package scene

import (
	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/input"
)

type InputEventKind uint8

const (
	InputEventKeyDown InputEventKind = iota + 1
	InputEventKeyUp
	InputEventTextInput
	InputEventTextComposition
	InputEventTextCommand
	InputEventTextSelection
	InputEventFilesDropped
	InputEventEnter
	InputEventLeave
	InputEventPress
	InputEventMove
	InputEventRelease
	InputEventTap
	InputEventWheel
	InputEventCancel
)

type InputMask uint8

const (
	InputMaskPointer InputMask = 1 << iota
	InputMaskKeyboard
	InputMaskText
	InputMaskFiles
	InputMaskAll = InputMaskPointer | InputMaskKeyboard | InputMaskText | InputMaskFiles
)

func (mask InputMask) Intersects(other InputMask) bool {
	return mask&other != 0
}

func (kind InputEventKind) Mask() InputMask {
	switch kind {
	case InputEventKeyDown, InputEventKeyUp:
		return InputMaskKeyboard
	case InputEventTextInput, InputEventTextComposition, InputEventTextCommand, InputEventTextSelection:
		return InputMaskText
	case InputEventFilesDropped:
		return InputMaskFiles
	case InputEventEnter, InputEventLeave, InputEventPress, InputEventMove,
		InputEventRelease, InputEventTap, InputEventWheel, InputEventCancel:
		return InputMaskPointer
	default:
		return 0
	}
}

func (kind InputEventKind) IsPointer() bool {
	return kind.Mask() == InputMaskPointer
}

func (kind InputEventKind) String() string {
	switch kind {
	case InputEventKeyDown:
		return "keydown"
	case InputEventKeyUp:
		return "keyup"
	case InputEventTextInput:
		return "textinput"
	case InputEventTextComposition:
		return "textcomposition"
	case InputEventTextCommand:
		return "textcommand"
	case InputEventTextSelection:
		return "textselection"
	case InputEventFilesDropped:
		return "filesdropped"
	case InputEventEnter:
		return "enter"
	case InputEventLeave:
		return "leave"
	case InputEventPress:
		return "press"
	case InputEventMove:
		return "move"
	case InputEventRelease:
		return "release"
	case InputEventTap:
		return "tap"
	case InputEventWheel:
		return "wheel"
	case InputEventCancel:
		return "cancel"
	default:
		return "unknown"
	}
}

// InputEvent is the unified ECS input delivered to scripts and systems.
// Entity and Path are populated for hit-tested pointer input and remain zero
// for global keyboard, text, and file input.
type InputEvent struct {
	File string
	Text string
	// Path contains the receiver followed by its parents for bubbling.
	Path        []ecs.Entity
	Composition input.TextComposition
	Selection   input.TextSelection
	Seq         uint64
	Entity      ecs.Entity
	Key         input.KeyCode
	X           float32
	Y           float32
	DeltaX      float32
	DeltaY      float32
	WheelX      float32
	WheelY      float32
	// Handled is set when a synchronous handler consumes the event.
	Handled bool
	Kind    InputEventKind
	Device  input.PointerDevice
	Button  input.MouseButton
	Command input.TextCommand
}

// InputEventFromPlatform converts non-pointer platform input into the unified
// ECS representation. Pointer input is emitted after 2D hit testing instead.
func InputEventFromPlatform(event input.Event) (InputEvent, bool) {
	result := InputEvent{Seq: event.Seq}
	switch event.Kind {
	case input.EventKeyDown:
		result.Kind, result.Key = InputEventKeyDown, event.Key
	case input.EventKeyUp:
		result.Kind, result.Key = InputEventKeyUp, event.Key
	case input.EventTextInput:
		result.Kind, result.Text = InputEventTextInput, event.Text
	case input.EventTextComposition:
		result.Kind, result.Composition = InputEventTextComposition, event.Composition
	case input.EventTextCommand:
		result.Kind, result.Command = InputEventTextCommand, event.Command
	case input.EventTextSelection:
		result.Kind, result.Selection = InputEventTextSelection, event.Selection
	case input.EventFilesDropped:
		result.Kind, result.File = InputEventFilesDropped, event.File
		result.X, result.Y = float32(event.X), float32(event.Y)
	default:
		return InputEvent{}, false
	}
	return result, true
}

type InputEventContext struct {
	Event   *InputEvent
	Target  ecs.Entity
	Current ecs.Entity
	Handled bool
}

func (c *InputEventContext) Consume() {
	if c != nil {
		c.Handled = true
	}
}
