package graphics

import "github.com/onix-engine/onix/render"

var meshShaderOps = newShaderResourceOps[render.MeshShaderResource](
	shaderResourceMesh,
	"mesh",
	"graphics: current renderer does not support custom mesh shaders",
	"graphics: mesh shader is disposed",
)

// MeshShader owns a user-defined 3D shader and recreates its native pipeline
// when the installed render device changes.
type MeshShader = shaderResource[render.MeshShaderResource]

func NewMeshShader(descriptor render.ShaderDescriptor) (*MeshShader, error) {
	return newShaderResource(descriptor, &meshShaderOps)
}
