package graphics

import "github.com/onix-engine/onix/render"

type AlphaMode uint8

const (
	AlphaOpaque AlphaMode = iota
	AlphaMask
	AlphaBlend
)

// StandardMaterial uses metallic/roughness PBR. Emissive is linear RGB in nits
// before EmissiveStrength. Construct it with NewStandardMaterial for defaults.
type StandardMaterial struct {
	MetallicRoughnessTexture *Texture
	Shader                   *MeshShader
	BaseColorTexture         *Texture
	NormalTexture            *Texture
	Surface                  MaterialSurface
	Emissive                 ColorRGBA
	BaseColor                ColorRGBA
	EmissiveStrength         float32 // unitless multiplier, defaults to 1
	Metallic                 float32
	PerceptualRoughness      float32
	IOR                      float32 // index of refraction; 0 means positive infinity
	AlphaCutoff              float32
	AlphaMode                AlphaMode
}

// Material3DData is the fixed GPU-facing material contract shared by built-in
// and user-defined materials. Custom values map to shader inputs 11 through 14.
type Material3DData struct {
	Shader                   *MeshShader
	BaseColorTexture         *Texture
	NormalTexture            *Texture
	MetallicRoughnessTexture *Texture
	Surface                  MaterialSurface
	Custom                   [4][4]float32
	Emissive                 ColorRGBA
	BaseColor                ColorRGBA
	EmissiveStrength         float32
	Metallic                 float32
	PerceptualRoughness      float32
	IOR                      float32
	AlphaCutoff              float32
	AlphaMode                AlphaMode
}

type Material3D interface {
	Material3D() Material3DData
}

// Material2DData is the per-instance data exposed to a custom 2D material.
type Material2DData struct {
	Shader *Material2DShader
	Custom [4][4]float32
}

type Material2D interface {
	Material2D() Material2DData
}

func NewStandardMaterial(color ColorRGBA) *StandardMaterial {
	return &StandardMaterial{
		Surface:             NewMaterialSurface(),
		BaseColor:           color,
		Emissive:            ColorRGBA{},
		EmissiveStrength:    1,
		PerceptualRoughness: 0.5,
		IOR:                 1.5,
		AlphaCutoff:         0.5,
	}
}

func (m StandardMaterial) Material3D() Material3DData {
	return Material3DData{
		Surface:                  m.Surface,
		Shader:                   m.Shader,
		BaseColor:                m.BaseColor,
		BaseColorTexture:         m.BaseColorTexture,
		NormalTexture:            m.NormalTexture,
		MetallicRoughnessTexture: m.MetallicRoughnessTexture,
		Emissive:                 m.Emissive,
		EmissiveStrength:         m.EmissiveStrength,
		Metallic:                 m.Metallic,
		PerceptualRoughness:      m.PerceptualRoughness,
		IOR:                      m.IOR,
		AlphaMode:                m.AlphaMode,
		AlphaCutoff:              m.AlphaCutoff,
	}
}

var _ Material3D = StandardMaterial{}

// SetEmission sets the emissive color using HDRP luminance units. A white
// color with intensity 100 nits emits linear RGB (100,100,100) before exposure.
// It resets EmissiveStrength to 1 so the requested intensity is exact.
// This does not turn a mesh into a light source for other objects.
func (m *StandardMaterial) SetEmission(color ColorRGBA, intensity float32, unit LightUnit) error {
	if err := ValidateLightIntensity(intensity, unit, LightNits, LightEV100); err != nil {
		return err
	}
	nits := AreaNits(intensity, unit, 1)
	m.Emissive = ColorRGBA{R: color.R * nits, G: color.G * nits, B: color.B * nits, A: color.A}
	m.EmissiveStrength = 1
	return nil
}

// MaterialSurface contains glTF surface properties. Color factors are linear;
// color textures use sRGB and data textures use UNORM.
type MaterialSurface struct {
	ClearcoatTexture          *Texture
	ClearcoatRoughnessTexture *Texture
	AnisotropyTexture         *Texture
	SheenRoughnessTexture     *Texture
	OcclusionTexture          *Texture
	EmissiveTexture           *Texture
	SpecularTexture           *Texture
	SheenColorTexture         *Texture
	ClearcoatNormalTexture    *Texture
	SpecularColorTexture      *Texture
	SheenColor                ColorRGBA
	SpecularColor             ColorRGBA
	AnisotropyStrength        float32
	NormalScale               float32
	AnisotropyRotation        float32 // radians in tangent space
	ClearcoatRoughness        float32
	OcclusionStrength         float32
	SheenRoughness            float32
	ClearcoatFactor           float32
	SpecularFactor            float32
	ClearcoatNormalScale      float32
	Unlit                     bool
	DoubleSided               bool
}

func NewMaterialSurface() MaterialSurface {
	return MaterialSurface{NormalScale: 1, OcclusionStrength: 1, SpecularFactor: 1, SpecularColor: White, ClearcoatNormalScale: 1}
}

func (s MaterialSurface) Textures() [render.MeshSurfaceTextureSlots]*Texture {
	return [render.MeshSurfaceTextureSlots]*Texture{s.OcclusionTexture, s.EmissiveTexture, s.SpecularTexture, s.SpecularColorTexture, s.ClearcoatTexture, s.ClearcoatRoughnessTexture, s.ClearcoatNormalTexture, s.SheenColorTexture, s.SheenRoughnessTexture, s.AnisotropyTexture}
}
