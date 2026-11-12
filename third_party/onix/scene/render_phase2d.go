package scene

import (
	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/graphics"
	"github.com/go-gl/mathgl/mgl32"
)

type renderPhaseItem2D interface {
	renderSortKey() renderSortKey2D
	renderMaterial() renderMaterialKey
}

// Read both keys through a pointer so traversal does not copy the full render
// item (which also contains transforms, materials and geometry references).
type renderPhaseReader2D[T any] interface {
	*T
	renderPhaseKeys() (renderSortKey2D, renderMaterialKey)
}

type renderPhaseList2D[T any] interface {
	len() int
	at(int) *T
}

type renderPhaseSlice2D[T any] []T

func (items renderPhaseSlice2D[T]) len() int        { return len(items) }
func (items renderPhaseSlice2D[T]) at(index int) *T { return &items[index] }

// Filtered views store only positions in the immutable extracted item array.
type renderItemList2D struct {
	items   []renderItem2D
	indices []uint32
	count   int
}

func (items renderItemList2D) len() int { return items.count }
func (items renderItemList2D) at(index int) *renderItem2D {
	if items.indices != nil {
		index = int(items.indices[index])
	}
	return &items.items[index]
}
func (items renderItemList2D) slice(start, end int) renderItemList2D {
	if items.indices != nil {
		items.indices = items.indices[start:end]
	} else {
		items.items = items.items[start:end]
	}
	items.count = end - start
	return items
}

type renderSortKey2D struct {
	scene     uint64
	group     uint64
	sortIndex int
	order     uint64
	depth     float32
	sequence  uint32
	ui        bool
}

type renderBatch2D struct {
	scene     uint64
	group     uint64
	start     int
	end       int
	ui        bool
	sortIndex int
	material  renderMaterialKey
}

func compareRenderPhaseItem2D[T renderPhaseItem2D](a, b T) int {
	aKey, bKey := a.renderSortKey(), b.renderSortKey()
	if aKey.scene < bKey.scene {
		return -1
	}
	if aKey.scene > bKey.scene {
		return 1
	}
	if aKey.ui != bKey.ui {
		if aKey.ui {
			return 1
		}
		return -1
	}
	if aKey.group < bKey.group {
		return -1
	}
	if aKey.group > bKey.group {
		return 1
	}
	if aKey.sortIndex < bKey.sortIndex {
		return -1
	}
	if aKey.sortIndex > bKey.sortIndex {
		return 1
	}
	if aKey.depth < bKey.depth {
		return -1
	}
	if aKey.depth > bKey.depth {
		return 1
	}
	if aKey.order < bKey.order {
		return -1
	}
	if aKey.order > bKey.order {
		return 1
	}
	if aKey.sequence < bKey.sequence {
		return -1
	}
	if aKey.sequence > bKey.sequence {
		return 1
	}
	return 0
}

// drawRenderPhase2D consumes sorted items, preserving order while gathering
// ranges only for renderers that require a complete batch (such as Spine).
func drawRenderPhase2D[T renderPhaseItem2D, P renderPhaseReader2D[T]](c *graphics.Canvas2D, world *ecs.World, items []T, views DrawViews2D, cull bool, batched func(*T) bool, draw func(renderBatch2D, renderViewport2D, bool)) {
	drawRenderPhaseList2D[T, P](c, world, renderPhaseSlice2D[T](items), views, cull, batched, draw)
}

func drawRenderPhaseList2D[T renderPhaseItem2D, P renderPhaseReader2D[T], L renderPhaseList2D[T]](c *graphics.Canvas2D, world *ecs.World, items L, views DrawViews2D, cull bool, batched func(*T) bool, draw func(renderBatch2D, renderViewport2D, bool)) {
	currentUI := false
	spaceReady := false
	var viewport renderViewport2D
	var previous renderBatch2D
	for start := 0; start < items.len(); {
		key, material := P(items.at(start)).renderPhaseKeys()
		batch := renderBatch2D{scene: key.scene, group: key.group, start: start, end: start + 1, ui: key.ui, sortIndex: key.sortIndex, material: material}
		begin := start == 0 || previous.scene != batch.scene || previous.group != batch.group || previous.ui != batch.ui || previous.sortIndex != batch.sortIndex || previous.material != batch.material

		if !spaceReady || currentUI != batch.ui {
			currentUI = batch.ui
			spaceReady = true
			if cull {
				viewport = renderViewport2DForSpace(c, world, batch.ui)
				if batch.ui {
					c.ResetProjectionMatrix()
					c.SetViewMatrix(mgl32.Ident4())
				} else if camera, projection, transform, ok := activeCamera2DComponents(world); ok {
					viewport := camera.ResolvedViewport()
					c.SetProjectionMatrix(projection.Matrix(viewport.Width, viewport.Height))
					c.SetViewMatrix(transform.Matrix.Inv())
				}
			} else {
				c.ResetProjectionMatrix()
				if batch.ui {
					c.SetViewMatrix(views.UI)
				} else {
					c.SetViewMatrix(views.World)
				}
				viewport = renderViewport2D{clipFromLayer: c.ProjectionMatrix().Mul4(c.ViewMatrix()), active: true}
			}
		}
		if begin && batched != nil && batched(items.at(start)) {
			for batch.end < items.len() {
				next, nextMaterial := P(items.at(batch.end)).renderPhaseKeys()
				if next.scene != key.scene || next.group != key.group || next.ui != key.ui || next.sortIndex != key.sortIndex || nextMaterial != material {
					break
				}
				batch.end++
			}
		}
		draw(batch, viewport, begin)
		previous = batch
		start = batch.end
	}
}
