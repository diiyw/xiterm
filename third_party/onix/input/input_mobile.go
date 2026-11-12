//go:build android || ios

package input

type KeyCode int

const (
	KeyA KeyCode = 4
	KeyB KeyCode = 5
	KeyC KeyCode = 6
	KeyD KeyCode = 7
	KeyE KeyCode = 8
	KeyF KeyCode = 9
	KeyG KeyCode = 10
	KeyH KeyCode = 11
	KeyI KeyCode = 12
	KeyJ KeyCode = 13
	KeyK KeyCode = 14
	KeyL KeyCode = 15
	KeyM KeyCode = 16
	KeyN KeyCode = 17
	KeyO KeyCode = 18
	KeyP KeyCode = 19
	KeyQ KeyCode = 20
	KeyR KeyCode = 21
	KeyS KeyCode = 22
	KeyT KeyCode = 23
	KeyU KeyCode = 24
	KeyV KeyCode = 25
	KeyW KeyCode = 26
	KeyX KeyCode = 27
	KeyY KeyCode = 28
	KeyZ KeyCode = 29

	Key0    KeyCode = 30
	Key1    KeyCode = 31
	Key2    KeyCode = 32
	Key3    KeyCode = 33
	Key4    KeyCode = 34
	Key5    KeyCode = 35
	Key6    KeyCode = 36
	Key7    KeyCode = 37
	Key8    KeyCode = 38
	Key9    KeyCode = 39
	KeyNum0         = Key0
	KeyNum1         = Key1
	KeyNum2         = Key2
	KeyNum3         = Key3
	KeyNum4         = Key4
	KeyNum5         = Key5
	KeyNum6         = Key6
	KeyNum7         = Key7
	KeyNum8         = Key8
	KeyNum9         = Key9

	KeyEnter  KeyCode = 40
	KeyEscape KeyCode = 41
	KeyDelete KeyCode = 42
	KeyTab    KeyCode = 43
	KeySpace  KeyCode = 44
	KeyMinus  KeyCode = 45

	KeyEqual        KeyCode = 46
	KeyLeftBracket  KeyCode = 47
	KeyRightBracket KeyCode = 48
	KeyBackslash    KeyCode = 49
	KeySemicolon    KeyCode = 51
	KeyApostrophe   KeyCode = 52
	KeyGraveAccent  KeyCode = 53
	KeyComma        KeyCode = 54
	KeyPeriod       KeyCode = 55
	KeySlash        KeyCode = 56

	KeyF1  KeyCode = 58
	KeyF2  KeyCode = 59
	KeyF3  KeyCode = 60
	KeyF4  KeyCode = 61
	KeyF5  KeyCode = 62
	KeyF6  KeyCode = 63
	KeyF7  KeyCode = 64
	KeyF8  KeyCode = 65
	KeyF9  KeyCode = 66
	KeyF10 KeyCode = 67
	KeyF11 KeyCode = 68
	KeyF12 KeyCode = 69

	KeyRight KeyCode = 79
	KeyLeft  KeyCode = 80
	KeyDown  KeyCode = 81
	KeyUp    KeyCode = 82

	KeyWorld1 KeyCode = -iota - 1
	KeyWorld2 KeyCode = -iota - 1
	KeyF13    KeyCode = -iota - 1
	KeyF14    KeyCode = -iota - 1
	KeyF15    KeyCode = -iota - 1
	KeyF16    KeyCode = -iota - 1
	KeyF17    KeyCode = -iota - 1
	KeyF18    KeyCode = -iota - 1
	KeyF19    KeyCode = -iota - 1
	KeyF20    KeyCode = -iota - 1
	KeyF21    KeyCode = -iota - 1
	KeyF22    KeyCode = -iota - 1
	KeyF23    KeyCode = -iota - 1
	KeyF24    KeyCode = -iota - 1
	KeyF25    KeyCode = -iota - 1

	KeyLeftShift   KeyCode = -iota - 1
	KeyRightShift  KeyCode = -iota - 1
	KeyLeftControl KeyCode = -iota - 1

	KeyLeftAlt KeyCode = -iota - 1

	KeyRightAlt KeyCode = -iota - 1

	KeyInsert       KeyCode = -iota - 1
	KeyPageUp       KeyCode = -iota - 1
	KeyPageDown     KeyCode = -iota - 1
	KeyHome         KeyCode = -iota - 1
	KeyRightControl KeyCode = -iota - 1

	KeyBackspace KeyCode = -iota - 1
	KeyEnd       KeyCode = -iota - 1
	KeyKP0       KeyCode = -iota - 1
	KeyKP1       KeyCode = -iota - 1
	KeyKP2       KeyCode = -iota - 1
	KeyKP3       KeyCode = -iota - 1
	KeyKP4       KeyCode = -iota - 1
	KeyKP5       KeyCode = -iota - 1
	KeyKP6       KeyCode = -iota - 1
	KeyKP7       KeyCode = -iota - 1
	KeyKP8       KeyCode = -iota - 1
	KeyKP9       KeyCode = -iota - 1

	KeyKPDivide   KeyCode = -iota - 1
	KeyKPMultiply KeyCode = -iota - 1
	KeyKPSubtract KeyCode = -iota - 1
	KeyKPAdd      KeyCode = -iota - 1
	KeyKPDecimal  KeyCode = -iota - 1
	KeyKPEqual    KeyCode = -iota - 1
	KeyKPEnter    KeyCode = -iota - 1

	KeyPrintScreen KeyCode = -iota - 1
	KeyNumLock     KeyCode = -iota - 1
	KeyCapsLock    KeyCode = -iota - 1
	KeyScrollLock  KeyCode = -iota - 1
	KeyPause       KeyCode = -iota - 1
	KeyLeftSuper   KeyCode = -iota - 1
	KeyRightSuper  KeyCode = -iota - 1
	KeyMenu        KeyCode = -iota - 1
)

type MouseButton int

const (
	MouseButton1 MouseButton = 0
	MouseButton2 MouseButton = 2 // Web MouseEvent has middle and right mouse buttons in reverse order.
	MouseButton3 MouseButton = 1 // Web MouseEvent has middle and right mouse buttons in reverse order.

	MouseNone         = MouseButton(-1)
	MouseButtonLeft   = MouseButton1
	MouseButtonRight  = MouseButton2
	MouseButtonMiddle = MouseButton3
	MouseLeft         = MouseButtonLeft
	MouseRight        = MouseButtonRight
	MouseMiddle       = MouseButtonMiddle
)
