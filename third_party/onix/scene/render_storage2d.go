package scene

import (
	"unsafe"

	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/graphics"
	"github.com/onix-engine/onix/internal/reuse"
)

func sliceStorage2D[T any](values []T) (used, capacity int) {
	var value T
	size := int(unsafe.Sizeof(value))
	return len(values) * size, cap(values) * size
}

func (node *retainedEntity2D) storage() (used, capacity int) {
	add := func(length, reserved int) { used += length; capacity += reserved }
	add(sliceStorage2D(node.sprites))
	add(sliceStorage2D(node.texts))
	add(sliceStorage2D(node.nineSlices))
	add(sliceStorage2D(node.extra))
	if s := node.snapshots; s != nil {
		add(sliceStorage2D(s.materials))
		add(sliceStorage2D(s.emitters))
		add(sliceStorage2D(s.tilemaps))
		add(sliceStorage2D(s.spines))
		add(sliceStorage2D(s.particles))
		add(sliceStorage2D(s.particleColors))
		add(sliceStorage2D(s.particleAppearances))
		for _, tilemap := range s.tilemaps {
			add(sliceStorage2D(tilemap.value.Tiles))
		}
	}
	return
}

func (cache *retainedRender2D) releaseNode(entity ecs.Entity, node *retainedEntity2D) {
	node.releaseUnused()
	used, capacity := node.storage()
	if capacity <= 1024 || used > capacity/4 {
		return
	}
	if cache.nestedRetention == nil {
		cache.nestedRetention = make(map[ecs.Entity]reuse.Retention)
	}
	if _, exists := cache.nestedRetention[entity]; !exists {
		cache.nestedRetention[entity] = reuse.Retention{}
	}
}

func (node *retainedEntity2D) compact(items []renderItem2D) {
	node.sprites = reuse.Compact(node.sprites, 4)
	node.nineSlices = reuse.Compact(node.nineSlices, 4)
	node.texts = reuse.Compact(node.texts, 2)
	node.extra = reuse.Compact(node.extra, 4)
	slice := 0
	for i := range node.sprites {
		if node.sprites[i].Slice != nil {
			node.sprites[i].Slice = &node.nineSlices[slice]
			slice++
		}
	}
	if s := node.snapshots; s != nil {
		s.materials = reuse.Compact(s.materials, 1)
		s.emitters = reuse.Compact(s.emitters, 1)
		s.tilemaps = reuse.Compact(s.tilemaps, 1)
		s.spines = reuse.Compact(s.spines, 1)
		s.particles = reuse.Compact(s.particles, 4)
		s.particleColors = reuse.Compact(s.particleColors, 4)
		s.particleAppearances = reuse.Compact(s.particleAppearances, 4)
		particle, color, appearance := 0, 0, 0
		for i := range s.emitters {
			e := &s.emitters[i]
			e.particles = s.particles[particle : particle+len(e.particles)]
			e.Colors = s.particleColors[color : color+len(e.Colors)]
			e.Appearances = s.particleAppearances[appearance : appearance+len(e.Appearances)]
			particle += len(e.particles)
			color += len(e.Colors)
			appearance += len(e.Appearances)
		}
		for i := range s.tilemaps {
			s.tilemaps[i].value.Tiles = reuse.Compact(s.tilemaps[i].value.Tiles, 4)
		}
	}
	// Extraction may have grown an owner's arrays between parts. Rebind by
	// emission order, including references to earlier backing arrays.
	sprite, text, emitter, tilemap, spine := 0, -1, 0, 0, 0
	var previousText *textPaint2D
	var previousMaterial *graphics.Material2DData
	material := -1
	for position := range node.positions {
		item := &items[position]
		if item.material2D != nil {
			if item.material2D != previousMaterial {
				previousMaterial = item.material2D
				material++
			}
			item.material2D = &node.snapshots.materials[material]
		}
		if item.snapshot[Sprite2D]() != nil {
			item.content = &node.sprites[sprite]
			sprite++
		}
		if item.snapshot[textPaint2D]() != nil {
			if item.snapshot[textPaint2D]() != previousText {
				previousText = item.snapshot[textPaint2D]()
				text++
			}
			item.content = &node.texts[text]
			node.text = item.snapshot[textPaint2D]()
		}
		if item.snapshot[ParticleEmitter2D]() != nil {
			item.content = &node.snapshots.emitters[emitter]
			emitter++
		}
		if item.snapshot[Tilemap2D]() != nil {
			item.content = &node.snapshots.tilemaps[tilemap].value
			tilemap++
		}
		if item.snapshot[Spine2D]() != nil {
			item.content = &node.snapshots.spines[spine]
			spine++
		}
	}
}

func (cache *retainedRender2D) trimNested() {
	for entity, retention := range cache.nestedRetention {
		node := cache.nodes[entity]
		if node == nil {
			delete(cache.nestedRetention, entity)
			continue
		}
		used, capacity := node.storage()
		if capacity <= 1024 || used > capacity/4 {
			delete(cache.nestedRetention, entity)
		} else if retention.TrimCapacity(capacity, used, 1024) != 0 {
			node.compact(cache.items)
			delete(cache.nestedRetention, entity)
		} else {
			cache.nestedRetention[entity] = retention
		}
	}
	for texture, retention := range cache.textureRetention {
		dependency := cache.textures[texture]
		if dependency == nil || cap(dependency.positions) <= 16 || len(dependency.positions) > cap(dependency.positions)/4 {
			delete(cache.textureRetention, texture)
		} else if retention.TrimCapacity(cap(dependency.positions), len(dependency.positions), 16) != 0 {
			dependency.positions = reuse.Compact(dependency.positions, 16)
			delete(cache.textureRetention, texture)
		} else {
			cache.textureRetention[texture] = retention
		}
	}
	if len(cache.nestedRetention) == 0 {
		cache.nestedRetention = nil
	}
	if len(cache.textureRetention) == 0 {
		cache.textureRetention = nil
	}
}

func (cache *retainedRender2D) retainTexture(texture *graphics.Texture, dependency *retainedTexture2D) {
	if cap(dependency.positions) <= 16 || len(dependency.positions) > cap(dependency.positions)/4 {
		return
	}
	if cache.textureRetention == nil {
		cache.textureRetention = make(map[*graphics.Texture]reuse.Retention)
	}
	if _, exists := cache.textureRetention[texture]; !exists {
		cache.textureRetention[texture] = reuse.Retention{}
	}
}

func (cache *retainedRender2D) compactNodes(target int) {
	blocks := cache.nodeBlocks[:0]
	for _, block := range cache.nodeBlocks {
		used := false
		for i := range block {
			used = used || block[i].allocated
		}
		if used {
			blocks = append(blocks, block)
		}
	}
	want := (max(0, target) + 63) / 64
	if len(blocks) > want {
		// Render items point into separately owned snapshot slices, not node
		// structs. Moving a node therefore preserves every drawing reference.
		blocks = make([][]retainedEntity2D, want)
		for i := range blocks {
			blocks[i] = make([]retainedEntity2D, 64)
		}
		index := 0
		for entity, old := range cache.nodes {
			node := &blocks[index/64][index%64]
			*node = *old
			cache.nodes[entity] = node
			index++
		}
	} else {
		if len(blocks) == len(cache.nodeBlocks) {
			return
		}
		clear(cache.nodeBlocks[len(blocks):])
		blocks = append([][]retainedEntity2D(nil), blocks...)
	}
	cache.nodeBlocks = blocks
	cache.freeNodes = make([]*retainedEntity2D, 0, len(blocks)*64-len(cache.nodes))
	for _, block := range blocks {
		for i := range block {
			if !block[i].allocated {
				cache.freeNodes = append(cache.freeNodes, &block[i])
			}
		}
	}
}
