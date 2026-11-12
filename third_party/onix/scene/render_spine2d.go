package scene

import (
	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/graphics"
	"github.com/onix-engine/onix/internal/reuse"
	"github.com/onix-engine/onix/render"
)

type spineRenderBatch2DScratch struct{}

type spineRenderScratch2D struct {
	batch     render.MeshBatch3D
	retention [4]reuse.Retention
}

func (s *spineRenderScratch2D) trim() {
	s.batch.Vertices = s.retention[0].Trim(s.batch.Vertices[:0], 64)
	s.batch.Indices = s.retention[1].Trim(s.batch.Indices[:0], 96)
	s.batch.Instances = s.retention[2].Trim(s.batch.Instances[:0], 1)
	s.batch.Draws = s.retention[3].Trim(s.batch.Draws[:0], 8)
}

func drawSpineRenderBatch2D(c *graphics.Canvas2D, world *ecs.World, items renderItemList2D, viewport renderViewport2D) {
	scratch := world.FrameValue[spineRenderBatch2DScratch, spineRenderScratch2D]()
	batch := &scratch.batch
	*batch = render.MeshBatch3D{
		Vertices: batch.Vertices[:0], Indices: batch.Indices[:0],
		Instances: batch.Instances[:0], Draws: batch.Draws[:0],
	}
	projection, view := c.ProjectionMatrix(), c.ViewMatrix()
	for index := 0; index < items.len(); index++ {
		item := items.at(index)
		model := SpineModelMatrix2D(item.transform, item.ui)
		vertices, indices, instances, draws := len(batch.Vertices), len(batch.Indices), len(batch.Instances), len(batch.Draws)
		var lastDraw render.MeshDraw3D
		if draws != 0 {
			lastDraw = batch.Draws[draws-1]
		}
		minX, minY, maxX, maxY, valid := item.snapshot[Spine2D]().Player.AppendMeshBatchInto(batch, projection, view.Mul4(model))
		scratch.retention[0].Record(len(batch.Vertices))
		scratch.retention[1].Record(len(batch.Indices))
		scratch.retention[2].Record(len(batch.Instances))
		scratch.retention[3].Record(len(batch.Draws))
		if !spineRenderItemVisible2D(viewport, model, item.clip, minX, minY, maxX, maxY, valid) {
			// Appending can extend the preceding draw. Roll that change back too
			// when the newly generated local bounds are outside this viewport.
			batch.Vertices, batch.Indices = batch.Vertices[:vertices], batch.Indices[:indices]
			batch.Instances, batch.Draws = batch.Instances[:instances], batch.Draws[:draws]
			if draws != 0 {
				batch.Draws[draws-1] = lastDraw
			}
		}
	}
	if len(batch.Draws) != 0 {
		_ = c.DrawMeshes(*batch)
	}
}
