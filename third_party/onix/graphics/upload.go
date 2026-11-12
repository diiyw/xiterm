package graphics

import (
	"errors"
	"time"
	"unsafe"

	"github.com/onix-engine/onix/render"
)

var ErrUploadUnavailable = errors.New("graphics: resource is not resident after upload")

// UploadBudget limits work started in one Advance call. Time is checked between
// resources: a single driver call cannot be interrupted. An oversized resource
// is allowed as the first unit to guarantee progress. Non-positive Time performs
// one unit; non-positive Bytes and Resources impose no additional limit.
type UploadBudget struct {
	Time      time.Duration
	Bytes     int64
	Resources int
}

type uploadResource interface {
	uploadFor(*render.Device) error
	uploadBytes() int64
}

type uploadTask struct {
	resource uploadResource
	bytes    int64
}

// UploadQueue deduplicates CPU-ready assets. All methods belong to the render
// thread, and all uploads use the explicitly supplied device.
type UploadQueue struct {
	seen   map[uploadResource]struct{}
	device *render.Device
	tasks  []uploadTask
	index  int
}

func (q *UploadQueue) add(value uploadResource) {
	if q.seen == nil {
		q.seen = make(map[uploadResource]struct{})
	}
	if _, ok := q.seen[value]; ok {
		return
	}
	q.seen[value] = struct{}{}
	q.tasks = append(q.tasks, uploadTask{value, value.uploadBytes()})
}

func (q *UploadQueue) AddTexture(texture *Texture) {
	if base := texture.baseTexture(); base != nil && base.usage.Has(ResourceUsageGPU) {
		q.add(base)
	}
}

func (q *UploadQueue) AddMesh(mesh *Mesh) {
	if mesh != nil {
		q.add(mesh)
	}
}

func (q *UploadQueue) AddMaterial2DShader(shader *Material2DShader) {
	if shader != nil {
		q.add(shader)
	}
}

func (q *UploadQueue) AddMeshShader(shader *MeshShader) {
	if shader != nil {
		q.add(shader)
	}
}

func (q *UploadQueue) AddMaterial3D(material Material3DData) {
	q.AddMeshShader(material.Shader)
	q.AddTexture(material.BaseColorTexture)
	q.AddTexture(material.NormalTexture)
	q.AddTexture(material.MetallicRoughnessTexture)
	for _, texture := range material.Surface.Textures() {
		q.AddTexture(texture)
	}
}

func (q *UploadQueue) Remaining() int { return len(q.tasks) - q.index }

func (q *UploadQueue) ReadyFor(device *render.Device) bool {
	return len(q.tasks) == 0 || q.device == device && q.index == len(q.tasks) && device.Driver() != nil
}

func (q *UploadQueue) Advance(device *render.Device, budget UploadBudget) (bool, error) {
	if q.device != device {
		q.device, q.index = device, 0
	}
	if len(q.tasks) == 0 {
		return true, nil
	}
	if device.Driver() == nil {
		return false, nil
	}
	if q.Remaining() == 0 {
		return true, nil
	}
	deadline := time.Now().Add(budget.Time)
	bytes, units := int64(0), 0
	for q.index < len(q.tasks) {
		task := q.tasks[q.index]
		if units > 0 && (budget.Bytes > 0 && task.bytes > budget.Bytes-bytes || budget.Resources > 0 && units >= budget.Resources || !time.Now().Before(deadline)) {
			break
		}
		if err := task.resource.uploadFor(device); err != nil {
			return false, err
		}
		q.index++
		bytes += task.bytes
		units++
	}
	return q.Remaining() == 0, nil
}

// Reset releases references and retains storage for the next preparation.
func (q *UploadQueue) Reset() {
	clear(q.tasks)
	q.tasks, q.index, q.device = q.tasks[:0], 0, nil
	clear(q.seen)
}

func (t *Texture) uploadBytes() int64 { return int64(t.payloadBytes()) }
func (t *Texture) uploadFor(device *render.Device) error {
	if t.RenderHandleFor(device).Valid() && t.UploadError() == nil {
		return nil
	}
	return errors.Join(ErrUploadUnavailable, t.UploadError())
}

func (m *Mesh) uploadBytes() int64 {
	return int64(len(m.Vertices))*int64(unsafe.Sizeof(render.Vertex3D{})) + int64(len(m.Indices))*4 +
		int64(len(m.Skin))*int64(unsafe.Sizeof(render.MeshSkinVertex{})) +
		int64(len(m.Vertices))*int64(len(m.MorphTargets))*int64(unsafe.Sizeof(render.MeshMorphVertex{}))
}
func (m *Mesh) uploadFor(device *render.Device) error {
	if m.RenderHandleFor(device).Valid() && m.UploadError() == nil {
		return nil
	}
	return errors.Join(ErrUploadUnavailable, m.UploadError())
}

func (s *shaderResource[R]) uploadBytes() int64 { return 0 }
func (s *shaderResource[R]) uploadFor(device *render.Device) error {
	if s.RenderHandleFor(device).Valid() && s.UploadError() == nil {
		return nil
	}
	return errors.Join(ErrUploadUnavailable, s.UploadError())
}
