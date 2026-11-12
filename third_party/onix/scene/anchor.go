package scene

type Anchor [2]float32

var (
	AnchorBottomLeft   = Anchor{-0.5, -0.5}
	AnchorBottomCenter = Anchor{0, -0.5}
	AnchorBottomRight  = Anchor{0.5, -0.5}
	AnchorCenterLeft   = Anchor{-0.5, 0}
	AnchorCenter       = Anchor{}
	AnchorCenterRight  = Anchor{0.5, 0}
	AnchorTopLeft      = Anchor{-0.5, 0.5}
	AnchorTopCenter    = Anchor{0, 0.5}
	AnchorTopRight     = Anchor{0.5, 0.5}
)

func (a Anchor) X() float32 {
	return a[0]
}

func (a Anchor) Y() float32 {
	return a[1]
}

// TopLeft returns the local offset from this anchor to the rectangle's corner.
// Screen coordinates grow down; world coordinates grow up.
func (a Anchor) TopLeft(width, height float32, screenSpace bool) (float32, float32) {
	x := (-0.5 - a.X()) * width
	if screenSpace {
		return x, (a.Y() - 0.5) * height
	}
	return x, (0.5 - a.Y()) * height
}
