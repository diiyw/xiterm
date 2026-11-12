package graphics

import (
	"math"

	"github.com/onix-engine/onix/render"
	"github.com/go-gl/mathgl/mgl32"
)

// GenerateTangents averages UV derivatives over shared vertices, preserving
// mirrored UV handedness. It runs at asset creation; degenerate UVs stay zero.
// indices refer to vertices starting at base in the original vertex buffer.
func GenerateTangents(vertices []render.Vertex3D, indices []uint32, base uint32) {
	bitangents := make([]mgl32.Vec3, len(vertices))
	for i := range vertices {
		vertices[i].Tangent = [4]float32{}
	}
	for offset := 0; offset+2 < len(indices); offset += 3 {
		a, b, c := vertices[indices[offset]-base], vertices[indices[offset+1]-base], vertices[indices[offset+2]-base]
		e1 := mgl32.Vec3(b.Position).Sub(mgl32.Vec3(a.Position))
		e2 := mgl32.Vec3(c.Position).Sub(mgl32.Vec3(a.Position))
		d1 := mgl32.Vec2(b.UV).Sub(mgl32.Vec2(a.UV))
		d2 := mgl32.Vec2(c.UV).Sub(mgl32.Vec2(a.UV))
		d := d1[0]*d2[1] - d2[0]*d1[1]
		if math.Abs(float64(d)) < 1e-8 {
			continue
		}
		t := e1.Mul(d2[1]).Sub(e2.Mul(d1[1])).Mul(1 / d)
		bitangent := e2.Mul(d1[0]).Sub(e1.Mul(d2[0])).Mul(1 / d)
		for _, index := range indices[offset : offset+3] {
			i := index - base
			for component := range 3 {
				vertices[i].Tangent[component] += t[component]
			}
			bitangents[i] = bitangents[i].Add(bitangent)
		}
	}
	for i := range vertices {
		n := mgl32.Vec3(vertices[i].Normal)
		t := mgl32.Vec3{vertices[i].Tangent[0], vertices[i].Tangent[1], vertices[i].Tangent[2]}
		t = t.Sub(n.Mul(n.Dot(t)))
		if length := t.Len(); length > 1e-8 {
			t = t.Mul(1 / length)
			w := float32(1)
			if n.Cross(t).Dot(bitangents[i]) < 0 {
				w = -1
			}
			vertices[i].Tangent = [4]float32{t[0], t[1], t[2], w}
		}
	}
}
