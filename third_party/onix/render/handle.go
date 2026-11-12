package render

import "fmt"

type TextureResource struct{}
type MeshResource struct{}
type MeshShaderResource struct{}
type Material2DShaderResource struct{}
type RenderTargetResource struct{}

// Handle is a typed GPU resource reference.
type Handle[T any] struct {
	id         uint32
	generation uint32
}

func NewHandle[T any](id, generation uint32) Handle[T] {
	return Handle[T]{id: id, generation: generation}
}

func (h Handle[T]) Valid() bool { return h.id != 0 }

func (h Handle[T]) ID() uint32 { return h.id }

func (h Handle[T]) Generation() uint32 { return h.generation }

func (h Handle[T]) String() string {
	return fmt.Sprintf("%d:%d", h.id, h.generation)
}

type TextureHandle = Handle[TextureResource]
type MeshHandle = Handle[MeshResource]
type MeshShaderHandle = Handle[MeshShaderResource]
type Material2DShaderHandle = Handle[Material2DShaderResource]
type RenderTargetHandle = Handle[RenderTargetResource]
