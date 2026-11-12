package scene

import "github.com/onix-engine/onix/graphics"

type retainedTexture2D struct {
	positions         []int
	content, geometry uint64
}

func (cache *retainedRender2D) trackTexture(position int) {
	texture := cache.items[position].texture
	if texture == nil {
		return
	}
	if cache.textures == nil {
		cache.textures = make(map[*graphics.Texture]*retainedTexture2D)
	}
	dependency := cache.textures[texture]
	if dependency == nil {
		dependency = &retainedTexture2D{}
		cache.textures[texture] = dependency
	}
	dependency.positions = append(dependency.positions, position)
	dependency.content, dependency.geometry = texture.ContentRevision(), texture.GeometryRevision()
}

func (cache *retainedRender2D) replaceTexture(position int, previous *graphics.Texture) {
	if previous == cache.items[position].texture {
		return
	}
	if dependency := cache.textures[previous]; dependency != nil {
		for i, p := range dependency.positions {
			if p == position {
				last := len(dependency.positions) - 1
				dependency.positions[i] = dependency.positions[last]
				dependency.positions = dependency.positions[:last]
				break
			}
		}
		if len(dependency.positions) == 0 {
			delete(cache.textures, previous)
		} else {
			cache.retainTexture(previous, dependency)
		}
	}
	cache.trackTexture(position)
}

// Check unique texture dependencies only after the global change signal moves.
// Pixel changes refresh paint fingerprints and damage without extracting ECS
// geometry; bounds and lifetime changes enqueue their owning entities.
func (cache *retainedRender2D) refreshTextures() {
	revision := graphics.TextureRevision()
	if cache.textureRevision == revision {
		return
	}
	cache.textureRevision = revision
	for texture, dependency := range cache.textures {
		content, geometry := texture.ContentRevision(), texture.GeometryRevision()
		if dependency.content == content && dependency.geometry == geometry {
			continue
		}
		for _, position := range dependency.positions {
			item := &cache.items[position]
			if dependency.geometry != geometry {
				cache.mark(item.entity)
				continue
			}
			cache.recordItemDamage(position)
			cache.paints[position] = preparePaintItem2D(item, cache.device)
			item.paint = &cache.paints[position]
			cache.stats.PaintUpdated++
		}
		dependency.content, dependency.geometry = content, geometry
	}
}
