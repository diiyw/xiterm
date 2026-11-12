package scene

import (
	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/graphics"
)

// QueueMaterial2DUploads uses the same registered generic material readers as
// rendering, including the entity's shader override.
func QueueMaterial2DUploads(queue *graphics.UploadQueue, world *ecs.World, entity ecs.Entity) {
	if state, ok := world.GetResource[*RenderWorld](); ok && state != nil {
		queue.AddMaterial2DShader(material2DForEntity(world, entity).Shader)
	} else if shader := world.GetRef[Shader[graphics.Material2DShader]](entity); shader != nil {
		queue.AddMaterial2DShader(shader.Program)
	}
}

// PrepareText2D shapes and rasterizes the current glyph window without uploading
// its atlas. Call on the owning loop thread after layout has settled.
func PrepareText2D(text *Text2D) bool {
	return text != nil && text.Font != nil && text.ensureLayout()
}

// QueueUploads adds resources referenced by CPU-side text layout. Layout and
// rasterization stay on the owning loop thread; the queue budgets first upload.
func (t *Text2D) QueueUploads(queue *graphics.UploadQueue) {
	if t == nil || t.cache == nil {
		return
	}
	for _, glyph := range t.glyphs {
		if texture, ok := t.cache.Atlas(glyph.raster.Glyph().Atlas); ok {
			queue.AddTexture(texture)
		}
	}
	for _, glyph := range t.outlineGlyphs {
		if texture, ok := t.cache.Atlas(glyph.Glyph().Atlas); ok {
			queue.AddTexture(texture)
		}
	}
}
