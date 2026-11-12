package graphics

import "github.com/onix-engine/onix/render"

var material2DShaderOps = newShaderResourceOps[render.Material2DShaderResource](
	shaderResourceMaterial2D,
	"Material2D",
	"graphics: current renderer does not support Material2D shaders",
	"graphics: Material2D shader is disposed",
)

// Material2DShader owns a user-defined sprite pipeline and recreates it when
// the installed render device changes.
type Material2DShader = shaderResource[render.Material2DShaderResource]

func NewMaterial2DShader(descriptor render.ShaderDescriptor) (*Material2DShader, error) {
	return newShaderResource(descriptor, &material2DShaderOps)
}
