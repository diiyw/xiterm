package scene

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-gl/mathgl/mgl32"
)

type ValKind uint8

const (
	ValAuto ValKind = iota
	ValPx
	ValPercent
	ValVw
	ValVh
	ValVMin
	ValVMax
)

type Val struct {
	Kind  ValKind
	Value float32
}

func Auto() Val                 { return Val{Kind: ValAuto} }
func Px(value float32) Val      { return Val{Kind: ValPx, Value: value} }
func Percent(value float32) Val { return Val{Kind: ValPercent, Value: value} }
func Vw(value float32) Val      { return Val{Kind: ValVw, Value: value} }
func Vh(value float32) Val      { return Val{Kind: ValVh, Value: value} }
func VMin(value float32) Val    { return Val{Kind: ValVMin, Value: value} }
func VMax(value float32) Val    { return Val{Kind: ValVMax, Value: value} }

type Val2 struct {
	X Val `json:"x,omitzero"`
	Y Val `json:"y,omitzero"`
}

func (v Val2) IsZero() bool {
	return v.X.IsZero() && v.Y.IsZero()
}

func Px2(x, y float32) Val2 {
	return Val2{X: Px(x), Y: Px(y)}
}

func Percent2(x, y float32) Val2 {
	return Val2{X: Percent(x), Y: Percent(y)}
}

type UIRect struct {
	Left   Val `json:"left,omitzero"`
	Right  Val `json:"right,omitzero"`
	Top    Val `json:"top,omitzero"`
	Bottom Val `json:"bottom,omitzero"`
}

func (r UIRect) IsZero() bool {
	return r.Left.IsZero() && r.Right.IsZero() && r.Top.IsZero() && r.Bottom.IsZero()
}

func UIRectAll(value Val) UIRect {
	return UIRect{Left: value, Right: value, Top: value, Bottom: value}
}

func UIRectPx(left, right, top, bottom float32) UIRect {
	return UIRect{Left: Px(left), Right: Px(right), Top: Px(top), Bottom: Px(bottom)}
}

func UIRectPercent(left, right, top, bottom float32) UIRect {
	return UIRect{Left: Percent(left), Right: Percent(right), Top: Percent(top), Bottom: Percent(bottom)}
}

type ResolvedUIRect struct {
	Left   float32
	Right  float32
	Top    float32
	Bottom float32
}

func (r ResolvedUIRect) Horizontal() float32 { return r.Left + r.Right }
func (r ResolvedUIRect) Vertical() float32   { return r.Top + r.Bottom }

type UIResolveContext struct {
	ScaleFactor float32
	BaseSize    mgl32.Vec2
	Viewport    mgl32.Vec2
}

func (v Val) Resolve(scaleFactor, baseValue float32, viewport mgl32.Vec2) (float32, bool) {
	if scaleFactor <= 0 {
		scaleFactor = 1
	}
	switch v.Kind {
	case ValPx:
		return v.Value * scaleFactor, true
	case ValPercent:
		return baseValue * v.Value / 100, true
	case ValVw:
		return viewport.X() * v.Value / 100, true
	case ValVh:
		return viewport.Y() * v.Value / 100, true
	case ValVMin:
		return min(viewport.X(), viewport.Y()) * v.Value / 100, true
	case ValVMax:
		return max(viewport.X(), viewport.Y()) * v.Value / 100, true
	default:
		return 0, false
	}
}

func ParseVal(value string) (Val, error) {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "auto") {
		return Auto(), nil
	}
	units := []struct {
		suffix string
		kind   ValKind
	}{{"vmin", ValVMin}, {"vmax", ValVMax}, {"px", ValPx}, {"vw", ValVw}, {"vh", ValVh}, {"%", ValPercent}}
	for _, unit := range units {
		if !strings.HasSuffix(strings.ToLower(value), unit.suffix) {
			continue
		}
		number := strings.TrimSpace(value[:len(value)-len(unit.suffix)])
		parsed, err := strconv.ParseFloat(number, 32)
		if err != nil {
			return Val{}, fmt.Errorf("ecs: parse Val %q: %w", value, err)
		}
		return Val{Kind: unit.kind, Value: float32(parsed)}, nil
	}
	return Val{}, fmt.Errorf("ecs: parse Val %q: missing or invalid unit", value)
}

func (v Val) String() string {
	if v.Kind == ValAuto {
		return "auto"
	}
	unit := "px"
	switch v.Kind {
	case ValPercent:
		unit = "%"
	case ValVw:
		unit = "vw"
	case ValVh:
		unit = "vh"
	case ValVMin:
		unit = "vmin"
	case ValVMax:
		unit = "vmax"
	}
	return strconv.FormatFloat(float64(v.Value), 'f', -1, 32) + unit
}

func (v Val) IsZero() bool {
	return v.Kind == ValAuto && v.Value == 0
}

func (v Val2) Resolve(context UIResolveContext) mgl32.Vec2 {
	x, _ := v.X.Resolve(context.ScaleFactor, context.BaseSize.X(), context.Viewport)
	y, _ := v.Y.Resolve(context.ScaleFactor, context.BaseSize.Y(), context.Viewport)
	return mgl32.Vec2{x, y}
}

var ErrIncompatibleValUnits = errors.New("ecs: incompatible Val units")

func (v Val) Add(other Val) (Val, error) {
	if v.Kind != other.Kind {
		return Val{}, ErrIncompatibleValUnits
	}
	return Val{Kind: v.Kind, Value: v.Value + other.Value}, nil
}

func (v Val) Sub(other Val) (Val, error) {
	if v.Kind != other.Kind {
		return Val{}, ErrIncompatibleValUnits
	}
	return Val{Kind: v.Kind, Value: v.Value - other.Value}, nil
}

func (v Val) Mul(factor float32) Val {
	return Val{Kind: v.Kind, Value: v.Value * factor}
}

type ViewportSize struct {
	Width       float32
	Height      float32
	ScaleFactor float32
}
