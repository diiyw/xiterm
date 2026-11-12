package scene

import "github.com/onix-engine/onix/graphics"

// Shader attaches a shared GPU program and per-element uniforms to geometry.
// T is graphics.Material2DShader for sprites or graphics.MeshShader for meshes.
type Shader[T graphics.Material2DShader | graphics.MeshShader] struct {
	Program *T
	Custom  [4][4]float32
}
