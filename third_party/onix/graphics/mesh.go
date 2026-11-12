package graphics

import (
	"fmt"
	"math"

	"github.com/onix-engine/onix/render"
	"github.com/go-gl/mathgl/mgl32"
)

// Mesh is indexed triangle geometry using the same position/normal/UV/tangent layout
// consumed by Onix's 3D render pipelines. Geometry and static deformation streams
// must be set before the first upload; changing them requires a new mesh.
type Mesh struct {
	err          error
	residencies  render.DeviceValues[render.MeshHandle]
	Vertices     []render.Vertex3D
	Indices      []uint32
	Submeshes    []Submesh
	Skin         []SkinVertex
	MorphTargets []MorphTarget
	gpuSkin      []render.MeshSkinVertex
	gpuMorph     []render.MeshMorphVertex
	Bounds       BoundingSphere
	handle       render.MeshHandle
	disposed     bool
}

// Submesh selects one material and an optional animated rigid node. Node is
// -1 for skinned geometry or geometry without a node transform.
type Submesh struct {
	FirstIndex uint32
	IndexCount uint32
	Material   uint32
	Node       int32
}

// MorphTarget stores imported position/normal/tangent deltas. Rendering
// backends may opt into applying these deltas without changing indexed base
// geometry or the mesh upload ABI.
type MorphTarget struct {
	Name          string
	PositionDelta [][3]float32
	NormalDelta   [][3]float32
	TangentDelta  [][3]float32
}

type SkinVertex struct {
	Joints  [4]uint32
	Weights [4]float32
	Active  bool
}

type BoundingSphere struct {
	Center mgl32.Vec3
	Radius float32
}

func NewMesh(vertices []render.Vertex3D, indices []uint32) (*Mesh, error) {
	if err := validateMeshGeometry(vertices, indices); err != nil {
		return nil, err
	}
	return newMeshOwned(append([]render.Vertex3D(nil), vertices...), append([]uint32(nil), indices...)), nil
}

// NewMeshOwned constructs a mesh by taking ownership of vertices and indices.
// Callers must not retain or mutate either slice after a successful call.
func NewMeshOwned(vertices []render.Vertex3D, indices []uint32) (*Mesh, error) {
	if err := validateMeshGeometry(vertices, indices); err != nil {
		return nil, err
	}
	return newMeshOwned(vertices, indices), nil
}

func validateMeshGeometry(vertices []render.Vertex3D, indices []uint32) error {
	if len(vertices) == 0 {
		return fmt.Errorf("graphics: mesh has no vertices")
	}
	if len(indices) == 0 || len(indices)%3 != 0 {
		return fmt.Errorf("graphics: mesh index count must be a non-zero multiple of three")
	}
	for _, index := range indices {
		if uint64(index) >= uint64(len(vertices)) {
			return fmt.Errorf("graphics: mesh index %d exceeds %d vertices", index, len(vertices))
		}
	}
	return nil
}

func newMeshOwned(vertices []render.Vertex3D, indices []uint32) *Mesh {
	mesh := &Mesh{Vertices: vertices, Indices: indices}
	mesh.Bounds = mesh.calculateBounds()
	return mesh
}

func (m *Mesh) RenderHandle() render.MeshHandle {
	return m.RenderHandleFor(render.ActiveDevice())
}

func (m *Mesh) RenderHandleFor(device *render.Device) render.MeshHandle {
	if m == nil {
		return render.MeshHandle{}
	}
	m.ensureUploadedFor(device)
	return m.handle
}

func (m *Mesh) UploadError() error {
	if m == nil {
		return nil
	}
	return m.err
}

func (m *Mesh) Dispose() error {
	if m == nil || m.disposed {
		return nil
	}
	if err := m.residencies.Release(func(device *render.Device, handle render.MeshHandle) error {
		return device.Driver().(render.MeshDriver).DestroyMesh(handle)
	}); err != nil {
		return err
	}
	m.handle = render.MeshHandle{}
	m.Vertices = nil
	m.Indices = nil
	m.Submeshes = nil
	m.Skin = nil
	m.MorphTargets = nil
	m.gpuSkin = nil
	m.gpuMorph = nil
	m.disposed = true
	m.err = fmt.Errorf("graphics: mesh is disposed")
	return nil
}

// GPUDeformationData returns cached renderer-neutral skin and morph streams.
// The returned slices remain owned by the mesh and are valid until Dispose.
func (m *Mesh) GPUDeformationData() ([]render.MeshSkinVertex, []render.MeshMorphVertex, error) {
	if m == nil || m.disposed {
		return nil, nil, fmt.Errorf("graphics: mesh is unavailable")
	}
	if len(m.Skin) != 0 {
		if len(m.Skin) != len(m.Vertices) {
			return nil, nil, fmt.Errorf("graphics: skin vertex count does not match mesh")
		}
		if len(m.gpuSkin) != len(m.Skin) {
			m.gpuSkin = make([]render.MeshSkinVertex, len(m.Skin))
			for index, value := range m.Skin {
				if value.Active {
					m.gpuSkin[index] = render.MeshSkinVertex{Joints: value.Joints, Weights: value.Weights}
				}
			}
		}
	} else {
		m.gpuSkin = nil
	}
	if len(m.MorphTargets) != 0 {
		morphVertices := len(m.Vertices) * len(m.MorphTargets)
		if len(m.gpuMorph) != morphVertices {
			m.gpuMorph = make([]render.MeshMorphVertex, morphVertices)
			for targetIndex, target := range m.MorphTargets {
				if len(target.PositionDelta) != len(m.Vertices) || (len(target.NormalDelta) != 0 && len(target.NormalDelta) != len(m.Vertices)) || (len(target.TangentDelta) != 0 && len(target.TangentDelta) != len(m.Vertices)) {
					m.gpuMorph = nil
					return nil, nil, fmt.Errorf("graphics: morph target %d does not match mesh vertices", targetIndex)
				}
				for vertex := range m.Vertices {
					value := render.MeshMorphVertex{Position: target.PositionDelta[vertex]}
					if len(target.NormalDelta) != 0 {
						value.Normal = target.NormalDelta[vertex]
					}
					if len(target.TangentDelta) != 0 {
						value.Tangent = target.TangentDelta[vertex]
					}
					m.gpuMorph[targetIndex*len(m.Vertices)+vertex] = value
				}
			}
		}
	} else {
		m.gpuMorph = nil
	}
	return m.gpuSkin, m.gpuMorph, nil
}

func (m *Mesh) ensureUploaded() {
	m.ensureUploadedFor(render.ActiveDevice())
}

func (m *Mesh) ensureUploadedFor(device *render.Device) {
	if m == nil || m.disposed || len(m.Vertices) == 0 || len(m.Indices) == 0 {
		return
	}
	if handle, ok := m.residencies.Get(device); ok {
		m.handle, m.err = handle, nil
		return
	}
	m.handle = render.MeshHandle{}
	driver := device.Driver()
	if driver == nil {
		return
	}
	meshDriver, ok := driver.(render.MeshDriver)
	if !ok {
		m.err = fmt.Errorf("graphics: current renderer does not support meshes")
		return
	}
	m.handle, m.err = meshDriver.CreateMesh(m.Vertices, m.Indices)
	if m.err != nil {
		m.err = fmt.Errorf("graphics: upload mesh: %w", m.err)
		return
	}
	if gpuDriver, ok := driver.(render.MeshGPUDeformationDriver); ok && (len(m.Skin) != 0 || len(m.MorphTargets) != 0) {
		skin, morph, err := m.GPUDeformationData()
		if err == nil {
			err = gpuDriver.SetMeshDeformation(m.handle, render.MeshDeformationDescriptor{Skin: skin, MorphTargets: morph, VertexCount: uint32(len(m.Vertices)), TargetCount: uint32(len(m.MorphTargets))})
		}
		if err != nil {
			_ = meshDriver.DestroyMesh(m.handle)
			m.handle = render.MeshHandle{}
			m.err = fmt.Errorf("graphics: upload mesh deformation: %w", err)
			return
		}
	}
	m.residencies.Set(device, m.handle)
}

func CubeMesh(size float32) *Mesh {
	h := size * 0.5
	vertices := make([]render.Vertex3D, 0, 24)
	indices := make([]uint32, 0, 36)
	faces := [...]struct {
		normal     [3]float32
		a, b, c, d [3]float32
	}{
		{[3]float32{0, 0, 1}, [3]float32{-h, -h, h}, [3]float32{h, -h, h}, [3]float32{h, h, h}, [3]float32{-h, h, h}},
		{[3]float32{0, 0, -1}, [3]float32{h, -h, -h}, [3]float32{-h, -h, -h}, [3]float32{-h, h, -h}, [3]float32{h, h, -h}},
		{[3]float32{1, 0, 0}, [3]float32{h, -h, h}, [3]float32{h, -h, -h}, [3]float32{h, h, -h}, [3]float32{h, h, h}},
		{[3]float32{-1, 0, 0}, [3]float32{-h, -h, -h}, [3]float32{-h, -h, h}, [3]float32{-h, h, h}, [3]float32{-h, h, -h}},
		{[3]float32{0, 1, 0}, [3]float32{-h, h, h}, [3]float32{h, h, h}, [3]float32{h, h, -h}, [3]float32{-h, h, -h}},
		{[3]float32{0, -1, 0}, [3]float32{-h, -h, -h}, [3]float32{h, -h, -h}, [3]float32{h, -h, h}, [3]float32{-h, -h, h}},
	}
	uv := [...][2]float32{{0, 1}, {1, 1}, {1, 0}, {0, 0}}
	for _, face := range faces {
		base := uint32(len(vertices))
		for index, position := range [...][3]float32{face.a, face.b, face.c, face.d} {
			vertices = append(vertices, render.Vertex3D{Position: position, Normal: face.normal, UV: uv[index]})
		}
		indices = append(indices, base, base+1, base+2, base, base+2, base+3)
	}
	mesh, _ := NewMesh(vertices, indices)
	return mesh
}

func PlaneMesh(size float32) *Mesh {
	h := size * 0.5
	mesh, _ := NewMesh([]render.Vertex3D{
		{Position: [3]float32{-h, 0, -h}, Normal: [3]float32{0, 1, 0}, UV: [2]float32{0, 1}},
		{Position: [3]float32{-h, 0, h}, Normal: [3]float32{0, 1, 0}, UV: [2]float32{0, 0}},
		{Position: [3]float32{h, 0, h}, Normal: [3]float32{0, 1, 0}, UV: [2]float32{1, 0}},
		{Position: [3]float32{h, 0, -h}, Normal: [3]float32{0, 1, 0}, UV: [2]float32{1, 1}},
	}, []uint32{0, 1, 2, 0, 2, 3})
	return mesh
}

func UVSphereMesh(radius float32, sectors, stacks int) *Mesh {
	if sectors < 3 || stacks < 2 {
		panic("graphics: UV sphere requires at least 3 sectors and 2 stacks")
	}
	vertices := make([]render.Vertex3D, 0, (sectors+1)*(stacks+1))
	indices := make([]uint32, 0, sectors*stacks*6)
	for stack := 0; stack <= stacks; stack++ {
		v := float32(stack) / float32(stacks)
		phi := float64(v-0.5) * math.Pi
		y := float32(math.Sin(phi))
		ring := float32(math.Cos(phi))
		for sector := 0; sector <= sectors; sector++ {
			u := float32(sector) / float32(sectors)
			theta := float64(u) * math.Pi * 2
			x, z := ring*float32(math.Cos(theta)), ring*float32(math.Sin(theta))
			vertices = append(vertices, render.Vertex3D{
				Position: [3]float32{x * radius, y * radius, z * radius},
				Normal:   [3]float32{x, y, z},
				UV:       [2]float32{u, 1 - v},
			})
		}
	}
	row := sectors + 1
	for stack := 0; stack < stacks; stack++ {
		for sector := 0; sector < sectors; sector++ {
			a := uint32(stack*row + sector)
			b := a + uint32(row)
			indices = append(indices, a, b, b+1, a, b+1, a+1)
		}
	}
	mesh, _ := NewMesh(vertices, indices)
	return mesh
}

func (m *Mesh) calculateBounds() BoundingSphere {
	center := mgl32.Vec3{}
	for _, vertex := range m.Vertices {
		center = center.Add(mgl32.Vec3(vertex.Position))
	}
	center = center.Mul(1 / float32(len(m.Vertices)))
	radiusSquared := float32(0)
	for _, vertex := range m.Vertices {
		delta := mgl32.Vec3(vertex.Position).Sub(center)
		radiusSquared = max(radiusSquared, delta.Dot(delta))
	}
	return BoundingSphere{Center: center, Radius: float32(math.Sqrt(float64(radiusSquared)))}
}
