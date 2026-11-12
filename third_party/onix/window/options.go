package window

type DecorationMode string

const (
	DecorationSystem DecorationMode = "system"
	DecorationNone   DecorationMode = "none"
)

type Options struct {
	DRMDevice           string
	Decorations         DecorationMode
	RenderScale         float64
	MaxDevicePixelRatio float64
	TitleBarHeight      float64
	// CornerRadius controls renderer-owned window corners in logical pixels.
	// Windows uses alpha composition for radii larger than its native 8 DIP.
	CornerRadius   int
	DRMConnectorID uint32
	Antialias      bool
	Transparent    bool
	Border         bool
	Shadow         bool
	Resizable      bool
}

const DefaultCornerRadius = 10

func DefaultOptions() Options {
	return Options{
		RenderScale:         1,
		MaxDevicePixelRatio: 2,
		Decorations:         DecorationSystem,
		CornerRadius:        DefaultCornerRadius,
		Border:              true,
		Shadow:              true,
		Resizable:           true,
	}
}

func (o Options) normalized() Options {
	if o.RenderScale <= 0 {
		o.RenderScale = 1
	}
	if o.MaxDevicePixelRatio <= 0 {
		o.MaxDevicePixelRatio = 2
	}
	if o.Decorations == "" {
		o.Decorations = DecorationSystem
	}
	if o.TitleBarHeight < 0 {
		o.TitleBarHeight = 0
	}
	if o.CornerRadius <= 0 {
		o.CornerRadius = DefaultCornerRadius
	}
	return o
}

func (m DecorationMode) Valid() bool {
	return m == "" || m == DecorationSystem || m == DecorationNone
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
