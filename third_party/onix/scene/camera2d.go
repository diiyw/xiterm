package scene

import "github.com/go-gl/mathgl/mgl32"

type ScalingModeKind uint8

const (
	ScalingModeWindowSize ScalingModeKind = iota
	ScalingModeFixed
	ScalingModeAutoMin
	ScalingModeAutoMax
	ScalingModeFixedVertical
	ScalingModeFixedHorizontal
)

type ScalingMode struct {
	Kind   ScalingModeKind
	Width  float32
	Height float32
}

func FixedScaling(width, height float32) ScalingMode {
	return ScalingMode{Kind: ScalingModeFixed, Width: width, Height: height}
}

func AutoMinScaling(minWidth, minHeight float32) ScalingMode {
	return ScalingMode{Kind: ScalingModeAutoMin, Width: minWidth, Height: minHeight}
}

func AutoMaxScaling(maxWidth, maxHeight float32) ScalingMode {
	return ScalingMode{Kind: ScalingModeAutoMax, Width: maxWidth, Height: maxHeight}
}

func FixedVerticalScaling(viewportHeight float32) ScalingMode {
	return ScalingMode{Kind: ScalingModeFixedVertical, Height: viewportHeight}
}

func FixedHorizontalScaling(viewportWidth float32) ScalingMode {
	return ScalingMode{Kind: ScalingModeFixedHorizontal, Width: viewportWidth}
}

type OrthographicArea struct {
	MinX float32
	MinY float32
	MaxX float32
	MaxY float32
}

type OrthographicProjection struct {
	// ReverseZ uses a [0,1] depth range, with positive Near/Far distances,
	// for 3D cameras. The default matrix retains the 2D [-1,1] depth range.
	ReverseZ       bool
	Near           float32
	Far            float32
	ViewportOrigin mgl32.Vec2
	ScalingMode    ScalingMode
	Scale          float32
	Area           OrthographicArea
}

func NewOrthographicProjection2D() OrthographicProjection {
	return OrthographicProjection{
		Near:           -1000,
		Far:            1000,
		ViewportOrigin: mgl32.Vec2{0.5, 0.5},
		Scale:          1,
		Area:           OrthographicArea{MinX: -1, MinY: -1, MaxX: 1, MaxY: 1},
	}
}

func (p *OrthographicProjection) Update(width, height float32) {
	if p == nil || width <= 0 || height <= 0 {
		return
	}
	projectionWidth, projectionHeight := width, height
	switch p.ScalingMode.Kind {
	case ScalingModeFixed:
		projectionWidth, projectionHeight = p.ScalingMode.Width, p.ScalingMode.Height
	case ScalingModeAutoMin:
		if width*p.ScalingMode.Height > p.ScalingMode.Width*height {
			projectionWidth = width * p.ScalingMode.Height / height
			projectionHeight = p.ScalingMode.Height
		} else {
			projectionWidth = p.ScalingMode.Width
			projectionHeight = height * p.ScalingMode.Width / width
		}
	case ScalingModeAutoMax:
		if width*p.ScalingMode.Height < p.ScalingMode.Width*height {
			projectionWidth = width * p.ScalingMode.Height / height
			projectionHeight = p.ScalingMode.Height
		} else {
			projectionWidth = p.ScalingMode.Width
			projectionHeight = height * p.ScalingMode.Width / width
		}
	case ScalingModeFixedVertical:
		projectionWidth = width * p.ScalingMode.Height / height
		projectionHeight = p.ScalingMode.Height
	case ScalingModeFixedHorizontal:
		projectionWidth = p.ScalingMode.Width
		projectionHeight = height * p.ScalingMode.Width / width
	}

	originX := projectionWidth * p.ViewportOrigin.X()
	originY := projectionHeight * p.ViewportOrigin.Y()
	p.Area = OrthographicArea{
		MinX: p.Scale * -originX,
		MinY: p.Scale * -originY,
		MaxX: p.Scale * (projectionWidth - originX),
		MaxY: p.Scale * (projectionHeight - originY),
	}
}

func (p OrthographicProjection) Matrix() mgl32.Mat4 {
	if p.ReverseZ {
		width, height, depth := p.Area.MaxX-p.Area.MinX, p.Area.MaxY-p.Area.MinY, p.Far-p.Near
		return mgl32.Mat4{
			2 / width, 0, 0, 0,
			0, 2 / height, 0, 0,
			0, 0, 1 / depth, 0,
			-(p.Area.MaxX + p.Area.MinX) / width, -(p.Area.MaxY + p.Area.MinY) / height, p.Far / depth, 1,
		}
	}
	return mgl32.Ortho(p.Area.MinX, p.Area.MaxX, p.Area.MinY, p.Area.MaxY, p.Near, p.Far)
}

type ProjectionKind uint8

const (
	ProjectionOrthographic ProjectionKind = iota
	ProjectionPerspective
)

type Projection struct {
	Kind         ProjectionKind
	Orthographic OrthographicProjection
	Perspective  PerspectiveProjection
}

func OrthographicProjection2D() Projection {
	return Projection{Kind: ProjectionOrthographic, Orthographic: NewOrthographicProjection2D()}
}

func PerspectiveProjection3D() Projection {
	return Projection{Kind: ProjectionPerspective, Perspective: NewPerspectiveProjection()}
}

func (p Projection) Matrix(width, height float32) mgl32.Mat4 {
	if width <= 0 || height <= 0 {
		return mgl32.Ident4()
	}
	if p.Kind == ProjectionPerspective {
		return p.Perspective.Matrix(width / height)
	}
	projection := p.Orthographic
	projection.Update(width, height)
	return projection.Matrix()
}
