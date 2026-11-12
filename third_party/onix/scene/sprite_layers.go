package scene

import "github.com/go-gl/mathgl/mgl32"

// SpriteLayerTransform2D describes the local transform of a standalone
// sprite layer. It is intentionally independent of animation playback.
type SpriteLayerTransform2D struct {
	X, Y, Z                float32
	ScaleX, ScaleY, ScaleZ float32
	RotationX, RotationY   float32
	Rotation               float32
}

func (t SpriteLayerTransform2D) LocalMatrix() mgl32.Mat4 {
	scaleX, scaleY, scaleZ := t.ScaleX, t.ScaleY, t.ScaleZ
	if scaleX == 0 {
		scaleX = 1
	}
	if scaleY == 0 {
		scaleY = 1
	}
	if scaleZ == 0 {
		scaleZ = 1
	}
	return mgl32.Translate3D(t.X, t.Y, t.Z).
		Mul4(mgl32.HomogRotate3DZ(t.Rotation)).
		Mul4(mgl32.HomogRotate3DY(t.RotationY)).
		Mul4(mgl32.HomogRotate3DX(t.RotationX)).
		Mul4(mgl32.Scale3D(scaleX, scaleY, scaleZ))
}

type SpriteLayer2D struct {
	ID        string
	Sprite    Sprite2D
	Order     int
	Transform SpriteLayerTransform2D
	Anchor    Anchor
	Visible   bool
}

// SpriteLayers2D is a collection of independently rendered sprite or shape
// fragments sharing one entity's transform, clip and paint order.
type SpriteLayers2D struct {
	Layers []SpriteLayer2D

	matrices    []mgl32.Mat4
	matrixDirty []bool
}

func (a *SpriteLayers2D) LayerMatrix(index int) mgl32.Mat4 {
	if a == nil || index < 0 || index >= len(a.Layers) {
		return mgl32.Ident4()
	}
	if index < len(a.matrices) {
		if a.matrixDirty[index] {
			a.matrices[index] = a.Layers[index].Transform.LocalMatrix()
			a.matrixDirty[index] = false
		}
		return a.matrices[index]
	}
	return a.Layers[index].Transform.LocalMatrix()
}

func (a *SpriteLayers2D) InvalidateMatrices() {
	if a == nil {
		return
	}
	if cap(a.matrices) < len(a.Layers) {
		a.matrices = make([]mgl32.Mat4, len(a.Layers))
		a.matrixDirty = make([]bool, len(a.Layers))
	} else {
		a.matrices = a.matrices[:len(a.Layers)]
		a.matrixDirty = a.matrixDirty[:len(a.Layers)]
	}
	for index := range a.matrixDirty {
		a.matrixDirty[index] = true
	}
}
