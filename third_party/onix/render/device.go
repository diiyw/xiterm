package render

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"github.com/go-gl/mathgl/mgl32"
)

type Color struct {
	R float32
	G float32
	B float32
	A float32
}

type Rect struct {
	X      int32
	Y      int32
	Width  int32
	Height int32
}

func (r Rect) Empty() bool {
	return r.Width <= 0 || r.Height <= 0
}

type BlendFactor uint8

const (
	BlendZero BlendFactor = iota
	BlendOne
	BlendSrcColor
	BlendOneMinusSrcColor
	BlendSrcAlpha
	BlendOneMinusSrcAlpha
	BlendDstAlpha
	BlendOneMinusDstAlpha
	BlendDstColor
	BlendOneMinusDstColor
	BlendSrcAlphaSaturate
	BlendConstantColor
	BlendOneMinusConstantColor
	BlendConstantAlpha
	BlendOneMinusConstantAlpha
)

type Blend struct {
	Src BlendFactor
	Dst BlendFactor
}

type BlendState struct {
	RGB      Blend
	Alpha    Blend
	Constant Color
}

var PremultipliedAlphaBlend = BlendState{
	RGB:   Blend{Src: BlendOne, Dst: BlendOneMinusSrcAlpha},
	Alpha: Blend{Src: BlendOne, Dst: BlendOneMinusSrcAlpha},
}

// LCDTextBlend combines RGB coverage with Constant foreground color in one
// pass: coverage*foreground + destination*(1-coverage). Surface alpha is kept.
var LCDTextBlend = BlendState{
	RGB:   Blend{Src: BlendConstantColor, Dst: BlendOneMinusSrcColor},
	Alpha: Blend{Src: BlendZero, Dst: BlendOne},
}

func spriteBlendMode(blend BlendState) (int, bool) {
	blend.Constant = Color{}
	switch blend {
	case PremultipliedAlphaBlend:
		return 0, true
	case LCDTextBlend:
		return 1, true
	default:
		return 0, false
	}
}

type TextureFilter uint8

const (
	TextureFilterLinear TextureFilter = iota
	TextureFilterLinearMipmapLinear
)

// TextureAddressMode controls sampling outside normalized texture coordinates.
type TextureAddressMode uint8

const (
	TextureAddressClamp TextureAddressMode = iota
	TextureAddressRepeat
	TextureAddressMirror
)

func (m TextureAddressMode) Valid() bool { return m <= TextureAddressMirror }
func (m TextureAddressMode) webGPUName() string {
	return [...]string{"clamp-to-edge", "repeat", "mirror-repeat"}[m]
}
func (m TextureAddressMode) glValue() int { return [...]int{0x812F, 0x2901, 0x8370}[m] }

type TextureDescriptor struct {
	Width  int
	Height int
	// Levels is the number of mip levels described by the pixel payload.
	// Zero and one both mean a single level. Compressed formats ship a
	// prebuilt chain because the GPU cannot generate mips for block data.
	Levels       int
	WrapU, WrapV TextureAddressMode
	Filter       TextureFilter
	Format       TextureFormat
}

// MipLevels reports the mip depth described by the descriptor.
func (d TextureDescriptor) MipLevels() int {
	return max(d.Levels, 1)
}

// TextureLevel locates one mip level inside the payload a descriptor describes.
type TextureLevel struct {
	Width  int
	Height int
	// Offset is the byte offset of the level inside the payload.
	Offset int
	// Bytes is the encoded size of the level. Block-compressed levels round
	// their dimensions up to whole blocks, so a 1x1 level still occupies one
	// full block.
	Bytes int
}

// MipChain walks the payload once, carrying dimensions and the byte offset
// forward instead of recomputing all preceding levels for every upload.
func (d TextureDescriptor) MipChain(yield func(int, TextureLevel) bool) {
	width, height, offset := d.Width, d.Height, 0
	for level := range d.MipLevels() {
		bytes := d.Format.LevelSize(width, height)
		if !yield(level, TextureLevel{Width: width, Height: height, Offset: offset, Bytes: bytes}) {
			return
		}
		offset += bytes
		width, height = max(1, width/2), max(1, height/2)
	}
}

// Level reports where one mip level lives inside the payload. Level 0 is the
// base level, which starts at offset zero; negative levels clamp to it.
//
// Sequential uploads use MipChain to avoid recalculating preceding levels.
func (d TextureDescriptor) Level(level int) TextureLevel {
	if level <= 0 {
		return TextureLevel{Width: d.Width, Height: d.Height, Bytes: d.Format.LevelSize(d.Width, d.Height)}
	}
	width, height := d.Width, d.Height
	for range level {
		width, height = max(1, width/2), max(1, height/2)
	}
	return TextureLevel{
		Width:  width,
		Height: height,
		Offset: d.Format.ChainSize(d.Width, d.Height, level),
		Bytes:  d.Format.LevelSize(width, height),
	}
}

type ExternalTextureKind uint8

const (
	ExternalTextureD3D11 ExternalTextureKind = iota + 1
	ExternalTextureDMABuf
	ExternalTextureVulkan
	ExternalTextureMetal
)

type ExternalTextureFormat uint8

const (
	ExternalTextureBGRA8 ExternalTextureFormat = iota + 1
	ExternalTextureRGBA8
)

// ExternalTextureDescriptor identifies a texture owned by a platform media
// pipeline. Handle is opaque to the render package and is interpreted only by
// a driver implementing ExternalTextureDriver.
type ExternalTextureDescriptor struct {
	Handle   uint64
	Width    int
	Height   int
	Modifier uint64
	Stride   uint32
	Offset   uint32
	Kind     ExternalTextureKind
	Format   ExternalTextureFormat
}

func (d ExternalTextureDescriptor) Validate() error {
	if d.Kind == 0 || d.Handle == 0 || d.Width <= 0 || d.Height <= 0 || d.Format == 0 {
		return errors.New("render: invalid external texture descriptor")
	}
	return nil
}

func (d TextureDescriptor) Validate(pixels []byte) error {
	if d.Width <= 0 || d.Height <= 0 {
		return errors.New("render: texture dimensions must be positive")
	}
	if !d.WrapU.Valid() || !d.WrapV.Valid() {
		return errors.New("render: unsupported texture address mode")
	}
	if !d.Format.Valid() {
		return errors.New("render: unsupported texture format")
	}
	if d.Levels > MipLevels(d.Width, d.Height) {
		return fmt.Errorf("render: texture format %s has %d mip levels at %dx%d, got %d",
			d.Format.Name(), MipLevels(d.Width, d.Height), d.Width, d.Height, d.Levels)
	}
	required := d.Format.ChainSize(d.Width, d.Height, d.MipLevels())
	if len(pixels) != required {
		return fmt.Errorf("render: texture format %s requires %d bytes, got %d",
			d.Format.Name(), required, len(pixels))
	}
	return nil
}

type TextureRegion struct {
	X      int
	Y      int
	Width  int
	Height int
}

type TextureUpdate struct {
	Pixels  []byte
	Region  TextureRegion
	Texture TextureHandle
}

// ShaderDescriptor contains the native shader representation for each
// renderer. A descriptor may target only the platforms shipped by the game;
// an omitted vertex or fragment stage inherits the pipeline's built-in stage.
type ShaderDescriptor struct {
	MetalSource          string
	WebGL2VertexSource   string
	WebGL2FragmentSource string
	WebGPUVertexSource   string
	WebGPUFragmentSource string
	VulkanVertexSPIRV    []byte
	VulkanFragmentSPIRV  []byte
	DirectXVertexHLSL    []byte
	DirectXFragmentHLSL  []byte
	WebGL2Generated      bool
}

func (d ShaderDescriptor) Validate() error {
	vulkan := len(d.VulkanVertexSPIRV) != 0 || len(d.VulkanFragmentSPIRV) != 0
	webGL2 := d.WebGL2VertexSource != "" || d.WebGL2FragmentSource != ""
	webGPU := d.WebGPUVertexSource != "" || d.WebGPUFragmentSource != ""
	if len(d.VulkanVertexSPIRV)%4 != 0 || len(d.VulkanFragmentSPIRV)%4 != 0 {
		return errors.New("render: Vulkan shader SPIR-V must be 4-byte aligned")
	}
	if !vulkan && len(d.DirectXVertexHLSL) == 0 && len(d.DirectXFragmentHLSL) == 0 && d.MetalSource == "" && !webGL2 && !webGPU {
		return errors.New("render: shader has no platform source")
	}
	return nil
}

type Pipeline uint8

const (
	PipelineSprite Pipeline = iota
	PipelineShape
	PipelineMesh
)

type Instance2D struct {
	Model        [6]float32
	TextureModel [6]float32
	Color        [4]float32
	Custom       [4][4]float32
	// TextureSlot selects a SpriteDraw texture in [0, SpriteTextureSlots).
	TextureSlot float32
}

// SpriteTextureSlots is the built-in sprite pipeline's per-draw texture limit.
const SpriteTextureSlots = 4

type SpriteDraw struct {
	Shader             Material2DShaderHandle
	Texture            TextureHandle
	AdditionalTextures [SpriteTextureSlots - 1]TextureHandle
	FirstInstance      uint32
	InstanceCount      uint32
}

// TextureAt fills unused bindings with the primary texture. Custom shaders
// use only Texture and retain their single-texture sampling contract.
func (d SpriteDraw) TextureAt(slot int) TextureHandle {
	if slot > 0 && d.AdditionalTextures[slot-1].Valid() {
		return d.AdditionalTextures[slot-1]
	}
	return d.Texture
}

type SpriteBatch struct {
	Instances  []Instance2D
	Draws      []SpriteDraw
	Projection [16]float32
}

type ShapeKind uint32

const (
	ShapeRoundedRect ShapeKind = iota + 1
	ShapeCheckmark
	ShapeCornerMask
	// ShapeTriangle is a filled directional triangle. Params.w selects the
	// direction: up, down, left, right (0..3).
	ShapeTriangle
)

type ShapeInstance2D struct {
	Model      [6]float32
	LocalModel [6]float32
	Fill       [4]float32
	Border     [4]float32
	Params     [4]float32
	Kind       ShapeKind
}

type ShapeBatch struct {
	Instances  []ShapeInstance2D
	Projection [16]float32
}

type Vertex3D struct {
	Position [3]float32
	Normal   [3]float32
	UV       [2]float32
	Tangent  [4]float32 // xyz direction; w is bitangent handedness, zero means absent
}

// MeshSkinVertex stores the four influences used by GPU linear-blend
// skinning. The layout is intentionally renderer-independent so backends can
// pack it into a storage buffer without importing graphics types.
type MeshSkinVertex struct {
	Joints  [4]uint32
	Weights [4]float32
}

// MeshMorphVertex stores one morph target delta for one vertex.
type MeshMorphVertex struct {
	Position [3]float32
	Normal   [3]float32
	Tangent  [3]float32
}

// MeshDeformationDescriptor is uploaded once and owned by the mesh resource.
// SetMeshDeformation copies its slices; changing the streams requires a new mesh.
type MeshDeformationDescriptor struct {
	Skin         []MeshSkinVertex
	MorphTargets []MeshMorphVertex
	VertexCount  uint32
	TargetCount  uint32
}

// MeshDeformationData borrows only the animated parameters for this submission.
// Static skin and morph streams reside in Mesh until DestroyMesh.
type MeshDeformationData struct {
	JointMatrices []mgl32.Mat4
	MorphWeights  []float32
	Mesh          MeshHandle
}

type Instance3D struct {
	Model            [16]float32
	BaseColor        [4]float32
	EmissiveMetallic [4]float32 // emissive RGB after strength, metallic
	Material         [4]float32 // roughness, IOR, alpha cutoff, flags
	UVTransform      [4]float32
	Custom           [4][4]float32
}

// SetPickingID stores an exact uint32 bit pattern in the instance lane used by
// the dedicated picking shaders. Picking batches are copies, so material custom
// data in the simulation world is never modified.
func (instance *Instance3D) SetPickingID(id uint32) {
	instance.Custom[3][3] = math.Float32frombits(id)
}

func (instance Instance3D) PickingID() uint32 {
	return math.Float32bits(instance.Custom[3][3])
}

type Light3DKind uint32

const (
	Light3DAmbient Light3DKind = iota
	Light3DDirectional
	Light3DPoint
	Light3DSpot
	Light3DPyramid
	Light3DBox
	Light3DRectangle
	Light3DDisc
	Light3DTube
)

// Light3D is a renderer-independent extracted light. PositionDirection uses
// xyz as a position for local lights and as a direction for directional lights.
// Color.rgb is linear; Color.w is the source radius. Params stores range,
// candela/nits (lux for box), and cone cosines. Right/Up carry orthonormal axes
// and shape dimensions in w.
type Light3D struct {
	Kind              Light3DKind
	Right             [4]float32
	Up                [4]float32
	PositionDirection [4]float32
	Direction         [4]float32
	Color             [4]float32
	Params            [4]float32
}

type MeshDraw3D struct {
	// SurfaceTextures follow MaterialSurface.Textures order.
	SurfaceTextures [MeshSurfaceTextureSlots]TextureHandle
	Mesh            MeshHandle
	Shader          MeshShaderHandle
	Texture         TextureHandle
	NormalTexture   TextureHandle
	// MetallicRoughnessTexture stores glTF's occlusion-independent
	// metallic (B) and roughness (G) channels. A zero handle makes the
	// renderer use the scalar material factors.
	MetallicRoughnessTexture TextureHandle
	FirstIndex               uint32
	IndexCount               uint32
	BaseVertex               int32
	FirstInstance            uint32
	InstanceCount            uint32
	AlphaMode                uint32
	BlendMode                MeshBlendMode
	// DeformationIndex is one-based into MeshBatch3D.Deformations. Zero
	// means that the draw uses its original vertex attributes.
	DeformationIndex uint32
}

type MeshBlendMode uint32

const (
	MeshBlendNormal MeshBlendMode = iota
	MeshBlendAdditive
	MeshBlendMultiply
	MeshBlendScreen
)

type MeshBatch3D struct {
	Lights           []Light3D
	Vertices         []Vertex3D
	Indices          []uint32
	Instances        []Instance3D
	Draws            []MeshDraw3D
	Deformations     []MeshDeformationData
	Lighting         MeshLighting
	ViewProjection   [16]float32
	CameraPosition   [4]float32
	AmbientLight     [4]float32
	LightDirection   [4]float32
	DirectionalLight [4]float32
}

// EnvironmentEncoding describes radiance in the first seven atlas tiles.
// The eighth tile always stores a linear BRDF LUT.
type EnvironmentEncoding uint8

const (
	EnvironmentRGBM EnvironmentEncoding = iota
	EnvironmentRGBE
)

// MeshLighting is shared by all backends. Shadow depth is encoded in an RGBA8
// color target, allowing filtering without requiring sampleable depth support.
type MeshLighting struct {
	FogColor                   [3]float32 // linear radiance in nits
	FogParams                  [4]float32 // start, end, density, mode (0 disabled, 1 linear, 2 exp2)
	ToneMapping                float32    // 0 Reinhard, 1 ACES filmic fit
	OcclusionInverseProjection [16]float32
	OcclusionParams            [4]float32 // world radius, world bias, strength, sample count
	OcclusionTexture           TextureHandle
	Shadows                    [MaxShadowMaps]ShadowMap3D
	LightShadows               [MaxMeshLights][4]float32 // one-based first map, map count, cascade blend; W carries camera color/fog and environment encoding on upload
	CameraForward              [4]float32                // world direction used to select directional cascades; W is replaced by SSAO sample count on upload
	Environment                [4]float32                // intensity, rotation, atlas inverse width, inverse tile height
	EnvironmentEncoding        EnvironmentEncoding
	Params                     [4]float32 // exposure (0 means 1), MeshPass*, SSAO strength on upload, backend winding flip
	EnvironmentTexture         TextureHandle
	ShadowTexture              TextureHandle
}

// Mesh passes reuse the same texture/uniform ABI. SSR resolve aliases shadow
// records and bindings after capturing direct/indirect illumination.
const (
	MeshPassLit = iota
	MeshPassShadowDepth
	MeshPassOcclusionDepth
	MeshPassReflectionCapture
	MeshPassReflectionResolve
	MeshPassReflectionDepth
	MeshPassReflectionOcclusion
	MeshPassFXAA
)

const MeshFlagNoReflection = 1 << 22

// MaxShadowMaps keeps the complete fragment uniform block within GLES3's
// minimum 224 vec4 uniforms and Metal's 4 KiB setFragmentBytes limit.
const MaxShadowMaps = 16

type ShadowMap3D struct {
	Matrix [16]float32
	Rect   [4]float32 // top-left atlas UV offset and size
	Params [4]float32 // depth bias, normal bias, cascade near/far distances
}

const MeshSurfaceTextureSlots = 10
const MeshExtraTextureSlots = MeshSurfaceTextureSlots + 3

func MeshSurfaceTextureFlag(slot int) uint32 {
	if slot < 4 {
		return 16 << slot
	}
	return 65536 << (slot - 4)
}

func (draw MeshDraw3D) ExtraTextures(lighting *MeshLighting) [MeshExtraTextureSlots]TextureHandle {
	var result [MeshExtraTextureSlots]TextureHandle
	copy(result[:4], draw.SurfaceTextures[:4])
	result[4], result[5], result[6] = lighting.EnvironmentTexture, lighting.ShadowTexture, lighting.OcclusionTexture
	copy(result[7:], draw.SurfaceTextures[4:])
	return result
}

// MeshDriver is implemented by drivers that expose Onix's indexed 3D mesh
// pipeline. It is separate from Driver so headless and specialized 2D drivers
// do not need meaningless 3D methods.
type MeshDriver interface {
	CreateMesh([]Vertex3D, []uint32) (MeshHandle, error)
	DestroyMesh(MeshHandle) error
	DrawMeshes(MeshBatch3D) error
}

// MeshPickingDriver draws mesh instances as uint32 entity IDs into the active
// R32Uint render target.
type MeshPickingDriver interface {
	DrawMeshPicking(MeshBatch3D) error
}

// MeshGPUDeformationDriver is implemented by renderers that evaluate skin
// and morph data in GPU buffers. Drivers that do not implement it use the
// CPU fallback assembled by scene.ExtractRender3DSystem.
type MeshGPUDeformationDriver interface {
	SetMeshDeformation(MeshHandle, MeshDeformationDescriptor) error
	DrawMeshesDeformed(MeshBatch3D) error
}

// MeshShaderDriver is implemented by mesh renderers that can compile and own
// user-provided mesh pipelines.
type MeshShaderDriver interface {
	CreateMeshShader(ShaderDescriptor) (MeshShaderHandle, error)
	DestroyMeshShader(MeshShaderHandle) error
}

// Material2DShaderDriver is implemented by sprite renderers that can compile
// user-provided Material2D pipelines.
type Material2DShaderDriver interface {
	CreateMaterial2DShader(ShaderDescriptor) (Material2DShaderHandle, error)
	DestroyMaterial2DShader(Material2DShaderHandle) error
}

// ExternalTextureDriver is optional. Existing renderers continue to use the
// CPU upload path unless they explicitly support importing platform surfaces.
type ExternalTextureDriver interface {
	CreateExternalTexture(ExternalTextureDescriptor) (TextureHandle, error)
}

// ExternalTextureRefreshDriver marks an imported texture as having new
// producer content. Renderers use this hook to attach cross-API synchronization
// only to frames that actually consume a new surface.
type ExternalTextureRefreshDriver interface {
	RefreshExternalTexture(TextureHandle) error
}

// Driver is the complete platform rendering boundary. Implementations submit
// native draw commands; callers never depend on API-specific constants.
type Driver interface {
	RendererInfo() RendererInfo
	BeginFrame(clear Color) error
	SetViewport(Rect)
	Viewport() Rect
	SetBlend(BlendState) error
	SetScissor(*Rect)
	// CreateTexture consumes pixels before returning; it must copy any bytes
	// needed by deferred work. The caller may immediately reuse the slice.
	CreateTexture(TextureDescriptor, []byte) (TextureHandle, error)
	// UpdateTextures consumes every update's pixels before returning.
	UpdateTextures([]TextureUpdate) error
	DestroyTexture(TextureHandle) error
	DrawSprites(SpriteBatch) error
	DrawShapes(ShapeBatch) error
	Present() error
	ResizeSurface(width, height int) error
	SetVSync(enabled bool)
	Destroy()
}

// Device owns one driver. Activation only selects the immediate rendering
// context; lifetime belongs to the window or host that created the device.
type Device struct {
	driver     Driver
	statistics *driverStatistics
	sync.RWMutex
}

var activeDevice atomic.Pointer[Device]
var defaultDevice struct {
	device *Device
	sync.Mutex
}

func NewDevice(driver Driver) (*Device, error) {
	if driver == nil {
		return nil, errors.New("render: nil driver")
	}
	device := &Device{driver: driver, statistics: &driverStatistics{}}
	if provider, ok := driver.(interface{ frameStatistics() *driverStatistics }); ok {
		device.statistics = provider.frameStatistics()
	}
	return device, nil
}

func (device *Device) Driver() Driver {
	if device == nil {
		return nil
	}
	device.RLock()
	driver := device.driver
	device.RUnlock()
	return driver
}

func (device *Device) Activate() {
	if device != nil {
		if driver, ok := device.driver.(interface{ Activate() }); ok {
			driver.Activate()
		}
	}
	activeDevice.Store(device)
}
func ActiveDevice() *Device { return activeDevice.Load() }

func (device *Device) Destroy() {
	if device == nil {
		return
	}
	device.Lock()
	driver := device.driver
	device.driver = nil
	device.Unlock()
	activeDevice.CompareAndSwap(device, nil)
	if driver != nil {
		driver.Destroy()
	}
}

func (device *Device) Present() error {
	if driver := device.Driver(); driver != nil {
		return driver.Present()
	}
	return nil
}

func (device *Device) ResizeSurface(width, height int) error {
	if driver := device.Driver(); driver != nil {
		return driver.ResizeSurface(width, height)
	}
	return nil
}

func (device *Device) SetVSync(enabled bool) {
	if driver := device.Driver(); driver != nil {
		driver.SetVSync(enabled)
	}
}

func (device *Device) SetSurfaceCornerRadius(radius int) {
	if driver, ok := device.Driver().(interface{ SetSurfaceCornerRadius(int) }); ok {
		driver.SetSurfaceCornerRadius(max(radius, 0))
	}
}

// Install replaces the standalone default device. Window-owned devices have
// independent lifetimes and are never destroyed by this operation.
func Install(driver Driver) error {
	defaultDevice.Lock()
	previous := defaultDevice.device
	if driver != nil && previous != nil && previous.Driver() == driver {
		previous.Activate()
		defaultDevice.Unlock()
		return nil
	}
	device, err := NewDevice(driver)
	if err != nil {
		defaultDevice.Unlock()
		return err
	}
	defaultDevice.device = device
	device.Activate()
	defaultDevice.Unlock()
	previous.Destroy()
	return nil
}

func Current() Driver {
	return ActiveDevice().Driver()
}

func Uninstall() {
	defaultDevice.Lock()
	device := defaultDevice.device
	defaultDevice.device = nil
	defaultDevice.Unlock()
	device.Destroy()
}

func Present() error {
	if driver := Current(); driver != nil {
		return driver.Present()
	}
	return nil
}

func ResizeSurface(width, height int) error {
	if driver := Current(); driver != nil {
		return driver.ResizeSurface(width, height)
	}
	return nil
}

func SetVSync(enabled bool) {
	if driver := Current(); driver != nil {
		driver.SetVSync(enabled)
	}
}

func SetSurfaceCornerRadius(radius int) {
	if driver, ok := Current().(interface{ SetSurfaceCornerRadius(int) }); ok {
		driver.SetSurfaceCornerRadius(max(radius, 0))
	}
}
