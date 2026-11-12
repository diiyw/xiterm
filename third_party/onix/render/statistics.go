package render

import (
	"sync"
	"unsafe"
)

// FrameStatistics describes the work submitted to the GPU in the current
// frame. It reports submission and upload counts, not fragment count or GPU
// execution time; use a platform GPU capture for those measurements.
type FrameStatistics struct {
	SurfaceWidth  uint64
	SurfaceHeight uint64

	DrawCalls uint64
	Sprites   uint64
	Shapes    uint64
	Meshes    uint64
	Vertices  uint64
	Triangles uint64

	SpriteDrawCalls uint64
	ShapeDrawCalls  uint64
	MeshDrawCalls   uint64

	BufferUploadBytes uint64

	DynamicMeshDrawCalls   uint64
	DynamicMeshVertices    uint64
	DynamicMeshIndices     uint64
	DynamicMeshUploadBytes uint64

	TextureUploads     uint64
	TextureUploadBytes uint64
}

// RendererInfo identifies the active graphics API and physical device.
type RendererInfo struct {
	Backend string
	Device  string
	// RenderTargetBottomLeft reports whether sampled color attachments use a
	// bottom-left origin. RenderTarget.Read always uses a top-left origin.
	RenderTargetBottomLeft bool
}

type driverStatistics struct {
	sync.RWMutex
	frame FrameStatistics
}

func (statistics *driverStatistics) frameStatistics() *driverStatistics { return statistics }

// Statistics returns a snapshot of the draw work submitted so far this frame.
func Statistics() FrameStatistics {
	return ActiveDevice().Statistics()
}

func (device *Device) Statistics() FrameStatistics {
	if device == nil {
		return FrameStatistics{}
	}
	state := device.statistics
	state.RLock()
	frame := state.frame
	state.RUnlock()
	return frame
}

// CurrentRendererInfo returns metadata reported by the active render driver.
func CurrentRendererInfo() RendererInfo { return ActiveDevice().RendererInfo() }

func (device *Device) RendererInfo() RendererInfo {
	driver := device.Driver()
	if driver == nil {
		return RendererInfo{}
	}
	return driver.RendererInfo()
}

// ResetStatistics starts a new frame statistics window.
func ResetStatistics() { ActiveDevice().ResetStatistics() }

func (device *Device) ResetStatistics() {
	if device != nil {
		device.statistics.beginFrameStatistics(0, 0)
	}
}

func (statisticsState *driverStatistics) beginFrameStatistics(width, height int) {
	var surfaceWidth, surfaceHeight uint64
	if width > 0 {
		surfaceWidth = uint64(width)
	}
	if height > 0 {
		surfaceHeight = uint64(height)
	}
	statisticsState.Lock()
	statisticsState.frame = FrameStatistics{
		SurfaceWidth:  surfaceWidth,
		SurfaceHeight: surfaceHeight,
	}
	statisticsState.Unlock()
}

func (statisticsState *driverStatistics) recordSpriteStatistics(batch SpriteBatch, sprites, uploadBytes uint64) {
	statisticsState.Lock()
	statisticsState.frame.DrawCalls += uint64(len(batch.Draws))
	statisticsState.frame.SpriteDrawCalls += uint64(len(batch.Draws))
	statisticsState.frame.Sprites += sprites
	statisticsState.frame.Vertices += sprites * 4
	statisticsState.frame.Triangles += sprites * 2
	statisticsState.frame.BufferUploadBytes += uploadBytes
	statisticsState.Unlock()
}

func (statisticsState *driverStatistics) recordShapeStatistics(batch ShapeBatch) {
	instances := uint64(len(batch.Instances))
	statisticsState.Lock()
	statisticsState.frame.DrawCalls++
	statisticsState.frame.ShapeDrawCalls++
	statisticsState.frame.Shapes += instances
	statisticsState.frame.Vertices += instances * 4
	statisticsState.frame.Triangles += instances * 2
	statisticsState.frame.BufferUploadBytes += sliceByteSize(batch.Instances)
	statisticsState.Unlock()
}

type meshDrawStatistics struct {
	instances, vertices, triangles uint64
}

func (s *meshDrawStatistics) add(draw MeshDraw3D) {
	s.instances += uint64(draw.InstanceCount)
	s.vertices += uint64(draw.IndexCount) * uint64(draw.InstanceCount)
	s.triangles += uint64(draw.IndexCount/3) * uint64(draw.InstanceCount)
}

func (statisticsState *driverStatistics) recordMeshStatistics(batch MeshBatch3D, counts meshDrawStatistics) {
	dynamicBytes := sliceByteSize(batch.Vertices) + sliceByteSize(batch.Indices)
	statisticsState.Lock()
	statisticsState.frame.DrawCalls += uint64(len(batch.Draws))
	statisticsState.frame.MeshDrawCalls += uint64(len(batch.Draws))
	statisticsState.frame.Meshes += counts.instances
	statisticsState.frame.Vertices += counts.vertices
	statisticsState.frame.Triangles += counts.triangles
	statisticsState.frame.BufferUploadBytes += sliceByteSize(batch.Instances) + dynamicBytes
	if len(batch.Vertices) != 0 || len(batch.Indices) != 0 {
		statisticsState.frame.DynamicMeshDrawCalls += uint64(len(batch.Draws))
		statisticsState.frame.DynamicMeshVertices += uint64(len(batch.Vertices))
		statisticsState.frame.DynamicMeshIndices += uint64(len(batch.Indices))
		statisticsState.frame.DynamicMeshUploadBytes += dynamicBytes
	}
	statisticsState.Unlock()
}

func (statisticsState *driverStatistics) recordTextureUploadStatistics(count int, bytes uint64) {
	if count <= 0 && bytes == 0 {
		return
	}
	statisticsState.Lock()
	statisticsState.frame.TextureUploads += uint64(max(count, 0))
	statisticsState.frame.TextureUploadBytes += bytes
	statisticsState.Unlock()
}

func (statisticsState *driverStatistics) recordTextureUpdates(updates []TextureUpdate) {
	if len(updates) == 0 {
		return
	}
	var bytes uint64
	for _, update := range updates {
		bytes += uint64(len(update.Pixels))
	}
	statisticsState.recordTextureUploadStatistics(len(updates), bytes)
}

func sliceByteSize[T any](values []T) uint64 {
	var value T
	return uint64(len(values)) * uint64(unsafe.Sizeof(value))
}
