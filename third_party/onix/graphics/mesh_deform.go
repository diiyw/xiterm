package graphics

import (
	"fmt"
	"math"

	"github.com/onix-engine/onix/render"
	"github.com/go-gl/mathgl/mgl32"
)

// DeformVertices applies morph targets and linear-blend skinning into dst.
// Reusing dst with enough capacity makes subsequent calls allocation-free.
func (m *Mesh) DeformVertices(dst []render.Vertex3D, morphWeights []float32, jointMatrices []mgl32.Mat4) ([]render.Vertex3D, error) {
	if m == nil || m.disposed {
		return nil, fmt.Errorf("graphics: mesh is unavailable")
	}
	if len(morphWeights) > len(m.MorphTargets) {
		return nil, fmt.Errorf("graphics: %d morph weights exceed %d targets", len(morphWeights), len(m.MorphTargets))
	}
	if len(m.Skin) != 0 && len(m.Skin) != len(m.Vertices) {
		return nil, fmt.Errorf("graphics: skin vertex count does not match mesh")
	}
	if cap(dst) < len(m.Vertices) {
		dst = make([]render.Vertex3D, len(m.Vertices))
	} else {
		dst = dst[:len(m.Vertices)]
	}
	copy(dst, m.Vertices)
	for targetIndex, weight := range morphWeights {
		if weight == 0 {
			continue
		}
		if math.IsNaN(float64(weight)) || math.IsInf(float64(weight), 0) {
			return nil, fmt.Errorf("graphics: morph weight %d is not finite", targetIndex)
		}
		target := m.MorphTargets[targetIndex]
		if len(target.PositionDelta) != len(dst) || len(target.NormalDelta) != 0 && len(target.NormalDelta) != len(dst) || len(target.TangentDelta) != 0 && len(target.TangentDelta) != len(dst) {
			return nil, fmt.Errorf("graphics: morph target %d does not match mesh vertices", targetIndex)
		}
		for vertex := range dst {
			for component := range 3 {
				if len(target.TangentDelta) != 0 {
					dst[vertex].Tangent[component] += target.TangentDelta[vertex][component] * weight
				}
				dst[vertex].Position[component] += target.PositionDelta[vertex][component] * weight
				if len(target.NormalDelta) != 0 {
					dst[vertex].Normal[component] += target.NormalDelta[vertex][component] * weight
				}
			}
		}
	}
	for vertex, skin := range m.Skin {
		if !skin.Active {
			continue
		}
		position := mgl32.Vec4{dst[vertex].Position[0], dst[vertex].Position[1], dst[vertex].Position[2], 1}
		normal := mgl32.Vec4{dst[vertex].Normal[0], dst[vertex].Normal[1], dst[vertex].Normal[2], 0}
		tangent := mgl32.Vec4{dst[vertex].Tangent[0], dst[vertex].Tangent[1], dst[vertex].Tangent[2], 0}
		var skinMatrix mgl32.Mat4
		for influence, weight := range skin.Weights {
			if weight == 0 {
				continue
			}
			joint := skin.Joints[influence]
			if uint64(joint) >= uint64(len(jointMatrices)) {
				return nil, fmt.Errorf("graphics: skin joint %d is out of range", joint)
			}
			skinMatrix = skinMatrix.Add(jointMatrices[joint].Mul(weight))
		}
		skinnedPosition := skinMatrix.Mul4x1(position)
		skinnedTangent := skinMatrix.Mul4x1(tangent)
		x, y, z := skinMatrix.Col(0).Vec3(), skinMatrix.Col(1).Vec3(), skinMatrix.Col(2).Vec3()
		cofactor := mgl32.Mat3FromCols(y.Cross(z), z.Cross(x), x.Cross(y))
		flip := float32(1)
		if x.Dot(cofactor.Col(0)) < 0 {
			flip = -1
		}
		dst[vertex].Tangent[3] *= flip
		dst[vertex].Position = [3]float32{skinnedPosition[0], skinnedPosition[1], skinnedPosition[2]}
		value := cofactor.Mul3x1(normal.Vec3()).Mul(flip)
		if length := value.Len(); length > 1e-8 {
			value = value.Mul(1 / length)
		}
		dst[vertex].Normal = [3]float32(value)
		dst[vertex].Tangent[0], dst[vertex].Tangent[1], dst[vertex].Tangent[2] = skinnedTangent[0], skinnedTangent[1], skinnedTangent[2]
	}
	return dst, nil
}
