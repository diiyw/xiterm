package render

import (
	"fmt"
	"math"
	"slices"
)

const meshDeformationHeaderWords = 8

// Static uploads borrow at most 4 MiB per pooled buffer. Native backends copy
// the payload before returning, so consecutive meshes can share this storage.
var meshDeformationBuffers = bufferPool[uint32]{maxCapacity: (4 << 20) / 4}

type meshDeformationLayout struct {
	skinOffset, morphOffset, vertexCount, targetCount, requiredJoints uint32
}

type meshDeformationSource interface {
	deformationLayout(MeshHandle) (meshDeformationLayout, bool)
}

func packMeshDeformationStatic(dst []uint32, value MeshDeformationDescriptor) ([]uint32, meshDeformationLayout, error) {
	layout := meshDeformationLayout{vertexCount: value.VertexCount, targetCount: value.TargetCount}
	if value.VertexCount == 0 || len(value.Skin) != 0 && uint64(len(value.Skin)) != uint64(value.VertexCount) || uint64(len(value.MorphTargets)) != uint64(value.VertexCount)*uint64(value.TargetCount) {
		return dst[:0], layout, fmt.Errorf("invalid static deformation dimensions")
	}
	// Offset zero means absent; reserve a sentinel even for an empty stream.
	words := slices.Grow(dst[:0], 1+len(value.Skin)*8+len(value.MorphTargets)*9)[:1]
	words[0] = 0
	if len(value.Skin) != 0 {
		layout.skinOffset = uint32(len(words))
		for _, vertex := range value.Skin {
			words = append(words, vertex.Joints[:]...)
			for influence, weight := range vertex.Weights {
				if !finiteDeformationValue(weight) || weight < 0 || weight > 1 || weight > 0 && vertex.Joints[influence] == math.MaxUint32 {
					return words[:0], layout, fmt.Errorf("invalid static skin influence")
				}
				if weight > 0 {
					layout.requiredJoints = max(layout.requiredJoints, vertex.Joints[influence]+1)
				}
				words = append(words, math.Float32bits(weight))
			}
		}
	}
	if len(value.MorphTargets) != 0 {
		layout.morphOffset = uint32(len(words))
		for _, vertex := range value.MorphTargets {
			for _, vector := range [3][3]float32{vertex.Position, vertex.Normal, vertex.Tangent} {
				for _, component := range vector {
					if !finiteDeformationValue(component) {
						return words[:0], layout, fmt.Errorf("static morph delta is not finite")
					}
					words = append(words, math.Float32bits(component))
				}
			}
		}
	}
	return words, layout, nil
}

func packMeshDeformations[S meshDeformationSource](dst []uint32, batch MeshBatch3D, source S) ([]uint32, error) {
	dst = dst[:0]
	headerWords := len(batch.Deformations) * meshDeformationHeaderWords
	for index, value := range batch.Deformations {
		layout, ok := source.deformationLayout(value.Mesh)
		if !ok || uint64(len(value.JointMatrices)) < uint64(layout.requiredJoints) || uint64(len(value.MorphWeights)) > uint64(layout.targetCount) {
			return dst[:0], fmt.Errorf("deformation %d mesh or animated parameters are invalid", index)
		}
		// Reserve the headers with the first payload, avoiding a separate cold
		// allocation. Subsequent payloads grow once, before writing components.
		payloadWords := len(value.JointMatrices)*16 + int(layout.targetCount)
		if index == 0 {
			dst = slices.Grow(dst, headerWords+payloadWords)[:headerWords]
		} else {
			dst = slices.Grow(dst, payloadWords)
		}
		jointOffset := uint32(0)
		if len(value.JointMatrices) != 0 {
			jointOffset = uint32(len(dst))
			for _, matrix := range value.JointMatrices {
				for _, component := range matrix {
					if !finiteDeformationValue(component) {
						return dst[:0], fmt.Errorf("deformation %d joint matrix is not finite", index)
					}
					dst = append(dst, math.Float32bits(component))
				}
			}
		}
		weightOffset := uint32(0)
		if layout.targetCount != 0 {
			weightOffset = uint32(len(dst))
			for weightIndex := uint32(0); weightIndex < layout.targetCount; weightIndex++ {
				weight := float32(0)
				if weightIndex < uint32(len(value.MorphWeights)) {
					weight = value.MorphWeights[weightIndex]
				}
				if !finiteDeformationValue(weight) {
					return dst[:0], fmt.Errorf("deformation %d morph weight is not finite", index)
				}
				dst = append(dst, math.Float32bits(weight))
			}
		}
		header := [...]uint32{layout.skinOffset, layout.morphOffset, jointOffset, weightOffset, layout.vertexCount, layout.targetCount, uint32(len(value.JointMatrices)), uint32(len(value.MorphWeights))}
		copy(dst[index*meshDeformationHeaderWords:], header[:])
	}
	for drawIndex, draw := range batch.Draws {
		if draw.DeformationIndex > uint32(len(batch.Deformations)) || draw.DeformationIndex != 0 && draw.Mesh != batch.Deformations[draw.DeformationIndex-1].Mesh {
			return dst[:0], fmt.Errorf("draw %d deformation does not match its mesh", drawIndex)
		}
	}
	return dst, nil
}

func finiteDeformationValue(value float32) bool {
	return math.Float32bits(value)&0x7f800000 != 0x7f800000
}
